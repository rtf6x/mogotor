package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("MOGOTOR_ADDR", "")
	t.Setenv("MOGOTOR_DATA_DIR", "")
	t.Setenv("MOGOTOR_REDIS_ADDR", "")
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("REDIS_PASSWORD", "")
	t.Setenv("MOGOTOR_REDIS_DB", "")
	t.Setenv("MOGOTOR_REDIS_WATCH_DBS", "")
	t.Setenv("MOGOTOR_RABBIT_URL", "")
	t.Setenv("MOGOTOR_RABBIT_USER", "")
	t.Setenv("MOGOTOR_RABBIT_PASSWORD", "")

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
	if len(cfg.RedisWatchDBs) != 2 || cfg.RedisWatchDBs[0] != 0 || cfg.RedisWatchDBs[1] != 4 {
		t.Fatalf("expected default watch dbs [0 4], got %v", cfg.RedisWatchDBs)
	}
	if cfg.RabbitURL != DefaultRabbitURL {
		t.Fatalf("expected default rabbit url %s, got %s", DefaultRabbitURL, cfg.RabbitURL)
	}
	if cfg.RabbitUser != DefaultRabbitUser {
		t.Fatalf("expected default rabbit user %s, got %s", DefaultRabbitUser, cfg.RabbitUser)
	}
	if cfg.RabbitPassword != "" {
		t.Fatalf("expected empty rabbit password by default")
	}
	if cfg.CollectInterval != time.Minute {
		t.Fatalf("expected 1m interval, got %s", cfg.CollectInterval)
	}
	if len(cfg.Services) != 5 {
		t.Fatalf("expected 5 default services, got %d", len(cfg.Services))
	}
	for _, name := range cfg.Services {
		if name == "mongod" || name == "redis-server" {
			t.Fatalf("host %s moved to Docker; should not be in systemd services", name)
		}
	}
}

func TestLoadRabbitFromEnv(t *testing.T) {
	t.Setenv("MOGOTOR_RABBIT_URL", "http://127.0.0.1:15672/rabbit")
	t.Setenv("MOGOTOR_RABBIT_USER", "monitor")
	t.Setenv("MOGOTOR_RABBIT_PASSWORD", "pw")

	cfg := Load()
	if cfg.RabbitURL != "http://127.0.0.1:15672/rabbit" {
		t.Fatalf("rabbit url: got %s", cfg.RabbitURL)
	}
	if cfg.RabbitUser != "monitor" || cfg.RabbitPassword != "pw" {
		t.Fatalf("rabbit auth: user=%q pass=%q", cfg.RabbitUser, cfg.RabbitPassword)
	}
}

func TestLoadRedisAddrFromSharedEnv(t *testing.T) {
	t.Setenv("MOGOTOR_REDIS_ADDR", "")
	t.Setenv("REDIS_ADDR", "llm.rootfox.cc:63719")

	cfg := Load()
	if cfg.RedisAddr != "llm.rootfox.cc:63719" {
		t.Fatalf("expected shared redis addr, got %s", cfg.RedisAddr)
	}
}
