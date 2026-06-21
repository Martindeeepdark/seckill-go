package traceresult

import "testing"

func TestKeyUsesJavaOrderResultPrefix(t *testing.T) {
	got := Key("TRACE1")
	want := "seckill:order:result:TRACE1"
	if got != want {
		t.Fatalf("Key() = %q, want %q", got, want)
	}
}
