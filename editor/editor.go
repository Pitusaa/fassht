package editor

import (
	"os/exec"
	"runtime"
	"strings"
)

// Resolve returns the editor command to use, applying priority:
// 1. override (typed by user for this session)
// 2. appEditor (from ~/.config/fassht/config.toml)
// 3. envEditor (value of $EDITOR)
// 4. system default: "open" on macOS, "xdg-open" on Linux
func Resolve(override, appEditor, envEditor string) string {
	if override != "" {
		return override
	}
	if appEditor != "" {
		return appEditor
	}
	if envEditor != "" {
		return envEditor
	}
	if runtime.GOOS == "darwin" {
		return "open"
	}
	return "xdg-open"
}

// Open launches the editor for the given file path and waits for it to exit.
// editorCmd may contain arguments (e.g. "code --wait"), which are split on spaces.
func Open(editorCmd, filePath string) error {
	parts := strings.Fields(editorCmd)
	if len(parts) == 0 {
		parts = []string{"open"}
	}
	args := append(parts[1:], filePath)
	cmd := exec.Command(parts[0], args...)
	return cmd.Run()
}
