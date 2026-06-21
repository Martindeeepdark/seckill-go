package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/go-kratos/kratos/v2/middleware/circuitbreaker"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	"seckill-common/tracing"

	"seckill-gateway-service/internal/application"
	"seckill-gateway-service/internal/config"
	"seckill-gateway-service/internal/server"
)

func TestFailRPCMapsInfrastructureErrors(t *testing.T) {
	traceID := "11111111111111111111111111111111"
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "resource exhausted",
			err:        fmt.Errorf("wrapped rpc error: %w", grpcstatus.Error(codes.ResourceExhausted, "busy")),
			wantStatus: http.StatusTooManyRequests,
			wantCode:   "rate_limited",
		},
		{
			name:       "unavailable",
			err:        fmt.Errorf("wrapped rpc error: %w", grpcstatus.Error(codes.Unavailable, "down")),
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   "service_degraded",
		},
		{
			name:       "deadline exceeded",
			err:        fmt.Errorf("wrapped rpc error: %w", grpcstatus.Error(codes.DeadlineExceeded, "slow")),
			wantStatus: http.StatusGatewayTimeout,
			wantCode:   "service_timeout",
		},
		{
			name:       "kratos circuit breaker",
			err:        circuitbreaker.ErrNotAllowed,
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   "service_degraded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.Use(server.TraceMiddleware())
			router.GET("/rpc", func(c *gin.Context) {
				failRPC(c, tt.err)
			})

			req := httptest.NewRequest(http.MethodGet, "/rpc", nil)
			req.Header.Set(tracing.HeaderTraceID, traceID)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != tt.wantStatus {
				t.Fatalf("status = %d body = %s, want %d", resp.Code, resp.Body.String(), tt.wantStatus)
			}
			var body Result
			if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Code != tt.wantCode {
				t.Fatalf("response = %+v, want code %s failure", body, tt.wantCode)
			}
			if body.RequestTraceID != traceID {
				t.Fatalf("requestTraceId = %q, want %q", body.RequestTraceID, traceID)
			}
			if got := resp.Header().Get(tracing.HeaderTraceID); got != traceID {
				t.Fatalf("trace header = %q, want %q", got, traceID)
			}
		})
	}
}

func TestActivityDetailReturnsDegradedWhenRPCUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	app := application.NewSeckillApp(
		config.GatewayConfig{},
		unavailableActivityGateway{},
		nil,
		nil,
		nil,
		nil,
		nil,
		&application.NopMachineChecker{},
		nil, // worker pool
		slog.Default(),
	)
	router := gin.New()
	router.Use(server.TraceMiddleware())
	NewSeckillHandler(app).Register(router)

	req := httptest.NewRequest(http.MethodGet, "/api/activities/A1", nil)
	req.Header.Set(tracing.HeaderTraceID, "22222222222222222222222222222222")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d body = %s, want 503", resp.Code, resp.Body.String())
	}
	var body Result
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "service_degraded" {
		t.Fatalf("code = %q, want service_degraded", body.Code)
	}
}

type unavailableActivityGateway struct{}

func (unavailableActivityGateway) ListActivities(context.Context) (application.ActivityList, error) {
	return application.ActivityList{}, grpcstatus.Error(codes.Unavailable, "activity service unavailable")
}

func (unavailableActivityGateway) GetActivity(context.Context, string) (*application.ActivityDetail, error) {
	return nil, grpcstatus.Error(codes.Unavailable, "activity service unavailable")
}

func (unavailableActivityGateway) CreateActivity(context.Context, application.CreateActivityRequest) (*application.ActivityDetail, error) {
	return nil, grpcstatus.Error(codes.Unavailable, "activity service unavailable")
}

func (unavailableActivityGateway) UpdateActivity(context.Context, application.UpdateActivityRequest) error {
	return grpcstatus.Error(codes.Unavailable, "activity service unavailable")
}

func (unavailableActivityGateway) EndActivity(context.Context, string) error {
	return grpcstatus.Error(codes.Unavailable, "activity service unavailable")
}

func (unavailableActivityGateway) AddProduct(context.Context, application.AddProductRequest) error {
	return grpcstatus.Error(codes.Unavailable, "activity service unavailable")
}

func (unavailableActivityGateway) RemoveProduct(context.Context, string, string) error {
	return grpcstatus.Error(codes.Unavailable, "activity service unavailable")
}
