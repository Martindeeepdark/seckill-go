package job

import (
	"context"
	"testing"
	"time"

	"seckill-job-service/internal/domain/entity"
)

func TestUpdateActivityStatuses(t *testing.T) {
	now := time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC)
	activities := []entity.Activity{
		{ActivityNo: "A1", Name: "open soon", Status: entity.ActivityPending, StartTime: now.Add(-time.Minute), EndTime: now.Add(time.Minute)},
		{ActivityNo: "A2", Name: "expired pending", Status: entity.ActivityPending, StartTime: now.Add(-time.Hour), EndTime: now.Add(-time.Minute)},
		{ActivityNo: "A3", Name: "expired active", Status: entity.ActivityOpen, StartTime: now.Add(-time.Hour), EndTime: now.Add(-time.Second)},
		{ActivityNo: "A4", Name: "future", Status: entity.ActivityPending, StartTime: now.Add(time.Hour), EndTime: now.Add(2 * time.Hour)},
	}
	activity := &fakeActivityGateway{activities: activities}
	cache := &fakeActivityCache{}
	runner := NewRunner(Config{}, activity, nil, nil, nil, nil, WithClock(func() time.Time { return now }), WithCacheInvalidator(cache))

	result, err := runner.UpdateActivityStatuses(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Activated != 1 || result.Ended != 2 {
		t.Fatalf("result = %+v, want 1 activated and 2 ended", result)
	}
	if activity.statuses["A1"] != entity.ActivityOpen {
		t.Fatalf("A1 status = %d, want open", activity.statuses["A1"])
	}
	if activity.statuses["A2"] != entity.ActivityEnded {
		t.Fatalf("A2 status = %d, want ended", activity.statuses["A2"])
	}
	if activity.statuses["A3"] != entity.ActivityEnded {
		t.Fatalf("A3 status = %d, want ended", activity.statuses["A3"])
	}
	if len(cache.evicted) != 3 {
		t.Fatalf("evicted activities = %v, want 3 entries", cache.evicted)
	}
}

func TestCloseTimeoutOrdersClosesPendingOrdersForEndedActivities(t *testing.T) {
	now := time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC)
	activity := &fakeActivityGateway{activities: []entity.Activity{
		{ActivityNo: "ended", Status: entity.ActivityEnded, EndTime: now.Add(-time.Minute)},
		{ActivityNo: "active", Status: entity.ActivityOpen, EndTime: now.Add(time.Minute)},
	}}
	orders := &fakeOrderGateway{byActivity: map[string][]entity.Order{
		"ended": {
			{OrderNo: "O1", ActivityNo: "ended", SKUNo: "S1", UserID: 7, Quantity: 2, Status: entity.OrderPending, CreatedAt: now.Add(-time.Hour)},
			{OrderNo: "O2", ActivityNo: "ended", SKUNo: "S2", UserID: 8, Quantity: 1, Status: entity.OrderPaid, CreatedAt: now.Add(-time.Hour)},
		},
		"active": {
			{OrderNo: "O3", ActivityNo: "active", SKUNo: "S3", UserID: 9, Quantity: 1, Status: entity.OrderPending, CreatedAt: now.Add(-time.Hour)},
		},
	}}
	stock := &fakeStockGateway{}
	payments := &fakePaymentGateway{}
	runner := NewRunner(Config{}, activity, orders, stock, payments, nil, WithClock(func() time.Time { return now }))

	result, err := runner.CloseTimeoutOrders(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Closed != 1 || result.StockReleased != 1 {
		t.Fatalf("result = %+v, want 1 closed and 1 released", result)
	}
	if len(orders.closed) != 1 || orders.closed[0] != "O1" {
		t.Fatalf("closed orders = %v, want [O1]", orders.closed)
	}
	if len(stock.released) != 1 {
		t.Fatalf("released stock = %+v, want 1 entry", stock.released)
	}
	if got := stock.released[0]; got.activityNo != "ended" || got.skuNo != "S1" || got.userID != 7 || got.quantity != 2 {
		t.Fatalf("released stock = %+v, want ended/S1/user 7/quantity 2", got)
	}
	if len(payments.closed) != 1 || payments.closed[0] != "O1" {
		t.Fatalf("closed payments = %v, want [O1]", payments.closed)
	}
}

