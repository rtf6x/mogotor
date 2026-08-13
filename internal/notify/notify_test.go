package notify

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rtf6x/mogotor/internal/models"
)

func TestMaybeSendPostsJSONTextOncePerDay(t *testing.T) {
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content-type %s", r.Header.Get("Content-Type"))
		}
		raw, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(raw))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	dir := t.TempDir()
	n := New(Config{
		URL:       srv.URL,
		StatePath: filepath.Join(dir, "last-daily-notify"),
		Hostname:  "ci.rootfox.cc",
	})
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	n.Location = loc

	snap := models.Snapshot{
		System: models.SystemSnapshot{
			MemoryAvailableBytes: 2 * 1024 * 1024 * 1024,
			MemoryTotalBytes:     4 * 1024 * 1024 * 1024,
			DiskUsedPercent:      47,
			DiskUsedBytes:        17 * 1024 * 1024 * 1024,
			DiskTotalBytes:       38 * 1024 * 1024 * 1024,
		},
		PM2:    models.PM2Snapshot{Available: true},
		Docker: models.DockerSnapshot{Available: true},
		Rabbit: models.RabbitSnapshot{Available: true},
	}

	atNine := time.Date(2026, 8, 14, 9, 0, 0, 0, loc)
	if err := n.MaybeSend(snap, atNine); err != nil {
		t.Fatalf("first send: %v", err)
	}
	if err := n.MaybeSend(snap, atNine.Add(time.Minute)); err != nil {
		t.Fatalf("same day: %v", err)
	}
	if len(bodies) != 1 {
		t.Fatalf("expected 1 POST, got %d %v", len(bodies), bodies)
	}

	var payload struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(bodies[0]), &payload); err != nil {
		t.Fatalf("json: %v body=%s", err, bodies[0])
	}
	if payload.Text == "" || !strings.Contains(payload.Text, "[mogotor] ci.rootfox.cc daily") || !strings.Contains(payload.Text, "disk /") {
		t.Fatalf("unexpected text: %q", payload.Text)
	}
}

func TestMaybeSendSkipsEmptyURL(t *testing.T) {
	n := New(Config{URL: "", StatePath: filepath.Join(t.TempDir(), "state")})
	loc, _ := time.LoadLocation("Europe/Berlin")
	n.Location = loc
	atNine := time.Date(2026, 8, 14, 9, 0, 0, 0, loc)
	if err := n.MaybeSend(models.Snapshot{}, atNine); err != nil {
		t.Fatal(err)
	}
}

func TestMaybeSendRetriesWhenPOSTFails(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	state := filepath.Join(t.TempDir(), "last-daily-notify")
	n := New(Config{URL: srv.URL, StatePath: state, Hostname: "host"})
	loc, _ := time.LoadLocation("Europe/Berlin")
	n.Location = loc
	atNine := time.Date(2026, 8, 14, 9, 0, 0, 0, loc)
	snap := models.Snapshot{PM2: models.PM2Snapshot{Available: true}, Docker: models.DockerSnapshot{Available: true}, Rabbit: models.RabbitSnapshot{Available: true}}

	if err := n.MaybeSend(snap, atNine); err == nil {
		t.Fatal("expected error on 500")
	}
	if _, err := os.Stat(state); !os.IsNotExist(err) {
		t.Fatalf("must not persist last-sent after failure, stat=%v", err)
	}
	if err := n.MaybeSend(snap, atNine.Add(time.Minute)); err == nil {
		t.Fatal("expected retry error")
	}
	if hits != 2 {
		t.Fatalf("expected 2 POSTs after failure, got %d", hits)
	}
}
