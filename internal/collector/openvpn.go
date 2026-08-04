package collector

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/rtf6x/mogotor/internal/models"
)

func CollectOpenVPN(statusPath, containerName string) models.OpenVPNSnapshot {
	return collectOpenVPN(statusPath, containerName, inspectOpenVPNContainer)
}

func collectOpenVPN(statusPath, containerName string, inspect func(string) (string, error)) models.OpenVPNSnapshot {
	snapshot := models.OpenVPNSnapshot{
		ServiceName: containerName,
		Clients:     []string{},
	}

	if containerName != "" {
		status, err := inspect(containerName)
		snapshot.Active, snapshot.SubState = openVPNStateFromDockerStatus(status)
		if err != nil {
			snapshot.Error = err.Error()
			if snapshot.Active == "" {
				snapshot.Active = "inactive"
			}
		}
	}

	if statusPath == "" {
		if snapshot.Error == "" {
			snapshot.Error = "openvpn status path not configured"
		}
		return snapshot
	}

	data, err := readOpenVPNStatus(statusPath)
	if err != nil {
		if snapshot.Error == "" {
			snapshot.Error = err.Error()
		}
		return snapshot
	}

	clients, updatedAt := parseOpenVPNStatus(string(data))
	snapshot.Available = true
	snapshot.UpdatedAt = updatedAt
	snapshot.Clients = clients
	return snapshot
}

func openVPNStateFromDockerStatus(status string) (active, subState string) {
	status = strings.TrimSpace(status)
	if status == "" {
		return "inactive", "unknown"
	}
	if status == "running" {
		return "active", status
	}
	return "inactive", status
}

func inspectOpenVPNContainer(name string) (string, error) {
	status, err := dockerContainerStatus("docker", name)
	if err != nil {
		status, err = dockerContainerStatus("sudo docker", name)
	}
	return status, err
}

func dockerContainerStatus(command, name string) (string, error) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty docker command")
	}
	args := append(parts[1:], "inspect", "--format", "{{.State.Status}}", name)
	cmd := exec.Command(parts[0], args...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s", trimExecError(err))
	}
	return strings.TrimSpace(string(out)), nil
}

func readOpenVPNStatus(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		return data, nil
	}

	cmd := exec.Command("sudo", "cat", path)
	out, sudoErr := cmd.Output()
	if sudoErr == nil {
		return out, nil
	}

	return nil, fmt.Errorf("read openvpn status: %s", trimExecError(err))
}

func parseOpenVPNStatus(content string) (clients []string, updatedAt string) {
	names := make(map[string]struct{})
	section := ""

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		switch line {
		case "OpenVPN CLIENT LIST":
			section = "clients"
			continue
		case "ROUTING TABLE":
			section = "routing"
			continue
		case "GLOBAL STATS", "END":
			section = ""
			continue
		}

		if strings.HasPrefix(line, "Updated,") {
			updatedAt = strings.TrimPrefix(line, "Updated,")
			continue
		}

		if strings.Contains(line, "Common Name") && strings.Contains(line, ",") {
			continue
		}
		if strings.HasPrefix(line, "Virtual Address,") {
			continue
		}

		fields := strings.Split(line, ",")
		if len(fields) < 2 {
			continue
		}

		var name string
		switch section {
		case "clients":
			name = strings.TrimSpace(fields[0])
		case "routing":
			name = strings.TrimSpace(fields[1])
		default:
			continue
		}

		if name == "" || name == "Common Name" {
			continue
		}
		names[name] = struct{}{}
	}

	clients = make([]string, 0, len(names))
	for name := range names {
		clients = append(clients, name)
	}
	sort.Strings(clients)
	return clients, updatedAt
}
