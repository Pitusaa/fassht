package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/Pitusaa/fassht/config"
	fasshtssh "github.com/Pitusaa/fassht/ssh"
)

var (
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))
	loadingStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

type permFlowState int

const (
	permFlowIdle     permFlowState = iota
	permFlowChecking               // running SSH permission check
	permFlowConfirm                // y/N: chmod without sudo or passwordless sudo
	permFlowSudoPass               // password input for sudo chmod
	permFlowFixing                 // running chmod
	permFlowDenied                 // cannot obtain write access
)

// BrowserModel is the second screen: remote file browser.
type BrowserModel struct {
	host   config.SSHHost
	client *fasshtssh.Client

	// Navigation state
	currentPath string
	entries     []fasshtssh.DirEntry
	cursor      int
	history     []string // path stack for "back" navigation

	// UI state
	loading bool
	errMsg  string

	// Search/filter within directory
	filterMode   bool
	filterInput  textinput.Model

	renderStart int
	renderEnd   int

	// Permission-fix flow (preserved from original)
	permState       permFlowState
	permGen         int // incremented per check; stale responses are discarded
	permPath        string
	permInfo        fasshtssh.PermissionInfo
	permReturnState permFlowState // state to restore on chmod error
	sudoInput       textinput.Model
	permErrMsg      string
}

func NewBrowserModel(client *fasshtssh.Client, host config.SSHHost, _ *config.AppConfig) BrowserModel {
	filter := textinput.New()
	filter.Placeholder = "filter…"

	sudo := textinput.New()
	sudo.Placeholder = "sudo password"
	sudo.EchoMode = textinput.EchoPassword
	sudo.EchoCharacter = '•'

	return BrowserModel{
		host:        host,
		client:      client,
		currentPath: ".", // will be resolved to $HOME on first load
		filterInput: filter,
		sudoInput:   sudo,
		renderStart: 0,
		renderEnd:   20,
	}
}

func (m BrowserModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, listDirCmd(m.client, "$HOME"))
}

// --- Message types ---

type permCheckMsg struct {
	gen  int
	path string
	info fasshtssh.PermissionInfo
	err  error
}

type chmodDoneMsg struct{ path string }
type chmodErrMsg struct{ err error }

type dirListMsg struct {
	path    string
	entries []fasshtssh.DirEntry
	err     error
}

// --- Update ---

