// Package application 提供 gateway 的应用层服务
// 包含秒杀、支付、管理等业务逻辑处理
package application

import (
	"context"
	"fmt"
	"time"

	commonlogs "github.com/Martindeeepdark/go-common/logs"

	commonerrors "seckill-common/errors"
	"seckill-common/metrics"
	"seckill-common/tracing"
	"seckill-common/traceresult"

	"seckill-gateway-service/internal/config"
)

const (
	pollStop              = "0" // 停止轮询
	pollContinue          = "1" // 继续轮询
	pollSuccess           = "2" // 轮询成功
	seckillSuccessCode    = "0"
	seckillFailCode       = "1"
	seckillStockEmptyCode = "6"
	seckillClosedCode     = "7"
	seckillRiskUserCode   = "8"
	queueStateTTL         = time.Minute      // Java 排队状态 TTL
	userRateLimiterTTL    = 5 * time.Minute  // Java 用户限流器默认 TTL
	defaultUserRate       = 10               // Java 默认每用户请求数
	defaultUserRateWindow = 10 * time.Second // Java 默认用户限流窗口

	seckillSuccessMessage    = "抢购成功，请等待排队结果"
	seckillFailMessage       = "当前请求人数较多，请稍后重试"
	seckillSystemBusyMessage = "系统繁忙，请重试"
	seckillActivityMissing   = "活动不存在"
	seckillClosedMessage     = "活动尚未开始或已结束"
	seckillRiskUserMessage   = "请求异常，请稍后重试"
	seckillStockEmptyMessage = "手慢了，商品已售罄"
)

// SeckillAppOption 配置秒杀应用服务。
type SeckillAppOption func(*SeckillApp)

// DynamicGatewayConfig 提供运行时可热更新的配置读取接口。
type DynamicGatewayConfig interface {
	RiskEnabled() bool
	MachineCheckEnabled() bool
}

// SeckillApp 协调秒杀参与流程
type SeckillApp struct {
	cfg              config.GatewayConfig
	activity         ActivityGateway
	stock            StockGateway
	risk             RiskGateway
	order            OrderGateway
	queue            QueuePublisher
	results          ResultStore
	checker          MachineChecker
	workerPool       *WorkerPool
	userLimiter      UserRateLimiter
	pipelineAcquirer PipelineAcquirer
	queueState       QueueStateStore
	dynamicConfig    DynamicGatewayConfig
}

// NewSeckillApp 创建新的秒杀应用服务
// workerPool 参数可选；如果为 nil，使用旧版同步路径
func NewSeckillApp(
	cfg config.GatewayConfig,
	activity ActivityGateway,
	stock StockGateway,
	risk RiskGateway,
	order OrderGateway,
	queue QueuePublisher,
	results ResultStore,
	checker MachineChecker,
	workerPool *WorkerPool,
	_ any, // logger 参数保留用于签名兼容
	opts ...SeckillAppOption,
) *SeckillApp {
	app := &SeckillApp{
		cfg:        cfg,
		activity:   activity,
		stock:      stock,
		risk:       risk,
		order:      order,
		queue:      queue,
		results:    results,
		checker:    checker,
		workerPool: workerPool,
	}
	for _, opt := range opts {
		opt(app)
	}
	return app
}

// WithUserRateLimiter 配置 Java 兼容的用户级限流器。
func WithUserRateLimiter(limiter UserRateLimiter) SeckillAppOption {
	return func(app *SeckillApp) {
		app.userLimiter = limiter
	}
}

// WithPipelineAcquirer 配置 Redis Pipeline 合并限流+SetProcessing。
func WithPipelineAcquirer(acquirer PipelineAcquirer) SeckillAppOption {
	return func(app *SeckillApp) {
		app.pipelineAcquirer = acquirer
	}
}

// WithQueueStateStore 配置 Java 兼容的排队状态存储。
func WithQueueStateStore(store QueueStateStore) SeckillAppOption {
	return func(app *SeckillApp) {
		app.queueState = store
	}
}

