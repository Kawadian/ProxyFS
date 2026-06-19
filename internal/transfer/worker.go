package transfer

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/lxcfh/lxcfh/internal/vfs"
)

var (
	ErrTransferNotFound = errors.New("transfer not found")
	ErrTransferDone     = errors.New("transfer already completed")
)

// Job describes a file transfer job processed by the worker.
type Job struct {
	ID          string
	NodeID      string
	Direction   string
	SourcePath  string
	DestPath    string
	TotalBytes  int64
	OffsetBytes int64
	ChunkSize   int64
	Status      string
}

// Worker processes transfer jobs with resume support.
type Worker struct {
	db          *sql.DB
	vfs         *vfs.VirtualFS
	chunkSize   int64
	concurrency int
	logger      *slog.Logger
	wg          sync.WaitGroup
	cancel      context.CancelFunc
}

// NewWorker creates a transfer worker.
func NewWorker(db *sql.DB, fs *vfs.VirtualFS, chunkSize int64, concurrency int, logger *slog.Logger) *Worker {
	if logger == nil {
		logger = slog.Default()
	}
	if chunkSize <= 0 {
		chunkSize = 8 * 1024 * 1024
	}
	if concurrency <= 0 {
		concurrency = 2
	}
	return &Worker{
		db:          db,
		vfs:         fs,
		chunkSize:   chunkSize,
		concurrency: concurrency,
		logger:      logger,
	}
}

// Start launches background workers that poll for pending jobs.
func (w *Worker) Start(ctx context.Context) {
	ctx, w.cancel = context.WithCancel(ctx)
	sem := make(chan struct{}, w.concurrency)
	ticker := time.NewTicker(2 * time.Second)
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				jobs, err := w.pendingJobs(ctx)
				if err != nil {
					w.logger.Error("list pending transfers", "error", err)
					continue
				}
				for _, job := range jobs {
					select {
					case <-ctx.Done():
						return
					case sem <- struct{}{}:
						w.wg.Add(1)
						go func(j Job) {
							defer w.wg.Done()
							defer func() { <-sem }()
							if err := w.runJob(ctx, j); err != nil {
								w.logger.Error("transfer failed", "id", j.ID, "error", err)
								_ = w.markFailed(ctx, j.ID, err.Error())
							}
						}(job)
					}
				}
			}
		}
	}()
}

// Stop shuts down the worker pool.
func (w *Worker) Stop() {
	if w.cancel != nil {
		w.cancel()
	}
	w.wg.Wait()
}

func (w *Worker) pendingJobs(ctx context.Context) ([]Job, error) {
	rows, err := w.db.QueryContext(ctx,
		`SELECT id, node_id, direction, source_path, dest_path, COALESCE(bytes_total,0), COALESCE(bytes_done,0), status
		 FROM transfers WHERE status IN ('pending', 'queued') ORDER BY created_at LIMIT ?`,
		w.concurrency,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var jobs []Job
	for rows.Next() {
		var j Job
		if err := rows.Scan(&j.ID, &j.NodeID, &j.Direction, &j.SourcePath, &j.DestPath,
			&j.TotalBytes, &j.OffsetBytes, &j.Status); err != nil {
			return nil, err
		}
		j.ChunkSize = w.chunkSize
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

func (w *Worker) runJob(ctx context.Context, job Job) error {
	if err := w.setStatus(ctx, job.ID, "running"); err != nil {
		return err
	}
	for job.OffsetBytes < job.TotalBytes || job.TotalBytes == 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		remaining := job.TotalBytes - job.OffsetBytes
		if job.TotalBytes == 0 {
			remaining = w.chunkSize
		}
		chunk := w.chunkSize
		if remaining > 0 && remaining < chunk {
			chunk = remaining
		}
		written, err := w.transferChunk(ctx, job, chunk)
		if err != nil {
			return err
		}
		if written == 0 && job.TotalBytes > 0 {
			break
		}
		job.OffsetBytes += written
		if err := w.updateOffset(ctx, job.ID, job.OffsetBytes); err != nil {
			return err
		}
		if job.TotalBytes == 0 {
			break
		}
	}
	return w.setStatusCompleted(ctx, job.ID)
}

func (w *Worker) transferChunk(ctx context.Context, job Job, chunk int64) (int64, error) {
	buf := make([]byte, chunk)
	rc, err := w.vfs.Open(ctx, job.SourcePath)
	if err != nil {
		return 0, err
	}
	if job.OffsetBytes > 0 {
		if _, err := io.CopyN(io.Discard, rc, job.OffsetBytes); err != nil {
			rc.Close()
			return 0, err
		}
	}
	n, err := io.ReadFull(io.LimitReader(rc, chunk), buf)
	rc.Close()
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return 0, err
	}
	if n == 0 {
		return 0, nil
	}
	return w.vfs.Write(ctx, job.DestPath, job.OffsetBytes, bytes.NewReader(buf[:n]))
}

func (w *Worker) setStatus(ctx context.Context, id, status string) error {
	_, err := w.db.ExecContext(ctx,
		`UPDATE transfers SET status = ?, updated_at = ? WHERE id = ?`, status, time.Now().UTC().Format(time.RFC3339), id)
	return err
}

func (w *Worker) setStatusCompleted(ctx context.Context, id string) error {
	ts := time.Now().UTC().Format(time.RFC3339)
	_, err := w.db.ExecContext(ctx,
		`UPDATE transfers SET status = 'completed', completed_at = ?, updated_at = ? WHERE id = ?`, ts, ts, id)
	return err
}

func (w *Worker) markFailed(ctx context.Context, id, msg string) error {
	_, err := w.db.ExecContext(ctx,
		`UPDATE transfers SET status = 'failed', error = ?, updated_at = ? WHERE id = ?`, msg, time.Now().UTC().Format(time.RFC3339), id)
	return err
}

func (w *Worker) updateOffset(ctx context.Context, id string, offset int64) error {
	_, err := w.db.ExecContext(ctx,
		`UPDATE transfers SET bytes_done = ?, updated_at = ? WHERE id = ?`, offset, time.Now().UTC().Format(time.RFC3339), id)
	return err
}
