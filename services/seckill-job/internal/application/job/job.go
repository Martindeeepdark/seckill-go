// Package job 提供秒杀系统的定时任务执行器，负责活动状态管理、订单超时处理、库存释放等后台任务。
package job

import (
	"context"
	stderrors "errors"
	"fmt"
	"sync"
	"time"

	commonlogs "github.com/Martindeeepdark/go-common/logs"

	"seckill-job-service/internal/domain/entity"
	"seckill-job-service/internal/domain/status"

	"seckill-common/errors"
)

// Config 定时任务配置
type Config struct {
	RunOnStart                  bool          // 是否在启动时立即执行任务
	ActivityStatusCheckInterval time.Duration // 活动状态检查间隔
	TimeoutOrderCheckInterval   time.Duration // 超时订单检查间隔
	StockReleaseCheckInterval   time.Duration // 库存释放检查间隔
	ActivityDataCleanupInterval time.Duration // 活动数据清理间隔
	ActivityDataRetention       time.Duration // 活动数据保留时长
	RiskUserCleanupInterval     time.Duration // 风控用户清理间隔
	DailyStatisticsInterval     time.Duration // 每日统计间隔
	CacheWarmupInterval         time.Duration // 缓存预热间隔
	CacheRefreshInterval        time.Duration // 缓存刷新间隔
	CacheWarmupAhead            time.Duration // 缓存预热提前时长
	PaymentReconcileInterval    time.Duration // 支付对账间隔
	OrderSyncCheckInterval      time.Duration // 订单同步检查间隔
}

// ActivityGateway 活动服务网关接口
type ActivityGateway interface {
	ListActivities(ctx context.Context) ([]entity.Activity, error)
	UpdateActivityStatus(ctx context.Context, activityNo string, status int) error
}

// OrderGateway 订单服务网关接口
type OrderGateway interface {
	ListOrdersByActivities(ctx context.Context, activityNos []string) (map[string][]entity.Order, error)
	CloseOrder(ctx context.Context, orderNo string) error
	MarkOrderPaid(ctx context.Context, orderNo string, transactionNo string, paidAt time.Time) error
}

// StockGateway 库存服务网关接口
type StockGateway interface {
	ReleaseStock(ctx context.Context, activityNo, skuNo string, userID int64, quantity int, orderNo string) error
	CleanupActivityStock(ctx context.Context, activityNo string, skuNos []string) (int, error)
	CleanupActivityPurchases(ctx context.Context, activityNo string) (int, error)
}

// PaymentGateway 支付服务网关接口
type PaymentGateway interface {
	QueryPayments(ctx context.Context, orderNos []string) (map[string]entity.PayQueryResult, error)
	ClosePayment(ctx context.Context, orderNo string) error
}

// OrderSyncGateway 订单同步服务网关接口
type OrderSyncGateway interface {
	SyncOrder(ctx context.Context, request entity.SyncOrderRequest) error
	ListSyncedOrdersByOrderNos(ctx context.Context, orderNos []string) (map[string]entity.SyncedOrder, error)
}

// RiskGateway 风控服务网关接口
type RiskGateway interface {
	CleanupExpiredRiskUsers(ctx context.Context) (int, error)
}

// CacheInvalidator 缓存失效接口
type CacheInvalidator interface {
	EvictActivity(ctx context.Context, activityNo string) error
}

// CacheWriter 缓存写入接口
type CacheWriter interface {
	WarmupActivity(ctx context.Context, activity entity.Activity) error
	RefreshActivity(ctx context.Context, activity entity.Activity) error
	RefreshActiveActivities(ctx context.Context, activities []entity.Activity) error
}

// Runner 定时任务执行器，负责管理和执行所有后台定时任务
type Runner struct {
	cfg         Config           // 任务配置
	activity    ActivityGateway  // 活动服务网关
	orders      OrderGateway     // 订单服务网关
	stock       StockGateway     // 库存服务网关
	payments    PaymentGateway   // 支付服务网关
	sync        OrderSyncGateway // 订单同步服务网关
	risk        RiskGateway      // 风控服务网关
	cache       CacheInvalidator // 缓存失效接口
	cacheWriter CacheWriter      // 缓存写入接口
	clock       func() time.Time // 时钟函数（用于测试）
}

// Option Runner 可选配置函数
type Option func(*Runner)

// WithClock 设置时钟函数（用于测试）
func WithClock(clock func() time.Time) Option {
	return func(r *Runner) { r.clock = clock }
}

// WithCacheInvalidator 设置缓存失效接口
func WithCacheInvalidator(cache CacheInvalidator) Option {
	return func(r *Runner) { r.cache = cache }
}