func (m BrowserModel) Update(msg tea.Msg) (BrowserModel, tea.Cmd) {
	switch msg := msg.(type) {
	case dirListMsg:
		m.loading = false
		if msg.err != nil {
			m.errMsg = msg.err.Error()
		} else {
			m.errMsg = ""
			m.currentPath = msg.path
			m.entries = msg.entries
			m.cursor = 0
			m.renderStart = 0
			m.renderEnd = 20
			if m.renderEnd > len(m.entries) {
				m.renderEnd = len(m.entries)
			}
		}
		return m, nil

	case permCheckMsg:
		if msg.gen != m.permGen {
			return m, nil // stale response
		}
		m.permErrMsg = ""
		if msg.err != nil {
			m.permState = permFlowIdle
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.permInfo = msg.info
		m.permPath = msg.path
		if msg.info.Readable && msg.info.Writable {
			m.permState = permFlowIdle
			return m, openFileCmd(m.client, msg.path, m.host)
		}
		if !msg.info.CanChmod && !msg.info.CanSudo {
			m.permState = permFlowDenied
			return m, nil
		}
		// needs sudo password only when user doesn't own the file AND sudo requires one
		if !msg.info.CanChmod && msg.info.NeedsSudo {
			m.sudoInput.SetValue("")
			m.sudoInput.Focus()
			m.permReturnState = permFlowSudoPass
			m.permState = permFlowSudoPass
			return m, textinput.Blink
		}
		m.permReturnState = permFlowConfirm
		m.permState = permFlowConfirm
		return m, nil

	case chmodDoneMsg:
		m.permState = permFlowIdle
		m.permErrMsg = ""
		return m, openFileCmd(m.client, msg.path, m.host)

	case chmodErrMsg:
		m.permErrMsg = msg.err.Error()
		m.permState = m.permReturnState
		if m.permReturnState == permFlowSudoPass {
			m.sudoInput.SetValue("")
			m.sudoInput.Focus()
			return m, textinput.Blink
		}
		return m, nil

	case tea.KeyPressMsg:
		switch m.permState {
		case permFlowChecking, permFlowFixing:
			return m, nil // ignore input while working

		case permFlowConfirm:
			switch strings.ToLower(msg.String()) {
			case "y":
				m.permState = permFlowFixing
				m.permErrMsg = ""
				if m.permInfo.CanChmod {
					return m, chmodCmd(m.client, m.permPath, false, "")
				}
				return m, chmodCmd(m.client, m.permPath, true, "")
			default:
				m.permState = permFlowIdle
				m.permErrMsg = ""
			}
			return m, nil

		case permFlowSudoPass:
			switch msg.String() {
			case "enter":
				pass := m.sudoInput.Value()
				m.sudoInput.SetValue("")
				m.permState = permFlowFixing
				m.permErrMsg = ""
				return m, chmodCmd(m.client, m.permPath, true, pass)
			case "esc":
				m.permState = permFlowIdle
				m.permErrMsg = ""
				m.sudoInput.SetValue("")
				return m, nil
			default:
				var cmd tea.Cmd
				m.sudoInput, cmd = m.sudoInput.Update(msg)
				return m, cmd
			}

		case permFlowDenied:
			switch msg.String() {
			case "enter", "esc":
				m.permState = permFlowIdle
			}
			return m, nil
		}

		switch msg.String() {
		case "esc":
			if m.filterMode {
				m.filterMode = false
				m.filterInput.SetValue("")
				return m, nil
			}
			return m, func() tea.Msg { return BackToConnectionsMsg{} }
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			if m.cursor < m.renderStart {
				m.renderStart = m.cursor
				m.renderEnd = m.renderStart + 20
				if m.renderEnd > len(m.entries) {
					m.renderEnd = len(m.entries)
				}
			}
		case "down", "j":
			if m.cursor < len(m.entries)-1 {
				m.cursor++
			}
			if m.cursor >= m.renderEnd {
				m.renderEnd = m.cursor + 1
				m.renderStart = m.renderEnd - 20
				if m.renderStart < 0 {
					m.renderStart = 0
				}
			}
		case "enter":
			if len(m.entries) == 0 {
				return m, nil
			}
			selected := m.entries[m.cursor]
			if selected.IsDir {
				m.history = append(m.history, m.currentPath)
				m.loading = true
				return m, listDirCmd(m.client, selected.Path)
			}
			m.permGen++
			m.permState = permFlowChecking
			m.permErrMsg = ""
			return m, permCheckCmd(m.client, selected.Path, m.permGen)
		case "backspace", "h":
			if len(m.history) > 0 {
				prev := m.history[len(m.history)-1]
				m.history = m.history[:len(m.history)-1]
				m.loading = true
				return m, listDirCmd(m.client, prev)
			}
			return m, func() tea.Msg { return BackToConnectionsMsg{} }
		case "r":
			m.loading = true
			return m, listDirCmd(m.client, m.currentPath)
		case "home", "g":
			m.cursor = 0
			m.renderStart = 0
			m.renderEnd = 20
			if m.renderEnd > len(m.entries) {
				m.renderEnd = len(m.entries)
			}
		case "end", "G":
			if len(m.entries) > 0 {
				m.cursor = len(m.entries) - 1
			}
			if m.cursor >= m.renderEnd {
				m.renderEnd = m.cursor + 1
				m.renderStart = m.renderEnd - 20
				if m.renderStart < 0 {
					m.renderStart = 0
				}
			}
		case "/":
			m.filterMode = true
			m.filterInput.SetValue("")
			m.filterInput.Focus()
			return m, textinput.Blink
		default:
			if m.filterMode {
				var cmd tea.Cmd
				m.filterInput, cmd = m.filterInput.Update(msg)
				return m, cmd
			}
		}
	}
	return m, nil
}

// --- View ---

func (m BrowserModel) View() tea.View {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render(fmt.Sprintf("fassht — %s@%s", m.host.User, m.host.Hostname)))
	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")).Render(m.currentPath))
	sb.WriteString("\n\n")

	done := true
	switch m.permState {
	case permFlowChecking:
		sb.WriteString(loadingStyle.Render("Checking permissions…"))

	case permFlowConfirm:
		verb := "chmod u+w"
		if !m.permInfo.CanChmod {
			verb = "sudo chmod u+w"
		}
		sb.WriteString(fmt.Sprintf("  File: %s\n\n", m.permPath))
		sb.WriteString(errorStyle.Render("File is not writable."))
		sb.WriteString(fmt.Sprintf(" Run %s to fix it? [y/N] ", verb))
		if m.permErrMsg != "" {
			sb.WriteString("\n" + errorStyle.Render(m.permErrMsg))
		}

	case permFlowSudoPass:
		sb.WriteString(fmt.Sprintf("  File: %s\n\n", m.permPath))
		sb.WriteString(errorStyle.Render("File is not writable.") + "\n\n")
		sb.WriteString("  Sudo password: " + m.sudoInput.View())
		if m.permErrMsg != "" {
			sb.WriteString("\n" + errorStyle.Render(m.permErrMsg))
		}
		sb.WriteString("\n\n" + dimStyle.Render("[enter] confirm  [esc] cancel"))

	case permFlowFixing:
		sb.WriteString(loadingStyle.Render("Setting write permission…"))

	case permFlowDenied:
		sb.WriteString(fmt.Sprintf("  File: %s\n\n", m.permPath))
		sb.WriteString(errorStyle.Render("File is not writable and permissions cannot be changed."))
		sb.WriteString("\n\n" + dimStyle.Render("[enter/esc] back"))

	default:
		done = false
	}

	if !done {
		if m.loading {
			sb.WriteString(loadingStyle.Render("Loading…"))
		} else if m.errMsg != "" {
			sb.WriteString(errorStyle.Render(m.errMsg))
		} else if len(m.entries) == 0 {
			sb.WriteString(dimStyle.Render("Empty directory"))
		} else {
			for i := m.renderStart; i < m.renderEnd && i < len(m.entries); i++ {
				entry := m.entries[i]
				if m.filterMode {
					filter := strings.ToLower(m.filterInput.Value())
					if filter != "" && !strings.Contains(strings.ToLower(entry.Name), filter) {
						continue
					}
				}

				prefix := "  "
				if i == m.cursor {
					prefix = selectedStyle.Render("> ")
				}

				name := entry.Name
				if entry.IsDir {
					name = name + "/"
				}

				if i == m.cursor {
					sb.WriteString(prefix + selectedStyle.Render(name) + "\n")
				} else {
					sb.WriteString(prefix + name + "\n")
				}
			}
			if len(m.entries) > m.renderEnd {
				sb.WriteString(dimStyle.Render(fmt.Sprintf("  … %d more items", len(m.entries)-m.renderEnd)))
			}
		}

		sb.WriteString("\n")
		if m.filterMode {
			sb.WriteString(m.filterInput.View())
			sb.WriteString("\n" + dimStyle.Render("[esc] clear filter"))
		} else {
			sb.WriteString(dimStyle.Render("[↑↓] navigate  [enter] open  [h] back  [r] refresh  [/] filter  [esc] disconnect"))
		}
	}

	v := tea.NewView(sb.String())
	v.AltScreen = true
	return v
}

