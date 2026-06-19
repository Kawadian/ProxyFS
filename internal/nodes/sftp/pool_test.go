package sftp

import (
	"context"
	"errors"
	"testing"
)

func TestPoolUsesCheckoutContextForNewConnections(t *testing.T) {
	refreshCtx, cancelRefresh := context.WithCancel(context.Background())
	cancelRefresh()

	var got context.Context
	p := newPool(1, func(ctx context.Context) (*pooledConn, error) {
		got = ctx
		return nil, errors.New("dial stopped")
	})

	checkoutCtx := context.WithValue(context.Background(), struct{}{}, "checkout")
	_, _ = p.Get(checkoutCtx)

	if got != checkoutCtx {
		t.Fatal("connection factory did not receive the checkout context")
	}
	if got == refreshCtx {
		t.Fatal("connection factory retained the refresh context")
	}
}
