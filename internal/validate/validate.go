package validate

import (
	"path/filepath"
	"runtime"
	"strings"

	"diskclone/internal/disk"
)

// Error codes, translated by the frontend (errors.<code>).
const (
	CodeSameDisk          = "same_disk"
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
	case dst.IsSystem:
		return fail(CodeDestSystem)
	case dst.SizeBytes < src.SizeBytes:
		return fail(CodeDestTooSmall)
	case devSafe && !devSafeBuses[dst.Bus]:
		return fail(CodeDevSafe)
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