func TestReleaseEndedActivityKeysCleansStockAndCache(t *testing.T) {
	now := time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC)
	activity := &fakeActivityGateway{activities: []entity.Activity{
		{
			ActivityNo: "ended",
			Status:     entity.ActivityEnded,
			EndTime:    now.Add(-time.Minute),
			SKUs: []entity.SKU{
				{SKUNo: "S1"},
				{SKUNo: "S2"},
			},
		},
		{
			ActivityNo: "active",
			Status:     entity.ActivityOpen,
			EndTime:    now.Add(time.Minute),
			SKUs:       []entity.SKU{{SKUNo: "S3"}},
		},
	}}
	stock := &fakeStockGateway{}
	cache := &fakeActivityCache{}
	runner := NewRunner(Config{}, activity, nil, stock, nil, nil, WithClock(func() time.Time { return now }), WithCacheInvalidator(cache))

	result, err := runner.ReleaseEndedActivityKeys(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Activities != 1 || result.Deleted != 2 {
		t.Fatalf("result = %+v, want 1 activity and 2 deleted", result)
	}
	if len(stock.cleaned) != 1 {
		t.Fatalf("cleaned activities = %+v, want 1 entry", stock.cleaned)
	}
	if got := stock.cleaned[0]; got.activityNo != "ended" || len(got.skuNos) != 2 || got.skuNos[0] != "S1" || got.skuNos[1] != "S2" {
		t.Fatalf("cleaned activity = %+v, want ended with [S1 S2]", got)
	}
	if len(cache.evicted) != 1 || cache.evicted[0] != "ended" {
		t.Fatalf("evicted activities = %v, want [ended]", cache.evicted)
	}
}

func TestCleanupExpiredActivityDataCleansOnlyEndedActivitiesBeyondRetention(t *testing.T) {
	now := time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC)
	activity := &fakeActivityGateway{activities: []entity.Activity{
		{ActivityNo: "old-ended", Status: entity.ActivityEnded, EndTime: now.Add(-25 * time.Hour)},
		{ActivityNo: "recent-ended", Status: entity.ActivityEnded, EndTime: now.Add(-23 * time.Hour)},
		{ActivityNo: "old-open", Status: entity.ActivityOpen, EndTime: now.Add(-25 * time.Hour)},
		{ActivityNo: "missing-end", Status: entity.ActivityEnded},
	}}
	stock := &fakeStockGateway{purchaseDeleted: map[string]int{"old-ended": 3}}
	runner := NewRunner(Config{ActivityDataRetention: 24 * time.Hour}, activity, nil, stock, nil, nil, WithClock(func() time.Time { return now }))

	result, err := runner.CleanupExpiredActivityData(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Activities != 1 || result.Deleted != 3 {
		t.Fatalf("result = %+v, want 1 activity and 3 deleted keys", result)
	}
	if len(stock.purchaseCleaned) != 1 || stock.purchaseCleaned[0] != "old-ended" {
		t.Fatalf("purchase cleaned activities = %v, want [old-ended]", stock.purchaseCleaned)
	}
}

func TestCleanupExpiredRiskUsersDelegatesToRiskGateway(t *testing.T) {
	risk := &fakeRiskGateway{deleted: 4}
	runner := NewRunner(Config{}, nil, nil, nil, nil, nil, WithRiskGateway(risk))

	result, err := runner.CleanupExpiredRiskUsers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Deleted != 4 {
		t.Fatalf("deleted = %d, want 4", result.Deleted)
	}
	if risk.calls != 1 {
		t.Fatalf("risk cleanup calls = %d, want 1", risk.calls)
	}
}

