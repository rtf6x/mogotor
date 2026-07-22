package collector

import (
	"time"

	"github.com/rtf6x/mogotor/internal/models"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
)

func CollectSystem(now time.Time) models.SystemSnapshot {
	snapshot := models.SystemSnapshot{Timestamp: now}

	if percents, err := cpu.Percent(0, false); err == nil && len(percents) > 0 {
		snapshot.CPUPercent = percents[0]
	}

	if vm, err := mem.VirtualMemory(); err == nil {
		snapshot.MemoryUsedBytes = vm.Used
		snapshot.MemoryTotalBytes = vm.Total
	}

	if swap, err := mem.SwapMemory(); err == nil {
		snapshot.SwapUsedBytes = swap.Used
		snapshot.SwapTotalBytes = swap.Total
	}

	if usage, err := disk.Usage("/"); err == nil {
		snapshot.DiskUsedBytes = usage.Used
		snapshot.DiskTotalBytes = usage.Total
		snapshot.DiskUsedPercent = usage.UsedPercent
	}

	if counters, err := net.IOCounters(false); err == nil && len(counters) > 0 {
		snapshot.NetBytesSent = counters[0].BytesSent
		snapshot.NetBytesRecv = counters[0].BytesRecv
	}

	if avg, err := load.Avg(); err == nil {
		snapshot.Load1 = avg.Load1
		snapshot.Load5 = avg.Load5
		snapshot.Load15 = avg.Load15
	}

	if info, err := host.Info(); err == nil {
		snapshot.UptimeSeconds = info.Uptime
	}

	return snapshot
}
