// Package handler 提供 HTTP 请求处理器
package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"seckill-gateway-service/internal/application"
)

const (
	headerUserID     = "X-User-Id"
	userIDContextKey = "userID"
)

// SeckillHandler 处理秒杀相关的 HTTP 路由
type SeckillHandler struct {
	app *application.SeckillApp
}

// NewSeckillHandler 创建新的秒杀处理器
func NewSeckillHandler(app *application.SeckillApp) *SeckillHandler {
	return &SeckillHandler{app: app}
}

// Register 在 gin 引擎上注册秒杀路由
func (h *SeckillHandler) Register(router *gin.Engine) {
	router.GET("/healthz", h.health)
	router.GET("/api/activities", h.activities)
	router.GET("/api/activities/:activityNo", h.activity)
	router.GET("/api/seckill/activity/:activityNo", h.seckillActivity)
	router.GET("/api/seckill/product/:activityNo", h.seckillProducts)
	router.GET("/api/orders/:orderNo", h.order)

	seckill := router.Group("/api/seckill", requireUser())
	seckill.GET("/pre-check", h.preCheck)
	seckill.POST("/part-in", h.partIn)
	seckill.POST("/queue/check", h.checkQueue)

	user := router.Group("/api/user", requireUser())
	user.GET("/orders", h.userOrders)
}

// health 处理健康检查请求
func (h *SeckillHandler) health(c *gin.Context) {
	ok(c, gin.H{"status": "ok"})
}

// activities 处理获取活动列表请求
func (h *SeckillHandler) activities(c *gin.Context) {
	list, err := h.app.Activities(c.Request.Context())
	if err != nil {
		failRPC(c, err)
		return
	}
	ok(c, list.Activities)
}

// activity 处理获取单个活动详情请求
func (h *SeckillHandler) activity(c *gin.Context) {
	activityNo := strings.TrimSpace(c.Param("activityNo"))
	detail, err := h.app.Activity(c.Request.Context(), activityNo)
	if err != nil {
		if handleRPCFailure(c, err) {
			return
		}
		fail(c, http.StatusNotFound, "not_found", "活动不存在")
		return
	}
	ok(c, detail)
}

// seckillActivity 处理获取秒杀活动请求
func (h *SeckillHandler) seckillActivity(c *gin.Context) {
	activityNo := strings.TrimSpace(c.Param("activityNo"))
	if activityNo == "active" {
		active, err := h.app.ActiveActivities(c.Request.Context())
		if err != nil {
			failRPC(c, err)
			return
		}
		ok(c, active)
		return
	}
	detail, err := h.app.Activity(c.Request.Context(), activityNo)
	if err != nil {
		if handleRPCFailure(c, err) {
			return
		}
		fail(c, http.StatusNotFound, "not_found", "活动不存在")
		return
	}
	ok(c, detail)
}

// seckillProducts 处理获取秒杀产品请求
func (h *SeckillHandler) seckillProducts(c *gin.Context) {
	activityNo := strings.TrimSpace(c.Param("activityNo"))
	detail, err := h.app.Activity(c.Request.Context(), activityNo)
	if err != nil {
		if handleRPCFailure(c, err) {
			return
		}
		fail(c, http.StatusNotFound, "not_found", "活动不存在")
		return
	}
	ok(c, detail.Products)
}

// order 处理获取订单详情请求
func (h *SeckillHandler) order(c *gin.Context) {
	orderNo := strings.TrimSpace(c.Param("orderNo"))
	order, err := h.app.Order(c.Request.Context(), orderNo)
	if err != nil {
		if handleRPCFailure(c, err) {
			return
		}
		fail(c, http.StatusNotFound, "not_found", "订单不存在")
		return
	}
	ok(c, order)
}

// userOrders 处理获取用户订单列表请求
func (h *SeckillHandler) userOrders(c *gin.Context) {
	uid := userID(c)
	orders, err := h.app.OrdersByUser(c.Request.Context(), uid)
	if err != nil {
		failRPC(c, err)
		return
	}
	ok(c, orders)
}

