package collector

import (
	"os/exec"
	"strconv"
	"strings"

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
	}
	cmd := exec.Command("systemctl", args...)

	out, err := cmd.Output()
	if err != nil {
		status.Error = trimExecError(err)
		status.Active = "unknown"
		return status
	}

	values := parseSystemctlProperties(string(out))
	status.Active = values["ActiveState"]
	status.SubState = values["SubState"]
	status.Description = values["Description"]
	status.MainPID, _ = strconv.Atoi(values["MainPID"])
	if mem := values["MemoryCurrent"]; mem != "" && mem != "[not set]" {
		if parsed, err := strconv.ParseUint(mem, 10, 64); err == nil {
			status.MemoryBytes = parsed
		}
	}
	if status.MemoryBytes == 0 && status.MainPID > 0 {
		status.MemoryBytes = processMemoryBytes(status.MainPID)
	}

	return status
}

func parseSystemctlProperties(output string) map[string]string {
	values := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[key] = value
	}
	return values
}
