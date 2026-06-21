// Package application 提供 gateway 的应用层服务
// 包含秒杀、支付、管理等业务逻辑处理
package application

import (
	"context"
	"crypto/hmac"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"seckill-gateway-service/internal/config"
)

const (
	defaultMachineRandomLength = 16
	defaultMachineTTL          = 30 * time.Second
	machineCheckKeyPrefix      = "seckill:machine:check:"
	machineAlphabet            = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

// MachineCheckStore 保存 Java 版机审一次性答案。
type MachineCheckStore interface {
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, key string) error
}

// HMMachineChecker 使用 HMAC 验证机器检查令牌
type HMMachineChecker struct {
	secret string
}

// NewHMMachineChecker 创建新的基于 HMAC 的机器检查器
func NewHMMachineChecker(cfg config.MachineCheckConfig) *HMMachineChecker {
	return &HMMachineChecker{secret: cfg.Secret}
}

// Challenge HMAC 模式不生成 Java challenge。
func (m *HMMachineChecker) Challenge(context.Context, int64) (MachineChallenge, error) {
	return MachineChallenge{}, nil
}

// Check 验证机器检查令牌。令牌格式为：timestamp.signature
func (m *HMMachineChecker) Check(_ context.Context, _ int64, token string) bool {
	if token == "" || m.secret == "" {
		return false
	}
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return false
	}
	tsStr, signature := parts[0], parts[1]
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return false
	}
	// 令牌 5 分钟后过期
	if time.Since(time.UnixMilli(ts)) > 5*time.Minute {
		return false
	}
	mac := hmac.New(sha256.New, []byte(m.secret))
	mac.Write([]byte(tsStr))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expected))
}

// JavaMachineChecker 使用 Java 秒杀服务同款随机串机审算法。
type JavaMachineChecker struct {
	randomLength int
	ttl          time.Duration
	store        MachineCheckStore
	now          func() time.Time
	random       func(length int) (string, error)
}

// NewJavaMachineChecker 创建 Java 版机器检查器。
func NewJavaMachineChecker(cfg config.MachineCheckConfig, store MachineCheckStore) *JavaMachineChecker {
	randomLength := cfg.RandomLength
	if randomLength <= 0 {
		randomLength = defaultMachineRandomLength
	}
	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = defaultMachineTTL
	}
	checker := &JavaMachineChecker{
		randomLength: randomLength,
		ttl:          ttl,
		store:        store,
		now:          time.Now,
	}
	checker.random = checker.randomString
	return checker
}

// Challenge 生成 Java 风格机审挑战，并保存一次性答案。
func (m *JavaMachineChecker) Challenge(ctx context.Context, userID int64) (MachineChallenge, error) {
	if m.store == nil {
		return MachineChallenge{}, fmt.Errorf("machine check store is required")
	}
	result, err := m.random(m.randomLength)
	if err != nil {
		return MachineChallenge{}, fmt.Errorf("machine check random: %w", err)
	}
	timestamp := m.now().UnixMilli()
	expected := javaMachineExpected(result, timestamp)
	if err := m.store.Set(ctx, machineCheckKey(userID), expected, m.ttl); err != nil {
		return MachineChallenge{}, fmt.Errorf("machine check set: %w", err)
	}
	return MachineChallenge{Result: result, Key: timestamp}, nil
}

// Check 校验 Java 风格机审答案，成功后删除一次性答案。
func (m *JavaMachineChecker) Check(ctx context.Context, userID int64, token string) bool {
	if m.store == nil || strings.TrimSpace(token) == "" {
		return false
	}
	key := machineCheckKey(userID)
	expected, err := m.store.Get(ctx, key)
	if err != nil || expected == "" || expected != token {
		return false
	}
	if err := m.store.Delete(ctx, key); err != nil {
		return false
	}
	return true
}

func (m *JavaMachineChecker) randomString(length int) (string, error) {
	if length <= 0 {
		length = defaultMachineRandomLength
	}
	buf := make([]byte, length)
	if _, err := cryptorand.Read(buf); err != nil {
		return "", fmt.Errorf("machine random: %w", err)
	}
	for i := range buf {
		buf[i] = machineAlphabet[int(buf[i])%len(machineAlphabet)]
	}
	return string(buf), nil
}

func javaMachineExpected(result string, timestamp int64) string {
	if result == "" {
		return ""
	}
	chars := []byte(result)
	index := int(timestamp % int64(len(chars)))
	chars[index] = 'z'
	return string(chars)
}

func machineCheckKey(userID int64) string {
	return machineCheckKeyPrefix + strconv.FormatInt(userID, 10)
}

// NopMachineChecker 总是通过检查（空操作）
type NopMachineChecker struct{}

// Challenge NopMachineChecker 不生成挑战，直接返回空值。
func (n *NopMachineChecker) Challenge(context.Context, int64) (MachineChallenge, error) {
	return MachineChallenge{}, nil
}

// Check NopMachineChecker 总是返回 true，放行所有请求。
func (n *NopMachineChecker) Check(context.Context, int64, string) bool { return true }
