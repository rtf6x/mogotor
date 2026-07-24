package collector

import (
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/rtf6x/mogotor/internal/models"
)

const (
	fail2banServiceName = "fail2ban"
	fail2banLogPath     = "/var/log/fail2ban.log"
	fail2banLogTail     = 2000
)

var (
	fail2banJailListRE = regexp.MustCompile(`(?i)Jail list:\s*(.*)$`)
	fail2banIntFieldRE = regexp.MustCompile(`(?i)(Currently failed|Total failed|Currently banned|Total banned):\s*(\d+)`)
	fail2banBannedRE   = regexp.MustCompile(`(?i)Banned IP list:\s*(.*)$`)
	fail2banLogBanRE   = regexp.MustCompile(`\[([^\]]+)\]\s+(Ban|Unban)\s+(\S+)`)
)

func CollectFail2ban() models.Fail2banSnapshot {
	snapshot := models.Fail2banSnapshot{}

	service := collectService(fail2banServiceName)
	snapshot.Active = service.Active
	snapshot.SubState = service.SubState
	if service.Error != "" {
		snapshot.Error = service.Error
	}

	if jails, err := collectFail2banFromClient(); err == nil {
		snapshot.Available = true
		snapshot.Source = "client"
		snapshot.Jails = jails
		snapshot.Error = ""
		return snapshot
	} else if snapshot.Error == "" {
		snapshot.Error = err.Error()
	}

	if jails, err := collectFail2banFromLog(); err == nil {
		snapshot.Available = true
		snapshot.Source = "log"
		snapshot.Jails = jails
		if len(jails) > 0 {
			snapshot.Error = ""
		}
		return snapshot
	} else if snapshot.Error == "" {
		snapshot.Error = err.Error()
	}

	if snapshot.Error == "" {
		snapshot.Error = "fail2ban status unavailable"
	}
	return snapshot
}

func collectFail2banFromClient() ([]models.Fail2banJail, error) {
	out, err := runFail2banClient("status")
	if err != nil {
		return nil, err
	}

	names := parseFail2banJailList(string(out))
	jails := make([]models.Fail2banJail, 0, len(names))
	for _, name := range names {
		jailOut, jailErr := runFail2banClient("status", name)
		if jailErr != nil {
			return nil, jailErr
		}
		jails = append(jails, parseFail2banJailStatus(name, string(jailOut)))
	}
	return jails, nil
}

func collectFail2banFromLog() ([]models.Fail2banJail, error) {
	data, err := readFail2banLog(fail2banLogTail)
	if err != nil {
		return nil, err
	}
	jails := parseFail2banLogCurrentlyBanned(string(data))
	return jails, nil
}

func runFail2banClient(args ...string) ([]byte, error) {
	cmdArgs := append([]string{}, args...)
	if out, err := exec.Command("fail2ban-client", cmdArgs...).Output(); err == nil {
		return out, nil
	}

	sudoArgs := append([]string{"fail2ban-client"}, cmdArgs...)
	out, err := exec.Command("sudo", sudoArgs...).Output()
	if err != nil {
		return nil, fmt.Errorf("fail2ban-client: %s", trimExecError(err))
	}
	return out, nil
}

func readFail2banLog(limit int) ([]byte, error) {
	if lines, err := tailFile(fail2banLogPath, limit); err == nil {
		return []byte(strings.Join(lines, "\n")), nil
	}
	out, err := exec.Command("sudo", "tail", "-n", strconv.Itoa(limit), fail2banLogPath).Output()
	if err != nil {
		return nil, fmt.Errorf("read fail2ban log: %s", trimExecError(err))
	}
	return out, nil
}

func parseFail2banJailList(output string) []string {
	for _, line := range strings.Split(output, "\n") {
		matches := fail2banJailListRE.FindStringSubmatch(strings.TrimSpace(line))
		if len(matches) != 2 {
			continue
		}
		raw := strings.TrimSpace(matches[1])
		if raw == "" {
			return nil
		}
		parts := strings.Split(raw, ",")
		jails := make([]string, 0, len(parts))
		for _, part := range parts {
			name := strings.TrimSpace(part)
			if name != "" {
				jails = append(jails, name)
			}
		}
		return jails
	}
	return nil
}

func parseFail2banJailStatus(name, output string) models.Fail2banJail {
	jail := models.Fail2banJail{
		Name:      name,
		BannedIPs: []string{},
	}

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if matches := fail2banIntFieldRE.FindStringSubmatch(line); len(matches) == 3 {
			value, _ := strconv.Atoi(matches[2])
			switch strings.ToLower(matches[1]) {
			case "currently failed":
				jail.CurrentlyFailed = value
			case "total failed":
				jail.TotalFailed = value
			case "currently banned":
				jail.CurrentlyBanned = value
			case "total banned":
				jail.TotalBanned = value
			}
			continue
		}

		if matches := fail2banBannedRE.FindStringSubmatch(line); len(matches) == 2 {
			raw := strings.TrimSpace(matches[1])
			if raw == "" {
				continue
			}
			for _, ip := range strings.Fields(raw) {
				jail.BannedIPs = append(jail.BannedIPs, ip)
			}
		}
	}

	if jail.CurrentlyBanned == 0 && len(jail.BannedIPs) > 0 {
		jail.CurrentlyBanned = len(jail.BannedIPs)
	}
	return jail
}

func parseFail2banLogCurrentlyBanned(content string) []models.Fail2banJail {
	banned := map[string]map[string]struct{}{}

	for _, line := range strings.Split(content, "\n") {
		matches := fail2banLogBanRE.FindStringSubmatch(line)
		if len(matches) != 4 {
			continue
		}
		jail := matches[1]
		action := strings.ToLower(matches[2])
		ip := matches[3]
		if banned[jail] == nil {
			banned[jail] = map[string]struct{}{}
		}
		switch action {
		case "ban":
			banned[jail][ip] = struct{}{}
		case "unban":
			delete(banned[jail], ip)
		}
	}

	names := make([]string, 0, len(banned))
	for name, ips := range banned {
		if len(ips) == 0 {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)

	jails := make([]models.Fail2banJail, 0, len(names))
	for _, name := range names {
		ips := make([]string, 0, len(banned[name]))
		for ip := range banned[name] {
			ips = append(ips, ip)
		}
		sort.Strings(ips)
		jails = append(jails, models.Fail2banJail{
			Name:            name,
			CurrentlyBanned: len(ips),
			BannedIPs:       ips,
		})
	}
	return jails
}
