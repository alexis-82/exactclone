package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"diskclone/internal/archive"
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

	// Used space per partition, measured once (mounting is slow) and
	// cleared by ListDisks.
	usageMu sync.Mutex
	usage   map[string]uint64
	measure func(disk.Partition) (uint64, error) // nil: measureUsage
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
	a.usageMu.Lock()
	a.usage = nil
	a.usageMu.Unlock()
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

func selectedPartitions(d disk.Disk, ids []string) []disk.Partition {
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	var parts []disk.Partition
	for _, p := range d.Partitions {
		if want[p.ID] {
			parts = append(parts, p)
		}
	}
	return parts
}

// DestinationInfo returns free space and file system of a folder.
func (a *App) DestinationInfo(path string) (mount.Info, error) {
	info, err := mount.Stat(path)
	if err != nil {
		return info, coded("stat_failed", err)
	}
	return info, nil
}

// PartitionEstimate is the used space of one partition.
type PartitionEstimate struct {
	ID        string `json:"id"`
	UsedBytes uint64 `json:"usedBytes"`
}

// Estimate is the expected archive size.
type Estimate struct {
	TotalBytes uint64              `json:"totalBytes"`
	Partitions []PartitionEstimate `json:"partitions"`
}

// EstimateArchive measures the used space of the selected partitions.
func (a *App) EstimateArchive(diskID string, partitionIDs []string) (Estimate, error) {
	d, err := findDisk(diskID)
	if err != nil {
		return Estimate{}, err
	}
	return a.estimate(d, selectedPartitions(d, partitionIDs))
}

func (a *App) estimate(d disk.Disk, parts []disk.Partition) (Estimate, error) {
	var est Estimate
	var usages []uint64
	for _, p := range parts {
		if !p.Supported {
			continue
		}
		used, err := a.usedSpace(d, p)
		if err != nil {
			return est, err
		}
		usages = append(usages, used)
		est.Partitions = append(est.Partitions, PartitionEstimate{ID: p.ID, UsedBytes: used})
	}
	est.TotalBytes = archive.Estimate(usages)
	return est, nil
}

// usedSpace returns the cached used space of p, measuring it the first time.
// The key includes serial and size so that another disk that got the same
// device name is never mistaken for this one.
func (a *App) usedSpace(d disk.Disk, p disk.Partition) (uint64, error) {
	key := fmt.Sprintf("%s|%s|%d|%s|%d", d.ID, d.Serial, d.SizeBytes, p.ID, p.SizeBytes)
	a.usageMu.Lock()
	used, ok := a.usage[key]
	a.usageMu.Unlock()
	if ok {
		return used, nil
	}
	measure := a.measure
	if measure == nil {
		measure = measureUsage
	}
	used, err := measure(p)
	if err != nil {
		return 0, err
	}
	a.usageMu.Lock()
	if a.usage == nil {
		a.usage = map[string]uint64{}
	}
	a.usage[key] = used
	a.usageMu.Unlock()
	return used, nil
}

// measureUsage mounts p read-only (if needed) and reads its used space.
func measureUsage(p disk.Partition) (uint64, error) {
	root, cleanup, err := mount.ReadOnly(p)
	if err != nil {
		return 0, coded("mount_failed", err)
	}
	defer cleanup()
	info, err := mount.Stat(root)
	if err != nil {
		return 0, coded("stat_failed", err)
	}
	return info.UsedBytes, nil
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

// StartArchive starts a file-by-file backup of the selected partitions to outPath.
func (a *App) StartArchive(diskID string, partitionIDs []string, outPath string) error {
	if !privilege.IsElevated() {
		return coded(validate.CodeNotElevated, nil)
	}
	d, err := findDisk(diskID)
	if err != nil {
		return err
	}
	parts := selectedPartitions(d, partitionIDs)
	est, err := a.estimate(d, parts)
	if err != nil {
		return err
	}
	destDir := filepath.Dir(outPath)
	info, err := mount.Stat(destDir)
	if err != nil {
		return coded("stat_failed", err)
	}
	if err := validate.Archive(parts, destDir, info, est.TotalBytes); err != nil {
		return err
	}
	if _, err := os.Stat(outPath); err == nil {
		return coded("file_exists", nil)
	}
	return a.start("archive", func(ctx context.Context, r *job.Reporter) (job.Result, error) {
		return runArchive(ctx, r, d, parts, est, outPath)
	})
}

// ReadArchiveInfo returns the manifest of an archive.
func (a *App) ReadArchiveInfo(path string) (archive.Manifest, error) {
	m, err := archive.ReadManifest(path)
	if err != nil {
		return m, coded("not_diskclone_archive", err)
	}
	return m, nil
}

// StartRestore extracts archivePath into destDir.
func (a *App) StartRestore(archivePath, destDir string) error {
	m, err := archive.ReadManifest(archivePath)
	if err != nil {
		return coded("not_diskclone_archive", err)
	}
	info, err := mount.Stat(destDir)
	if err != nil {
		return coded("stat_failed", err)
	}
	if err := validate.Restore(info, restoreNeeded(m)); err != nil {
		return err
	}
	return a.start("restore", func(ctx context.Context, r *job.Reporter) (job.Result, error) {
		return runRestore(ctx, r, archivePath, destDir)
	})
}

// restoreNeeded is the space needed to extract an archive: the apparent size
// of its files (sparse files are written in full).
func restoreNeeded(m archive.Manifest) uint64 {
	if m.ContentBytes > 0 {
		return uint64(m.ContentBytes)
	}
	var used uint64
	for _, p := range m.Partitions {
		used += p.UsedBytes
	}
	return used
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

// PickArchive asks for an existing archive.
func (a *App) PickArchive() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Filters: []runtime.FileFilter{{DisplayName: "Diskclone archive (*.tar.zst)", Pattern: "*.tar.zst"}},
	})
}

// PickFolder asks for a destination folder.
func (a *App) PickFolder() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{})
}

var unsafeChars = regexp.MustCompile(`[^A-Za-z0-9_-]+`)

// rootName is the archive folder of the i-th selected partition, e.g. "part1_DATA".
func rootName(i int, p disk.Partition) string {
	name := fmt.Sprintf("part%d", i+1)
	if label := unsafeChars.ReplaceAllString(p.Label, "_"); label != "" && label != "_" {
		name += "_" + label
	}
	return name
}
