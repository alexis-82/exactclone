//go:build !windows

package archive

import (
	"io/fs"
	"os"
	"syscall"
)

type fileKey struct{ dev, ino uint64 }

// hardLinkKey identifies files with more than one hard link.
func hardLinkKey(fi fs.FileInfo) (fileKey, bool) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok || st.Nlink < 2 {
		return fileKey{}, false
	}
	return fileKey{dev: uint64(st.Dev), ino: uint64(st.Ino)}, true
}

func isReparsePoint(fs.FileInfo) bool { return false }

// owner is the uid/gid given to extracted files when running as root.
type owner struct {
	uid, gid int
	ok       bool
}

func ownerOf(dir string) owner {
	if os.Geteuid() != 0 {
		return owner{}
	}
	fi, err := os.Stat(dir)
	if err != nil {
		return owner{}
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return owner{}
	}
	return owner{uid: int(st.Uid), gid: int(st.Gid), ok: true}
}

func (o owner) apply(p string) {
	if o.ok {
		os.Lchown(p, o.uid, o.gid)
	}
}
