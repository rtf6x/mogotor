package notify

import (
	"strings"
	"testing"
	"time"

	"github.com/rtf6x/mogotor/internal/models"
)

func TestFormatDigestIncludesHostDiskRamLoadPM2DockerRabbit(t *testing.T) {
	snap := models.Snapshot{
		System: models.SystemSnapshot{
			MemoryUsedBytes:      3 * 1024 * 1024 * 1024,
			MemoryAvailableBytes: 2 * 1024 * 1024 * 1024,
			MemoryTotalBytes:     4 * 1024 * 1024 * 1024,
			DiskUsedBytes:        17 * 1024 * 1024 * 1024,
			DiskTotalBytes:       38 * 1024 * 1024 * 1024,
			DiskUsedPercent:      47.2,
			Load1:                0.21,
			Load5:                0.30,
			Load15:               0.40,
		},
		PM2: models.PM2Snapshot{
			Available: true,
			Processes: []models.PM2Process{
				{Name: "a", Status: "online"},
				{Name: "b", Status: "online"},
				{Name: "c", Status: "errored"},
			},
		},
		Docker: models.DockerSnapshot{
			Available: true,
			Containers: []models.DockerContainer{
				{Name: "mongo"},
				{Name: "redis"},
				{Name: "rabbitmq"},
			},
		},
		Rabbit: models.RabbitSnapshot{
			Available: true,
			Queues: []models.RabbitQueueSnapshot{
				{Name: "summary.jobs", Consumers: 1},
				{Name: "summary.events.ws", Consumers: 1},
				{Name: "advice.jobs", Consumers: 0},
				{Name: "advice.events.ws", Consumers: 0},
				{Name: "advice.events.mad-news", Consumers: 1},
			},
		},
		GoModel: models.GoModelSnapshot{
			Available: true,
			Models: []models.GoModelModel{
				{Selector: "opencode-zen/big-pickle", Available: true},
				{Selector: "opencode-zen/mimo-v2.5-free", Available: true},
				{Selector: "opencode-zen/laguna-s-2.1-free", Available: false},
			},
		},
	}

	text := FormatDigest("ci.rootfox.cc", snap)
	want := []string{
		"[mogotor] ci.rootfox.cc daily",
		"disk / 47% (17G / 38G)",
		"ram 50% (2G / 4G)",
		"load 0.21 0.30 0.40",
		"pm2 2 online, 1 other",
		"docker 3",
		"rabbit 5 queues, 2 without consumer",
		"llm 2/3 up, down: opencode-zen/laguna-s-2.1-free",
	}
	for _, line := range want {
		if !strings.Contains(text, line) {
			t.Fatalf("missing %q in:\n%s", line, text)
		}
	}
	if strings.Contains(text, "secret") || strings.Contains(text, "password") {
		t.Fatalf("digest must not contain secrets: %s", text)
	}
}

func TestFormatDigestUsesMemAvailableNotSwapInflatedUsed(t *testing.T) {
	snap := models.Snapshot{
		System: models.SystemSnapshot{
			MemoryUsedBytes:      3500 * 1024 * 1024,
			MemoryAvailableBytes: 2 * 1024 * 1024 * 1024,
			MemoryTotalBytes:     4 * 1024 * 1024 * 1024,
		},
	}
	text := FormatDigest("host", snap)
	if !strings.Contains(text, "ram 50%") {
		t.Fatalf("expected ram 50%% from MemAvailable, got:\n%s", text)
	}
}

func TestFormatDigestUnavailablePanels(t *testing.T) {
	text := FormatDigest("host", models.Snapshot{})
	for _, line := range []string{"pm2 down", "docker down", "rabbit down", "llm down"} {
		if !strings.Contains(text, line) {
			t.Fatalf("missing %q in:\n%s", line, text)
		}
	}
}

func TestFormatDigestGoModelAllUp(t *testing.T) {
	text := FormatDigest("host", models.Snapshot{
		GoModel: models.GoModelSnapshot{
			Available: true,
			Models: []models.GoModelModel{
				{Selector: "opencode-zen/big-pickle", Available: true},
				{Selector: "opencode-zen/mimo-v2.5-free", Available: true},
			},
		},
	})
	if !strings.Contains(text, "llm 2/2 up") {
		t.Fatalf("expected all-up llm line, got:\n%s", text)
	}
}

func TestShouldSendDailyAtNineBerlinOnce(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}

	before := time.Date(2026, 8, 14, 8, 59, 0, 0, loc)
	if ShouldSendDaily(before, time.Time{}, loc) {
		t.Fatal("must not send before 09:00 Berlin")
	}

	atNine := time.Date(2026, 8, 14, 9, 0, 0, 0, loc)
	if !ShouldSendDaily(atNine, time.Time{}, loc) {
		t.Fatal("must send at 09:00 Berlin when never sent")
	}

	if ShouldSendDaily(atNine.Add(time.Minute), atNine, loc) {
		t.Fatal("must not send twice the same Berlin day")
	}

	nextMorning := time.Date(2026, 8, 15, 9, 0, 0, 0, loc)
	if !ShouldSendDaily(nextMorning, atNine, loc) {
		t.Fatal("must send again the next Berlin day")
	}
}
