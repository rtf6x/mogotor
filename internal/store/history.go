package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rtf6x/mogotor/internal/models"
)

const historyKey = "mogotor:history"

type History struct {
	mu          sync.RWMutex
	retention   time.Duration
	points      []models.SystemSnapshot
	lastNet     netCounters
	redis       *redis.Client
	legacyPath  string
}

type netCounters struct {
	sent uint64
	recv uint64
	at   time.Time
}

func NewHistory(retention time.Duration, rdb *redis.Client) *History {
	return &History{
		retention: retention,
		redis:     rdb,
	}
}

func (h *History) SetLegacyPath(path string) {
	h.legacyPath = path
}

func (h *History) Load() error {
	if h.redis == nil {
		return nil
	}

	ctx := context.Background()
	cutoff := scoreFor(time.Now().Add(-h.retention))
	results, err := h.redis.ZRangeByScore(ctx, historyKey, &redis.ZRangeBy{
		Min: fmt.Sprintf("%f", cutoff),
		Max: "+inf",
	}).Result()
	if err != nil {
		return err
	}

	points := make([]models.SystemSnapshot, 0, len(results))
	for _, raw := range results {
		var point models.SystemSnapshot
		if err := json.Unmarshal([]byte(raw), &point); err != nil {
			return err
		}
		points = append(points, point)
	}

	if len(points) == 0 && h.legacyPath != "" {
		if err := h.migrateLegacyFile(ctx); err != nil {
			return err
		}
		return h.Load()
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	h.points = trim(points, h.retention)
	restoreLastNet(&h.lastNet, h.points)
	return nil
}

func (h *History) Persist() error {
	if h.redis == nil {
		return nil
	}

	ctx := context.Background()
	cutoff := scoreFor(time.Now().Add(-h.retention))
	return h.redis.ZRemRangeByScore(ctx, historyKey, "-inf", fmt.Sprintf("%f", cutoff)).Err()
}

func (h *History) Add(snapshot models.SystemSnapshot) {
	h.mu.Lock()
	snapshot = applyNetRates(snapshot, &h.lastNet)
	h.points = append(h.points, snapshot)
	h.points = trim(h.points, h.retention)
	h.mu.Unlock()

	if h.redis == nil {
		return
	}

	data, err := json.Marshal(snapshot)
	if err != nil {
		return
	}

	ctx := context.Background()
	pipe := h.redis.Pipeline()
	pipe.ZAdd(ctx, historyKey, redis.Z{
		Score:  scoreFor(snapshot.Timestamp),
		Member: string(data),
	})
	cutoff := scoreFor(time.Now().Add(-h.retention))
	pipe.ZRemRangeByScore(ctx, historyKey, "-inf", fmt.Sprintf("%f", cutoff))
	_, _ = pipe.Exec(ctx)
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

func (h *History) migrateLegacyFile(ctx context.Context) error {
	data, err := os.ReadFile(h.legacyPath)
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
	if len(points) == 0 {
		return nil
	}

	pipe := h.redis.Pipeline()
	for _, point := range points {
		raw, err := json.Marshal(point)
		if err != nil {
			return err
		}
		pipe.ZAdd(ctx, historyKey, redis.Z{
			Score:  scoreFor(point.Timestamp),
			Member: string(raw),
		})
	}
	_, err = pipe.Exec(ctx)
	return err
}

func scoreFor(ts time.Time) float64 {
	return float64(ts.UnixMilli())
}

func restoreLastNet(last *netCounters, points []models.SystemSnapshot) {
	if len(points) == 0 {
		return
	}
	point := points[len(points)-1]
	last.sent = point.NetBytesSent
	last.recv = point.NetBytesRecv
	last.at = point.Timestamp
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
