package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("MOGOTOR_ADDR", "")
	t.Setenv("MOGOTOR_DATA_DIR", "")

	cfg := Load()
	if cfg.Addr != ":8188" {
		t.Fatalf("expected default addr :8188, got %s", cfg.Addr)
	}
	if cfg.CollectInterval != time.Minute {
		t.Fatalf("expected 1m interval, got %s", cfg.CollectInterval)
	}
	if len(cfg.Services) != 1 {
		t.Fatalf("expected 1 default service, got %d", len(cfg.Services))
	}
}
