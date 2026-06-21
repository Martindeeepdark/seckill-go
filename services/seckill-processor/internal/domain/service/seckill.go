// Package service 提供秒杀领域的核心业务服务
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"seckill-common/metrics"

	"seckill-processor-service/internal/domain/event"
	"seckill-processor-service/internal/domain/model"
	"seckill-processor-service/internal/domain/status"
)

var (
	ErrNotFound         = errors.New("not found")
	ErrDuplicateTraceID = errors.New("duplicate trace_id") // CreateOrder 命中 DB UNIQUE INDEX (23505)
	ErrOrderNotFound    = errors.New("order not found")    // GetByUserAndTrace 未找到订单
)

// EventPublisher 事件发布器接口
type EventPublisher interface {
	Publish(topic string, args ...interface{})
}

// IDGenerator 订单号生成器接口
type IDGenerator interface {
	NextOrderNo() string
}

// TemporaryChecker 临时错误判断接口
type TemporaryChecker interface {
	IsTemporary(err error) bool
}

// ActivityQuery 活动查询接口
type ActivityQuery interface {
	GetActivity(ctx context.Context, activityNo string) (model.ActivityInfo, error)
	GetSKU(ctx context.Context, activityNo, skuNo string) (model.SKUInfo, error)
}

// StockGateway 库存网关接口
type StockGateway interface {
	DeductStockWithLimit(ctx context.Context, activityNo, skuNo string, userID int64, quantity, purchaseLimit int64, orderNo string) (bool, error)
	ReleaseStock(ctx context.Context, activityNo, skuNo string, userID int64, quantity int64, orderNo string) error
}

// RiskGateway 风控网关接口
type RiskGateway interface {
	Evaluate(ctx context.Context, userID int64, requestIP string) (model.RiskResult, error)
}

// OrderCreator 订单创建器接口
// CreateOrder 抛出 ErrDuplicateTraceID 表示 trace_id 冲突(Layer 2 兜底命中)
// GetByUserAndTrace 用于 DuplicateKey 回查,返回 ErrOrderNotFound 表示无对应订单
type OrderCreator interface {
	CreateOrder(ctx context.Context, order model.OrderRequest) error
	GetByUserAndTrace(ctx context.Context, userID int64, traceID string) (model.OrderInfo, error)
}

// FreeCardGateway 自由卡网关接口
type FreeCardGateway interface {
	IssueCard(ctx context.Context, payload model.IssueCardPayload) (string, error)
}

// OrderSyncGateway 订单同步网关接口
type OrderSyncGateway interface {
	SyncOrder(ctx context.Context, payload model.SyncOrderPayload) error
}

// SeckillService 秒杀服务
// 负责处理秒杀请求的核心业务逻辑
type SeckillService struct {
	activity   ActivityQuery    // 活动查询
	stock      StockGateway     // 库存网关
	risk       RiskGateway      // 风控网关
	orders     OrderCreator     // 订单创建器
	events     EventPublisher   // 事件发布器
	idGen      IDGenerator      // 订单号生成器
	tmpChecker TemporaryChecker // 临时错误判断器
	now        func() time.Time // 当前时间函数（可测试）
	logger     *slog.Logger     // 日志记录器
}

// NewSeckillService 创建秒杀服务实例
func NewSeckillService(
	activity ActivityQuery,
	stock StockGateway,
	risk RiskGateway,
	orders OrderCreator,
	events EventPublisher,
	idGen IDGenerator,
	tmpChecker TemporaryChecker,
	logger *slog.Logger,
) *SeckillService {
	if logger == nil {
		logger = slog.Default()
	}
	return &SeckillService{
		activity:   activity,
		stock:      stock,
		risk:       risk,
		orders:     orders,
		events:     events,
		idGen:      idGen,
		tmpChecker: tmpChecker,
		now:        time.Now,
		logger:     logger,
	}
}

