// Package discovery 提供服务发现功能
// 支持 Redis 和静态两种服务发现模式
package discovery

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-kratos/kratos/v2/registry"
)

// StaticDiscovery 静态服务发现实现
// 使用预配置的地址映射提供服务发现
type StaticDiscovery struct {
	services map[string][]string // 服务名到地址列表的映射
}

// NewStaticDiscovery 创建静态服务发现实例
func NewStaticDiscovery(services map[string][]string) *StaticDiscovery {
	return &StaticDiscovery{services: services}
}

// GetService 获取指定服务的实例列表
// 返回预配置的服务地址
func (d *StaticDiscovery) GetService(ctx context.Context, serviceName string) ([]*registry.ServiceInstance, error) {
	addrs, ok := d.services[serviceName]
	if !ok {
		return nil, nil
	}
	instances := make([]*registry.ServiceInstance, 0, len(addrs))
	for _, addr := range addrs {
		instances = append(instances, &registry.ServiceInstance{
			Name: serviceName, Version: "v1",
			Endpoints: []string{addr},
			Metadata:  map[string]string{"grpcAddr": addr},
		})
	}
	return instances, nil
}

// Watch 监听服务变更
// 静态发现先返回一次配置快照，之后阻塞直到停止
func (d *StaticDiscovery) Watch(ctx context.Context, serviceName string) (registry.Watcher, error) {
	instances, err := d.GetService(ctx, serviceName)
	if err != nil {
		return nil, err
	}
	watchCtx, cancel := context.WithCancel(ctx)
	return &staticWatcher{
		ctx:       watchCtx,
		cancel:    cancel,
		instances: instances,
	}, nil
}

type staticWatcher struct {
	ctx       context.Context
	cancel    context.CancelFunc
	instances []*registry.ServiceInstance
	mu        sync.Mutex
	sent      bool
}

func (w *staticWatcher) Next() ([]*registry.ServiceInstance, error) {
	w.mu.Lock()
	if !w.sent {
		w.sent = true
		instances := w.instances
		w.mu.Unlock()
		return instances, nil
	}
	w.mu.Unlock()
	<-w.ctx.Done()
	return nil, fmt.Errorf("static watcher next: %w", w.ctx.Err())
}

func (w *staticWatcher) Stop() error {
	if w.cancel != nil {
		w.cancel()
	}
	return nil
}
