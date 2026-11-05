//go:build !windows

package operations

import "golang.org/x/sys/unix"

func Disk(path string) (DiskInfo, error) {
	var v unix.Statfs_t
	e := unix.Statfs(path, &v)
	return DiskInfo{Total: v.Blocks * uint64(v.Bsize), Free: v.Bavail * uint64(v.Bsize), Scope: "Shipper data filesystem"}, e
}
