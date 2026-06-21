package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"seckill-gateway-service/internal/server"
)

func TestConfigHandlerGetSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rc := server.NewGatewayRuntimeConfig()
	rc.UpdateRisk(true)
	rc.UpdateRateLimit(true, server.RateLimitOptions{MaxQPS: 500})

	handler := NewConfigHandler(rc, nil)

	router := gin.New()
	handler.Register(router.Group("/api/admin"))

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/config", nil)
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /config status = %d, want 200", resp.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	data, _ := body["data"].(map[string]any)
	if data == nil {
		t.Fatal("response should have data field")
	}
	if data["riskEnabled"] != true {
		t.Error("snapshot should show riskEnabled=true")
	}
}

func TestConfigHandlerPutNoEtcd(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rc := server.NewGatewayRuntimeConfig()
	handler := NewConfigHandler(rc, nil)

	router := gin.New()
	handler.Register(router.Group("/api/admin"))

	payload, _ := json.Marshal(map[string]any{
		"risk": map[string]any{"enabled": true},
	})

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/admin/config", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("PUT /config status = %d, want 200", resp.Code)
	}

	// Verify RuntimeConfig was updated via etcd=nil fallback (direct update)
	if !rc.RiskEnabled() {
		t.Error("risk should be enabled after PUT")
	}
}

func TestConfigHandlerDeleteNoEtcd(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rc := server.NewGatewayRuntimeConfig()
	rc.UpdateRisk(true)
	handler := NewConfigHandler(rc, nil)

	router := gin.New()
	handler.Register(router.Group("/api/admin"))

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/config", nil)
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("DELETE /config status = %d, want 200", resp.Code)
	}
}
