package discovery

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/registry"
	goredis "github.com/redis/go-redis/v9"
)

func TestRedisRegistryRegisterStoresEndpointAndRefreshesTTL(t *testing.T) {
	ctx := context.Background()
	store := newFakeRedisRegistryStore()
	r := &RedisRegistry{commands: store, namespace: "test", ttl: 3 * time.Second}

	instance := &registry.ServiceInstance{
		Name:      "order-service",
		Version:   "v1",
		Endpoints: []string{"grpc://127.0.0.1:9004"},
	}
	if err := r.Register(ctx, instance); err != nil {
		t.Fatalf("register: %v", err)
	}

	key := instanceKey("test", "order-service", "v1")
	if got := store.hashes[key]["grpc://127.0.0.1:9004"]; got != "grpc://127.0.0.1:9004" {
		t.Fatalf("registered endpoint = %q", got)
	}
	if got := store.ttls[key]; got != 3*time.Second {
		t.Fatalf("ttl = %s, want 3s", got)
	}

	store.ttls[key] = time.Second
	if err := r.Register(ctx, instance); err != nil {
		t.Fatalf("refresh register: %v", err)
	}
	if refreshedTTL := store.ttls[key]; refreshedTTL != 3*time.Second {
		t.Fatalf("refreshed ttl = %s, want 3s", refreshedTTL)
	}
}

func TestRedisRegistryDeregisterRemovesEndpoint(t *testing.T) {
	ctx := context.Background()
	store := newFakeRedisRegistryStore()
	r := &RedisRegistry{commands: store, namespace: "test", ttl: time.Minute}

	instance := &registry.ServiceInstance{
		Name:      "stock-service",
		Version:   "v1",
		Endpoints: []string{"grpc://127.0.0.1:9002"},
	}
	if err := r.Register(ctx, instance); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := r.Deregister(ctx, instance); err != nil {
		t.Fatalf("deregister: %v", err)
	}
	key := instanceKey("test", "stock-service", "v1")
	if got := store.hashes[key]["grpc://127.0.0.1:9002"]; got != "" {
		t.Fatalf("endpoint after deregister = %q, want empty", got)
	}
}

func TestRedisRegistryWatchReturnsInitialInstancesAndUpdates(t *testing.T) {
	ctx := context.Background()
	store := newFakeRedisRegistryStore()
	r := &RedisRegistry{commands: store, namespace: "test", poll: 5 * time.Millisecond}
	if err := r.Register(ctx, &registry.ServiceInstance{
		Name:      "activity-service",
		Version:   "v1",
		Endpoints: []string{"grpc://127.0.0.1:9001"},
	}); err != nil {
		t.Fatalf("register initial: %v", err)
	}
	watcher, err := r.Watch(ctx, "activity-service")
	if err != nil {
		t.Fatalf("watch: %v", err)
	}
	defer func() {
		if err := watcher.Stop(); err != nil {
			t.Fatalf("stop watcher: %v", err)
		}
	}()

	initial := nextWatcher(t, watcher)
	if got := endpointsOf(initial); fmt.Sprint(got) != "[grpc://127.0.0.1:9001]" {
		t.Fatalf("initial endpoints = %v, want first endpoint", got)
	}

	nextCh := make(chan watcherResult, 1)
	go func() {
		instances, err := watcher.Next()
		nextCh <- watcherResult{instances: instances, err: err}
	}()
	if err := r.Register(ctx, &registry.ServiceInstance{
		Name:      "activity-service",
		Version:   "v1",
		Endpoints: []string{"grpc://127.0.0.1:9101"},
	}); err != nil {
		t.Fatalf("register update: %v", err)
	}

	result := receiveWatcherResult(t, nextCh)
	if result.err != nil {
		t.Fatalf("next update: %v", result.err)
	}
	if got := endpointsOf(result.instances); fmt.Sprint(got) != "[grpc://127.0.0.1:9001 grpc://127.0.0.1:9101]" {
		t.Fatalf("updated endpoints = %v, want both endpoints", got)
	}
}

