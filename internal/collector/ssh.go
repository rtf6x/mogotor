package collector

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rtf6x/mogotor/internal/models"
)

var (
	authAcceptedRE = regexp.MustCompile(`Accepted (\w+) for (\S+) from ([0-9a-fA-F:.]+)`)
	authFailedRE   = regexp.MustCompile(`Failed password for (?:invalid user )?(\S+) from ([0-9a-fA-F:.]+)`)
	authInvalidRE  = regexp.MustCompile(`Invalid user (\S+) from ([0-9a-fA-F:.]+)`)
	lastLoginRE    = regexp.MustCompile(`^(\S+)\s+(\S+)\s+(\S+)\s+(\w{3}\s+\w{3}\s+\d+\s+[\d:]+)`)
)

const (
	sshLoginLimit    = 20
	sshFailureLimit  = 30
	authLogTailLines = 2000
)

func CollectSSH() models.SSHSnapshot {
	logins := collectLastLogins(sshLoginLimit)
	failures := collectSSHFailures(sshFailureLimit)

	if len(logins) == 0 && len(failures) == 0 {
		return models.SSHSnapshot{
			Available: false,
			Error:     "no SSH auth data available",
		}
	}

	return models.SSHSnapshot{
		Available: true,
		Logins:    logins,
		Failures:  failures,
	}
}

func collectLastLogins(limit int) []models.SSHAuthEvent {
	out, err := exec.Command("last", "-n", "30", "-i").Output()
	if err != nil {
		return collectAcceptedFromAuthLog(limit)
	}

	events := make([]models.SSHAuthEvent, 0, limit)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "wtmp") || strings.HasPrefix(line, "reboot") {
			continue
		}

		matches := lastLoginRE.FindStringSubmatch(line)
		if len(matches) != 5 {
			continue
		}

		user := matches[1]
		ip := matches[3]
		if user == "reboot" || ip == "0.0.0.0" || strings.HasPrefix(ip, "::") {
			continue
		}

		when, err := parseLastTimestamp(matches[4])
		if err != nil {
			continue
		}

		events = append(events, models.SSHAuthEvent{
			Timestamp: withCurrentYear(when),
			User:      user,
			IP:        ip,
			Kind:      "accepted",
		})
		if len(events) >= limit {
			break
		}
	}

	if len(events) > 0 {
		return events
	}
	return collectAcceptedFromAuthLog(limit)
}

func collectAcceptedFromAuthLog(limit int) []models.SSHAuthEvent {
	lines, err := readAuthLogLines(authLogTailLines)
	if err != nil {
		return nil
	}

	now := time.Now()
	events := make([]models.SSHAuthEvent, 0, limit)
	for _, line := range lines {
		event, ok := parseAuthLogSSHAccepted(line, now)
		if !ok {
			continue
		}
		events = append(events, event)
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.After(events[j].Timestamp)
	})
	if len(events) > limit {
		events = events[:limit]
	}
	return events
}

func collectSSHFailures(limit int) []models.SSHAuthEvent {
	lines, err := readAuthLogLines(authLogTailLines)
	if err != nil {
		return nil
	}

	now := time.Now()
	events := make([]models.SSHAuthEvent, 0, limit)
	for _, line := range lines {
		event, ok := parseAuthLogSSHFailure(line, now)
		if !ok {
			continue
		}
		events = append(events, event)
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.After(events[j].Timestamp)
	})
	if len(events) > limit {
		events = events[:limit]
	}
	return events
}

func readAuthLogLines(limit int) ([]string, error) {
	if lines, err := tailFile("/var/log/auth.log", limit); err == nil {
		return lines, nil
	}
	if out, err := exec.Command("sudo", "tail", "-n", strconv.Itoa(limit), "/var/log/auth.log").Output(); err == nil {
		return splitLines(out), nil
	}
	if out, err := exec.Command("journalctl", "-t", "sshd", "-n", strconv.Itoa(limit), "--no-pager").Output(); err == nil {
		return splitLines(out), nil
	}
	return nil, os.ErrPermission
}

func tailFile(path string, limit int) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	lines := make([]string, 0, limit)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > limit {
			lines = lines[1:]
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

func parseAuthLogSSHFailure(line string, now time.Time) (models.SSHAuthEvent, bool) {
	when, message, ok := parseAuthLogLine(line)
	if !ok || !strings.HasPrefix(message, "sshd") {
		return models.SSHAuthEvent{}, false
	}

	if matches := authFailedRE.FindStringSubmatch(message); len(matches) == 3 {
		return models.SSHAuthEvent{
			Timestamp: withCurrentYear(when),
			User:      matches[1],
			IP:        matches[2],
			Kind:      "failed_password",
		}, true
	}

	if matches := authInvalidRE.FindStringSubmatch(message); len(matches) == 3 {
		return models.SSHAuthEvent{
			Timestamp: withCurrentYear(when),
			User:      matches[1],
			IP:        matches[2],
			Kind:      "invalid_user",
		}, true
	}

	return models.SSHAuthEvent{}, false
}

func parseAuthLogSSHAccepted(line string, now time.Time) (models.SSHAuthEvent, bool) {
	when, message, ok := parseAuthLogLine(line)
	if !ok || !strings.HasPrefix(message, "sshd") {
		return models.SSHAuthEvent{}, false
	}

	if matches := authAcceptedRE.FindStringSubmatch(message); len(matches) == 4 {
		return models.SSHAuthEvent{
			Timestamp: withCurrentYear(when),
			User:      matches[2],
			IP:        matches[3],
			Method:    matches[1],
			Kind:      "accepted",
		}, true
	}

	return models.SSHAuthEvent{}, false
}

func parseAuthLogLine(line string) (time.Time, string, bool) {
	idx := strings.Index(line, " sshd")
	if idx < 0 {
		return time.Time{}, "", false
	}

	fields := strings.Fields(line[:idx])
	if len(fields) < 3 {
		return time.Time{}, "", false
	}

	when, err := time.ParseInLocation("Jan _2 15:04:05", strings.Join(fields[:3], " "), time.Local)
	if err != nil {
		return time.Time{}, "", false
	}

	return when, strings.TrimSpace(line[idx+1:]), true
}

func parseLastTimestamp(value string) (time.Time, error) {
	for _, layout := range []string{"Mon Jan _2 15:04:05", "Mon Jan _2 15:04"} {
		if when, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return when, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid last timestamp: %s", value)
}

func withCurrentYear(value time.Time) time.Time {
	now := time.Now()
	return time.Date(now.Year(), value.Month(), value.Day(), value.Hour(), value.Minute(), value.Second(), 0, value.Location())
}

func splitLines(data []byte) []string {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil
	}
	return strings.Split(string(data), "\n")
}
