package vo

import "errors"

var (
	ErrInvalidTransactionNo = errors.New("transaction no cannot be empty")
	ErrInvalidAmount        = errors.New("amount must be positive")
)

// PaymentInfo 记录支付信息，包含交易流水号和金额。
type PaymentInfo struct {
	TransactionNo string // 交易流水号
	Amount        int64  // 支付金额（分）
}

// Validate 校验支付信息，要求流水号非空且金额为正。
func (p PaymentInfo) Validate() error {
	if p.TransactionNo == "" {
		return ErrInvalidTransactionNo
	}
	if p.Amount <= 0 {
		return ErrInvalidAmount
	}
	return nil
}
