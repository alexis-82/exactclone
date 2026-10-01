package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config is persisted in <user config dir>/diskclone/config.json.
type Config struct {
	Language string `json:"language"` // "it" or "en"
}

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "diskclone", "config.json"), nil
}

func loadConfig() Config {
	cfg := Config{Language: "it"}
	p, err := configPath()
	if err != nil {
		return cfg
	}
	if data, err := os.ReadFile(p); err == nil {
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
