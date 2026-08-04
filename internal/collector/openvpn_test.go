package collector

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestParseOpenVPNStatus(t *testing.T) {
	input := `OpenVPN CLIENT LIST
Updated,Thu Jul 23 04:11:48 2026
Common Name,Real Address,Bytes Received,Bytes Sent,Connected Since
alice,203.0.113.10:51234,1234,5678,Thu Jul 23 03:00:01 2026
bob,203.0.113.11:51235,2345,6789,Thu Jul 23 03:05:01 2026
ROUTING TABLE
Virtual Address,Common Name,Real Address,Last Ref
10.8.0.6,charlie,203.0.113.12:51236,Thu Jul 23 04:10:01 2026
GLOBAL STATS
Max bcast/mcast queue length,1
END
`

	clients, updatedAt := parseOpenVPNStatus(input)
	if updatedAt != "Thu Jul 23 04:11:48 2026" {
		t.Fatalf("unexpected updatedAt: %q", updatedAt)
	}
	if len(clients) != 3 {
		t.Fatalf("expected 3 clients, got %d: %v", len(clients), clients)
	}
	if clients[0] != "alice" || clients[1] != "bob" || clients[2] != "charlie" {
		t.Fatalf("unexpected clients: %v", clients)
	}
}

func TestParseOpenVPNStatusEmpty(t *testing.T) {
	input := `OpenVPN CLIENT LIST
Updated,Thu Jul 23 04:11:48 2026
Common Name,Real Address,Bytes Received,Bytes Sent,Connected Since
ROUTING TABLE
Virtual Address,Common Name,Real Address,Last Ref
GLOBAL STATS
Max bcast/mcast queue length,1
END
`

	clients, updatedAt := parseOpenVPNStatus(input)
	if updatedAt != "Thu Jul 23 04:11:48 2026" {
		t.Fatalf("unexpected updatedAt: %q", updatedAt)
	}
	if len(clients) != 0 {
		t.Fatalf("expected no clients, got %v", clients)
	}
}

func TestOpenVPNStateFromDockerStatus(t *testing.T) {
	active, sub := openVPNStateFromDockerStatus("running")
	if active != "active" || sub != "running" {
		t.Fatalf("running: got %s / %s", active, sub)
	}

	active, sub = openVPNStateFromDockerStatus("exited")
	if active != "inactive" || sub != "exited" {
		t.Fatalf("exited: got %s / %s", active, sub)
	}

	active, sub = openVPNStateFromDockerStatus("")
	if active != "inactive" || sub != "unknown" {
		t.Fatalf("empty: got %s / %s", active, sub)
	}
}

func TestCollectOpenVPNRunningWithClients(t *testing.T) {
	path := writeOpenVPNStatusFile(t, `OpenVPN CLIENT LIST
Updated,2026-08-04 18:00:00
Common Name,Real Address,Bytes Received,Bytes Sent,Connected Since
rtf6x,203.0.113.10:1,1,1,2026-08-04 17:00:00
ROUTING TABLE
Virtual Address,Common Name,Real Address,Last Ref
GLOBAL STATS
END
`)

	snap := collectOpenVPN(path, "openvpn", func(string) (string, error) {
		return "running", nil
	})
	if snap.ServiceName != "openvpn" {
		t.Fatalf("service name: %q", snap.ServiceName)
	}
	if snap.Active != "active" || snap.SubState != "running" {
		t.Fatalf("state: %s / %s", snap.Active, snap.SubState)
	}
	if !snap.Available || len(snap.Clients) != 1 || snap.Clients[0] != "rtf6x" {
		t.Fatalf("clients: available=%v %v", snap.Available, snap.Clients)
	}
}

func TestCollectOpenVPNStoppedContainer(t *testing.T) {
	path := writeOpenVPNStatusFile(t, `OpenVPN CLIENT LIST
Updated,2026-08-04 18:00:00
Common Name,Real Address,Bytes Received,Bytes Sent,Connected Since
ROUTING TABLE
Virtual Address,Common Name,Real Address,Last Ref
GLOBAL STATS
END
`)

	snap := collectOpenVPN(path, "openvpn", func(string) (string, error) {
		return "exited", nil
	})
	if snap.Active != "inactive" || snap.SubState != "exited" {
		t.Fatalf("state: %s / %s", snap.Active, snap.SubState)
	}
	if !snap.Available {
		t.Fatalf("status file should still be available")
	}
}

func TestCollectOpenVPNMissingContainer(t *testing.T) {
	path := writeOpenVPNStatusFile(t, `OpenVPN CLIENT LIST
Updated,2026-08-04 18:00:00
Common Name,Real Address,Bytes Received,Bytes Sent,Connected Since
ROUTING TABLE
Virtual Address,Common Name,Real Address,Last Ref
GLOBAL STATS
END
`)

	snap := collectOpenVPN(path, "openvpn", func(string) (string, error) {
		return "", fmt.Errorf("No such container: openvpn")
	})
	if snap.Active != "inactive" {
		t.Fatalf("expected inactive, got %s", snap.Active)
	}
	if snap.Error == "" {
		t.Fatal("expected inspect error")
	}
	if !snap.Available {
		t.Fatalf("status file should still be available, err=%s", snap.Error)
	}
}

func writeOpenVPNStatusFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "openvpn-status.log")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
