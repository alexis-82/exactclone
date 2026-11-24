package format

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"

	"exactclone/internal/disk"
)

// Run cleans disk d, writes a new partition table (scheme) with one partition
// over the whole disk, formats it with the file system fsID and assigns a
// drive letter, through a diskpart script.
func Run(ctx context.Context, d disk.Disk, fsID, scheme, label string) error {
	s, ok := lookup(goos, fsID)
	if !ok {
		return fmt.Errorf("unknown file system %q", fsID)
	}
	n, err := strconv.Atoi(strings.TrimPrefix(d.ID, "PhysicalDrive"))
	if err != nil {
		return fmt.Errorf("disk number of %s: %w", d.ID, err)
	}
	f, err := os.CreateTemp("", "exactclone-diskpart-*.txt")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, err = f.WriteString(diskpartScript(n, s, scheme, label))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "diskpart", "/s", f.Name())
	// GUI app: no console window flashing.
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("diskpart: %v\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
