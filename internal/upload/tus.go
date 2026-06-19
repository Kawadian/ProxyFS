package upload

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	ErrUploadNotFound = errors.New("upload not found")
	ErrOffsetMismatch = errors.New("upload offset mismatch")
	ErrUploadComplete = errors.New("upload already complete")
)

// Upload represents a tus-compatible resumable upload session.
type Upload struct {
	ID          string
	TusID       string
	NodeID      string
	UserID      string
	Path        string
	TotalSize   int64
	OffsetBytes int64
	Metadata    map[string]string
	Status      string
	StoragePath string
	ExpiresAt   time.Time
}

// Store manages tus upload sessions and on-disk staging files.
type Store struct {
	db       *sql.DB
	dataDir  string
	mu       sync.RWMutex
	handlers map[string]*os.File
}

// NewStore creates an upload store with a staging directory.
func NewStore(db *sql.DB, dataDir string) (*Store, error) {
	staging := filepath.Join(dataDir, "uploads")
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return nil, err
	}
	return &Store{
		db:       db,
		dataDir:  staging,
		handlers: make(map[string]*os.File),
	}, nil
}

// Create initiates a new tus upload (POST).
func (s *Store) Create(ctx context.Context, nodeID, userID, path string, totalSize int64, metadata map[string]string) (*Upload, error) {
	tusID, err := randomID(16)
	if err != nil {
		return nil, err
	}
	id, err := randomID(16)
	if err != nil {
		return nil, err
	}
	storagePath := filepath.Join(s.dataDir, tusID)
	f, err := os.OpenFile(storagePath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	f.Close()

	metaJSON, _ := json.Marshal(metadata)
	expires := time.Now().Add(24 * time.Hour)
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO uploads (id, node_id, user_id, tus_id, path, total_size, offset_bytes, metadata_json, status, storage_path, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, 0, ?, 'created', ?, ?)`,
		id, nodeID, nullStr(userID), tusID, path, totalSize, string(metaJSON), storagePath,
		expires.UTC().Format(time.RFC3339),
	)
	if err != nil {
		os.Remove(storagePath)
		return nil, err
	}
	return &Upload{
		ID:          id,
		TusID:       tusID,
		NodeID:      nodeID,
		UserID:      userID,
		Path:        path,
		TotalSize:   totalSize,
		Metadata:    metadata,
		Status:      "created",
		StoragePath: storagePath,
		ExpiresAt:   expires,
	}, nil
}

// Head returns upload metadata for tus HEAD requests.
func (s *Store) Head(ctx context.Context, tusID string) (*Upload, error) {
	return s.getByTusID(ctx, tusID)
}

// Patch appends data at the expected offset (tus PATCH).
func (s *Store) Patch(ctx context.Context, tusID string, offset int64, r io.Reader) (int64, error) {
	u, err := s.getByTusID(ctx, tusID)
	if err != nil {
		return 0, err
	}
	if u.Status == "completed" {
		return 0, ErrUploadComplete
	}
	if offset != u.OffsetBytes {
		return 0, fmt.Errorf("%w: expected %d, got %d", ErrOffsetMismatch, u.OffsetBytes, offset)
	}
	f, err := s.openFile(u)
	if err != nil {
		return 0, err
	}
	defer s.closeFile(tusID, f)

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return 0, err
	}
	written, err := io.Copy(f, r)
	if err != nil {
		return 0, err
	}
	newOffset := offset + written
	status := "uploading"
	if u.TotalSize > 0 && newOffset >= u.TotalSize {
		status = "completed"
	}
	_, err = s.db.ExecContext(ctx,
		`UPDATE uploads SET offset_bytes = ?, status = ?, updated_at = datetime('now') WHERE tus_id = ?`,
		newOffset, status, tusID,
	)
	return written, err
}

// Get returns an upload by tus ID.
func (s *Store) Get(ctx context.Context, tusID string) (*Upload, error) {
	return s.getByTusID(ctx, tusID)
}

// Cancel marks an upload as cancelled and removes the staging file.
func (s *Store) Cancel(ctx context.Context, tusID string) error {
	u, err := s.getByTusID(ctx, tusID)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`UPDATE uploads SET status = 'cancelled', updated_at = datetime('now') WHERE tus_id = ?`, tusID)
	if err != nil {
		return err
	}
	s.closeFile(tusID, nil)
	return os.Remove(u.StoragePath)
}

// OpenStaging opens the staging file for reading (e.g. to enqueue a transfer).
func (s *Store) OpenStaging(ctx context.Context, tusID string) (io.ReadCloser, int64, error) {
	u, err := s.getByTusID(ctx, tusID)
	if err != nil {
		return nil, 0, err
	}
	f, err := os.Open(u.StoragePath)
	if err != nil {
		return nil, 0, err
	}
	return f, u.TotalSize, nil
}

// TusHeaders returns standard tus response headers.
func (u *Upload) TusHeaders() map[string]string {
	return map[string]string{
		"Tus-Resumable":  "1.0.0",
		"Upload-Offset":  fmt.Sprintf("%d", u.OffsetBytes),
		"Upload-Length":  fmt.Sprintf("%d", u.TotalSize),
		"Upload-Expires": u.ExpiresAt.UTC().Format(time.RFC1123),
		"Cache-Control":  "no-store",
	}
}

func (s *Store) getByTusID(ctx context.Context, tusID string) (*Upload, error) {
	var u Upload
	var userID sql.NullString
	var metaJSON string
	var expiresStr sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, tus_id, node_id, user_id, path, total_size, offset_bytes, metadata_json, status, storage_path, expires_at
		 FROM uploads WHERE tus_id = ?`, tusID,
	).Scan(&u.ID, &u.TusID, &u.NodeID, &userID, &u.Path, &u.TotalSize, &u.OffsetBytes,
		&metaJSON, &u.Status, &u.StoragePath, &expiresStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUploadNotFound
		}
		return nil, err
	}
	if userID.Valid {
		u.UserID = userID.String
	}
	_ = json.Unmarshal([]byte(metaJSON), &u.Metadata)
	if u.Metadata == nil {
		u.Metadata = make(map[string]string)
	}
	if expiresStr.Valid {
		u.ExpiresAt, _ = time.Parse(time.RFC3339, expiresStr.String)
	}
	return &u, nil
}

func (s *Store) openFile(u *Upload) (*os.File, error) {
	s.mu.RLock()
	f, ok := s.handlers[u.TusID]
	s.mu.RUnlock()
	if ok {
		return f, nil
	}
	file, err := os.OpenFile(u.StoragePath, os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.handlers[u.TusID] = file
	s.mu.Unlock()
	return file, nil
}

func (s *Store) closeFile(tusID string, f *os.File) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f != nil {
		f.Close()
	}
	if cached, ok := s.handlers[tusID]; ok {
		cached.Close()
		delete(s.handlers, tusID)
	}
}

func randomID(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
