package health

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/lxcfh/lxcfh/internal/nodes"
)

// Checker periodically pings registered node providers and records results.
type Checker struct {
	db      *sql.DB
	ttl     time.Duration
	mu      sync.RWMutex
	cache   map[string]cachedHealth
	workers int
}

type cachedHealth struct {
	status  nodes.HealthStatus
	expires time.Time
}

// NewChecker creates a health checker with the given result cache TTL.
func NewChecker(db *sql.DB, ttl time.Duration) *Checker {
	return &Checker{
		db:      db,
		ttl:     ttl,
		cache:   make(map[string]cachedHealth),
		workers: 4,
	}
}

// Ping checks a single provider and stores the result.
func (c *Checker) Ping(ctx context.Context, provider nodes.NodeProvider) (nodes.HealthStatus, error) {
	info := provider.Info()
	c.mu.RLock()
	if cached, ok := c.cache[info.ID]; ok && time.Now().Before(cached.expires) {
		c.mu.RUnlock()
		return cached.status, nil
	}
	c.mu.RUnlock()

	status, err := provider.Ping(ctx)
	if err != nil {
		status = nodes.HealthStatus{
			NodeID:    info.ID,
			Status:    "down",
			Message:   err.Error(),
			CheckedAt: time.Now(),
		}
	}
	c.mu.Lock()
	c.cache[info.ID] = cachedHealth{status: status, expires: time.Now().Add(c.ttl)}
	c.mu.Unlock()

	if err := c.persist(ctx, status); err != nil {
		return status, err
	}
	return status, nil
}

// PingAll checks all providers concurrently.
func (c *Checker) PingAll(ctx context.Context, providers []nodes.NodeProvider) []nodes.HealthStatus {
	results := make([]nodes.HealthStatus, len(providers))
	var wg sync.WaitGroup
	sem := make(chan struct{}, c.workers)
	for i, p := range providers {
		wg.Add(1)
		go func(idx int, provider nodes.NodeProvider) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			status, _ := c.Ping(ctx, provider)
			results[idx] = status
		}(i, p)
	}
	wg.Wait()
	return results
}

// Latest returns the most recent stored health check for a node.
func (c *Checker) Latest(ctx context.Context, nodeID string) (nodes.HealthStatus, error) {
	var status, message string
	var latency int
	var checkedAt string
	err := c.db.QueryRowContext(ctx,
		`SELECT status, latency_ms, message, checked_at FROM health_checks WHERE node_id = ? ORDER BY checked_at DESC LIMIT 1`,
		nodeID,
	).Scan(&status, &latency, &message, &checkedAt)
	if err != nil {
		return nodes.HealthStatus{}, err
	}
	t, _ := time.Parse(time.RFC3339, checkedAt)
	return nodes.HealthStatus{
		NodeID:    nodeID,
		Status:    status,
		Latency:   time.Duration(latency) * time.Millisecond,
		Message:   message,
		CheckedAt: t,
	}, nil
}

// Invalidate clears the in-memory cache for a node.
func (c *Checker) Invalidate(nodeID string) {
	c.mu.Lock()
	delete(c.cache, nodeID)
	c.mu.Unlock()
}

func (c *Checker) persist(ctx context.Context, status nodes.HealthStatus) error {
	id := status.NodeID + "-" + status.CheckedAt.Format(time.RFC3339Nano)
	_, err := c.db.ExecContext(ctx,
		`INSERT INTO health_checks (id, node_id, status, latency_ms, message, checked_at) VALUES (?, ?, ?, ?, ?, ?)`,
		id,
		status.NodeID,
		status.Status,
		status.Latency.Milliseconds(),
		status.Message,
		status.CheckedAt.UTC().Format(time.RFC3339),
	)
	return err
}
