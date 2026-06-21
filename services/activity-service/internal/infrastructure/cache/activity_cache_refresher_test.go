package cache

import "testing"

func TestActivityStockKeyUsesJavaCompatiblePrefix(t *testing.T) {
	if got := activityStockKey("A1", "S1"); got != "seckill:product:sku:stock:A1:S1" {
		t.Fatalf("activityStockKey = %q, want Java stock key", got)
	}
}
