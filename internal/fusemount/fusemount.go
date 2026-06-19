package fusemount

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// Mount exposes backendDir at mountPoint using a FUSE loopback filesystem.
// The process blocks until it receives SIGINT or SIGTERM.
func Mount(mountPoint, backendDir string) error {
	mountPoint = filepath.Clean(mountPoint)
	backendDir = filepath.Clean(backendDir)

	if err := os.MkdirAll(backendDir, 0o755); err != nil {
		return fmt.Errorf("backend dir: %w", err)
	}
	if err := os.MkdirAll(mountPoint, 0o755); err != nil {
		return fmt.Errorf("mount point: %w", err)
	}

	root, err := fs.NewLoopbackRoot(backendDir)
	if err != nil {
		return fmt.Errorf("loopback root: %w", err)
	}

	opts := &fs.Options{
		MountOptions: fuse.MountOptions{
			AllowOther:  true,
			Name:        "lxcfh-fuse",
			DirectMount: true,
		},
	}

	server, err := fs.Mount(mountPoint, root, opts)
	if err != nil {
		return fmt.Errorf("fuse mount: %w", err)
	}
	defer func() { _ = server.Unmount() }()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf("FUSE mounted %s -> %s", mountPoint, backendDir)

	done := make(chan struct{})
	go func() {
		server.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return server.Unmount()
	case <-done:
		return nil
	}
}