// Submit 处理秒杀提交请求
// 执行流程：
// 1. 并行执行：检查活动状态 + 风控评估（errgroup fan-out）
// 2. 获取 SKU 信息
// 3. 扣减库存（带限购检查）
// 4. 创建订单
// 5. 发布订单创建事件
// 如果任何步骤失败，会拒绝请求并发布拒绝事件
func (s *SeckillService) Submit(ctx context.Context, cmd model.SubmitCommand) error {
	submitStart := time.Now()
	// 1. 并行执行活动检查和风控评估（errgroup fan-out）
	g, gctx := errgroup.WithContext(ctx)

	var activity model.ActivityInfo
	g.Go(func() error {
		var err error
		activity, err = s.activity.GetActivity(gctx, cmd.ActivityNo)
		if errors.Is(err, ErrNotFound) {
			return fmt.Errorf("activity %s not found: %w", cmd.ActivityNo, ErrNotFound)
		}
		if err != nil {
			return fmt.Errorf("get activity %s: %w", cmd.ActivityNo, err)
		}
		if !activity.IsOpen(s.now()) {
			return fmt.Errorf("activity %s closed", cmd.ActivityNo)
		}
		return nil
	})

	var evaluation model.RiskResult
	g.Go(func() error {
		var err error
		evaluation, err = s.risk.Evaluate(gctx, cmd.UserID, cmd.RequestIP)
		if err != nil {
			return fmt.Errorf("risk evaluate user %d: %w", cmd.UserID, err)
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		// 区分业务拒绝与基础设施错误
		errMsg := err.Error()
		if strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "closed") {
			s.reject(cmd, event.ReasonActivityClosed)
			return nil
		}
		return err
	}
	fanoutElapsed := time.Since(submitStart) // 含 GetActivity + RiskEvaluate 并行段

	// 两个 goroutine 均成功后检查风控结果
	if evaluation.Risk {
		s.logger.Warn("risk user blocked in processor",
			"userId", cmd.UserID,
			"traceId", cmd.TraceID,
			"reason", evaluation.Reason,
		)
		s.reject(cmd, event.ReasonRiskUser)
		return nil
	}

	// 3. 获取 SKU 信息
	skuStart := time.Now()
	sku, err := s.activity.GetSKU(ctx, cmd.ActivityNo, cmd.SKUNo)
	if errors.Is(err, ErrNotFound) {
		s.reject(cmd, event.ReasonActivityClosed)
		return nil
	}
	if err != nil {
		return fmt.Errorf("get sku %s/%s: %w", cmd.ActivityNo, cmd.SKUNo, err)
	}
	skuElapsed := time.Since(skuStart)

	// 4. 扣减库存（带限购检查）
	limit := sku.EffectiveLimit(activity.PurchaseLimit)
	if os.Getenv("SMOKE_NO_LIMIT") == "1" {
		limit = 0 // 压测：关闭限购，测 processor 纯消费天花板
	}
	baseOrderNo := s.idGen.NextOrderNo()
	shardID := cmd.UserID % 4 // 分片标识：与分区表的哈希分区对应
	orderNo := fmt.Sprintf("%s%d", baseOrderNo, shardID)
	deductStart := time.Now()
	deducted, err := s.stock.DeductStockWithLimit(ctx, cmd.ActivityNo, cmd.SKUNo, cmd.UserID, cmd.Quantity, limit, orderNo)
	deductElapsed := time.Since(deductStart)
	if err != nil {
		return fmt.Errorf("deduct stock: %w", err)
	}
	if !deducted {
		metrics.IncrStockEmpty(ctx, cmd.RunID)
		s.logger.Info("stock empty during deduction",
			"activityNo", cmd.ActivityNo,
			"skuNo", cmd.SKUNo,
			"userId", cmd.UserID,
		)
		s.reject(cmd, event.ReasonStockEmpty)
		return nil
	}

	// 5. 创建订单
	payAmount := cmd.TotalFee
	if payAmount <= 0 {
		payAmount = sku.CalcPrice(cmd.Quantity)
	}
	order := model.OrderRequest{
		OrderNo:        orderNo,
		UserID:         cmd.UserID,
		ActivityNo:     cmd.ActivityNo,
		SKUNo:          cmd.SKUNo,
		Quantity:       cmd.Quantity,
		PayAmount:      payAmount,
		Status:         status.OrderPending,
		TraceID:        cmd.TraceID,
		RequestTraceID: cmd.RequestTraceID,
		CreatedAt:      s.now(),
	}
	orderStart := time.Now()
	if err := s.orders.CreateOrder(ctx, order); err != nil {
		// Layer 2 兜底: CreateOrder 命中 trace_id UNIQUE INDEX → 回查已存在订单
		if errors.Is(err, ErrDuplicateTraceID) {
			return s.handleDuplicateTraceID(ctx, cmd, err)
		}

		// 创建订单失败，需要释放库存
		if s.tmpChecker.IsTemporary(err) {
			if releaseErr := s.stock.ReleaseStock(ctx, cmd.ActivityNo, cmd.SKUNo, cmd.UserID, cmd.Quantity, orderNo); releaseErr != nil {
				s.logger.Warn("release stock after temporary order failure failed",
					"traceId", cmd.TraceID,
					"userId", cmd.UserID,
					"error", releaseErr,
				)
			}
			return fmt.Errorf("create order temporary failure: %w", err)
		}
		// 非临时错误，释放库存并拒绝请求
		if releaseErr := s.stock.ReleaseStock(ctx, cmd.ActivityNo, cmd.SKUNo, cmd.UserID, cmd.Quantity, orderNo); releaseErr != nil {
			s.logger.Warn("release stock after order failure failed",
				"traceId", cmd.TraceID,
				"userId", cmd.UserID,
				"error", releaseErr,
			)
		}
		s.reject(cmd, event.ReasonOrderFail)
		s.logger.Warn("create order failed, stock released",
			"traceId", cmd.TraceID,
			"userId", cmd.UserID,
			"error", err,
		)
		return nil
	}

	// 6. Smoke counter: 订单创建成功
	metrics.IncrSuccess(ctx, cmd.RunID)

	// 7. 发布订单创建事件
	if s.events != nil {
		s.events.Publish(event.TopicOrderCreated, event.OrderCreated{
			OrderNo:        order.OrderNo,
			UserID:         order.UserID,
			ActivityNo:     order.ActivityNo,
			SKUNo:          order.SKUNo,
			Quantity:       order.Quantity,
			PayAmount:      order.PayAmount,
			TraceID:        order.TraceID,
			RequestTraceID: order.RequestTraceID,
		})
	}
	orderElapsed := time.Since(orderStart)
	totalElapsed := time.Since(submitStart)
	// 正常请求保持简洁；慢请求（>50ms）才带分阶段耗时，便于排查长尾。
	args := []any{
		"traceId", cmd.TraceID,
		"requestTraceId", cmd.RequestTraceID,
		"orderNo", order.OrderNo,
		"userId", cmd.UserID,
		"totalMs", totalElapsed.Milliseconds(),
	}
	if totalElapsed > 50*time.Millisecond {
		args = append(args,
			"fanoutMs", fanoutElapsed.Milliseconds(),
			"skuMs", skuElapsed.Milliseconds(),
			"deductMs", deductElapsed.Milliseconds(),
			"orderMs", orderElapsed.Milliseconds(),
		)
	}
	s.logger.Info("seckill order created", args...)
	return nil
}

// reject 拒绝秒杀请求并发布拒绝事件
func (s *SeckillService) reject(cmd model.SubmitCommand, reason string) {
	if s.events == nil {
		return
	}
	s.events.Publish(event.TopicSeckillRejected, event.SeckillRejected{
		TraceID:        cmd.TraceID,
		Reason:         reason,
		RequestTraceID: cmd.RequestTraceID,
	})
}

// handleDuplicateTraceID 处理 CreateOrder 命中 (user_id, trace_id) UNIQUE INDEX 的场景
// 调用 GetByUserAndTrace 双键回查已存在订单,触发 OrderCreated 事件让上层 markTraceSuccess 写两个 key
// 回查失败(ErrOrderNotFound 或其他)时返回错误让上层 release processor idem key,记 ERROR 日志供告警
func (s *SeckillService) handleDuplicateTraceID(ctx context.Context, cmd model.SubmitCommand, createErr error) error {
	existing, err := s.orders.GetByUserAndTrace(ctx, cmd.UserID, cmd.TraceID)
	if err != nil {
		s.logger.Error("duplicate trace_id but lookup failed",
			"traceId", cmd.TraceID,
			"userId", cmd.UserID,
			"createError", createErr,
			"lookupError", err,
		)
		return fmt.Errorf("duplicate trace_id %s but lookup failed: %w (create err: %v)", cmd.TraceID, err, createErr)
	}
	// 回查成功 → 视为订单已创建,发布 OrderCreated 事件
	// 上层 onOrderCreated 会 markTraceSuccess 双写两个 idem key
	metrics.IncrSuccess(ctx, cmd.RunID)
	if s.events != nil {
		s.events.Publish(event.TopicOrderCreated, event.OrderCreated{
			OrderNo:        existing.OrderNo,
			UserID:         cmd.UserID,
			ActivityNo:     cmd.ActivityNo,
			SKUNo:          cmd.SKUNo,
			Quantity:       cmd.Quantity,
			PayAmount:      existing.PayAmount,
			TraceID:        cmd.TraceID,
			RequestTraceID: cmd.RequestTraceID,
		})
	}
	s.logger.Info("duplicate trace_id resolved via lookup",
		"traceId", cmd.TraceID,
		"userId", cmd.UserID,
		"existingOrderNo", existing.OrderNo,
	)
	return nil
}
