package sftp

import (
	"context"
	"testing"
)

func TestPoolFactoryUsesGetContext(t *testing.T) {
	creationCtx, cancelCreation := context.WithCancel(context.Background())
	cancelCreation()

	var factoryCtx context.Context
	p := newPool(1, func(ctx context.Context) (*pooledConn, error) {
		factoryCtx = ctx
		return &pooledConn{}, nil
	})
	defer p.Close()

	getCtx := context.WithValue(context.Background(), struct{}{}, "get")
	conn, err := p.Get(getCtx)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	defer p.Put(conn)

	if factoryCtx != getCtx {
		t.Fatal("connection factory did not receive the Get context")
	}
	if creationCtx.Err() == nil {
		t.Fatal("creation context should be canceled for this regression test")
	}
	if factoryCtx.Err() != nil {
		t.Fatalf("factory context unexpectedly canceled: %v", factoryCtx.Err())
	}
}
