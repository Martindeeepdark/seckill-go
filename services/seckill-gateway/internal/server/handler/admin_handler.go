package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"seckill-gateway-service/internal/application"
	"seckill-gateway-service/internal/server"
)

// AdminHandler 处理管理员相关的 HTTP 路由
type AdminHandler struct {
	activityApp  *application.ActivityAdminApp
	orderApp     *application.OrderAdminApp
	stockApp     *application.StockAdminApp
	configHandler *ConfigHandler
}

// NewAdminHandler 创建新的管理员处理器
func NewAdminHandler(activity *application.ActivityAdminApp, order *application.OrderAdminApp, stock *application.StockAdminApp) *AdminHandler {
	return &AdminHandler{activityApp: activity, orderApp: order, stockApp: stock}
}

// WithConfigHandler 注入动态配置处理器。
func (h *AdminHandler) WithConfigHandler(ch *ConfigHandler) *AdminHandler {
	h.configHandler = ch
	return h
}

// Register 在 gin 引擎上注册管理员路由
func (h *AdminHandler) Register(router *gin.Engine) {
	admin := router.Group("/api/admin", server.RequireAdmin())
	admin.GET("/activities", h.activities)
	admin.GET("/activities/:activityNo", h.activity)
	admin.POST("/activities", h.createActivity)
	admin.PUT("/activities/:activityNo", h.updateActivity)
	admin.PUT("/activities/:activityNo/end", h.endActivity)
	admin.POST("/activities/:activityNo/products", h.addProduct)
	admin.DELETE("/activities/:activityNo/products/:skuNo", h.removeProduct)
	admin.GET("/orders", h.listOrders)
	admin.GET("/orders/:orderNo", h.getOrder)
	admin.PUT("/orders/:orderNo/close", h.closeOrder)
	admin.GET("/stock/:activityNo/:skuNo", h.peekStock)
	if h.configHandler != nil {
		h.configHandler.Register(admin)
	}
}

func (h *AdminHandler) activities(c *gin.Context) {
	list, err := h.activityApp.ListActivities(c.Request.Context())
	if err != nil {
		failRPC(c, err)
		return
	}
	ok(c, list.Activities)
}

func (h *AdminHandler) activity(c *gin.Context) {
	activityNo := strings.TrimSpace(c.Param("activityNo"))
	detail, err := h.activityApp.GetActivity(c.Request.Context(), activityNo)
	if err != nil {
		if handleRPCFailure(c, err) {
			return
		}
		fail(c, http.StatusNotFound, "not_found", "活动不存在")
		return
	}
	ok(c, detail)
}

func (h *AdminHandler) createActivity(c *gin.Context) {
	var req application.CreateActivityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "bad_request", "请求参数错误")
		return
	}
	detail, err := h.activityApp.CreateActivity(c.Request.Context(), req)
	if err != nil {
		if isValidationError(err) {
			fail(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		failRPC(c, err)
		return
	}
	ok(c, detail)
}

func (h *AdminHandler) updateActivity(c *gin.Context) {
	activityNo := strings.TrimSpace(c.Param("activityNo"))
	var req application.UpdateActivityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "bad_request", "请求参数错误")
		return
	}
	req.ActivityNo = activityNo
	if err := h.activityApp.UpdateActivity(c.Request.Context(), req); err != nil {
		if isValidationError(err) {
			fail(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		failRPC(c, err)
		return
	}
	ok(c, gin.H{"activityNo": activityNo})
}

func (h *AdminHandler) endActivity(c *gin.Context) {
	activityNo := strings.TrimSpace(c.Param("activityNo"))
	if err := h.activityApp.EndActivity(c.Request.Context(), activityNo); err != nil {
		if isValidationError(err) {
			fail(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		failRPC(c, err)
		return
	}
	ok(c, gin.H{"activityNo": activityNo})
}

func (h *AdminHandler) addProduct(c *gin.Context) {
	activityNo := strings.TrimSpace(c.Param("activityNo"))
	var req application.AddProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "bad_request", "请求参数错误")
		return
	}
	req.ActivityNo = activityNo
	if err := h.activityApp.AddProduct(c.Request.Context(), req); err != nil {
		if isValidationError(err) {
			fail(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		failRPC(c, err)
		return
	}
	ok(c, gin.H{"activityNo": activityNo, "skuNo": req.SKUNo})
}

func (h *AdminHandler) removeProduct(c *gin.Context) {
	activityNo := strings.TrimSpace(c.Param("activityNo"))
	skuNo := strings.TrimSpace(c.Param("skuNo"))
	if err := h.activityApp.RemoveProduct(c.Request.Context(), activityNo, skuNo); err != nil {
		failRPC(c, err)
		return
	}
	ok(c, gin.H{"activityNo": activityNo, "skuNo": skuNo})
}

func (h *AdminHandler) listOrders(c *gin.Context) {
	activityNo := strings.TrimSpace(c.Query("activityNo"))
	if activityNo != "" {
		orders, err := h.orderApp.ListOrdersByActivity(c.Request.Context(), activityNo)
		if err != nil {
			failRPC(c, err)
			return
		}
		ok(c, orders)
		return
	}
	userIDStr := strings.TrimSpace(c.Query("userId"))
	if userIDStr != "" {
		userID, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil || userID <= 0 {
			fail(c, http.StatusBadRequest, "bad_request", "userId参数错误")
			return
		}
		orders, err := h.orderApp.ListOrdersByUser(c.Request.Context(), userID)
		if err != nil {
			failRPC(c, err)
			return
		}
		ok(c, orders)
		return
	}
	fail(c, http.StatusBadRequest, "bad_request", "缺少activityNo或userId参数")
}

func (h *AdminHandler) getOrder(c *gin.Context) {
	orderNo := strings.TrimSpace(c.Param("orderNo"))
	order, err := h.orderApp.GetOrder(c.Request.Context(), orderNo)
	if err != nil {
		if handleRPCFailure(c, err) {
			return
		}
		fail(c, http.StatusNotFound, "not_found", "订单不存在")
		return
	}
	ok(c, order)
}

func (h *AdminHandler) closeOrder(c *gin.Context) {
	orderNo := strings.TrimSpace(c.Param("orderNo"))
	if err := h.orderApp.CloseOrder(c.Request.Context(), orderNo); err != nil {
		failRPC(c, err)
		return
	}
	ok(c, gin.H{"orderNo": orderNo})
}

func (h *AdminHandler) peekStock(c *gin.Context) {
	activityNo := strings.TrimSpace(c.Param("activityNo"))
	skuNo := strings.TrimSpace(c.Param("skuNo"))
	stock, err := h.stockApp.PeekStock(c.Request.Context(), activityNo, skuNo)
	if err != nil {
		failRPC(c, err)
		return
	}
	ok(c, gin.H{"activityNo": activityNo, "skuNo": skuNo, "stock": stock})
}

// isValidationError 判断是否为参数校验错误
func isValidationError(err error) bool {
	return strings.Contains(err.Error(), "is required") ||
		strings.Contains(err.Error(), "must be positive")
}
