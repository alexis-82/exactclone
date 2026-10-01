//go:build integration && linux

// Integration tests on loop devices. Run as root on Linux:
//
//	sudo go test -tags integration -count=1 -v ./internal/itest/
package itest

import (
	"context"
	"crypto/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"diskclone/internal/clone"
	"diskclone/internal/disk"
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

	release, err := disk.PrepareForWrite(src, dst)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
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

// Drive -> Image -> Drive between loop devices, with the source automounted.
func TestImageRoundTripLoop(t *testing.T) {
	src := partitioned(t, "ext4")
	mnt := t.TempDir()
	run(t, "mount", src.Partitions[0].Path, mnt)
	t.Cleanup(func() { exec.Command("umount", mnt).Run() })
	src = findDisk(t, src.Path)
	dstPath, _ := loop(t, false)
	dst := findDisk(t, dstPath)

	imageRoundTrip(t, src, dst)
	if err := disk.FinishWrite(dst); err != nil {
		t.Fatal(err)
	}
	run(t, "udevadm", "settle")
	after := findDisk(t, dst.Path)
	if len(after.Partitions) != 1 || after.Partitions[0].FSType != "ext4" {
		t.Fatalf("destination after restore = %+v", after.Partitions)
	}
}