// --- Commands ---

func listDirCmd(client *fasshtssh.Client, path string) tea.Cmd {
	return func() tea.Msg {
		if path == "$HOME" {
			home, err := client.HomeDir()
			if err != nil {
				return dirListMsg{path: path, err: err}
			}
			path = home
		}
		entries, err := client.ListDir(path)
		return dirListMsg{path: path, entries: entries, err: err}
	}
}

func permCheckCmd(client *fasshtssh.Client, path string, gen int) tea.Cmd {
	return func() tea.Msg {
		info, err := client.CheckWritePermission(path)
		return permCheckMsg{gen: gen, path: path, info: info, err: err}
	}
}

func chmodCmd(client *fasshtssh.Client, path string, useSudo bool, password string) tea.Cmd {
	return func() tea.Msg {
		var err error
		if useSudo {
			err = client.SudoChmodWritable(path, password)
		} else {
			err = client.ChmodWritable(path)
		}
		if err != nil {
			return chmodErrMsg{err}
		}
		return chmodDoneMsg{path: path}
	}
}

func openFileCmd(client *fasshtssh.Client, remotePath string, host config.SSHHost) tea.Cmd {
	return func() tea.Msg {
		localPath := fasshtssh.TempFilePath(remotePath)
		if err := client.Download(remotePath, localPath); err != nil {
			return filesErrMsg{err}
		}
		return OpenFileMsg{TempPath: localPath, RemotePath: remotePath, Host: host}
	}
}

type filesErrMsg struct{ err error }