// WithOrderSync 设置订单同步网关
func WithOrderSync(sync OrderSyncGateway) Option {
	return func(r *Runner) { r.sync = sync }
}

// WithRiskGateway 设置风控网关
func WithRiskGateway(risk RiskGateway) Option {
	return func(r *Runner) { r.risk = risk }
}

// WithCacheWriter 设置缓存写入接口
func WithCacheWriter(writer CacheWriter) Option {
	return func(r *Runner) { r.cacheWriter = writer }
}

// ActivityStatusResult 活动状态更新结果
type ActivityStatusResult struct {
	Activated int // 已激活的活动数
	Ended     int // 已结束的活动数
}

// TimeoutOrderResult 超时订单处理结果
type TimeoutOrderResult struct {
	Closed        int // 已关闭的订单数
	StockReleased int // 已释放的库存数
}

// StockReleaseResult 库存释放结果
type StockReleaseResult struct {
	Activities int // 处理的活动数
	Deleted    int // 删除的缓存键数
}

// ActivityDataCleanupResult 活动数据清理结果
type ActivityDataCleanupResult struct {
	Activities int // 清理的活动数
	Deleted    int // 删除的数据记录数
}

// RiskUserCleanupResult 风控用户清理结果
type RiskUserCleanupResult struct {
	Deleted int // 删除的用户数
}

// DailyStatisticsResult 每日统计结果
type DailyStatisticsResult struct {
	ActiveCount int                  // 进行中的活动数
	EndedCount  int                  // 已结束的活动数
	TotalCount  int                  // 总活动数
	Activities  []ActivityStatistics // 活动统计详情
}

// ActivityStatistics 活动统计信息
type ActivityStatistics struct {
	ActivityNo     string    // 活动编号
	ActivityName   string    // 活动名称
	TotalInitStock int64     // 总初始库存
	StartTime      time.Time // 开始时间
	EndTime        time.Time // 结束时间
}

// ActivityCacheWarmupResult 活动缓存预热结果
type ActivityCacheWarmupResult struct {
	Warmed   int // 已预热的缓存数
	Active   int // 进行中的活动数
	Upcoming int // 即将开始的活动数
}

// ActivityCacheRefreshResult 活动缓存刷新结果
type ActivityCacheRefreshResult struct {
	Refreshed int // 已刷新的缓存数
	Active    int // 进行中的活动数
}

// PaymentReconcileResult 支付对账结果
type PaymentReconcileResult struct {
	Activities    int // 涉及的活动数
	Checked       int // 检查的订单数
	Compensated   int // 补偿的订单数
	MismatchCount int // 不一致的订单数
}

// OrderSyncCheckResult 订单同步检查结果
type OrderSyncCheckResult struct {
	Activities    int // 涉及的活动数
	Checked       int // 检查的订单数
	Compensated   int // 补偿同步的订单数
	MismatchCount int // 未同步的订单数
}

