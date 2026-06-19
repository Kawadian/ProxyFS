package samba

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/lxcfh/lxcfh/internal/hub"
	"golang.org/x/crypto/md4" //nolint:staticcheck // Samba NT hash requires MD4
)

// Config controls Samba user synchronization.
type Config struct {
	SmbpasswdPath string
	PdbeditPath   string
	DryRun        bool
}

// Syncer synchronizes Hub users into Samba credentials with transactional compensation.
type Syncer struct {
	db     *sql.DB
	cfg    Config
	logger *slog.Logger
}

// UserRecord is a Hub user eligible for Samba sync.
type UserRecord struct {
	ID           string
	Username     string
	PasswordHash string
	Role         string
	Enabled      bool
}

// SyncResult summarizes a sync run.
type SyncResult struct {
	Created int
	Updated int
	Removed int
	Errors  []string
}

// compensation tracks external side effects for rollback.
type compensation struct {
	username string
	action   string // "add" | "delete"
}

// NewSyncer creates a Samba sync service.
func NewSyncer(db *sql.DB, cfg Config, logger *slog.Logger) *Syncer {
	if cfg.SmbpasswdPath == "" {
		cfg.SmbpasswdPath = "smbpasswd"
	}
	if cfg.PdbeditPath == "" {
		cfg.PdbeditPath = "pdbedit"
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Syncer{db: db, cfg: cfg, logger: logger}
}

// SyncAll reconciles all enabled Hub users with Samba.
func (s *Syncer) SyncAll(ctx context.Context, plaintextPasswords map[string]string) (*SyncResult, error) {
	if plaintextPasswords == nil {
		plaintextPasswords = make(map[string]string)
	}
	if pending, err := s.loadPendingPasswords(ctx); err != nil {
		return nil, err
	} else {
		for k, v := range pending {
			plaintextPasswords[k] = v
		}
	}

	users, err := s.listUsers(ctx)
	if err != nil {
		return nil, err
	}
	result := &SyncResult{}
	var stack []compensation

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
			s.compensate(stack)
		}
	}()

	seen := make(map[string]struct{})
	for _, u := range users {
		if !u.Enabled {
			continue
		}
		seen[u.Username] = struct{}{}
		plain := plaintextPasswords[u.Username]
		uid, gid := hub.UIDGID(u.ID)

		var existing, existingHash string
		err := tx.QueryRowContext(ctx, `SELECT username, nt_hash FROM samba_accounts WHERE user_id = ?`, u.ID).Scan(&existing, &existingHash)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			if plain == "" {
				plain = deriveSambaPassword(u.PasswordHash, u.Username)
			}
			ntHash := ntHashLM(plain)
			if err := s.ensureUnixUser(ctx, u.Username, uid, gid); err != nil {
				result.Errors = append(result.Errors, err.Error())
				return result, err
			}
			if err := s.applySambaUser(ctx, u.Username, plain, true); err != nil {
				result.Errors = append(result.Errors, err.Error())
				return result, err
			}
			stack = append(stack, compensation{username: u.Username, action: "add"})
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO samba_accounts (user_id, username, nt_hash, uid, gid, synced_at)
				VALUES (?, ?, ?, ?, ?, ?)`,
				u.ID, u.Username, ntHash, uid, gid, time.Now().UTC().Format(time.RFC3339),
			); err != nil {
				return result, fmt.Errorf("insert samba account %s: %w", u.Username, err)
			}
			s.logAction(ctx, tx, u.Username, "create", "ok", "")
			if _, err := tx.ExecContext(ctx, `DELETE FROM samba_pending_passwords WHERE username = ?`, u.Username); err != nil {
				return result, err
			}
			result.Created++
		default:
			if err != nil {
				return result, err
			}
			ntHash := existingHash
			if err := s.ensureUnixUser(ctx, u.Username, uid, gid); err != nil {
				result.Errors = append(result.Errors, err.Error())
				return result, err
			}
			exists, err := s.sambaUserExists(ctx, u.Username)
			if err != nil {
				return result, err
			}
			if !exists {
				createPassword := plain
				if createPassword == "" {
					createPassword = deriveSambaPassword(u.PasswordHash, u.Username)
				}
				if err := s.applySambaUser(ctx, u.Username, createPassword, true); err != nil {
					result.Errors = append(result.Errors, err.Error())
					return result, err
				}
				if plain == "" {
					if err := s.setNTHash(ctx, u.Username, existingHash); err != nil {
						result.Errors = append(result.Errors, err.Error())
						return result, err
					}
				} else {
					ntHash = ntHashLM(plain)
				}
			} else if plain != "" {
				if err := s.applySambaUser(ctx, u.Username, plain, false); err != nil {
					result.Errors = append(result.Errors, err.Error())
					return result, err
				}
				ntHash = ntHashLM(plain)
			}
			if _, err := tx.ExecContext(ctx, `
				UPDATE samba_accounts SET nt_hash = ?, uid = ?, gid = ?, synced_at = ? WHERE user_id = ?`,
				ntHash, uid, gid, time.Now().UTC().Format(time.RFC3339), u.ID,
			); err != nil {
				return result, fmt.Errorf("update samba account %s: %w", u.Username, err)
			}
			s.logAction(ctx, tx, u.Username, "update", "ok", "")
			if _, err := tx.ExecContext(ctx, `DELETE FROM samba_pending_passwords WHERE username = ?`, u.Username); err != nil {
				return result, err
			}
			result.Updated++
		}
	}

	rows, err := tx.QueryContext(ctx, `SELECT user_id, username FROM samba_accounts`)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var userID, username string
		if err := rows.Scan(&userID, &username); err != nil {
			return result, err
		}
		if _, ok := seen[username]; ok {
			continue
		}
		if err := s.removeSambaUser(ctx, username); err != nil {
			result.Errors = append(result.Errors, err.Error())
			return result, err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM samba_accounts WHERE user_id = ?`, userID); err != nil {
			return result, err
		}
		s.logAction(ctx, tx, username, "delete", "ok", "")
		result.Removed++
	}

	if err := tx.Commit(); err != nil {
		s.compensate(stack)
		return result, err
	}
	committed = true
	stack = nil
	return result, nil
}

