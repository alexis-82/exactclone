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
	CodeSameDisk          = "same_disk"
	CodeSourceSystem      = "source_system"
	CodeImageOnDest       = "image_on_dest"
	CodeDestSystem        = "dest_system"
	CodeDestTooSmall      = "dest_too_small"
	CodeDevSafe           = "dev_safe_not_external"
	CodeDestOnSource      = "dest_on_source"
	CodeInsufficientSpace = "insufficient_space"
	CodeFAT32Limit        = "fat32_limit"
	CodeNotElevated       = "not_elevated"
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
	case src.IsSystem:
		return fail(CodeSourceSystem) // in use: a "hot" copy is not supported
	case dst.IsSystem:
		return fail(CodeDestSystem)
	case dst.SizeBytes < src.SizeBytes:
		return fail(CodeDestTooSmall)
	case devSafe && !devSafeBuses[dst.Bus]:
		return fail(CodeDevSafe)
	}
	return nil
}

// Image checks the creation of an image of src in the folder destDir, whose
// file system is described by dest. The compressed size is unknown in
// advance, so the whole disk size must fit.
func Image(src disk.Disk, destDir string, dest mount.Info) error {
	switch {
	case src.IsSystem:
		return fail(CodeSourceSystem)
	case onDisk(destDir, src):
		return fail(CodeDestOnSource) // the image would contain itself
	case dest.FreeBytes < uint64(src.SizeBytes):
		return fail(CodeInsufficientSpace)
	}
	if max := mount.MaxFileSize(dest.FSType); max > 0 && uint64(src.SizeBytes) > max {
		return fail(CodeFAT32Limit)
	}
	return nil
}

// RestoreImage checks writing the image at imagePath (made from a disk of
// imageSize bytes) onto dst.
func RestoreImage(imagePath string, imageSize int64, dst disk.Disk, devSafe bool) error {
	switch {
	case dst.IsSystem:
		return fail(CodeDestSystem)
	case onDisk(imagePath, dst):
		return fail(CodeImageOnDest) // the image would be overwritten while read
	case dst.SizeBytes < imageSize:
		return fail(CodeDestTooSmall)
	case devSafe && !devSafeBuses[dst.Bus]:
		return fail(CodeDevSafe)
	}
	return nil
}

// onDisk reports whether path is on a mounted partition of d.
func onDisk(path string, d disk.Disk) bool {
	for _, p := range d.Partitions {
		for _, m := range p.MountPoints {
			if m != "" && m != "[SWAP]" && isUnder(path, m) {
				return true
			}
		}
	}
	return false
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
