# Seckill Common - 秒杀系统公共库

## 模块说明

### domain
领域层基础抽象：
- `AggregateRoot` - 聚合根基类
- `DomainEvent` - 领域事件接口
- `BaseEvent` - 领域事件基础实现

### eventbus
统一事件总线：
- `Bus` 接口 - 统一的事件发布订阅接口
- `local.LocalBus` - 进程内事件总线
- `nats.NATSBus` - 基于 NATS JetStream 的跨服务事件总线
- `hybrid.HybridBus` - 混合事件总线（本地 + 远程）

## 使用示例

### 1. 定义领域事件

```go
package event

import "seckill-common/domain"

type OrderCreatedEvent struct {
    domain.BaseEvent
    OrderNo string
    UserID  int64
}

func (e *OrderCreatedEvent) EventName() string {
    return "order.created"
}
```

### 2. 聚合根记录事件

```go
type Order struct {
    domain.AggregateRoot
    OrderNo string
    Status  string
}

func (o *Order) Create(orderNo string) {
    o.OrderNo = orderNo
    o.Status = "PENDING"

    // 记录领域事件
    o.RecordEvent(&OrderCreatedEvent{
        BaseEvent: domain.NewBaseEvent(orderNo),
        OrderNo:   orderNo,
    })
}
```

### 3. 应用层发布事件

```go
func (s *OrderService) CreateOrder(ctx context.Context, orderNo string) error {
    order := &Order{}
    order.Create(orderNo)

    // 保存聚合
    if err := s.repo.Save(ctx, order); err != nil {
        return err
    }

    // 发布事件
    for _, event := range order.GetUncommittedEvents() {
        s.eventBus.Publish(ctx, event)
    }
    order.ClearEvents()

    return nil
}
```

## 测试

```bash
# 运行所有测试
go test ./... -v

# 运行特定模块测试
go test ./domain -v
go test ./eventbus/local -v
```