func TestDailyStatisticsCountsActivitiesAndEndedStock(t *testing.T) {
	now := time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC)
	activity := &fakeActivityGateway{activities: []entity.Activity{
		{
			ActivityNo: "active",
			Name:       "active sale",
			Status:     entity.ActivityOpen,
			EndTime:    now.Add(time.Hour),
			SKUs: []entity.SKU{
				{SKUNo: "S1", TotalStock: 10},
			},
		},
		{
			ActivityNo: "ended",
			Name:       "ended sale",
			Status:     entity.ActivityEnded,
			StartTime:  now.Add(-2 * time.Hour),
			EndTime:    now.Add(-time.Hour),
			SKUs: []entity.SKU{
				{SKUNo: "S2", TotalStock: 20},
				{SKUNo: "S3", TotalStock: 30},
			},
		},
		{
			ActivityNo: "pending",
			Name:       "future sale",
			Status:     entity.ActivityPending,
			StartTime:  now.Add(time.Hour),
		},
	}}
	runner := NewRunner(Config{}, activity, nil, nil, nil, nil, WithClock(func() time.Time { return now }))

	result, err := runner.DailyStatistics(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.ActiveCount != 1 || result.EndedCount != 1 || result.TotalCount != 2 {
		t.Fatalf("summary = %+v, want 1 active, 1 ended, total 2", result)
	}
	if len(result.Activities) != 1 {
		t.Fatalf("activity metrics = %+v, want 1 ended activity", result.Activities)
	}
	metric := result.Activities[0]
	if metric.ActivityNo != "ended" || metric.ActivityName != "ended sale" || metric.TotalInitStock != 50 || !metric.StartTime.Equal(now.Add(-2*time.Hour)) || !metric.EndTime.Equal(now.Add(-time.Hour)) {
		t.Fatalf("ended metric = %+v, want ended stock 50 with activity times", metric)
	}
}

func TestWarmupActivityCacheWarmsActiveAndSoonActivities(t *testing.T) {
	now := time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC)
	activity := &fakeActivityGateway{activities: []entity.Activity{
		{ActivityNo: "active", Status: entity.ActivityOpen, StartTime: now.Add(-time.Minute), EndTime: now.Add(time.Minute)},
		{ActivityNo: "soon", Status: entity.ActivityPending, StartTime: now.Add(5 * time.Minute), EndTime: now.Add(time.Hour)},
		{ActivityNo: "later", Status: entity.ActivityPending, StartTime: now.Add(20 * time.Minute), EndTime: now.Add(time.Hour)},
		{ActivityNo: "ended", Status: entity.ActivityEnded, StartTime: now.Add(-time.Hour), EndTime: now.Add(-time.Minute)},
	}}
	cacheWriter := &fakeActivityCacheWriter{}
	runner := NewRunner(
		Config{CacheWarmupAhead: 10 * time.Minute},
		activity,
		nil,
		nil,
		nil,
		nil,
		WithClock(func() time.Time { return now }),
		WithCacheWriter(cacheWriter),
	)

	result, err := runner.WarmupActivityCache(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Warmed != 2 || result.Active != 1 || result.Upcoming != 1 {
		t.Fatalf("result = %+v, want 2 warmed, 1 active, 1 upcoming", result)
	}
	if len(cacheWriter.warmed) != 2 || cacheWriter.warmed[0] != "active" || cacheWriter.warmed[1] != "soon" {
		t.Fatalf("warmed activities = %v, want [active soon]", cacheWriter.warmed)
	}
}

