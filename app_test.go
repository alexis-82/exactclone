package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Review R5: after PrepareForWrite the Windows destination is offline even
// when the clone fails or is canceled, and the user must be told.
func TestCloneNotices(t *testing.T) {
	failed := coded("copy_failed", errors.New("operation canceled"))
	cases := []struct {
		name         string
		goos         string
		err          error
		gptMisplaced bool
		want         []string
	}{
		{"windows ok", "windows", nil, false, []string{NoticeCloneOffline}},
		{"windows canceled", "windows", failed, true, []string{NoticeDestOffline}},
		{"windows ok gpt misplaced", "windows", nil, true, []string{NoticeCloneOffline, NoticeGPTBackupHeader}},
		{"linux ok", "linux", nil, false, nil},
		{"linux gpt misplaced", "linux", nil, true, []string{NoticeGPTBackupHeader}},
		{"linux failed", "linux", failed, true, nil},
	}
	for _, c := range cases {
		got := cloneNotices(c.goos, c.err, c.gptMisplaced)
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
	if info.Name != "ExactClone" || info.Version == "" || info.Author == "" {
		t.Fatalf("product data not read from wails.json: %+v", info)
	}
	if info.OS == "" || info.Arch == "" || info.GoVersion == "" || info.ConfigPath == "" {
		t.Fatalf("environment data missing: %+v", info)
	}
}

// A config.json from 1.0.0 has no checkUpdates: the check stays on.
func TestConfigCheckUpdates(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	os.MkdirAll(filepath.Join(dir, "exactclone"), 0o755)
	os.WriteFile(filepath.Join(dir, "exactclone", "config.json"), []byte(`{"language":"en"}`), 0o644)
	if cfg := loadConfig(); !cfg.CheckUpdates || cfg.Language != "en" {
		t.Fatalf("1.0.0 config: %+v", cfg)
	}
	if err := (&App{}).SetCheckUpdates(false); err != nil {
		t.Fatal(err)
	}
	if cfg := loadConfig(); cfg.CheckUpdates || cfg.Language != "en" {
		t.Fatalf("after SetCheckUpdates(false): %+v", cfg)
	}
}

// Settings saved before the rename (config dir "diskclone") are still read.
func TestConfigMigratesFromOldName(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)         // Windows
	t.Setenv("XDG_CONFIG_HOME", dir) // Linux
	if loadConfig().Language != "it" {
		t.Fatal("default language must be it")
	}
	os.MkdirAll(filepath.Join(dir, "diskclone"), 0o755)
	os.WriteFile(filepath.Join(dir, "diskclone", "config.json"), []byte(`{"language":"en"}`), 0o644)
	if got := loadConfig().Language; got != "en" {
		t.Fatalf("old setting not migrated: %q", got)
	}
	if err := (&App{}).SetLanguage("it"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "exactclone", "config.json")); err != nil {
		t.Fatal("new settings must be saved under exactclone")
	}
	if got := loadConfig().Language; got != "it" {
		t.Fatalf("new setting must win over the old one: %q", got)
	}
}

// The version shown in the Info tab is the one set by the build
// (-ldflags "-X main.version=<tag>"), not productVersion in wails.json.
func TestAppInfoVersionFromBuild(t *testing.T) {
	withVersion(t, "v9.8.7")
	if got := appInfo().Version; got != "v9.8.7" {
		t.Fatalf("version %q, want the build version v9.8.7", got)
	}
	if got := (&App{}).GetVersion(); got != "v9.8.7" {
		t.Fatalf("GetVersion %q", got)
	}
}
