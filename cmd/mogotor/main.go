package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rtf6x/mogotor/internal/collector"
	"github.com/rtf6x/mogotor/internal/config"
	"github.com/rtf6x/mogotor/internal/server"
	"github.com/rtf6x/mogotor/internal/store"
)

func main() {
	cfg := config.Load()
	log.Printf("mogotor starting on %s (data: %s, redis: %s db=%d)", cfg.Addr, cfg.DataDir, cfg.RedisAddr, cfg.RedisDB)

	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		log.Fatalf("create data dir: %v", err)
	}

	ctx := context.Background()
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("redis: %v", err)
	}
	defer rdb.Close()

	history := store.NewHistory(cfg.Retention, rdb)
	history.SetLegacyPath(filepath.Join(cfg.DataDir, "history.json"))
	if err := history.Load(); err != nil {
		log.Printf("warning: could not load history: %v", err)
	}

	latest := store.NewLatest()
	col := collector.New(cfg, history, latest)

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go col.Run(runCtx)

	srv := server.New(history, latest)
	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("listening on http://localhost%s", cfg.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("shutting down")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = httpServer.Shutdown(shutdownCtx)
}
