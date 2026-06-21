package application

import (
	"context"
	"fmt"
	"runtime"
	"time"

	commonlogs "github.com/Martindeeepdark/go-common/logs"

	"seckill-common/metrics"
	"seckill-common/tracing"
)

// WorkerPoolConfig 配置 worker pool 的行为。
type WorkerPoolConfig struct {
	// QueueSize 是事件 channel 的缓冲大小。
	// 满时 Submit 返回错误，提供背压（backpressure）。
	QueueSize int

	// WorkerCount 是处理事件的 goroutine 数量。
	// 为零时默认使用 runtime.NumCPU()。
	WorkerCount int
}

// DefaultWorkerPoolConfig 返回合理的默认配置。
func DefaultWorkerPoolConfig() WorkerPoolConfig {
	return WorkerPoolConfig{
		QueueSize:   10000,
		WorkerCount: runtime.NumCPU(),
	}
}

// WorkerPool 异步处理秒杀参与事件。
// 它校验事件并发布到 NATS，同时把结果存入 ResultStore 供前端轮询。
type WorkerPool struct {
	cfg       WorkerPoolConfig
	events    chan PartInEvent
	activity  ActivityGateway
	risk      RiskGateway
	stock     StockGateway
	publisher QueuePublisher
	results   ResultStore
	riskCheck func() bool // 返回是否执行 risk check（nil 表示始终执行）
	stockPeek func() bool // 返回是否执行 stock peek 预检（nil 表示始终执行）
}

// NewWorkerPool 根据给定配置创建 worker pool。
func NewWorkerPool(
	cfg WorkerPoolConfig,
	activity ActivityGateway,
	risk RiskGateway,
	stock StockGateway,
	publisher QueuePublisher,
	results ResultStore,
	_ any, // logger 参数保留用于签名兼容，实际不使用
) *WorkerPool {
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = 10000
	}
	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = runtime.NumCPU()
	}

	return &WorkerPool{
		cfg:       cfg,
		events:    make(chan PartInEvent, cfg.QueueSize),
		activity:  activity,
		risk:      risk,
		stock:     stock,
		publisher: publisher,
		results:   results,
	}
}

// Start 启动 worker goroutine。阻塞直到 context 取消且所有事件处理完毕。
func (p *WorkerPool) Start(ctx context.Context) {
	commonlogs.Infof("starting worker pool, queueSize=%d, workerCount=%d",
		p.cfg.QueueSize, p.cfg.WorkerCount)

	// 启动 worker goroutine
	for i := 0; i < p.cfg.WorkerCount; i++ {
		workerID := i
		go func() {
			commonlogs.Infof("DEBUG launching worker goroutine workerID=%d", workerID)
			p.worker(ctx, workerID)
		}()
	}
	commonlogs.Info("DEBUG all workers launched")

	// 等待 context 取消
	<-ctx.Done()

	commonlogs.Info("worker pool shutting down, draining remaining events")
	close(p.events)

	// 等待 channel 排空（worker 在 channel 关闭且为空时退出）
	// 这是简单的排空逻辑——生产环境可能需要超时控制
	for len(p.events) > 0 {
		time.Sleep(100 * time.Millisecond)
	}

	commonlogs.Info("worker pool stopped")
}

// SetRiskCheckFunc 注入 risk check 开关函数（nil 表示始终执行 risk check）。
// 用于压测等场景按需关闭 gateway 侧风控预检（processor 侧仍有 errgroup 兜底）。
func (p *WorkerPool) SetRiskCheckFunc(fn func() bool) {
	p.riskCheck = fn
}

// SetStockPeekFunc 注入 stock peek 预检开关函数（nil 表示始终执行）。
// 压测场景可关闭：processor 的 DeductStockWithLimit 会原子检查并扣减，gateway 预检非必需。
func (p *WorkerPool) SetStockPeekFunc(fn func() bool) {
	p.stockPeek = fn
}

// Submit 把事件加入队列异步处理。
// 队列满时立即返回错误，提供背压（backpressure）。
func (p *WorkerPool) Submit(ctx context.Context, event PartInEvent) error {
	select {
	case p.events <- event:
		return nil
	default:
		return fmt.Errorf("worker pool queue full (size=%d)", p.cfg.QueueSize)
	}
}

// worker 从 channel 消费事件，直到 context 取消且 channel 关闭排空。
func (p *WorkerPool) worker(ctx context.Context, workerID int) {
	commonlogs.Infof("DEBUG worker started workerID=%d", workerID)
	defer commonlogs.Debugf("worker stopped workerID=%d", workerID)

	for {
		select {
		case event, ok := <-p.events:
			commonlogs.Infof("DEBUG worker received event workerID=%d ok=%v runID=%s",
				workerID, ok, event.RunID)
			if !ok {
				// channel 已关闭，无更多事件
				return
			}
			p.processEvent(ctx, event, workerID)

		case <-ctx.Done():
			// context 已取消，但继续排空剩余事件
			// 持续处理直到 channel 关闭
			for {
				select {
				case event, ok := <-p.events:
					if !ok {
						return
					}
					p.processEvent(ctx, event, workerID)
				default:
					return
				}
			}
		}
	}
}

