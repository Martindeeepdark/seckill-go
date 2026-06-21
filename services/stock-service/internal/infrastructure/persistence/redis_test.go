package persistence

import (
	"errors"
	"testing"
)

func TestRedisBusinessRejectionClassifiers(t *testing.T) {
	t.Run("limit exceeded", func(t *testing.T) {
		if !isLimitExceeded(errors.New("cache check_limit seckill:purchase:1:A1:S1: would exceed limit 1")) {
			t.Fatal("expected purchase limit error to be classified as business rejection")
		}
		if isLimitExceeded(errors.New("redis unavailable")) {
			t.Fatal("expected redis unavailable to remain infrastructure error")
		}
	})

	t.Run("stock rejected", func(t *testing.T) {
		cases := []string{
			"cache deduct_stock_order seckill:product:sku:stock:A1:S1: insufficient stock",
			"cache deduct_stock_order seckill:product:sku:stock:A1:S1: key not found",
		}
		for _, msg := range cases {
			if !isStockRejected(errors.New(msg)) {
				t.Fatalf("expected %q to be classified as stock rejection", msg)
			}
		}
		if isStockRejected(errors.New("redis connection refused")) {
			t.Fatal("expected redis connection failure to remain infrastructure error")
		}
	})
}

func TestRedisKeysUseJavaCompatibleStockAndPurchasePrefixes(t *testing.T) {
	if got := redisStockKey("A1", "S1"); got != "seckill:product:sku:stock:A1:S1" {
		t.Fatalf("redisStockKey = %q, want Java stock key", got)
	}
	if got := legacyRedisStockKey("A1", "S1"); got != "seckill:stock:A1:S1" {
		t.Fatalf("legacyRedisStockKey = %q, want old Go stock key", got)
	}
	if got := redisUserPurchaseKey(7, "A1", "S1"); got != "seckill:user:purchase:limit:7:A1:S1" {
		t.Fatalf("redisUserPurchaseKey = %q, want Java purchase key", got)
	}
}
