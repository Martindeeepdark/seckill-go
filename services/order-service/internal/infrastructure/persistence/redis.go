// Package persistence 提供 Redis 存储实现，作为内存存储的缓存层
package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	commonredis "github.com/Martindeeepdark/go-common/cache/redis"
	goredis "github.com/redis/go-redis/v9"

	"seckill-order-service/internal/application"
	"seckill-order-service/internal/domain/entity"
)

// RedisStore Redis 存储，组合内存存储实现读写穿透
type RedisStore struct {
	*MemoryStore                     // 嵌入内存存储作为后备
	cache        *commonredis.Client // Redis 客户端
}

// NewRedisStore 创建 Redis 存储实例
// ctx: 上下文
// addr: Redis 地址
// password: Redis 密码
// db: Redis 数据库编号
// memory: 内存存储实例
// 返回 Redis 存储实例和错误
func NewRedisStore(ctx context.Context, addr, password string, db int, memory *MemoryStore) (*RedisStore, error) {
	cache := commonredis.New(addr, commonredis.WithPassword(password), commonredis.WithDB(db))
	if err := cache.Redis().Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return &RedisStore{MemoryStore: memory, cache: cache}, nil
}

// CreateOrder 创建订单，同时写入 Redis 和内存
// ctx: 上下文
// order: 订单实体
// 返回错误表示创建失败
func (s *RedisStore) CreateOrder(ctx context.Context, order entity.Order) error {
	// 先写入内存存储
	if err := s.MemoryStore.CreateOrder(ctx, order); err != nil {
		return fmt.Errorf("create order in memory: %w", err)
	}
	// 序列化订单（通过DTO）
	dto := application.ToOrderDTO(order)
	data, err := json.Marshal(dto)
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}
	// 写入 Redis
	return fmt.Errorf("redis hset: %w", s.cache.Redis().HSet(ctx, redisOrdersKey(), order.OrderNo, data).Err())
}

// GetOrder 获取订单，优先从 Redis 读取
// ctx: 上下文
// orderNo: 订单号
// 返回订单实体和错误
func (s *RedisStore) GetOrder(ctx context.Context, orderNo string) (entity.Order, error) {
	// 从 Redis 获取
	data, err := s.cache.Redis().HGet(ctx, redisOrdersKey(), orderNo).Bytes()
	if errors.Is(err, goredis.Nil) {
		// Redis 未命中，从内存存储获取
		return s.MemoryStore.GetOrder(ctx, orderNo)
	}
	if err != nil {
		return entity.Order{}, fmt.Errorf("redis hget: %w", err)
	}
	// 反序列化订单（通过DTO）
	var dto application.OrderDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		return entity.Order{}, fmt.Errorf("json unmarshal: %w", err)
	}
	return application.ToOrder(dto), nil
}

// ListOrdersByActivity 根据活动编号列出订单
// ctx: 上下文
// activityNo: 活动编号
// 返回订单列表和错误
func (s *RedisStore) ListOrdersByActivity(ctx context.Context, activityNo string) ([]entity.Order, error) {
	// 从 Redis 获取所有订单
	all, err := s.listAllFromRedis(ctx)
	if err != nil {
		// Redis 失败，降级到内存存储
		return s.MemoryStore.ListOrdersByActivity(ctx, activityNo)
	}
	// 按活动编号过滤
	orders := make([]entity.Order, 0)
	for _, order := range all {
		if order.ActivityNo == activityNo {
			orders = append(orders, order)
		}
	}
	// 按创建时间降序排序
	sort.Slice(orders, func(i, j int) bool {
		if orders[i].CreatedAt.Equal(orders[j].CreatedAt) {
			return orders[i].OrderNo > orders[j].OrderNo
		}
		return orders[i].CreatedAt.After(orders[j].CreatedAt)
	})
	return orders, nil
}

// ListOrdersByActivities 根据多个活动编号批量列出订单
// ctx: 上下文
// activityNos: 活动编号列表
// 返回按活动编号分组的订单映射和错误
func (s *RedisStore) ListOrdersByActivities(ctx context.Context, activityNos []string) (map[string][]entity.Order, error) {
	// 从 Redis 获取所有订单
	all, err := s.listAllFromRedis(ctx)
	if err != nil {
		// Redis 失败，降级到内存存储
		return s.MemoryStore.ListOrdersByActivities(ctx, activityNos)
	}
	// 构建需要查询的活动编号集合
	wanted := make(map[string]struct{}, len(activityNos))
	for _, a := range activityNos {
		if a != "" {
			wanted[a] = struct{}{}
		}
	}
	// 初始化结果映射
	result := make(map[string][]entity.Order, len(wanted))
	for a := range wanted {
		result[a] = nil
	}
	// 按活动编号分组
	for _, order := range all {
		if _, ok := wanted[order.ActivityNo]; ok {
			result[order.ActivityNo] = append(result[order.ActivityNo], order)
		}
	}
	// 对每个活动的订单按创建时间降序排序
	for a := range result {
		sort.Slice(result[a], func(i, j int) bool {
			if result[a][i].CreatedAt.Equal(result[a][j].CreatedAt) {
				return result[a][i].OrderNo > result[a][j].OrderNo
			}
			return result[a][i].CreatedAt.After(result[a][j].CreatedAt)
		})
	}
	return result, nil
}

