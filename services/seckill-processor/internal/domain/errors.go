package domain

import "errors"

// ErrUnknownPostPayTask 未知的支付后任务类型错误
var ErrUnknownPostPayTask = errors.New("unknown post-pay task")
