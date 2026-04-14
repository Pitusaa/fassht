package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/juanperetto/fassht/config"
)

func TestLoadAppConfig_Defaults(t *testing.T) {
	dir := t.TempDir()
	cfg, err := config.LoadAppConfigFrom(filepath.Join(dir, "config.toml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Editor != "" {
		t.Errorf("expected empty editor default, got '%s'", cfg.Editor)
	}
}

func TestLoadAppConfig_ReadsEditor(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	os.WriteFile(path, []byte(`editor = "code"`), 0644)

	cfg, err := config.LoadAppConfigFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Editor != "code" {
		t.Errorf("expected 'code', got '%s'", cfg.Editor)
	}
}
