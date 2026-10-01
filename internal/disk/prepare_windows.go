package disk

import (
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	"golang.org/x/sys/windows"
)

const (
	fsctlLockVolume            = 0x00090018
	fsctlDismountVolume        = 0x00090020
	ioctlDiskSetDiskAttributes = 0x0007C0F4
	ioctlDiskUpdateProperties  = 0x00070140
	diskAttributeOffline       = 0x1
)

// PrepareForRead locks and dismounts every volume of src, so it does not
// change while it is copied into an image. release closes the locks; Windows
// remounts the volumes on the next access.
func PrepareForRead(src Disk) (release func(), err error) {
	return lockDisks(src)
}

// PrepareForWrite locks and dismounts every volume of src and dst (so neither
// changes during the copy) and takes dst offline, so Windows does not mount
// the new partitions while the partition table is being written. release
// closes the volume locks; dst stays offline on purpose (see FinishWrite).
// src may be an empty Disk (restore from an image).
func PrepareForWrite(src, dst Disk) (release func(), err error) {
	release, err = lockDisks(src, dst)
	if err != nil {
		return nil, err
	}
	if err := setOffline(dst.Path, true); err != nil {
		release()
		return nil, err
	}
	return release, nil
}

func lockDisks(disks ...Disk) (func(), error) {
	var locked []windows.Handle
	closeAll := func() {
		for _, h := range locked {
			windows.CloseHandle(h)
		}
	}
	for _, d := range disks {
		for _, p := range d.Partitions {
			if !strings.HasPrefix(p.Path, `\\?\Volume{`) {
				continue // partition without a volume: nothing to lock
			}
			h, err := lockVolume(p.Path, p.MountPoints)
			if err != nil {
				closeAll()
				return nil, err
			}
			locked = append(locked, h)
		}
	}
	return closeAll, nil
}

// Lock attempts before forcing the dismount: a volume that has just been
// mounted is often held open for a moment (Explorer/AutoPlay, indexing,
// antivirus).
const (
	lockAttempts = 20
	lockDelay    = 250 * time.Millisecond
)

// lockVolume locks and dismounts a volume. If it stays in use, the volume is
// force-dismounted (other processes' handles become invalid) and locked again.
func lockVolume(guidPath string, mountPoints []string) (windows.Handle, error) {
	name := guidPath
	if len(mountPoints) > 0 {
		name = mountPoints[0] + " (" + guidPath + ")"
	}
	p, _ := windows.UTF16PtrFromString(strings.TrimSuffix(guidPath, `\`))
	h, err := windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		return 0, fmt.Errorf("open volume %s: %w", name, err)
	}
	var n uint32
	ioctl := func(code uint32) error { return windows.DeviceIoControl(h, code, nil, 0, nil, 0, &n, nil) }

	err = ioctl(fsctlLockVolume)
	for i := 1; err != nil && i < lockAttempts; i++ {
		time.Sleep(lockDelay)
		err = ioctl(fsctlLockVolume)
	}
	if err != nil {
		if derr := ioctl(fsctlDismountVolume); derr != nil {
			windows.CloseHandle(h)
			return 0, fmt.Errorf("volume %s is in use and cannot be dismounted: %w", name, derr)
		}
		if err = ioctl(fsctlLockVolume); err != nil {
			windows.CloseHandle(h)
			return 0, fmt.Errorf("volume %s is in use (lock failed after forced dismount): %w", name, err)
		}
	}
	if err := ioctl(fsctlDismountVolume); err != nil {
		windows.CloseHandle(h)
		return 0, fmt.Errorf("dismount volume %s: %w", name, err)
	}
	return h, nil
}

// setOffline changes the disk offline state until the next reboot or reconnection.
func setOffline(diskPath string, offline bool) error {
	p, _ := windows.UTF16PtrFromString(diskPath)
	h, err := windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		return fmt.Errorf("open %s: %w", diskPath, err)
	}
	defer windows.CloseHandle(h)
	// SET_DISK_ATTRIBUTES{Version, Persist, Reserved1[3], Attributes, AttributesMask, Reserved2[4]}
	in := make([]byte, 40)
	binary.LittleEndian.PutUint32(in[0:], 40)
	if offline {
		binary.LittleEndian.PutUint64(in[8:], diskAttributeOffline)
	}
	binary.LittleEndian.PutUint64(in[16:], diskAttributeOffline)
	var n uint32
	if err := windows.DeviceIoControl(h, ioctlDiskSetDiskAttributes, &in[0], uint32(len(in)), nil, 0, &n, nil); err != nil {
		return fmt.Errorf("set %s offline=%v: %w", diskPath, offline, err)
	}
	return nil
}

// FinishWrite refreshes the cached partition layout of dst. The disk is left
// offline: bringing a clone online next to its source makes Windows change
// its disk signature, which breaks booting from the clone.
func FinishWrite(dst Disk) error {
	p, _ := windows.UTF16PtrFromString(dst.Path)
	h, err := windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	var n uint32
	return windows.DeviceIoControl(h, ioctlDiskUpdateProperties, nil, 0, nil, 0, &n, nil)
}
