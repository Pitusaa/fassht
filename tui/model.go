package tui

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/Pitusaa/fassht/config"
	fasshtssh "github.com/Pitusaa/fassht/ssh"
)

// State represents which screen is active.
type State int

const (
	StateConnections State = iota
	StateBrowser
	StateSession
)

// Model is the top-level Bubbletea model.
type Model struct {
	state       State
	connections ConnectionsModel
	browser     BrowserModel
	session     SessionModel

	sshClient *fasshtssh.Client
	appConfig *config.AppConfig
}

// NewModel creates the initial model.
func NewModel(appCfg *config.AppConfig) Model {
	return Model{
		state:       StateConnections,
		connections: NewConnectionsModel(),
		appConfig:   appCfg,
	}
}

func (m Model) Init() tea.Cmd {
	return m.connections.Init()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

	// Transition: connections → browser
	case ConnectSuccessMsg:
		m.sshClient = msg.Client
		m.state = StateBrowser
		m.browser = NewBrowserModel(msg.Client, msg.Host, m.appConfig)
		return m, m.browser.Init()

	// Transition: browser → session
	case OpenFileMsg:
		m.state = StateSession
		m.session = NewSessionModel(msg.TempPath, msg.RemotePath, msg.Host, m.sshClient, m.appConfig)
		return m, m.session.Init()

	// Transition: session → browser
	case SessionDoneMsg:
		m.state = StateBrowser
		return m, nil

	// Transition: browser → connections (Esc)
	case BackToConnectionsMsg:
		if m.sshClient != nil {
			m.sshClient.Close()
			m.sshClient = nil
		}
		m.state = StateConnections
		m.connections.connecting = false
		m.connections.errMsg = ""
		return m, nil
	}

	// Delegate to active screen
	switch m.state {
	case StateConnections:
		updated, cmd := m.connections.Update(msg)
		m.connections = updated
		return m, cmd
	case StateBrowser:
		updated, cmd := m.browser.Update(msg)
		m.browser = updated
		return m, cmd
	case StateSession:
		updated, cmd := m.session.Update(msg)
		m.session = updated
		return m, cmd
	}

	return m, nil
}

func (m Model) View() string {
	switch m.state {
	case StateConnections:
		return m.connections.View()
	case StateBrowser:
		return m.browser.View()
	case StateSession:
		return m.session.View()
	}
	return ""
}

// --- Messages used for screen transitions ---

// ConnectSuccessMsg is emitted when SSH connection succeeds.
type ConnectSuccessMsg struct {
	Client *fasshtssh.Client
	Host   config.SSHHost
}

// OpenFileMsg is emitted when the user selects a file in the browser.
type OpenFileMsg struct {
	TempPath   string
	RemotePath string
	Host       config.SSHHost
}

// SessionDoneMsg is emitted when the user exits the editing session.
type SessionDoneMsg struct{}

// BackToConnectionsMsg is emitted when the user presses Esc in the browser.
type BackToConnectionsMsg struct{}

// CleanupTempFile deletes the given path, used as a deferred cleanup.
func CleanupTempFile(path string) {
	if path != "" {
		os.Remove(path)
	}
}
