package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/juanperetto/fassht/config"
	"github.com/juanperetto/fassht/fuzzy"
	fasshtssh "github.com/juanperetto/fassht/ssh"
)

var (
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))
	loadingStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

// BrowserModel is the second screen: fuzzy file picker.
type BrowserModel struct {
	host   config.SSHHost
	client *fasshtssh.Client

	allFiles []string
	filtered []string
	cursor   int

	search  textinput.Model
	loading bool
	errMsg  string
}

func NewBrowserModel(client *fasshtssh.Client, host config.SSHHost, appConfig *config.AppConfig) BrowserModel {
	ti := textinput.New()
	ti.Placeholder = "type to filter…"
	ti.Focus()

	return BrowserModel{
		host:    host,
		client:  client,
		search:  ti,
		loading: true,
	}
}

func (m BrowserModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, loadFilesCmd(m.client))
}

func (m BrowserModel) Update(msg tea.Msg) (BrowserModel, tea.Cmd) {
	switch msg := msg.(type) {
	case filesLoadedMsg:
		m.loading = false
		m.allFiles = msg.files
		m.filtered = m.allFiles
		return m, nil

	case filesErrMsg:
		m.loading = false
		m.errMsg = msg.err.Error()
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return BackToConnectionsMsg{} }
		case "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.filtered) > 0 {
				return m, openFileCmd(m.client, m.filtered[m.cursor], m.host)
			}
		default:
			var cmd tea.Cmd
			m.search, cmd = m.search.Update(msg)
			m.filtered = fuzzy.Filter(m.allFiles, m.search.Value())
			m.cursor = 0
			return m, cmd
		}
	}
	return m, nil
}

func (m BrowserModel) View() string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render(fmt.Sprintf("fassht — %s@%s", m.host.User, m.host.Hostname)))
	sb.WriteString("\n\n")
	sb.WriteString(m.search.View())
	sb.WriteString("\n\n")

	if m.loading {
		sb.WriteString(loadingStyle.Render("Loading files…"))
		return sb.String()
	}
	if m.errMsg != "" {
		sb.WriteString(errorStyle.Render(m.errMsg))
		return sb.String()
	}
	if len(m.filtered) == 0 {
		sb.WriteString(dimStyle.Render("No files match."))
		return sb.String()
	}

	// Show up to 20 results
	start := 0
	if m.cursor > 19 {
		start = m.cursor - 19
	}
	end := start + 20
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	for i := start; i < end; i++ {
		prefix := "  "
		line := m.filtered[i]
		if i == m.cursor {
			sb.WriteString(selectedStyle.Render("> "+line) + "\n")
		} else {
			sb.WriteString(prefix + line + "\n")
		}
	}

	sb.WriteString("\n" + dimStyle.Render(fmt.Sprintf("%d/%d  [↑↓] navigate  [enter] open  [esc] back", len(m.filtered), len(m.allFiles))))
	return sb.String()
}

// loadFilesCmd runs `find` on the remote server and returns the file list.
func loadFilesCmd(client *fasshtssh.Client) tea.Cmd {
	return func() tea.Msg {
		entries, err := client.ListFiles("$HOME")
		if err != nil {
			return filesErrMsg{err}
		}
		paths := make([]string, len(entries))
		for i, e := range entries {
			paths[i] = e.Path
		}
		return filesLoadedMsg{files: paths}
	}
}

// openFileCmd downloads the selected file and emits OpenFileMsg.
func openFileCmd(client *fasshtssh.Client, remotePath string, host config.SSHHost) tea.Cmd {
	return func() tea.Msg {
		localPath := fasshtssh.TempFilePath(remotePath)
		if err := client.Download(remotePath, localPath); err != nil {
			return filesErrMsg{err}
		}
		return OpenFileMsg{TempPath: localPath, RemotePath: remotePath, Host: host}
	}
}

type filesLoadedMsg struct{ files []string }
type filesErrMsg struct{ err error }
