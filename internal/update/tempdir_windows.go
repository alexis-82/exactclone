//go:build windows

package update

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// downloadDirSDDL: owner Administrators, protected DACL with full access for
// SYSTEM and Administrators only, and no implicit WRITE_DAC for the owner.
// The user's own non-elevated processes cannot touch the directory.
const downloadDirSDDL = "O:BAD:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;RC;;;OW)"

// privateTempDir creates the download directory under %SystemRoot%\Temp.
// %TEMP% is not used: ExactClone runs elevated but %TEMP% is writable by the
// non-elevated processes of the same user, which could replace the installer
// between its verification and its launch (and get it run without UAC).
// Standard users cannot rename or delete what others create in Windows\Temp.
func privateTempDir() (string, error) {
	sd, err := windows.SecurityDescriptorFromString(downloadDirSDDL)
	if err != nil {
		return "", err
	}
	sa := &windows.SecurityAttributes{SecurityDescriptor: sd}
	sa.Length = uint32(unsafe.Sizeof(*sa))
	win, err := windows.GetSystemWindowsDirectory()
	if err != nil {
		return "", err
	}
	root := filepath.Join(win, "Temp")
	removeStaleDownloads(root, time.Now().Add(-24*time.Hour))
	for range 10 {
		b := make([]byte, 8)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		dir := filepath.Join(root, "exactclone-update-"+hex.EncodeToString(b))
		p, err := windows.UTF16PtrFromString(dir)
		if err != nil {
			return "", err
		}
		err = windows.CreateDirectory(p, sa)
		if err == nil {
			return dir, nil
		}
		if !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
			return "", err
		}
	}
	return "", errors.New("cannot create the download directory")
}

// removeStaleDownloads deletes the download directories of past updates: the
// installer cannot delete itself and the app quits right after starting it.
// Recent ones are kept, their installer may still be running.
func removeStaleDownloads(root string, before time.Time) {
	dirs, _ := filepath.Glob(filepath.Join(root, "exactclone-update-*"))
	for _, d := range dirs {
		if fi, err := os.Stat(d); err == nil && fi.IsDir() && fi.ModTime().Before(before) {
			os.RemoveAll(d)
		}
	}
}
