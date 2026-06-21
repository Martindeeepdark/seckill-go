// Package entity 定义风控领域实体
package entity

import "time"

const (
	RiskActionSeckill          = "SECKILL"            // 秒杀操作
	RiskActionPreCheck         = "PRE_CHECK"          // 活动前检查
	RiskActionMachineCheckFail = "MACHINE_CHECK_FAIL" // 机器验证失败
	RiskActionRateLimitHit     = "RATE_LIMIT_HIT"     // 触发限流
	RiskActionRepeatSubmit     = "REPEAT_SUBMIT"      // 重复提交

	RiskLevelNormal     = 0 // 正常
	RiskLevelSuspicious = 1 // 可疑
	RiskLevelHigh       = 2 // 高风险
)

// RiskRecord 风控记录
type RiskRecord struct {
	UserID      int64     // 用户ID
	ActionType  string    // 操作类型
	RiskLevel   int64     // 风险等级
	RequestIP   string    // 请求IP
	RequestInfo string    // 请求信息
	CreatedAt   time.Time // 创建时间
}

// RiskEvaluation 风险评估结果
type RiskEvaluation struct {
	Risk   bool   // 是否存在风险
	Level  int64  // 风险等级
	Reason string // 风险原因
}
