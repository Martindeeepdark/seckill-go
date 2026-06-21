// Package application 提供 gateway 的应用层服务
// 包含秒杀、支付、管理等业务逻辑处理
package application

import (
	"context"
	"errors"
	"strings"
	"time"
)

const (
	RiskActionSeckill = "SECKILL" // 风险操作类型：秒杀
	RiskLevelNormal   = 0         // 正常用户风险等级
)

// ActivityGateway 从活动服务查询和管理活动数据
type ActivityGateway interface {
	ListActivities(ctx context.Context) (ActivityList, error)
	GetActivity(ctx context.Context, activityNo string) (*ActivityDetail, error)
	// 管理方法
	CreateActivity(ctx context.Context, req CreateActivityRequest) (*ActivityDetail, error)
	UpdateActivity(ctx context.Context, req UpdateActivityRequest) error
	EndActivity(ctx context.Context, activityNo string) error
	AddProduct(ctx context.Context, req AddProductRequest) error
	RemoveProduct(ctx context.Context, activityNo, skuNo string) error
}

// StockGateway 从库存服务查询和操作库存数据
type StockGateway interface {
	Peek(ctx context.Context, activityNo, skuNo string) (int64, error)
	Deduct(ctx context.Context, req DeductRequest) (bool, error)
	Release(ctx context.Context, activityNo, skuNo, userID string, quantity int) error
}

// RiskGateway 从风险服务评估风险
type RiskGateway interface {
	Evaluate(ctx context.Context, userID int64, requestIP string) (*RiskEvaluation, error)
	IsRiskUser(ctx context.Context, userID int64) (bool, error)
	MarkSuspicious(ctx context.Context, activity *ActivityDetail, userID int64, requestIP string) error
	RecordAction(ctx context.Context, record RiskRecord) error
}

// OrderGateway 通过订单服务管理订单
type OrderGateway interface {
	GetOrder(ctx context.Context, orderNo string) (*OrderDetail, error)
	ListOrdersByUser(ctx context.Context, userID int64) ([]OrderDetail, error)
	ListOrdersByActivity(ctx context.Context, activityNo string) ([]OrderDetail, error)
	CreateOrder(ctx context.Context, req CreateOrderRequest) error
	MarkPaid(ctx context.Context, orderNo, transactionNo string, paidAt time.Time) error
	CloseOrder(ctx context.Context, orderNo string) error
}

// PaymentGateway 通过支持服务管理支付
type PaymentGateway interface {
	Prepay(ctx context.Context, req PrepayPaymentRequest) (*PayResult, error)
	Notify(ctx context.Context, req PaymentNotifyRequest) error
	QueryPayment(ctx context.Context, orderNo string) (*PayQueryResult, error)
	ClosePayment(ctx context.Context, orderNo string) error
}

// QueuePublisher 发布秒杀参与事件
type QueuePublisher interface {
	Publish(ctx context.Context, event PartInEvent) error
}

// PostPayTaskPublisher 发布支付后订单补偿任务
type PostPayTaskPublisher interface {
	PublishPostPayTask(ctx context.Context, task PostPayTask) error
}

// ResultStore 存储异步秒杀结果供客户端轮询
type ResultStore interface {
	SetProcessing(ctx context.Context, requestID string) error
	SetSuccess(ctx context.Context, requestID, orderNo string) error
	SetFailed(ctx context.Context, requestID, errMsg string) error
	Get(ctx context.Context, requestID string) (*SeckillResult, error)
}

// UserRateLimiter 按用户维度限制秒杀入口请求频率。
type UserRateLimiter interface {
	TryAcquire(ctx context.Context, userID int64, rate int, interval time.Duration, ttl time.Duration) (bool, error)
}

// PipelineAcquirer 合并用户限流和 SetProcessing 为一次 Redis Pipeline 调用，
// 将 fast path 的 2 RT 降为 1 RT。
type PipelineAcquirer interface {
	// TryAcquireAndSetProcessing 通过 Pipeline 合并执行限流检查和 SetProcessing。
	// 如果限流拒绝（allowed=false），resultKey 不会被设置。
	// 如果限流允许（allowed=true），同时设置 resultKey 为 PROCESSING。
	TryAcquireAndSetProcessing(ctx context.Context, userID int64, rate int, interval, ttl time.Duration, resultKey string) (bool, error)
}

// QueueStateStore 保存 Java 兼容的用户排队状态。
type QueueStateStore interface {
	SetQueued(ctx context.Context, userID int64, activityNo string, traceID string, ttl time.Duration) error
	GetQueuedTrace(ctx context.Context, userID int64, activityNo string) (string, error)
}

// SeckillResult 表示异步秒杀请求的处理状态
type SeckillResult struct {
	Status  string `json:"status"` // 状态：processing/success/failed
	OrderNo string `json:"orderNo,omitempty"`
	Error   string `json:"error,omitempty"`
}

