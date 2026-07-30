package collector

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/rtf6x/mogotor/internal/models"
)

func startMiniRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()

	srv, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(srv.Close)
	return srv
}

func TestParseRedisKeyspace(t *testing.T) {
	info := "# Keyspace\r\ndb0:keys=1,expires=0,avg_ttl=0\r\ndb3:keys=42,expires=40,avg_ttl=3600000\r\n"
	dbs := parseRedisKeyspace(info)
	if len(dbs) != 2 {
		t.Fatalf("expected 2 dbs, got %d", len(dbs))
	}
	if dbs[0].DB != 0 || dbs[0].Keys != 1 {
		t.Fatalf("db0: %+v", dbs[0])
	}
	if dbs[1].DB != 3 || dbs[1].Keys != 42 || dbs[1].Expires != 40 || dbs[1].AvgTTLMs != 3600000 {
		t.Fatalf("db3: %+v", dbs[1])
	}
}

func TestCollectRedisQueueDB(t *testing.T) {
	srv := startMiniRedis(t)
	ctx := context.Background()

	rdb := redis.NewClient(&redis.Options{Addr: srv.Addr(), DB: 3})
	t.Cleanup(func() { _ = rdb.Close() })

	jobQueued := map[string]any{
		"id":      "job-queued",
		"status":  "queued",
		"attempt": 1,
		"prompt":  "secret prompt",
	}
	jobProcessing := map[string]any{
		"id":      "job-processing",
		"status":  "processing",
		"attempt": 2,
		"prompt":  "another secret",
	}
	queuedRaw, _ := json.Marshal(jobQueued)
	processingRaw, _ := json.Marshal(jobProcessing)

	if err := rdb.Set(ctx, "advice:job:job-queued", queuedRaw, time.Hour).Err(); err != nil {
		t.Fatalf("set queued job: %v", err)
	}
	if err := rdb.Set(ctx, "advice:job:job-processing", processingRaw, time.Hour).Err(); err != nil {
		t.Fatalf("set processing job: %v", err)
	}
	if err := rdb.RPush(ctx, "advice:queue", "job-queued", "job-extra").Err(); err != nil {
		t.Fatalf("queue push: %v", err)
	}
	if err := rdb.RPush(ctx, "advice:processing", "job-processing").Err(); err != nil {
		t.Fatalf("processing push: %v", err)
	}

	snapshot := CollectRedis(srv.Addr(), "", 3, []int{0, 1, 2, 3, 4})
	if !snapshot.Available {
		t.Fatalf("expected available redis, got error: %s", snapshot.Error)
	}

	var queueDB *models.RedisDBSnapshot
	for i := range snapshot.Databases {
		if snapshot.Databases[i].DB == 3 {
			queueDB = &snapshot.Databases[i]
			break
		}
	}
	if queueDB == nil {
		t.Fatalf("expected db 3 in snapshot: %+v", snapshot.Databases)
	}
	if queueDB.Mode != "queue" {
		t.Fatalf("expected queue mode, got %q", queueDB.Mode)
	}
	if queueDB.Queue == nil {
		t.Fatal("expected queue details")
	}
	if queueDB.Queue.Pending != 2 || queueDB.Queue.Processing != 1 {
		t.Fatalf("queue counts: pending=%d processing=%d", queueDB.Queue.Pending, queueDB.Queue.Processing)
	}
	if len(queueDB.Queue.Jobs) == 0 {
		t.Fatal("expected job summaries")
	}
	for _, job := range queueDB.Queue.Jobs {
		if job.ID == "job-queued" && job.Status != "queued" {
			t.Fatalf("queued job status: %+v", job)
		}
	}
}