// NewRunner 创建任务执行器实例
func NewRunner(cfg Config, activity ActivityGateway, orders OrderGateway, stock StockGateway, payments PaymentGateway, _ any, opts ...Option) *Runner {
	r := &Runner{
		cfg:      cfg.normalize(),
		activity: activity,
		orders:   orders,
		stock:    stock,
		payments: payments,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Run 启动所有定时任务，并发执行10个后台任务
func (r *Runner) Run(ctx context.Context) error {
	const numJobs = 10
	errCh := make(chan error, numJobs)
	wg := &sync.WaitGroup{}
	wg.Add(numJobs)

	launchJob := func(name string, interval time.Duration, fn func(context.Context) error) {
		go func() {
			defer wg.Done()
			errCh <- r.loop(ctx, name, interval, fn)
		}()
	}

	launchJob("activityStatusUpdate", r.cfg.ActivityStatusCheckInterval, func(ctx context.Context) error {
		result, err := r.UpdateActivityStatuses(ctx)
		if err == nil && (result.Activated > 0 || result.Ended > 0) {
			commonlogs.Infof("activity status job completed activated=%d ended=%d", result.Activated, result.Ended)
		}
		return err
	})
	launchJob("cancelTimeoutOrder", r.cfg.TimeoutOrderCheckInterval, func(ctx context.Context) error {
		result, err := r.CloseTimeoutOrders(ctx)
		if err == nil && result.Closed > 0 {
			commonlogs.Infof("timeout order job completed closed=%d stockReleased=%d", result.Closed, result.StockReleased)
		}
		return err
	})
	launchJob("stockRelease", r.cfg.StockReleaseCheckInterval, func(ctx context.Context) error {
		result, err := r.ReleaseEndedActivityKeys(ctx)
		if err == nil && result.Activities > 0 {
			commonlogs.Infof("stock release job completed activities=%d deleted=%d", result.Activities, result.Deleted)
		}
		return err
	})
	launchJob("activityDataCleanup", r.cfg.ActivityDataCleanupInterval, func(ctx context.Context) error {
		result, err := r.CleanupExpiredActivityData(ctx)
		if err == nil && result.Activities > 0 {
			commonlogs.Infof("activity data cleanup job completed activities=%d deleted=%d", result.Activities, result.Deleted)
		}
		return err
	})
	launchJob("riskUserCleanup", r.cfg.RiskUserCleanupInterval, func(ctx context.Context) error {
		result, err := r.CleanupExpiredRiskUsers(ctx)
		if err == nil && result.Deleted > 0 {
			commonlogs.Infof("risk user cleanup job completed deleted=%d", result.Deleted)
		}
		return err
	})
	launchJob("dailyStatistics", r.cfg.DailyStatisticsInterval, func(ctx context.Context) error {
		_, err := r.DailyStatistics(ctx)
		return err
	})
	launchJob("cacheWarmup", r.cfg.CacheWarmupInterval, func(ctx context.Context) error {
		result, err := r.WarmupActivityCache(ctx)
		if err == nil && result.Warmed > 0 {
			commonlogs.Infof("activity cache warmup job completed warmed=%d active=%d upcoming=%d", result.Warmed, result.Active, result.Upcoming)
		}
		return err
	})
	launchJob("cacheRefresh", r.cfg.CacheRefreshInterval, func(ctx context.Context) error {
		result, err := r.RefreshActivityCache(ctx)
		if err == nil && result.Refreshed > 0 {
			commonlogs.Infof("activity cache refresh job completed refreshed=%d active=%d", result.Refreshed, result.Active)
		}
		return err
	})
	launchJob("paymentReconcile", r.cfg.PaymentReconcileInterval, func(ctx context.Context) error {
		result, err := r.ReconcilePayments(ctx)
		if err == nil && result.Checked > 0 {
			commonlogs.Infof("payment reconcile job completed activities=%d checked=%d compensated=%d mismatchCount=%d",
				result.Activities, result.Checked, result.Compensated, result.MismatchCount)
		}
		return err
	})
	launchJob("orderSyncCheck", r.cfg.OrderSyncCheckInterval, func(ctx context.Context) error {
		result, err := r.CheckOrderSync(ctx)
		if err == nil && result.Checked > 0 {
			commonlogs.Infof("order sync check job completed activities=%d checked=%d compensated=%d mismatchCount=%d",
				result.Activities, result.Checked, result.Compensated, result.MismatchCount)
		}
		return err
	})

	select {
	case <-ctx.Done():
		wg.Wait()
		return nil
	case err := <-errCh:
		if stderrors.Is(err, context.Canceled) || stderrors.Is(err, context.DeadlineExceeded) {
			return nil
		}
		return err
	}
}

// UpdateActivityStatuses 更新所有活动状态
func (r *Runner) UpdateActivityStatuses(ctx context.Context) (ActivityStatusResult, error) {
	var result ActivityStatusResult
	if r.activity == nil {
		return result, nil
	}
	activities, err := r.activity.ListActivities(ctx)
	if err != nil {
		return result, fmt.Errorf("list activities for status update: %w", err)
	}
	now := r.now()
	for _, activity := range activities {
		target, ok := nextActivityStatus(activity, now)
		if !ok {
			continue
		}
		if err := r.activity.UpdateActivityStatus(ctx, activity.ActivityNo, target); err != nil {
			return result, fmt.Errorf("update activity %s status to %d: %w", activity.ActivityNo, target, err)
		}
		r.evictActivity(ctx, activity.ActivityNo)
		switch target {
		case status.ActivityOpen:
			result.Activated++
			commonlogs.Infof("activity opened by job activityNo=%s activityName=%s", activity.ActivityNo, activity.Name)
		case status.ActivityEnded:
			result.Ended++
			commonlogs.Infof("activity ended by job activityNo=%s activityName=%s", activity.ActivityNo, activity.Name)
		}
	}
	return result, nil
}

// CloseTimeoutOrders 关闭已结束活动的待支付订单并释放库存
func (r *Runner) CloseTimeoutOrders(ctx context.Context) (TimeoutOrderResult, error) {
	var result TimeoutOrderResult
	if r.activity == nil || r.orders == nil {
		return result, nil
	}
	activities, err := r.activity.ListActivities(ctx)
	if err != nil {
		return result, fmt.Errorf("list activities for timeout orders: %w", err)
	}
	now := r.now()
	ended := make([]entity.Activity, 0, len(activities))
	for _, activity := range activities {
		if !isEndedActivity(activity, now) {
			continue
		}
		ended = append(ended, activity)
	}
	ordersByActivity, err := r.orders.ListOrdersByActivities(ctx, activityNos(ended))
	if err != nil {
		return result, fmt.Errorf("list orders by activities for timeout check: %w", err)
	}
	for _, activity := range ended {
		closed, released, err := r.closeActivityPendingOrders(ctx, activity, ordersByActivity[activity.ActivityNo])
		if err != nil {
			return result, fmt.Errorf("close pending orders for activity %s: %w", activity.ActivityNo, err)
		}
		result.Closed += closed
		result.StockReleased += released
	}
	return result, nil
}

// ReleaseEndedActivityKeys 释放已结束活动的库存缓存
func (r *Runner) ReleaseEndedActivityKeys(ctx context.Context) (StockReleaseResult, error) {
	var result StockReleaseResult
	if r.activity == nil || r.stock == nil {
		return result, nil
	}
	activities, err := r.activity.ListActivities(ctx)
	if err != nil {
		return result, fmt.Errorf("list activities for stock release: %w", err)
	}
	now := r.now()
	for _, activity := range activities {
		if !isEndedActivity(activity, now) {
			continue
		}
		r.evictActivity(ctx, activity.ActivityNo)
		deleted, err := r.stock.CleanupActivityStock(ctx, activity.ActivityNo, skuNos(activity))
		if err != nil {
			return result, fmt.Errorf("cleanup activity %s stock: %w", activity.ActivityNo, err)
		}
		result.Activities++
		result.Deleted += deleted
		commonlogs.Infof("activity stock keys released by job activityNo=%s deleted=%d", activity.ActivityNo, deleted)
	}
	return result, nil
}

// CleanupExpiredActivityData 清理过期活动的临时数据
func (r *Runner) CleanupExpiredActivityData(ctx context.Context) (ActivityDataCleanupResult, error) {
	var result ActivityDataCleanupResult
	if r.activity == nil || r.stock == nil {
		return result, nil
	}
	activities, err := r.activity.ListActivities(ctx)
	if err != nil {
		return result, fmt.Errorf("list activities for data cleanup: %w", err)
	}
	threshold := r.now().Add(-r.cfg.ActivityDataRetention)
	for _, activity := range activities {
		if !shouldCleanupActivityData(activity, threshold) {
			continue
		}
		deleted, err := r.stock.CleanupActivityPurchases(ctx, activity.ActivityNo)
		if err != nil {
			return result, fmt.Errorf("cleanup activity %s purchases: %w", activity.ActivityNo, err)
		}
		result.Activities++
		result.Deleted += deleted
		commonlogs.Infof("activity temporary data cleaned by job activityNo=%s deleted=%d", activity.ActivityNo, deleted)
	}
	return result, nil
}

// CleanupExpiredRiskUsers 清理过期的风控用户记录
func (r *Runner) CleanupExpiredRiskUsers(ctx context.Context) (RiskUserCleanupResult, error) {
	var result RiskUserCleanupResult
	if r.risk == nil {
		return result, nil
	}
	deleted, err := r.risk.CleanupExpiredRiskUsers(ctx)
	if err != nil {
		return result, fmt.Errorf("cleanup expired risk users: %w", err)
	}
	result.Deleted = deleted
	return result, nil
}

// DailyStatistics 生成每日活动统计信息
func (r *Runner) DailyStatistics(ctx context.Context) (DailyStatisticsResult, error) {
	var result DailyStatisticsResult
	if r.activity == nil {
		return result, nil
	}
	activities, err := r.activity.ListActivities(ctx)
	if err != nil {
		return result, fmt.Errorf("list activities for daily statistics: %w", err)
	}
	for _, activity := range activities {
		switch activity.Status {
		case status.ActivityOpen:
			result.ActiveCount++
		case status.ActivityEnded:
			result.EndedCount++
			result.Activities = append(result.Activities, ActivityStatistics{
				ActivityNo:     activity.ActivityNo,
				ActivityName:   activity.Name,
				TotalInitStock: totalInitStock(activity.SKUs),
				StartTime:      activity.StartTime,
				EndTime:        activity.EndTime,
			})
		}
	}
	result.TotalCount = result.ActiveCount + result.EndedCount
	commonlogs.Infof("daily activity statistics activeCount=%d endedCount=%d totalCount=%d",
		result.ActiveCount, result.EndedCount, result.TotalCount)
	for _, metric := range result.Activities {
		commonlogs.Infof("activity statistics activityNo=%s activityName=%s totalInitStock=%d startTime=%v endTime=%v",
			metric.ActivityNo, metric.ActivityName, metric.TotalInitStock, metric.StartTime, metric.EndTime)
	}
	return result, nil
}

// WarmupActivityCache 预热活动缓存
func (r *Runner) WarmupActivityCache(ctx context.Context) (ActivityCacheWarmupResult, error) {
	var result ActivityCacheWarmupResult
	if r.activity == nil || r.cacheWriter == nil {
		return result, nil
	}
	activities, err := r.activity.ListActivities(ctx)
	if err != nil {
		return result, fmt.Errorf("list activities for cache warmup: %w", err)
	}
	now := r.now()
	deadline := now.Add(r.cfg.CacheWarmupAhead)
	for _, activity := range activities {
		if isActiveActivity(activity, now) {
			if err := r.cacheWriter.WarmupActivity(ctx, activity); err != nil {
				return result, fmt.Errorf("warmup activity %s cache: %w", activity.ActivityNo, err)
			}
			result.Warmed++
			result.Active++
			continue
		}
		if isUpcomingActivity(activity, now, deadline) {
			if err := r.cacheWriter.WarmupActivity(ctx, activity); err != nil {
				return result, fmt.Errorf("warmup upcoming activity %s cache: %w", activity.ActivityNo, err)
			}
			result.Warmed++
			result.Upcoming++
		}
	}
	return result, nil
}

// RefreshActivityCache 刷新活动缓存
func (r *Runner) RefreshActivityCache(ctx context.Context) (ActivityCacheRefreshResult, error) {
	var result ActivityCacheRefreshResult
	if r.activity == nil || r.cacheWriter == nil {
		return result, nil
	}
	activities, err := r.activity.ListActivities(ctx)
	if err != nil {
		return result, fmt.Errorf("list activities for cache refresh: %w", err)
	}
	now := r.now()
	active := make([]entity.Activity, 0, len(activities))
	for _, activity := range activities {
		if !isActiveActivity(activity, now) {
			continue
		}
		active = append(active, activity)
		if err := r.cacheWriter.RefreshActivity(ctx, activity); err != nil {
			return result, fmt.Errorf("refresh activity %s cache: %w", activity.ActivityNo, err)
		}
		result.Refreshed++
	}
	if err := r.cacheWriter.RefreshActiveActivities(ctx, active); err != nil {
		return result, fmt.Errorf("refresh active activities list: %w", err)
	}
	result.Active = len(active)
	return result, nil
}

// ReconcilePayments 支付对账
func (r *Runner) ReconcilePayments(ctx context.Context) (PaymentReconcileResult, error) {
	var result PaymentReconcileResult
	if r.activity == nil || r.orders == nil || r.payments == nil {
		return result, nil
	}
	activities, err := r.activity.ListActivities(ctx)
	if err != nil {
		return result, fmt.Errorf("list activities for payment reconcile: %w", err)
	}
	now := r.now()
	targets := make([]entity.Activity, 0, len(activities))
	for _, activity := range activities {
		if !shouldReconcileActivity(activity, now) {
			continue
		}
		targets = append(targets, activity)
	}
	ordersByActivity, err := r.orders.ListOrdersByActivities(ctx, activityNos(targets))
	if err != nil {
		return result, fmt.Errorf("list orders for payment reconcile: %w", err)
	}
	paymentQueries, err := r.queryPayments(ctx, paymentOrderNos(targets, ordersByActivity))
	if err != nil {
		return result, fmt.Errorf("query payments for reconcile: %w", err)
	}
	for _, activity := range targets {
		checked, compensated, mismatches, err := r.reconcileActivityPayments(ctx, activity, ordersByActivity[activity.ActivityNo], paymentQueries)
		if err != nil {
			return result, fmt.Errorf("reconcile payments for activity %s: %w", activity.ActivityNo, err)
		}
		if checked > 0 {
			result.Activities++
		}
		result.Checked += checked
		result.Compensated += compensated
		result.MismatchCount += mismatches
	}
	return result, nil
}

// CheckOrderSync 检查订单同步状态
func (r *Runner) CheckOrderSync(ctx context.Context) (OrderSyncCheckResult, error) {
	var result OrderSyncCheckResult
	if r.activity == nil || r.orders == nil || r.sync == nil {
		return result, nil
	}
	activities, err := r.activity.ListActivities(ctx)
	if err != nil {
		return result, fmt.Errorf("list activities for order sync check: %w", err)
	}
	now := r.now()
	targets := make([]entity.Activity, 0, len(activities))
	for _, activity := range activities {
		if !shouldReconcileActivity(activity, now) {
			continue
		}
		targets = append(targets, activity)
	}
	ordersByActivity, err := r.orders.ListOrdersByActivities(ctx, activityNos(targets))
	if err != nil {
		return result, fmt.Errorf("list orders for sync check: %w", err)
	}
	syncedOrders, err := r.listSyncedOrdersByOrderNos(ctx, paidOrderNos(targets, ordersByActivity))
	if err != nil {
		return result, fmt.Errorf("list synced orders for check: %w", err)
	}
	for _, activity := range targets {
		checked, compensated, mismatches, err := r.checkActivityOrderSync(ctx, activity, ordersByActivity[activity.ActivityNo], syncedOrders)
		if err != nil {
			return result, fmt.Errorf("check order sync for activity %s: %w", activity.ActivityNo, err)
		}
		if checked > 0 {
			result.Activities++
		}
		result.Checked += checked
		result.Compensated += compensated
		result.MismatchCount += mismatches
	}
	return result, nil
}

// -- Private helpers --

func (r *Runner) checkActivityOrderSync(ctx context.Context, activity entity.Activity, orders []entity.Order, syncedOrders map[string]entity.SyncedOrder) (int, int, int, error) {
	checked := 0
	compensated := 0
	mismatches := 0
	for _, order := range orders {
		if order.Status != status.OrderPaid {
			continue
		}
		checked++
		if _, ok := syncedOrders[order.OrderNo]; ok {
			continue
		}
		mismatches++
		if err := r.sync.SyncOrder(ctx, r.syncOrderRequest(order)); err != nil {
			return checked, compensated, mismatches, fmt.Errorf("sync order %s for activity %s: %w", order.OrderNo, activity.ActivityNo, err)
		}
		compensated++
		commonlogs.Warnf("main order missing, synced by compensation job orderNo=%s activityNo=%s userId=%d", order.OrderNo, activity.ActivityNo, order.UserID)
	}
	return checked, compensated, mismatches, nil
}

func (r *Runner) reconcileActivityPayments(ctx context.Context, activity entity.Activity, orders []entity.Order, paymentQueries map[string]entity.PayQueryResult) (int, int, int, error) {
	checked := 0
	compensated := 0
	mismatches := 0
	for _, order := range orders {
		if order.Status != status.OrderPending && order.Status != status.OrderPaid {
			continue
		}
		checked++
		query, ok := paymentQueries[order.OrderNo]
		if !ok {
			if order.Status == status.OrderPaid {
				mismatches++
				commonlogs.Warnf("paid seckill order missing payment record orderNo=%s activityNo=%s", order.OrderNo, activity.ActivityNo)
			}
			continue
		}
		if query.PayStatus == status.PayStatusPaid && order.Status == status.OrderPending {
			paidAt := query.PaidAt
			if paidAt == nil {
				now := r.now()
				paidAt = &now
			}
			if err := r.orders.MarkOrderPaid(ctx, order.OrderNo, query.TransactionNo, *paidAt); err != nil {
				if stderrors.Is(err, errors.ErrInvalidState) || stderrors.Is(err, errors.ErrNotFound) {
					continue
				}
				return checked, compensated, mismatches, fmt.Errorf("mark order %s paid: %w", order.OrderNo, err)
			}
			compensated++
			mismatches++
			commonlogs.Warnf("payment callback missed, order paid by reconcile job orderNo=%s activityNo=%s transactionNo=%s",
				order.OrderNo, activity.ActivityNo, query.TransactionNo)
			continue
		}
		if order.Status == status.OrderPaid && query.PayStatus != status.PayStatusPaid {
			mismatches++
			commonlogs.Warnf("paid seckill order inconsistent with payment gateway orderNo=%s activityNo=%s payStatus=%s",
				order.OrderNo, activity.ActivityNo, query.PayStatus)
		}
	}
	return checked, compensated, mismatches, nil
}

func (r *Runner) closeActivityPendingOrders(ctx context.Context, activity entity.Activity, orders []entity.Order) (int, int, error) {
	closed := 0
	released := 0
	now := r.now()
	compensationThreshold := 15 * time.Minute // 10min 延迟消息 + 5min 容错窗口

	for _, order := range orders {
		if order.Status != status.OrderPending {
			continue
		}

		// 只处理创建超过 15 分钟仍未支付的订单（延迟队列未处理的遗漏）
		if now.Sub(order.CreatedAt) < compensationThreshold {
			continue
		}

		if err := r.orders.CloseOrder(ctx, order.OrderNo); err != nil {
			if stderrors.Is(err, errors.ErrInvalidState) || stderrors.Is(err, errors.ErrNotFound) {
				continue
			}
			return closed, released, fmt.Errorf("close order %s: %w", order.OrderNo, err)
		}
		closed++
		if err := r.releaseStock(ctx, order); err != nil {
			return closed, released, fmt.Errorf("release stock for order %s: %w", order.OrderNo, err)
		}
		released++
		if err := r.closePayment(ctx, order.OrderNo); err != nil {
			return closed, released, err
		}
		commonlogs.Infof("timeout order closed by compensation job (fallback) orderNo=%s activityNo=%s userId=%d createdAt=%s ageMinutes=%d",
			order.OrderNo, activity.ActivityNo, order.UserID, order.CreatedAt.Format(time.RFC3339), int(now.Sub(order.CreatedAt).Minutes()))
	}
	return closed, released, nil
}

func (r *Runner) queryPayments(ctx context.Context, orderNos []string) (map[string]entity.PayQueryResult, error) {
	if len(orderNos) == 0 {
		return map[string]entity.PayQueryResult{}, nil
	}
	result, err := r.payments.QueryPayments(ctx, orderNos)
	if err != nil {
		return nil, fmt.Errorf("query payments for %d orders: %w", len(orderNos), err)
	}
	return result, nil
}

func (r *Runner) listSyncedOrdersByOrderNos(ctx context.Context, orderNos []string) (map[string]entity.SyncedOrder, error) {
	if len(orderNos) == 0 {
		return map[string]entity.SyncedOrder{}, nil
	}
	result, err := r.sync.ListSyncedOrdersByOrderNos(ctx, orderNos)
	if err != nil {
		return nil, fmt.Errorf("list synced orders for %d order nos: %w", len(orderNos), err)
	}
	return result, nil
}

func (r *Runner) syncOrderRequest(order entity.Order) entity.SyncOrderRequest {
	paidAt := order.CreatedAt
	if order.PaidAt != nil {
		paidAt = *order.PaidAt
	}
	if paidAt.IsZero() {
		paidAt = r.now()
	}
	return entity.SyncOrderRequest{
		OrderNo:        order.OrderNo,
		UserID:         order.UserID,
		OrderSource:    "SECKILL",
		TotalAmount:    order.PayAmount,
		DiscountAmount: 0,
		PayAmount:      order.PayAmount,
		PaidAt:         paidAt,
		TransactionNo:  order.TransactionNo,
	}
}

func (r *Runner) loop(ctx context.Context, name string, interval time.Duration, fn func(context.Context) error) error {
	if r.cfg.RunOnStart {
		if err := fn(ctx); err != nil {
			if stderrors.Is(err, errors.ErrCircuitOpen) {
				commonlogs.Warnf("熔断器打开，跳过首轮任务 job=%s", name)
			} else {
				return err
			}
		}
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("job %s context done: %w", name, ctx.Err())
		case <-ticker.C:
			if err := fn(ctx); err != nil {
				if stderrors.Is(err, errors.ErrCircuitOpen) {
					commonlogs.Warnf("熔断器打开，跳过本轮任务 job=%s", name)
				} else {
					commonlogs.Warnf("job run failed job=%s error=%v", name, err)
				}
			}
		}
	}
}

func (r *Runner) releaseStock(ctx context.Context, order entity.Order) error {
	if r.stock == nil {
		return nil
	}
	if err := r.stock.ReleaseStock(ctx, order.ActivityNo, order.SKUNo, order.UserID, order.Quantity, order.OrderNo); err != nil {
		return fmt.Errorf("release stock for order %s: %w", order.OrderNo, err)
	}
	return nil
}

func (r *Runner) closePayment(ctx context.Context, orderNo string) error {
	if r.payments == nil {
		return nil
	}
	if err := r.payments.ClosePayment(ctx, orderNo); err != nil && !stderrors.Is(err, errors.ErrNotFound) {
		return fmt.Errorf("close payment for order %s: %w", orderNo, err)
	}
	return nil
}

func (r *Runner) evictActivity(ctx context.Context, activityNo string) {
	if r.cache == nil {
		return
	}
	if err := r.cache.EvictActivity(ctx, activityNo); err != nil {
		commonlogs.Warnf("evict activity cache after job update failed activityNo=%s error=%v", activityNo, err)
	}
}

func (r *Runner) now() time.Time {
	if r.clock != nil {
		return r.clock()
	}
	return time.Now()
}

func (cfg Config) normalize() Config {
	if cfg.ActivityStatusCheckInterval <= 0 {
		cfg.ActivityStatusCheckInterval = time.Minute
	}
	if cfg.TimeoutOrderCheckInterval <= 0 {
		cfg.TimeoutOrderCheckInterval = 1 * time.Hour
	}
	if cfg.StockReleaseCheckInterval <= 0 {
		cfg.StockReleaseCheckInterval = 10 * time.Minute
	}
	if cfg.ActivityDataCleanupInterval <= 0 {
		cfg.ActivityDataCleanupInterval = 24 * time.Hour
	}
	if cfg.ActivityDataRetention <= 0 {
		cfg.ActivityDataRetention = 24 * time.Hour
	}
	if cfg.RiskUserCleanupInterval <= 0 {
		cfg.RiskUserCleanupInterval = 24 * time.Hour
	}
	if cfg.DailyStatisticsInterval <= 0 {
		cfg.DailyStatisticsInterval = 24 * time.Hour
	}
	if cfg.CacheWarmupInterval <= 0 {
		cfg.CacheWarmupInterval = 5 * time.Minute
	}
	if cfg.CacheRefreshInterval <= 0 {
		cfg.CacheRefreshInterval = time.Minute
	}
	if cfg.CacheWarmupAhead <= 0 {
		cfg.CacheWarmupAhead = 10 * time.Minute
	}
	if cfg.PaymentReconcileInterval <= 0 {
		cfg.PaymentReconcileInterval = 30 * time.Minute
	}
	if cfg.OrderSyncCheckInterval <= 0 {
		cfg.OrderSyncCheckInterval = 5 * time.Minute
	}
	return cfg
}

// -- Pure helper functions --

func nextActivityStatus(activity entity.Activity, now time.Time) (int, bool) {
	switch activity.Status {
	case status.ActivityPending:
		if activity.StartTime.IsZero() || activity.StartTime.After(now) {
			return 0, false
		}
		if activity.EndTime.IsZero() || activity.EndTime.After(now) {
			return status.ActivityOpen, true
		}
		return status.ActivityEnded, true
	case status.ActivityOpen:
		if !activity.EndTime.IsZero() && !activity.EndTime.After(now) {
			return status.ActivityEnded, true
		}
	}
	return 0, false
}

func isEndedActivity(activity entity.Activity, now time.Time) bool {
	return activity.Status == status.ActivityEnded || (!activity.EndTime.IsZero() && !activity.EndTime.After(now))
}

func shouldReconcileActivity(activity entity.Activity, now time.Time) bool {
	return activity.Status == status.ActivityOpen || isEndedActivity(activity, now)
}

func shouldCleanupActivityData(activity entity.Activity, threshold time.Time) bool {
	return activity.Status == status.ActivityEnded && !activity.EndTime.IsZero() && activity.EndTime.Before(threshold)
}

func isActiveActivity(activity entity.Activity, now time.Time) bool {
	return activity.Status == status.ActivityOpen && !activity.StartTime.After(now) && activity.EndTime.After(now)
}

func isUpcomingActivity(activity entity.Activity, now time.Time, deadline time.Time) bool {
	return activity.Status == status.ActivityPending && activity.StartTime.After(now) && !activity.StartTime.After(deadline)
}

func skuNos(activity entity.Activity) []string {
	values := make([]string, 0, len(activity.SKUs))
	for _, sku := range activity.SKUs {
		if sku.SKUNo != "" {
			values = append(values, sku.SKUNo)
		}
	}
	return values
}

func activityNos(activities []entity.Activity) []string {
	values := make([]string, 0, len(activities))
	for _, activity := range activities {
		if activity.ActivityNo != "" {
			values = append(values, activity.ActivityNo)
		}
	}
	return values
}

func paymentOrderNos(activities []entity.Activity, ordersByActivity map[string][]entity.Order) []string {
	return orderNosFor(activities, ordersByActivity, func(order entity.Order) bool {
		return order.Status == status.OrderPending || order.Status == status.OrderPaid
	})
}

func paidOrderNos(activities []entity.Activity, ordersByActivity map[string][]entity.Order) []string {
	return orderNosFor(activities, ordersByActivity, func(order entity.Order) bool {
		return order.Status == status.OrderPaid
	})
}

func orderNosFor(activities []entity.Activity, ordersByActivity map[string][]entity.Order, include func(entity.Order) bool) []string {
	values := make([]string, 0)
	seen := make(map[string]struct{})
	for _, activity := range activities {
		for _, order := range ordersByActivity[activity.ActivityNo] {
			if order.OrderNo == "" || !include(order) {
				continue
			}
			if _, ok := seen[order.OrderNo]; ok {
				continue
			}
			seen[order.OrderNo] = struct{}{}
			values = append(values, order.OrderNo)
		}
	}
	return values
}

func totalInitStock(skus []entity.SKU) int64 {
	var total int64
	for _, sku := range skus {
		total += sku.TotalStock
	}
	return total
}
