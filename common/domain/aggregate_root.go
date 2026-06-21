package domain

// AggregateRoot 聚合根基类，提供领域事件管理和版本控制
//
// AggregateRoot 是 DDD（领域驱动设计）中的核心概念，用于维护聚合的一致性边界。
// 它负责：
//   - 管理未提交的领域事件（通过事件 sourcing 实现审计和重放）
//   - 提供版本控制以支持乐观锁并发控制
//   - 作为所有聚合根的基类，通过嵌入方式复用
//
// 使用示例：
//
//	type OrderAggregate struct {
//	    domain.AggregateRoot
//	    order *Order
//	}
//
//	func (o *OrderAggregate) Create() {
//	    o.order = &Order{ID: uuid.New()}
//	    o.RecordEvent(&OrderCreatedEvent{OrderID: o.order.ID})
//	}
type AggregateRoot struct {
	uncommittedEvents []DomainEvent
	version           int64
}

// NewAggregateRoot 创建一个新的 AggregateRoot 实例
//
// 返回的 AggregateRoot 实例：
//   - 未提交事件列表为空
//   - 版本号初始化为 0
//
// 这是创建聚合根的推荐方式，确保正确初始化内部状态。
func NewAggregateRoot() *AggregateRoot {
	return &AggregateRoot{
		uncommittedEvents: make([]DomainEvent, 0),
		version:           0,
	}
}

// RecordEvent 记录领域事件到未提交事件列表
//
// 此方法不会发布事件，仅将事件添加到内部缓冲区。
// 事件需要在适当的时机通过事件发布器发布。
//
// 参数：
//   - event: 要记录的领域事件，必须实现 DomainEvent 接口
//
// 注意：
//   - 此方法由子类通过嵌入 AggregateRoot 来访问
//   - 事件会被追加到现有未提交事件列表末尾
//   - 发布后应调用 ClearEvents() 清空事件列表
func (a *AggregateRoot) RecordEvent(event DomainEvent) {
	a.uncommittedEvents = append(a.uncommittedEvents, event)
}

// GetUncommittedEvents 获取未提交的事件列表（返回副本）
//
// 返回未提交事件的副本，防止外部修改影响内部状态。
// 这保持了封装性，避免外部代码直接修改内部事件列表。
//
// 返回：
//   - 未提交领域事件的副本，如果没有事件则返回空切片（非 nil）
//
// 使用场景：
//   - 事件发布器读取并发布待发布事件
//   - 事件溯源重放时读取事件流
func (a *AggregateRoot) GetUncommittedEvents() []DomainEvent {
	result := make([]DomainEvent, len(a.uncommittedEvents))
	copy(result, a.uncommittedEvents)
	return result
}

// GetDomainEvents 获取领域事件列表，兼容旧接口命名。
func (a *AggregateRoot) GetDomainEvents() []DomainEvent {
	return a.GetUncommittedEvents()
}

// ClearEvents 清空未提交事件列表
//
// 将内部事件列表重置为 nil，释放内存。
// 通常在事件发布完成后调用，表示这些事件已成功处理。
//
// 注意：
//   - 清空后 GetUncommittedEvents() 将返回空切片
//   - 版本号不受影响
//   - 此操作不可逆，调用前应确保事件已正确处理
func (a *AggregateRoot) ClearEvents() {
	a.uncommittedEvents = nil
}

// ClearDomainEvents 清空领域事件列表，兼容旧接口命名。
func (a *AggregateRoot) ClearDomainEvents() {
	a.ClearEvents()
}

// GetVersion 获取聚合根的当前版本号
//
// 版本号用于乐观锁并发控制，每次状态变更应递增版本号。
//
// 返回：
//   - 当前版本号，初始值为 0
//
// 使用场景：
//   - 保存聚合根时检查版本冲突
//   - 事件溯源中标记事件版本
//   - 分布式系统中的一致性保证
func (a *AggregateRoot) GetVersion() int64 {
	return a.version
}

// IncrementVersion 递增聚合根版本号
//
// 每次聚合根状态变更时调用此方法，用于乐观锁版本控制。
//
// 注意：
//   - 版本号从 0 开始，每次调用增加 1
//   - 应在状态修改完成后调用
//   - 配合 GetVersion() 用于并发冲突检测
func (a *AggregateRoot) IncrementVersion() {
	a.version++
}