// WithDynamicConfig 配置运行时动态配置源。
func WithDynamicConfig(dc DynamicGatewayConfig) SeckillAppOption {
	return func(app *SeckillApp) {
		app.dynamicConfig = dc
	}
}

// riskEnabled 返回当前风控开关，优先从动态配置读取。
func (a *SeckillApp) riskEnabled() bool {
	if a.dynamicConfig != nil {
		return a.dynamicConfig.RiskEnabled()
	}
	return a.cfg.Risk.Enabled
}

// machineCheckEnabled 返回当前机审开关，优先从动态配置读取。
func (a *SeckillApp) machineCheckEnabled() bool {
	if a.dynamicConfig != nil {
		return a.dynamicConfig.MachineCheckEnabled()
	}
	return a.cfg.MachineCheck.Enabled
}

// Activities 列出所有秒杀活动
func (a *SeckillApp) Activities(ctx context.Context) (ActivityList, error) {
	res, err := a.activity.ListActivities(ctx)
	if err != nil {
		return ActivityList{}, fmt.Errorf("list activities: %w", err)
	}
	return res, nil
}

// ActiveActivities 仅列出活跃（开放）的活动
func (a *SeckillApp) ActiveActivities(ctx context.Context) ([]ActivityItem, error) {
	list, err := a.activity.ListActivities(ctx)
	if err != nil {
		return nil, fmt.Errorf("list activities: %w", err)
	}
	var active []ActivityItem
	for _, item := range list.Activities {
		if item.ActivityStatus == 1 {
			active = append(active, item)
		}
	}
	return active, nil
}

// Activity 返回单个活动详情
func (a *SeckillApp) Activity(ctx context.Context, activityNo string) (*ActivityDetail, error) {
	res, err := a.activity.GetActivity(ctx, activityNo)
	if err != nil {
		return nil, fmt.Errorf("get activity %s: %w", activityNo, err)
	}
	return res, nil
}

// PreCheck 执行秒杀前验证：风险评估 + 活动检查
func (a *SeckillApp) PreCheck(ctx context.Context, userID int64, activityNo, requestIP string) (bool, string) {
	if a.riskEnabled() {
		eval, err := a.risk.Evaluate(ctx, userID, requestIP)
		if err != nil {
			commonlogs.CtxWarnf(ctx, "risk evaluate failed, allowing by default: %v", err)
		} else if eval.Risk {
			return false, "risk_blocked"
		}
	}
	activity, err := a.activity.GetActivity(ctx, activityNo)
	if err != nil {
		return false, "activity_not_found"
	}
	now := time.Now()
	startTime, err := time.Parse(time.RFC3339, activity.StartTime)
	if err != nil {
		return false, "invalid_start_time"
	}
	endTime, err := time.Parse(time.RFC3339, activity.EndTime)
	if err != nil {
		return false, "invalid_end_time"
	}
	if activity.ActivityStatus != 1 || now.Before(startTime) || now.After(endTime) {
		return false, "activity_not_open"
	}
	return true, ""
}

// MachineChallenge 生成 Java 风格机审挑战。
func (a *SeckillApp) MachineChallenge(ctx context.Context, userID int64) (MachineChallenge, error) {
	if a.checker == nil {
		return MachineChallenge{}, nil
	}
	challenge, err := a.checker.Challenge(ctx, userID)
	if err != nil {
		return MachineChallenge{}, fmt.Errorf("machine challenge: %w", err)
	}
	return challenge, nil
}

