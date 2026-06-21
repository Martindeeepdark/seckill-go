// Package identity 提供身份相关的基础设施实现
// 包括基于 Snowflake 的订单号生成器和 gRPC 临时错误判断器
package identity

import (
	"fmt"

	"github.com/Martindeeepdark/go-common/snowflake"

	"seckill-processor-service/internal/domain/service"
)

var _ service.IDGenerator = (*SnowflakeIDGenerator)(nil)

// SnowflakeIDGenerator 基于 snowflake 的订单号生成器
type SnowflakeIDGenerator struct{}

// NextOrderNo 生成下一个订单号
// 格式：O + Snowflake ID
// 不包含分片标识，因为分区键是 user_id，订单号无法预知属于哪个分区
func (SnowflakeIDGenerator) NextOrderNo() string { return fmt.Sprintf("O%d", snowflake.NewID()) }

// SnowflakeIDGeneratorWithShard 带分片标识的订单号生成器
// 订单号格式：O + Snowflake ID + 分片标识(user_id % 4)
// 优点：按订单号查询时可以直接路由到目标分区，避免扫描所有分区
type SnowflakeIDGeneratorWithShard struct {
	userID int64
}

// NewSnowflakeIDGeneratorWithShard 创建带分片标识的订单号生成器
func NewSnowflakeIDGeneratorWithShard(userID int64) *SnowflakeIDGeneratorWithShard {
	return &SnowflakeIDGeneratorWithShard{userID: userID}
}

// NextOrderNo 生成带分片标识的订单号
// 格式：O{snowflakeID}{shardID}
// 示例：O1308203777303511180 (最后一位 0 是分片标识，表示 user_id % 4 = 0)
func (g *SnowflakeIDGeneratorWithShard) NextOrderNo() string {
	shardID := g.userID % 4
	return fmt.Sprintf("O%d%d", snowflake.NewID(), shardID)
}
