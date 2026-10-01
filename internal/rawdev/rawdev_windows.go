package rawdev

import (
	"encoding/binary"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

const ioctlDiskGetLengthInfo = 0x0007405C

type windowsDevice struct {
	*os.File
	h     windows.Handle
	size  int64
	write bool
}

// Open opens \\.\PhysicalDriveN for unbuffered raw I/O (admin rights needed).
// Before opening for writing, the caller must lock and dismount the disk
// volumes (disk.PrepareForWrite).
func Open(path string, write bool) (Device, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	access := uint32(windows.GENERIC_READ)
	flags := uint32(windows.FILE_FLAG_NO_BUFFERING)
	if write {
		access |= windows.GENERIC_WRITE
		flags |= windows.FILE_FLAG_WRITE_THROUGH
	}
	h, err := windows.CreateFile(p, access, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, flags, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	out := make([]byte, 8)
	var n uint32
	if err := windows.DeviceIoControl(h, ioctlDiskGetLengthInfo, nil, 0, &out[0], uint32(len(out)), &n, nil); err != nil {
		windows.CloseHandle(h)
		return nil, fmt.Errorf("IOCTL_DISK_GET_LENGTH_INFO %s: %w", path, err)
	}
	return &windowsDevice{
		File:  os.NewFile(uintptr(h), path),
		h:     h,
		size:  int64(binary.LittleEndian.Uint64(out)),
		write: write,
	}, nil
}

func (d *windowsDevice) Size() int64 { return d.size }

// DropCache flushes the device write cache. Raw disk handles opened with
// FILE_FLAG_NO_BUFFERING bypass the system cache, so reads hit the medium.
func (d *windowsDevice) DropCache() error {
	if !d.write {
		return nil
	}
	return windows.FlushFileBuffers(d.h)
}

func (d *windowsDevice) Close() error {
	var flushErr error
	if d.write {
		flushErr = windows.FlushFileBuffers(d.h)
	}
	if err := d.File.Close(); err != nil {
		return err
	}
	return flushErr
}
