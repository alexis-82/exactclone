// Package disk enumerates physical disks and their partitions and prepares
// a destination disk for raw writing. OS-specific code lives in *_linux.go
// and *_windows.go files.
package disk

// Disk is a physical block device (whole disk, not a partition).
type Disk struct {
	ID         string      `json:"id"`         // stable id used by the UI (e.g. "sdb", "PhysicalDrive2")
	Path       string      `json:"path"`       // raw device path (/dev/sdb, \\.\PhysicalDrive2)
	Model      string      `json:"model"`
	Serial     string      `json:"serial"`
	Bus        string      `json:"bus"` // usb, sata, nvme, ...
	SizeBytes  int64       `json:"sizeBytes"`
	Removable  bool        `json:"removable"`
	IsSystem   bool        `json:"isSystem"` // contains the running OS: never a valid destination
	Partitions []Partition `json:"partitions"`
}

// Partition is a partition of a Disk.
type Partition struct {
	ID          string   `json:"id"`
	Path        string   `json:"path"` // /dev/sdb1, \\?\Volume{GUID}\
	Number      int      `json:"number"`
	SizeBytes   int64    `json:"sizeBytes"`
	FSType      string   `json:"fsType"` // ntfs, vfat, exfat, ext4, ... ("" if unknown)
	Label       string   `json:"label"`
	MountPoints []string `json:"mountPoints"`
	// Supported is true when the running OS can read the file system natively.
	Supported bool `json:"supported"`
}