func (s *Syncer) ensureUnixUser(ctx context.Context, username string, uid, gid uint32) error {
	if s.cfg.DryRun {
		return nil
	}
	if err := exec.CommandContext(ctx, "getent", "passwd", username).Run(); err == nil {
		return nil
	}

	groupName := fmt.Sprintf("lxcfh-%d", gid)
	if err := exec.CommandContext(ctx, "getent", "group", fmt.Sprint(gid)).Run(); err != nil {
		if out, err := exec.CommandContext(ctx, "groupadd", "--gid", fmt.Sprint(gid), groupName).CombinedOutput(); err != nil {
			return fmt.Errorf("create unix group %s: %w: %s", groupName, err, strings.TrimSpace(string(out)))
		}
	}
	if out, err := exec.CommandContext(
		ctx,
		"useradd",
		"--uid", fmt.Sprint(uid),
		"--gid", fmt.Sprint(gid),
		"--no-create-home",
		"--shell", "/usr/sbin/nologin",
		username,
	).CombinedOutput(); err != nil {
		return fmt.Errorf("create unix user %s: %w: %s", username, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (s *Syncer) sambaUserExists(ctx context.Context, username string) (bool, error) {
	if s.cfg.DryRun {
		return true, nil
	}
	cmd := exec.CommandContext(ctx, s.cfg.PdbeditPath, "-L", "-u", username)
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *Syncer) setNTHash(ctx context.Context, username, ntHash string) error {
	if s.cfg.DryRun {
		return nil
	}
	out, err := exec.CommandContext(
		ctx,
		s.cfg.PdbeditPath,
		"-r",
		"-u", username,
		"--set-nt-hash", ntHash,
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("restore samba password %s: %w: %s", username, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (s *Syncer) loadPendingPasswords(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT username, password FROM samba_pending_passwords`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]string)
	for rows.Next() {
		var username, password string
		if err := rows.Scan(&username, &password); err != nil {
			return nil, err
		}
		out[username] = password
	}
	return out, rows.Err()
}

func (s *Syncer) listUsers(ctx context.Context) ([]UserRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, username, password_hash, role, enabled FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UserRecord
	for rows.Next() {
		var u UserRecord
		var enabled int
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &enabled); err != nil {
			return nil, err
		}
		u.Enabled = enabled == 1
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Syncer) applySambaUser(ctx context.Context, username, password string, create bool) error {
	if s.cfg.DryRun {
		s.logger.Info("samba dry-run", "user", username, "create", create)
		return nil
	}
	if create {
		cmd := exec.CommandContext(ctx, s.cfg.SmbpasswdPath, "-a", "-s", username)
		cmd.Stdin = strings.NewReader(password + "\n" + password + "\n")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("smbpasswd add %s: %w", username, err)
		}
		return nil
	}
	cmd := exec.CommandContext(ctx, s.cfg.SmbpasswdPath, "-s", username)
	cmd.Stdin = strings.NewReader(password + "\n" + password + "\n")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("smbpasswd update %s: %w", username, err)
	}
	return nil
}

func (s *Syncer) removeSambaUser(ctx context.Context, username string) error {
	if s.cfg.DryRun {
		s.logger.Info("samba dry-run delete", "user", username)
		return nil
	}
	cmd := exec.CommandContext(ctx, s.cfg.SmbpasswdPath, "-x", username)
	if err := cmd.Run(); err != nil {
		// user may not exist in samba yet
		s.logger.Warn("smbpasswd delete", "user", username, "error", err)
	}
	return nil
}

func (s *Syncer) compensate(stack []compensation) {
	for i := len(stack) - 1; i >= 0; i-- {
		c := stack[i]
		switch c.action {
		case "add":
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			_ = s.removeSambaUser(ctx, c.username)
			cancel()
			s.logger.Warn("compensated samba add", "user", c.username)
		}
	}
}

type contextExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func (s *Syncer) logAction(ctx context.Context, execer contextExecer, username, action, status, detail string) {
	_, _ = execer.ExecContext(ctx, `
		INSERT INTO samba_sync_log (username, action, status, detail, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		username, action, status, detail, time.Now().UTC().Format(time.RFC3339),
	)
}

func deriveSambaPassword(passwordHash, username string) string {
	sum := md5.Sum([]byte(passwordHash + ":" + username))
	return hex.EncodeToString(sum[:16])
}

// ntHashLM computes the Samba NT hash (MD4 of UTF-16LE password).
func ntHashLM(password string) string {
	runes := utf16.Encode([]rune(password))
	buf := make([]byte, len(runes)*2)
	for i, r := range runes {
		buf[i*2] = byte(r)
		buf[i*2+1] = byte(r >> 8)
	}
	h := md4.New()
	_, _ = h.Write(buf)
	return strings.ToUpper(hex.EncodeToString(h.Sum(nil)))
}

// RenderSMBConf fills the Samba configuration template.
func RenderSMBConf(template string, sharePath, workgroup string) string {
	if workgroup == "" {
		workgroup = "LXCFH"
	}
	out := strings.ReplaceAll(template, "{{SHARE_PATH}}", sharePath)
	out = strings.ReplaceAll(out, "{{WORKGROUP}}", workgroup)
	return out
}
