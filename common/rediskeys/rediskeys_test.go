package rediskeys

import "testing"

func TestJavaCompatibleRedisKeys(t *testing.T) {
	cases := map[string]string{
		"activity info":           ActivityInfo("A1"),
		"activity product list":   ActivityProductList("A1"),
		"product sku stock":       ProductSKUStock("A1", "S1"),
		"product sku pattern":     ProductSKUStockPattern("A1"),
		"user purchase limit":     UserPurchaseLimit(7, "A1", "S1"),
		"user purchase pattern":   UserPurchaseLimitPattern("A1"),
		"legacy stock":            LegacyProductSKUStock("A1", "S1"),
		"legacy stock pattern":    LegacyProductSKUStockPattern("A1"),
		"legacy purchase":         LegacyUserPurchaseLimit(7, "A1", "S1"),
		"legacy purchase pattern": LegacyUserPurchaseLimitPattern("A1"),
	}

	want := map[string]string{
		"activity info":           "seckill:activity:info:A1",
		"activity product list":   "seckill:activity:product:list:A1",
		"product sku stock":       "seckill:product:sku:stock:A1:S1",
		"product sku pattern":     "seckill:product:sku:stock:A1:*",
		"user purchase limit":     "seckill:user:purchase:limit:7:A1:S1",
		"user purchase pattern":   "seckill:user:purchase:limit:*:A1:*",
		"legacy stock":            "seckill:stock:A1:S1",
		"legacy stock pattern":    "seckill:stock:A1:*",
		"legacy purchase":         "seckill:purchase:7:A1:S1",
		"legacy purchase pattern": "seckill:purchase:*:A1:*",
	}

	for name, got := range cases {
		if got != want[name] {
			t.Fatalf("%s = %q, want %q", name, got, want[name])
		}
	}
}