// preCheck 处理秒杀前检查请求
func (h *SeckillHandler) preCheck(c *gin.Context) {
	uid := userID(c)
	challenge, err := h.app.MachineChallenge(c.Request.Context(), uid)
	if err != nil {
		failRPC(c, err)
		return
	}
	ok(c, challenge)
}

// partInRequest 秒杀参与请求
type partInRequest struct {
	ActivityNo   string `json:"activityNo"`
	SkuNo        string `json:"skuNo"`
	SkuID        string `json:"skuId"`
	Quantity     int    `json:"quantity"`
	MachineToken string `json:"machineToken"`
	Random       string `json:"random"`
}

// partIn 处理秒杀参与请求
func (h *SeckillHandler) partIn(c *gin.Context) {
	uid := userID(c)
	var req partInRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "bad_request", "请求参数错误")
		return
	}
	if req.SkuNo == "" {
		req.SkuNo = req.SkuID
	}
	if req.MachineToken == "" {
		req.MachineToken = req.Random
	}
	if req.ActivityNo == "" || req.SkuNo == "" {
		fail(c, http.StatusBadRequest, "bad_request", "缺少activityNo或skuNo")
		return
	}
	if req.Quantity <= 0 {
		req.Quantity = 1
	}
	requestIP := c.ClientIP()
	runID := strings.TrimSpace(c.GetHeader("X-Smoke-Run-Id"))
	result, err := h.app.PartIn(
		c.Request.Context(),
		uid,
		req.ActivityNo,
		req.SkuNo,
		requestIP,
		req.MachineToken,
		req.Quantity,
			runID,
	)
	if err != nil {
		failRPC(c, err)
		return
	}
	ok(c, result)
}

// queueCheckRequest 队列检查请求
type queueCheckRequest struct {
	Token      string `json:"token"`
	TraceID    string `json:"traceId"`
	ActivityNo string `json:"activityNo"`
}

// checkQueue 处理队列检查请求
func (h *SeckillHandler) checkQueue(c *gin.Context) {
	uid := userID(c)
	var req queueCheckRequest
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, http.StatusBadRequest, "bad_request", "请求参数错误")
			return
		}
	}
	traceID := strings.TrimSpace(c.Query("traceId"))
	if traceID == "" {
		traceID = strings.TrimSpace(req.TraceID)
	}
	if traceID != "" {
		result, err := h.app.CheckQueue(c.Request.Context(), traceID)
		if err != nil {
			failRPC(c, err)
			return
		}
		ok(c, result)
		return
	}

	activityNo := strings.TrimSpace(c.Query("activityNo"))
	if activityNo == "" {
		activityNo = strings.TrimSpace(req.ActivityNo)
	}
	if activityNo == "" {
		fail(c, http.StatusBadRequest, "bad_request", "缺少traceId参数")
		return
	}
	orderNo, err := h.app.CheckQueueByActivity(c.Request.Context(), uid, activityNo)
	if err != nil {
		failRPC(c, err)
		return
	}
	ok(c, gin.H{"orderNo": orderNo, "found": orderNo != ""})
}

// requireUser 是提取或验证用户 ID 的中间件
func requireUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		value, exists := c.Get(userIDContextKey)
		if exists {
			if userID, ok := value.(int64); ok && userID > 0 {
				c.Next()
				return
			}
		}
		header := strings.TrimSpace(c.GetHeader(headerUserID))
		parsed, err := strconv.ParseInt(header, 10, 64)
		if header == "" || err != nil || parsed <= 0 {
			fail(c, http.StatusUnauthorized, "not_login", "缺少或非法的X-User-Id")
			c.Abort()
			return
		}
		c.Set(userIDContextKey, parsed)
		c.Next()
	}
}

// userID 从上下文获取用户 ID
func userID(c *gin.Context) int64 {
	value, _ := c.Get(userIDContextKey)
	parsed, ok := value.(int64)
	if !ok {
		return 0
	}
	return parsed
}
