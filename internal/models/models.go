package models

import "time"

type SystemSnapshot struct {
	Timestamp        time.Time `json:"timestamp"`
	CPUPercent       float64   `json:"cpuPercent"`
	MemoryUsedBytes  uint64    `json:"memoryUsedBytes"`
	MemoryTotalBytes uint64    `json:"memoryTotalBytes"`
	SwapUsedBytes    uint64    `json:"swapUsedBytes"`
	SwapTotalBytes   uint64    `json:"swapTotalBytes"`
	DiskUsedBytes    uint64    `json:"diskUsedBytes"`
	DiskTotalBytes   uint64    `json:"diskTotalBytes"`
	DiskUsedPercent  float64   `json:"diskUsedPercent"`
	NetBytesSent     uint64    `json:"netBytesSent"`
	NetBytesRecv     uint64    `json:"netBytesRecv"`
	NetSendBps       float64   `json:"netSendBps"`
	NetRecvBps       float64   `json:"netRecvBps"`
	Load1            float64   `json:"load1"`
	Load5            float64   `json:"load5"`
	Load15           float64   `json:"load15"`
	UptimeSeconds    uint64    `json:"uptimeSeconds"`
}

type DiskUsage struct {
	Device      string  `json:"device,omitempty"`
	Mountpoint  string  `json:"mountpoint"`
	Fstype      string  `json:"fstype,omitempty"`
	UsedBytes   uint64  `json:"usedBytes"`
	TotalBytes  uint64  `json:"totalBytes"`
	UsedPercent float64 `json:"usedPercent"`
}

type PM2Process struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Status      string  `json:"status"`
	CPU         float64 `json:"cpu"`
	MemoryBytes uint64  `json:"memoryBytes"`
	Restarts    int     `json:"restarts"`
	UptimeMs    int64   `json:"uptimeMs"`
	ExecMode    string  `json:"execMode"`
	Script      string  `json:"script"`
}

type PM2Snapshot struct {
	Available bool         `json:"available"`
	Error     string       `json:"error,omitempty"`
	Processes []PM2Process `json:"processes"`
}

type DockerContainer struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	CPUPercent  float64 `json:"cpuPercent"`
	MemoryBytes uint64  `json:"memoryBytes"`
	MemoryLimit uint64  `json:"memoryLimit"`
	NetInput    uint64  `json:"netInput"`
	NetOutput   uint64  `json:"netOutput"`
	BlockInput  uint64  `json:"blockInput"`
	BlockOutput uint64  `json:"blockOutput"`
	PIDs        int     `json:"pids"`
}

type DockerSnapshot struct {
	Available  bool              `json:"available"`
	Error      string            `json:"error,omitempty"`
	Containers []DockerContainer `json:"containers"`
}

type ServiceStatus struct {
	Name        string `json:"name"`
	Active      string `json:"active"`
	SubState    string `json:"subState"`
	Description string `json:"description"`
	MainPID     int    `json:"mainPid"`
	MemoryBytes uint64 `json:"memoryBytes"`
	Error       string `json:"error,omitempty"`
}

type DploSnapshot struct {
	Available       bool   `json:"available"`
	Error           string `json:"error,omitempty"`
	PID             int    `json:"pid,omitempty"`
	RSSBytes        uint64 `json:"rssBytes,omitempty"`
	CgroupBytes     uint64 `json:"cgroupBytes,omitempty"`
	ProjectCount    int    `json:"projectCount,omitempty"`
	EnabledCount    int    `json:"enabledCount,omitempty"`
	RunCount        int    `json:"runCount,omitempty"`
	RunningCount    int    `json:"runningCount,omitempty"`
	DataBytes       uint64 `json:"dataBytes,omitempty"`
	DataDir         string `json:"dataDir,omitempty"`
	APIHealthy      bool   `json:"apiHealthy,omitempty"`
	RunnerBusy      bool   `json:"runnerBusy,omitempty"`
	ActiveProjectID string `json:"activeProjectId,omitempty"`
	ActiveRunID     string `json:"activeRunId,omitempty"`
}

type MongoSnapshot struct {
	Available            bool   `json:"available"`
	Source               string `json:"source,omitempty"`
	Error                string `json:"error,omitempty"`
	Version              string `json:"version"`
	UptimeSeconds        int64  `json:"uptimeSeconds"`
	Connections          int    `json:"connections"`
	ConnectionsAvailable int    `json:"connectionsAvailable"`
	MemoryResidentMb     int    `json:"memoryResidentMb"`
	MemoryVirtualMb      int    `json:"memoryVirtualMb"`
	ProcessMemoryBytes   uint64 `json:"processMemoryBytes,omitempty"`
	CacheBytes           uint64 `json:"cacheBytes"`
	CacheMaxBytes        uint64 `json:"cacheMaxBytes"`
	OpsQuery             int64  `json:"opsQuery"`
	OpsInsert            int64  `json:"opsInsert"`
	OpsUpdate            int64  `json:"opsUpdate"`
	OpsDelete            int64  `json:"opsDelete"`
}

type SSHAuthEvent struct {
	Timestamp time.Time `json:"timestamp"`
	User      string    `json:"user"`
	IP        string    `json:"ip"`
	Method    string    `json:"method,omitempty"`
	Kind      string    `json:"kind"`
}

type SSHSnapshot struct {
	Available bool           `json:"available"`
	Error     string         `json:"error,omitempty"`
	Logins    []SSHAuthEvent `json:"logins"`
	Failures  []SSHAuthEvent `json:"failures"`
}

type Fail2banJail struct {
	Name            string   `json:"name"`
	CurrentlyFailed int      `json:"currentlyFailed"`
	TotalFailed     int      `json:"totalFailed"`
	CurrentlyBanned int      `json:"currentlyBanned"`
	TotalBanned     int      `json:"totalBanned"`
	BannedIPs       []string `json:"bannedIps"`
}

type Fail2banSnapshot struct {
	Available bool           `json:"available"`
	Error     string         `json:"error,omitempty"`
	Source    string         `json:"source,omitempty"`
	Active    string         `json:"active,omitempty"`
	SubState  string         `json:"subState,omitempty"`
	Jails     []Fail2banJail `json:"jails"`
}

type OpenVPNSnapshot struct {
	Available   bool     `json:"available"`
	Error       string   `json:"error,omitempty"`
	ServiceName string   `json:"serviceName,omitempty"`
	Active      string   `json:"active,omitempty"`
	SubState    string   `json:"subState,omitempty"`
	UpdatedAt   string   `json:"updatedAt,omitempty"`
	Clients     []string `json:"clients"`
}

type Snapshot struct {
	Timestamp time.Time        `json:"timestamp"`
	System    SystemSnapshot   `json:"system"`
	Disks     []DiskUsage      `json:"disks"`
	PM2       PM2Snapshot      `json:"pm2"`
	Docker    DockerSnapshot   `json:"docker"`
	Services  []ServiceStatus  `json:"services"`
	Dplo      DploSnapshot     `json:"dplo"`
	Mongo     MongoSnapshot    `json:"mongo"`
	OpenVPN   OpenVPNSnapshot  `json:"openvpn"`
	SSH       SSHSnapshot      `json:"ssh"`
	Fail2ban  Fail2banSnapshot `json:"fail2ban"`
}

type HistoryResponse struct {
	Retention time.Time        `json:"retentionFrom"`
	Points    []SystemSnapshot `json:"points"`
}
