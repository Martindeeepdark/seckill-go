package status

import (
	"testing"
)

func TestCanActivityTransitionTo(t *testing.T) {
	tests := []struct {
		name string
		from int64
		to   int64
		want bool
	}{
		{name: "待开始->进行中", from: ActivityPending, to: ActivityOpen, want: true},
		{name: "待开始->已结束", from: ActivityPending, to: ActivityEnded, want: true},
		{name: "进行中->已暂停", from: ActivityOpen, to: ActivityPaused, want: true},
		{name: "已暂停->进行中", from: ActivityPaused, to: ActivityOpen, want: true},
		{name: "进行中->已结束", from: ActivityOpen, to: ActivityEnded, want: true},
		{name: "已结束为终态", from: ActivityEnded, to: ActivityOpen, want: false},
		{name: "进行中不可回待开始", from: ActivityOpen, to: ActivityPending, want: false},
		{name: "同状态转换非法", from: ActivityPending, to: ActivityPending, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CanActivityTransitionTo(tt.from, tt.to)
			if got != tt.want {
				t.Fatalf("CanActivityTransitionTo(%d, %d) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

func TestActivityStatusLabel(t *testing.T) {
	label, ok := ActivityStatusLabel[ActivityOpen]
	if !ok {
		t.Fatal("activity open status label not found")
	}
	if label != "进行中" {
		t.Fatalf("activity open label = %q, want 进行中", label)
	}
}
