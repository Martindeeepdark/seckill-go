package model

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"seckill-processor-service/internal/domain/status"
)

func TestPostPayTaskJSONContract(t *testing.T) {
	task := PostPayTask{
		Type:           status.PostPayTaskSyncOrder,
		OrderNo:        "O1",
		RequestTraceID: "trace-1",
		SyncOrder: &SyncOrderPayload{
			OrderNo:       "O1",
			UserID:        7,
			OrderSource:   "SECKILL",
			TotalAmount:   9900,
			PayAmount:     9900,
			PaidAt:        time.Unix(1, 0).UTC(),
			TransactionNo: "T1",
		},
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("marshal post-pay task: %v", err)
	}
	body := string(data)
	for _, key := range []string{`"type"`, `"orderNo"`, `"requestTraceId"`, `"syncOrder"`, `"userId"`, `"payAmount"`} {
		if !strings.Contains(body, key) {
			t.Fatalf("post-pay json %s missing key %s", body, key)
		}
	}
	if strings.Contains(body, `"OrderNo"`) || strings.Contains(body, `"RequestTraceID"`) {
		t.Fatalf("post-pay json should use explicit lower-camel keys: %s", body)
	}
}

func TestPaymentTimeoutTaskJSONContract(t *testing.T) {
	task := PaymentTimeoutTask{
		OrderNo:        "O1",
		RequestTraceID: "trace-1",
		DueAt:          time.Unix(1, 0).UTC(),
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("marshal payment timeout task: %v", err)
	}
	body := string(data)
	for _, key := range []string{`"orderNo"`, `"requestTraceId"`, `"dueAt"`} {
		if !strings.Contains(body, key) {
			t.Fatalf("payment-timeout json %s missing key %s", body, key)
		}
	}
	if strings.Contains(body, `"OrderNo"`) || strings.Contains(body, `"RequestTraceID"`) {
		t.Fatalf("payment-timeout json should use explicit lower-camel keys: %s", body)
	}
}
