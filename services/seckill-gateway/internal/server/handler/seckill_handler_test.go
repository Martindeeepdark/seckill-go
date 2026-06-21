package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"seckill-gateway-service/internal/application"
	"seckill-gateway-service/internal/config"
)

func TestSeckillHandlerPreCheckReturnsJavaMachineChallenge(t *testing.T) {
	gin.SetMode(gin.TestMode)
	checker := &recordingMachineChecker{
		challenge: application.MachineChallenge{Result: "ABCD", Key: 1001},
	}
	app := application.NewSeckillApp(
		config.GatewayConfig{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		checker,
		nil,
		slog.Default(),
	)
	router := gin.New()
	NewSeckillHandler(app).Register(router)

	req := httptest.NewRequest(http.MethodGet, "/api/seckill/pre-check", nil)
	req.Header.Set(headerUserID, "7")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s, want 200", resp.Code, resp.Body.String())
	}
	if checker.challengeUserID != 7 {
		t.Fatalf("challenge userID = %d, want 7", checker.challengeUserID)
	}
	var body struct {
		Data application.MachineChallenge `json:"data"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Data.Result != "ABCD" || body.Data.Key != 1001 {
		t.Fatalf("data = %+v, want ABCD/1001", body.Data)
	}
}

func TestSeckillHandlerPartInAcceptsJavaRandomAndSkuID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	checker := &recordingMachineChecker{}
	queue := &handlerQueuePublisher{}
	app := application.NewSeckillApp(
		config.GatewayConfig{MachineCheck: config.MachineCheckConfig{Enabled: true}},
		&handlerActivityGateway{detail: &application.ActivityDetail{ActivityNo: "A1", ActivityOpen: true}},
		&handlerStockGateway{stock: 10},
		nil,
		nil,
		queue,
		nil,
		checker,
		nil,
		slog.Default(),
	)
	router := gin.New()
	NewSeckillHandler(app).Register(router)

	body := []byte(`{"activityNo":"A1","skuId":"S1","quantity":1,"random":"AzCD"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/seckill/part-in", bytes.NewReader(body))
	req.Header.Set(headerUserID, "7")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s, want 200", resp.Code, resp.Body.String())
	}
	if checker.checkedUserID != 7 || checker.checkedToken != "AzCD" {
		t.Fatalf("checked = user %d token %q, want 7/AzCD", checker.checkedUserID, checker.checkedToken)
	}
	if len(queue.events) != 1 || queue.events[0].SkuNo != "S1" {
		t.Fatalf("queued events = %+v, want sku S1", queue.events)
	}
}

func TestSeckillHandlerPartInBusinessRejectReturnsJavaDataCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	checker := &recordingMachineChecker{reject: true}
	app := application.NewSeckillApp(
		config.GatewayConfig{MachineCheck: config.MachineCheckConfig{Enabled: true}},
		&handlerActivityGateway{detail: &application.ActivityDetail{ActivityNo: "A1", ActivityOpen: true}},
		&handlerStockGateway{stock: 10},
		nil,
		nil,
		&handlerQueuePublisher{},
		nil,
		checker,
		nil,
		slog.Default(),
	)
	router := gin.New()
	NewSeckillHandler(app).Register(router)

	body := []byte(`{"activityNo":"A1","skuId":"S1","quantity":1,"random":"bad"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/seckill/part-in", bytes.NewReader(body))
	req.Header.Set(headerUserID, "7")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s, want 200", resp.Code, resp.Body.String())
	}
	var envelope struct {
		Code string                   `json:"code"`
		Data application.PartInResult `json:"data"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Code != "success" {
		t.Fatalf("envelope = %+v, want success envelope", envelope)
	}
	if envelope.Data.Code != "1" || envelope.Data.Message != "当前请求人数较多，请稍后重试" || envelope.Data.SKUNo != "S1" {
		t.Fatalf("data = %+v, want Java business fail code/message/skuId", envelope.Data)
	}
}

type recordingMachineChecker struct {
	challenge       application.MachineChallenge
	challengeUserID int64
	checkedUserID   int64
	checkedToken    string
	reject          bool
}

func (c *recordingMachineChecker) Challenge(_ context.Context, userID int64) (application.MachineChallenge, error) {
	c.challengeUserID = userID
	return c.challenge, nil
}

func (c *recordingMachineChecker) Check(_ context.Context, userID int64, token string) bool {
	c.checkedUserID = userID
	c.checkedToken = token
	return !c.reject
}

type handlerActivityGateway struct {
	detail *application.ActivityDetail
}

func (g *handlerActivityGateway) ListActivities(context.Context) (application.ActivityList, error) {
	return application.ActivityList{}, nil
}

func (g *handlerActivityGateway) GetActivity(context.Context, string) (*application.ActivityDetail, error) {
	return g.detail, nil
}

func (g *handlerActivityGateway) CreateActivity(context.Context, application.CreateActivityRequest) (*application.ActivityDetail, error) {
	return g.detail, nil
}

func (g *handlerActivityGateway) UpdateActivity(context.Context, application.UpdateActivityRequest) error {
	return nil
}

func (g *handlerActivityGateway) EndActivity(context.Context, string) error {
	return nil
}

func (g *handlerActivityGateway) AddProduct(context.Context, application.AddProductRequest) error {
	return nil
}

func (g *handlerActivityGateway) RemoveProduct(context.Context, string, string) error {
	return nil
}

type handlerStockGateway struct {
	stock int64
}

func (g *handlerStockGateway) Peek(context.Context, string, string) (int64, error) {
	return g.stock, nil
}

func (g *handlerStockGateway) Deduct(context.Context, application.DeductRequest) (bool, error) {
	return true, nil
}

func (g *handlerStockGateway) Release(context.Context, string, string, string, int) error {
	return nil
}

type handlerQueuePublisher struct {
	events []application.PartInEvent
}

func (p *handlerQueuePublisher) Publish(_ context.Context, event application.PartInEvent) error {
	p.events = append(p.events, event)
	return nil
}

type handlerOrderGateway struct{}

func (handlerOrderGateway) GetOrder(context.Context, string) (*application.OrderDetail, error) {
	return nil, nil
}

func (handlerOrderGateway) ListOrdersByUser(context.Context, int64) ([]application.OrderDetail, error) {
	return nil, nil
}

func (handlerOrderGateway) ListOrdersByActivity(context.Context, string) ([]application.OrderDetail, error) {
	return nil, nil
}

func (handlerOrderGateway) CreateOrder(context.Context, application.CreateOrderRequest) error {
	return nil
}

func (handlerOrderGateway) MarkPaid(context.Context, string, string, time.Time) error {
	return nil
}

func (handlerOrderGateway) CloseOrder(context.Context, string) error {
	return nil
}