func TestRedisRegistryWatchBlocksUntilFirstNonEmpty(t *testing.T) {
	ctx := context.Background()
	store := newFakeRedisRegistryStore()
	r := &RedisRegistry{commands: store, namespace: "test", poll: 5 * time.Millisecond}
	watcher, err := r.Watch(ctx, "risk-service")
	if err != nil {
		t.Fatalf("watch: %v", err)
	}
	defer func() {
		if err := watcher.Stop(); err != nil {
			t.Fatalf("stop watcher: %v", err)
		}
	}()

	nextCh := make(chan watcherResult, 1)
	go func() {
		instances, err := watcher.Next()
		nextCh <- watcherResult{instances: instances, err: err}
	}()
	select {
	case result := <-nextCh:
		t.Fatalf("watcher returned before first instance: %+v", result)
	case <-time.After(15 * time.Millisecond):
	}

	if err := r.Register(ctx, &registry.ServiceInstance{
		Name:      "risk-service",
		Version:   "v1",
		Endpoints: []string{"grpc://127.0.0.1:9003"},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	result := receiveWatcherResult(t, nextCh)
	if result.err != nil {
		t.Fatalf("next first non-empty: %v", result.err)
	}
	if got := endpointsOf(result.instances); fmt.Sprint(got) != "[grpc://127.0.0.1:9003]" {
		t.Fatalf("endpoints = %v, want risk endpoint", got)
	}
}

func TestRedisRegistryWatchStopUnblocksNext(t *testing.T) {
	ctx := context.Background()
	store := newFakeRedisRegistryStore()
	r := &RedisRegistry{commands: store, namespace: "test", poll: time.Minute}
	watcher, err := r.Watch(ctx, "order-service")
	if err != nil {
		t.Fatalf("watch: %v", err)
	}

	nextCh := make(chan watcherResult, 1)
	go func() {
		instances, err := watcher.Next()
		nextCh <- watcherResult{instances: instances, err: err}
	}()
	if err := watcher.Stop(); err != nil {
		t.Fatalf("stop watcher: %v", err)
	}
	result := receiveWatcherResult(t, nextCh)
	if !errors.Is(result.err, context.Canceled) {
		t.Fatalf("next err = %v, want context canceled", result.err)
	}
}

type fakeRedisRegistryStore struct {
	mu     sync.Mutex
	hashes map[string]map[string]string
	ttls   map[string]time.Duration
}

func newFakeRedisRegistryStore() *fakeRedisRegistryStore {
	return &fakeRedisRegistryStore{
		hashes: make(map[string]map[string]string),
		ttls:   make(map[string]time.Duration),
	}
}

func (s *fakeRedisRegistryStore) HSet(_ context.Context, key string, values ...interface{}) *goredis.IntCmd {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.hashes[key] == nil {
		s.hashes[key] = make(map[string]string)
	}
	var count int64
	for i := 0; i+1 < len(values); i += 2 {
		field := fmt.Sprint(values[i])
		value := fmt.Sprint(values[i+1])
		if _, exists := s.hashes[key][field]; !exists {
			count++
		}
		s.hashes[key][field] = value
	}
	return goredis.NewIntResult(count, nil)
}

func (s *fakeRedisRegistryStore) HDel(_ context.Context, key string, fields ...string) *goredis.IntCmd {
	s.mu.Lock()
	defer s.mu.Unlock()
	var count int64
	for _, field := range fields {
		if _, exists := s.hashes[key][field]; exists {
			delete(s.hashes[key], field)
			count++
		}
	}
	return goredis.NewIntResult(count, nil)
}

func (s *fakeRedisRegistryStore) Expire(_ context.Context, key string, ttl time.Duration) *goredis.BoolCmd {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ttls[key] = ttl
	return goredis.NewBoolResult(true, nil)
}

func (s *fakeRedisRegistryStore) HGetAll(_ context.Context, key string) *goredis.MapStringStringCmd {
	s.mu.Lock()
	defer s.mu.Unlock()
	values := make(map[string]string, len(s.hashes[key]))
	for field, value := range s.hashes[key] {
		values[field] = value
	}
	return goredis.NewMapStringStringResult(values, nil)
}

type watcherResult struct {
	instances []*registry.ServiceInstance
	err       error
}

func nextWatcher(t *testing.T, watcher registry.Watcher) []*registry.ServiceInstance {
	t.Helper()
	nextCh := make(chan watcherResult, 1)
	go func() {
		instances, err := watcher.Next()
		nextCh <- watcherResult{instances: instances, err: err}
	}()
	result := receiveWatcherResult(t, nextCh)
	if result.err != nil {
		t.Fatalf("watcher next: %v", result.err)
	}
	return result.instances
}

func receiveWatcherResult(t *testing.T, nextCh <-chan watcherResult) watcherResult {
	t.Helper()
	select {
	case result := <-nextCh:
		return result
	case <-time.After(500 * time.Millisecond):
		t.Fatal("watcher next timed out")
		return watcherResult{}
	}
}

func endpointsOf(instances []*registry.ServiceInstance) []string {
	endpoints := make([]string, 0, len(instances))
	for _, instance := range instances {
		endpoints = append(endpoints, instance.Endpoints...)
	}
	return endpoints
}
