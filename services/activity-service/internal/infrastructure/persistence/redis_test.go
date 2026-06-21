package persistence

import "testing"

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
