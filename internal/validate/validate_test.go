package validate

import (
	"errors"
	"path/filepath"
	"testing"

	"diskclone/internal/disk"
	"diskclone/internal/mount"
)

func code(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	if err != nil {
		return "unexpected: " + err.Error()
	}
	return ""
}

func TestClone(t *testing.T) {
	src := disk.Disk{ID: "a", SizeBytes: 100, Bus: "usb"}
	cases := []struct {
		name    string
		dst     disk.Disk
		devSafe bool
		want    string
	}{
		{"ok", disk.Disk{ID: "b", SizeBytes: 100, Bus: "usb"}, false, ""},
		{"bigger ok", disk.Disk{ID: "b", SizeBytes: 200, Bus: "sata"}, false, ""},
		{"same disk", disk.Disk{ID: "a", SizeBytes: 100}, false, CodeSameDisk},
		{"system", disk.Disk{ID: "b", SizeBytes: 500, IsSystem: true}, false, CodeDestSystem},
		{"too small", disk.Disk{ID: "b", SizeBytes: 99}, false, CodeDestTooSmall},
		{"dev safe sata", disk.Disk{ID: "b", SizeBytes: 100, Bus: "sata", Removable: true}, true, CodeDevSafe},
		{"dev safe vhd", disk.Disk{ID: "b", SizeBytes: 100, Bus: "virtual"}, true, ""},
		{"dev safe loop", disk.Disk{ID: "b", SizeBytes: 100, Bus: "loop"}, true, ""},
	}
	for _, c := range cases {
		if got := code(Clone(src, c.dst, c.devSafe)); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestArchive(t *testing.T) {
	srcMount := filepath.Join(string(filepath.Separator)+"media", "usb")
	part := disk.Partition{ID: "p1", Path: "/dev/sdb1", Supported: true, MountPoints: []string{srcMount}}
	ntfs := mount.Info{FreeBytes: 1000, FSType: "ntfs"}
	elsewhere := filepath.Join(string(filepath.Separator)+"home", "me")
	cases := []struct {
		name     string
		parts    []disk.Partition
		destDir  string
		dest     mount.Info
		estimate uint64
		want     string
	}{
		{"ok", []disk.Partition{part}, elsewhere, ntfs, 1000, ""},
		{"no partition", nil, elsewhere, ntfs, 0, CodeNoPartition},
		{"unsupported", []disk.Partition{{ID: "x", Supported: false}}, elsewhere, ntfs, 0, CodePartitionUnsupported},
		{"dest inside source", []disk.Partition{part}, filepath.Join(srcMount, "backups"), ntfs, 1, CodeDestOnSource},
		{"dest is source root", []disk.Partition{part}, srcMount, ntfs, 1, CodeDestOnSource},
		{"similar prefix is not inside", []disk.Partition{part}, srcMount + "2", ntfs, 1, ""},
		{"no space", []disk.Partition{part}, elsewhere, ntfs, 1001, CodeInsufficientSpace},
		{"fat32 limit", []disk.Partition{part}, elsewhere, mount.Info{FreeBytes: 10 << 30, FSType: "vfat"}, 5 << 30, CodeFAT32Limit},
		{"fat32 small ok", []disk.Partition{part}, elsewhere, mount.Info{FreeBytes: 10 << 30, FSType: "vfat"}, 1 << 30, ""},
		{"exfat big ok", []disk.Partition{part}, elsewhere, mount.Info{FreeBytes: 10 << 30, FSType: "exfat"}, 5 << 30, ""},
	}
	for _, c := range cases {
		if got := code(Archive(c.parts, c.destDir, c.dest, c.estimate)); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestRestore(t *testing.T) {
	if code(Restore(mount.Info{FreeBytes: 10}, 10)) != "" {
		t.Fatal("exact space must be accepted")
	}
	if code(Restore(mount.Info{FreeBytes: 9}, 10)) != CodeInsufficientSpace {
		t.Fatal("insufficient space not detected")
	}
}
