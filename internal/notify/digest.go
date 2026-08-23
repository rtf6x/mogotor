package notify

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/rtf6x/mogotor/internal/models"
)

const dailyHour = 9

func FormatDigest(hostname string, snap models.Snapshot) string {
	lines := []string{
		fmt.Sprintf("[mogotor] %s daily", hostname),
		diskLine(snap.System),
		ramLine(snap.System),
		fmt.Sprintf("load %.2f %.2f %.2f", snap.System.Load1, snap.System.Load5, snap.System.Load15),
		pm2Line(snap.PM2),
		dockerLine(snap.Docker),
		rabbitLine(snap.Rabbit),
		goModelLine(snap.GoModel),
	}
	return strings.Join(lines, "\n")
}

func ShouldSendDaily(now, lastSent time.Time, loc *time.Location) bool {
	if loc == nil {
		loc = time.UTC
	}
	local := now.In(loc)
	if local.Hour() < dailyHour {
		return false
	}
	if lastSent.IsZero() {
		return true
	}
	last := lastSent.In(loc)
	ly, lm, ld := last.Date()
	ny, nm, nd := local.Date()
	return ny != ly || nm != lm || nd != ld
}

func diskLine(s models.SystemSnapshot) string {
	pct := math.Round(s.DiskUsedPercent)
	return fmt.Sprintf("disk / %.0f%% (%s / %s)", pct, formatBytes(s.DiskUsedBytes), formatBytes(s.DiskTotalBytes))
}

func ramLine(s models.SystemSnapshot) string {
	if s.MemoryTotalBytes == 0 {
		return "ram n/a"
	}
	used := s.MemoryUsedBytes
	if s.MemoryAvailableBytes > 0 && s.MemoryAvailableBytes <= s.MemoryTotalBytes {
		used = s.MemoryTotalBytes - s.MemoryAvailableBytes
	}
	pct := math.Round(float64(used) / float64(s.MemoryTotalBytes) * 100)
	return fmt.Sprintf("ram %.0f%% (%s / %s)", pct, formatBytes(used), formatBytes(s.MemoryTotalBytes))
}

func pm2Line(pm2 models.PM2Snapshot) string {
	if !pm2.Available {
		return "pm2 down"
	}
	online := 0
	for _, p := range pm2.Processes {
		if p.Status == "online" {
			online++
		}
	}
	other := len(pm2.Processes) - online
	return fmt.Sprintf("pm2 %d online, %d other", online, other)
}

func dockerLine(docker models.DockerSnapshot) string {
	if !docker.Available {
		return "docker down"
	}
	return fmt.Sprintf("docker %d", len(docker.Containers))
}

func rabbitLine(rabbit models.RabbitSnapshot) string {
	if !rabbit.Available {
		return "rabbit down"
	}
	idle := 0
	for _, q := range rabbit.Queues {
		if q.Consumers == 0 {
			idle++
		}
	}
	return fmt.Sprintf("rabbit %d queues, %d without consumer", len(rabbit.Queues), idle)
}

func goModelLine(gm models.GoModelSnapshot) string {
	if !gm.Available {
		return "llm down"
	}
	up := 0
	downNames := make([]string, 0)
	for _, m := range gm.Models {
		if m.Available {
			up++
			continue
		}
		downNames = append(downNames, m.Selector)
	}
	total := len(gm.Models)
	if total == 0 {
		return "llm 0 enabled"
	}
	if len(downNames) == 0 {
		return fmt.Sprintf("llm %d/%d up", up, total)
	}
	shown := downNames
	if len(shown) > 3 {
		shown = append(shown[:3], "...")
	}
	return fmt.Sprintf("llm %d/%d up, down: %s", up, total, strings.Join(shown, ", "))
}

func formatBytes(n uint64) string {
	const (
		mib = 1024 * 1024
		gib = 1024 * 1024 * 1024
	)
	switch {
	case n >= gib:
		return fmt.Sprintf("%.0fG", float64(n)/float64(gib))
	case n >= mib:
		return fmt.Sprintf("%.0fM", float64(n)/float64(mib))
	default:
		return fmt.Sprintf("%dB", n)
	}
}
