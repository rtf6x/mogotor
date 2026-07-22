package collector

import (
	"testing"
	"time"
)

func TestParseAuthLogSSHFailure(t *testing.T) {
	now := mustParseTime(t, "2026-07-22 10:00:00")
	line := `Jul 22 10:12:35 rootfox sshd[2206]: Failed password for root from 20.84.101.167 port 40360 ssh2`

	event, ok := parseAuthLogSSHFailure(line, now)
	if !ok {
		t.Fatal("expected failure event")
	}
	if event.User != "root" || event.IP != "20.84.101.167" || event.Kind != "failed_password" {
		t.Fatalf("unexpected event: %+v", event)
	}
}

func TestParseAuthLogSSHAccepted(t *testing.T) {
	now := mustParseTime(t, "2026-07-22 10:00:00")
	line := `Jul 22 10:12:44 rootfox sshd[2314]: Accepted password for root from 185.71.88.5 port 61253 ssh2`

	event, ok := parseAuthLogSSHAccepted(line, now)
	if !ok {
		t.Fatal("expected accepted event")
	}
	if event.User != "root" || event.IP != "185.71.88.5" || event.Method != "password" {
		t.Fatalf("unexpected event: %+v", event)
	}
}

func TestParseAuthLogInvalidUser(t *testing.T) {
	now := mustParseTime(t, "2026-07-22 10:00:00")
	line := `Jul 22 10:12:35 rootfox sshd[2206]: Invalid user admin from 20.84.101.167 port 40360`

	event, ok := parseAuthLogSSHFailure(line, now)
	if !ok {
		t.Fatal("expected invalid user event")
	}
	if event.User != "admin" || event.Kind != "invalid_user" {
		t.Fatalf("unexpected event: %+v", event)
	}
}

func TestParseLastTimestamp(t *testing.T) {
	when, err := parseLastTimestamp("Wed Jul 22 10:23")
	if err != nil {
		t.Fatalf("parse last timestamp: %v", err)
	}
	if when.Month() != time.July || when.Day() != 22 || when.Hour() != 10 || when.Minute() != 23 {
		t.Fatalf("unexpected time: %v", when)
	}
}

func mustParseTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.Local)
	if err != nil {
		t.Fatalf("parse time: %v", err)
	}
	return parsed
}
