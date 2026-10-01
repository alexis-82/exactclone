//go:build integration && linux

// Integration tests on loop devices. Run as root on Linux:
//
//	sudo go test -tags integration -count=1 -v ./internal/itest/
package itest

import (
	"context"
	"crypto/rand"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"diskclone/internal/clone"
	"diskclone/internal/disk"
	"diskclone/internal/mount"
	"diskclone/internal/rawdev"
)

const loopSize = 256 << 20

func run(t *testing.T, name string, args ...string) string {
	t.Helper()
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// loop creates a backing file filled with data (random if nil) and attaches
// it as a partition-scanning loop device.
func loop(t *testing.T, random bool) (dev, backing string) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("integration tests need root")
	}
	backing = filepath.Join(t.TempDir(), "disk.img")
	data := make([]byte, loopSize)
	if random {
		rand.Read(data)
	}
	if err := os.WriteFile(backing, data, 0o600); err != nil {
		t.Fatal(err)
	}
	dev = run(t, "losetup", "--find", "--show", "--partscan", backing)
	t.Cleanup(func() { exec.Command("losetup", "-d", dev).Run() })
	return dev, backing
}

func findDisk(t *testing.T, dev string) disk.Disk {
	t.Helper()
	t.Setenv("DISKCLONE_DEV_SAFE", "1")
	disks, err := disk.List()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range disks {
		if d.Path == dev {
			return d
		}
	}
	t.Fatalf("%s not listed", dev)
	return disk.Disk{}
}

// partitioned creates a GPT loop disk with one partition formatted with fs
// and containing hello.txt.
func partitioned(t *testing.T, fs string) disk.Disk {
	t.Helper()
	dev, _ := loop(t, false)
	cmd := exec.Command("sfdisk", "--quiet", dev)
	cmd.Stdin = strings.NewReader("label: gpt\n,,\n")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("sfdisk: %v %s", err, out)
	}
	run(t, "udevadm", "settle")
	part := dev + "p1"
	run(t, "mkfs."+fs, part)
	dir := t.TempDir()
	run(t, "mount", part, dir)
	os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello"), 0o644)
	run(t, "umount", dir)
	return findDisk(t, dev)
}

func TestPrepareForWriteUnmountsAndFinishRescans(t *testing.T) {
	src := partitioned(t, "ext4")
	dst := partitioned(t, "vfat")

	// Simulate the desktop automounter on the destination.
	mnt := t.TempDir()
	run(t, "mount", dst.Partitions[0].Path, mnt)
	t.Cleanup(func() { exec.Command("umount", mnt).Run() })
	dst = findDisk(t, dst.Path)
	if _, err := rawdev.Open(dst.Path, true); err == nil {
		t.Fatal("exclusive open must fail while a partition is mounted")
	}

	if err := disk.PrepareForWrite(src, dst); err != nil {
		t.Fatal(err)
	}
	s, err := rawdev.Open(src.Path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	d, err := rawdev.Open(dst.Path, true)
	if err != nil {
		t.Fatalf("open after PrepareForWrite: %v", err)
	}
	if _, err := clone.Copy(context.Background(), s, d, s.Size(), clone.Options{HeadLast: true}, nil); err != nil {
		t.Fatal(err)
	}
	d.Close()
	if err := disk.FinishWrite(dst); err != nil {
		t.Fatal(err)
	}
	run(t, "udevadm", "settle")
	after := findDisk(t, dst.Path)
	if len(after.Partitions) != 1 || after.Partitions[0].FSType != "ext4" {
		t.Fatalf("destination after clone = %+v, want the ext4 partition of the source", after.Partitions)
	}
}

func TestMountReadOnly(t *testing.T) {
	for _, fs := range []string{"ext4", "vfat"} {
		t.Run(fs, func(t *testing.T) {
			d := partitioned(t, fs)
			p := d.Partitions[0]
			if !p.Supported {
				t.Fatalf("%s partition not supported: %+v", fs, p)
			}
			root, cleanup, err := mount.ReadOnly(p)
			if err != nil {
				t.Fatal(err)
			}
			if data, err := os.ReadFile(filepath.Join(root, "hello.txt")); err != nil || string(data) != "hello" {
				t.Fatalf("read: %q %v", data, err)
			}
			if err := os.WriteFile(filepath.Join(root, "x"), nil, 0o644); err == nil {
				t.Fatal("mount is not read-only")
			}
			info, err := mount.Stat(root)
			if err != nil || info.UsedBytes == 0 || info.FSType != fs {
				t.Fatalf("Stat = %+v, %v", info, err)
			}
			if err := cleanup(); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(root); !os.IsNotExist(err) {
				t.Fatal("temporary mount folder not removed")
			}
		})
	}
}

func TestMountReusesExistingMount(t *testing.T) {
	d := partitioned(t, "ext4")
	mnt := t.TempDir()
	run(t, "mount", d.Partitions[0].Path, mnt)
	t.Cleanup(func() { exec.Command("umount", mnt).Run() })
	d = findDisk(t, d.Path)

	root, cleanup, err := mount.ReadOnly(d.Partitions[0])
	if err != nil || root != mnt {
		t.Fatalf("root = %q, %v; want existing %q", root, err, mnt)
	}
	cleanup()
	if _, err := os.Stat(filepath.Join(mnt, "hello.txt")); err != nil {
		t.Fatal("cleanup unmounted a mount it did not create")
	}
}

func TestCloneAndVerifyLoop(t *testing.T) {
	srcPath, _ := loop(t, true)
	dstPath, dstBacking := loop(t, false)

	src, err := rawdev.Open(srcPath, false)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	dst, err := rawdev.Open(dstPath, true)
	if err != nil {
		t.Fatal(err)
	}
	defer dst.Close()
	if src.Size() != loopSize || dst.Size() != loopSize {
		t.Fatalf("sizes %d %d", src.Size(), dst.Size())
	}

	sum, err := clone.Copy(context.Background(), src, dst, src.Size(), clone.Options{HeadLast: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := clone.Verify(context.Background(), dst, src.Size(), sum, nil); err != nil {
		t.Fatalf("verify: %v", err)
	}

	// Corrupt the medium behind the device: Verify must notice it, which
	// proves it does not read stale data from the page cache.
	f, _ := os.OpenFile(dstBacking, os.O_WRONLY, 0)
	f.WriteAt([]byte("CORRUPTED"), 100<<20)
	f.Sync()
	f.Close()
	if err := clone.Verify(context.Background(), dst, src.Size(), sum, nil); !errors.Is(err, clone.ErrVerifyMismatch) {
		t.Fatalf("verify after corruption: %v, want mismatch", err)
	}
}
