package config

import (
	"os"
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
	t.Setenv("MOGOTOR_OPENVPN_CONTAINER", "")
	t.Setenv("MOGOTOR_OPENVPN_STATUS_PATH", "")

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
	if cfg.OpenVPNContainerName != DefaultOpenVPNContainerName {
		t.Fatalf("expected default openvpn container %s, got %s", DefaultOpenVPNContainerName, cfg.OpenVPNContainerName)
	}
	if len(cfg.Services) != 4 {
		t.Fatalf("expected 4 default services, got %d", len(cfg.Services))
	}
	for _, name := range cfg.Services {
		if name == "mongod" || name == "redis-server" || name == "openvpn@server" {
			t.Fatalf("host %s moved to Docker; should not be in systemd services", name)
		}
	}
}

func TestLoadOpenVPNContainerFromEnv(t *testing.T) {
	t.Setenv("MOGOTOR_OPENVPN_CONTAINER", "vpn")
	cfg := Load()
	if cfg.OpenVPNContainerName != "vpn" {
		t.Fatalf("expected openvpn container vpn, got %s", cfg.OpenVPNContainerName)
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

func TestLoadNotifyURLUnsetDisables(t *testing.T) {
	orig, had := os.LookupEnv("MOGOTOR_NOTIFY_URL")
	os.Unsetenv("MOGOTOR_NOTIFY_URL")
	t.Cleanup(func() {
		if had {
			os.Setenv("MOGOTOR_NOTIFY_URL", orig)
		} else {
			os.Unsetenv("MOGOTOR_NOTIFY_URL")
		}
	})

	cfg := Load()
	if cfg.NotifyURL != "" {
		t.Fatalf("unset MOGOTOR_NOTIFY_URL should disable notify, got %s", cfg.NotifyURL)
	}
}

func TestLoadNotifyURLEmptyDisables(t *testing.T) {
	t.Setenv("MOGOTOR_NOTIFY_URL", "")
	cfg := Load()
	if cfg.NotifyURL != "" {
		t.Fatalf("empty MOGOTOR_NOTIFY_URL should disable notify, got %s", cfg.NotifyURL)
	}
}

func TestLoadNotifyURLFromEnv(t *testing.T) {
	t.Setenv("MOGOTOR_NOTIFY_URL", "https://example.test/hook")
	cfg := Load()
	if cfg.NotifyURL != "https://example.test/hook" {
		t.Fatalf("notify url: got %s", cfg.NotifyURL)
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
