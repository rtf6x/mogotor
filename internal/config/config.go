package config

import (
	"os"
	"strconv"
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
	CollectInterval time.Duration
	Retention       time.Duration
	Services        []string
}

func Load() Config {
	return Config{
		Addr:            envOr("MOGOTOR_ADDR", ":"+strconv.Itoa(DefaultPort)),
		DataDir:         envOr("MOGOTOR_DATA_DIR", defaultDataDir()),
		CollectInterval: DefaultCollectInterval,
		Retention:       DefaultRetention,
		Services:        []string{"mongod", "mongodb"},
	}
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