func TestRefreshActivityCacheRefreshesOnlyActiveActivities(t *testing.T) {
	now := time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC)
	activity := &fakeActivityGateway{activities: []entity.Activity{
		{ActivityNo: "active", Status: entity.ActivityOpen, StartTime: now.Add(-time.Minute), EndTime: now.Add(time.Minute)},
		{ActivityNo: "future", Status: entity.ActivityPending, StartTime: now.Add(time.Minute), EndTime: now.Add(time.Hour)},
		{ActivityNo: "ended", Status: entity.ActivityEnded, StartTime: now.Add(-time.Hour), EndTime: now.Add(-time.Minute)},
	}}
	cacheWriter := &fakeActivityCacheWriter{}
	runner := NewRunner(Config{}, activity, nil, nil, nil, nil, WithClock(func() time.Time { return now }), WithCacheWriter(cacheWriter))

	result, err := runner.RefreshActivityCache(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Refreshed != 1 || result.Active != 1 {
		t.Fatalf("result = %+v, want 1 refreshed active activity", result)
	}
	if len(cacheWriter.refreshed) != 1 || cacheWriter.refreshed[0] != "active" {
		t.Fatalf("refreshed activities = %v, want [active]", cacheWriter.refreshed)
	}
	if len(cacheWriter.activeLists) != 1 || len(cacheWriter.activeLists[0]) != 1 || cacheWriter.activeLists[0][0] != "active" {
		t.Fatalf("active lists = %v, want [[active]]", cacheWriter.activeLists)
	}
}

func TestReconcilePaymentsCompensatesMissedCallback(t *testing.T) {
	now := time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC)
	activity := &fakeActivityGateway{activities: []entity.Activity{
		{ActivityNo: "active", Status: entity.ActivityOpen, EndTime: now.Add(time.Hour)},
		{ActivityNo: "future", Status: entity.ActivityPending, StartTime: now.Add(time.Hour), EndTime: now.Add(2 * time.Hour)},
	}}
	paidAt := now.Add(-time.Minute)
	orders := &fakeOrderGateway{byActivity: map[string][]entity.Order{
		"active": {
			{OrderNo: "O1", ActivityNo: "active", UserID: 7, Status: entity.OrderPending},
			{OrderNo: "O2", ActivityNo: "active", UserID: 8, Status: entity.OrderPaid},
			{OrderNo: "O3", ActivityNo: "active", UserID: 9, Status: entity.OrderClosed},
		},
		"future": {
			{OrderNo: "O4", ActivityNo: "future", UserID: 10, Status: entity.OrderPending},
		},
	}}
	payments := &fakePaymentGateway{queries: map[string]entity.PayQueryResult{
		"O1": {OrderNo: "O1", PayStatus: entity.PayStatusPaid, TransactionNo: "T1", PaidAt: &paidAt},
		"O2": {OrderNo: "O2", PayStatus: entity.PayStatusPaid, TransactionNo: "T2", PaidAt: &paidAt},
		"O4": {OrderNo: "O4", PayStatus: entity.PayStatusPaid, TransactionNo: "T4", PaidAt: &paidAt},
	}}
	runner := NewRunner(Config{}, activity, orders, nil, payments, nil, WithClock(func() time.Time { return now }))

	result, err := runner.ReconcilePayments(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Activities != 1 || result.Checked != 2 || result.Compensated != 1 || result.MismatchCount != 1 {
		t.Fatalf("result = %+v, want 1 activity, 2 checked, 1 compensated, 1 mismatch", result)
	}
	if len(orders.paid) != 1 || orders.paid[0].orderNo != "O1" || orders.paid[0].transactionNo != "T1" {
		t.Fatalf("paid updates = %+v, want O1/T1", orders.paid)
	}
}

