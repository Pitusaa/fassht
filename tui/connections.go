package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/juanperetto/fassht/config"
	fasshtssh "github.com/juanperetto/fassht/ssh"
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

// hostItem wraps config.SSHHost to satisfy bubbles/list.Item.
type hostItem struct{ host config.SSHHost }

func (h hostItem) Title() string { return h.host.Name }
func (h hostItem) Description() string {
	return fmt.Sprintf("%s@%s:%s", h.host.User, h.host.Hostname, h.host.Port)
}
func (h hostItem) FilterValue() string { return h.host.Name }

// addFormField tracks which input is focused in the add-connection form.
type addFormField int

const (
	fieldName addFormField = iota
	fieldHostname
	fieldUser
	fieldPort
	fieldIdentityFile
	fieldCount
)

// ConnectionsModel is the first screen: list of SSH connections.
type ConnectionsModel struct {
	list       list.Model
	adding     bool
	inputs     [fieldCount]textinput.Model
	focus      addFormField
	errMsg     string
	connecting bool
}

func NewConnectionsModel() ConnectionsModel {
	hosts, _ := config.LoadSSHHosts()
	items := make([]list.Item, len(hosts))
	for i, h := range hosts {
		items[i] = hostItem{h}
	}

	delegate := list.NewDefaultDelegate()
	l := list.New(items, delegate, 0, 0)
	l.Title = "fassht — connections"
	l.SetShowHelp(true)

	inputs := [fieldCount]textinput.Model{}
	labels := []string{"Name", "Hostname", "User", "Port (default 22)", "Identity file (optional)"}
	for i := range inputs {
		ti := textinput.New()
		ti.Placeholder = labels[i]
		inputs[i] = ti
	}
	inputs[fieldName].Focus()

	return ConnectionsModel{list: l, inputs: inputs}
}

func (m ConnectionsModel) Init() tea.Cmd { return nil }

func (m ConnectionsModel) Update(msg tea.Msg) (ConnectionsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height-4)

	case connectErrMsg:
		m.connecting = false
		m.errMsg = msg.err.Error()
		return m, nil

	case tea.KeyMsg:
		if m.adding {
			return m.updateAddForm(msg)
		}
		switch msg.String() {
		case "a":
			m.adding = true
			m.errMsg = ""
			m.inputs[fieldName].Focus()
			return m, textinput.Blink
		case "enter":
			if item, ok := m.list.SelectedItem().(hostItem); ok {
				m.connecting = true
				m.errMsg = ""
				return m, connectCmd(item.host)
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m ConnectionsModel) updateAddForm(msg tea.KeyMsg) (ConnectionsModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.adding = false
		for i := range m.inputs {
			m.inputs[i].SetValue("")
		}
		m.focus = fieldName
		return m, nil
	case "tab", "down":
		m.inputs[m.focus].Blur()
		m.focus = (m.focus + 1) % fieldCount
		m.inputs[m.focus].Focus()
		return m, textinput.Blink
	case "shift+tab", "up":
		m.inputs[m.focus].Blur()
		if m.focus == 0 {
			m.focus = fieldCount - 1
		} else {
			m.focus--
		}
		m.inputs[m.focus].Focus()
		return m, textinput.Blink
	case "enter":
		if m.focus < fieldCount-1 {
			m.inputs[m.focus].Blur()
			m.focus++
			m.inputs[m.focus].Focus()
			return m, textinput.Blink
		}
		return m.saveNewHost()
	}

	var cmd tea.Cmd
	m.inputs[m.focus], cmd = m.inputs[m.focus].Update(msg)
	return m, cmd
}

func (m ConnectionsModel) saveNewHost() (ConnectionsModel, tea.Cmd) {
	name := m.inputs[fieldName].Value()
	hostname := m.inputs[fieldHostname].Value()
	if name == "" || hostname == "" {
		m.errMsg = "Name and Hostname are required"
		return m, nil
	}
	port := m.inputs[fieldPort].Value()
	if port == "" {
		port = "22"
	}
	host := config.SSHHost{
		Name:         name,
		Hostname:     hostname,
		User:         m.inputs[fieldUser].Value(),
		Port:         port,
		IdentityFile: m.inputs[fieldIdentityFile].Value(),
	}
	if err := config.AppendSSHHost(host); err != nil {
		m.errMsg = err.Error()
		return m, nil
	}
	// Refresh list
	m.list.InsertItem(len(m.list.Items()), hostItem{host})
	m.adding = false
	// Reset form
	for i := range m.inputs {
		m.inputs[i].SetValue("")
	}
	m.focus = fieldName
	return m, nil
}

func (m ConnectionsModel) View() string {
	if m.adding {
		return m.viewAddForm()
	}
	var sb strings.Builder
	sb.WriteString(m.list.View())
	if m.errMsg != "" {
		sb.WriteString("\n" + errorStyle.Render(m.errMsg))
	}
	if m.connecting {
		sb.WriteString("\n" + dimStyle.Render("Connecting…"))
	} else {
		sb.WriteString("\n" + dimStyle.Render("[a] add connection  [enter] connect  [q] quit"))
	}
	return sb.String()
}

func (m ConnectionsModel) viewAddForm() string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Add connection") + "\n\n")
	labels := []string{"Name", "Hostname", "User", "Port", "Identity file"}
	for i, inp := range m.inputs {
		prefix := "  "
		if addFormField(i) == m.focus {
			prefix = "> "
		}
		sb.WriteString(fmt.Sprintf("%s%s: %s\n", prefix, labels[i], inp.View()))
	}
	if m.errMsg != "" {
		sb.WriteString("\n" + errorStyle.Render(m.errMsg))
	}
	sb.WriteString("\n" + dimStyle.Render("[tab] next  [shift+tab] prev  [enter] save  [esc] cancel"))
	return sb.String()
}

// connectCmd is a Bubbletea command that dials SSH asynchronously.
func connectCmd(host config.SSHHost) tea.Cmd {
	return func() tea.Msg {
		client, err := fasshtssh.Connect(host)
		if err != nil {
			return connectErrMsg{err}
		}
		return ConnectSuccessMsg{Client: client, Host: host}
	}
}

type connectErrMsg struct{ err error }
