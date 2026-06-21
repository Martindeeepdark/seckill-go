package vo

import "testing"

func TestPaymentInfo_Validate(t *testing.T) {
	tests := []struct {
		name          string
		transactionNo string
		amount        int64
		wantErr       bool
	}{
		{
			name:          "valid payment info",
			transactionNo: "TXN-001",
			amount:        10000,
			wantErr:       false,
		},
		{
			name:          "empty transaction no",
			transactionNo: "",
			amount:        10000,
			wantErr:       true,
		},
		{
			name:          "zero amount",
			transactionNo: "TXN-001",
			amount:        0,
			wantErr:       true,
		},
		{
			name:          "negative amount",
			transactionNo: "TXN-001",
			amount:        -100,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pi := PaymentInfo{
				TransactionNo: tt.transactionNo,
				Amount:        tt.amount,
			}

			err := pi.Validate()

			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
