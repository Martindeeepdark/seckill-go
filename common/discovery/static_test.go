package discovery

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestStaticDiscoveryWatchReturnsConfiguredInstancesOnce(t *testing.T) {
	discovery := NewStaticDiscovery(map[string][]string{
		"order-service": {"grpc://127.0.0.1:9004", "grpc://127.0.0.1:9104"},
	})
	watcher, err := discovery.Watch(context.Background(), "order-service")
	if err != nil {
		t.Fatalf("watch: %v", err)
	}
	defer func() {
		if err := watcher.Stop(); err != nil {
			t.Fatalf("stop watcher: %v", err)
		}
	}()

	instances := nextWatcher(t, watcher)
	if got := endpointsOf(instances); fmt.Sprint(got) != "[grpc://127.0.0.1:9004 grpc://127.0.0.1:9104]" {
		t.Fatalf("endpoints = %v, want configured endpoints", got)
	}
}

func TestStaticDiscoveryWatchBlocksAfterInitialSnapshotUntilStopped(t *testing.T) {
	discovery := NewStaticDiscovery(map[string][]string{
		"stock-service": {"grpc://127.0.0.1:9002"},
	})
	watcher, err := discovery.Watch(context.Background(), "stock-service")
	if err != nil {
		t.Fatalf("watch: %v", err)
	}
	_ = nextWatcher(t, watcher)

	nextCh := make(chan watcherResult, 1)
	go func() {
		instances, err := watcher.Next()
		nextCh <- watcherResult{instances: instances, err: err}
	}()

	select {
	case result := <-nextCh:
		t.Fatalf("watcher returned before stop: %+v", result)
	case <-time.After(15 * time.Millisecond):
	}

	if err := watcher.Stop(); err != nil {
		t.Fatalf("stop watcher: %v", err)
	}
	result := receiveWatcherResult(t, nextCh)
	if !errors.Is(result.err, context.Canceled) {
		t.Fatalf("next err = %v, want context canceled", result.err)
	}
}
