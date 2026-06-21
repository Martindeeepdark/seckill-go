// Package entity 定义支持服务的领域实体
package entity

import "time"

// PostPayTaskType 支付后任务类型
type PostPayTaskType string

const (
	PostPayTaskSyncOrder PostPayTaskType = "SYNC_ORDER" // 订单同步任务
	PostPayTaskIssueCard PostPayTaskType = "ISSUE_CARD" // 发卡任务
)

// PostPayTask 支付后任务
type PostPayTask struct {
	ID             string            // 任务ID
	Type           PostPayTaskType   // 任务类型
	OrderNo        string            // 订单号
	RequestTraceID string            // 请求追踪ID
	SyncOrder      *SyncOrderRequest // 订单同步请求
	IssueCard      *IssueCardRequest // 发卡请求
	Attempts       int64             // 重试次数
	LastError      string            // 最后一次错误信息
	CreatedAt      time.Time         // 创建时间
	UpdatedAt      *time.Time        // 更新时间
}
