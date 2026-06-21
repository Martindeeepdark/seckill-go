package eventbus

import (
	"context"
	"seckill-common/domain"
)

// Bus 定义事件总线接口
type Bus interface {
	// Publish 发布事件
	Publish(ctx context.Context, event domain.Event) error
}
