package application

import "testing"

func TestStartActivityCommand_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cmd     StartActivityCommand
		wantErr bool
	}{
		{
			name: "valid command",
			cmd: StartActivityCommand{
				ActivityNo: "ACT-001",
			},
			wantErr: false,
		},
		{
			name: "empty activity no",
			cmd: StartActivityCommand{
				ActivityNo: "",
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

func TestAddSKUCommand_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cmd     AddSKUCommand
		wantErr bool
	}{
		{
			name: "valid command",
			cmd: AddSKUCommand{
				ActivityNo: "ACT-001",
				SKUNo:      "SKU-001",
				Stock:      100,
				Price:      9900,
			},
			wantErr: false,
		},
		{
			name: "zero stock",
			cmd: AddSKUCommand{
				ActivityNo: "ACT-001",
				SKUNo:      "SKU-001",
				Stock:      0,
				Price:      9900,
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

func TestEndActivityCommand_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cmd     EndActivityCommand
		wantErr bool
	}{
		{
			name: "valid command",
			cmd: EndActivityCommand{
				ActivityNo: "ACT-001",
				Reason:     "Time expired",
			},
			wantErr: false,
		},
		{
			name: "empty activity no",
			cmd: EndActivityCommand{
				ActivityNo: "",
				Reason:     "Time expired",
			},
			wantErr: true,
		},
		{
			name: "empty reason",
			cmd: EndActivityCommand{
				ActivityNo: "ACT-001",
				Reason:     "",
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

func TestRemoveSKUCommand_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cmd     RemoveSKUCommand
		wantErr bool
	}{
		{
			name: "valid command",
			cmd: RemoveSKUCommand{
				ActivityNo: "ACT-001",
				SKUNo:      "SKU-001",
				Reason:     "Out of stock",
			},
			wantErr: false,
		},
		{
			name: "empty activity no",
			cmd: RemoveSKUCommand{
				ActivityNo: "",
				SKUNo:      "SKU-001",
				Reason:     "Out of stock",
			},
			wantErr: true,
		},
		{
			name: "empty sku no",
			cmd: RemoveSKUCommand{
				ActivityNo: "ACT-001",
				SKUNo:      "",
				Reason:     "Out of stock",
			},
			wantErr: true,
		},
		{
			name: "empty reason",
			cmd: RemoveSKUCommand{
				ActivityNo: "ACT-001",
				SKUNo:      "SKU-001",
				Reason:     "",
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
