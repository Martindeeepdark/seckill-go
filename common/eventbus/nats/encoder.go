package nats

import (
	"encoding/json"
	"errors"
	"fmt"
	"seckill-common/domain"
)

// ErrUnregisteredEventType 表示事件类型未在编码器注册表中登记。
var ErrUnregisteredEventType = errors.New("unregistered event type")

// Encoder 事件编码器接口
type Encoder interface {
	Encode(event domain.DomainEvent) ([]byte, error)
	Decode(data []byte, eventType string) (domain.DomainEvent, error)
}

// JSONEncoder JSON 编码器
type JSONEncoder struct {
	registry map[string]func() domain.DomainEvent
}

// NewJSONEncoder 创建 JSON 编码器
func NewJSONEncoder() *JSONEncoder {
	return &JSONEncoder{
		registry: make(map[string]func() domain.DomainEvent),
	}
}

// Register 注册事件类型
func (e *JSONEncoder) Register(eventName string, factory func() domain.DomainEvent) {
	e.registry[eventName] = factory
}

// Encode 编码事件为 JSON
func (e *JSONEncoder) Encode(event domain.DomainEvent) ([]byte, error) {
	data, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("marshal event: %w", err)
	}
	return data, nil
}

// Decode 解码 JSON 为事件
func (e *JSONEncoder) Decode(data []byte, eventType string) (domain.DomainEvent, error) {
	factory, ok := e.registry[eventType]
	if !ok {
		return nil, ErrUnregisteredEventType
	}

	event := factory()
	err := json.Unmarshal(data, event)
	if err != nil {
		return nil, fmt.Errorf("unmarshal event data: %w", err)
	}

	return event, nil
}
