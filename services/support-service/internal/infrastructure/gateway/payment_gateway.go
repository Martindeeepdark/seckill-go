// Package gateway 提供外部服务的网关实现
package gateway

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	paymentdomain "seckill-support-service/internal/domain/entity"
	supportstatus "seckill-support-service/internal/domain/status"
	"seckill-support-service/internal/infrastructure/store"

	"github.com/Martindeeepdark/go-common/snowflake"
)

// MockPaymentGateway 模拟支付网关（用于测试）
type MockPaymentGateway struct {
	mu       sync.Mutex
	payments map[string]paymentdomain.Payment
}

// NewMockPaymentGateway 创建模拟支付网关
func NewMockPaymentGateway() *MockPaymentGateway {
	return &MockPaymentGateway{payments: map[string]paymentdomain.Payment{}}
}

// CreatePayment 创建支付
func (g *MockPaymentGateway) CreatePayment(_ context.Context, req paymentdomain.CreatePayRequest) (paymentdomain.PayResult, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	// 幂等处理：如果订单已存在且未关闭，返回原有结果
	if p, ok := g.payments[req.OrderNo]; ok && p.Status != supportstatus.PayStatusClosed {
		return p.Result, nil
	}
	// 生成支付结果
	nonce := strconv.FormatInt(snowflake.NewID(), 10)
	r := paymentdomain.PayResult{OrderNo: req.OrderNo, PayChannel: req.PayChannel, PrepayID: fmt.Sprintf("prepay_%d", snowflake.NewID()), NonceStr: nonce, TimeStamp: strconv.FormatInt(time.Now().Unix(), 10), Sign: "mock_sign_" + nonce}
	g.payments[req.OrderNo] = paymentdomain.Payment{Request: req, Result: r, Status: supportstatus.PayStatusPending, CreatedAt: time.Now()}
	return r, nil
}

// QueryPayment 查询支付状态
func (g *MockPaymentGateway) QueryPayment(_ context.Context, no string) (paymentdomain.PayQueryResult, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	p, ok := g.payments[no]
	if !ok {
		return paymentdomain.PayQueryResult{}, store.ErrNotFound
	}
	return paymentdomain.PayQueryResult{OrderNo: no, PayStatus: p.Status, TransactionNo: p.TransactionNo, PaidAt: p.PaidAt}, nil
}

// QueryPayments 批量查询支付状态
func (g *MockPaymentGateway) QueryPayments(_ context.Context, nos []string) (map[string]paymentdomain.PayQueryResult, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	r := make(map[string]paymentdomain.PayQueryResult, len(nos))
	for _, no := range nos {
		if p, ok := g.payments[no]; ok {
			r[no] = paymentdomain.PayQueryResult{OrderNo: no, PayStatus: p.Status, TransactionNo: p.TransactionNo, PaidAt: p.PaidAt}
		}
	}
	return r, nil
}

// ClosePayment 关闭支付
func (g *MockPaymentGateway) ClosePayment(_ context.Context, no string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	p, ok := g.payments[no]
	if !ok {
		return store.ErrNotFound
	}
	// 已支付的订单不能关闭
	if p.Status == supportstatus.PayStatusPaid {
		return nil
	}
	// 状态转换
	u, ok := supportstatus.TransitPaymentClosed(p)
	if !ok {
		return store.ErrInvalidState
	}
	g.payments[no] = u
	return nil
}
