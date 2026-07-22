package collector

import (
	"context"
	"encoding/json"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/rtf6x/mogotor/internal/models"
)

func CollectServices(names []string) []models.ServiceStatus {
	out := make([]models.ServiceStatus, 0, len(names))
	for _, name := range names {
		out = append(out, collectService(name))
	}
	return out
}

func collectService(name string) models.ServiceStatus {
	status := models.ServiceStatus{Name: name}

	args := []string{
		"show", name,
		"-p", "ActiveState",
		"-p", "SubState",
		"-p", "Description",
		"-p", "MainPID",
		"-p", "MemoryCurrent",
		"--value",
	}
	cmd := exec.Command("systemctl", args...)

	out, err := cmd.Output()
	if err != nil {
		status.Error = trimExecError(err)
		status.Active = "unknown"
		return status
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 5 {
		status.Error = "unexpected systemctl output"
		status.Active = "unknown"
		return status
	}

	status.Active = strings.TrimSpace(lines[0])
	status.SubState = strings.TrimSpace(lines[1])
	status.Description = strings.TrimSpace(lines[2])
	status.MainPID, _ = strconv.Atoi(strings.TrimSpace(lines[3]))
	if mem := strings.TrimSpace(lines[4]); mem != "" && mem != "[not set]" {
		if parsed, err := strconv.ParseUint(mem, 10, 64); err == nil {
			status.MemoryBytes = parsed
		}
	}

	return status
}

func CollectMongo(uri string) models.MongoSnapshot {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "mongosh", uri, "--quiet", "--eval", "JSON.stringify(db.serverStatus())")
	out, err := cmd.Output()
	if err != nil {
		return models.MongoSnapshot{Available: false, Error: trimExecError(err)}
	}

	var raw struct {
		Version string `json:"version"`
		Uptime  int64  `json:"uptime"`
		Connections struct {
			Current int `json:"current"`
		} `json:"connections"`
		Mem struct {
			Resident int `json:"resident"`
		} `json:"mem"`
		Opcounters struct {
			Query int64 `json:"query"`
		} `json:"opcounters"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return models.MongoSnapshot{Available: false, Error: "invalid mongosh output: " + err.Error()}
	}

	return models.MongoSnapshot{
		Available:      true,
		Version:        raw.Version,
		UptimeSeconds:  raw.Uptime,
		Connections:    raw.Connections.Current,
		MemoryResident: raw.Mem.Resident,
		OpsPerSecond:   float64(raw.Opcounters.Query),
	}
}