func TestCheckOrderSyncCompensatesMissingMainOrder(t *testing.T) {
	now := time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC)
	paidAt := now.Add(-time.Minute)
	activity := &fakeActivityGateway{activities: []entity.Activity{
		{ActivityNo: "active", Status: entity.ActivityOpen, EndTime: now.Add(time.Hour)},
		{ActivityNo: "future", Status: entity.ActivityPending, StartTime: now.Add(time.Hour), EndTime: now.Add(2 * time.Hour)},
	}}
	orders := &fakeOrderGateway{byActivity: map[string][]entity.Order{
		"active": {
			{OrderNo: "O1", ActivityNo: "active", UserID: 7, Status: entity.OrderPaid, PayAmount: 9900, TransactionNo: "T1", PaidAt: &paidAt},
			{OrderNo: "O2", ActivityNo: "active", UserID: 8, Status: entity.OrderPaid, PayAmount: 8800, TransactionNo: "T2", PaidAt: &paidAt},
			{OrderNo: "O3", ActivityNo: "active", UserID: 9, Status: entity.OrderPending, PayAmount: 7700},
		},
		"future": {
			{OrderNo: "O4", ActivityNo: "future", UserID: 10, Status: entity.OrderPaid, PayAmount: 6600, TransactionNo: "T4", PaidAt: &paidAt},
		},
	}}
	sync := &fakeOrderSyncGateway{existing: map[string]entity.SyncedOrder{
		"O2": {OrderNo: "O2"},
	}}
	runner := NewRunner(Config{}, activity, orders, nil, nil, nil, WithClock(func() time.Time { return now }), WithOrderSync(sync))

	result, err := runner.CheckOrderSync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Activities != 1 || result.Checked != 2 || result.Compensated != 1 || result.MismatchCount != 1 {
		t.Fatalf("result = %+v, want 1 activity, 2 checked, 1 compensated, 1 mismatch", result)
	}
	if len(sync.synced) != 1 {
		t.Fatalf("synced orders = %+v, want 1 entry", sync.synced)
	}
	request := sync.synced[0]
	if request.OrderNo != "O1" || request.UserID != 7 || request.OrderSource != "SECKILL" || request.PayAmount != 9900 || request.TransactionNo != "T1" || !request.PaidAt.Equal(paidAt) {
		t.Fatalf("sync request = %+v, want O1 paid order payload", request)
	}
}

// -- Fakes --

type fakeActivityGateway struct {
	activities []entity.Activity
	statuses   map[string]int
}

func (g *fakeActivityGateway) ListActivities(_ context.Context) ([]entity.Activity, error) {
	return append([]entity.Activity(nil), g.activities...), nil
}

func (g *fakeActivityGateway) UpdateActivityStatus(_ context.Context, activityNo string, status int) error {
	if g.statuses == nil {
		g.statuses = map[string]int{}
	}
	g.statuses[activityNo] = status
	for i := range g.activities {
		if g.activities[i].ActivityNo == activityNo {
			g.activities[i].Status = status
		}
	}
	return nil
}

type fakeOrderGateway struct {
	byActivity map[string][]entity.Order
	closed     []string
	paid       []paidOrder
}

type paidOrder struct {
	orderNo       string
	transactionNo string
	paidAt        time.Time
}

func (g *fakeOrderGateway) ListOrdersByActivities(_ context.Context, activityNos []string) (map[string][]entity.Order, error) {
	result := make(map[string][]entity.Order, len(activityNos))
	for _, activityNo := range activityNos {
		result[activityNo] = append([]entity.Order(nil), g.byActivity[activityNo]...)
	}
	return result, nil
}

func (g *fakeOrderGateway) CloseOrder(_ context.Context, orderNo string) error {
	g.closed = append(g.closed, orderNo)
	return nil
}

func (g *fakeOrderGateway) MarkOrderPaid(_ context.Context, orderNo string, transactionNo string, paidAt time.Time) error {
	g.paid = append(g.paid, paidOrder{orderNo: orderNo, transactionNo: transactionNo, paidAt: paidAt})
	return nil
}

type fakeStockGateway struct {
	released        []releasedStock
	cleaned         []cleanedStock
	purchaseCleaned []string
	purchaseDeleted map[string]int
}

