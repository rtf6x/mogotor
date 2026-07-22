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
)

type Config struct {
	Addr            string
	DataDir         string
	MongoURI        string
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

func defaultDataDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return home + "/.local/share/mogotor"
	}
	return "./data"
}
