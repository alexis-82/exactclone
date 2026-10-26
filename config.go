package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config is persisted in <user config dir>/exactclone/config.json; settings
// saved under the old name (diskclone) are read when it does not exist yet.
type Config struct {
	Language     string `json:"language"`     // "it" or "en"
	CheckUpdates bool   `json:"checkUpdates"` // look for a new release at startup
}

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "exactclone", "config.json"), nil
}

func loadConfig() Config {
	// Defaults; a file without a field (e.g. from 1.0.0) keeps them.
	cfg := Config{Language: "it", CheckUpdates: true}
	p, err := configPath()
	if err != nil {
		return cfg
	}
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		data, err = os.ReadFile(filepath.Join(filepath.Dir(filepath.Dir(p)), "diskclone", "config.json"))
	}
	if err == nil {
		json.Unmarshal(data, &cfg)
	}
	if cfg.Language != "it" && cfg.Language != "en" {
		cfg.Language = "it"
	}
	return cfg
}

func saveConfig(cfg Config) error {
	p, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	return os.WriteFile(p, data, 0o644)
}

// GetConfig returns the saved settings.
func (a *App) GetConfig() Config { return loadConfig() }

// SetLanguage saves the UI language ("it" or "en").
func (a *App) SetLanguage(lang string) error {
	if lang != "it" && lang != "en" {
		return coded("invalid_language", nil)
	}
	cfg := loadConfig()
	cfg.Language = lang
	return saveConfig(cfg)
}

// SetCheckUpdates saves whether to look for updates at startup.
func (a *App) SetCheckUpdates(on bool) error {
	cfg := loadConfig()
	cfg.CheckUpdates = on
	return saveConfig(cfg)
}
