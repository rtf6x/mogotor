package store

import (
	"testing"
	"time"

	"github.com/rtf6x/mogotor/internal/models"
)

func TestHistoryTrimAndNetRates(t *testing.T) {
	h := NewHistory(time.Hour, "")
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
	dir := t.TempDir()
	path := dir + "/history.json"
	h := NewHistory(24*time.Hour, path)

	h.Add(models.SystemSnapshot{
		Timestamp:  time.Now(),
		CPUPercent: 12.5,
	})

	if err := h.Persist(); err != nil {
		t.Fatalf("persist: %v", err)
	}

	loaded := NewHistory(24*time.Hour, path)
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
