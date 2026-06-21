// Package entity 定义支持服务的领域实体
package entity

// User 用户实体
type User struct {
	ID          int64  // 用户ID
	Username    string // 用户名
	Phone       string // 手机号
	Nickname    string // 昵称
	Avatar      string // 头像
	MemberLevel int64  // 会员等级
	Status      int64  // 用户状态
}
