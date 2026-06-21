package handler

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"seckill-gateway-service/internal/application"
)

func setupAdminRouter(activity *application.ActivityAdminApp, order *application.OrderAdminApp, stock *application.StockAdminApp) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Request.Header.Set("X-User-Role", "admin")
		c.Next()
	})
	NewAdminHandler(activity, order, stock).Register(router)
	return router
}

func TestAdminHandler_Activities(t *testing.T) {
	gin.SetMode(gin.TestMode)
	activityGW := &fakeAdminActivityGateway{
		list: []application.ActivityItem{
			{ActivityNo: "A1", ActivityName: "Flash Sale"},
			{ActivityNo: "A2", ActivityName: "Mid Sale"},
		},
	}
	activityApp := application.NewActivityAdminApp(activityGW, slog.Default())
	router := setupAdminRouter(activityApp, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/activities", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}
	body := resp.Body.String()
	if !strings.Contains(body, "A1") || !strings.Contains(body, "Flash Sale") {
		t.Fatalf("body missing activity data: %s", body)
	}
}

func TestAdminHandler_GetActivity_NotFound(t *testing.T) {
	activityGW := &fakeAdminActivityGateway{detail: nil}
	activityApp := application.NewActivityAdminApp(activityGW, slog.Default())
	router := setupAdminRouter(activityApp, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/activities/NOTFOUND", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusNotFound)
	}
}

func TestAdminHandler_CreateActivity(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		activityGW := &fakeAdminActivityGateway{
			detail: &application.ActivityDetail{ActivityNo: "NEW1"},
		}
		activityApp := application.NewActivityAdminApp(activityGW, slog.Default())
		router := setupAdminRouter(activityApp, nil, nil)

		body := `{"activityName":"Flash Sale","startTime":"2024-01-01T00:00:00Z","endTime":"2024-12-31T23:59:59Z"}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/activities", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d: %s", resp.Code, http.StatusOK, resp.Body.String())
		}
		if !activityGW.createCalled {
			t.Fatal("expected CreateActivity to be called")
		}
	})

	t.Run("rejects missing name", func(t *testing.T) {
		activityGW := &fakeAdminActivityGateway{}
		activityApp := application.NewActivityAdminApp(activityGW, slog.Default())
		router := setupAdminRouter(activityApp, nil, nil)

		body := `{"startTime":"2024-01-01T00:00:00Z","endTime":"2024-12-31T23:59:59Z"}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/activities", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		if resp.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", resp.Code, http.StatusBadRequest)
		}
	})
}

func TestAdminHandler_UpdateActivity(t *testing.T) {
	activityGW := &fakeAdminActivityGateway{}
	activityApp := application.NewActivityAdminApp(activityGW, slog.Default())
	router := setupAdminRouter(activityApp, nil, nil)

	body := `{"activityName":"Updated Name","purchaseLimit":5}`
	req := httptest.NewRequest(http.MethodPut, "/api/admin/activities/A1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}
	if !activityGW.updateCalled || activityGW.updateReq.ActivityNo != "A1" {
		t.Fatal("expected UpdateActivity called with A1")
	}
}

func TestAdminHandler_EndActivity(t *testing.T) {
	activityGW := &fakeAdminActivityGateway{}
	activityApp := application.NewActivityAdminApp(activityGW, slog.Default())
	router := setupAdminRouter(activityApp, nil, nil)

	req := httptest.NewRequest(http.MethodPut, "/api/admin/activities/A1/end", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}
	if !activityGW.endCalled || activityGW.endActivityNo != "A1" {
		t.Fatal("expected EndActivity called with A1")
	}
}

func TestAdminHandler_AddProduct(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		activityGW := &fakeAdminActivityGateway{}
		activityApp := application.NewActivityAdminApp(activityGW, slog.Default())
		router := setupAdminRouter(activityApp, nil, nil)

		body := `{"skuNo":"SKU1","productName":"Product 1","activityStock":100}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/activities/A1/products", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
		}
		if !activityGW.addProductCalled || activityGW.addProductReq.SKUNo != "SKU1" {
			t.Fatal("expected AddProduct called with SKU1")
		}
	})

	t.Run("rejects missing skuNo", func(t *testing.T) {
		activityGW := &fakeAdminActivityGateway{}
		activityApp := application.NewActivityAdminApp(activityGW, slog.Default())
		router := setupAdminRouter(activityApp, nil, nil)

		body := `{"activityStock":100}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/activities/A1/products", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		if resp.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", resp.Code, http.StatusBadRequest)
		}
	})
}

