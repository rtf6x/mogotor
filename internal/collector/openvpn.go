package collector

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/rtf6x/mogotor/internal/models"
)

func CollectOpenVPN(statusPath, serviceName string) models.OpenVPNSnapshot {
	snapshot := models.OpenVPNSnapshot{
		ServiceName: serviceName,
	}

	if serviceName != "" {
		service := collectService(serviceName)
		snapshot.Active = service.Active
		snapshot.SubState = service.SubState
		if service.Error != "" {
			snapshot.Error = service.Error
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
