package main

import (
	"errors"
	"testing"

	"diskclone/internal/archive"
	"diskclone/internal/disk"
)

func TestRootName(t *testing.T) {
	cases := []struct{ label, want string }{
		{"", "part1"},
		{"DATA", "part1_DATA"},
		{"Mio Disco!", "part1_Mio_Disco_"},
		{"cc/..", "part1_cc_"},
		{"àèì", "part1"},
	}
	for _, c := range cases {
		if got := rootName(0, disk.Partition{Label: c.label}); got != c.want {
			t.Errorf("rootName(%q) = %q, want %q", c.label, got, c.want)
		}
	}
}

func TestRestoreNeeded(t *testing.T) {
	m := archive.Manifest{ContentBytes: 500, Partitions: []archive.ManifestPartition{{UsedBytes: 10}, {UsedBytes: 20}}}
	if got := restoreNeeded(m); got != 500 {
		t.Fatalf("needed = %d, want the apparent size 500", got)
	}
	m.ContentBytes = 0
	if got := restoreNeeded(m); got != 30 {
		t.Fatalf("needed = %d, want the used-space fallback 30", got)
	}
}

func TestCoded(t *testing.T) {
	inner := errors.New("disk unplugged")
	err := coded("copy_failed", inner)
	if err.Error() != "copy_failed|disk unplugged" || !errors.Is(err, inner) {
		t.Fatalf("coded = %v", err)
	}
	if coded("busy", nil).Error() != "busy" {
		t.Fatal("code without detail")
	}
}
