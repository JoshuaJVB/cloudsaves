package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

type GameEntry struct {
	Name      string `json:"name"`
	LocalPath string `json:"local_path"`
}

type Config struct {
	ServerURL   string               `json:"server_url"`
	APIKey      string               `json:"api_key"`
	MachineName string               `json:"machine_name"`
	Games       map[string]GameEntry `json:"games"`
}

func configPath() (string, error) {
	var base string
	switch runtime.GOOS {
	case "windows":
		base = os.Getenv("APPDATA")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, "AppData", "Roaming")
		}
	default:
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			base = xdg
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, ".config")
		}
	}
	return filepath.Join(base, "cloudsave", "config.json"), nil
}

func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		hostname, _ := os.Hostname()
		return &Config{
			ServerURL:   "http://localhost:8080",
			APIKey:      "",
			MachineName: hostname,
			Games:       make(map[string]GameEntry),
		}, nil
	}
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Games == nil {
		cfg.Games = make(map[string]GameEntry)
	}
	return &cfg, nil
}

func (c *Config) Save() error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
