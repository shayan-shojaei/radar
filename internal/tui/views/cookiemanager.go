package views

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	tuimsg "github.com/shayan-shojaei/radar/internal/tui/msg"
	"github.com/shayan-shojaei/radar/internal/tui/styles"
	"github.com/shayan-shojaei/radar/pkg/models"
)

type cookieRow struct {
	name  string
	entry models.CookieEntry
}

// CookieManagerModel manages the cookie jar and global auth header.
type CookieManagerModel struct {
	authInput   textinput.Model
	authFocused bool
	cookies     []cookieRow
	cursor      int
	addMode     bool
	addKey      textinput.Model
	addVal      textinput.Model
	addFocus    int // 0=key, 1=val
	width       int
	height      int
}

// NewCookieManagerModel creates a CookieManagerModel from the given jar and auth header.
func NewCookieManagerModel(jar map[string]models.CookieEntry, authHeader string) CookieManagerModel {
	authInput := styles.NewStyledTextInput("Bearer …")
	authInput.CharLimit = 512
	authInput.SetValue(authHeader)

	addKey := styles.NewStyledTextInput("name")
	addKey.CharLimit = 256
	addVal := styles.NewStyledTextInput("value")
	addVal.CharLimit = 512

	var rows []cookieRow
	for name, entry := range jar {
		rows = append(rows, cookieRow{name: name, entry: entry})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })

	return CookieManagerModel{
		authInput: authInput,
		cookies:   rows,
		addKey:    addKey,
		addVal:    addVal,
	}
}

// Resize sets panel dimensions.
func (m *CookieManagerModel) Resize(w, h int) {
	m.width = w
	m.height = h
}

// toJar converts cookie rows back to a map.
func (m CookieManagerModel) toJar() map[string]models.CookieEntry {
	jar := make(map[string]models.CookieEntry, len(m.cookies))
	for _, c := range m.cookies {
		jar[c.name] = c.entry
	}
	return jar
}

// Init implements tea.Model.
func (m CookieManagerModel) Init() tea.Cmd { return textinput.Blink }

// Update implements tea.Model.
func (m CookieManagerModel) Update(msg tea.Msg) (CookieManagerModel, tea.Cmd) {
	switch km := msg.(type) {
	case tea.KeyMsg:
		if m.addMode {
			return m.handleAddKey(km)
		}
		if m.authFocused {
			return m.handleAuthKey(km)
		}
		return m.handleNavKey(km)
	}
	return m, nil
}

func (m CookieManagerModel) handleAuthKey(km tea.KeyMsg) (CookieManagerModel, tea.Cmd) {
	switch km.String() {
	case "esc", "enter", "tab":
		m.authFocused = false
		m.authInput.Blur()
		return m, nil
	}
	var cmd tea.Cmd
	m.authInput, cmd = m.authInput.Update(km)
	return m, cmd
}

func (m CookieManagerModel) handleNavKey(km tea.KeyMsg) (CookieManagerModel, tea.Cmd) {
	switch km.String() {
	case "esc":
		jar := m.toJar()
		auth := m.authInput.Value()
		return m, func() tea.Msg {
			return tuimsg.CookieJarUpdatedMsg{Jar: jar, AuthHeader: auth}
		}
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.cookies)-1 {
			m.cursor++
		}
	case "e", "enter":
		m.authFocused = true
		return m, m.authInput.Focus()
	case " ":
		if m.cursor < len(m.cookies) {
			m.cookies[m.cursor].entry.Enabled = !m.cookies[m.cursor].entry.Enabled
		}
	case "d":
		if m.cursor < len(m.cookies) {
			m.cookies = append(m.cookies[:m.cursor], m.cookies[m.cursor+1:]...)
			if m.cursor >= len(m.cookies) && m.cursor > 0 {
				m.cursor--
			}
		}
	case "a":
		m.addMode = true
		m.addFocus = 0
		m.addKey.SetValue("")
		m.addVal.SetValue("")
		return m, m.addKey.Focus()
	}
	return m, nil
}

