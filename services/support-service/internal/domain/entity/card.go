// Package entity 定义支持服务的领域实体
package entity

import "time"

// IssueCardRequest 发卡请求
type IssueCardRequest struct {
	UserID    int64  // 用户ID
	OrderNo   string // 订单号
	CardName  string // 卡名称
	FaceValue int64  // 卡面值（单位：分）
	ValidDays int64  // 有效天数
}

// ActivateCardRequest 激活卡请求
type ActivateCardRequest struct {
	CardNo  string // 卡号
	UserID  int64  // 用户ID
	OrderNo string // 订单号
}

// FreeCard 自由卡实体
type FreeCard struct {
	CardNo      string     // 卡号
	CardName    string     // 卡名称
	FaceValue   int64      // 卡面值（单位：分）
	UserID      int64      // 用户ID
	OrderNo     string     // 订单号
	Status      int64      // 卡状态
	ValidDays   int64      // 有效天数
	ActivatedAt *time.Time // 激活时间
	ExpireAt    *time.Time // 过期时间
	CreatedAt   time.Time  // 创建时间
}
