package sftp

import (
	"context"
	"errors"
	"sync"
)

var ErrPoolClosed = errors.New("connection pool closed")

type factoryFunc func(context.Context) (*pooledConn, error)

type pool struct {
	size    int
	factory factoryFunc
	mu      sync.Mutex
	cond    *sync.Cond
	idle    []*pooledConn
	active  int
	closed  bool
}

func newPool(size int, factory factoryFunc) *pool {
	p := &pool{
		size:    size,
		factory: factory,
		idle:    make([]*pooledConn, 0, size),
	}
	p.cond = sync.NewCond(&p.mu)
	return p
}

func (p *pool) Get(ctx context.Context) (*pooledConn, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for {
		if p.closed {
			return nil, ErrPoolClosed
		}
		if len(p.idle) > 0 {
			conn := p.idle[len(p.idle)-1]
			p.idle = p.idle[:len(p.idle)-1]
			p.active++
			return conn, nil
		}
		if p.active < p.size {
			p.active++
			p.mu.Unlock()
			conn, err := p.factory(ctx)
			p.mu.Lock()
			if err != nil {
				p.active--
				return nil, err
			}
			return conn, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		done := make(chan struct{})
		go func() {
			select {
			case <-ctx.Done():
				p.cond.Broadcast()
			case <-done:
			}
		}()
		p.cond.Wait()
		close(done)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
}

func (p *pool) Put(conn *pooledConn) {
	if conn == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		conn.close()
		return
	}
	p.idle = append(p.idle, conn)
	p.active--
	p.cond.Signal()
}

func (p *pool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	for _, conn := range p.idle {
		conn.close()
	}
	p.idle = nil
	p.cond.Broadcast()
	return nil
}

func (c *pooledConn) close() {
	if c.client != nil {
		c.client.Close()
	}
	if c.ssh != nil {
		c.ssh.Close()
	}
}