// PartIn 验证请求并将其入队进行异步处理
//
// 快速路径（当启用 workerPool 时）：
// 1. 验证输入参数（activityNo、skuNo 非空）
// 2. 执行机器检查（轻量级，本地）
// 3. 生成 traceID
// 4. 在结果存储中设置处理中状态
// 5. 调用 workerPool.Submit(ctx, event) - 非阻塞
// 6. 如果提交失败（队列满），返回 HTTP 适当的错误
// 7. 立即返回 PartInResult - 无下游 gRPC 调用
//
// 慢速路径（当 workerPool 为 nil 时 - 向后兼容）：
// 1. 机器检查
// 2. 风险评估
// 3. 活动/SKU/库存预检查
// 4. NATS 移交给处理器
func (a *SeckillApp) PartIn(ctx context.Context, userID int64, activityNo, skuNo, requestIP, machineToken string, quantity int, runID string) (*PartInResult, error) {
	traceID := tracing.TraceID(ctx)
	if traceID == "" {
		traceID = tracing.NewTraceID()
		ctx = tracing.WithTraceID(ctx, traceID)
	}

	// 快速路径：启用工作池
	if a.workerPool != nil {
		return a.partInFastPath(ctx, userID, activityNo, skuNo, requestIP, machineToken, quantity, traceID, runID)
	}

	// 慢速路径：旧版同步处理
	return a.partInSlowPath(ctx, userID, activityNo, skuNo, requestIP, machineToken, quantity, traceID, runID)
}

// partInFastPath 实现使用工作池的快速非阻塞路径
// 此路径零次下游 gRPC 调用 - 仅本地验证 + 入队
func (a *SeckillApp) partInFastPath(ctx context.Context, userID int64, activityNo, skuNo, requestIP, machineToken string, quantity int, traceID, runID string) (*PartInResult, error) {
	// 1. 验证输入参数
	if activityNo == "" {
		return nil, fmt.Errorf("activity_no is required")
	}
	if skuNo == "" {
		return nil, fmt.Errorf("sku_no is required")
	}

	// 2. 机器检查（轻量级，本地）
	if a.machineCheckEnabled() {
		if a.checker == nil || !a.checker.Check(ctx, userID, machineToken) {
			metrics.IncrOther(ctx, runID)
			return seckillRejected(seckillFailCode, seckillFailMessage, skuNo), nil
		}
	}

	// 3. 用户限流 + SetProcessing 合并为 Pipeline（从 2 RT 降到 1 RT）
	if a.pipelineAcquirer != nil && a.cfg.RateLimit.UserEnabled && userID > 0 {
		allowed, err := a.pipelineAcquirer.TryAcquireAndSetProcessing(
			ctx, userID,
			a.userRateOrDefault(),
			a.userIntervalOrDefault(),
			a.userExpireOrDefault(),
			traceresult.Key(traceID),
		)
		if err != nil {
			commonlogs.CtxWarnf(ctx, "pipeline acquire failed, falling back to separate calls: %v", err)
			// Pipeline 失败，回退到分开调用
			if !a.acquireUserRateLimit(ctx, userID) {
				metrics.IncrRateLimit(ctx, runID)
				return seckillRejected(seckillFailCode, seckillFailMessage, skuNo), nil
			}
			if a.results != nil {
				if setErr := a.results.SetProcessing(ctx, traceID); setErr != nil {
					commonlogs.CtxWarnf(ctx, "failed to set processing status: %v", setErr)
				}
			}
		} else if !allowed {
			commonlogs.CtxWarnf(ctx, "user rate limit triggered (pipeline) userId=%d", userID)
			metrics.IncrRateLimit(ctx, runID)
			return seckillRejected(seckillFailCode, seckillFailMessage, skuNo), nil
		}
		// Pipeline 成功且限流允许：SetProcessing 已在 Pipeline 中完成
	} else {
		// 无 Pipeline 或不需要限流，使用原始逻辑
		if !a.acquireUserRateLimit(ctx, userID) {
			return seckillRejected(seckillFailCode, seckillFailMessage, skuNo), nil
		}
		// 4. 在结果存储中设置处理中状态
		if a.results != nil {
			if err := a.results.SetProcessing(ctx, traceID); err != nil {
				commonlogs.CtxWarnf(ctx, "failed to set processing status: %v", err)
			}
		}
	}

	// 5. 创建事件
	event := PartInEvent{
		UserID:      userID,
		ActivityNo:  activityNo,
		SkuNo:       skuNo,
		Quantity:    quantity,
		RequestIP:   requestIP,
		TraceID:     traceID,
		RunID:       runID,
		MachinePass: true,
	}

	// 6. 提交到工作池（非阻塞）
	if err := a.workerPool.Submit(ctx, event); err != nil {
		// 7. 队列满 - 返回 HTTP 适当的错误
		metrics.IncrOther(ctx, runID)
		if a.results != nil {
			if setErr := a.results.SetFailed(ctx, traceID, "system busy: queue full"); setErr != nil {
				commonlogs.CtxWarnf(ctx, "failed to set failed status: %v", setErr)
			}
		}
		return seckillRejected(seckillFailCode, seckillSystemBusyMessage, skuNo), nil
	}
	a.setQueuedTrace(ctx, userID, activityNo, traceID)

	commonlogs.CtxInfof(ctx, "seckill part-in queued to worker pool userId=%d activityNo=%s skuNo=%s",
		userID, activityNo, skuNo)

	// 8. 立即返回 - 无下游 gRPC 调用
	return seckillQueued(traceID, skuNo), nil
}

