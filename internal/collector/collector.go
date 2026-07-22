package collector

import (
	"context"
	"time"

	"github.com/rtf6x/mogotor/internal/config"
	"github.com/rtf6x/mogotor/internal/models"
	"github.com/rtf6x/mogotor/internal/store"
)

type Collector struct {
	cfg     config.Config
	history *store.History
	latest  *store.Latest
}

func New(cfg config.Config, history *store.History, latest *store.Latest) *Collector {
	return &Collector{
		cfg:     cfg,
		history: history,
		latest:  latest,
	}
}

func (c *Collector) Run(ctx context.Context) {
	c.collect()
	ticker := time.NewTicker(c.cfg.CollectInterval)
	defer ticker.Stop()

	persistTicker := time.NewTicker(5 * time.Minute)
	defer persistTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			_ = c.history.Persist()
			return
		case <-ticker.C:
			c.collect()
		case <-persistTicker.C:
			_ = c.history.Persist()
		}
	}
}

func (c *Collector) collect() {
	now := time.Now()
	system := CollectSystem(now)
	pm2 := CollectPM2("")
	docker := CollectDocker("docker")
	services := CollectServices(c.cfg.Services)
	mongo := CollectMongo(c.cfg.MongoURI)

	snapshot := models.Snapshot{
		Timestamp: now,
		System:    system,
		PM2:       pm2,
		Docker:    docker,
		Services:  services,
		Mongo:     mongo,
	}

	c.history.Add(system)
	c.latest.Set(snapshot)
}
