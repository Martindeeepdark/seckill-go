// Package rpc 提供gRPC服务实现
package rpc

import (
	"sort"
	"strings"
	"time"

	free_cardv1 "seckill-api/free_card/v1"
	memberv1 "seckill-api/member/v1"
	order_syncv1 "seckill-api/order_sync/v1"
	paymentv1 "seckill-api/payment/v1"
	paymentdomain "seckill-support-service/internal/domain/entity"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// createPayFromPB 将protobuf创建支付请求转换为领域实体
func createPayFromPB(r *paymentv1.CreatePay) paymentdomain.CreatePayRequest {
	if r == nil {
		return paymentdomain.CreatePayRequest{}
	}
	return paymentdomain.CreatePayRequest{OrderNo: r.GetOrderNo(), UserID: r.GetUserId(), PayAmount: r.GetPayAmount(), PayChannel: r.GetPayChannel(), Subject: r.GetSubject(), ExpireAt: timeFromPB(r.GetExpireAt())}
}

// payResultToPB 将领域支付结果转换为protobuf
func payResultToPB(r paymentdomain.PayResult) *paymentv1.PayResult {
	return &paymentv1.PayResult{OrderNo: r.OrderNo, PayChannel: r.PayChannel, PrepayId: r.PrepayID, NonceStr: r.NonceStr, TimeStamp: r.TimeStamp, Sign: r.Sign}
}

// payQueryResultToPB 将领域支付查询结果转换为protobuf
func payQueryResultToPB(r paymentdomain.PayQueryResult) *paymentv1.PayQueryResult {
	return &paymentv1.PayQueryResult{OrderNo: r.OrderNo, PayStatus: r.PayStatus, TransactionNo: r.TransactionNo, PaidAt: timePtrToPB(r.PaidAt)}
}

// payQueryResultsToPB 批量将支付查询结果转换为protobuf
func payQueryResultsToPB(results map[string]paymentdomain.PayQueryResult, orderNos []string) []*paymentv1.PayQueryResult {
	if len(results) == 0 {
		return nil
	}
	out := make([]*paymentv1.PayQueryResult, 0, len(results))
	seen := map[string]bool{}
	for _, no := range orderNos {
		if r, ok := results[no]; ok {
			seen[no] = true
			out = append(out, payQueryResultToPB(r))
		}
	}
	var rem []string
	for k := range results {
		if !seen[k] {
			rem = append(rem, k)
		}
	}
	sort.Strings(rem)
	for _, no := range rem {
		out = append(out, payQueryResultToPB(results[no]))
	}
	return out
}

// issueCardFromPB 将protobuf发卡请求转换为领域实体
func issueCardFromPB(r *free_cardv1.IssueCard) paymentdomain.IssueCardRequest {
	if r == nil {
		return paymentdomain.IssueCardRequest{}
	}
	return paymentdomain.IssueCardRequest{UserID: r.GetUserId(), OrderNo: r.GetOrderNo(), CardName: r.GetCardName(), FaceValue: r.GetFaceValue(), ValidDays: r.GetValidDays()}
}

// activateCardFromPB 将protobuf激活卡请求转换为领域实体
func activateCardFromPB(r *free_cardv1.ActivateCardRequest) paymentdomain.ActivateCardRequest {
	if r == nil {
		return paymentdomain.ActivateCardRequest{}
	}
	return paymentdomain.ActivateCardRequest{CardNo: r.GetCardNo(), UserID: r.GetUserId(), OrderNo: r.GetOrderNo()}
}

// freeCardToPB 将领域自由卡转换为protobuf
func freeCardToPB(c paymentdomain.FreeCard) *free_cardv1.FreeCard {
	return &free_cardv1.FreeCard{CardNo: c.CardNo, CardName: c.CardName, FaceValue: c.FaceValue, UserId: c.UserID, OrderNo: c.OrderNo, Status: c.Status, ValidDays: c.ValidDays, ActivatedAt: timePtrToPB(c.ActivatedAt), ExpireAt: timePtrToPB(c.ExpireAt), CreatedAt: timeToPB(c.CreatedAt)}
}

// freeCardsToPB 批量将自由卡转换为protobuf
func freeCardsToPB(cards []paymentdomain.FreeCard) []*free_cardv1.FreeCard {
	out := make([]*free_cardv1.FreeCard, 0, len(cards))
	for _, c := range cards {
		out = append(out, freeCardToPB(c))
	}
	return out
}

// syncOrderFromPB 将protobuf订单同步请求转换为领域实体
func syncOrderFromPB(r *order_syncv1.SyncOrder) paymentdomain.SyncOrderRequest {
	if r == nil {
		return paymentdomain.SyncOrderRequest{}
	}
	return paymentdomain.SyncOrderRequest{OrderNo: r.GetOrderNo(), UserID: r.GetUserId(), OrderSource: r.GetOrderSource(), TotalAmount: r.GetTotalAmount(), DiscountAmount: r.GetDiscountAmount(), PayAmount: r.GetPayAmount(), PaidAt: timeFromPB(r.GetPaidAt()), TransactionNo: r.GetTransactionNo()}
}

// syncedOrderToPB 将领域已同步订单转换为protobuf
func syncedOrderToPB(o paymentdomain.SyncedOrder) *order_syncv1.SyncedOrder {
	return &order_syncv1.SyncedOrder{OrderNo: o.OrderNo, UserId: o.UserID, OrderSource: o.OrderSource, TotalAmount: o.TotalAmount, DiscountAmount: o.DiscountAmount, PayAmount: o.PayAmount, OrderStatus: o.OrderStatus, PaidAt: timeToPB(o.PaidAt), CompletedAt: timePtrToPB(o.CompletedAt), TransactionNo: o.TransactionNo, CreatedAt: timeToPB(o.CreatedAt)}
}

// syncedOrdersToPB 批量将已同步订单转换为protobuf
func syncedOrdersToPB(orders []paymentdomain.SyncedOrder) []*order_syncv1.SyncedOrder {
	out := make([]*order_syncv1.SyncedOrder, 0, len(orders))
	for _, o := range orders {
		out = append(out, syncedOrderToPB(o))
	}
	return out
}

// syncedOrdersByOrderNoToPB 根据订单号批量将已同步订单转换为protobuf
func syncedOrdersByOrderNoToPB(orders map[string]paymentdomain.SyncedOrder, nos []string) []*order_syncv1.SyncedOrder {
	if len(orders) == 0 {
		return nil
	}
	out := make([]*order_syncv1.SyncedOrder, 0, len(orders))
	seen := map[string]bool{}
	for _, no := range nos {
		if o, ok := orders[no]; ok {
			seen[no] = true
			out = append(out, syncedOrderToPB(o))
		}
	}
	var rem []string
	for k := range orders {
		if !seen[k] {
			rem = append(rem, k)
		}
	}
	sort.Strings(rem)
	for _, no := range rem {
		out = append(out, syncedOrderToPB(orders[no]))
	}
	return out
}

// userToPB 将领域用户转换为protobuf
func userToPB(u paymentdomain.User) *memberv1.User {
	return &memberv1.User{Id: u.ID, Username: u.Username, Phone: u.Phone, Nickname: u.Nickname, Avatar: u.Avatar, MemberLevel: u.MemberLevel, Status: u.Status}
}

// uniqueNonEmptyStrings 去重并过滤空字符串
func uniqueNonEmptyStrings(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

// timeToPB 将时间转换为protobuf时间戳
func timeToPB(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

// timeFromPB 将protobuf时间戳转换为时间
func timeFromPB(ts *timestamppb.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return ts.AsTime()
}

// timePtrToPB 将时间指针转换为protobuf时间戳
func timePtrToPB(t *time.Time) *timestamppb.Timestamp {
	if t == nil || t.IsZero() {
		return nil
	}
	return timestamppb.New(*t)
}
