package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"seckill-common/tracing"
)

func TestTraceMiddlewareUsesTraceParentWhenTraceIDHeaderMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	traceID := "55555555555555555555555555555555"
	traceparent := "00-" + traceID + "-6666666666666666-01"
	router := gin.New()
	router.Use(TraceMiddleware())
	router.GET("/ok", func(c *gin.Context) {
		if got := tracing.TraceID(c.Request.Context()); got != traceID {
			t.Fatalf("context traceID = %q, want %q", got, traceID)
		}
		c.Status(http.StatusOK)
	})

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	req.Header.Set(tracing.HeaderTraceParent, traceparent)
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	if got := resp.Header().Get(tracing.HeaderTraceID); got != traceID {
		t.Fatalf("response traceID = %q, want %q", got, traceID)
	}
}
