package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config holds runtime configuration for the hub service.
type Config struct {
	BindHost          string
	BindPort          int
	DataDir           string
	DBPath            string
	DatabaseURL       string
	MigrationsPath    string
	MasterKeyPath     string
	MasterKey         []byte
	SMBEnabled        bool
	FuseMountPath     string
	FuseBackendPath   string
	SFTPPort          int
	SMBPort           int
	SessionTTL        time.Duration
	DirMetadataTTL    time.Duration
	StatTTL           time.Duration
	TransferChunkSize int64
	TransferWorkers   int
	HealthTTL         time.Duration
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	bindPort, err := strconv.Atoi(envOr("LXCFH_BIND_PORT", "8080"))
	if err != nil {
		return nil, fmt.Errorf("LXCFH_BIND_PORT: %w", err)
	}
	sftpPort, err := strconv.Atoi(envOr("LXCFH_SFTP_PORT", "2022"))
	if err != nil {
		return nil, fmt.Errorf("LXCFH_SFTP_PORT: %w", err)
	}
	smbPort, err := strconv.Atoi(envOr("LXCFH_SMB_PORT", "4450"))
	if err != nil {
		return nil, fmt.Errorf("LXCFH_SMB_PORT: %w", err)
	}
	transferWorkers, err := strconv.Atoi(envOr("LXCFH_TRANSFER_WORKERS", "4"))
	if err != nil {
		return nil, fmt.Errorf("LXCFH_TRANSFER_WORKERS: %w", err)
	}
	transferChunk, err := strconv.ParseInt(envOr("LXCFH_TRANSFER_CHUNK_SIZE", "8388608"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("LXCFH_TRANSFER_CHUNK_SIZE: %w", err)
	}

	dataDir := envOr("LXCFH_DATA_DIR", "/var/lib/lxcfh")
	dbPath := envOr("LXCFH_DB_PATH", filepath.Join(dataDir, "lxcfh.db"))
	masterKeyPath := envOr("LXCFH_MASTER_KEY_PATH", "/run/secrets/master.key")

	cfg := &Config{
		BindHost:          envOr("LXCFH_BIND_HOST", "0.0.0.0"),
		BindPort:          bindPort,
		DataDir:           dataDir,
		DBPath:            dbPath,
		DatabaseURL:       envOr("LXCFH_DATABASE_URL", "sqlite://"+dbPath),
		MigrationsPath:    envOr("LXCFH_MIGRATIONS_PATH", "/app/migrations"),
		MasterKeyPath:     masterKeyPath,
		SMBEnabled:        parseBool(envOr("SMB_ENABLED", "false")),
		FuseMountPath:     envOr("LXCFH_FUSE_MOUNT", "/fuse-mount"),
		FuseBackendPath:   envOr("LXCFH_FUSE_BACKEND", "/fuse-share"),
		SFTPPort:          sftpPort,
		SMBPort:           smbPort,
		SessionTTL:        parseDuration(envOr("LXCFH_SESSION_TTL", "12h")),
		DirMetadataTTL:    parseDuration(envOr("LXCFH_DIR_METADATA_TTL", "2s")),
		StatTTL:           parseDuration(envOr("LXCFH_STAT_TTL", "2s")),
		TransferChunkSize: transferChunk,
		TransferWorkers:   transferWorkers,
		HealthTTL:         parseDuration(envOr("LXCFH_HEALTH_TTL", "1h")),
	}

	if cfg.SMBEnabled && (cfg.FuseMountPath == "" || cfg.FuseBackendPath == "") {
		return nil, fmt.Errorf("fuse paths required when SMB_ENABLED=true")
	}

	masterKey, err := loadMasterKey(masterKeyPath)
	if err != nil {
		return nil, err
	}
	cfg.MasterKey = masterKey

	return cfg, nil
}

// WebAddr returns the HTTP listen address.
func (c *Config) WebAddr() string {
	return fmt.Sprintf("%s:%d", c.BindHost, c.BindPort)
}

func loadMasterKey(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Dev fallback: deterministic 32-byte key when secret file is absent.
			return []byte("lxcfh-dev-master-key-32-bytes!!"), nil
		}
		return nil, fmt.Errorf("read master key: %w", err)
	}
	key := strings.TrimSpace(string(raw))
	if len(key) < 32 {
		return nil, fmt.Errorf("master key at %s must be at least 32 bytes", path)
	}
	return []byte(key[:32]), nil
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func parseBool(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func parseDuration(v string) time.Duration {
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0
	}
	return d
}
