package risk

import "time"

// BlackListConfig 黑名单配置
type BlackListConfig struct {
	Enabled         bool          // 是否启用黑名单
	MarkStartBefore time.Duration // 活动开始前多久标记可疑用户
	MarkEndBefore   time.Duration // 活动开始前多久停止标记
	ExpireAfter     time.Duration // 标记后过期时长
}

// RiskConfig 风控配置
type RiskConfig struct {
	HighRiskThreshold int           // 高风险阈值（秒杀次数）
	RiskUserTTL       time.Duration // 风险用户TTL
	RecentWindow      time.Duration // 近期统计窗口
	HighRiskWindow    time.Duration // 高风险记录窗口
}