// partInSlowPath 实现旧版同步处理路径
// 此路径在发布到 NATS 之前进行下游 gRPC 调用
func (a *SeckillApp) partInSlowPath(ctx context.Context, userID int64, activityNo, skuNo, requestIP, machineToken string, quantity int, traceID, runID string) (*PartInResult, error) {
	// 机器检查
	if a.machineCheckEnabled() {
		if a.checker == nil || !a.checker.Check(ctx, userID, machineToken) {
			metrics.IncrOther(ctx, runID)
			return seckillRejected(seckillFailCode, seckillFailMessage, skuNo), nil
		}
	}
	if !a.acquireUserRateLimit(ctx, userID) {
		metrics.IncrRateLimit(ctx, runID)
		return seckillRejected(seckillFailCode, seckillFailMessage, skuNo), nil
	}

	activity, err := a.activity.GetActivity(ctx, activityNo)
	if err != nil {
		if commonerrors.IsRPCNotFoundError(err) {
			return seckillRejected(seckillClosedCode, seckillActivityMissing, skuNo), nil
		}
		return nil, fmt.Errorf("activity not found: %w", err)
	}
	if activity == nil {
		return seckillRejected(seckillClosedCode, seckillActivityMissing, skuNo), nil
	}
	if !activity.ActivityOpen {
		a.markSuspicious(ctx, activity, userID, requestIP)
		return seckillRejected(seckillClosedCode, seckillClosedMessage, skuNo), nil
	}

	// 风险检查。Java 入口在活动开放检查之后执行小黑屋检查。
	if a.riskEnabled() && a.risk != nil {
		isRisk, err := a.risk.IsRiskUser(ctx, userID)
		if err != nil {
			commonlogs.CtxWarnf(ctx, "risk check failed, allowing by default: %v", err)
		} else if isRisk {
			metrics.IncrRisk(ctx, runID)
			return seckillRejected(seckillRiskUserCode, seckillRiskUserMessage, skuNo), nil
		}
	}

	stock, err := a.stock.Peek(ctx, activityNo, skuNo)
	if err != nil {
		if commonerrors.IsRPCNotFoundError(err) || commonerrors.IsRPCFailedPreconditionError(err) {
			metrics.IncrStockEmpty(ctx, runID)
			return seckillRejected(seckillStockEmptyCode, seckillStockEmptyMessage, skuNo), nil
		}
		return nil, fmt.Errorf("stock pre-check: %w", err)
	} else if stock <= 0 {
		metrics.IncrStockEmpty(ctx, runID)
		return seckillRejected(seckillStockEmptyCode, seckillStockEmptyMessage, skuNo), nil
	}

	event := PartInEvent{
		UserID:      userID,
		ActivityNo:  activityNo,
		SkuNo:       skuNo,
		Quantity:    quantity,
		TotalFee:    a.calculatePayAmount(activity, skuNo, quantity),
		RequestIP:   requestIP,
		TraceID:     traceID,
		RunID:       runID,
		MachinePass: true,
	}
	if err := a.queue.Publish(ctx, event); err != nil {
		commonlogs.CtxWarnf(ctx, "publish seckill message failed: %v", err)
		return seckillRejected(seckillFailCode, seckillSystemBusyMessage, skuNo), nil
	}
	a.setQueuedTrace(ctx, userID, activityNo, traceID)
	a.recordSeckillAction(ctx, event)

	commonlogs.CtxInfof(ctx, "seckill part-in queued userId=%d activityNo=%s skuNo=%s",
		userID, activityNo, skuNo)
	return seckillQueued(traceID, skuNo), nil
}

