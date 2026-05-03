package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/Pitusaa/fassht/config"
	"github.com/Pitusaa/fassht/editor"
	fasshtssh "github.com/Pitusaa/fassht/ssh"
)

var (
	uploadedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	headerStyle   = lipgloss.NewStyle().Bold(true)
)

type sessionPermState int

const (
	sessionPermIdle sessionPermState = iota
	sessionPermChecking
	sessionPermConfirm
	sessionPermSudoPass
	sessionPermFixing
	sessionPermDenied
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

	permState       sessionPermState
	permGen         int
	permInfo        fasshtssh.PermissionInfo
	permReturnState sessionPermState
	sudoInput       textinput.Model
	permErrMsg      string
}

func NewSessionModel(
	tempPath, remotePath string,
	host config.SSHHost,
	client *fasshtssh.Client,
	appConfig *config.AppConfig,
) SessionModel {
	sudo := textinput.New()
	sudo.Placeholder = "sudo password"
	sudo.EchoMode = textinput.EchoPassword
	sudo.EchoCharacter = '•'
	return SessionModel{
		tempPath:   tempPath,
		remotePath: remotePath,
		host:       host,
		client:     client,
		appConfig:  appConfig,
		sudoInput:  sudo,
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
		m.permState = sessionPermIdle
		return m, nil

	case uploadErrMsg:
		m.errMsg = msg.err.Error()
		m.uploadMsg = ""
		return m, nil

	case permCheckMsg:
		if msg.gen != m.permGen {
			return m, nil
		}
		m.permErrMsg = ""
		if msg.err != nil {
			m.permState = sessionPermIdle
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.permInfo = msg.info
		if msg.info.Writable {
			m.permState = sessionPermIdle
			m.uploadMsg = "Uploading…"
			return m, uploadCmd(m.client, m.tempPath, m.remotePath)
		}
		if !msg.info.CanChmod && !msg.info.CanSudo {
			m.permState = sessionPermDenied
			return m, nil
		}
		if !msg.info.CanChmod && msg.info.NeedsSudo {
			m.sudoInput.SetValue("")
			m.sudoInput.Focus()
			m.permReturnState = sessionPermSudoPass
			m.permState = sessionPermSudoPass
			return m, textinput.Blink
		}
		m.permReturnState = sessionPermConfirm
		m.permState = sessionPermConfirm
		return m, nil

	case chmodDoneMsg:
		m.permState = sessionPermIdle
		m.permErrMsg = ""
		m.uploadMsg = "Uploading…"
		return m, uploadCmd(m.client, m.tempPath, m.remotePath)

	case chmodErrMsg:
		m.permErrMsg = msg.err.Error()
		m.permState = m.permReturnState
		if m.permReturnState == sessionPermSudoPass {
			m.sudoInput.SetValue("")
			m.sudoInput.Focus()
			return m, textinput.Blink
		}
		return m, nil

	case tea.KeyPressMsg:
		switch m.permState {
		case sessionPermChecking, sessionPermFixing:
			return m, nil

		case sessionPermConfirm:
			switch strings.ToLower(msg.String()) {
			case "y":
				m.permState = sessionPermFixing
				m.permErrMsg = ""
				if m.permInfo.CanChmod {
					return m, chmodCmd(m.client, m.remotePath, false, "")
				}
				return m, chmodCmd(m.client, m.remotePath, true, "")
			default:
				m.permState = sessionPermIdle
				m.permErrMsg = ""
			}
			return m, nil

		case sessionPermSudoPass:
			switch msg.String() {
			case "enter":
				pass := m.sudoInput.Value()
				m.sudoInput.SetValue("")
				m.permState = sessionPermFixing
				m.permErrMsg = ""
				return m, chmodCmd(m.client, m.remotePath, true, pass)
			case "esc":
				m.permState = sessionPermIdle
				m.permErrMsg = ""
				m.sudoInput.SetValue("")
				return m, nil
			default:
				var cmd tea.Cmd
				m.sudoInput, cmd = m.sudoInput.Update(msg)
				return m, cmd
			}

		case sessionPermDenied:
			switch msg.String() {
			case "enter", "esc":
				m.permState = sessionPermIdle
			}
			return m, nil
		}

		switch strings.ToLower(msg.String()) {
		case "w":
			m.errMsg = ""
			m.permGen++
			m.permState = sessionPermChecking
			return m, permCheckCmd(m.client, m.remotePath, m.permGen)
		case "q":
			os.Remove(m.tempPath)
			return m, func() tea.Msg { return SessionDoneMsg{} }
		case "o":
			return m, openEditorCmd(m.appConfig, m.editorOverride, m.tempPath)
		}
	}
	return m, nil
}

func (m SessionModel) View() tea.View {
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

	switch m.permState {
	case sessionPermChecking:
		sb.WriteString(loadingStyle.Render("Checking write permission…") + "\n")

	case sessionPermConfirm:
		verb := "chmod u+w"
		if !m.permInfo.CanChmod {
			verb = "sudo chmod u+w"
		}
		sb.WriteString(fmt.Sprintf("  File: %s\n\n", m.remotePath))
		sb.WriteString(errorStyle.Render("File is not writable."))
		sb.WriteString(fmt.Sprintf(" Run %s to fix it? [y/N] ", verb))
		if m.permErrMsg != "" {
			sb.WriteString("\n" + errorStyle.Render(m.permErrMsg))
		}

	case sessionPermSudoPass:
		sb.WriteString(fmt.Sprintf("  File: %s\n\n", m.remotePath))
		sb.WriteString(errorStyle.Render("File is not writable.") + "\n\n")
		sb.WriteString("  Sudo password: " + m.sudoInput.View())
		if m.permErrMsg != "" {
			sb.WriteString("\n" + errorStyle.Render(m.permErrMsg))
		}
		sb.WriteString("\n\n" + dimStyle.Render("[enter] confirm  [esc] cancel"))

	case sessionPermFixing:
		sb.WriteString(loadingStyle.Render("Setting write permission…") + "\n")

	case sessionPermDenied:
		sb.WriteString(fmt.Sprintf("  File: %s\n\n", m.remotePath))
		sb.WriteString(errorStyle.Render("File is not writable and permissions cannot be changed."))
		sb.WriteString("\n\n" + dimStyle.Render("[enter/esc] back"))

	default:
		sb.WriteString("\n" + dimStyle.Render("[w] upload  [o] re-open editor  [q] discard and exit session"))
	}

	v := tea.NewView(sb.String())
	v.AltScreen = true
	return v
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
