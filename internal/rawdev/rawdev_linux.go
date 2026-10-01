package rawdev

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

type linuxDevice struct {
	*os.File
	size  int64
	write bool
	block bool
}

// Open opens a block device (or a regular file, for tests). For writing the
// device is opened with O_EXCL, which the kernel refuses while any of its
// partitions is mounted.
func Open(path string, write bool) (Device, error) {
	flags := os.O_RDONLY
	if write {
		flags = os.O_RDWR | unix.O_EXCL
	}
	f, err := os.OpenFile(path, flags, 0)
	if err != nil {
		return nil, err
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	d := &linuxDevice{File: f, write: write}
	if fi.Mode()&os.ModeDevice != 0 {
		var size uint64
		if _, _, errno := unix.Syscall(unix.SYS_IOCTL, f.Fd(), unix.BLKGETSIZE64, uintptr(unsafe.Pointer(&size))); errno != 0 {
			f.Close()
			return nil, fmt.Errorf("BLKGETSIZE64 %s: %w", path, errno)
		}
		d.size, d.block = int64(size), true
	} else {
		d.size = fi.Size()
	}
	return d, nil
}

func (d *linuxDevice) Size() int64 { return d.size }

// DropCache writes dirty pages and evicts the device page cache, so the
// verification pass reads the physical medium.
func (d *linuxDevice) DropCache() error {
	if err := d.File.Sync(); err != nil {
		return err
	}
	fd := int(d.File.Fd())
	if d.block {
		if err := unix.IoctlSetInt(fd, unix.BLKFLSBUF, 0); err != nil {
			return fmt.Errorf("BLKFLSBUF: %w", err)
		}
	}
	return unix.Fadvise(fd, 0, 0, unix.FADV_DONTNEED)
}

func (d *linuxDevice) Close() error {
	var syncErr error
	if d.write {
		syncErr = d.File.Sync()
	}
	if err := d.File.Close(); err != nil {
		return err
	}
	return syncErr
}
