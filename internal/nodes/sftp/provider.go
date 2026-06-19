package sftp

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"sync"
	"time"

	"github.com/lxcfh/lxcfh/internal/nodes"
	"github.com/lxcfh/lxcfh/internal/vfs"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// Factory creates SFTP node providers.
type Factory struct {
	PoolSize int
	Timeout  time.Duration
}

// Create builds an SFTPNodeProvider for the given node and credential.
func (f *Factory) Create(ctx context.Context, node nodes.NodeInfo, cred nodes.Credential) (nodes.NodeProvider, error) {
	if node.Provider != "sftp" {
		return nil, fmt.Errorf("expected sftp provider, got %s", node.Provider)
	}
	poolSize := f.PoolSize
	if poolSize <= 0 {
		poolSize = 4
	}
	timeout := f.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	pool := newPool(poolSize, func(connectCtx context.Context) (*pooledConn, error) {
		return dial(connectCtx, node, cred, timeout)
	})
	return &SFTPNodeProvider{
		node: node,
		pool: pool,
	}, nil
}

// SFTPNodeProvider implements nodes.NodeProvider over SFTP.
type SFTPNodeProvider struct {
	node nodes.NodeInfo
	pool *pool
}

func (p *SFTPNodeProvider) Info() nodes.NodeInfo {
	return p.node
}

func (p *SFTPNodeProvider) Backend() vfs.NodeBackend {
	return &sftpBackend{provider: p, root: p.node.RootPath}
}

func (p *SFTPNodeProvider) Ping(ctx context.Context) (nodes.HealthStatus, error) {
	start := time.Now()
	conn, err := p.pool.Get(ctx)
	if err != nil {
		return nodes.HealthStatus{
			NodeID:    p.node.ID,
			Status:    "down",
			Message:   err.Error(),
			CheckedAt: time.Now(),
		}, nil
	}
	_, err = conn.client.Getwd()
	p.pool.Put(conn)
	latency := time.Since(start)
	status := "up"
	msg := "ok"
	if err != nil {
		status = "down"
		msg = err.Error()
	}
	return nodes.HealthStatus{
		NodeID:    p.node.ID,
		Status:    status,
		Latency:   latency,
		Message:   msg,
		CheckedAt: time.Now(),
	}, nil
}

func (p *SFTPNodeProvider) Close() error {
	return p.pool.Close()
}

type sftpBackend struct {
	provider *SFTPNodeProvider
	root     string
}

func (b *sftpBackend) join(nodePath string) string {
	clean := path.Clean("/" + nodePath)
	return path.Join(b.root, clean)
}

func (b *sftpBackend) withClient(ctx context.Context, fn func(*sftp.Client) error) error {
	conn, err := b.provider.pool.Get(ctx)
	if err != nil {
		return err
	}
	defer b.provider.pool.Put(conn)
	return fn(conn.client)
}

func (b *sftpBackend) Stat(ctx context.Context, nodePath string) (vfs.FileInfo, error) {
	var info vfs.FileInfo
	err := b.withClient(ctx, func(c *sftp.Client) error {
		fi, err := c.Stat(b.join(nodePath))
		if err != nil {
			if os.IsNotExist(err) {
				return vfs.ErrNotFound
			}
			return err
		}
		info = vfs.FileInfo{
			Name:    fi.Name(),
			Path:    nodePath,
			Size:    fi.Size(),
			Mode:    fi.Mode(),
			ModTime: fi.ModTime(),
			IsDir:   fi.IsDir(),
		}
		return nil
	})
	return info, err
}

func (b *sftpBackend) ReadDir(ctx context.Context, nodePath string) ([]vfs.FileInfo, error) {
	var out []vfs.FileInfo
	err := b.withClient(ctx, func(c *sftp.Client) error {
		entries, err := c.ReadDir(b.join(nodePath))
		if err != nil {
			if os.IsNotExist(err) {
				return vfs.ErrNotFound
			}
			return err
		}
		out = make([]vfs.FileInfo, 0, len(entries))
		for _, e := range entries {
			out = append(out, vfs.FileInfo{
				Name:    e.Name(),
				Path:    path.Join(nodePath, e.Name()),
				Size:    e.Size(),
				Mode:    e.Mode(),
				ModTime: e.ModTime(),
				IsDir:   e.IsDir(),
			})
		}
		return nil
	})
	return out, err
}

