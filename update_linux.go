//go:build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"syscall"

	ghupdate "exactclone/internal/update"
)

// ExactClone is installed on Linux in /usr/bin/exactclone in three ways:
//   - .deb package (nfpm), updated with apt;
//   - .rpm package (nfpm, Fedora), updated with dnf;
//   - build/linux/install.sh, updated with exactclone-update.
//
// The path is the same, so the package database tells them apart.
const (
	linuxBinary       = "/usr/bin/exactclone"
	linuxUpdateScript = "/usr/bin/exactclone-update"
)

// ownedByDeb reports whether exe belongs to the "exactclone" Debian package.
// query runs `dpkg-query -S <path>` ("exactclone: /usr/bin/exactclone").
func ownedByDeb(exe string, query func(path string) (string, error)) bool {
	if exe != linuxBinary {
		return false
	}
	out, err := query(exe)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(out, "\n") {
		if pkgs, _, ok := strings.Cut(line, ": "); ok && pkgs == "exactclone" {
			return true
		}
	}
	return false
}

// ownedByRpm reports whether exe belongs to the "exactclone" rpm package.
// No rpm, or a file owned by no package (rpm exits with 1), means false.
func ownedByRpm(exe string, query func(path string) (string, error)) bool {
	if exe != linuxBinary {
		return false
	}
	name, err := query(exe)
	return err == nil && strings.TrimSpace(name) == "exactclone"
}

func dpkgQuery(path string) (string, error) {
	out, err := exec.Command("dpkg-query", "-S", path).Output()
	return string(out), err
}

func rpmQuery(path string) (string, error) {
	out, err := exec.Command("rpm", "-qf", "--qf", "%{NAME}", path).Output()
	return string(out), err
}

func runningExecutable() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		return resolved
	}
	return exe
}

// platformAsset picks the release file matching how ExactClone was installed.
func platformAsset(assets []ghupdate.Asset) (ghupdate.Asset, error) {
	exe := runningExecutable()
	if ownedByDeb(exe, dpkgQuery) {
		return ghupdate.PickDeb(assets, goruntime.GOARCH)
	}
	if ownedByRpm(exe, rpmQuery) {
		return ghupdate.PickRpm(assets, goruntime.GOARCH)
	}
	return ghupdate.PickAsset(assets, "linux", goruntime.GOARCH)
}

// installUpdate installs the downloaded file and starts the new version; the
// app quits right after. ExactClone already runs as root (pkexec), so the
// package manager is run directly.
func installUpdate(path string) error {
	exe := runningExecutable()

	var cmd *exec.Cmd
	switch {
	case strings.HasSuffix(path, ".deb"):
		// apt also installs any new dependency of the package.
		cmd = exec.Command("apt-get", "install", "-y", path)
		cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	case strings.HasSuffix(path, ".rpm"):
		dnf, err := exec.LookPath("dnf")
		if err != nil {
			return coded("update_tool_missing", fmt.Errorf("dnf: sudo dnf install %s", path))
		}
		cmd = exec.Command(dnf, "install", "-y", path)
	default:
		if _, err := os.Stat(linuxUpdateScript); err != nil {
			return coded("update_tool_missing", fmt.Errorf("%s: sudo exactclone-update", linuxUpdateScript))
		}
		cmd = exec.Command(linuxUpdateScript, path)
	}

	if out, err := cmd.CombinedOutput(); err != nil {
		return coded("update_install_failed", fmt.Errorf("%v\n%s", err, strings.TrimSpace(string(out))))
	}
	os.RemoveAll(filepath.Dir(path))

	// Same binary path, same environment (display, language) as this
	// process: no new password prompt. Its own session survives our exit.
	relaunch := exec.Command(exe)
	relaunch.Env = os.Environ()
	relaunch.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := relaunch.Start(); err != nil {
		return coded("update_relaunch_failed", err)
	}
	return nil
}
