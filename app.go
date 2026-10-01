package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"diskclone/internal/disk"
	"diskclone/internal/job"
	"diskclone/internal/mount"
	"diskclone/internal/privilege"
	"diskclone/internal/validate"
)

// Errors returned to the frontend have the form "code" or "code|detail";
// the frontend translates errors.<code>.
func coded(code string, err error) error {
	if err == nil {
		return errors.New(code)
	}
	return fmt.Errorf("%s|%w", code, err)
}

// App holds the methods bound to the frontend.
type App struct {
	ctx  context.Context
	jobs *job.Manager
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.jobs = job.NewManager(func(name string, data any) { runtime.EventsEmit(ctx, name, data) })
}

func devSafe() bool { return os.Getenv("DISKCLONE_DEV_SAFE") == "1" }

// IsElevated reports whether the app runs as administrator/root.
func (a *App) IsElevated() bool { return privilege.IsElevated() }

// IsDevSafe reports whether dev-safe mode (only USB/VHD/loop destinations) is on.
func (a *App) IsDevSafe() bool { return devSafe() }

// ListDisks returns the physical disks.
func (a *App) ListDisks() ([]disk.Disk, error) {
	disks, err := disk.List()
	if err != nil {
		return nil, coded("list_disks_failed", err)
	}
	return disks, nil
}

func findDisk(id string) (disk.Disk, error) {
	disks, err := disk.List()
	if err != nil {
		return disk.Disk{}, coded("list_disks_failed", err)
	}
	for _, d := range disks {
		if d.ID == id {
			return d, nil
		}
	}
	return disk.Disk{}, coded("disk_not_found", errors.New(id))
}

// DestinationInfo returns free space and file system of a folder.
func (a *App) DestinationInfo(path string) (mount.Info, error) {
	info, err := mount.Stat(path)
	if err != nil {
		return info, coded("stat_failed", err)
	}
	return info, nil
}

// StartClone starts a bit-by-bit copy of disk srcID onto disk dstID.
func (a *App) StartClone(srcID, dstID string, verify bool) error {
	if !privilege.IsElevated() {
		return coded(validate.CodeNotElevated, nil)
	}
	src, err := findDisk(srcID)
	if err != nil {
		return err
	}
	dst, err := findDisk(dstID)
	if err != nil {
		return err
	}
	if err := validate.Clone(src, dst, devSafe()); err != nil {
		return err
	}
	return a.start("clone", func(ctx context.Context, r *job.Reporter) (job.Result, error) {
		return runClone(ctx, r, src, dst, verify)
	})
}

func (a *App) start(kind string, fn job.Func) error {
	if err := a.jobs.Start(kind, fn); err != nil {
		return coded("busy", nil)
	}
	return nil
}

// Cancel stops the running operation.
func (a *App) Cancel() { a.jobs.Cancel() }

// PickSaveFile asks where to save a new archive.
func (a *App) PickSaveFile(defaultName string) (string, error) {
	return runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename: defaultName,
		Filters:         []runtime.FileFilter{{DisplayName: "Diskclone archive (*.tar.zst)", Pattern: "*.tar.zst"}},
	})
}
