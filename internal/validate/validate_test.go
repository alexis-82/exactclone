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

func TestCloneRejectsSystemSource(t *testing.T) {
	src := disk.Disk{ID: "a", SizeBytes: 100, IsSystem: true}
	if got := code(Clone(src, disk.Disk{ID: "b", SizeBytes: 100}, false)); got != CodeSourceSystem {
		t.Fatalf("got %q", got)
	}
}

func TestImage(t *testing.T) {
	sep := string(filepath.Separator)
	srcMount := sep + filepath.Join("media", "usb")
	src := disk.Disk{ID: "a", SizeBytes: 8 << 30, Partitions: []disk.Partition{{ID: "a1", MountPoints: []string{srcMount}}}}
	elsewhere := sep + filepath.Join("home", "me")
	ntfs := mount.Info{FreeBytes: 8 << 30, FSType: "ntfs"}
	cases := []struct {
		name    string
		src     disk.Disk
		destDir string
		dest    mount.Info
		want    string
	}{
		{"ok, exact space", src, elsewhere, ntfs, ""},
		{"system source", disk.Disk{ID: "s", IsSystem: true}, elsewhere, ntfs, CodeSourceSystem},
		{"inside source partition", src, filepath.Join(srcMount, "img"), ntfs, CodeDestOnSource},
		{"source partition root", src, srcMount, ntfs, CodeDestOnSource},
		{"similar prefix is not inside", src, srcMount + "2", ntfs, ""},
		{"space < disk size", src, elsewhere, mount.Info{FreeBytes: 8<<30 - 1, FSType: "ntfs"}, CodeInsufficientSpace},
		{"fat32 big disk", src, elsewhere, mount.Info{FreeBytes: 64 << 30, FSType: "vfat"}, CodeFAT32Limit},
		{"fat32 small disk", disk.Disk{ID: "s", SizeBytes: 2 << 30}, elsewhere, mount.Info{FreeBytes: 64 << 30, FSType: "vfat"}, ""},
		{"exfat big disk", src, elsewhere, mount.Info{FreeBytes: 64 << 30, FSType: "exfat"}, ""},
	}
	for _, c := range cases {
		if got := code(Image(c.src, c.destDir, c.dest)); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestRestoreImage(t *testing.T) {
	sep := string(filepath.Separator)
	dstMount := sep + filepath.Join("media", "target")
	dst := disk.Disk{ID: "b", SizeBytes: 16 << 30, Bus: "usb", Partitions: []disk.Partition{{ID: "b1", MountPoints: []string{dstMount}}}}
	img := sep + filepath.Join("home", "me", "disk.img.zst")
	cases := []struct {
		name    string
		path    string
		size    int64
		dst     disk.Disk
		devSafe bool
		want    string
	}{
		{"ok", img, 16 << 30, dst, false, ""},
		{"smaller image ok", img, 8 << 30, dst, false, ""},
		{"too small", img, 16<<30 + 1, dst, false, CodeDestTooSmall},
		{"system", img, 1, disk.Disk{ID: "c", SizeBytes: 1 << 40, IsSystem: true}, false, CodeDestSystem},
		{"image on destination", filepath.Join(dstMount, "disk.img.zst"), 1, dst, false, CodeImageOnDest},
		{"dev safe sata", img, 1, disk.Disk{ID: "d", SizeBytes: 1 << 40, Bus: "sata"}, true, CodeDevSafe},
		{"dev safe usb", img, 1, dst, true, ""},
	}
	for _, c := range cases {
		if got := code(RestoreImage(c.path, c.size, c.dst, c.devSafe)); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
