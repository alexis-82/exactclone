package main

import (
	"context"
	"errors"
	"sync"

	"exactclone/internal/job"
	ghupdate "exactclone/internal/update"
)

// UpdateInfo is the result of an update check, as shown by the frontend.
type UpdateInfo struct {
	Current    string `json:"current"`
	Latest     string `json:"latest"`
	Available  bool   `json:"available"`  // latest is newer than current
	CanInstall bool   `json:"canInstall"` // there is a file to install on this installation
	NoRelease  bool   `json:"noRelease"`  // the repository has no release yet
	Notes      string `json:"notes"`
	PageURL    string `json:"pageUrl"`
}

// updater holds the update state of the app. The function fields are the
// OS-specific parts (update_<os>.go), replaceable in tests.
type updater struct {
	mu      sync.Mutex
	checker *ghupdate.Checker
	latest  *ghupdate.Release
	// busy is set while an update is downloaded and installed, and stays set
	// once the installer has started: no disk job may start meanwhile.
	busy    bool
	pick    func(assets []ghupdate.Asset) (ghupdate.Asset, error)
	install func(path string) error
	quit    func(ctx context.Context)
}

func newUpdater(quit func(ctx context.Context)) updater {
	return updater{checker: ghupdate.NewChecker(version), pick: platformAsset, install: installUpdate, quit: quit}
}

// updateError turns the errors of internal/update into frontend error codes.
func updateError(err error) error {
	switch {
	case errors.Is(err, ghupdate.ErrNoAsset):
		return coded("update_no_asset", nil)
	case errors.Is(err, ghupdate.ErrUnreachable):
		return coded("github_unreachable", err)
	case errors.Is(err, ghupdate.ErrDownload):
		return coded("update_download_failed", err)
	case errors.Is(err, ghupdate.ErrChecksum):
		return coded("update_checksum", err)
	}
	return coded("update_failed", err)
}

// GetVersion returns the installed version ("dev" for development builds).
func (a *App) GetVersion() string { return version }

// CheckUpdate asks GitHub for the latest release.
func (a *App) CheckUpdate() (UpdateInfo, error) {
	info := UpdateInfo{Current: version}
	r, err := a.updates.checker.Latest(a.ctx)
	if errors.Is(err, ghupdate.ErrNoRelease) {
		info.NoRelease = true
		return info, nil
	}
	if err != nil {
		return info, updateError(err)
	}
	a.updates.mu.Lock()
	a.updates.latest = &r
	a.updates.mu.Unlock()

	info.Latest = r.Version
	info.Notes = r.Notes
	info.PageURL = r.PageURL
	if newer, err := ghupdate.IsNewer(r.Version, version); err == nil {
		info.Available = newer // false for "dev" builds
	}
	_, assetErr := a.updates.pick(r.Assets)
	info.CanInstall = info.Available && assetErr == nil
	return info, nil
}

// InstallUpdate downloads the release file for this installation, starts the
// installer and quits. It returns only on error. It is refused while a disk
// job runs, and no job can start until it fails or the app exits.
func (a *App) InstallUpdate() error {
	u := &a.updates
	u.mu.Lock()
	if u.busy {
		u.mu.Unlock()
		return coded("update_busy", nil)
	}
	if a.jobs.Status() == job.StatusRunning {
		u.mu.Unlock()
		return coded("update_job_running", nil)
	}
	r := u.latest
	u.busy = true
	u.mu.Unlock()

	if err := a.downloadAndInstall(r); err != nil {
		u.mu.Lock()
		u.busy = false
		u.mu.Unlock()
		return err
	}
	u.quit(a.ctx)
	return nil
}

func (a *App) downloadAndInstall(r *ghupdate.Release) error {
	u := &a.updates
	if r == nil {
		return coded("update_check_first", nil)
	}
	if newer, err := ghupdate.IsNewer(r.Version, version); err != nil || !newer {
		return coded("update_none", nil)
	}
	asset, err := u.pick(r.Assets)
	if err != nil {
		return updateError(err)
	}
	path, err := u.checker.Download(a.ctx, asset, func(done, total int64) {
		a.emit("update:progress", map[string]any{"done": done, "total": total})
	})
	if err != nil {
		return updateError(err)
	}
	return u.install(path)
}
