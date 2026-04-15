package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/Pitusaa/fassht/config"
	"github.com/Pitusaa/fassht/editor"
	fasshtssh "github.com/Pitusaa/fassht/ssh"
)

var (
	uploadedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	headerStyle   = lipgloss.NewStyle().Bold(true)
)

// SessionModel is the third screen: active editing session.
type SessionModel struct {
	tempPath   string
	remotePath string
	host       config.SSHHost
	client     *fasshtssh.Client
	appConfig  *config.AppConfig

	lastUpload     time.Time
	uploadMsg      string
	errMsg         string
	editorOverride string
}

func NewSessionModel(
	tempPath, remotePath string,
	host config.SSHHost,
	client *fasshtssh.Client,
	appConfig *config.AppConfig,
) SessionModel {
	return SessionModel{
		tempPath:   tempPath,
		remotePath: remotePath,
		host:       host,
		client:     client,
		appConfig:  appConfig,
	}
}

func (m SessionModel) Init() tea.Cmd {
	return openEditorCmd(m.appConfig, m.editorOverride, m.tempPath)
}

func (m SessionModel) Update(msg tea.Msg) (SessionModel, tea.Cmd) {
	switch msg := msg.(type) {
	case uploadDoneMsg:
		m.lastUpload = time.Now()
		m.uploadMsg = "Uploaded successfully"
		m.errMsg = ""
		return m, nil

	case uploadErrMsg:
		m.errMsg = msg.err.Error()
		m.uploadMsg = ""
		return m, nil

	case tea.KeyMsg:
		switch strings.ToLower(msg.String()) {
		case "w":
			m.uploadMsg = "Uploading…"
			m.errMsg = ""
			return m, uploadCmd(m.client, m.tempPath, m.remotePath)
		case "q":
			os.Remove(m.tempPath)
			return m, func() tea.Msg { return SessionDoneMsg{} }
		case "o":
			return m, openEditorCmd(m.appConfig, m.editorOverride, m.tempPath)
		}
	}
	return m, nil
}

func (m SessionModel) View() string {
	var sb strings.Builder
	sb.WriteString(headerStyle.Render("fassht — editing session") + "\n\n")
	sb.WriteString(fmt.Sprintf("  Remote:  %s@%s:%s\n", m.host.User, m.host.Hostname, m.remotePath))
	sb.WriteString(fmt.Sprintf("  Local:   %s\n", m.tempPath))

	if !m.lastUpload.IsZero() {
		sb.WriteString(fmt.Sprintf("  Uploaded: %s\n", m.lastUpload.Format("15:04:05")))
	} else {
		sb.WriteString("  Uploaded: never\n")
	}

	sb.WriteString("\n")
	if m.uploadMsg != "" {
		sb.WriteString(uploadedStyle.Render(m.uploadMsg) + "\n")
	}
	if m.errMsg != "" {
		sb.WriteString(errorStyle.Render(m.errMsg) + "\n")
	}

	sb.WriteString("\n" + dimStyle.Render("[w] upload  [o] re-open editor  [q] discard and exit session"))
	return sb.String()
}

// openEditorCmd launches the editor in the background so the TUI remains responsive.
func openEditorCmd(appCfg *config.AppConfig, override, filePath string) tea.Cmd {
	return func() tea.Msg {
		editorCmd := editor.Resolve(override, appCfg.Editor, os.Getenv("EDITOR"))
		go editor.Open(editorCmd, filePath) //nolint:errcheck
		return nil
	}
}

// uploadCmd uploads the temp file to the server.
func uploadCmd(client *fasshtssh.Client, localPath, remotePath string) tea.Cmd {
	return func() tea.Msg {
		if err := client.Upload(localPath, remotePath); err != nil {
			return uploadErrMsg{err}
		}
		return uploadDoneMsg{}
	}
}

type uploadDoneMsg struct{}
type uploadErrMsg struct{ err error }
