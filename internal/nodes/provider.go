package nodes

import (
	"context"
	"io"
	"time"

	"github.com/lxcfh/lxcfh/internal/vfs"
)

// NodeInfo describes a registered storage node.
type NodeInfo struct {
	ID       string
	Name     string
	Provider string
	Host     string
	Port     int
	RootPath string
	Enabled  bool
}

// Credential holds authentication material for a node.
type Credential struct {
	ID         string
	NodeID     string
	Name       string
	AuthType   string
	Username   string
	Secret     []byte
	Passphrase []byte
}

// HealthStatus reports node reachability.
type HealthStatus struct {
	NodeID    string
	Status    string
	Latency   time.Duration
	Message   string
	CheckedAt time.Time
}

// NodeProvider abstracts remote storage backends.
type NodeProvider interface {
	Info() NodeInfo
	Backend() vfs.NodeBackend
	Ping(ctx context.Context) (HealthStatus, error)
	Close() error
}

// ProviderFactory builds a NodeProvider from node metadata and credentials.
type ProviderFactory interface {
	Create(ctx context.Context, node NodeInfo, cred Credential) (NodeProvider, error)
}

// TransferReader supports ranged reads for resumable transfers.
type TransferReader interface {
	ReadAt(ctx context.Context, path string, offset int64, buf []byte) (int, error)
	Size(ctx context.Context, path string) (int64, error)
}

// TransferWriter supports ranged writes for resumable transfers.
type TransferWriter interface {
	WriteAt(ctx context.Context, path string, offset int64, r io.Reader) (int64, error)
	Truncate(ctx context.Context, path string, size int64) error
}
