package application_test

import (
	"testing"

	"seckill-stock-service/internal/application"
)

func TestReserveStockCommandValidate(t *testing.T) {
	tests := []struct {
		name    string
		cmd     application.ReserveStockCommand
		wantErr bool
	}{
		{
			name: "valid command",
			cmd: application.ReserveStockCommand{
				ActivityNo:    "ACT001",
				SKUNo:         "SKU001",
				UserID:        1001,
				Quantity:      2,
				PurchaseLimit: 5,
				OrderNo:       "ORD001",
			},
			wantErr: false,
		},
		{
			name: "missing activity no",
			cmd: application.ReserveStockCommand{
				SKUNo:    "SKU001",
				UserID:   1001,
				Quantity: 2,
				OrderNo:  "ORD001",
			},
			wantErr: true,
		},
		{
			name: "missing sku no",
			cmd: application.ReserveStockCommand{
				ActivityNo: "ACT001",
				UserID:     1001,
				Quantity:   2,
				OrderNo:    "ORD001",
			},
			wantErr: true,
		},
		{
			name: "invalid quantity",
			cmd: application.ReserveStockCommand{
				ActivityNo: "ACT001",
				SKUNo:      "SKU001",
				UserID:     1001,
				Quantity:   0,
				OrderNo:    "ORD001",
			},
			wantErr: true,
		},
		{
			name: "negative quantity",
			cmd: application.ReserveStockCommand{
				ActivityNo: "ACT001",
				SKUNo:      "SKU001",
				UserID:     1001,
				Quantity:   -1,
				OrderNo:    "ORD001",
			},
			wantErr: true,
		},
		{
			name: "missing order no",
			cmd: application.ReserveStockCommand{
				ActivityNo: "ACT001",
				SKUNo:      "SKU001",
				UserID:     1001,
				Quantity:   2,
			},
			wantErr: true,
		},
		{
			name: "invalid user id",
			cmd: application.ReserveStockCommand{
				ActivityNo:    "ACT001",
				SKUNo:         "SKU001",
				UserID:        0,
				Quantity:      2,
				PurchaseLimit: 5,
				OrderNo:       "ORD001",
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

func TestReleaseStockCommandValidate(t *testing.T) {
	tests := []struct {
		name    string
		cmd     application.ReleaseStockCommand
		wantErr bool
	}{
		{
			name: "valid command",
			cmd: application.ReleaseStockCommand{
				ActivityNo: "ACT001",
				SKUNo:      "SKU001",
				UserID:     1001,
				Quantity:   1,
				OrderNo:    "ORD001",
			},
			wantErr: false,
		},
		{
			name: "missing activity no",
			cmd: application.ReleaseStockCommand{
				SKUNo:    "SKU001",
				UserID:   1001,
				Quantity: 1,
				OrderNo:  "ORD001",
			},
			wantErr: true,
		},
		{
			name: "invalid quantity",
			cmd: application.ReleaseStockCommand{
				ActivityNo: "ACT001",
				SKUNo:      "SKU001",
				UserID:     1001,
				Quantity:   -1,
				OrderNo:    "ORD001",
			},
			wantErr: true,
		},
		{
			name: "zero quantity",
			cmd: application.ReleaseStockCommand{
				ActivityNo: "ACT001",
				SKUNo:      "SKU001",
				UserID:     1001,
				Quantity:   0,
				OrderNo:    "ORD001",
			},
			wantErr: true,
		},
		{
			name: "missing order no",
			cmd: application.ReleaseStockCommand{
				ActivityNo: "ACT001",
				SKUNo:      "SKU001",
				UserID:     1001,
				Quantity:   1,
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
