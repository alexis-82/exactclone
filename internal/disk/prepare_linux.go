package disk

import (
	"fmt"
	"os"
	"os/exec"

	"golang.org/x/sys/unix"
)

// PrepareForRead unmounts every mounted partition of src, so it does not
// change while it is copied into an image. release is a no-op on Linux.
func PrepareForRead(src Disk) (release func(), err error) {
	return unmountDisks(src)
}

// PrepareForWrite unmounts every mounted partition of src and dst (desktop
// environments automount USB drives), so the destination can be opened
// exclusively and the source is not modified during the copy. src may be an
// empty Disk (restore from an image). release is a no-op on Linux.
func PrepareForWrite(src, dst Disk) (release func(), err error) {
	return unmountDisks(src, dst)
}

func unmountDisks(disks ...Disk) (release func(), err error) {
	release = func() {}
	for _, d := range disks {
		for _, p := range d.Partitions {
			for _, m := range p.MountPoints {
				if m == "[SWAP]" {
					if out, err := exec.Command("swapoff", p.Path).CombinedOutput(); err != nil {
						return nil, fmt.Errorf("swapoff %s: %v: %s", p.Path, err, out)
					}
					continue
				}
				if err := unix.Unmount(m, 0); err != nil && err != unix.EINVAL {
					return nil, fmt.Errorf("unmount %s (%s): %w", m, p.Path, err)
				}
			}
		}
	}
	return release, nil
}

// FinishWrite asks the kernel to re-read the partition table of the written
// disk, so its new partitions appear without reconnecting it.
func FinishWrite(d Disk) error {
	f, err := os.Open(d.Path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := unix.IoctlSetInt(int(f.Fd()), unix.BLKRRPART, 0); err != nil {
		return fmt.Errorf("BLKRRPART %s: %w", d.Path, err)
	}
	return nil
}
