// Package persistence 提供风险控制的持久化层实现
package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	commonredis "github.com/Martindeeepdark/go-common/cache/redis"
	goredis "github.com/redis/go-redis/v9"

	"seckill-risk-service/internal/application"
	"seckill-risk-service/internal/domain/entity"
)

// RedisStore Redis 存储实现，支持持久化和缓存
type RedisStore struct {
	*MemoryStore                     // 嵌入内存存储，用于本地缓存
	cache        *commonredis.Client // Redis 客户端
}

// NewRedisStore 创建 Redis 存储
func NewRedisStore(ctx context.Context, addr, password string, db int, memory *MemoryStore) (*RedisStore, error) {
	cache := commonredis.New(addr, commonredis.WithPassword(password), commonredis.WithDB(db))
	if err := cache.Redis().Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return &RedisStore{MemoryStore: memory, cache: cache}, nil
}

// MarkRiskUser 标记用户为风险用户，ttl 指定过期时长
// 同时更新内存存储和 Redis，确保数据一致性
func (s *RedisStore) MarkRiskUser(ctx context.Context, userID int64, ttl time.Duration) error {
	if err := s.MemoryStore.MarkRiskUser(ctx, userID, ttl); err != nil {
		return fmt.Errorf("mark risk user in memory: %w", err)
	}
	return fmt.Errorf("redis set: %w", s.cache.Redis().Set(ctx, redisRiskUserKey(userID), "blacklist", ttl).Err())
}

// IsRiskUser 检查用户是否为风险用户
// 直接从 Redis 查询，利用 Redis 的过期机制自动清理
func (s *RedisStore) IsRiskUser(ctx context.Context, userID int64) (bool, error) {
	count, err := s.cache.Redis().Exists(ctx, redisRiskUserKey(userID)).Result()
	if err != nil {
		return false, fmt.Errorf("redis exists: %w", err)
	}
	return count > 0, nil
}

// CleanupExpiredRiskUsers 清理过期的风险用户，返回清理数量
// 通过扫描 Redis 键并检查 TTL 来清理已过期但未被 Redis 自动删除的键
func (s *RedisStore) CleanupExpiredRiskUsers(ctx context.Context) (int, error) {
	_, _ = s.MemoryStore.CleanupExpiredRiskUsers(ctx) //nolint:errcheck // 内存存储清理尽力而为
	return s.deleteExpiredKeysByPattern(ctx, "seckill:risk:user:*")
}

// RecordRiskAction 记录风险行为到 Redis 有序集合中
// 同时更新内存存储，使用时间戳作为分数以便按时间排序和范围查询
func (s *RedisStore) RecordRiskAction(ctx context.Context, record entity.RiskRecord) error {
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now()
	}
	if err := s.MemoryStore.RecordRiskAction(ctx, record); err != nil {
		return fmt.Errorf("record risk action in memory: %w", err)
	}
	// 序列化为 JSON 以存储到 Redis（通过 DTO）
	dto := application.ToRiskRecordDTO(record)
	payload, err := json.Marshal(dto)
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}
	key := redisRiskRecordsKey(record.UserID)
	score := float64(record.CreatedAt.UnixMilli()) // 使用时间戳毫秒数作为分数
	if err := s.cache.Redis().ZAdd(ctx, key, goredis.Z{Score: score, Member: string(payload)}).Err(); err != nil {
		return fmt.Errorf("redis zadd: %w", err)
	}
	// 清理 48 小时前的旧记录并设置过期时间
	cutoff := time.Now().Add(-48 * time.Hour).UnixMilli()
	pipe := s.cache.Redis().Pipeline()
	pipe.ZRemRangeByScore(ctx, key, "-inf", strconv.FormatInt(cutoff, 10))
	pipe.Expire(ctx, key, 48*time.Hour)
	_, err = pipe.Exec(ctx)
	return fmt.Errorf("redis pipeline exec: %w", err)
}

