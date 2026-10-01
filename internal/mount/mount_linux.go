package mount

import (
	"golang.org/x/sys/unix"
)

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
