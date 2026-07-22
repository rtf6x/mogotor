package collector

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/rtf6x/mogotor/internal/models"
)

func CollectDocker(command string) models.DockerSnapshot {
	containers, err := dockerStats(command)
	if err != nil {
		if containers, err = dockerStats("sudo " + command); err != nil {
			return models.DockerSnapshot{Available: false, Error: err.Error()}
		}
	}
	return models.DockerSnapshot{Available: true, Containers: containers}
}

func dockerStats(command string) ([]models.DockerContainer, error) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty docker command")
	}

	args := append(parts[1:], "stats", "--no-stream", "--format", "{{json .}}")
	cmd := exec.Command(parts[0], args...)

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s", trimExecError(err))
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	containers := make([]models.DockerContainer, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var raw dockerStatRaw
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}
		containers = append(containers, raw.toModel())
	}

	if len(containers) == 0 && len(lines) == 1 && lines[0] == "" {
		return []models.DockerContainer{}, nil
	}

	return containers, nil
}

type dockerStatRaw struct {
	Container string `json:"Container"`
	Name        string `json:"Name"`
	CPUPerc     string `json:"CPUPerc"`
	MemUsage    string `json:"MemUsage"`
	MemPerc     string `json:"MemPerc"`
	NetIO       string `json:"NetIO"`
	BlockIO     string `json:"BlockIO"`
	PIDs        string `json:"PIDs"`
}

func (d dockerStatRaw) toModel() models.DockerContainer {
	memUsed, memLimit := parseMemUsage(d.MemUsage)
	netIn, netOut := parseIOPair(d.NetIO)
	blockIn, blockOut := parseIOPair(d.BlockIO)

	pids, _ := strconv.Atoi(strings.TrimSpace(d.PIDs))
	name := strings.TrimPrefix(d.Name, "/")

	return models.DockerContainer{
		ID:          d.Container,
		Name:        name,
		CPUPercent:  parsePercent(d.CPUPerc),
		MemoryBytes: memUsed,
		MemoryLimit: memLimit,
		NetInput:    netIn,
		NetOutput:   netOut,
		BlockInput:  blockIn,
		BlockOutput: blockOut,
		PIDs:        pids,
	}
}

func parsePercent(value string) float64 {
	value = strings.TrimSpace(strings.TrimSuffix(value, "%"))
	parsed, _ := strconv.ParseFloat(value, 64)
	return parsed
}

func parseMemUsage(value string) (uint64, uint64) {
	parts := strings.Split(value, "/")
	if len(parts) != 2 {
		return 0, 0
	}
	return parseSize(strings.TrimSpace(parts[0])), parseSize(strings.TrimSpace(parts[1]))
}

func parseIOPair(value string) (uint64, uint64) {
	parts := strings.Split(value, "/")
	if len(parts) != 2 {
		return 0, 0
	}
	return parseSize(strings.TrimSpace(parts[0])), parseSize(strings.TrimSpace(parts[1]))
}

func parseSize(value string) uint64 {
	value = strings.TrimSpace(value)
	if value == "" || value == "0B" {
		return 0
	}

	multiplier := uint64(1)
	switch {
	case strings.HasSuffix(value, "KiB"):
		multiplier = 1024
		value = strings.TrimSuffix(value, "KiB")
	case strings.HasSuffix(value, "MiB"):
		multiplier = 1024 * 1024
		value = strings.TrimSuffix(value, "MiB")
	case strings.HasSuffix(value, "GiB"):
		multiplier = 1024 * 1024 * 1024
		value = strings.TrimSuffix(value, "GiB")
	case strings.HasSuffix(value, "TiB"):
		multiplier = 1024 * 1024 * 1024 * 1024
		value = strings.TrimSuffix(value, "TiB")
	case strings.HasSuffix(value, "KB"):
		multiplier = 1000
		value = strings.TrimSuffix(value, "KB")
	case strings.HasSuffix(value, "MB"):
		multiplier = 1000 * 1000
		value = strings.TrimSuffix(value, "MB")
	case strings.HasSuffix(value, "GB"):
		multiplier = 1000 * 1000 * 1000
		value = strings.TrimSuffix(value, "GB")
	case strings.HasSuffix(value, "B"):
		value = strings.TrimSuffix(value, "B")
	}

	parsed, _ := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return uint64(parsed * float64(multiplier))
}
