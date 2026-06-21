package application

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPayResultJSONContract(t *testing.T) {
	result := PayResult{
		OrderNo:    "O1",
		PayChannel: "mock",
		PrepayID:   "prepay-1",
		NonceStr:   "nonce-1",
		TimeStamp:  "123",
		Sign:       "sign-1",
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal pay result: %v", err)
	}
	body := string(data)
	for _, key := range []string{`"orderNo"`, `"payChannel"`, `"prepayId"`, `"nonceStr"`, `"timeStamp"`, `"sign"`} {
		if !strings.Contains(body, key) {
			t.Fatalf("pay result json %s missing key %s", body, key)
		}
	}
	if strings.Contains(body, `"PrepayID"`) || strings.Contains(body, `"OrderNo"`) {
		t.Fatalf("pay result json should use lower-camel keys: %s", body)
	}
}

func TestOrderDetailJSONContract(t *testing.T) {
	order := OrderDetail{
		OrderNo:        "O1",
		UserID:         7,
		ActivityNo:     "1001",
		SKUNo:          "2001",
		Quantity:       1,
		PayAmount:      9900,
		Status:         "PAID",
		TraceID:        "trace-1",
		RequestTraceID: "trace-1",
		TransactionNo:  "tx-1",
	}

	data, err := json.Marshal(order)
	if err != nil {
		t.Fatalf("marshal order detail: %v", err)
	}
	body := string(data)
	for _, key := range []string{`"orderNo"`, `"userId"`, `"activityNo"`, `"skuNo"`, `"payAmount"`, `"traceId"`, `"requestTraceId"`} {
		if !strings.Contains(body, key) {
			t.Fatalf("order detail json %s missing key %s", body, key)
		}
	}
	if strings.Contains(body, `"SKUNo"`) || strings.Contains(body, `"UserID"`) {
		t.Fatalf("order detail json should use lower-camel keys: %s", body)
	}
}
