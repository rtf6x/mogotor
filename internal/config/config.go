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
	DefaultRedisAddr       = "127.0.0.1:63719"
	DefaultRedisDB         = 4
	DefaultDploDataDir     = "/var/lib/dplo"
	DefaultDploHealthURL   = "http://127.0.0.1:8090/health"
)

type Config struct {
	Addr            string
	DataDir         string
	MongoURI        string
	RedisAddr       string
	RedisPassword   string
	RedisDB         int
	DploDataDir     string
	DploHealthURL   string
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
		RedisAddr:       resolveRedisAddr(),
		RedisPassword:   os.Getenv("REDIS_PASSWORD"),
		RedisDB:         envIntOr("MOGOTOR_REDIS_DB", DefaultRedisDB),
		DploDataDir:     envOr("MOGOTOR_DPLO_DATA_DIR", DefaultDploDataDir),
		DploHealthURL:   envOr("MOGOTOR_DPLO_HEALTH_URL", DefaultDploHealthURL),
		CollectInterval: DefaultCollectInterval,
		Retention:       DefaultRetention,
		Services: []string{
			"mongod",
			"nginx",
			"docker",
			"dplo",
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

func resolveRedisAddr() string {
	if addr := strings.TrimSpace(os.Getenv("MOGOTOR_REDIS_ADDR")); addr != "" {
		return addr
	}
	if addr := strings.TrimSpace(os.Getenv("REDIS_ADDR")); addr != "" {
		return addr
	}
	return DefaultRedisAddr
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
