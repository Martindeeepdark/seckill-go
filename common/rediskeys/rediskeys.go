// Package rediskeys 集中管理 Redis key 名称，这些 key 必须与
// Java 秒杀参考服务保持兼容。
package rediskeys

import "fmt"

const (
	ActivityInfoPrefix        = "seckill:activity:info:"
	ActivityProductListPrefix = "seckill:activity:product:list:"
	ProductSKUStockPrefix     = "seckill:product:sku:stock:"
	UserPurchaseLimitPrefix   = "seckill:user:purchase:limit:"

	LegacyProductSKUStockPrefix   = "seckill:stock:"
	LegacyUserPurchaseLimitPrefix = "seckill:purchase:"
)

// ActivityInfo 拼接活动详情的 Redis key。
func ActivityInfo(activityNo string) string {
	return ActivityInfoPrefix + activityNo
}

// ActivityProductList 拼接活动商品列表的 Redis key。
func ActivityProductList(activityNo string) string {
	return ActivityProductListPrefix + activityNo
}

// ProductSKUStock 拼接活动 SKU 库存的 Redis key。
func ProductSKUStock(activityNo, skuNo string) string {
	return fmt.Sprintf("%s%s:%s", ProductSKUStockPrefix, activityNo, skuNo)
}

// ProductSKUStockPattern 拼接某活动下所有 SKU 库存 key 的扫描通配符。
func ProductSKUStockPattern(activityNo string) string {
	return fmt.Sprintf("%s%s:*", ProductSKUStockPrefix, activityNo)
}

// LegacyProductSKUStock 拼接旧版库存 key（兼容 Java 参考服务）。
func LegacyProductSKUStock(activityNo, skuNo string) string {
	return fmt.Sprintf("%s%s:%s", LegacyProductSKUStockPrefix, activityNo, skuNo)
}

// LegacyProductSKUStockPattern 拼接旧版库存 key 的扫描通配符。
func LegacyProductSKUStockPattern(activityNo string) string {
	return fmt.Sprintf("%s%s:*", LegacyProductSKUStockPrefix, activityNo)
}

// UserPurchaseLimit 拼接用户限购记录的 Redis key。
func UserPurchaseLimit(userID int64, activityNo, skuNo string) string {
	return fmt.Sprintf("%s%d:%s:%s", UserPurchaseLimitPrefix, userID, activityNo, skuNo)
}

// UserPurchaseLimitPattern 拼接某活动下所有限购记录 key 的扫描通配符。
func UserPurchaseLimitPattern(activityNo string) string {
	return fmt.Sprintf("%s*:%s:*", UserPurchaseLimitPrefix, activityNo)
}

// LegacyUserPurchaseLimit 拼接旧版用户限购记录 key（兼容 Java 参考服务）。
func LegacyUserPurchaseLimit(userID int64, activityNo, skuNo string) string {
	return fmt.Sprintf("%s%d:%s:%s", LegacyUserPurchaseLimitPrefix, userID, activityNo, skuNo)
}

// LegacyUserPurchaseLimitPattern 拼接旧版限购记录 key 的扫描通配符。
func LegacyUserPurchaseLimitPattern(activityNo string) string {
	return fmt.Sprintf("%s*:%s:*", LegacyUserPurchaseLimitPrefix, activityNo)
}
