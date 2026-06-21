// Package handler 提供 HTTP 请求处理器
package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"seckill-gateway-service/internal/application"
)

// PaymentHandler 处理支付相关的 HTTP 路由
type PaymentHandler struct {
	app *application.PaymentApp
}

// NewPaymentHandler 创建新的支付处理器
func NewPaymentHandler(app *application.PaymentApp) *PaymentHandler {
	return &PaymentHandler{app: app}
}

// Register 在 gin 引擎上注册支付路由
func (h *PaymentHandler) Register(router *gin.Engine) {
	pay := router.Group("/api/pay")
	pay.POST("/prepay", requireUser(), h.prepay)
	pay.POST("/notify/:channel", h.notify)
}

// prepayRequest 预支付请求
type prepayRequest struct {
	OrderNo    string `json:"orderNo"`
	PayChannel string `json:"payChannel"`
}

// notifyRequest 通知请求
type notifyRequest struct {
	OrderNo       string `json:"orderNo"`
	TransactionNo string `json:"transactionNo"`
}

// prepay 处理预支付请求
func (h *PaymentHandler) prepay(c *gin.Context) {
	uid := userID(c)
	orderNo := strings.TrimSpace(c.Query("orderNo"))
	payChannel := strings.TrimSpace(c.Query("payChannel"))
	if orderNo == "" {
		var payload prepayRequest
		if err := c.ShouldBindJSON(&payload); err == nil {
			orderNo = strings.TrimSpace(payload.OrderNo)
			payChannel = strings.TrimSpace(payload.PayChannel)
		}
	}
	if orderNo == "" {
		fail(c, http.StatusBadRequest, "bad_request", "缺少orderNo参数")
		return
	}
	result, err := h.app.Prepay(c.Request.Context(), uid, orderNo, payChannel)
	if err != nil {
		if handleRPCFailure(c, err) {
			return
		}
		fail(c, http.StatusInternalServerError, "payment_error", err.Error())
		return
	}
	ok(c, result)
}

// notify 处理支付回调通知
func (h *PaymentHandler) notify(c *gin.Context) {
	channel := strings.TrimSpace(c.Param("channel"))
	orderNo := strings.TrimSpace(c.PostForm("order_no"))
	transactionNo := strings.TrimSpace(c.PostForm("transaction_no"))
	if orderNo == "" {
		orderNo = strings.TrimSpace(c.Query("order_no"))
	}
	if transactionNo == "" {
		transactionNo = strings.TrimSpace(c.Query("transaction_no"))
	}
	if orderNo == "" || transactionNo == "" {
		var payload notifyRequest
		if err := c.ShouldBindJSON(&payload); err == nil {
			if orderNo == "" {
				orderNo = strings.TrimSpace(payload.OrderNo)
			}
			if transactionNo == "" {
				transactionNo = strings.TrimSpace(payload.TransactionNo)
			}
		}
	}
	if orderNo == "" {
		c.String(http.StatusBadRequest, "MISSING_ORDER_NO")
		return
	}
	if transactionNo == "" {
		c.String(http.StatusBadRequest, "MISSING_TRANSACTION_NO")
		return
	}
	if err := h.app.Notify(c.Request.Context(), orderNo, transactionNo, channel); err != nil {
		c.String(http.StatusInternalServerError, "FAIL")
		return
	}
	c.String(http.StatusOK, "SUCCESS")
}
