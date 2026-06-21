// Package gateway 提供 gRPC 客户端的适配器
// 将外部 gRPC 服务适配为应用层接口
package gateway

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/registry"
	"google.golang.org/grpc"

	activityv1 "seckill-api/activity/v1"
	orderv1 "seckill-api/order/v1"
	paymentv1 "seckill-api/payment/v1"
	riskv1 "seckill-api/risk/v1"
	stockv1 "seckill-api/stock/v1"
	"seckill-common/rpcclient"
)

// Clients 包含所有 gRPC 服务客户端
type Clients struct {
	Activity activityv1.ActivityServiceClient
	Stock    stockv1.StockServiceClient
	Risk     riskv1.RiskServiceClient
	Order    orderv1.OrderServiceClient
	Payment  paymentv1.PaymentServiceClient
	conns    []*grpc.ClientConn
}

// RPCConfig 包含每个后端服务的端点地址
type RPCConfig struct {
	Activity              string
	Stock                 string
	Risk                  string
	Order                 string
	Payment               string
	Timeout               time.Duration
	CircuitBreakerEnabled bool
	Pool                  rpcclient.ConnectionPoolConfig
}

// NewClients 拨打所有后端服务并返回初始化的客户端
func NewClients(ctx context.Context, cfg RPCConfig, discovery registry.Discovery) (*Clients, error) {
	clients := &Clients{}
	dial := func(name string, endpoint string) (*grpc.ClientConn, error) {
		conn, err := rpcclient.Dial(ctx, rpcclient.Config{
			Endpoint:       endpoint,
			Timeout:        cfg.Timeout,
			CircuitBreaker: cfg.CircuitBreakerEnabled,
			Discovery:      discovery,
			Pool:           cfg.Pool,
		})
		if err != nil {
			_ = clients.Close() //nolint:errcheck // cleanup on shutdown
			return nil, fmt.Errorf("dial %s: %w", name, err)
		}
		clients.conns = append(clients.conns, conn)
		return conn, nil
	}
	activityConn, err := dial("activity", cfg.Activity)
	if err != nil {
		return nil, err
	}
	stockConn, err := dial("stock", cfg.Stock)
	if err != nil {
		return nil, err
	}
	riskConn, err := dial("risk", cfg.Risk)
	if err != nil {
		return nil, err
	}
	orderConn, err := dial("order", cfg.Order)
	if err != nil {
		return nil, err
	}
	paymentConn, err := dial("payment", cfg.Payment)
	if err != nil {
		return nil, err
	}
	clients.Activity = activityv1.NewActivityServiceClient(activityConn)
	clients.Stock = stockv1.NewStockServiceClient(stockConn)
	clients.Risk = riskv1.NewRiskServiceClient(riskConn)
	clients.Order = orderv1.NewOrderServiceClient(orderConn)
	clients.Payment = paymentv1.NewPaymentServiceClient(paymentConn)
	return clients, nil
}

// Close 关闭所有 gRPC 连接
func (c *Clients) Close() error {
	var first error
	for _, conn := range c.conns {
		if err := conn.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}
