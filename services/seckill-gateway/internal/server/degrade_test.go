package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"seckill-common/tracing"
)

func TestDegradeMiddlewareOpensAfterFailuresAndKeepsTraceID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Unix(100, 0)
	controller := newDegradeController(2, time.Second, func() time.Time { return now })
	router := gin.New()
	router.Use(TraceMiddleware())
	router.Use(newDegradeMiddleware(controller))

	calls := 0
	router.GET("/unstable", func(c *gin.Context) {
		calls++
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false})
	})

	traceID := "11111111111111111111111111111111"
	for i := 0; i < 2; i++ {
		resp := performDegradeRequest(router, traceID)
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("failure %d status = %d body = %s, want 500", i+1, resp.Code, resp.Body.String())
		}
	}

	resp := performDegradeRequest(router, traceID)
	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("open status = %d body = %s, want 503", resp.Code, resp.Body.String())
	}
	if calls != 2 {
		t.Fatalf("handler calls = %d, want 2", calls)
	}

	var body degradeResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "service_degraded" || body.Success {
		t.Fatalf("response = %+v, want service_degraded failure", body)
	}
	if body.RequestTraceID != traceID {
		t.Fatalf("requestTraceId = %q, want %q", body.RequestTraceID, traceID)
	}
}

func TestDegradeMiddlewareHalfOpenSuccessClosesBreaker(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Unix(100, 0)
	controller := newDegradeController(1, time.Second, func() time.Time { return now })
	router := gin.New()
	router.Use(newDegradeMiddleware(controller))

	fail := true
	router.GET("/unstable", func(c *gin.Context) {
		if fail {
			c.JSON(http.StatusBadGateway, gin.H{"ok": false})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	resp := performDegradeRequest(router, "")
	if resp.Code != http.StatusBadGateway {
		t.Fatalf("first status = %d, want 502", resp.Code)
	}
	resp = performDegradeRequest(router, "")
	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("open status = %d, want 503", resp.Code)
	}

	now = now.Add(time.Second)
	fail = false
	resp = performDegradeRequest(router, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("half-open probe status = %d body = %s, want 200", resp.Code, resp.Body.String())
	}
	resp = performDegradeRequest(router, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("closed status = %d body = %s, want 200", resp.Code, resp.Body.String())
	}
}

func performDegradeRequest(router *gin.Engine, traceID string) *httptest.ResponseRecorder {
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/unstable", nil)
	if traceID != "" {
		req.Header.Set(tracing.HeaderTraceID, traceID)
	}
	router.ServeHTTP(resp, req)
	return resp
}

func TestDynamicDegradeMiddlewareDisabledPasses(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rc := NewGatewayRuntimeConfig()
	rc.UpdateDegrade(DegradeOptions{Enabled: false})

	router := gin.New()
	router.Use(DynamicDegradeMiddleware(rc))
	router.GET("/ok", func(c *gin.Context) { c.Status(http.StatusOK) })

	for i := 0; i < 5; i++ {
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/ok", nil))
		if resp.Code != http.StatusOK {
			t.Fatalf("request %d status = %d, want 200 (disabled)", i+1, resp.Code)
		}
	}
}

func TestDynamicDegradeMiddlewareEnabledBreaks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Unix(100, 0)

	rc := NewGatewayRuntimeConfig()
	rc.UpdateDegrade(DegradeOptions{
		Enabled:          true,
		FailureThreshold: 1,
		Timeout:          time.Second,
	})

	controller := newDegradeController(1, time.Second, func() time.Time { return now })
	router := gin.New()
	router.Use(DynamicDegradeMiddlewareWithController(rc, controller))
	router.GET("/unstable", func(c *gin.Context) {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false})
	})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/unstable", nil))
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("first status = %d, want 500", resp.Code)
	}

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/unstable", nil))
	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("open status = %d, want 503", resp.Code)
	}
}

func TestDynamicDegradeMiddlewareConfigChange(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rc := NewGatewayRuntimeConfig()
	rc.UpdateDegrade(DegradeOptions{Enabled: true, FailureThreshold: 1})

	controller := newDegradeController(1, time.Second, time.Now)
	router := gin.New()
	router.Use(DynamicDegradeMiddlewareWithController(rc, controller))
	router.GET("/unstable", func(c *gin.Context) {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false})
	})

	// Trigger failure to open breaker
	httptest.NewRecorder()
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/unstable", nil))

	// Disable degrade dynamically
	rc.UpdateDegrade(DegradeOptions{Enabled: false})

	// Now requests should pass even though breaker is open
	for i := 0; i < 3; i++ {
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/unstable", nil))
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("after disable, request %d status = %d, want 500 (handler runs)", i+1, resp.Code)
		}
	}
}
