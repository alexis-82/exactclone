package mount

import (
	"golang.org/x/sys/windows"

	"exactclone/internal/disk"
)

// Stat returns used/free space and the file system type of the given path.
func Stat(path string) (Info, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return Info{}, err
	}
	var avail, total, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(p, &avail, &total, &totalFree); err != nil {
		return Info{}, err
	}
	info := Info{UsedBytes: total - totalFree, FreeBytes: avail}
	root := make([]uint16, windows.MAX_PATH+1)
	if err := windows.GetVolumePathName(p, &root[0], uint32(len(root))); err == nil {
		fsn := make([]uint16, windows.MAX_PATH+1)
		if err := windows.GetVolumeInformation(&root[0], nil, 0, nil, nil, nil, &fsn[0], uint32(len(fsn))); err == nil {
			info.FSType = disk.NormalizeFS(windows.UTF16ToString(fsn))
		}
	}
	return info, nil
}
