package main

import (
	"errors"
	"testing"
)

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

func TestAppInfo(t *testing.T) {
	info := appInfo()
	if info.Name != "Disk Clone" || info.Version == "" || info.Author == "" {
		t.Fatalf("product data not read from wails.json: %+v", info)
	}
	if info.OS == "" || info.Arch == "" || info.GoVersion == "" || info.ConfigPath == "" {
		t.Fatalf("environment data missing: %+v", info)
	}
}
