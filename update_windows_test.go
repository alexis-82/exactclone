//go:build windows

package main

import (
	"errors"
	"os"
	"testing"

	ghupdate "exactclone/internal/update"
)

func TestSameDir(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{`C:\Program Files\ExactClone`, `C:\Program Files\ExactClone`, true},
		{`C:\Program Files\ExactClone`, `c:\program files\exactclone\`, true},
		{`C:\Program Files\ExactClone\`, `C:\Program Files\ExactClone`, true},
		{`C:\Users\u\Downloads`, `C:\Program Files\ExactClone`, false},
		{`C:\Program Files\ExactClone`, ``, false},
		{``, ``, false},
	}
	for _, c := range cases {
		if got := sameDir(c.a, c.b); got != c.want {
			t.Errorf("sameDir(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

// The test binary runs from a temporary folder, never from the folder of an
// NSIS installation: like a portable exe, it is not offered the setup even
// when the release has one.
func TestPlatformAssetPortable(t *testing.T) {
	assets := []ghupdate.Asset{{Name: "ExactClone-1.2.0-amd64-installer.exe"}}
	if _, err := platformAsset(assets); !errors.Is(err, ghupdate.ErrNoAsset) {
		t.Fatalf("portable exe: expected ErrNoAsset, got %v", err)
	}
}

// Opt-in check of the real installer (run as administrator, see
// .claude/test-report.md): a test binary copied into the NSIS installation
// folder must find InstallLocation in the 64-bit registry view.
func TestIsInstalledFromInstallDir(t *testing.T) {
	if os.Getenv("EXACTCLONE_EXPECT_INSTALLED") != "1" {
		t.Skip("set EXACTCLONE_EXPECT_INSTALLED=1 and run from the installation folder")
	}
	if !isInstalled() {
		t.Fatalf("not detected as installed: InstallLocation=%q", installedDir())
	}
	if _, err := platformAsset([]ghupdate.Asset{{Name: "ExactClone-1.2.0-amd64-installer.exe"}}); err != nil {
		t.Fatalf("installed copy must be offered the setup: %v", err)
	}
}