func TestAdminHandler_RemoveProduct(t *testing.T) {
	activityGW := &fakeAdminActivityGateway{}
	activityApp := application.NewActivityAdminApp(activityGW, slog.Default())
	router := setupAdminRouter(activityApp, nil, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/admin/activities/A1/products/SKU1", http.NoBody)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}
	if !activityGW.removeProductCalled {
		t.Fatal("expected RemoveProduct called")
	}
}

func TestAdminHandler_ListOrders(t *testing.T) {
	t.Run("by activity", func(t *testing.T) {
		orderGW := &fakeAdminOrderGateway{
			orders: []application.OrderDetail{
				{OrderNo: "O1", ActivityNo: "A1"},
			},
		}
		orderApp := application.NewOrderAdminApp(orderGW, slog.Default())
		router := setupAdminRouter(nil, orderApp, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/admin/orders?activityNo=A1", nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
		}
		if !orderGW.listByActivityCalled {
			t.Fatal("expected ListOrdersByActivity called")
		}
	})

	t.Run("rejects missing params", func(t *testing.T) {
		orderGW := &fakeAdminOrderGateway{}
		orderApp := application.NewOrderAdminApp(orderGW, slog.Default())
		router := setupAdminRouter(nil, orderApp, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/admin/orders", nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		if resp.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", resp.Code, http.StatusBadRequest)
		}
	})
}

func TestAdminHandler_GetOrder(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		orderGW := &fakeAdminOrderGateway{
			order: &application.OrderDetail{OrderNo: "O1", Status: "paid"},
		}
		orderApp := application.NewOrderAdminApp(orderGW, slog.Default())
		router := setupAdminRouter(nil, orderApp, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/admin/orders/O1", nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
		}
	})

	t.Run("not found", func(t *testing.T) {
		orderGW := &fakeAdminOrderGateway{order: nil}
		orderApp := application.NewOrderAdminApp(orderGW, slog.Default())
		router := setupAdminRouter(nil, orderApp, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/admin/orders/NOTFOUND", nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		if resp.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", resp.Code, http.StatusNotFound)
		}
	})
}

func TestAdminHandler_CloseOrder(t *testing.T) {
	orderGW := &fakeAdminOrderGateway{}
	orderApp := application.NewOrderAdminApp(orderGW, slog.Default())
	router := setupAdminRouter(nil, orderApp, nil)

	req := httptest.NewRequest(http.MethodPut, "/api/admin/orders/O1/close", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}
	if !orderGW.closeCalled {
		t.Fatal("expected CloseOrder called")
	}
}

func TestAdminHandler_PeekStock(t *testing.T) {
	stockGW := &fakeAdminStockGateway{stock: 100}
	stockApp := application.NewStockAdminApp(stockGW, slog.Default())
	router := setupAdminRouter(nil, nil, stockApp)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/stock/A1/SKU1", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}
	if !stockGW.peekCalled {
		t.Fatal("expected Peek called")
	}
	if !strings.Contains(resp.Body.String(), "100") {
		t.Fatalf("body missing stock 100: %s", resp.Body.String())
	}
}

// ========== Fakes ==========