// MachineChecker 验证请求是否来自真实用户
type MachineChecker interface {
	Challenge(ctx context.Context, userID int64) (MachineChallenge, error)
	Check(ctx context.Context, userID int64, token string) bool
}

// MachineChallenge 是 Java 秒杀服务 pre-check 返回的机审挑战。
type MachineChallenge struct {
	Result string `json:"result"`
	Key    int64  `json:"key"`
}

// ========== 数据传输对象 ==========

// ActivityList 活动列表
type ActivityList struct {
	Activities []ActivityItem `json:"activities"`
}

// ActivityItem 活动项
type ActivityItem struct {
	ActivityNo     string `json:"activityNo"`
	ActivityName   string `json:"activityName"`
	StartTime      string `json:"startTime"`
	EndTime        string `json:"endTime"`
	ActivityStatus int    `json:"activityStatus"`
}

// ActivityDetail 活动详情
type ActivityDetail struct {
	ActivityNo     string          `json:"activityNo"`
	ActivityName   string          `json:"activityName"`
	StartTime      string          `json:"startTime"`
	EndTime        string          `json:"endTime"`
	ActivityStatus int             `json:"activityStatus"`
	PurchaseLimit  int             `json:"purchaseLimit"` // 每人限购数量
	ActivityOpen   bool            `json:"activityOpen"`  // 活动是否开放
	Products       []ProductDetail `json:"products"`
}

// ProductDetail 产品详情
type ProductDetail struct {
	SKUNo           string `json:"skuNo"`
	ProductName     string `json:"productName"`
	ProductImage    string `json:"productImage"`
	OriginalPrice   int64  `json:"originalPrice"`   // 原价
	SeckillPrice    int64  `json:"seckillPrice"`    // 秒杀价
	ActivityStock   int64  `json:"activityStock"`   // 活动库存
	DiscountType    int    `json:"discountType"`    // 折扣类型
	DiscountPrice   int64  `json:"discountPrice"`   // 折扣金额
	DiscountPercent int64  `json:"discountPercent"` // 折扣百分比
	SortOrder       int    `json:"sortOrder"`
}

// DeductRequest 扣库存请求
type DeductRequest struct {
	ActivityNo    string
	SkuNo         string
	UserID        int64
	Quantity      int
	PurchaseLimit int
}

// RiskEvaluation 风险评估结果
type RiskEvaluation struct {
	Risk   bool   `json:"risk"`   // 是否有风险
	Level  int    `json:"level"`  // 风险等级
	Reason string `json:"reason"` // 风险原因
}

// RiskRecord 风险记录
type RiskRecord struct {
	UserID      int64
	ActionType  string
	RiskLevel   int64
	RequestIP   string
	RequestInfo string
	CreatedAt   time.Time
}

// OrderDetail 订单详情
type OrderDetail struct {
	OrderNo        string `json:"orderNo"`
	UserID         int64  `json:"userId"`
	ActivityNo     string `json:"activityNo"`
	SKUNo          string `json:"skuNo"`
	Quantity       int    `json:"quantity"`
	PayAmount      int64  `json:"payAmount"`
	Status         string `json:"status"`
	TraceID        string `json:"traceId"`
	RequestTraceID string `json:"requestTraceId"`
	TransactionNo  string `json:"transactionNo,omitempty"`
	PaidAt         string `json:"paidAt,omitempty"`
	ClosedAt       string `json:"closedAt,omitempty"`
	CreatedAt      string `json:"createdAt,omitempty"`
}

// CreateOrderRequest 创建订单请求
type CreateOrderRequest struct {
	OrderNo        string
	UserID         int64
	ActivityNo     string
	SKUNo          string
	Quantity       int
	PayAmount      int64
	Status         string
	TraceID        string
	RequestTraceID string
}

// CreatePaymentRequest 创建支付请求
type CreatePaymentRequest struct {
	OrderNo    string
	UserID     int64
	PayAmount  int64
	PayChannel string
	Subject    string
}

// PrepayPaymentRequest 预支付请求
type PrepayPaymentRequest struct {
	OrderNo    string
	UserID     int64
	PayChannel string
}

// PaymentNotifyRequest 支付通知请求
type PaymentNotifyRequest struct {
	Channel       string
	OrderNo       string
	TransactionNo string
	Params        map[string]string
}

// PayResult 支付结果
type PayResult struct {
	OrderNo    string `json:"orderNo"`
	PayChannel string `json:"payChannel"`
	PrepayID   string `json:"prepayId"`
	NonceStr   string `json:"nonceStr"`
	TimeStamp  string `json:"timeStamp"`
	Sign       string `json:"sign"`
}