// ListOrdersByUser 根据用户ID列出订单
// ctx: 上下文
// userID: 用户ID
// 返回订单列表和错误
func (s *RedisStore) ListOrdersByUser(ctx context.Context, userID int64) ([]entity.Order, error) {
	// 从 Redis 获取所有订单
	all, err := s.listAllFromRedis(ctx)
	if err != nil {
		// Redis 失败，降级到内存存储
		return s.MemoryStore.ListOrdersByUser(ctx, userID)
	}
	// 按用户ID过滤
	orders := make([]entity.Order, 0)
	for _, order := range all {
		if order.UserID == userID {
			orders = append(orders, order)
		}
	}
	// 按创建时间降序排序
	sort.Slice(orders, func(i, j int) bool {
		if orders[i].CreatedAt.Equal(orders[j].CreatedAt) {
			return orders[i].OrderNo > orders[j].OrderNo
		}
		return orders[i].CreatedAt.After(orders[j].CreatedAt)
	})
	return orders, nil
}

// MarkOrderPaid 标记订单为已支付
// ctx: 上下文
// orderNo: 订单号
// transactionNo: 交易流水号
// paidAt: 支付时间
// 返回错误表示标记失败
func (s *RedisStore) MarkOrderPaid(ctx context.Context, orderNo string, transactionNo string, paidAt time.Time) error {
	// 先更新内存存储
	if err := s.MemoryStore.MarkOrderPaid(ctx, orderNo, transactionNo, paidAt); err != nil {
		return fmt.Errorf("mark order paid in memory: %w", err)
	}
	// 获取更新后的订单
	order, err := s.MemoryStore.GetOrder(ctx, orderNo)
	if err != nil {
		return fmt.Errorf("get order from memory: %w", err)
	}
	// 同步到 Redis
	return s.saveToRedis(ctx, order)
}

// CloseOrder 关闭订单
// ctx: 上下文
// orderNo: 订单号
// 返回错误表示关闭失败
func (s *RedisStore) CloseOrder(ctx context.Context, orderNo string) error {
	// 先更新内存存储
	if err := s.MemoryStore.CloseOrder(ctx, orderNo); err != nil {
		return fmt.Errorf("close order in memory: %w", err)
	}
	// 获取更新后的订单
	order, err := s.MemoryStore.GetOrder(ctx, orderNo)
	if err != nil {
		return fmt.Errorf("get order from memory: %w", err)
	}
	// 同步到 Redis
	return s.saveToRedis(ctx, order)
}

// listAllFromRedis 从 Redis 获取所有订单
// ctx: 上下文
// 返回订单列表和错误
func (s *RedisStore) listAllFromRedis(ctx context.Context) ([]entity.Order, error) {
	data, err := s.cache.Redis().HGetAll(ctx, redisOrdersKey()).Result()
	if err != nil {
		return nil, fmt.Errorf("redis hgetall: %w", err)
	}
	// 反序列化所有订单（通过DTO）
	orders := make([]entity.Order, 0, len(data))
	for _, raw := range data {
		var dto application.OrderDTO
		if err := json.Unmarshal([]byte(raw), &dto); err != nil {
			continue
		}
		orders = append(orders, application.ToOrder(dto))
	}
	return orders, nil
}

// saveToRedis 保存订单到 Redis
// ctx: 上下文
// order: 订单实体
// 返回错误表示保存失败
func (s *RedisStore) saveToRedis(ctx context.Context, order entity.Order) error {
	dto := application.ToOrderDTO(order)
	data, err := json.Marshal(dto)
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}
	return fmt.Errorf("redis hset: %w", s.cache.Redis().HSet(ctx, redisOrdersKey(), order.OrderNo, data).Err())
}

// redisOrdersKey 返回 Redis 中存储订单的键名
func redisOrdersKey() string {
	return "seckill:orders"
}
