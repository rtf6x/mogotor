package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultPort            = 8188
	DefaultCollectInterval = time.Minute
	DefaultRetention       = 24 * time.Hour
	DefaultRedisAddr       = "127.0.0.1:6379"
	DefaultRedisDB         = 4
)

type Config struct {
	Addr            string
	DataDir         string
	MongoURI        string
	RedisAddr       string
	RedisPassword   string
	RedisDB         int
	CollectInterval time.Duration
	Retention       time.Duration
	Services        []string
}

func Load() Config {
	dataDir := envOr("MOGOTOR_DATA_DIR", defaultDataDir())
	return Config{
		Addr:            envOr("MOGOTOR_ADDR", ":"+strconv.Itoa(DefaultPort)),
		DataDir:         dataDir,
		MongoURI:        resolveMongoURI(dataDir),
		RedisAddr:       envOr("MOGOTOR_REDIS_ADDR", DefaultRedisAddr),
		RedisPassword:   os.Getenv("REDIS_PASSWORD"),
		RedisDB:         envIntOr("MOGOTOR_REDIS_DB", DefaultRedisDB),
		CollectInterval: DefaultCollectInterval,
		Retention:       DefaultRetention,
		Services: []string{
			"mongod",
			"nginx",
			"docker",
			"jenkins",
			"redis-server",
			"fail2ban",
		},
	}
}

func resolveMongoURI(dataDir string) string {
	if uri := strings.TrimSpace(os.Getenv("MOGOTOR_MONGO_URI")); uri != "" {
		return uri
	}
	path := filepath.Join(dataDir, "mongo.uri")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOr(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func defaultDataDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return home + "/.local/share/mogotor"
	}
	return "./data"
}