// PayQueryResult 支付查询结果
type PayQueryResult struct {
	OrderNo       string `json:"orderNo"`
	PayStatus     int32  `json:"payStatus"`
	TransactionNo string `json:"transactionNo,omitempty"`
	PaidAt        string `json:"paidAt,omitempty"`
}

// PostPayTask 支付后任务
type PostPayTask struct {
	Type           string            `json:"type"`
	OrderNo        string            `json:"orderNo"`
	RequestTraceID string            `json:"requestTraceId,omitempty"`
	SyncOrder      *SyncOrderPayload `json:"syncOrder,omitempty"`
	IssueCard      *IssueCardPayload `json:"issueCard,omitempty"`
}

// SyncOrderPayload 同步订单负载
type SyncOrderPayload struct {
	OrderNo        string    `json:"orderNo"`
	UserID         int64     `json:"userId"`
	OrderSource    string    `json:"orderSource"`
	TotalAmount    int64     `json:"totalAmount"`
	DiscountAmount int64     `json:"discountAmount"`
	PayAmount      int64     `json:"payAmount"`
	PaidAt         time.Time `json:"paidAt"`
	TransactionNo  string    `json:"transactionNo"`
}

// IssueCardPayload 发券负载
type IssueCardPayload struct {
	UserID    int64  `json:"userId"`
	OrderNo   string `json:"orderNo"`
	CardName  string `json:"cardName"`
	FaceValue int64  `json:"faceValue"`
	ValidDays int    `json:"validDays"`
}

// PartInResult 秒杀参与结果
type PartInResult struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Token   string `json:"token,omitempty"`
	SKUNo   string `json:"skuNo,omitempty"`
	Queued  bool   `json:"queued"`
}

// QueueCheckResult 队列检查结果
type QueueCheckResult struct {
	PollStatus string `json:"pollStatus"` // 0=停止 1=继续 2=成功
	OrderNo    string `json:"orderNo,omitempty"`
	Reason     string `json:"reason,omitempty"`
	TraceID    string `json:"traceId,omitempty"`
}

// PartInEvent 秒杀参与事件
type PartInEvent struct {
	UserID      int64  `json:"userId"`
	ActivityNo  string `json:"activityNo"`
	SkuNo       string `json:"skuNo"`
	Quantity    int    `json:"quantity"`
	TotalFee    int64  `json:"totalFee"`
	RequestIP   string `json:"requestIp"`
	TraceID     string `json:"traceId"`
	RunID       string `json:"runId,omitempty"` // smoke 压测 run-id，缺失时为空
	MachinePass bool   `json:"machinePass"`
}

// CreateActivityRequest 创建活动请求
type CreateActivityRequest struct {
	ActivityName  string `json:"activityName"`
	StartTime     string `json:"startTime"`
	EndTime       string `json:"endTime"`
	PurchaseLimit int    `json:"purchaseLimit"`
	Remark        string `json:"remark"`
}

// Validate 校验创建活动请求的活动名称和起止时间是否为空。
func (r CreateActivityRequest) Validate() error {
	if strings.TrimSpace(r.ActivityName) == "" {
		return errors.New("activityName is required")
	}
	if strings.TrimSpace(r.StartTime) == "" {
		return errors.New("startTime is required")
	}
	if strings.TrimSpace(r.EndTime) == "" {
		return errors.New("endTime is required")
	}
	return nil
}

// UpdateActivityRequest 更新活动请求
type UpdateActivityRequest struct {
	ActivityNo    string `json:"activityNo"`
	ActivityName  string `json:"activityName"`
	StartTime     string `json:"startTime"`
	EndTime       string `json:"endTime"`
	PurchaseLimit int    `json:"purchaseLimit"`
	Remark        string `json:"remark"`
}

// AddProductRequest 添加商品请求
type AddProductRequest struct {
	ActivityNo      string `json:"activityNo"`
	SKUNo           string `json:"skuNo"`
	ProductName     string `json:"productName"`
	ProductImage    string `json:"productImage"`
	OriginalPrice   int64  `json:"originalPrice"`
	SeckillPrice    int64  `json:"seckillPrice"`
	ActivityStock   int64  `json:"activityStock"`
	LimitQuantity   int64  `json:"limitQuantity"`
	DiscountType    int    `json:"discountType"`
	DiscountPrice   int64  `json:"discountPrice"`
	DiscountPercent int64  `json:"discountPercent"`
}

// Validate 校验添加商品请求的 SKU 编号和活动库存是否合法。
func (r AddProductRequest) Validate() error {
	if strings.TrimSpace(r.SKUNo) == "" {
		return errors.New("skuNo is required")
	}
	if r.ActivityStock <= 0 {
		return errors.New("activityStock must be positive")
	}
	return nil
}
