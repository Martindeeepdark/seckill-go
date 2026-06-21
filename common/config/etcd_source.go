package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// EtcdConfigSource 从 etcd 加载配置
type EtcdConfigSource struct {
	client *clientv3.Client
	prefix string
}

// NewEtcdConfigSource 创建 etcd 配置源
// prefix 是配置 key 前缀，如 "/seckill/config"
// 连接失败时 panic，防止服务带着错误配置运行
func NewEtcdConfigSource(ctx context.Context, endpoints []string, prefix string) (*EtcdConfigSource, error) {
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("etcd endpoints is required")
	}
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		panic(fmt.Sprintf("etcd config source connect failed: %v (endpoints: %v)", err, endpoints))
	}
	return &EtcdConfigSource{client: client, prefix: prefix}, nil
}

// Load 从 etcd 加载指定服务的配置
// key 格式: {prefix}/{serviceName}
// value 格式: JSON 对象，会被递归合并到现有配置中
func (s *EtcdConfigSource) Load(ctx context.Context, serviceName string) (map[string]interface{}, error) {
	key := s.prefix + "/" + serviceName
	resp, err := s.client.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("etcd get config %s: %w", key, err)
	}
	if len(resp.Kvs) == 0 {
		return nil, nil
	}
	var result map[string]interface{}
	if err := json.Unmarshal(resp.Kvs[0].Value, &result); err != nil {
		return nil, fmt.Errorf("etcd config unmarshal %s: %w", key, err)
	}
	return result, nil
}

// Watch 监听配置变更，变更时回调 onChange
// PUT 事件解析 JSON 后回调；DELETE 事件回调 nil（回退到默认值）
func (s *EtcdConfigSource) Watch(ctx context.Context, serviceName string, onChange func(map[string]interface{})) error {
	key := s.prefix + "/" + serviceName
	watcher := clientv3.NewWatcher(s.client)
	defer watcher.Close()
	watchCh := watcher.Watch(ctx, key)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case resp, ok := <-watchCh:
			if !ok {
				return fmt.Errorf("etcd watcher closed for %s", key)
			}
			for _, ev := range resp.Events {
				handleWatchEvent(ev, onChange)
			}
		}
	}
}

// handleWatchEvent 处理单个 watch 事件。PUT 解析 JSON 回调；DELETE 回调 nil。
func handleWatchEvent(ev *clientv3.Event, onChange func(map[string]interface{})) {
	if ev.Type == clientv3.EventTypeDelete {
		onChange(nil)
		return
	}
	var result map[string]interface{}
	if err := json.Unmarshal(ev.Kv.Value, &result); err != nil {
		return
	}
	onChange(result)
}

// Put 写入配置到 etcd
func (s *EtcdConfigSource) Put(ctx context.Context, serviceName string, cfg map[string]interface{}) error {
	key := s.prefix + "/" + serviceName
	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	_, err = s.client.Put(ctx, key, string(data))
	if err != nil {
		return fmt.Errorf("etcd put config %s: %w", key, err)
	}
	return nil
}

// Delete 删除 etcd 中的服务配置，触发 watch 回退到 YAML 默认值
func (s *EtcdConfigSource) Delete(ctx context.Context, serviceName string) error {
	if s == nil {
		return nil
	}
	key := s.prefix + "/" + serviceName
	_, err := s.client.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("etcd delete config %s: %w", key, err)
	}
	return nil
}

// Close 关闭 etcd 客户端
func (s *EtcdConfigSource) Close() error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

// LoadWithEtcd 从本地 YAML 加载配置，然后用 etcd 中的配置覆盖
// 当配置了 etcd endpoints 但连接/读取失败时 panic
// 返回合并后的原始 map，以及 etcd 配置源（用于后续 watch）
func LoadWithEtcd(path string, serviceName string, endpoints []string, etcdPrefix string, defaultEndpoints []ServiceEndpoint) (map[string]interface{}, *EtcdConfigSource, error) {
	// 1. 加载本地 YAML
	raw, err := LoadRaw(path, defaultEndpoints)
	if err != nil {
		return nil, nil, err
	}

	if len(endpoints) == 0 {
		return raw, nil, nil
	}

	// 2. 连接 etcd 并加载配置 — 失败直接 panic
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	source, err := NewEtcdConfigSource(ctx, endpoints, etcdPrefix)
	if err != nil {
		// NewEtcdConfigSource 内部已 panic，这里不会被触发
		return nil, nil, err
	}

	etcdCfg, err := source.Load(ctx, serviceName)
	if err != nil {
		panic(fmt.Sprintf("etcd config load failed for %s: %v (endpoints: %v)", serviceName, err, endpoints))
	}

	// 3. 合并：etcd 配置覆盖本地
	if etcdCfg != nil {
		MergeMap(raw, etcdCfg)
	}

	return raw, source, nil
}

// WatchConfig 启动配置变更监听协程
// onChange 接收合并后的完整 Config 对象
func WatchConfig(ctx context.Context, source *EtcdConfigSource, serviceName string, base map[string]interface{}, logger *slog.Logger, onChange func(map[string]interface{})) {
	if source == nil {
		return
	}
	go func() {
		if err := source.Watch(ctx, serviceName, func(etcdCfg map[string]interface{}) {
			merged := make(map[string]interface{})
			for k, v := range base {
				merged[k] = v
			}
			MergeMap(merged, etcdCfg)
			onChange(merged)
			if logger != nil {
				logger.Info("config updated from etcd", "service", serviceName)
			}
		}); err != nil && ctx.Err() == nil {
			if logger != nil {
				logger.Warn("etcd config watch stopped", "service", serviceName, "error", err)
			}
		}
	}()
}