type fakeAdminActivityGateway struct {
	list                    []application.ActivityItem
	detail                  *application.ActivityDetail
	createCalled            bool
	createReq               application.CreateActivityRequest
	updateCalled            bool
	updateReq               application.UpdateActivityRequest
	endCalled               bool
	endActivityNo           string
	addProductCalled        bool
	addProductReq           application.AddProductRequest
	removeProductCalled     bool
	removeProductActivityNo string
	removeProductSkuNo      string
}

func (g *fakeAdminActivityGateway) ListActivities(context.Context) (application.ActivityList, error) {
	return application.ActivityList{Activities: g.list}, nil
}

func (g *fakeAdminActivityGateway) GetActivity(context.Context, string) (*application.ActivityDetail, error) {
	if g.detail == nil {
		return nil, errNotFound
	}
	return g.detail, nil
}

func (g *fakeAdminActivityGateway) CreateActivity(_ context.Context, req application.CreateActivityRequest) (*application.ActivityDetail, error) {
	g.createCalled = true
	g.createReq = req
	return g.detail, nil
}

func (g *fakeAdminActivityGateway) UpdateActivity(_ context.Context, req application.UpdateActivityRequest) error {
	g.updateCalled = true
	g.updateReq = req
	return nil
}

func (g *fakeAdminActivityGateway) EndActivity(_ context.Context, activityNo string) error {
	g.endCalled = true
	g.endActivityNo = activityNo
	return nil
}

func (g *fakeAdminActivityGateway) AddProduct(_ context.Context, req application.AddProductRequest) error {
	g.addProductCalled = true
	g.addProductReq = req
	return nil
}

func (g *fakeAdminActivityGateway) RemoveProduct(_ context.Context, activityNo, skuNo string) error {
	g.removeProductCalled = true
	g.removeProductActivityNo = activityNo
	g.removeProductSkuNo = skuNo
	return nil
}

type fakeAdminOrderGateway struct {
	orders               []application.OrderDetail
	order                *application.OrderDetail
	getCalled            bool
	getOrderNo           string
	listByActivityCalled bool
	listByActivityNo     string
	listByUserCalled     bool
	listByUserID         int64
	closeCalled          bool
	closeOrderNo         string
}

func (g *fakeAdminOrderGateway) GetOrder(_ context.Context, orderNo string) (*application.OrderDetail, error) {
	g.getCalled = true
	g.getOrderNo = orderNo
	if g.order == nil {
		return nil, errNotFound
	}
	return g.order, nil
}

func (g *fakeAdminOrderGateway) ListOrdersByUser(_ context.Context, userID int64) ([]application.OrderDetail, error) {
	g.listByUserCalled = true
	g.listByUserID = userID
	return g.orders, nil
}

func (g *fakeAdminOrderGateway) ListOrdersByActivity(_ context.Context, activityNo string) ([]application.OrderDetail, error) {
	g.listByActivityCalled = true
	g.listByActivityNo = activityNo
	return g.orders, nil
}

func (g *fakeAdminOrderGateway) CreateOrder(context.Context, application.CreateOrderRequest) error { return nil }
func (g *fakeAdminOrderGateway) MarkPaid(context.Context, string, string, time.Time) error         { return nil }
func (g *fakeAdminOrderGateway) CloseOrder(_ context.Context, orderNo string) error {
	g.closeCalled = true
	g.closeOrderNo = orderNo
	return nil
}

type fakeAdminStockGateway struct {
	stock          int64
	peekCalled     bool
	peekActivityNo string
	peekSkuNo      string
}

func (g *fakeAdminStockGateway) Peek(_ context.Context, activityNo, skuNo string) (int64, error) {
	g.peekCalled = true
	g.peekActivityNo = activityNo
	g.peekSkuNo = skuNo
	return g.stock, nil
}

func (g *fakeAdminStockGateway) Deduct(context.Context, application.DeductRequest) (bool, error) {
	return true, nil
}

func (g *fakeAdminStockGateway) Release(context.Context, string, string, string, int) error { return nil }

var errNotFound = notFoundError{}

type notFoundError struct{}

func (notFoundError) Error() string { return "not found" }