// processEvent 校验单个秒杀事件并发布。
// 镜像 SeckillApp.PartIn 的校验逻辑，但异步执行。
func (p *WorkerPool) processEvent(ctx context.Context, event PartInEvent, workerID int) {
	submitStart := time.Now()
	traceID := event.TraceID
	if traceID == "" {
		traceID = fmt.Sprintf("worker-%d-%d", workerID, time.Now().UnixNano())
	}
	ctx = tracing.WithTraceID(ctx, traceID)

	commonlogs.CtxInfof(ctx, "DEBUG processEvent called runID=%s runIDLen=%d userId=%s activityNo=%s skuNo=%s workerID=%d",
		event.RunID, len(event.RunID), event.UserID, event.ActivityNo, event.SkuNo, workerID)

	// 标记为处理中
	if p.results != nil {
		if err := p.results.SetProcessing(ctx, traceID); err != nil {
			commonlogs.CtxWarnf(ctx, "failed to set processing status: %v", err)
		}
	}

	// 风控检查
	riskStart := time.Now()
	if p.risk != nil && (p.riskCheck == nil || p.riskCheck()) {
		isRisk, err := p.risk.IsRiskUser(ctx, event.UserID)
		if err != nil {
			commonlogs.CtxWarnf(ctx, "risk check failed, allowing by default: %v", err)
		} else if isRisk {
			metrics.IncrRisk(ctx, event.RunID)
			p.markFailed(ctx, traceID, "risk_user_blocked")
			commonlogs.CtxInfof(ctx, "event rejected: risk user blocked")
			return
		}
	}
	riskMs := time.Since(riskStart).Milliseconds()

	// 活动查询
	actStart := time.Now()
	activity, err := p.activity.GetActivity(ctx, event.ActivityNo)
	if err != nil {
		commonlogs.CtxWarnf(ctx, "activity lookup failed, skipping: %v", err)
		p.markFailed(ctx, traceID, "activity_not_found")
		return
	}
	actMs := time.Since(actStart).Milliseconds()

	// 检查活动是否开放
	if !activity.ActivityOpen {
		commonlogs.CtxInfof(ctx, "DEBUG before metrics.IncrOther runID=%s", event.RunID)
		metrics.IncrOther(ctx, event.RunID)
		commonlogs.CtxInfof(ctx, "DEBUG after metrics.IncrOther runID=%s", event.RunID)
		p.markFailed(ctx, traceID, "activity_not_open")
		commonlogs.CtxInfof(ctx, "event rejected: activity not open")
		return
	}

	// 库存预检
	stockStart := time.Now()
	var stock int64 = 1 // 默认放行，交由 processor DeductStockWithLimit 兜底
	if p.stock != nil && (p.stockPeek == nil || p.stockPeek()) {
		stock, err = p.stock.Peek(ctx, event.ActivityNo, event.SkuNo)
	}
	if err != nil {
		metrics.IncrOther(ctx, event.RunID)
		commonlogs.CtxWarnf(ctx, "stock peek failed, skipping: %v", err)
		p.markFailed(ctx, traceID, "stock_check_failed")
		return
	}
	stockMs := time.Since(stockStart).Milliseconds()

	if stock <= 0 {
		metrics.IncrStockEmpty(ctx, event.RunID)
		p.markFailed(ctx, traceID, "STOCK_EMPTY")
		commonlogs.CtxInfof(ctx, "event rejected: out of stock")
		return
	}

	// 发布到 NATS
	pubStart := time.Now()
	if err := p.publisher.Publish(ctx, event); err != nil {
		commonlogs.CtxErrorf(ctx, "failed to publish event: %v", err)
		p.markFailed(ctx, traceID, "publish_failed")
		return
	}
	pubMs := time.Since(pubStart).Milliseconds()

	// 高频路径（~8k/s 压测、数百/s 生产）：用 Debug 避免日志风暴，
	// 排查 worker 阶段耗时（risk/activity/stock/publish）时调高日志级别即可看到。
	commonlogs.CtxDebugf(ctx, "event processed and published successfully riskMs=%d actMs=%d stockMs=%d pubMs=%d totalMs=%d",
		riskMs, actMs, stockMs, pubMs, time.Since(submitStart).Milliseconds())
}

// markFailed 把失败结果记录到结果存储。
func (p *WorkerPool) markFailed(ctx context.Context, traceID, reason string) {
	if p.results == nil {
		return
	}
	if err := p.results.SetFailed(ctx, traceID, reason); err != nil {
		commonlogs.CtxWarnf(ctx, "failed to mark result as failed, reason=%s: %v", reason, err)
	}
}
