package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rtf6x/mogotor/internal/models"
)

const (
	redisCollectTimeout = 5 * time.Second
	redisMaxJobSummaries = 8

	oracleQueueKey      = "advice:queue"
	oracleProcessingKey = "advice:processing"
	oracleJobPrefix     = "advice:job:"
	oracleEventsChannel = "advice:events"
	mogotorHistoryKey   = "mogotor:history"
	redisMemoryKeyLimit = 500
)

var knownRedisDBLabels = map[int]string{
	1: "mad-news-bot",
	3: "bad-advice-oracle",
	4: "mogotor",
}

type redisKeyspaceDB struct {
	DB       int
	Keys     int
	Expires  int
	AvgTTLMs int64
}

type redisJobRaw struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Attempt int    `json:"attempt"`
}

func CollectRedis(addr, password string, selfDB int, watchDBs []int) models.RedisSnapshot {
	if addr == "" {
		return models.RedisSnapshot{
			Available: false,
			Error:     "redis addr not configured",
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisCollectTimeout)
	defer cancel()

	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
	})
	defer client.Close()

	info, err := client.Info(ctx).Result()
	if err != nil {
		return models.RedisSnapshot{
			Available: false,
			Error:     err.Error(),
		}
	}

	snapshot := models.RedisSnapshot{
		Available:        true,
		Version:          redisInfoValue(info, "redis_version"),
		UsedMemoryBytes:  parseRedisUint(redisInfoValue(info, "used_memory")),
		ConnectedClients: parseRedisInt(redisInfoValue(info, "connected_clients")),
		UptimeSeconds:    parseRedisInt64(redisInfoValue(info, "uptime_in_seconds")),
	}

	keyspace := mergeRedisDBs(parseRedisKeyspace(info), probeRedisDBs(ctx, addr, password, 16), watchDBs)
	if len(keyspace) == 0 {
		return snapshot
	}

	watched := watchedDBSet(watchDBs)
	databases := make([]models.RedisDBSnapshot, 0, len(keyspace))
	for _, db := range keyspace {
		if db.Keys == 0 && !watched[db.DB] {
			continue
		}
		dbClient := redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: password,
			DB:       db.DB,
		})
		dbSnapshot := collectRedisDB(ctx, dbClient, db, selfDB)
		_ = dbClient.Close()
		databases = append(databases, dbSnapshot)
	}

	snapshot.Databases = databases
	return snapshot
}

func collectRedisDB(ctx context.Context, client *redis.Client, meta redisKeyspaceDB, selfDB int) models.RedisDBSnapshot {
	snapshot := models.RedisDBSnapshot{
		DB:       meta.DB,
		Keys:     meta.Keys,
		Expires:  meta.Expires,
		AvgTTLMs: meta.AvgTTLMs,
		Mode:     "summary",
	}
	if label := knownRedisDBLabels[meta.DB]; label != "" {
		snapshot.Label = label
	}

	if meta.Keys > 0 {
		memoryBytes, approx := collectDBMemoryBytes(ctx, client)
		snapshot.MemoryBytes = memoryBytes
		snapshot.MemoryApprox = approx
	}

	if isOracleQueueDB(ctx, client) {
		snapshot.Mode = "queue"
		snapshot.Label = "bad-advice-oracle"
		snapshot.Queue = collectOracleQueue(ctx, client)
		return snapshot
	}

	snapshot.Highlights = collectRedisHighlights(ctx, client, selfDB, meta.DB)
	if meta.DB == selfDB && snapshot.Label == "" {
		snapshot.Label = "mogotor"
	}
	if meta.Keys == 0 {
		snapshot.Highlights = []string{"empty"}
	}
	return snapshot
}

func isOracleQueueDB(ctx context.Context, client *redis.Client) bool {
	queueExists, err := client.Exists(ctx, oracleQueueKey, oracleProcessingKey).Result()
	if err != nil {
		return false
	}
	return queueExists > 0
}

func collectOracleQueue(ctx context.Context, client *redis.Client) *models.RedisQueueSnapshot {
	pending, _ := client.LLen(ctx, oracleQueueKey).Result()
	processing, _ := client.LLen(ctx, oracleProcessingKey).Result()

	pendingIDs, _ := client.LRange(ctx, oracleQueueKey, 0, -1).Result()
	processingIDs, _ := client.LRange(ctx, oracleProcessingKey, 0, -1).Result()
	jobIDs := uniqueStrings(append(processingIDs, pendingIDs...))

	jobs := make([]models.RedisJobSummary, 0, redisMaxJobSummaries)
	for _, id := range jobIDs {
		if len(jobs) >= redisMaxJobSummaries {
			break
		}
		if summary, ok := loadOracleJobSummary(ctx, client, id); ok {
			jobs = append(jobs, summary)
		} else {
			jobs = append(jobs, models.RedisJobSummary{ID: id, Status: "unknown"})
		}
	}

	jobCount, _ := client.Eval(ctx, `
local n = 0
local cursor = "0"
repeat
  local res = redis.call("SCAN", cursor, "MATCH", "advice:job:*", "COUNT", 100)
  cursor = res[1]
  n = n + #res[2]
until cursor == "0"
return n
`, []string{}).Int()

	queue := &models.RedisQueueSnapshot{
		Name:       "advice",
		Pending:    int(pending),
		Processing: int(processing),
		JobCount:   jobCount,
		Jobs:       jobs,
		PubSubChan: oracleEventsChannel,
	}

	if subs, err := client.PubSubNumSub(ctx, oracleEventsChannel).Result(); err == nil {
		if count, ok := subs[oracleEventsChannel]; ok {
			queue.Subscribers = int(count)
		}
	}

	return queue
}

