package collector

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rtf6x/mogotor/internal/models"
)

const defaultDploDataDir = "/var/lib/dplo"

func CollectDplo(dataDir, healthURL string) models.DploSnapshot {
	if dataDir == "" {
		dataDir = defaultDploDataDir
	}

	snapshot := models.DploSnapshot{DataDir: dataDir}

	pid, err := dploPID()
	if err != nil {
		snapshot.Error = err.Error()
	} else {
		snapshot.Available = true
		snapshot.PID = pid
		snapshot.RSSBytes = processMemoryBytes(pid)
		snapshot.CgroupBytes = serviceMemoryBytes("dplo")
	}

	snapshot.APIHealthy = dploHealthCheck(healthURL)
	snapshot.ProjectCount, snapshot.EnabledCount = countDploProjects(dataDir)
	snapshot.RunCount, snapshot.RunningCount = countDploRuns(dataDir)
	snapshot.DataBytes = dirSizeBytes(dataDir)

	if snapshot.RunningCount > 0 {
		snapshot.RunnerBusy = true
		if projectID, runID := activeDploRun(dataDir); projectID != "" {
			snapshot.ActiveProjectID = projectID
			snapshot.ActiveRunID = runID
		}
	}

	return snapshot
}

func dploPID() (int, error) {
	out, err := exec.Command("systemctl", "show", "dplo", "-p", "MainPID", "--value").Output()
	if err != nil {
		return 0, fmt.Errorf("%s", trimExecError(err))
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil || pid <= 0 {
		return 0, fmt.Errorf("dplo is not running")
	}
	return pid, nil
}

func dploHealthCheck(healthURL string) bool {
	if healthURL == "" {
		return false
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(healthURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256))
	if err != nil {
		return false
	}
	return strings.Contains(string(body), "ok")
}

type dploProjectMeta struct {
	Enabled bool `json:"enabled"`
}

type dploRunMeta struct {
	ProjectID string `json:"projectId"`
	Status    string `json:"status"`
}

func countDploProjects(dataDir string) (total int, enabled int) {
	entries, err := os.ReadDir(filepath.Join(dataDir, "projects"))
	if err != nil {
		return 0, 0
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(dataDir, "projects", entry.Name(), "project.json")
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		total++
		var meta dploProjectMeta
		if json.Unmarshal(data, &meta) == nil && meta.Enabled {
			enabled++
		}
	}
	return total, enabled
}

func countDploRuns(dataDir string) (total int, running int) {
	entries, err := os.ReadDir(filepath.Join(dataDir, "runs"))
	if err != nil {
		return 0, 0
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(dataDir, "runs", entry.Name(), "meta.json")
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		total++
		var meta dploRunMeta
		if json.Unmarshal(data, &meta) != nil {
			continue
		}
		if meta.Status == "running" || meta.Status == "queued" {
			running++
		}
	}
	return total, running
}

func activeDploRun(dataDir string) (projectID, runID string) {
	entries, err := os.ReadDir(filepath.Join(dataDir, "runs"))
	if err != nil {
		return "", ""
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(dataDir, "runs", entry.Name(), "meta.json")
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var meta dploRunMeta
		if json.Unmarshal(data, &meta) != nil {
			continue
		}
		if meta.Status == "running" {
			return meta.ProjectID, entry.Name()
		}
	}
	return "", ""
}

func serviceMemoryBytes(name string) uint64 {
	out, err := exec.Command("systemctl", "show", name, "-p", "MemoryCurrent", "--value").Output()
	if err != nil {
		return 0
	}
	value := strings.TrimSpace(string(out))
	if value == "" || value == "[not set]" {
		return 0
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func dirSizeBytes(path string) uint64 {
	cmd := exec.Command("du", "-sb", path)
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		return 0
	}
	size, err := strconv.ParseUint(fields[0], 10, 64)
	if err != nil {
		return 0
	}
	return size
}
