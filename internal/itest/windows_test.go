//go:build integration && windows

// Integration tests on VHD files. Run from an elevated (administrator) shell:
//
//	go test -tags integration -count=1 -v ./internal/itest/
package itest

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows"

	"exactclone/internal/clone"
	"exactclone/internal/disk"
	"exactclone/internal/rawdev"
)

const ioctlDiskGetDiskAttributes = 0x000700F0

func diskpart(t *testing.T, script string) {
	t.Helper()
	f := filepath.Join(t.TempDir(), "dp.txt")
	os.WriteFile(f, []byte(script), 0o600)
	if out, err := exec.Command("diskpart", "/s", f).CombinedOutput(); err != nil {
		t.Fatalf("diskpart: %v\n%s\n%s", err, script, out)
	}
}

func listIDs(t *testing.T) map[string]bool {
	t.Helper()
	disks, err := disk.List()
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, d := range disks {
		ids[d.ID] = true
	}
	return ids
}

// vhd creates and attaches a 256 MiB VHDX prepared by layout (diskpart
// commands run with the new disk selected) and returns the new disk.
func vhd(t *testing.T, layout string) disk.Disk {
	t.Helper()
	if !windows.GetCurrentProcessToken().IsElevated() {
		t.Skip("integration tests need an elevated shell")
	}
	before := listIDs(t)
	file := filepath.Join(t.TempDir(), "disk.vhdx")
	diskpart(t, fmt.Sprintf("create vdisk file=\"%s\" maximum=256 type=expandable\nselect vdisk file=\"%s\"\nattach vdisk\n%s", file, file, layout))
	t.Cleanup(func() {
		diskpart(t, fmt.Sprintf("select vdisk file=\"%s\"\ndetach vdisk", file))
	})
	time.Sleep(2 * time.Second) // let Windows mount the new volumes
	disks, _ := disk.List()
	for _, d := range disks {
		if !before[d.ID] {
			return d
		}
	}
	t.Fatal("new VHD disk not found")
	return disk.Disk{}
}

func isOffline(t *testing.T, d disk.Disk) bool {
	t.Helper()
	p, _ := windows.UTF16PtrFromString(d.Path)
	h, err := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(h)
	out := make([]byte, 16)
	var n uint32
	if err := windows.DeviceIoControl(h, ioctlDiskGetDiskAttributes, nil, 0, &out[0], 16, &n, nil); err != nil {
		t.Fatal(err)
	}
	return binary.LittleEndian.Uint64(out[8:])&1 != 0
}

const sourceLayout = `convert gpt
create partition primary size=100
format fs=ntfs quick label=SRCNTFS
assign
create partition primary size=60
format fs=fat32 quick label=SRCFAT
assign
create partition primary
format fs=ntfs quick label=NOLETTER
`

func TestCloneGPTWithMountedVolumes(t *testing.T) {
	src := vhd(t, sourceLayout)
	dst := vhd(t, "convert gpt\ncreate partition primary\nformat fs=fat32 quick label=DSTFAT\nassign\n")
	t.Logf("src %s %+v", src.ID, src.Partitions)
	t.Logf("dst %s %+v", dst.ID, dst.Partitions)
	if len(src.Partitions) < 3 {
		t.Fatalf("source has %d partitions, want 3 (+MSR)", len(src.Partitions))
	}

	release, err := disk.PrepareForWrite(src, dst)
	if err != nil {
		t.Fatal(err)
	}
	s, err := rawdev.Open(src.Path, false)
	if err != nil {
		t.Fatal(err)
	}
	d, err := rawdev.Open(dst.Path, true)
	if err != nil {
		t.Fatal(err)
	}
	sum, err := clone.Copy(context.Background(), s, d, s.Size(), clone.Options{HeadLast: true}, nil)
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if err := clone.Verify(context.Background(), d, s.Size(), sum, nil); err != nil {
		t.Fatalf("verify: %v", err)
	}
	d.Close()
	s.Close()
	release()
	if err := disk.FinishWrite(dst); err != nil {
		t.Fatal(err)
	}
	if !isOffline(t, dst) {
		t.Fatal("destination must stay offline after the clone")
	}
}

// Drive -> Image -> Drive with mounted volumes on both disks; the destination
// ends up offline like after a clone.
func TestImageRoundTripVHD(t *testing.T) {
	src := vhd(t, sourceLayout)
	dst := vhd(t, "convert gpt\ncreate partition primary\nformat fs=fat32 quick label=DSTFAT\nassign\n")
	imageRoundTrip(t, src, dst)
	if err := disk.FinishWrite(dst); err != nil {
		t.Fatal(err)
	}
	if !isOffline(t, dst) {
		t.Fatal("destination must stay offline after the restore")
	}
}
