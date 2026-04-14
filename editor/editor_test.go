package editor_test

import (
	"testing"

	"github.com/juanperetto/fassht/editor"
)

func TestResolve_UsesOverrideFirst(t *testing.T) {
	cmd := editor.Resolve("subl", "code", "")
	if cmd != "subl" {
		t.Errorf("expected 'subl', got '%s'", cmd)
	}
}

func TestResolve_UsesAppConfigSecond(t *testing.T) {
	cmd := editor.Resolve("", "code", "")
	if cmd != "code" {
		t.Errorf("expected 'code', got '%s'", cmd)
	}
}

func TestResolve_UsesEnvThird(t *testing.T) {
	cmd := editor.Resolve("", "", "nano")
	if cmd != "nano" {
		t.Errorf("expected 'nano', got '%s'", cmd)
	}
}

func TestResolve_FallsBackToSystemOpen(t *testing.T) {
	cmd := editor.Resolve("", "", "")
	// on macOS this is "open", on Linux "xdg-open"
	if cmd != "open" && cmd != "xdg-open" {
		t.Errorf("expected 'open' or 'xdg-open', got '%s'", cmd)
	}
}
