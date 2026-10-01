package mount

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"diskclone/internal/disk"
)

// systemPartition returns the partition of the system drive, reached by its
// volume GUID path as if it had no drive letter.
func systemPartition(t *testing.T) disk.Partition {
	t.Helper()
	disks, err := disk.List()
	if err != nil {
		t.Fatal(err)
	}
	sys := os.Getenv("SystemDrive") + `\`
	for _, d := range disks {
		for _, p := range d.Partitions {
			for _, m := range p.MountPoints {
				if strings.EqualFold(m, sys) {
					return p
				}
			}
		}
	}
	t.Fatal("system partition not found")
	return disk.Partition{}
}

func TestReadOnlyUsesDriveLetter(t *testing.T) {
	p := systemPartition(t)
	root, cleanup, err := ReadOnly(p)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if !strings.EqualFold(root, os.Getenv("SystemDrive")+`\`) {
		t.Fatalf("root = %q", root)
	}
}

func TestReadOnlyVolumeWithoutLetter(t *testing.T) {
	p := systemPartition(t)
	p.MountPoints = nil
	root, cleanup, err := ReadOnly(p)
	if err != nil {
		t.Fatal(err)
	}
	if root != p.Path {
		t.Fatalf("root = %q, want GUID path %q", root, p.Path)
	}
	if _, err := os.Stat(filepath.Join(root, "Windows", "win.ini")); err != nil {
		t.Fatalf("cannot read through GUID path: %v", err)
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
}

func TestStat(t *testing.T) {
	p := systemPartition(t)
	byLetter, err := Stat(os.Getenv("SystemDrive") + `\`)
	if err != nil {
		t.Fatal(err)
	}
	byGUID, err := Stat(p.Path)
	if err != nil {
		t.Fatal(err)
	}
	if byLetter.FSType != "ntfs" || byLetter.UsedBytes == 0 || byLetter.FreeBytes == 0 {
		t.Fatalf("Stat = %+v", byLetter)
	}
	if byGUID.FSType != "ntfs" || byGUID.UsedBytes == 0 {
		t.Fatalf("Stat by GUID = %+v", byGUID)
	}
	sub, err := Stat(filepath.Join(os.Getenv("SystemDrive")+`\`, "Windows"))
	if err != nil || sub.FSType != "ntfs" {
		t.Fatalf("Stat of a subfolder = %+v, %v", sub, err)
	}
}

func TestMaxFileSize(t *testing.T) {
	if MaxFileSize("vfat") != 1<<32-1 || MaxFileSize("ntfs") != 0 || MaxFileSize("exfat") != 0 {
		t.Fatal("wrong limits")
	}
}
