package collector

import "testing"

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
