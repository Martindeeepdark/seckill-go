package model

// RiskResult 风控评估结果
type RiskResult struct {
	Risk   bool   // 是否存在风险
	Level  int64  // 风险等级
	Reason string // 风险原因
}
