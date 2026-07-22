package collector

import "testing"

func TestIncludeDiskPartition(t *testing.T) {
	tests := []struct {
		mountpoint string
		fstype     string
		want       bool
	}{
		{"/", "ext4", true},
		{"/var", "ext4", true},
		{"/boot", "ext4", true},
		{"/mnt/data", "xfs", true},
		{"/run", "tmpfs", false},
		{"/dev/shm", "tmpfs", false},
		{"/run/user/1000", "tmpfs", false},
		{"/sys/fs/cgroup", "cgroup2", false},
		{"/snap/lxd/current", "squashfs", false},
	}

	for _, tc := range tests {
		if got := includeDiskPartition(tc.mountpoint, tc.fstype); got != tc.want {
			t.Fatalf("%s (%s): expected %v, got %v", tc.mountpoint, tc.fstype, tc.want, got)
		}
	}
}
