package archive

import (
	"io/fs"
	"syscall"
)

type fileKey struct{}

// hardLinkKey: hard links are rare on Windows volumes; they are stored as copies.
func hardLinkKey(fs.FileInfo) (fileKey, bool) { return fileKey{}, false }

// isReparsePoint reports junctions, mount points and other reparse points,
// which Go may report as plain directories.
func isReparsePoint(fi fs.FileInfo) bool {
	if d, ok := fi.Sys().(*syscall.Win32FileAttributeData); ok {
		return d.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0
	}
	return false
}

// owner is a no-op on Windows: extracted files inherit the folder ACLs.
type owner struct{}

func ownerOf(string) owner { return owner{} }
func (owner) apply(string) {}
