package validate

import (
	"errors"
	"testing"

	"diskclone/internal/disk"
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
