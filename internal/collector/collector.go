package collector

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/rtf6x/mogotor/internal/config"
	"github.com/rtf6x/mogotor/internal/models"
	"github.com/rtf6x/mogotor/internal/notify"
	"github.com/rtf6x/mogotor/internal/store"
)

type Collector struct {
	cfg      config.Config
	history  *store.History
	latest   *store.Latest
	notifier *notify.Notifier
}

func New(cfg config.Config, history *store.History, latest *store.Latest) *Collector {
	hostname, _ := os.Hostname()
	return &Collector{
		cfg:     cfg,
		history: history,
		latest:  latest,
		notifier: notify.New(notify.Config{
			URL:       cfg.NotifyURL,
			StatePath: filepath.Join(cfg.DataDir, "last-daily-notify"),
			Hostname:  hostname,
		}),
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
	disks := CollectDisks()
	pm2 := CollectPM2("")
	docker := CollectDocker("docker")
	services := CollectServices(c.cfg.Services)
	mongo := CollectMongo(c.cfg.MongoURI)
	dplo := CollectDplo(c.cfg.DploDataDir, c.cfg.DploHealthURL)
	openvpn := CollectOpenVPN(c.cfg.OpenVPNStatusPath, c.cfg.OpenVPNContainerName)
	ssh := CollectSSH()
	fail2ban := CollectFail2ban()
	redisSnap := CollectRedis(c.cfg.RedisAddr, c.cfg.RedisPassword, c.cfg.RedisDB, c.cfg.RedisWatchDBs)
	rabbitSnap := CollectRabbit(c.cfg.RabbitURL, c.cfg.RabbitUser, c.cfg.RabbitPassword)

	snapshot := models.Snapshot{
		Timestamp: now,
		System:    system,
		Disks:     disks,
		PM2:       pm2,
		Docker:    docker,
		Services:  services,
		Dplo:      dplo,
		Mongo:     mongo,
		OpenVPN:   openvpn,
		SSH:       ssh,
		Fail2ban:  fail2ban,
		Redis:     redisSnap,
		Rabbit:    rabbitSnap,
	}

	c.history.Add(system)
	c.latest.Set(snapshot)
	if err := c.notifier.MaybeSend(snapshot, now); err != nil {
		log.Printf("notify: %v", err)
	}
}