func (m CookieManagerModel) handleAddKey(km tea.KeyMsg) (CookieManagerModel, tea.Cmd) {
	switch km.String() {
	case "esc":
		m.addMode = false
		m.addKey.Blur()
		m.addVal.Blur()
		return m, nil
	case "tab", "shift+tab":
		if m.addFocus == 0 {
			m.addFocus = 1
			m.addKey.Blur()
			return m, m.addVal.Focus()
		}
		m.addFocus = 0
		m.addVal.Blur()
		return m, m.addKey.Focus()
	case "enter":
		if m.addFocus == 0 {
			m.addFocus = 1
			m.addKey.Blur()
			return m, m.addVal.Focus()
		}
		key := strings.TrimSpace(m.addKey.Value())
		val := m.addVal.Value()
		if key != "" {
			found := false
			for i, c := range m.cookies {
				if c.name == key {
					m.cookies[i].entry.Value = val
					m.cookies[i].entry.Enabled = true
					found = true
					break
				}
			}
			if !found {
				m.cookies = append(m.cookies, cookieRow{
					name:  key,
					entry: models.CookieEntry{Value: val, Enabled: true},
				})
				sort.Slice(m.cookies, func(i, j int) bool { return m.cookies[i].name < m.cookies[j].name })
			}
		}
		m.addMode = false
		m.addKey.Blur()
		m.addVal.Blur()
		return m, nil
	}
	var cmd tea.Cmd
	if m.addFocus == 0 {
		m.addKey, cmd = m.addKey.Update(km)
	} else {
		m.addVal, cmd = m.addVal.Update(km)
	}
	return m, cmd
}

// View renders the cookie manager panel content.
func (m CookieManagerModel) View() string {
	w := m.width
	if w < 20 {
		w = 60
	}
	var sb strings.Builder

	// Auth header section.
	sb.WriteString(styles.SectionDivider("AUTH HEADER", w))
	sb.WriteByte('\n')
	label := styles.InputLabel.Render(fmt.Sprintf("%-16s", "Authorization")) + "  "
	authW := w - 20
	if authW < 10 {
		authW = 10
	}
	m.authInput.Width = authW
	if m.authFocused {
		m.authInput.PromptStyle = lipgloss.NewStyle().Foreground(styles.ColorBorderActive)
	} else {
		m.authInput.PromptStyle = lipgloss.NewStyle().Foreground(styles.ColorDimText)
	}
	sb.WriteString(label + m.authInput.View())
	sb.WriteByte('\n')
	if !m.authFocused {
		sb.WriteString(styles.Dim.Render("  e/enter: edit auth header"))
		sb.WriteByte('\n')
	}
	sb.WriteByte('\n')

	// Cookie jar section.
	sb.WriteString(styles.SectionDivider("COOKIE JAR", w))
	sb.WriteByte('\n')

	if len(m.cookies) == 0 {
		sb.WriteString(styles.Dim.Render("  no cookies"))
		sb.WriteByte('\n')
	} else {
		for i, c := range m.cookies {
			selected := i == m.cursor
			cur := "  "
			if selected {
				cur = lipgloss.NewStyle().Foreground(styles.ColorBorderActive).Bold(true).Render("›") + " "
			}
			check := "✗"
			checkSty := styles.Dim
			if c.entry.Enabled {
				check = "✓"
				checkSty = lipgloss.NewStyle().Foreground(styles.ColorMethodGET)
			}
			name := truncStr(c.name, 20)
			valW := w - 32
			if valW < 8 {
				valW = 8
			}
			val := truncStr(c.entry.Value, valW)
			hint := ""
			if c.entry.HTTPOnly {
				hint = styles.Dim.Render(" (httpOnly)")
			}
			line := cur + checkSty.Render("["+check+"] ") +
				styles.Normal.Render(fmt.Sprintf("%-20s", name)) + "  " +
				styles.Dim.Render(val) + hint
			sb.WriteString(line)
			sb.WriteByte('\n')
		}
	}

	if m.addMode {
		sb.WriteByte('\n')
		sb.WriteString(styles.SectionDivider("ADD COOKIE", w))
		sb.WriteByte('\n')
		m.addKey.Width = 20
		valW := w - 20
		if valW < 10 {
			valW = 10
		}
		m.addVal.Width = valW
		sb.WriteString(styles.InputLabel.Render(fmt.Sprintf("%-8s", "Name")) + "  " + m.addKey.View())
		sb.WriteByte('\n')
		sb.WriteString(styles.InputLabel.Render(fmt.Sprintf("%-8s", "Value")) + "  " + m.addVal.View())
		sb.WriteByte('\n')
		sb.WriteString(styles.Dim.Render("  tab: next field  enter: confirm  esc: cancel"))
		sb.WriteByte('\n')
	} else {
		sb.WriteByte('\n')
		sb.WriteString(styles.Dim.Render("  space: toggle  d: delete  a: add cookie  esc: save & back"))
		sb.WriteByte('\n')
	}

	return sb.String()
}
