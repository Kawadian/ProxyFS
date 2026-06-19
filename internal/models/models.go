package models

import "time"

type Role string

const (
	RoleAdmin    Role = "admin"
	RoleEditor   Role = "editor"
	RoleOperator Role = "operator" // alias for editor
	RoleViewer   Role = "viewer"
)

type User struct {
	ID          string     `json:"id"`
	Username    string     `json:"username"`
	DisplayName string     `json:"display_name,omitempty"`
	Email       string     `json:"email,omitempty"`
	Role        Role       `json:"role"`
	Enabled     bool       `json:"enabled"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

type UserSSHKey struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	Fingerprint string    `json:"fingerprint"`
	PublicKey   string    `json:"public_key"`
	CreatedAt   time.Time `json:"created_at"`
}

type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	CSRFToken string    `json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type Node struct {
	ID                 string            `json:"id"`
	Name               string            `json:"name"`
	Slug               string            `json:"slug,omitempty"`
	DisplayName        string            `json:"display_name,omitempty"`
	Host               string            `json:"host"`
	Port               int               `json:"port"`
	Username           string            `json:"username"`
	RootPath           string            `json:"root_path,omitempty"`
	Provider           string            `json:"provider,omitempty"`
	CredentialID       string            `json:"credential_id,omitempty"`
	KeyID              string            `json:"key_id,omitempty"`
	Labels             map[string]string `json:"labels,omitempty"`
	Tags               []string          `json:"tags,omitempty"`
	Enabled            bool              `json:"enabled"`
	ReadOnly           bool              `json:"read_only,omitempty"`
	HostKeyStatus      string            `json:"host_key_status"`
	HostKeyFingerprint string            `json:"host_key_fingerprint,omitempty"`
	LastPingAt         *time.Time        `json:"last_ping_at,omitempty"`
	LastPingStatus     string            `json:"last_ping_status,omitempty"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
}

type Credential struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Username  string    `json:"username,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type SSHKey struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Fingerprint string    `json:"fingerprint"`
	PublicKey   string    `json:"public_key"`
	Comment     string    `json:"comment,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type FileEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	ModTime string `json:"mod_time"`
	Owner   string `json:"owner,omitempty"`
	Group   string `json:"group,omitempty"`
}

type FileStat struct {
	Path    string `json:"path"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	ModTime string `json:"mod_time"`
	Owner   string `json:"owner,omitempty"`
	Group   string `json:"group,omitempty"`
}

type TransferStatus string

const (
	TransferPending   TransferStatus = "pending"
	TransferRunning   TransferStatus = "running"
	TransferPaused    TransferStatus = "paused"
	TransferCompleted TransferStatus = "completed"
	TransferFailed    TransferStatus = "failed"
	TransferCancelled TransferStatus = "cancelled"
)

type Transfer struct {
	ID          string         `json:"id"`
	NodeID      string         `json:"node_id"`
	SourcePath  string         `json:"source_path"`
	DestPath    string         `json:"dest_path"`
	Direction   string         `json:"direction"`
	Status      TransferStatus `json:"status"`
	BytesTotal  int64          `json:"bytes_total"`
	BytesDone   int64          `json:"bytes_done"`
	Error       string         `json:"error,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
}

type Upload struct {
	ID        string    `json:"id"`
	NodeID    string    `json:"node_id"`
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
	Offset    int64     `json:"offset"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Settings struct {
	SiteName            string           `json:"site_name"`
	SessionTimeoutMin   int              `json:"session_timeout_min"`
	MaxUploadSizeMB     int              `json:"max_upload_size_mb"`
	RateLimitPerMinute  int              `json:"rate_limit_per_minute"`
	RequireReauth       bool             `json:"require_reauth"`
	AllowRegistration   bool             `json:"allow_registration"`
	DefaultNodePort     int              `json:"default_node_port"`
	BackupRetentionDays int              `json:"backup_retention_days"`
	Protocols           ProtocolSettings `json:"protocols"`
}

// ProtocolSettings controls hub protocol services from the Web UI.
type ProtocolSettings struct {
	SFTPEnabled   bool `json:"sftp_enabled"`
	WebDAVEnabled bool `json:"webdav_enabled"`
	SMBEnabled    bool `json:"smb_enabled"`
}

// DefaultProtocolSettings matches legacy always-on SFTP/WebDAV behavior.
func DefaultProtocolSettings() ProtocolSettings {
	return ProtocolSettings{
		SFTPEnabled:   true,
		WebDAVEnabled: true,
		SMBEnabled:    false,
	}
}

type ProtocolStatus struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	Running bool   `json:"running"`
	Port    int    `json:"port,omitempty"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message,omitempty"`
}

type ProtocolsOverview struct {
	Protocols []ProtocolStatus `json:"protocols"`
}

type Dashboard struct {
	NodeCount       int   `json:"node_count"`
	ActiveTransfers int   `json:"active_transfers"`
	TotalUsers      int   `json:"total_users"`
	StorageUsedMB   int64 `json:"storage_used_mb"`
	RecentErrors    int   `json:"recent_errors"`
}

type ConfigDocument struct {
	Version     int                    `json:"version" yaml:"version"`
	Settings    Settings               `json:"settings" yaml:"settings"`
	Nodes       []Node                 `json:"nodes" yaml:"nodes"`
	Credentials []Credential           `json:"credentials" yaml:"credentials"`
	Keys        []SSHKey               `json:"keys" yaml:"keys"`
	Users       []User                 `json:"users" yaml:"users"`
	Extra       map[string]interface{} `json:"extra,omitempty" yaml:"extra,omitempty"`
}

type PingResult struct {
	NodeID    string `json:"node_id"`
	Reachable bool   `json:"reachable"`
	LatencyMs int64  `json:"latency_ms"`
	Message   string `json:"message,omitempty"`
}

type TestResult struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

type BackupResult struct {
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}

type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

type ConfigPreview struct {
	Changes []ConfigChange `json:"changes"`
}

type ConfigChange struct {
	Resource string `json:"resource"`
	Action   string `json:"action"`
	Detail   string `json:"detail"`
}