type releasedStock struct {
	activityNo string
	skuNo      string
	userID     int64
	quantity   int
}

func (g *fakeStockGateway) ReleaseStock(_ context.Context, activityNo, skuNo string, userID int64, quantity int, _ string) error {
	g.released = append(g.released, releasedStock{activityNo: activityNo, skuNo: skuNo, userID: userID, quantity: quantity})
	return nil
}

type cleanedStock struct {
	activityNo string
	skuNos     []string
}

func (g *fakeStockGateway) CleanupActivityStock(_ context.Context, activityNo string, skuNos []string) (int, error) {
	g.cleaned = append(g.cleaned, cleanedStock{activityNo: activityNo, skuNos: append([]string(nil), skuNos...)})
	return len(skuNos), nil
}

func (g *fakeStockGateway) CleanupActivityPurchases(_ context.Context, activityNo string) (int, error) {
	g.purchaseCleaned = append(g.purchaseCleaned, activityNo)
	return g.purchaseDeleted[activityNo], nil
}

type fakePaymentGateway struct {
	closed       []string
	queries      map[string]entity.PayQueryResult
	batchQueried [][]string
}

func (g *fakePaymentGateway) ClosePayment(_ context.Context, orderNo string) error {
	g.closed = append(g.closed, orderNo)
	return nil
}

func (g *fakePaymentGateway) QueryPayments(_ context.Context, orderNos []string) (map[string]entity.PayQueryResult, error) {
	g.batchQueried = append(g.batchQueried, append([]string(nil), orderNos...))
	result := make(map[string]entity.PayQueryResult, len(orderNos))
	for _, orderNo := range orderNos {
		if query, ok := g.queries[orderNo]; ok {
			result[orderNo] = query
		}
	}
	return result, nil
}

type fakeOrderSyncGateway struct {
	existing     map[string]entity.SyncedOrder
	batchChecked [][]string
	synced       []entity.SyncOrderRequest
}

func (g *fakeOrderSyncGateway) ListSyncedOrdersByOrderNos(_ context.Context, orderNos []string) (map[string]entity.SyncedOrder, error) {
	g.batchChecked = append(g.batchChecked, append([]string(nil), orderNos...))
	result := make(map[string]entity.SyncedOrder, len(orderNos))
	for _, orderNo := range orderNos {
		if order, ok := g.existing[orderNo]; ok {
			result[orderNo] = order
		}
	}
	return result, nil
}

func (g *fakeOrderSyncGateway) SyncOrder(_ context.Context, request entity.SyncOrderRequest) error {
	g.synced = append(g.synced, request)
	return nil
}

type fakeRiskGateway struct {
	deleted int
	calls   int
}

func (g *fakeRiskGateway) CleanupExpiredRiskUsers(_ context.Context) (int, error) {
	g.calls++
	return g.deleted, nil
}

type fakeActivityCache struct {
	evicted []string
}

func (c *fakeActivityCache) EvictActivity(_ context.Context, activityNo string) error {
	c.evicted = append(c.evicted, activityNo)
	return nil
}

type fakeActivityCacheWriter struct {
	warmed      []string
	refreshed   []string
	activeLists [][]string
}

func (w *fakeActivityCacheWriter) WarmupActivity(_ context.Context, activity entity.Activity) error {
	w.warmed = append(w.warmed, activity.ActivityNo)
	return nil
}

func (w *fakeActivityCacheWriter) RefreshActivity(_ context.Context, activity entity.Activity) error {
	w.refreshed = append(w.refreshed, activity.ActivityNo)
	return nil
}

func (w *fakeActivityCacheWriter) RefreshActiveActivities(_ context.Context, activities []entity.Activity) error {
	values := make([]string, 0, len(activities))
	for _, activity := range activities {
		values = append(values, activity.ActivityNo)
	}
	w.activeLists = append(w.activeLists, values)
	return nil
}
