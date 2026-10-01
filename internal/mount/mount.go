package mount

// Info describes the file system that contains a path.
type Info struct {
	UsedBytes uint64 `json:"usedBytes"`
	FreeBytes uint64 `json:"freeBytes"` // available to the current user
	FSType    string `json:"fsType"`    // normalized: vfat, exfat, ntfs, ext4, ...
}

// MaxFileSize returns the largest file the file system can hold, or 0 when
// there is no practical limit.
func MaxFileSize(fsType string) uint64 {
	if fsType == "vfat" {
		return 1<<32 - 1 // FAT32: 4 GiB - 1
	}
	return 0
}