func loadOracleJobSummary(ctx context.Context, client *redis.Client, id string) (models.RedisJobSummary, bool) {
	raw, err := client.Get(ctx, oracleJobPrefix+id).Bytes()
	if err != nil {
		return models.RedisJobSummary{}, false
	}

	var job redisJobRaw
	if err := json.Unmarshal(raw, &job); err != nil {
		return models.RedisJobSummary{ID: id, Status: "unknown"}, true
	}
	if job.ID == "" {
		job.ID = id
	}
	return models.RedisJobSummary{
		ID:      job.ID,
		Status:  job.Status,
		Attempt: job.Attempt,
	}, true
}

func collectRedisHighlights(ctx context.Context, client *redis.Client, selfDB, db int) []string {
	highlights := make([]string, 0, 2)

	if exists, _ := client.Exists(ctx, mogotorHistoryKey).Result(); exists > 0 {
		if count, err := client.ZCard(ctx, mogotorHistoryKey).Result(); err == nil {
			highlights = append(highlights, fmt.Sprintf("%s (zset, %d points, 24h metrics)", mogotorHistoryKey, count))
		}
	}

	if db == selfDB && len(highlights) == 0 {
		highlights = append(highlights, "mogotor history db")
	}

	return highlights
}

func collectDBMemoryBytes(ctx context.Context, client *redis.Client) (uint64, bool) {
	total, err := client.Eval(ctx, `
local cursor = "0"
local total = 0
local scanned = 0
repeat
  local res = redis.call("SCAN", cursor, "MATCH", "*", "COUNT", 100)
  cursor = res[1]
  for _, key in ipairs(res[2]) do
    local usage = redis.call("MEMORY", "USAGE", key)
    if usage then
      total = total + usage
    end
    scanned = scanned + 1
    if scanned >= tonumber(ARGV[1]) then
      return {total, 1}
    end
  end
until cursor == "0"
return {total, 0}
`, []string{}, redisMemoryKeyLimit).Result()
	if err != nil {
		return 0, false
	}
	values, ok := total.([]any)
	if !ok || len(values) != 2 {
		return 0, false
	}
	bytes, _ := values[0].(int64)
	approx, _ := values[1].(int64)
	return uint64(bytes), approx == 1
}

func mergeRedisDBs(keyspace, probed []redisKeyspaceDB, watchDBs []int) []redisKeyspaceDB {
	byDB := make(map[int]redisKeyspaceDB)
	for _, db := range keyspace {
		byDB[db.DB] = db
	}
	for _, db := range probed {
		existing, ok := byDB[db.DB]
		if !ok || db.Keys > existing.Keys {
			if ok {
				db.Expires = existing.Expires
				db.AvgTTLMs = existing.AvgTTLMs
			}
			byDB[db.DB] = db
		}
	}
	for _, dbNum := range watchDBs {
		if _, ok := byDB[dbNum]; !ok {
			byDB[dbNum] = redisKeyspaceDB{DB: dbNum}
		}
	}
	out := make([]redisKeyspaceDB, 0, len(byDB))
	for _, db := range byDB {
		out = append(out, db)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].DB < out[j].DB
	})
	return out
}

func watchedDBSet(watchDBs []int) map[int]bool {
	if len(watchDBs) == 0 {
		return map[int]bool{0: true, 1: true, 2: true, 3: true, 4: true}
	}
	out := make(map[int]bool, len(watchDBs))
	for _, db := range watchDBs {
		out[db] = true
	}
	return out
}

func parseRedisKeyspace(info string) []redisKeyspaceDB {
	var dbs []redisKeyspaceDB
	for _, line := range strings.Split(info, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "db") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		dbNum, err := strconv.Atoi(strings.TrimPrefix(parts[0], "db"))
		if err != nil {
			continue
		}

		entry := redisKeyspaceDB{DB: dbNum}
		for _, field := range strings.Split(parts[1], ",") {
			kv := strings.SplitN(strings.TrimSpace(field), "=", 2)
			if len(kv) != 2 {
				continue
			}
			switch kv[0] {
			case "keys":
				entry.Keys = parseRedisInt(kv[1])
			case "expires":
				entry.Expires = parseRedisInt(kv[1])
			case "avg_ttl":
				entry.AvgTTLMs = parseRedisInt64(kv[1])
			}
		}
		dbs = append(dbs, entry)
	}

	sort.Slice(dbs, func(i, j int) bool {
		return dbs[i].DB < dbs[j].DB
	})
	return dbs
}

func probeRedisDBs(ctx context.Context, addr, password string, maxDB int) []redisKeyspaceDB {
	dbs := make([]redisKeyspaceDB, 0, 4)
	for db := 0; db < maxDB; db++ {
		client := redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: password,
			DB:       db,
		})
		size, err := client.DBSize(ctx).Result()
		_ = client.Close()
		if err != nil || size == 0 {
			continue
		}
		dbs = append(dbs, redisKeyspaceDB{
			DB:   db,
			Keys: int(size),
		})
	}
	return dbs
}

func redisInfoValue(info, key string) string {
	prefix := key + ":"
	for _, line := range strings.Split(info, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

func parseRedisInt(value string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(value))
	return n
}

func parseRedisInt64(value string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return n
}

func parseRedisUint(value string) uint64 {
	n, _ := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	return n
}

func uniqueStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}
