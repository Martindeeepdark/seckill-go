package model

import "time"

// PaymentTimeoutTask 支付超时任务
// 用于延迟检查订单是否超时未支付
type PaymentTimeoutTask struct {
	OrderNo        string    `json:"orderNo"`                  // 订单号
	RequestTraceID string    `json:"requestTraceId,omitempty"` // 请求追踪 ID
	DueAt          time.Time `json:"dueAt"`                    // 到期时间
}

// IsOverdue 检查支付超时是否已到。
func (t *PaymentTimeoutTask) IsOverdue(now time.Time) bool {
	return now.After(t.DueAt)
}

// PostPayTask 支付后任务
// 包括订单同步和自由卡发放
type PostPayTask struct {
	Type           string            `json:"type"`                     // 任务类型：SYNC_ORDER 或 ISSUE_CARD
	OrderNo        string            `json:"orderNo"`                  // 订单号
	RequestTraceID string            `json:"requestTraceId,omitempty"` // 请求追踪 ID
	SyncOrder      *SyncOrderPayload `json:"syncOrder,omitempty"`      // 订单同步负载
	IssueCard      *IssueCardPayload `json:"issueCard,omitempty"`      // 自由卡发放负载
}

// IsSyncOrder 检查是否为同步订单任务。
func (t *PostPayTask) IsSyncOrder() bool {
	return t.Type == "SYNC_ORDER"
}

// IsIssueCard 检查是否为自由卡发放任务。
func (t *PostPayTask) IsIssueCard() bool {
	return t.Type == "ISSUE_CARD"
}

// SyncOrderPayload 订单同步负载
// 用于将订单信息同步到外部系统
type SyncOrderPayload struct {
	OrderNo        string    `json:"orderNo"`                  // 订单号
	UserID         int64     `json:"userId"`                   // 用户 ID
	OrderSource    string    `json:"orderSource"`              // 订单来源
	TotalAmount    int64     `json:"totalAmount"`              // 订单总金额
	DiscountAmount int64     `json:"discountAmount,omitempty"` // 优惠金额
	PayAmount      int64     `json:"payAmount"`                // 实付金额
	PaidAt         time.Time `json:"paidAt"`                   // 支付时间
	TransactionNo  string    `json:"transactionNo"`            // 交易流水号
}

// IssueCardPayload 自由卡发放负载
type IssueCardPayload struct {
	UserID    int64  `json:"userId"`    // 用户 ID
	OrderNo   string `json:"orderNo"`   // 订单号
	CardName  string `json:"cardName"`  // 卡名称
	FaceValue int64  `json:"faceValue"` // 卡面值（分）
	ValidDays int64  `json:"validDays"` // 有效天数
}
