package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/rtf6x/mogotor/internal/models"
)

const mongoStatusEval = `var s=db.serverStatus(); print(JSON.stringify({
	version: s.version,
	uptime: s.uptime,
	connections: s.connections.current,
	connectionsAvailable: s.connections.available,
	memResident: s.mem.resident,
	memVirtual: s.mem.virtual,
	cacheBytes: s.wiredTiger.cache["bytes currently in the cache"],
	cacheMaxBytes: s.wiredTiger.cache["maximum bytes configured"],
	opsQuery: s.opcounters.query * 1,
	opsInsert: s.opcounters.insert * 1,
	opsUpdate: s.opcounters.update * 1,
	opsDelete: s.opcounters.delete * 1
}))`

func CollectMongo(uri, containerName string) models.MongoSnapshot {
	if snapshot, err := collectMongoServerStatus(uri, containerName); err == nil {
		snapshot.Source = "serverStatus"
		snapshot.Available = true
		return snapshot
	} else if uri != "" {
		return collectMongoProcessFallback(err.Error())
	}

	for _, fallbackURI := range []string{
		"mongodb://127.0.0.1:28888/admin",
		"mongodb://127.0.0.1:27017/admin",
	} {
		if snapshot, err := collectMongoServerStatus(fallbackURI, containerName); err == nil {
			snapshot.Source = "serverStatus"
			snapshot.Available = true
			return snapshot
		}
	}

	return collectMongoProcessFallback("mongo serverStatus unavailable")
}

func collectMongoServerStatus(uri, containerName string) (models.MongoSnapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmds := mongoCommands(ctx, uri, mongoStatusEval, containerName)
	if len(cmds) == 0 {
		return models.MongoSnapshot{}, fmt.Errorf("mongo shell not found")
	}

	var lastErr error
	for _, cmd := range cmds {
		out, err := cmd.Output()
		if err != nil {
			lastErr = fmt.Errorf("%s", trimExecError(err))
			continue
		}

		var raw mongoStatusRaw
		if err := json.Unmarshal(bytesTrim(out), &raw); err != nil {
			lastErr = fmt.Errorf("invalid mongo output: %w", err)
			continue
		}

		return raw.toModel(), nil
	}

	return models.MongoSnapshot{}, lastErr
}

func collectMongoProcessFallback(reason string) models.MongoSnapshot {
	status := collectService("mongod")
	if status.Active != "active" {
		return models.MongoSnapshot{
			Available: false,
			Error:     reason,
		}
	}

	snapshot := models.MongoSnapshot{
		Available:          true,
		Source:             "process",
		MemoryResidentMb:   int(status.MemoryBytes / 1024 / 1024),
		ProcessMemoryBytes: status.MemoryBytes,
	}
	if snapshot.MemoryResidentMb == 0 && status.MemoryBytes > 0 {
		snapshot.MemoryResidentMb = 1
	}
	if reason != "mongo serverStatus unavailable" {
		snapshot.Error = reason
	}
	return snapshot
}

func mongoCommands(ctx context.Context, uri, eval, containerName string) []*exec.Cmd {
	var cmds []*exec.Cmd
	if path, err := exec.LookPath("mongosh"); err == nil {
		cmds = append(cmds, exec.CommandContext(ctx, path, uri, "--quiet", "--eval", eval))
	}
	if path, err := exec.LookPath("mongo"); err == nil {
		cmds = append(cmds, exec.CommandContext(ctx, path, uri, "--quiet", "--eval", eval))
	}

	if containerName == "" {
		return cmds
	}
	dockerPath, err := exec.LookPath("docker")
	if err != nil {
		return cmds
	}
	containerURI, ok := containerMongoURI(uri)
	if !ok {
		return cmds
	}
	cmds = append(cmds,
		exec.CommandContext(ctx, dockerPath, "exec", containerName, "mongosh", containerURI, "--quiet", "--eval", eval),
		exec.CommandContext(ctx, dockerPath, "exec", containerName, "mongo", containerURI, "--quiet", "--eval", eval),
	)
	return cmds
}

// containerMongoURI rewrites a loopback mongo URI to the container-internal
// mongod port (27017), since the configured URI carries the host-mapped
// docker port (e.g. 28888) which is only reachable from outside the container.
func containerMongoURI(uri string) (string, bool) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", false
	}
	host := u.Hostname()
	if host != "127.0.0.1" && host != "localhost" {
		return "", false
	}
	u.Host = net.JoinHostPort(host, "27017")
	return u.String(), true
}

type mongoStatusRaw struct {
	Version              string `json:"version"`
	Uptime               int64  `json:"uptime"`
	Connections          int    `json:"connections"`
	ConnectionsAvailable int    `json:"connectionsAvailable"`
	MemResident          int    `json:"memResident"`
	MemVirtual           int    `json:"memVirtual"`
	CacheBytes           uint64 `json:"cacheBytes"`
	CacheMaxBytes        uint64 `json:"cacheMaxBytes"`
	OpsQuery             int64  `json:"opsQuery"`
	OpsInsert            int64  `json:"opsInsert"`
	OpsUpdate            int64  `json:"opsUpdate"`
	OpsDelete            int64  `json:"opsDelete"`
}

func (m mongoStatusRaw) toModel() models.MongoSnapshot {
	return models.MongoSnapshot{
		Version:              m.Version,
		UptimeSeconds:        m.Uptime,
		Connections:          m.Connections,
		ConnectionsAvailable: m.ConnectionsAvailable,
		MemoryResidentMb:     m.MemResident,
		MemoryVirtualMb:      m.MemVirtual,
		CacheBytes:           m.CacheBytes,
		CacheMaxBytes:        m.CacheMaxBytes,
		OpsQuery:             m.OpsQuery,
		OpsInsert:            m.OpsInsert,
		OpsUpdate:            m.OpsUpdate,
		OpsDelete:            m.OpsDelete,
	}
}

func processMemoryBytes(pid int) uint64 {
	if pid <= 0 {
		return 0
	}

	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0
	}

	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "VmRSS:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0
		}
		kb, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return 0
		}
		return kb * 1024
	}

	return 0
}

func bytesTrim(data []byte) []byte {
	return []byte(strings.TrimSpace(string(data)))
}