// CountRecentRiskActions 统计指定时间以来特定类型的风险行为数量
// 从 Redis 有序集合中获取指定时间范围内的记录并过滤类型
func (s *RedisStore) CountRecentRiskActions(ctx context.Context, userID int64, actionType string, since time.Time) (int, error) {
	records, err := s.riskRecordsSince(ctx, userID, since)
	if err != nil {
		return 0, fmt.Errorf("risk records since: %w", err)
	}
	count := 0
	for _, record := range records {
		if record.ActionType == actionType {
			count++
		}
	}
	return count, nil
}

// HasHighRiskRecord 检查指定时间以来是否存在高风险记录
// 风险等级 >= RiskLevelHigh 视为高风险
func (s *RedisStore) HasHighRiskRecord(ctx context.Context, userID int64, since time.Time) (bool, error) {
	records, err := s.riskRecordsSince(ctx, userID, since)
	if err != nil {
		return false, fmt.Errorf("risk records since: %w", err)
	}
	for _, record := range records {
		if record.RiskLevel >= entity.RiskLevelHigh {
			return true, nil
		}
	}
	return false, nil
}

// ListRiskRecords 列出用户的所有风险行为记录，按时间倒序排列
// 从 Redis 有序集合中获取并解码为实体对象
func (s *RedisStore) ListRiskRecords(ctx context.Context, userID int64) ([]entity.RiskRecord, error) {
	values, err := s.cache.Redis().ZRangeArgs(ctx, goredis.ZRangeArgs{
		Key:   redisRiskRecordsKey(userID),
		Start: 0,
		Stop:  -1,
		Rev:   true,
	}).Result()
	if errors.Is(err, goredis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redis zrange: %w", err)
	}
	return decodeRiskRecords(values), nil
}

// riskRecordsSince 获取指定时间以来的风险记录
// 使用有序集合的范围查询功能按分数（时间戳）筛选
func (s *RedisStore) riskRecordsSince(ctx context.Context, userID int64, since time.Time) ([]entity.RiskRecord, error) {
	values, err := s.cache.Redis().ZRangeArgs(ctx, goredis.ZRangeArgs{
		Key:     redisRiskRecordsKey(userID),
		Start:   strconv.FormatInt(since.UnixMilli(), 10),
		Stop:    "+inf",
		ByScore: true,
	}).Result()
	if errors.Is(err, goredis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redis zrange: %w", err)
	}
	return decodeRiskRecords(values), nil
}

// deleteExpiredKeysByPattern 按模式扫描并删除已过期的键，返回删除数量
// 使用 SCAN 命令遍历匹配的键，检查 TTL 并删除已过期的键
func (s *RedisStore) deleteExpiredKeysByPattern(ctx context.Context, pattern string) (int, error) {
	var cursor uint64
	deleted := 0
	for {
		keys, next, err := s.cache.Redis().Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return deleted, fmt.Errorf("redis scan: %w", err)
		}
		for _, key := range keys {
			ttl, err := s.cache.Redis().TTL(ctx, key).Result()
			if err != nil {
				return deleted, fmt.Errorf("redis ttl: %w", err)
			}
			if ttl > 0 {
				continue
			}
			if ttl == -time.Second {
				continue
			}
			count, err := s.cache.Redis().Del(ctx, key).Result()
			if err != nil {
				return deleted, fmt.Errorf("redis ttl: %w", err)
			}
			deleted += int(count)
		}
		cursor = next
		if cursor == 0 {
			return deleted, nil
		}
	}
}

// redisRiskUserKey 生成风险用户的 Redis 键名
// 格式：seckill:risk:user:{userID}
func redisRiskUserKey(userID int64) string {
	return fmt.Sprintf("seckill:risk:user:%d", userID)
}

// redisRiskRecordsKey 生成风险记录的 Redis 键名
// 格式：seckill:risk:record:{userID}，使用有序集合存储
func redisRiskRecordsKey(userID int64) string {
	return fmt.Sprintf("seckill:risk:record:%d", userID)
}

// decodeRiskRecords 将 JSON 字符串数组解码为风险记录实体（通过 DTO）
func decodeRiskRecords(values []string) []entity.RiskRecord {
	records := make([]entity.RiskRecord, 0, len(values))
	for _, value := range values {
		var dto application.RiskRecordDTO
		if err := json.Unmarshal([]byte(value), &dto); err == nil {
			records = append(records, application.ToRiskRecord(dto))
		}
	}
	return records
}
