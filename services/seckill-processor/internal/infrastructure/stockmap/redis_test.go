package stockmap

import "testing"

func TestKey(t *testing.T) {
	got := Key("O1")
	want := "seckill:stock:order:map:O1"
	if got != want {
		t.Fatalf("Key() = %q, want %q", got, want)
	}
}
