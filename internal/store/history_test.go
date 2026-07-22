package store

import (
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/rtf6x/mogotor/internal/models"
)

func testRedis(t *testing.T) *redis.Client {
	t.Helper()

	srv, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(srv.Close)

	return redis.NewClient(&redis.Options{Addr: srv.Addr(), DB: 4})
}

func TestHistoryTrimAndNetRates(t *testing.T) {
	h := NewHistory(time.Hour, nil)
	now := time.Now()

	h.Add(models.SystemSnapshot{
		Timestamp:    now.Add(-2 * time.Hour),
		NetBytesSent: 1000,
		NetBytesRecv: 2000,
	})
	h.Add(models.SystemSnapshot{
		Timestamp:    now.Add(-30 * time.Minute),
		NetBytesSent: 1600,
		NetBytesRecv: 3200,
	})

	points := h.Points()
	if len(points) != 1 {
		t.Fatalf("expected 1 point after retention trim, got %d", len(points))
	}

	if points[0].NetSendBps <= 0 || points[0].NetRecvBps <= 0 {
		t.Fatalf("expected positive network rates, got send=%f recv=%f", points[0].NetSendBps, points[0].NetRecvBps)
	}
}

func TestHistoryPersistRoundTrip(t *testing.T) {
	rdb := testRedis(t)

	h := NewHistory(24*time.Hour, rdb)
	h.Add(models.SystemSnapshot{
		Timestamp:  time.Now(),
		CPUPercent: 12.5,
	})

	if err := h.Persist(); err != nil {
		t.Fatalf("persist: %v", err)
	}

	loaded := NewHistory(24*time.Hour, rdb)
	if err := loaded.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}

	points := loaded.Points()
	if len(points) != 1 {
		t.Fatalf("expected 1 point, got %d", len(points))
	}
	if points[0].CPUPercent != 12.5 {
		t.Fatalf("unexpected cpu value: %v", points[0].CPUPercent)
	}
}

func TestHistoryMigratesLegacyFile(t *testing.T) {
	rdb := testRedis(t)
	dir := t.TempDir()
	path := dir + "/history.json"

	legacy := []byte(`[{"timestamp":"2026-07-22T10:00:00Z","cpuPercent":42.1}]`)
	if err := os.WriteFile(path, legacy, 0o644); err != nil {
		t.Fatalf("write legacy file: %v", err)
	}

	h := NewHistory(24*time.Hour, rdb)
	h.SetLegacyPath(path)
	if err := h.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}

	points := h.Points()
	if len(points) != 1 {
		t.Fatalf("expected 1 migrated point, got %d", len(points))
	}
	if points[0].CPUPercent != 42.1 {
		t.Fatalf("unexpected cpu value: %v", points[0].CPUPercent)
	}
}
