package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("MOGOTOR_ADDR", "")
	t.Setenv("MOGOTOR_DATA_DIR", "")
	t.Setenv("MOGOTOR_REDIS_ADDR", "")
	t.Setenv("REDIS_PASSWORD", "")
	t.Setenv("MOGOTOR_REDIS_DB", "")

	cfg := Load()
	if cfg.Addr != ":8188" {
		t.Fatalf("expected default addr :8188, got %s", cfg.Addr)
	}
	if cfg.RedisAddr != DefaultRedisAddr {
		t.Fatalf("expected default redis addr %s, got %s", DefaultRedisAddr, cfg.RedisAddr)
	}
	if cfg.RedisDB != DefaultRedisDB {
		t.Fatalf("expected default redis db %d, got %d", DefaultRedisDB, cfg.RedisDB)
	}
	if cfg.CollectInterval != time.Minute {
		t.Fatalf("expected 1m interval, got %s", cfg.CollectInterval)
	}
	if len(cfg.Services) != 6 {
		t.Fatalf("expected 6 default services, got %d", len(cfg.Services))
	}
}
