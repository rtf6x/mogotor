package collector

import "testing"

func TestParseSystemctlProperties(t *testing.T) {
	values := parseSystemctlProperties(`MainPID=731
MemoryCurrent=113995776
Description=MongoDB Database Server
ActiveState=active
SubState=running
`)

	if values["ActiveState"] != "active" {
		t.Fatalf("unexpected active state: %q", values["ActiveState"])
	}
	if values["MainPID"] != "731" {
		t.Fatalf("unexpected pid: %q", values["MainPID"])
	}
	if values["Description"] != "MongoDB Database Server" {
		t.Fatalf("unexpected description: %q", values["Description"])
	}
}
