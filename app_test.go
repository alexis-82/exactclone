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

// Review R5: after PrepareForWrite the Windows destination is offline even
// when the clone fails or is canceled, and the user must be told.
func TestCloneNotices(t *testing.T) {
	failed := coded("copy_failed", errors.New("operation canceled"))
	cases := []struct {
		name       string
		goos       string
		err        error
		gpt, large bool
		want       []string
	}{
		{"windows ok", "windows", nil, false, false, []string{NoticeCloneOffline}},
		{"windows canceled", "windows", failed, true, true, []string{NoticeDestOffline}},
		{"windows ok gpt larger", "windows", nil, true, true, []string{NoticeCloneOffline, NoticeGPTBackupHeader}},
		{"linux ok", "linux", nil, false, false, nil},
		{"linux gpt larger", "linux", nil, true, true, []string{NoticeGPTBackupHeader}},
		{"linux failed", "linux", failed, true, true, nil},
	}
	for _, c := range cases {
		got := cloneNotices(c.goos, c.err, c.gpt, c.large)
		if len(got) != len(c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s: %v, want %v", c.name, got, c.want)
			}
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
