package repository

import (
	"context"
	"errors"
	"time"

	"seckill-order-service/internal/domain/entity"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrDuplicate      = errors.New("duplicate")
	ErrOptimisticLock = errors.New("optimistic lock conflict")
)

// OrderRepository 定义订单聚合根的 DDD 仓储接口，使用乐观锁保证一致性。
type OrderRepository interface {
	// Save 保存聚合根（完整保存，使用乐观锁）
	Save(ctx context.Context, order *entity.Order) error

	// GetByOrderNo 根据订单号加载聚合根
	GetByOrderNo(ctx context.Context, orderNo string) (*entity.Order, error)

	// GetByUserID 根据用户ID加载订单列表
	GetByUserID(ctx context.Context, userID int64) ([]*entity.Order, error)

	// GetPendingOrders 获取待支付订单（用于超时关单）
	GetPendingOrders(ctx context.Context, beforeTime time.Time, limit int) ([]*entity.Order, error)
}

// OrderStore 是 RPC/基础设施层使用的订单存储端口。
// 现阶段保留旧 CRUD 形态，避免和 DDD 聚合仓储强行耦合。
type OrderStore interface {
	CreateOrder(ctx context.Context, order entity.Order) error
	GetOrder(ctx context.Context, orderNo string) (entity.Order, error)
	// GetByUserAndTrace 根据 (user_id, trace_id) 组合查询订单
	// 用于 DuplicateKey (23505) 场景下回查已存在的订单
	// 未找到时返回 ErrNotFound
	GetByUserAndTrace(ctx context.Context, userID int64, traceID string) (entity.Order, error)
	ListOrdersByActivity(ctx context.Context, activityNo string) ([]entity.Order, error)
	ListOrdersByActivities(ctx context.Context, activityNos []string) (map[string][]entity.Order, error)
	ListOrdersByUser(ctx context.Context, userID int64) ([]entity.Order, error)
	MarkOrderPaid(ctx context.Context, orderNo string, transactionNo string, paidAt time.Time) error
	CloseOrder(ctx context.Context, orderNo string) error
}
