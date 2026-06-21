// Package application 提供应用层的数据传输对象（DTO）
package application

import (
	"time"

	"seckill-order-service/internal/domain/entity"
)

// OrderDTO 订单数据传输对象
type OrderDTO struct {
	OrderNo        string     `json:"orderNo"`
	UserID         int64      `json:"userId"`
	ActivityNo     string     `json:"activityNo"`
	SKUNo          string     `json:"skuNo"`
	Quantity       int64      `json:"quantity"`
	PayAmount      int64      `json:"payAmount"`
	Status         string     `json:"status"`
	TraceID        string     `json:"traceId"`
	RequestTraceID string     `json:"requestTraceId,omitempty"`
	TransactionNo  string     `json:"transactionNo,omitempty"`
	PaidAt         *time.Time `json:"paidAt,omitempty"`
	ClosedAt       *time.Time `json:"closedAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
}

// PaymentTimeoutTaskDTO 支付超时任务数据传输对象
type PaymentTimeoutTaskDTO struct {
	ID             string    `json:"id"`
	OrderNo        string    `json:"orderNo"`
	RequestTraceID string    `json:"requestTraceId,omitempty"`
	DueAt          time.Time `json:"dueAt"`
	Attempts       int64     `json:"attempts"`
	LastError      string    `json:"lastError,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt,omitempty"`
}

// SeckillMessageDTO 秒杀消息数据传输对象
type SeckillMessageDTO struct {
	UserID         int64     `json:"userId"`
	ActivityNo     string    `json:"activityNo"`
	SKUNo          string    `json:"skuNo"`
	Quantity       int64     `json:"quantity"`
	TotalFee       int64     `json:"totalFee"`
	Token          string    `json:"token"`
	TraceID        string    `json:"traceId"`
	RequestTraceID string    `json:"requestTraceId,omitempty"`
	RequestTime    time.Time `json:"requestTime"`
}

// ToOrderDTO 将订单领域实体转换为DTO
func ToOrderDTO(order entity.Order) OrderDTO {
	return OrderDTO{
		OrderNo:        order.OrderNo,
		UserID:         order.UserID,
		ActivityNo:     order.ActivityNo,
		SKUNo:          order.SKUNo,
		Quantity:       order.Quantity,
		PayAmount:      order.PayAmount,
		Status:         order.Status,
		TraceID:        order.TraceID,
		RequestTraceID: order.RequestTraceID,
		TransactionNo:  order.TransactionNo,
		PaidAt:         order.PaidAt,
		ClosedAt:       order.ClosedAt,
		CreatedAt:      order.CreatedAt,
	}
}

// ToOrderDTOList 将订单列表转换为DTO列表
func ToOrderDTOList(orders []entity.Order) []OrderDTO {
	dtos := make([]OrderDTO, len(orders))
	for i, order := range orders {
		dtos[i] = ToOrderDTO(order)
	}
	return dtos
}

// ToOrder 从DTO重建订单领域实体（慎用，仅用于反序列化）
func ToOrder(dto OrderDTO) entity.Order {
	return entity.Order{
		OrderNo:        dto.OrderNo,
		UserID:         dto.UserID,
		ActivityNo:     dto.ActivityNo,
		SKUNo:          dto.SKUNo,
		Quantity:       dto.Quantity,
		PayAmount:      dto.PayAmount,
		Status:         dto.Status,
		TraceID:        dto.TraceID,
		RequestTraceID: dto.RequestTraceID,
		TransactionNo:  dto.TransactionNo,
		PaidAt:         dto.PaidAt,
		ClosedAt:       dto.ClosedAt,
		CreatedAt:      dto.CreatedAt,
	}
}

// ToPaymentTimeoutTaskDTO 将支付超时任务领域实体转换为DTO
func ToPaymentTimeoutTaskDTO(task entity.PaymentTimeoutTask) PaymentTimeoutTaskDTO {
	return PaymentTimeoutTaskDTO{
		ID:             task.ID,
		OrderNo:        task.OrderNo,
		RequestTraceID: task.RequestTraceID,
		DueAt:          task.DueAt,
		Attempts:       task.Attempts,
		LastError:      task.LastError,
		CreatedAt:      task.CreatedAt,
		UpdatedAt:      task.UpdatedAt,
	}
}

// ToPaymentTimeoutTask 从DTO重建支付超时任务领域实体（慎用，仅用于反序列化）
func ToPaymentTimeoutTask(dto PaymentTimeoutTaskDTO) entity.PaymentTimeoutTask {
	return entity.PaymentTimeoutTask{
		ID:             dto.ID,
		OrderNo:        dto.OrderNo,
		RequestTraceID: dto.RequestTraceID,
		DueAt:          dto.DueAt,
		Attempts:       dto.Attempts,
		LastError:      dto.LastError,
		CreatedAt:      dto.CreatedAt,
		UpdatedAt:      dto.UpdatedAt,
	}
}

// ToSeckillMessageDTO 将秒杀消息领域实体转换为DTO
func ToSeckillMessageDTO(msg entity.SeckillMessage) SeckillMessageDTO {
	return SeckillMessageDTO{
		UserID:         msg.UserID,
		ActivityNo:     msg.ActivityNo,
		SKUNo:          msg.SKUNo,
		Quantity:       msg.Quantity,
		TotalFee:       msg.TotalFee,
		Token:          msg.Token,
		TraceID:        msg.TraceID,
		RequestTraceID: msg.RequestTraceID,
		RequestTime:    msg.RequestTime,
	}
}

// ToSeckillMessage 从DTO重建秒杀消息领域实体（慎用，仅用于反序列化）
func ToSeckillMessage(dto SeckillMessageDTO) entity.SeckillMessage {
	return entity.SeckillMessage{
		UserID:         dto.UserID,
		ActivityNo:     dto.ActivityNo,
		SKUNo:          dto.SKUNo,
		Quantity:       dto.Quantity,
		TotalFee:       dto.TotalFee,
		Token:          dto.Token,
		TraceID:        dto.TraceID,
		RequestTraceID: dto.RequestTraceID,
		RequestTime:    dto.RequestTime,
	}
}
