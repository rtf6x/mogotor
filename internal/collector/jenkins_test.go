package collector

import "testing"

func TestParseHeapMaxMB(t *testing.T) {
	if got := parseMemoryFlagMB("-Xmx256m"); got != 256 {
		t.Fatalf("expected 256, got %d", got)
	}
	if got := parseMemoryFlagMB("-Xmx1g"); got != 1024 {
		t.Fatalf("expected 1024, got %d", got)
	}
}

func TestParseHeapKB(t *testing.T) {
	if got := parseHeapKB("used 187456K, committed 262144K"); got != 187456 {
		t.Fatalf("expected 187456, got %d", got)
	}
}

func TestParseNativeMemoryTotalMB(t *testing.T) {
	input := `
Native Memory Tracking:

Total: 412MB reserved=500MB committed=420MB
`
	if got := parseNativeMemoryTotalMB(input); got != 412 {
		t.Fatalf("expected 412, got %d", got)
	}
}
