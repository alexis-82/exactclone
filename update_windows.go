//go:build windows

package main

import (
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	ghupdate "exactclone/internal/update"
)

// uninstallKey is written by the NSIS installer (build/windows/installer/
// exactclone.nsi) in the 64-bit registry view (SetRegView 64).
const uninstallKey = `Software\Microsoft\Windows\CurrentVersion\Uninstall\ExactClone`

// installedDir returns the folder recorded by the installer, "" if none.
func installedDir() string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, uninstallKey, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return ""
	}
	defer k.Close()
	dir, _, err := k.GetStringValue("InstallLocation")
	if err != nil {
		return ""
	}
	return dir
}

// sameDir compares Windows folders: case-insensitive, trailing separator ignored.
func sameDir(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	clean := func(p string) string { return strings.TrimRight(filepath.Clean(p), `\/`) }
	return strings.EqualFold(clean(a), clean(b))
}

// isInstalled reports whether the running exe is the one installed by the
// setup; otherwise it is the portable exe, which the setup would not update.
func isInstalled() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return sameDir(filepath.Dir(exe), installedDir())
}

// platformAsset returns the setup for an installed ExactClone, ErrNoAsset for
// the portable exe (the update dialog then links the release page).
func platformAsset(assets []ghupdate.Asset) (ghupdate.Asset, error) {
	if !isInstalled() {
		return ghupdate.Asset{}, ghupdate.ErrNoAsset
	}
	return ghupdate.PickAsset(assets, "windows", goruntime.GOARCH)
}

// installUpdate starts the downloaded setup; the app quits right after.
// ShellExecute (unlike exec.Command) also handles an installer that asks
// for elevation; ExactClone is already elevated, so no UAC prompt appears.
func installUpdate(path string) error {
	err := windows.ShellExecute(0, windows.StringToUTF16Ptr("open"), windows.StringToUTF16Ptr(path),
		nil, windows.StringToUTF16Ptr(filepath.Dir(path)), windows.SW_SHOWNORMAL)
	if err != nil {
		return coded("update_install_failed", err)
	}
	return nil
}
