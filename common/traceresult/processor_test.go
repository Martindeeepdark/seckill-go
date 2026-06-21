//go:build integration

package traceresult

import (
	"context"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const testRedisAddr = "localhost:16379" // docker-compose 端口(见 CLAUDE.md)

func newTestClient(t *testing.T) *goredis.Client {
	t.Helper()
	client := goredis.NewClient(&goredis.Options{Addr: testRedisAddr})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Skipf("redis not available at %s: %v (run 'make redis' first)", testRedisAddr, err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestProcessorKey_UsesProcessorPrefix(t *testing.T) {
	got := ProcessorKey("TRACE1")
	want := "seckill:processor:idem:TRACE1"
	if got != want {
		t.Fatalf("ProcessorKey() = %q, want %q", got, want)
	}
}

func TestProcessorStore_TryStart_FirstCallSucceeds_SecondFails(t *testing.T) {
	client := newTestClient(t)
	store := NewProcessorStore(client)
	ctx := context.Background()
	traceID := "TEST-TryStart-" + t.Name()

	// 清理可能的残留
	_ = client.Del(ctx, ProcessorKey(traceID)).Err()
	t.Cleanup(func() { _ = client.Del(ctx, ProcessorKey(traceID)).Err() })

	first, err := store.TryStart(ctx, traceID, time.Minute)
	if err != nil {
		t.Fatalf("first TryStart: %v", err)
	}
	if !first {
		t.Fatal("first TryStart should succeed (key was absent)")
	}

	second, err := store.TryStart(ctx, traceID, time.Minute)
	if err != nil {
		t.Fatalf("second TryStart: %v", err)
	}
	if second {
		t.Fatal("second TryStart should fail (key already PROCESSING)")
	}
}

func TestProcessorStore_TryStart_EmptyTraceID_NoOp(t *testing.T) {
	client := newTestClient(t)
	store := NewProcessorStore(client)
	ok, err := store.TryStart(context.Background(), "", time.Minute)
	if err != nil {
		t.Fatalf("empty trace TryStart err: %v", err)
	}
	if ok {
		t.Fatal("empty trace TryStart should return false (no-op)")
	}
}

func TestProcessorStore_MarkSuccess_OverwritesProcessing(t *testing.T) {
	client := newTestClient(t)
	store := NewProcessorStore(client)
	ctx := context.Background()
	traceID := "TEST-MarkSuccess-" + t.Name()
	_ = client.Del(ctx, ProcessorKey(traceID)).Err()
	t.Cleanup(func() { _ = client.Del(ctx, ProcessorKey(traceID)).Err() })

	if _, err := store.TryStart(ctx, traceID, time.Minute); err != nil {
		t.Fatalf("TryStart: %v", err)
	}
	if err := store.MarkSuccess(ctx, traceID, "O123", 5*time.Minute); err != nil {
		t.Fatalf("MarkSuccess: %v", err)
	}

	val, err := client.Get(ctx, ProcessorKey(traceID)).Result()
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val != "O123" {
		t.Errorf("value = %s, want O123", val)
	}
}

func TestProcessorStore_MarkFail_OverwritesProcessing(t *testing.T) {
	client := newTestClient(t)
	store := NewProcessorStore(client)
	ctx := context.Background()
	traceID := "TEST-MarkFail-" + t.Name()
	_ = client.Del(ctx, ProcessorKey(traceID)).Err()
	t.Cleanup(func() { _ = client.Del(ctx, ProcessorKey(traceID)).Err() })

	if _, err := store.TryStart(ctx, traceID, time.Minute); err != nil {
		t.Fatalf("TryStart: %v", err)
	}
	if err := store.MarkFail(ctx, traceID, "STOCK_INSUFFICIENT", 5*time.Minute); err != nil {
		t.Fatalf("MarkFail: %v", err)
	}

	val, err := client.Get(ctx, ProcessorKey(traceID)).Result()
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val != "STOCK_INSUFFICIENT" {
		t.Errorf("value = %s, want STOCK_INSUFFICIENT", val)
	}
}

func TestProcessorStore_Release_DeletesProcessing(t *testing.T) {
	client := newTestClient(t)
	store := NewProcessorStore(client)
	ctx := context.Background()
	traceID := "TEST-Release-Processing-" + t.Name()
	_ = client.Del(ctx, ProcessorKey(traceID)).Err()
	t.Cleanup(func() { _ = client.Del(ctx, ProcessorKey(traceID)).Err() })

	if _, err := store.TryStart(ctx, traceID, time.Minute); err != nil {
		t.Fatalf("TryStart: %v", err)
	}
	if err := store.Release(ctx, traceID); err != nil {
		t.Fatalf("Release: %v", err)
	}

	exists, err := client.Exists(ctx, ProcessorKey(traceID)).Result()
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if exists != 0 {
		t.Errorf("key should be deleted, exists=%d", exists)
	}
}

// TestProcessorStore_Release_DoesNotDeleteFinalResult 验证 Lua CAS 不误删最终结果
func TestProcessorStore_Release_DoesNotDeleteFinalResult(t *testing.T) {
	client := newTestClient(t)
	store := NewProcessorStore(client)
	ctx := context.Background()
	traceID := "TEST-Release-Final-" + t.Name()
	_ = client.Del(ctx, ProcessorKey(traceID)).Err()
	t.Cleanup(func() { _ = client.Del(ctx, ProcessorKey(traceID)).Err() })

	// 写入最终结果(订单号)
	if err := store.MarkSuccess(ctx, traceID, "O456", 5*time.Minute); err != nil {
		t.Fatalf("MarkSuccess: %v", err)
	}

	// 错误路径调用 Release
	if err := store.Release(ctx, traceID); err != nil {
		t.Fatalf("Release: %v", err)
	}

	// 验证 key 仍然存在,值仍是订单号(Lua CAS 检测到不是 PROCESSING,未删除)
	val, err := client.Get(ctx, ProcessorKey(traceID)).Result()
	if err != nil {
		t.Fatalf("Get after release: %v", err)
	}
	if val != "O456" {
		t.Errorf("value after release = %s, want O456 (CAS should not delete final result)", val)
	}
}

// TestProcessorStore_Release_NonExistentKey_NoError 验证对不存在 key 的 Release 不报错
func TestProcessorStore_Release_NonExistentKey_NoError(t *testing.T) {
	client := newTestClient(t)
	store := NewProcessorStore(client)
	traceID := "TEST-Release-NonExistent-" + t.Name()
	_ = client.Del(context.Background(), ProcessorKey(traceID)).Err()

	if err := store.Release(context.Background(), traceID); err != nil {
		t.Errorf("Release on non-existent key should not error: %v", err)
	}
}
