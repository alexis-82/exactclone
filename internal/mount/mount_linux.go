package mount

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/sys/unix"

	"diskclone/internal/disk"
)

// ReadOnly makes a partition readable and returns its root path. A partition
// already mounted (e.g. by the desktop automounter) is reused and left mounted
// by cleanup; otherwise it is mounted read-only in a temporary folder that
// cleanup unmounts and removes.
func ReadOnly(p disk.Partition) (path string, cleanup func() error, err error) {
	for _, m := range p.MountPoints {
		if m != "[SWAP]" {
			return m, func() error { return nil }, nil
		}
	}
	dir, err := os.MkdirTemp("", "diskclone-")
	if err != nil {
		return "", nil, err
	}
	if out, err := exec.Command("mount", "-o", "ro", p.Path, dir).CombinedOutput(); err != nil {
		os.Remove(dir)
		return "", nil, fmt.Errorf("mount %s: %v: %s", p.Path, err, strings.TrimSpace(string(out)))
	}
	cleanup = func() error {
		if err := unix.Unmount(dir, 0); err != nil {
			return fmt.Errorf("unmount %s: %w", dir, err)
		}
		return os.Remove(dir)
	}
	return dir, cleanup, nil
}

// fsMagic maps statfs magic numbers to lsblk-style names.
var fsMagic = map[int64]string{
	0x4d44:     "vfat",
	0x2011BAB0: "exfat",
	0x5346544e: "ntfs",
	0x65735546: "fuseblk", // ntfs-3g
	0xEF53:     "ext4",
	0x58465342: "xfs",
	0x9123683E: "btrfs",
}

// Stat returns used/free space and the file system type of the given path.
func Stat(path string) (Info, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return Info{}, err
	}
	bs := uint64(st.Bsize)
	return Info{
		UsedBytes: (st.Blocks - st.Bfree) * bs,
		FreeBytes: st.Bavail * bs,
		FSType:    fsMagic[int64(st.Type)],
	}, nil
}
