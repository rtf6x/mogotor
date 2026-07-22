package collector

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/rtf6x/mogotor/internal/models"
)

const (
	jenkinsDataDir    = "/var/lib/jenkins"
	jenkinsPluginsDir = "/var/lib/jenkins/plugins"
)

func CollectJenkins() models.JenkinsSnapshot {
	snapshot := models.JenkinsSnapshot{
		DataDir: jenkinsDataDir,
	}

	pid, err := jenkinsPID()
	if err != nil {
		snapshot.Error = err.Error()
		return snapshot
	}
	snapshot.Available = true
	snapshot.PID = pid
	snapshot.RSSBytes = processMemoryBytes(pid)
	snapshot.CgroupBytes = serviceMemoryBytes("jenkins")
	snapshot.HeapMaxMB = parseHeapMaxMB()

	if heapUsed, nativeUsed, err := jenkinsMemoryBreakdown(pid); err == nil {
		snapshot.HeapUsedMB = heapUsed
		snapshot.NativeUsedMB = nativeUsed
	}

	plugins, err := topJenkinsPlugins(8)
	if err == nil {
		snapshot.PluginCount = countJenkinsPlugins()
		snapshot.TopPlugins = plugins
	}

	snapshot.JobCount = countJenkinsJobs()
	snapshot.WorkspaceBytes = dirSizeBytes(filepath.Join(jenkinsDataDir, "workspace"))
	snapshot.PluginsBytes = dirSizeBytes(jenkinsPluginsDir)

	return snapshot
}

func jenkinsPID() (int, error) {
	out, err := exec.Command("systemctl", "show", "jenkins", "-p", "MainPID", "--value").Output()
	if err != nil {
		return 0, fmt.Errorf("%s", trimExecError(err))
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil || pid <= 0 {
		return 0, fmt.Errorf("jenkins is not running")
	}
	return pid, nil
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

func parseHeapMaxMB() int {
	data, err := os.ReadFile("/etc/default/jenkins")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if idx := strings.Index(line, "-Xmx"); idx >= 0 {
			return parseMemoryFlagMB(line[idx:])
		}
	}
	return 0
}

func parseMemoryFlagMB(flag string) int {
	flag = strings.Fields(flag)[0]
	if !strings.HasPrefix(flag, "-Xmx") {
		return 0
	}
	value := strings.TrimPrefix(flag, "-Xmx")
	if value == "" {
		return 0
	}
	if strings.HasSuffix(value, "m") || strings.HasSuffix(value, "M") {
		n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimSuffix(value, "m"), "M"))
		if err == nil {
			return n
		}
	}
	if strings.HasSuffix(value, "g") || strings.HasSuffix(value, "G") {
		n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimSuffix(value, "g"), "G"))
		if err == nil {
			return n * 1024
		}
	}
	n, err := strconv.Atoi(value)
	if err == nil {
		return n / (1024 * 1024)
	}
	return 0
}

func jenkinsMemoryBreakdown(pid int) (heapUsedMB int, nativeUsedMB int, err error) {
	cmd := exec.Command("jcmd", strconv.Itoa(pid), "GC.heap_info")
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, err
	}

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "used "):
			if kb := parseHeapKB(line); kb > 0 {
				heapUsedMB = int((kb + 1023) / 1024)
			}
		}
	}

	nativeCmd := exec.Command("jcmd", strconv.Itoa(pid), "VM.native_memory", "summary", "scale=MB")
	nativeOut, err := nativeCmd.Output()
	if err != nil {
		return heapUsedMB, 0, nil
	}

	total := 0
	for _, line := range strings.Split(string(nativeOut), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Total:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if mb, parseErr := strconv.Atoi(strings.TrimSuffix(fields[1], "MB")); parseErr == nil {
					total = mb
				}
			}
		}
	}
	return heapUsedMB, total, nil
}

func parseHeapKB(line string) uint64 {
	// used 187456K, committed 262144K
	fields := strings.Split(line, ",")
	if len(fields) == 0 {
		return 0
	}
	used := strings.Fields(strings.TrimSpace(fields[0]))
	if len(used) < 2 {
		return 0
	}
	value := strings.TrimSuffix(used[1], "K")
	kb, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0
	}
	return kb
}

func countJenkinsPlugins() int {
	matches, err := filepath.Glob(filepath.Join(jenkinsPluginsDir, "*.jpi"))
	if err != nil {
		return 0
	}
	return len(matches)
}

func topJenkinsPlugins(limit int) ([]models.JenkinsPlugin, error) {
	matches, err := filepath.Glob(filepath.Join(jenkinsPluginsDir, "*.jpi"))
	if err != nil {
		return nil, err
	}

	plugins := make([]models.JenkinsPlugin, 0, len(matches))
	for _, path := range matches {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		name := strings.TrimSuffix(filepath.Base(path), ".jpi")
		plugins = append(plugins, models.JenkinsPlugin{
			Name:        name,
			SizeBytes:   uint64(info.Size()),
		})
	}

	sort.Slice(plugins, func(i, j int) bool {
		return plugins[i].SizeBytes > plugins[j].SizeBytes
	})
	if len(plugins) > limit {
		plugins = plugins[:limit]
	}
	return plugins, nil
}

func countJenkinsJobs() int {
	entries, err := os.ReadDir(filepath.Join(jenkinsDataDir, "jobs"))
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			count++
		}
	}
	return count
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

func parseNativeMemoryTotalMB(output string) int {
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "Total:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if mb, err := strconv.Atoi(strings.TrimSuffix(fields[1], "MB")); err == nil {
					return mb
				}
			}
		}
	}
	return 0
}
