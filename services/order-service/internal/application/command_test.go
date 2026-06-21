package application

import (
	"testing"
	"time"
)

func TestCreateOrderCommand_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cmd     CreateOrderCommand
		wantErr bool
	}{
		{
			name: "valid command",
			cmd: CreateOrderCommand{
				OrderNo:    "ORD-001",
				UserID:     12345,
				ActivityNo: "ACT-001",
				SKUNo:      "SKU-001",
				Quantity:   1,
				PayAmount:  10000,
				TraceID:    "trace-001",
			},
			wantErr: false,
		},
		{
			name: "empty order no",
			cmd: CreateOrderCommand{
				UserID:    12345,
				Quantity:  1,
				PayAmount: 10000,
			},
			wantErr: true,
		},
		{
			name: "zero quantity",
			cmd: CreateOrderCommand{
				OrderNo:   "ORD-001",
				UserID:    12345,
				Quantity:  0,
				PayAmount: 10000,
			},
			wantErr: true,
		},
		{
			name: "negative user id",
			cmd: CreateOrderCommand{
				OrderNo:   "ORD-001",
				UserID:    -1,
				Quantity:  1,
				PayAmount: 10000,
			},
			wantErr: true,
		},
		{
			name: "empty activity no",
			cmd: CreateOrderCommand{
				OrderNo:   "ORD-001",
				UserID:    12345,
				Quantity:  1,
				PayAmount: 10000,
			},
			wantErr: true,
		},
		{
			name: "empty sku no",
			cmd: CreateOrderCommand{
				OrderNo:    "ORD-001",
				UserID:     12345,
				ActivityNo: "ACT-001",
				Quantity:   1,
				PayAmount:  10000,
			},
			wantErr: true,
		},
		{
			name: "zero pay amount",
			cmd: CreateOrderCommand{
				OrderNo:    "ORD-001",
				UserID:     12345,
				ActivityNo: "ACT-001",
				SKUNo:      "SKU-001",
				Quantity:   1,
				PayAmount:  0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cmd.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPayOrderCommand_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cmd     PayOrderCommand
		wantErr bool
	}{
		{
			name: "valid command",
			cmd: PayOrderCommand{
				OrderNo:       "ORD-001",
				TransactionNo: "TXN-001",
				Amount:        10000,
				PaidAt:        time.Now(),
			},
			wantErr: false,
		},
		{
			name: "empty order no",
			cmd: PayOrderCommand{
				TransactionNo: "TXN-001",
				Amount:        10000,
			},
			wantErr: true,
		},
		{
			name: "empty transaction no",
			cmd: PayOrderCommand{
				OrderNo: "ORD-001",
				Amount:  10000,
			},
			wantErr: true,
		},
		{
			name: "zero amount",
			cmd: PayOrderCommand{
				OrderNo:       "ORD-001",
				TransactionNo: "TXN-001",
				Amount:        0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cmd.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCloseOrderCommand_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cmd     CloseOrderCommand
		wantErr bool
	}{
		{
			name: "valid command",
			cmd: CloseOrderCommand{
				OrderNo:  "ORD-001",
				Reason:   "user cancelled",
				ClosedAt: time.Now(),
			},
			wantErr: false,
		},
		{
			name: "empty order no",
			cmd: CloseOrderCommand{
				Reason:   "user cancelled",
				ClosedAt: time.Now(),
			},
			wantErr: true,
		},
		{
			name: "empty reason",
			cmd: CloseOrderCommand{
				OrderNo:  "ORD-001",
				ClosedAt: time.Now(),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cmd.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
