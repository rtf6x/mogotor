package collector

import (
	"sort"
	"strings"

	"github.com/rtf6x/mogotor/internal/models"
	"github.com/shirou/gopsutil/v4/disk"
)

var skipDiskFstypes = map[string]struct{}{
	"autofs":      {},
	"binfmt_misc": {},
	"bpf":         {},
	"cgroup":      {},
	"cgroup2":     {},
	"configfs":    {},
	"debugfs":     {},
	"devpts":      {},
	"devtmpfs":    {},
	"fusectl":     {},
	"hugetlbfs":   {},
	"mqueue":      {},
	"overlay":     {},
	"proc":        {},
	"pstore":      {},
	"securityfs":  {},
	"squashfs":    {},
	"sysfs":       {},
	"tmpfs":       {},
	"tracefs":     {},
}

func CollectDisks() []models.DiskUsage {
	partitions, err := disk.Partitions(false)
	if err != nil {
		return nil
	}

	disks := make([]models.DiskUsage, 0, len(partitions))
	seen := make(map[string]struct{}, len(partitions))

	for _, partition := range partitions {
		if !includeDiskPartition(partition.Mountpoint, partition.Fstype) {
			continue
		}
		if _, ok := seen[partition.Mountpoint]; ok {
			continue
		}

		usage, err := disk.Usage(partition.Mountpoint)
		if err != nil || usage.Total == 0 {
			continue
		}

		seen[partition.Mountpoint] = struct{}{}
		disks = append(disks, models.DiskUsage{
			Device:      partition.Device,
			Mountpoint:  partition.Mountpoint,
			Fstype:      partition.Fstype,
			UsedBytes:   usage.Used,
			TotalBytes:  usage.Total,
			UsedPercent: usage.UsedPercent,
		})
	}

	sort.Slice(disks, func(i, j int) bool {
		return disks[i].Mountpoint < disks[j].Mountpoint
	})

	return disks
}

func includeDiskPartition(mountpoint, fstype string) bool {
	if mountpoint == "" {
		return false
	}
	if _, skip := skipDiskFstypes[strings.ToLower(fstype)]; skip {
		return false
	}
	switch mountpoint {
	case "/dev", "/dev/shm", "/run", "/run/lock":
		return false
	}
	if strings.HasPrefix(mountpoint, "/proc/") ||
		strings.HasPrefix(mountpoint, "/sys/") ||
		strings.HasPrefix(mountpoint, "/run/user/") {
		return false
	}
	return true
}
