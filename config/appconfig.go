package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// AppConfig holds fassht's user preferences.
type AppConfig struct {
	Editor string `toml:"editor"`
}

// DefaultAppConfigPath returns ~/.config/fassht/config.toml.
func DefaultAppConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "fassht", "config.toml")
}

// LoadAppConfigFrom reads the config file at path. If the file does not exist,
// it returns a zero-value AppConfig with no error.
func LoadAppConfigFrom(path string) (*AppConfig, error) {
	cfg := &AppConfig{}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil
	}
	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// LoadAppConfig loads from the default config path.
func LoadAppConfig() (*AppConfig, error) {
	return LoadAppConfigFrom(DefaultAppConfigPath())
}
