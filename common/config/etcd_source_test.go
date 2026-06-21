package config

import (
	"context"
	"testing"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/api/v3/mvccpb"
)

// TestDeleteMethodExists verifies Delete method compiles and returns nil for nil receiver.
func TestDeleteMethodExists(t *testing.T) {
	var src *EtcdConfigSource
	err := src.Delete(context.Background(), "test")
	if err != nil {
		t.Errorf("nil receiver Delete should return nil, got %v", err)
	}
}

// TestWatchHandlesDeleteEvent verifies handleWatchEvent processes DELETE events.
func TestWatchHandlesDeleteEvent(t *testing.T) {
	var received map[string]interface{}
	onChange := func(cfg map[string]interface{}) {
		received = cfg
	}

	ev := &clientv3.Event{
		Type: clientv3.EventTypeDelete,
	}
	handleWatchEvent(ev, onChange)

	if received != nil {
		t.Errorf("DELETE event should call onChange(nil), got %v", received)
	}
}

// TestWatchHandlesPutEvent verifies handleWatchEvent processes PUT events.
func TestWatchHandlesPutEvent(t *testing.T) {
	var received map[string]interface{}
	onChange := func(cfg map[string]interface{}) {
		received = cfg
	}

	ev := &clientv3.Event{
		Type: clientv3.EventTypePut,
		Kv: &mvccpb.KeyValue{
			Value: []byte(`{"enabled": true, "max_qps": 200}`),
		},
	}
	handleWatchEvent(ev, onChange)

	if received == nil {
		t.Fatal("PUT event should call onChange with parsed config")
	}
	if received["enabled"] != true {
		t.Error("expected enabled=true")
	}
}
