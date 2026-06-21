package status

import (
	"testing"
)

func TestCanPaymentTransitionTo(t *testing.T) {
	if !CanPaymentTransitionTo(PayStatusPending, PayStatusPaid) { t.Error("pending->paid should be true") }
	if CanPaymentTransitionTo(PayStatusPaid, PayStatusClosed) { t.Error("paid should be terminal") }
	if CanPaymentTransitionTo(PayStatusPending, PayStatusPending) { t.Error("same state should be false") }
}