func seckillQueued(traceID, skuNo string) *PartInResult {
	return &PartInResult{
		Code:    seckillSuccessCode,
		Message: seckillSuccessMessage,
		Token:   traceID,
		SKUNo:   skuNo,
		Queued:  true,
	}
}

func seckillRejected(code, message, skuNo string) *PartInResult {
	return &PartInResult{
		Code:    code,
		Message: message,
		SKUNo:   skuNo,
		Queued:  false,
	}
}

// acquireUserRateLimit 执行 Java 兼容的用户级限流。
func (a *SeckillApp) acquireUserRateLimit(ctx context.Context, userID int64) bool {
	if !a.cfg.RateLimit.UserEnabled || a.userLimiter == nil || userID <= 0 {
		return true
	}
	rate := a.userRateOrDefault()
	interval := a.userIntervalOrDefault()
	ttl := a.userExpireOrDefault()
	allowed, err := a.userLimiter.TryAcquire(ctx, userID, rate, interval, ttl)
	if err != nil {
		commonlogs.CtxWarnf(ctx, "user rate limit failed, allowing by default userId=%d: %v", userID, err)
		return true
	}
	if !allowed {
		commonlogs.CtxWarnf(ctx, "user rate limit triggered userId=%d rate=%d interval=%s",
			userID, rate, interval.String())
	}
	return allowed
}

func (a *SeckillApp) userRateOrDefault() int {
	if a.cfg.RateLimit.UserRate > 0 {
		return a.cfg.RateLimit.UserRate
	}
	return defaultUserRate
}

func (a *SeckillApp) userIntervalOrDefault() time.Duration {
	if a.cfg.RateLimit.UserInterval > 0 {
		return a.cfg.RateLimit.UserInterval
	}
	return defaultUserRateWindow
}

func (a *SeckillApp) userExpireOrDefault() time.Duration {
	if a.cfg.RateLimit.UserExpire > 0 {
		return a.cfg.RateLimit.UserExpire
	}
	return userRateLimiterTTL
}

// setQueuedTrace 保存 Java 兼容的排队状态。
func (a *SeckillApp) setQueuedTrace(ctx context.Context, userID int64, activityNo string, traceID string) {
	if a.queueState == nil || userID <= 0 || activityNo == "" || traceID == "" {
		return
	}
	if err := a.queueState.SetQueued(ctx, userID, activityNo, traceID, queueStateTTL); err != nil {
		commonlogs.CtxWarnf(ctx, "set queued trace failed userId=%d activityNo=%s: %v",
			userID, activityNo, err)
	}
}

// markSuspicious 标记可疑用户
func (a *SeckillApp) markSuspicious(ctx context.Context, activity *ActivityDetail, userID int64, requestIP string) {
	if !a.riskEnabled() || a.risk == nil {
		return
	}
	if err := a.risk.MarkSuspicious(ctx, activity, userID, requestIP); err != nil {
		commonlogs.CtxWarnf(ctx, "mark suspicious user failed userId=%d activityNo=%s: %v",
			userID, activity.ActivityNo, err)
	}
}

