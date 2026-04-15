package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/Pitusaa/fassht/config"
	"github.com/Pitusaa/fassht/tui"
)

func main() {
	appCfg, err := config.LoadAppConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	// Ensure ~/.config/fassht directory exists
	if cfgPath := config.DefaultAppConfigPath(); cfgPath != "" {
		if err := os.MkdirAll(filepath.Dir(cfgPath), 0700); err != nil {
			fmt.Fprintf(os.Stderr, "error creating config directory: %v\n", err)
			os.Exit(1)
		}
	}

	model := tui.NewModel(appCfg)

	p := tea.NewProgram(model, tea.WithAltScreen())

	// Handle SIGINT/SIGTERM to gracefully shutdown the TUI
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		p.Quit()
	}()

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
