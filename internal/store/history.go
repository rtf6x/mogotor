package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rtf6x/mogotor/internal/models"
)

type History struct {
	mu         sync.RWMutex
	retention  time.Duration
	points     []models.SystemSnapshot
	lastNet    netCounters
	persistMu  sync.Mutex
	persistPath string
}

type netCounters struct {
	sent uint64
	recv uint64
	at   time.Time
}

func NewHistory(retention time.Duration, persistPath string) *History {
	return &History{
		retention:   retention,
		persistPath: persistPath,
	}
}

func (h *History) Load() error {
	if h.persistPath == "" {
		return nil
	}

	data, err := os.ReadFile(h.persistPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var points []models.SystemSnapshot
	if err := json.Unmarshal(data, &points); err != nil {
		return err
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	h.points = trim(points, h.retention)
	return nil
}

func (h *History) Persist() error {
	if h.persistPath == "" {
		return nil
	}

	h.persistMu.Lock()
	defer h.persistMu.Unlock()

	h.mu.RLock()
	data, err := json.Marshal(h.points)
	h.mu.RUnlock()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(h.persistPath), 0o755); err != nil {
		return err
	}

	tmp := h.persistPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, h.persistPath)
}

func (h *History) Add(snapshot models.SystemSnapshot) {
	h.mu.Lock()
	defer h.mu.Unlock()

	snapshot = applyNetRates(snapshot, &h.lastNet)
	h.points = append(h.points, snapshot)
	h.points = trim(h.points, h.retention)
}

func (h *History) Points() []models.SystemSnapshot {
	h.mu.RLock()
	defer h.mu.RUnlock()

	out := make([]models.SystemSnapshot, len(h.points))
	copy(out, h.points)
	return out
}

func (h *History) RetentionFrom() time.Time {
	return time.Now().Add(-h.retention)
}

func applyNetRates(snapshot models.SystemSnapshot, last *netCounters) models.SystemSnapshot {
	if !last.at.IsZero() {
		elapsed := snapshot.Timestamp.Sub(last.at).Seconds()
		if elapsed > 0 {
			if snapshot.NetBytesSent >= last.sent {
				snapshot.NetSendBps = float64(snapshot.NetBytesSent-last.sent) / elapsed
			}
			if snapshot.NetBytesRecv >= last.recv {
				snapshot.NetRecvBps = float64(snapshot.NetBytesRecv-last.recv) / elapsed
			}
		}
	}
	last.sent = snapshot.NetBytesSent
	last.recv = snapshot.NetBytesRecv
	last.at = snapshot.Timestamp
	return snapshot
}

func trim(points []models.SystemSnapshot, retention time.Duration) []models.SystemSnapshot {
	if len(points) == 0 {
		return points
	}
	cutoff := time.Now().Add(-retention)
	start := 0
	for start < len(points) && points[start].Timestamp.Before(cutoff) {
		start++
	}
	if start == 0 {
		return points
	}
	return append([]models.SystemSnapshot(nil), points[start:]...)
}
