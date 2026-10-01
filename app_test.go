package main

import (
	"errors"
	"testing"

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
