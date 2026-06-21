package vo

import "testing"

func TestIdentityValueObjectsTrimAndRejectEmpty(t *testing.T) {
	activityNo, ok := NewActivityNo(" A1 ")
	if !ok {
		t.Fatal("activity no should be valid")
	}
	if activityNo.String() != "A1" {
		t.Fatalf("activity no = %q, want A1", activityNo.String())
	}

	if _, ok := NewSKUNo(""); ok {
		t.Fatal("blank sku no should be invalid")
	}
	if _, ok := NewTraceID(""); ok {
		t.Fatal("blank trace id should be invalid")
	}
}

func TestAmountValueObjectsValidateBoundary(t *testing.T) {
	money, ok := NewMoney(0)
	if !ok || money.Cents() != 0 {
		t.Fatalf("money = %d, valid = %v, want zero valid", money.Cents(), ok)
	}
	if _, ok := NewMoney(-1); ok {
		t.Fatal("negative money should be invalid")
	}

	quantity, ok := NewQuantity(1)
	if !ok || quantity.Int() != 1 {
		t.Fatalf("quantity = %d, valid = %v, want 1 valid", quantity.Int(), ok)
	}
	if _, ok := NewQuantity(0); ok {
		t.Fatal("zero quantity should be invalid")
	}
}

func TestActivityValueObjectsValidateInput(t *testing.T) {
	name, ok := NewActivityName(" 秒杀活动 ")
	if !ok {
		t.Fatal("activity name should be valid")
	}
	if name.String() != "秒杀活动" {
		t.Fatalf("activity name = %q, want 秒杀活动", name.String())
	}
	if _, ok := NewActivityName(" "); ok {
		t.Fatal("blank activity name should be invalid")
	}

	limit, ok := NewPurchaseLimit(2)
	if !ok || limit.Int() != 2 {
		t.Fatalf("purchase limit = %d, valid = %v, want 2 valid", limit.Int(), ok)
	}
	if _, ok := NewPurchaseLimit(0); ok {
		t.Fatal("zero purchase limit should be invalid")
	}
	if DefaultPurchaseLimit().Int() != 1 {
		t.Fatalf("default purchase limit = %d, want 1", DefaultPurchaseLimit().Int())
	}
}
