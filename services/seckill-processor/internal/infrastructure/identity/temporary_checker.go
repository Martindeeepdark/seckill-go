package identity

import (
	commonerrors "seckill-common/errors"

	"seckill-processor-service/internal/domain/service"
)

var _ service.TemporaryChecker = (*RPCTemporaryChecker)(nil)

// RPCTemporaryChecker 基于 gRPC 状态码的临时错误判断器
type RPCTemporaryChecker struct{}

// IsTemporary 判断错误是否为临时性 gRPC 错误
func (RPCTemporaryChecker) IsTemporary(err error) bool { return commonerrors.IsTemporaryRPCError(err) }
