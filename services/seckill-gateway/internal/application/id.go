// Package application 提供 gateway 的应用层服务
// 包含秒杀、支付、管理等业务逻辑处理
package application

import (
	"crypto/rand"
	"fmt"
	"sync/atomic"
	"time"
)

var sequence uint64

// NewOrderID 生成唯一的订单编号
// 格式：SK{时间戳毫秒}{序列号}{随机字节}
func NewOrderID() string {
	now := time.Now()
	seq := atomic.AddUint64(&sequence, 1)
	var b [2]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Sprintf("crypto/rand.Read failed: %v", err))
	}
	return fmt.Sprintf("SK%d%04d%02x%02x", now.UnixMilli(), seq%10000, b[0], b[1])
}
