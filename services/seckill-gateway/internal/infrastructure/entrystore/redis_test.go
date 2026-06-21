package entrystore

import "testing"

func TestJavaCompatibleKeys(t *testing.T) {
	if got, want := UserRateLimitKey(7), "seckill:user:rate:limit:7"; got != want {
		t.Fatalf("UserRateLimitKey() = %q, want %q", got, want)
	}
	if got, want := OrderQueuingKey(7, "A1"), "seckill:order:queuing:7:A1"; got != want {
		t.Fatalf("OrderQueuingKey() = %q, want %q", got, want)
	}
}

func TestTryAcquireAndSetProcessingNilReceiver(t *testing.T) {
	var s *RedisStore
	allowed, err := s.TryAcquireAndSetProcessing(nil, 7, 10, 0, 0, "seckill:order:result:test")
	if err != nil || !allowed {
		t.Fatalf("nil receiver: allowed=%v err=%v, want true nil", allowed, err)
	}
}

func TestTryAcquireAndSetProcessingZeroUserID(t *testing.T) {
	s := &RedisStore{}
	allowed, err := s.TryAcquireAndSetProcessing(nil, 0, 10, 0, 0, "seckill:order:result:test")
	if err != nil || !allowed {
		t.Fatalf("zero userID: allowed=%v err=%v, want true nil", allowed, err)
	}
}

func TestTryAcquireAndSetProcessingZeroRate(t *testing.T) {
	s := &RedisStore{}
	allowed, err := s.TryAcquireAndSetProcessing(nil, 7, 0, 0, 0, "seckill:order:result:test")
	if err != nil || !allowed {
		t.Fatalf("zero rate: allowed=%v err=%v, want true nil", allowed, err)
	}
}
