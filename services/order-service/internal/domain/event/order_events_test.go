package event

import (
	"testing"
	"time"
)

func TestOrderCreatedEvent(t *testing.T) {
	orderNo := "ORD-001"
	event := NewOrderCreatedEvent(orderNo, 12345, "ACT-001", "SKU-001", 1, 10000)

	if event.EventName() != "order.created" {
		t.Errorf("expected event name 'order.created', got %s", event.EventName())
	}
	if event.AggregateID() != orderNo {
		t.Errorf("expected aggregate ID %s, got %s", orderNo, event.AggregateID())
	}
	if event.OrderNo != orderNo {
		t.Errorf("expected order no %s, got %s", orderNo, event.OrderNo)
	}
}

func TestOrderPaidEvent(t *testing.T) {
	orderNo := "ORD-001"
	transactionNo := "TXN-001"
	event := NewOrderPaidEvent(orderNo, 12345, transactionNo, 10000, time.Now())

	if event.EventName() != "order.paid" {
		t.Errorf("expected event name 'order.paid', got %s", event.EventName())
	}
	if event.TransactionNo != transactionNo {
		t.Errorf("expected transaction no %s, got %s", transactionNo, event.TransactionNo)
	}
}

func TestOrderClosedEvent(t *testing.T) {
	orderNo := "ORD-001"
	reason := "timeout"
	event := NewOrderClosedEvent(orderNo, 12345, reason, time.Now())

	if event.EventName() != "order.closed" {
		t.Errorf("expected event name 'order.closed', got %s", event.EventName())
	}
	if event.Reason != reason {
		t.Errorf("expected reason %s, got %s", reason, event.Reason)
	}
}