// recordSeckillAction 记录秒杀操作用于风险评估
func (a *SeckillApp) recordSeckillAction(ctx context.Context, event PartInEvent) {
	if !a.riskEnabled() || a.risk == nil {
		return
	}
	record := RiskRecord{
		UserID:      event.UserID,
		ActionType:  RiskActionSeckill,
		RiskLevel:   RiskLevelNormal,
		RequestIP:   event.RequestIP,
		RequestInfo: fmt.Sprintf("activityNo=%s skuNo=%s quantity=%d traceId=%s", event.ActivityNo, event.SkuNo, event.Quantity, event.TraceID),
		CreatedAt:   time.Now(),
	}
	if err := a.risk.RecordAction(ctx, record); err != nil {
		commonlogs.CtxWarnf(ctx, "record seckill risk action failed userId=%d activityNo=%s skuNo=%s: %v",
			event.UserID, event.ActivityNo, event.SkuNo, err)
	}
}

// CheckQueue 通过 traceID 检查异步秒杀请求的状态
func (a *SeckillApp) CheckQueue(ctx context.Context, traceID string) (*QueueCheckResult, error) {
	if traceID == "" {
		return &QueueCheckResult{PollStatus: pollStop}, nil
	}
	if a.results != nil {
		result, err := a.results.Get(ctx, traceID)
		if err != nil {
			return nil, fmt.Errorf("get result for trace %s: %w", traceID, err)
		}
		if result == nil {
			return &QueueCheckResult{PollStatus: pollContinue, TraceID: traceID}, nil
		}
		switch result.Status {
		case "processing":
			return &QueueCheckResult{PollStatus: pollContinue, TraceID: traceID}, nil
		case "success":
			return &QueueCheckResult{PollStatus: pollSuccess, TraceID: traceID, OrderNo: result.OrderNo}, nil
		case "failed":
			return &QueueCheckResult{PollStatus: pollStop, TraceID: traceID, Reason: result.Error}, nil
		}
	}
	return &QueueCheckResult{PollStatus: pollContinue, TraceID: traceID}, nil
}

// CheckQueueByActivity 保持旧的兼容路径，供尚未切换到 traceId 轮询的调用者使用
func (a *SeckillApp) CheckQueueByActivity(ctx context.Context, userID int64, activityNo string) (string, error) {
	if a.queueState != nil {
		traceID, err := a.queueState.GetQueuedTrace(ctx, userID, activityNo)
		if err != nil {
			return "", fmt.Errorf("get queued trace %d/%s: %w", userID, activityNo, err)
		}
		if traceID != "" {
			result, err := a.CheckQueue(ctx, traceID)
			if err != nil {
				return "", err
			}
			if result.PollStatus == pollSuccess {
				return result.OrderNo, nil
			}
			return "", nil
		}
	}
	if a.order == nil {
		return "", nil
	}
	orders, err := a.order.ListOrdersByActivity(ctx, activityNo)
	if err != nil {
		return "", fmt.Errorf("list orders by activity %s: %w", activityNo, err)
	}
	for _, order := range orders {
		if order.UserID == userID {
			return order.OrderNo, nil
		}
	}
	return "", nil
}

// calculatePayAmount 计算支付金额
func (a *SeckillApp) calculatePayAmount(activity *ActivityDetail, skuNo string, quantity int) int64 {
	for _, p := range activity.Products {
		if p.SKUNo == skuNo {
			return p.SeckillPrice * int64(quantity)
		}
	}
	return 0
}

// Order 检索订单详情
func (a *SeckillApp) Order(ctx context.Context, orderNo string) (*OrderDetail, error) {
	res, err := a.order.GetOrder(ctx, orderNo)
	if err != nil {
		return nil, fmt.Errorf("get order %s: %w", orderNo, err)
	}
	return res, nil
}

// OrdersByUser 列出用户的订单
func (a *SeckillApp) OrdersByUser(ctx context.Context, userID int64) ([]OrderDetail, error) {
	res, err := a.order.ListOrdersByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list orders by user %d: %w", userID, err)
	}
	return res, nil
}
