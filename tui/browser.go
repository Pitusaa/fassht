package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/Pitusaa/fassht/config"
	fasshtssh "github.com/Pitusaa/fassht/ssh"
)

var (
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))
	loadingStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

// BrowserModel is the second screen: server-side fuzzy file picker.
type BrowserModel struct {
	host   config.SSHHost
	client *fasshtssh.Client

	results    []string
	cursor     int
	searching  bool
	generation int // incremented on each new search; stale results are discarded
	errMsg     string

	search textinput.Model
}

func NewBrowserModel(client *fasshtssh.Client, host config.SSHHost, appConfig *config.AppConfig) BrowserModel {
	ti := textinput.New()
	ti.Placeholder = "type to search files on server…"
	ti.Focus()

	return BrowserModel{
		host:   host,
		client: client,
		search: ti,
	}
}

func (m BrowserModel) Init() tea.Cmd {
	return textinput.Blink
}

// searchResultMsg carries results back from a server search.
type searchResultMsg struct {
	generation int
	files      []string
	err        error
}

func (m BrowserModel) Update(msg tea.Msg) (BrowserModel, tea.Cmd) {
	switch msg := msg.(type) {
	case searchResultMsg:
		if msg.generation != m.generation {
			// stale result from a superseded search — discard
			return m, nil
		}
		m.searching = false
		if msg.err != nil {
			m.errMsg = msg.err.Error()
		} else {
			m.errMsg = ""
			m.results = msg.files
			m.cursor = 0
		}
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
			if m.cursor < len(m.results)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.results) > 0 {
				return m, openFileCmd(m.client, m.results[m.cursor], m.host)
			}
		default:
			var cmd tea.Cmd
			m.search, cmd = m.search.Update(msg)
			query := m.search.Value()
			if len([]rune(query)) < 2 {
				m.results = nil
				m.searching = false
				return m, cmd
			}
			m.generation++
			m.searching = true
			return m, tea.Batch(cmd, searchFilesCmd(m.client, query, m.generation))
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

	switch {
	case m.errMsg != "":
		sb.WriteString(errorStyle.Render(m.errMsg))
	case len(m.search.Value()) < 2:
		sb.WriteString(dimStyle.Render("Start typing to search files…"))
	case m.searching:
		sb.WriteString(loadingStyle.Render("Searching…"))
	case len(m.results) == 0:
		sb.WriteString(dimStyle.Render("No files match."))
	default:
		start := 0
		if m.cursor > 19 {
			start = m.cursor - 19
		}
		end := start + 20
		if end > len(m.results) {
			end = len(m.results)
		}
		for i := start; i < end; i++ {
			if i == m.cursor {
				sb.WriteString(selectedStyle.Render("> "+m.results[i]) + "\n")
			} else {
				sb.WriteString("  " + m.results[i] + "\n")
			}
		}
		total := len(m.results)
		label := fmt.Sprintf("%d results", total)
		if total == 200 {
			label = "200+ results (refine query)"
		}
		sb.WriteString("\n" + dimStyle.Render(label+"  [↑↓] navigate  [enter] open  [esc] back"))
	}
	return sb.String()
}

// searchFilesCmd runs a server-side find and returns results tagged with gen.
func searchFilesCmd(client *fasshtssh.Client, query string, gen int) tea.Cmd {
	return func() tea.Msg {
		entries, err := client.SearchFiles("$HOME", query)
		if err != nil {
			return searchResultMsg{generation: gen, err: err}
		}
		paths := make([]string, len(entries))
		for i, e := range entries {
			paths[i] = e.Path
		}
		return searchResultMsg{generation: gen, files: paths}
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

type filesErrMsg struct{ err error }
