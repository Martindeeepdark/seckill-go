package server

import (
	"context"
	"hash/fnv"
	"sync"
	"time"
)

const shardCount = 64

// ShardedRateLimitStore 分片限流存储，使用 FNV-1a hash 将 key 分配到 64 个 shard 以减少锁竞争
type ShardedRateLimitStore struct {
	shards [shardCount]rateLimitShard
	now    func() time.Time
}

// rateLimitShard 单个分片，包含独立的互斥锁和令牌桶映射
type rateLimitShard struct {
	mu       sync.Mutex
	limiters map[string]*rateLimiter
}

// NewShardedRateLimitStore 创建分片限流存储
func NewShardedRateLimitStore(now func() time.Time) *ShardedRateLimitStore {
	if now == nil {
		now = time.Now
	}
	s := &ShardedRateLimitStore{now: now}
	for i := range s.shards {
		s.shards[i].limiters = make(map[string]*rateLimiter)
	}
	return s
}

// getShard 根据 key 的 FNV-1a hash 值返回对应的分片
func (s *ShardedRateLimitStore) getShard(key string) *rateLimitShard {
	h := fnv.New32a()
	h.Write([]byte(key))
	return &s.shards[h.Sum32()%shardCount]
}

// Allow 检查是否允许请求（RateLimitStore 接口实现）
func (s *ShardedRateLimitStore) Allow(_ context.Context, key string, limit int, interval time.Duration) (bool, error) {
	shard := s.getShard(key)
	shard.mu.Lock()
	limiter, ok := shard.limiters[key]
	if !ok || !limiter.sameLimit(limit, interval) {
		limiter = newRateLimiterForInterval(limit, interval, s.now)
		shard.limiters[key] = limiter
	}
	shard.mu.Unlock()
	return limiter.Allow(), nil
}
