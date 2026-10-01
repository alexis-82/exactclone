package disk

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// lsblkColumns are the columns requested from lsblk (-b: sizes in bytes).
// MOUNTPOINTS needs util-linux >= 2.37; older versions get MOUNTPOINT.
const (
	lsblkColumns    = "NAME,PATH,SIZE,MODEL,SERIAL,TRAN,TYPE,FSTYPE,LABEL,MOUNTPOINTS,PKNAME,RM"
	lsblkColumnsOld = "NAME,PATH,SIZE,MODEL,SERIAL,TRAN,TYPE,FSTYPE,LABEL,MOUNTPOINT,PKNAME,RM"
)

// linuxReadableFS are the file systems the Linux kernel (or ntfs-3g) reads natively.
var linuxReadableFS = map[string]bool{
	"vfat": true, "exfat": true, "ntfs": true, "ntfs3": true,
	"ext2": true, "ext3": true, "ext4": true, "xfs": true, "btrfs": true,
}

// systemMountPoints identify the disk running the OS.
var systemMountPoints = map[string]bool{
	"/": true, "/boot": true, "/boot/efi": true, "/usr": true, "/var": true, "[SWAP]": true,
}

// flexBool accepts true/false as well as "0"/"1" (older lsblk versions).
type flexBool bool

func (b *flexBool) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	*b = s == "true" || s == "1"
	return nil
}

// flexInt accepts numbers and numeric strings.
type flexInt int64

func (n *flexInt) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	if s == "null" || s == "" {
		*n = 0
		return nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	*n = flexInt(v)
	return err
}

type lsblkDevice struct {
	Name        string        `json:"name"`
	Path        string        `json:"path"`
	Size        flexInt       `json:"size"`
	Model       *string       `json:"model"`
	Serial      *string       `json:"serial"`
	Tran        *string       `json:"tran"`
	Type        string        `json:"type"`
	FSType      *string       `json:"fstype"`
	Label       *string       `json:"label"`
	MountPoints []*string     `json:"mountpoints"`
	MountPoint  *string       `json:"mountpoint"`
	RM          flexBool      `json:"rm"`
	Children    []lsblkDevice `json:"children"`
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

func (d lsblkDevice) mounts() []string {
	var out []string
	for _, m := range d.MountPoints {
		if m != nil && *m != "" {
			out = append(out, *m)
		}
	}
	if len(out) == 0 && d.MountPoint != nil && *d.MountPoint != "" {
		out = append(out, *d.MountPoint)
	}
	return out
}

// hasSystemMount reports whether d or any descendant (partition, LVM, LUKS)
// is mounted on a system mount point.
func (d lsblkDevice) hasSystemMount() bool {
	for _, m := range d.mounts() {
		if systemMountPoints[m] {
			return true
		}
	}
	for _, c := range d.Children {
		if c.hasSystemMount() {
			return true
		}
	}
	return false
}

// partitionNumber extracts the trailing number of sdb1, nvme0n1p2, mmcblk0p1.
func partitionNumber(name string) int {
	i := len(name)
	for i > 0 && name[i-1] >= '0' && name[i-1] <= '9' {
		i--
	}
	n, _ := strconv.Atoi(name[i:])
	return n
}

// parseLsblk converts `lsblk -J -b -o <columns>` output to disks.
// includeLoop adds attached loop devices (used only for tests/dev-safe mode).
func parseLsblk(data []byte, includeLoop bool) ([]Disk, error) {
	var out struct {
		BlockDevices []lsblkDevice `json:"blockdevices"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("parse lsblk output: %w", err)
	}
	disks := []Disk{}
	for _, d := range out.BlockDevices {
		if d.Type != "disk" && !(includeLoop && d.Type == "loop" && d.Size > 0) {
			continue
		}
		bus := str(d.Tran)
		if d.Type == "loop" {
			bus = "loop"
		}
		disk := Disk{
			ID:         d.Name,
			Path:       d.Path,
			Model:      str(d.Model),
			Serial:     str(d.Serial),
			Bus:        bus,
			SizeBytes:  int64(d.Size),
			Removable:  bool(d.RM) || bus == "usb",
			IsSystem:   d.hasSystemMount(),
			Partitions: []Partition{},
		}
		for _, c := range d.Children {
			if c.Type != "part" {
				continue
			}
			fs := str(c.FSType)
			disk.Partitions = append(disk.Partitions, Partition{
				ID:          c.Name,
				Path:        c.Path,
				Number:      partitionNumber(c.Name),
				SizeBytes:   int64(c.Size),
				FSType:      fs,
				Label:       str(c.Label),
				MountPoints: c.mounts(),
				Supported:   linuxReadableFS[fs] && len(c.Children) == 0,
			})
		}
		disks = append(disks, disk)
	}
	return disks, nil
}