func (b *sftpBackend) Open(ctx context.Context, nodePath string) (io.ReadCloser, error) {
	conn, err := b.provider.pool.Get(ctx)
	if err != nil {
		return nil, err
	}
	f, err := conn.client.Open(b.join(nodePath))
	if err != nil {
		b.provider.pool.Put(conn)
		if os.IsNotExist(err) {
			return nil, vfs.ErrNotFound
		}
		return nil, err
	}
	return &pooledReadCloser{file: f, conn: conn, pool: b.provider.pool}, nil
}

func (b *sftpBackend) Write(ctx context.Context, nodePath string, offset int64, r io.Reader) (int64, error) {
	var n int64
	err := b.withClient(ctx, func(c *sftp.Client) error {
		full := b.join(nodePath)
		flags := os.O_CREATE | os.O_WRONLY
		f, err := c.OpenFile(full, flags)
		if err != nil {
			return err
		}
		defer f.Close()
		if offset > 0 {
			if _, err := f.Seek(offset, io.SeekStart); err != nil {
				return err
			}
		}
		written, err := io.Copy(f, r)
		n = written
		return err
	})
	return n, err
}

func (b *sftpBackend) Remove(ctx context.Context, nodePath string) error {
	return b.withClient(ctx, func(c *sftp.Client) error {
		return c.Remove(b.join(nodePath))
	})
}

func (b *sftpBackend) Mkdir(ctx context.Context, nodePath string) error {
	return b.withClient(ctx, func(c *sftp.Client) error {
		return c.MkdirAll(b.join(nodePath))
	})
}

func (b *sftpBackend) Rename(ctx context.Context, oldPath, newPath string) error {
	return b.withClient(ctx, func(c *sftp.Client) error {
		return c.Rename(b.join(oldPath), b.join(newPath))
	})
}

type pooledReadCloser struct {
	file *sftp.File
	conn *pooledConn
	pool *pool
	once sync.Once
}

func (p *pooledReadCloser) Read(buf []byte) (int, error) {
	return p.file.Read(buf)
}

func (p *pooledReadCloser) Close() error {
	var err error
	p.once.Do(func() {
		err = p.file.Close()
		p.pool.Put(p.conn)
	})
	return err
}

type pooledConn struct {
	ssh    *ssh.Client
	client *sftp.Client
}

func dial(ctx context.Context, node nodes.NodeInfo, cred nodes.Credential, timeout time.Duration) (*pooledConn, error) {
	addr := fmt.Sprintf("%s:%d", node.Host, node.Port)
	dialer := &net.Dialer{Timeout: timeout}
	var authMethods []ssh.AuthMethod
	switch cred.AuthType {
	case "password":
		authMethods = append(authMethods, ssh.Password(string(cred.Secret)))
	case "private_key":
		signer, err := ssh.ParsePrivateKeyWithPassphrase(cred.Secret, cred.Passphrase)
		if err != nil {
			signer, err = ssh.ParsePrivateKey(cred.Secret)
			if err != nil {
				return nil, fmt.Errorf("parse private key: %w", err)
			}
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	default:
		return nil, fmt.Errorf("unsupported auth type: %s", cred.AuthType)
	}
	config := &ssh.ClientConfig{
		User:            cred.Username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         timeout,
	}
	rawConn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	sshConn, chans, reqs, err := ssh.NewClientConn(rawConn, addr, config)
	if err != nil {
		rawConn.Close()
		return nil, err
	}
	client := ssh.NewClient(sshConn, chans, reqs)
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		client.Close()
		return nil, err
	}
	return &pooledConn{ssh: client, client: sftpClient}, nil
}
