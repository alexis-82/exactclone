package validate

import (
	"path/filepath"
	"runtime"
	"strings"

	"diskclone/internal/disk"
	"diskclone/internal/mount"
)

// Error codes, translated by the frontend (errors.<code>).
const (
	CodeSameDisk             = "same_disk"
	CodeDestSystem           = "dest_system"
	CodeDestTooSmall         = "dest_too_small"
	CodeDevSafe              = "dev_safe_not_external"
	CodeNoPartition          = "no_partition"
	CodePartitionUnsupported = "partition_unsupported"
	CodeDestOnSource         = "dest_on_source"
	CodeInsufficientSpace    = "insufficient_space"
	CodeFAT32Limit           = "fat32_limit"
	CodeNotElevated          = "not_elevated"
)

// Error is a rule violation identified by a code.
type Error struct{ Code string }

func (e *Error) Error() string { return e.Code }

func fail(code string) error { return &Error{Code: code} }

// devSafeBuses are the only destinations allowed in dev-safe mode
// (DISKCLONE_DEV_SAFE=1): USB drives, loop devices and VHDs. The removable
// flag is not used because Windows reports hot-plug SATA disks as removable.
var devSafeBuses = map[string]bool{"usb": true, "loop": true, "virtual": true}

// Clone checks a bit-by-bit disk-to-disk copy.
func Clone(src, dst disk.Disk, devSafe bool) error {
	switch {
	case src.ID == dst.ID:
		return fail(CodeSameDisk)
	case dst.IsSystem:
		return fail(CodeDestSystem)
	case dst.SizeBytes < src.SizeBytes:
		return fail(CodeDestTooSmall)
	case devSafe && !devSafeBuses[dst.Bus]:
		return fail(CodeDevSafe)
	}
	return nil
}

// Archive checks a file-by-file backup of parts into the folder destDir,
// whose file system is described by dest; estimate is the archive size.
func Archive(parts []disk.Partition, destDir string, dest mount.Info, estimate uint64) error {
	if len(parts) == 0 {
		return fail(CodeNoPartition)
	}
	for _, p := range parts {
		if !p.Supported {
			return fail(CodePartitionUnsupported)
		}
		for _, m := range append(append([]string{}, p.MountPoints...), p.Path) {
			if m != "" && isUnder(destDir, m) {
				return fail(CodeDestOnSource) // the archive would contain itself
			}
		}
	}
	if dest.FreeBytes < estimate {
		return fail(CodeInsufficientSpace)
	}
	if max := mount.MaxFileSize(dest.FSType); max > 0 && estimate > max {
		return fail(CodeFAT32Limit)
	}
	return nil
}

// Restore checks the extraction of an archive whose content needs `needed` bytes.
func Restore(dest mount.Info, needed uint64) error {
	if dest.FreeBytes < needed {
		return fail(CodeInsufficientSpace)
	}
	return nil
}

// isUnder reports whether path is root or inside it.
func isUnder(path, root string) bool {
	p, r := filepath.Clean(path), filepath.Clean(root)
	if runtime.GOOS == "windows" {
		p, r = strings.ToLower(p), strings.ToLower(r)
	}
	if p == r {
		return true
	}
	if !strings.HasSuffix(r, string(filepath.Separator)) {
		r += string(filepath.Separator)
	}
	return strings.HasPrefix(p, r)
}
