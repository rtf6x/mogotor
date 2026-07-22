package collector

import (
	"encoding/json"
	"os"
	"os/exec"
	"os/user"
	"strings"

	"github.com/rtf6x/mogotor/internal/models"
)

func CollectPM2(pm2User string) models.PM2Snapshot {
	cmd, err := pm2Command(pm2User, "jlist")
	if err != nil {
		return models.PM2Snapshot{Available: false, Error: err.Error()}
	}

	out, err := cmd.Output()
	if err != nil {
		return models.PM2Snapshot{Available: false, Error: trimExecError(err)}
	}

	var raw []pm2ProcessRaw
	if err := json.Unmarshal(out, &raw); err != nil {
		return models.PM2Snapshot{Available: false, Error: "invalid pm2 json: " + err.Error()}
	}

	processes := make([]models.PM2Process, 0, len(raw))
	for _, item := range raw {
		processes = append(processes, item.toModel())
	}

	return models.PM2Snapshot{Available: true, Processes: processes}
}

type pm2ProcessRaw struct {
	PMID     int    `json:"pm_id"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	ExecMode string `json:"exec_mode"`
	Monit    struct {
		CPU float64 `json:"cpu"`
		Mem int64   `json:"memory"`
	} `json:"monit"`
	PM2Env struct {
		Status   string `json:"status"`
		Restart  int    `json:"restart_time"`
		PMUptime int64  `json:"pm_uptime"`
		ExecMode string `json:"exec_mode"`
		Script   string `json:"pm_exec_path"`
	} `json:"pm2_env"`
}

func (p pm2ProcessRaw) toModel() models.PM2Process {
	status := p.Status
	if status == "" {
		status = p.PM2Env.Status
	}
	execMode := p.ExecMode
	if execMode == "" {
		execMode = p.PM2Env.ExecMode
	}

	return models.PM2Process{
		ID:          p.PMID,
		Name:        p.Name,
		Status:      status,
		CPU:         p.Monit.CPU,
		MemoryBytes: uint64(max64(p.Monit.Mem, 0)),
		Restarts:    p.PM2Env.Restart,
		UptimeMs:    p.PM2Env.PMUptime,
		ExecMode:    execMode,
		Script:      p.PM2Env.Script,
	}
}

func pm2Command(pm2User string, args ...string) (*exec.Cmd, error) {
	current, err := user.Current()
	if err != nil {
		return nil, err
	}

	if pm2User == "" || current.Username == pm2User {
		cmd := exec.Command("pm2", args...)
		cmd.Env = os.Environ()
		return cmd, nil
	}

	u, err := user.Lookup(pm2User)
	if err != nil {
		return nil, err
	}

	cmdArgs := append([]string{"-u", pm2User, "pm2"}, args...)
	cmd := exec.Command("sudo", cmdArgs...)
	cmd.Env = append(os.Environ(), "HOME="+u.HomeDir)
	return cmd, nil
}

func max64(v int64, min int64) int64 {
	if v < min {
		return min
	}
	return v
}

func trimExecError(err error) string {
	if exitErr, ok := err.(*exec.ExitError); ok {
		msg := strings.TrimSpace(string(exitErr.Stderr))
		if msg != "" {
			return msg
		}
	}
	return err.Error()
}
