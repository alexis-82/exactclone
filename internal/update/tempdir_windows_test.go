//go:build windows

package update

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// The download directory must be closed to the non-elevated processes of the
// user: owner Administrators and a protected DACL (no inherited ACEs).
func TestPrivateTempDirACL(t *testing.T) {
	dir, err := privateTempDir()
	if err != nil {
		t.Skipf("needs an elevated process: %v", err)
	}
	defer os.RemoveAll(dir)
	sd, err := windows.GetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	owner, _, err := sd.Owner()
	if err != nil {
		t.Fatal(err)
	}
	if !owner.IsWellKnown(windows.WinBuiltinAdministratorsSid) {
		t.Fatalf("owner %s, want BUILTIN Administrators", owner)
	}
	ctrl, _, err := sd.Control()
	if err != nil {
		t.Fatal(err)
	}
	if ctrl&windows.SE_DACL_PROTECTED == 0 {
		t.Fatal("DACL is not protected: it inherits the ACEs of the parent")
	}
}

func TestRemoveStaleDownloads(t *testing.T) {
	root := t.TempDir()
	old := filepath.Join(root, "exactclone-update-old")
	recent := filepath.Join(root, "exactclone-update-recent")
	other := filepath.Join(root, "something-else")
	for _, d := range []string{old, recent, other} {
		os.Mkdir(d, 0o700)
		os.WriteFile(filepath.Join(d, "setup.exe"), []byte("x"), 0o600)
	}
	twoDays := time.Now().Add(-48 * time.Hour)
	os.Chtimes(old, twoDays, twoDays)
	os.Chtimes(other, twoDays, twoDays)

	removeStaleDownloads(root, time.Now().Add(-24*time.Hour))

	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Error("old download directory not removed")
	}
	if _, err := os.Stat(recent); err != nil {
		t.Error("recent download directory removed: its installer may be running")
	}
	if _, err := os.Stat(other); err != nil {
		t.Error("unrelated directory removed")
	}
}
