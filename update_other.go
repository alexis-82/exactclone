//go:build !windows && !linux

package main

import (
	goruntime "runtime"

	ghupdate "exactclone/internal/update"
)

func installUpdate(path string) error {
	return coded("update_install_unsupported", nil)
}

func platformAsset(assets []ghupdate.Asset) (ghupdate.Asset, error) {
	return ghupdate.PickAsset(assets, goruntime.GOOS, goruntime.GOARCH)
}