func TestCollectRedisSummaryDB(t *testing.T) {
	srv := startMiniRedis(t)
	ctx := context.Background()

	rdb := redis.NewClient(&redis.Options{Addr: srv.Addr(), DB: 4})
	t.Cleanup(func() { _ = rdb.Close() })

	if err := rdb.ZAdd(ctx, "mogotor:history", redis.Z{Score: 1, Member: `{"cpuPercent":1}`}).Err(); err != nil {
		t.Fatalf("zadd: %v", err)
	}
	if err := rdb.ZAdd(ctx, "mogotor:history", redis.Z{Score: 2, Member: `{"cpuPercent":2}`}).Err(); err != nil {
		t.Fatalf("zadd: %v", err)
	}

	snapshot := CollectRedis(srv.Addr(), "", 4, []int{0, 1, 2, 3, 4})
	if !snapshot.Available {
		t.Fatalf("expected available redis, got error: %s", snapshot.Error)
	}

	var summaryDB *models.RedisDBSnapshot
	for i := range snapshot.Databases {
		if snapshot.Databases[i].DB == 4 {
			summaryDB = &snapshot.Databases[i]
			break
		}
	}
	if summaryDB == nil {
		t.Fatalf("expected db 4 in snapshot: %+v", snapshot.Databases)
	}
	if summaryDB.Mode != "summary" {
		t.Fatalf("expected summary mode, got %q", summaryDB.Mode)
	}
	foundHistory := false
	for _, highlight := range summaryDB.Highlights {
		if highlight == "mogotor:history (zset, 2 points, 24h metrics)" {
			foundHistory = true
		}
	}
	if !foundHistory {
		t.Fatalf("expected history highlight, got %#v", summaryDB.Highlights)
	}
}

func TestMergeRedisDBsIncludesWatchedEmptyDB(t *testing.T) {
	merged := mergeRedisDBs(
		[]redisKeyspaceDB{{DB: 4, Keys: 1}},
		[]redisKeyspaceDB{{DB: 4, Keys: 1}},
		[]int{1, 3, 4},
	)
	byDB := map[int]redisKeyspaceDB{}
	for _, db := range merged {
		byDB[db.DB] = db
	}
	if _, ok := byDB[1]; !ok || byDB[1].Keys != 0 {
		t.Fatalf("expected watched empty db1, got %+v", byDB[1])
	}
	if _, ok := byDB[3]; !ok || byDB[3].Keys != 0 {
		t.Fatalf("expected watched empty db3, got %+v", byDB[3])
	}
	if byDB[4].Keys != 1 {
		t.Fatalf("expected db4 with keys, got %+v", byDB[4])
	}
}

func TestCollectRedisShowsWatchedEmptyDB(t *testing.T) {
	srv := startMiniRedis(t)
	snapshot := CollectRedis(srv.Addr(), "", 4, []int{1, 3, 4})
	if !snapshot.Available {
		t.Fatalf("expected available redis, got error: %s", snapshot.Error)
	}
	seen := map[int]bool{}
	for _, db := range snapshot.Databases {
		seen[db.DB] = true
		if db.DB == 1 && len(db.Highlights) == 0 {
			t.Fatalf("expected empty highlight for db1, got %+v", db)
		}
		if db.DB == 3 {
			if db.Mode != "queue" {
				t.Fatalf("expected queue mode for empty oracle db, got %q", db.Mode)
			}
			if db.Queue == nil {
				t.Fatal("expected queue payload for oracle db")
			}
		}
	}
	if !seen[1] || !seen[3] {
		t.Fatalf("expected watched empty dbs in snapshot, got %#v", snapshot.Databases)
	}
}

func TestCollectOracleStats(t *testing.T) {
	srv := startMiniRedis(t)
	ctx := context.Background()
	rdb := redis.NewClient(&redis.Options{Addr: srv.Addr(), DB: 3})
	t.Cleanup(func() { _ = rdb.Close() })

	if err := rdb.HSet(ctx, oracleStatsKey,
		"enqueued", 12,
		"done", 10,
		"failed", 2,
		"last_done_at", "2026-07-30T08:00:00Z",
		"last_done_job_id", "job-done-1",
	).Err(); err != nil {
		t.Fatalf("hset stats: %v", err)
	}

	stats := collectOracleStats(ctx, rdb)
	if stats == nil {
		t.Fatal("expected stats")
	}
	if stats.Enqueued != 12 || stats.Done != 10 || stats.Failed != 2 {
		t.Fatalf("unexpected counters: %+v", stats)
	}
	if stats.LastDoneJobID != "job-done-1" {
		t.Fatalf("unexpected last done job: %+v", stats)
	}
}

func TestCollectRedisUnavailable(t *testing.T) {
	snapshot := CollectRedis("127.0.0.1:1", "", 4, nil)
	if snapshot.Available {
		t.Fatal("expected unavailable redis")
	}
	if snapshot.Error == "" {
		t.Fatal("expected error message")
	}
}
