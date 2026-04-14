package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/juanperetto/fassht/config"
	"github.com/juanperetto/fassht/tui"
)

func main() {
	appCfg, err := config.LoadAppConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	// Ensure ~/.config/fassht directory exists
	cfgPath := config.DefaultAppConfigPath()
	os.MkdirAll(cfgPath[:len(cfgPath)-len("/config.toml")], 0700)

	model := tui.NewModel(appCfg)

	p := tea.NewProgram(model, tea.WithAltScreen())

	// Handle SIGINT/SIGTERM to clean up temp files before exit
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
