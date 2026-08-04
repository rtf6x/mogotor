package collector

import (
	"context"
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

func TestCollectRedisIgnoresLegacyQueueKeys(t *testing.T) {
	srv := startMiniRedis(t)
	ctx := context.Background()

	rdb := redis.NewClient(&redis.Options{Addr: srv.Addr(), DB: 3})
	t.Cleanup(func() { _ = rdb.Close() })

	if err := rdb.RPush(ctx, "advice:queue", "job-queued").Err(); err != nil {
		t.Fatalf("queue push: %v", err)
	}
	if err := rdb.Set(ctx, "advice:job:job-queued", `{"id":"job-queued","status":"queued"}`, time.Hour).Err(); err != nil {
		t.Fatalf("set job: %v", err)
	}

	snapshot := CollectRedis(srv.Addr(), "", 4, []int{0, 4})
	if !snapshot.Available {
		t.Fatalf("expected available redis, got error: %s", snapshot.Error)
	}

	var leftover *models.RedisDBSnapshot
	for i := range snapshot.Databases {
		if snapshot.Databases[i].DB == 3 {
			leftover = &snapshot.Databases[i]
			break
		}
	}
	if leftover == nil {
		t.Fatalf("expected probed db 3 with leftover keys: %+v", snapshot.Databases)
	}
	if leftover.Mode == "queue" {
		t.Fatalf("queue scraping should be disabled, got mode=%q", leftover.Mode)
	}
	if leftover.Mode != "summary" {
		t.Fatalf("expected summary mode, got %q", leftover.Mode)
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

	snapshot := CollectRedis(srv.Addr(), "", 4, []int{0, 4})
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
		[]int{0, 4},
	)
	byDB := map[int]redisKeyspaceDB{}
	for _, db := range merged {
		byDB[db.DB] = db
	}
	if _, ok := byDB[0]; !ok || byDB[0].Keys != 0 {
		t.Fatalf("expected watched empty db0, got %+v", byDB[0])
	}
	if byDB[4].Keys != 1 {
		t.Fatalf("expected db4 with keys, got %+v", byDB[4])
	}
	if _, ok := byDB[3]; ok {
		t.Fatalf("did not expect unwatched db3, got %+v", byDB[3])
	}
}

func TestCollectRedisShowsWatchedEmptyDB(t *testing.T) {
	srv := startMiniRedis(t)
	snapshot := CollectRedis(srv.Addr(), "", 4, []int{0, 4})
	if !snapshot.Available {
		t.Fatalf("expected available redis, got error: %s", snapshot.Error)
	}
	seen := map[int]bool{}
	for _, db := range snapshot.Databases {
		seen[db.DB] = true
		if db.DB == 0 {
			if db.Mode != "apod" {
				t.Fatalf("expected apod mode for mad-news db, got %q", db.Mode)
			}
			if db.APOD == nil || db.APOD.CacheKey != madNewsAPODKey {
				t.Fatalf("expected apod payload, got %+v", db.APOD)
			}
		}
		if db.DB == 4 {
			if db.Mode != "summary" {
				t.Fatalf("expected summary mode for mogotor db, got %q", db.Mode)
			}
		}
		if db.Mode == "queue" {
			t.Fatalf("queue mode should not appear, got db %+v", db)
		}
	}
	if !seen[0] || !seen[4] {
		t.Fatalf("expected watched empty dbs in snapshot, got %#v", snapshot.Databases)
	}
}

func TestCollectMadNewsAPOD(t *testing.T) {
	srv := startMiniRedis(t)
	ctx := context.Background()
	rdb := redis.NewClient(&redis.Options{Addr: srv.Addr(), DB: 0})
	t.Cleanup(func() { _ = rdb.Close() })

	raw := []byte(`{"date":"2026-07-30","title":"Red Sun","media_type":"image","copyright":"Test","url":"https://apod.nasa.gov/x.jpg"}`)
	if err := rdb.Set(ctx, madNewsAPODKey, raw, 2*time.Hour).Err(); err != nil {
		t.Fatalf("set apod: %v", err)
	}

	apod := collectMadNewsAPOD(ctx, rdb)
	if apod == nil || !apod.Cached {
		t.Fatalf("expected cached apod, got %+v", apod)
	}
	if apod.CacheKey != madNewsAPODKey {
		t.Fatalf("unexpected cache key: %q", apod.CacheKey)
	}
	if apod.TTLSeconds <= 0 {
		t.Fatalf("expected positive ttl, got %d", apod.TTLSeconds)
	}
}

func TestCollectRedisDoesNotTreatFormerQueueDBsAsQueues(t *testing.T) {
	srv := startMiniRedis(t)
	ctx := context.Background()

	for _, tc := range []struct {
		db  int
		key string
	}{
		{2, "critique:queue"},
		{5, "summary:queue"},
	} {
		rdb := redis.NewClient(&redis.Options{Addr: srv.Addr(), DB: tc.db})
		if err := rdb.RPush(ctx, tc.key, "job-1").Err(); err != nil {
			t.Fatalf("db%d push: %v", tc.db, err)
		}
		_ = rdb.Close()
	}

	snapshot := CollectRedis(srv.Addr(), "", 4, []int{0, 4})
	for _, db := range snapshot.Databases {
		if db.DB == 2 || db.DB == 5 {
			if db.Mode == "queue" {
				t.Fatalf("db%d should not use queue mode: %+v", db.DB, db)
			}
		}
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
