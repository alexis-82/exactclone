package mount

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows"

	"diskclone/internal/disk"
)

// ReadOnly returns a path from which the partition files can be read.
// Windows mounts every recognized volume, so the drive letter (or the
// \\?\Volume{GUID}\ path for volumes without one) is used directly; only if
// that path is not readable is the volume mounted on a temporary folder.
// The files are only read, never modified.
func ReadOnly(p disk.Partition) (path string, cleanup func() error, err error) {
	noop := func() error { return nil }
	if len(p.MountPoints) > 0 {
		return p.MountPoints[0], noop, nil
	}
	if !strings.HasPrefix(p.Path, `\\?\Volume{`) {
		return "", nil, fmt.Errorf("partition %s has no volume", p.ID)
	}
	if _, err := os.ReadDir(p.Path); err == nil {
		return p.Path, noop, nil
	}
	dir, err := os.MkdirTemp("", "diskclone-")
	if err != nil {
		return "", nil, err
	}
	mp, _ := windows.UTF16PtrFromString(dir + `\`)
	vol, _ := windows.UTF16PtrFromString(p.Path)
	if err := windows.SetVolumeMountPoint(mp, vol); err != nil {
		os.Remove(dir)
		return "", nil, fmt.Errorf("mount %s on %s: %w", p.Path, dir, err)
	}
	cleanup = func() error {
		if err := windows.DeleteVolumeMountPoint(mp); err != nil {
			return fmt.Errorf("unmount %s: %w", dir, err)
		}
		return os.Remove(dir)
	}
	return dir + `\`, cleanup, nil
}

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
