package main

import (
	_ "embed"
	"encoding/json"
	"runtime"
	"runtime/debug"

	"diskclone/internal/privilege"
)

// wails.json is the single source of the product name and version (Wails
// also writes them into the Windows executable properties).
//
//go:embed wails.json
var wailsJSON []byte

// AppInfo is shown in the Info tab.
type AppInfo struct {
	Name         string `json:"name"`
	Version      string `json:"version"`
	Author       string `json:"author"`
	OS           string `json:"os"`
	Arch         string `json:"arch"`
	GoVersion    string `json:"goVersion"`
	WailsVersion string `json:"wailsVersion"`
	ConfigPath   string `json:"configPath"`
	Elevated     bool   `json:"elevated"`
	DevSafe      bool   `json:"devSafe"`
}

func appInfo() AppInfo {
	var cfg struct {
		Author struct {
			Name string `json:"name"`
		} `json:"author"`
		Info struct {
			ProductName    string `json:"productName"`
			ProductVersion string `json:"productVersion"`
		} `json:"info"`
	}
	json.Unmarshal(wailsJSON, &cfg)
	info := AppInfo{
		Name:      cfg.Info.ProductName,
		Version:   cfg.Info.ProductVersion,
		Author:    cfg.Author.Name,
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		GoVersion: runtime.Version(),
		Elevated:  privilege.IsElevated(),
		DevSafe:   devSafe(),
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, dep := range bi.Deps {
			if dep.Path == "github.com/wailsapp/wails/v2" {
				info.WailsVersion = dep.Version
			}
		}
	}
	if p, err := configPath(); err == nil {
		info.ConfigPath = p
	}
	return info
}

// GetAppInfo returns version, build and environment details.
func (a *App) GetAppInfo() AppInfo { return appInfo() }
