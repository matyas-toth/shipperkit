package operations

import "golang.org/x/sys/windows"

func Disk(path string) (DiskInfo, error) {
	v, e := windows.UTF16PtrFromString(path)
	if e != nil {
		return DiskInfo{}, e
	}
	var free, total, available uint64
	e = windows.GetDiskFreeSpaceEx(v, &available, &total, &free)
	return DiskInfo{Total: total, Free: available, Scope: "Shipper data filesystem"}, e
}
