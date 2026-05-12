package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/shayan-shojaei/radar/internal/specsstore"
	"github.com/shayan-shojaei/radar/internal/tui/styles"
)

// ── row types ─────────────────────────────────────────────────────────────────

type pickerRowKind int

const (
	pickerRowHeader pickerRowKind = iota // section header, not selectable
	pickerRowSaved                       // entry from saved specs
	pickerRowRecent                      // entry from recent specs
)

type pickerRow struct {
	kind  pickerRowKind
	label string               // for header rows
	spec  specsstore.SavedSpec // for saved/recent rows
}

// ── model ─────────────────────────────────────────────────────────────────────

// SpecPickerModel is a standalone TUI for selecting a saved or recent spec.
type SpecPickerModel struct {
	savedSpecs  []specsstore.SavedSpec
	recentSpecs []specsstore.SavedSpec
	rows        []pickerRow
	cursor      int // always points to a non-header row
	addMode     bool
	renaming    bool
	savingURL   string // set when "s" is pressed on a recent row
	nameInput   textinput.Model
	urlInput    textinput.Model
	addFocus    int // 0=name, 1=url
	chosen      string
	done        bool
	err         string
	storageDir  string
	width       int
	height      int
}

// NewSpecPickerModel creates a SpecPickerModel from saved and recent spec lists.
func NewSpecPickerModel(saved, recent []specsstore.SavedSpec, storageDir string) SpecPickerModel {
	nameInput := styles.NewStyledTextInput("my-api")
	nameInput.CharLimit = 64
	urlInput := styles.NewStyledTextInput("https://api.example.com/openapi.json")
	urlInput.CharLimit = 512

	m := SpecPickerModel{
		savedSpecs:  saved,
		recentSpecs: recent,
		nameInput:   nameInput,
		urlInput:    urlInput,
		storageDir:  storageDir,
	}
	m.buildRows()
	m.moveCursorTo(0)
	return m
}

// Chosen returns the selected URL, or "" if the user quit without selecting.
func (m SpecPickerModel) Chosen() string { return m.chosen }

// Done reports whether the picker has finished.
func (m SpecPickerModel) Done() bool { return m.done }

// ── row management ────────────────────────────────────────────────────────────

func (m *SpecPickerModel) buildRows() {
	m.rows = m.rows[:0]
	if len(m.savedSpecs) > 0 {
		m.rows = append(m.rows, pickerRow{kind: pickerRowHeader, label: "SAVED SPECS"})
		for _, s := range m.savedSpecs {
			m.rows = append(m.rows, pickerRow{kind: pickerRowSaved, spec: s})
		}
	}
	if len(m.recentSpecs) > 0 {
		m.rows = append(m.rows, pickerRow{kind: pickerRowHeader, label: "RECENT"})
		for _, s := range m.recentSpecs {
			m.rows = append(m.rows, pickerRow{kind: pickerRowRecent, spec: s})
		}
	}
}

// moveCursorTo sets cursor to the given index, skipping header rows.
func (m *SpecPickerModel) moveCursorTo(idx int) {
	for idx < len(m.rows) && m.rows[idx].kind == pickerRowHeader {
		idx++
	}
	if idx >= len(m.rows) {
		idx = m.firstSelectableRow()
	}
	m.cursor = idx
}

func (m SpecPickerModel) firstSelectableRow() int {
	for i, r := range m.rows {
		if r.kind != pickerRowHeader {
			return i
		}
	}
	return 0
}

func (m SpecPickerModel) nextSelectableRow() int {
	for i := m.cursor + 1; i < len(m.rows); i++ {
		if m.rows[i].kind != pickerRowHeader {
			return i
		}
	}
	return m.cursor
}

func (m SpecPickerModel) prevSelectableRow() int {
	for i := m.cursor - 1; i >= 0; i-- {
		if m.rows[i].kind != pickerRowHeader {
			return i
		}
	}
	return m.cursor
}

func (m SpecPickerModel) currentRow() (pickerRow, bool) {
	if m.cursor >= 0 && m.cursor < len(m.rows) {
		return m.rows[m.cursor], true
	}
	return pickerRow{}, false
}

// ── Bubble Tea ────────────────────────────────────────────────────────────────

func (m SpecPickerModel) Init() tea.Cmd { return textinput.Blink }

func (m SpecPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch km := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = km.Width
		m.height = km.Height
		return m, nil
	case tea.KeyMsg:
		if m.addMode {
			return m.handleAddKey(km)
		}
		return m.handleNavKey(km)
	}
	return m, nil
}

func (m SpecPickerModel) handleNavKey(km tea.KeyMsg) (tea.Model, tea.Cmd) {
	row, hasRow := m.currentRow()

	switch km.String() {
	case "q", "esc", "ctrl+c":
		m.done = true
		return m, tea.Quit

	case "up", "k":
		m.cursor = m.prevSelectableRow()

	case "down", "j":
		m.cursor = m.nextSelectableRow()

	case "enter":
		if hasRow {
			m.chosen = row.spec.URL
			m.done = true
			// Touch LastUsed for the chosen spec.
			if row.kind == pickerRowSaved {
				specsstore.Touch(m.storageDir, row.spec.Name) //nolint:errcheck
			} else {
				specsstore.AddRecent(m.storageDir, row.spec.URL) //nolint:errcheck
			}
			return m, tea.Quit
		}

	case "a":
		m.addMode = true
		m.renaming = false
		m.savingURL = ""
		m.addFocus = 0
		m.nameInput.SetValue("")
		m.urlInput.SetValue("")
		m.err = ""
		return m, m.nameInput.Focus()

	case "r":
		if hasRow && row.kind == pickerRowSaved {
			m.addMode = true
			m.renaming = true
			m.savingURL = ""
			m.addFocus = 0
			m.nameInput.SetValue(row.spec.Name)
			m.urlInput.SetValue(row.spec.URL)
			m.err = ""
			return m, m.nameInput.Focus()
		}

	case "s":
		// Save a recent spec by giving it a name.
		if hasRow && row.kind == pickerRowRecent {
			m.addMode = true
			m.renaming = false
			m.savingURL = row.spec.URL
			m.addFocus = 0
			m.nameInput.SetValue("")
			m.urlInput.SetValue(row.spec.URL)
			m.err = ""
			return m, m.nameInput.Focus()
		}

	case "d":
		if !hasRow {
			break
		}
		if row.kind == pickerRowSaved {
			specsstore.Delete(m.storageDir, row.spec.Name) //nolint:errcheck
			specs, _ := specsstore.Load(m.storageDir)
			m.savedSpecs = specs
		} else if row.kind == pickerRowRecent {
			specsstore.DeleteRecent(m.storageDir, row.spec.URL) //nolint:errcheck
			recents, _ := specsstore.LoadRecent(m.storageDir)
			m.recentSpecs = recents
		}
		prev := m.cursor
		m.buildRows()
		m.moveCursorTo(prev)
	}
	return m, nil
}

func (m SpecPickerModel) handleAddKey(km tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch km.String() {
	case "esc":
		m.addMode = false
		m.renaming = false
		m.savingURL = ""
		m.nameInput.Blur()
		m.urlInput.Blur()
		return m, nil

	case "tab", "shift+tab":
		if m.addFocus == 0 {
			m.addFocus = 1
			m.nameInput.Blur()
			return m, m.urlInput.Focus()
		}
		m.addFocus = 0
		m.urlInput.Blur()
		return m, m.nameInput.Focus()

	case "enter":
		if m.addFocus == 0 {
			m.addFocus = 1
			m.nameInput.Blur()
			return m, m.urlInput.Focus()
		}
		name := strings.TrimSpace(m.nameInput.Value())
		url := strings.TrimSpace(m.urlInput.Value())
		if name == "" {
			m.err = "name is required"
			return m, nil
		}
		if url == "" {
			m.err = "URL is required"
			return m, nil
		}
		if m.renaming {
			row, ok := m.currentRow()
			if ok && row.kind == pickerRowSaved && row.spec.Name != name {
				specsstore.Delete(m.storageDir, row.spec.Name) //nolint:errcheck
			}
		}
		if err := specsstore.Add(m.storageDir, name, url); err != nil {
			m.err = err.Error()
			return m, nil
		}
		// Reload both lists.
		saved, _ := specsstore.Load(m.storageDir)
		recents, _ := specsstore.LoadRecent(m.storageDir)
		m.savedSpecs = saved
		m.recentSpecs = recents
		m.buildRows()
		// Move cursor to the newly saved spec.
		for i, r := range m.rows {
			if r.kind == pickerRowSaved && r.spec.Name == name {
				m.cursor = i
				break
			}
		}
		m.addMode = false
		m.renaming = false
		m.savingURL = ""
		m.nameInput.Blur()
		m.urlInput.Blur()
		m.err = ""
		return m, nil
	}

	var cmd tea.Cmd
	if m.addFocus == 0 {
		m.nameInput, cmd = m.nameInput.Update(km)
	} else {
		m.urlInput, cmd = m.urlInput.Update(km)
	}
	return m, cmd
}

// ── rendering ─────────────────────────────────────────────────────────────────

func (m SpecPickerModel) View() string {
	if m.width == 0 {
		return ""
	}

	// Box inner width: fill the terminal generously, cap at 100.
	innerW := m.width - 8
	if innerW < 44 {
		innerW = 44
	}
	if innerW > 100 {
		innerW = 100
	}

	urlW := innerW - 24 // room for cursor + name column
	if urlW < 10 {
		urlW = 10
	}

	var sb strings.Builder

	// ── title ─────────────────────────────────────────────────────────────────
	title := styles.Highlight.Render("radar — Saved Specs")
	titlePad := (innerW - lipgloss.Width(title)) / 2
	if titlePad < 0 {
		titlePad = 0
	}
	sb.WriteString(strings.Repeat(" ", titlePad) + title + "\n\n")

	// ── rows ──────────────────────────────────────────────────────────────────
	if len(m.rows) == 0 && !m.addMode {
		sb.WriteString(styles.Dim.Render("  no saved specs yet — press [a] to add one") + "\n")
	}

	for i, row := range m.rows {
		switch row.kind {
		case pickerRowHeader:
			sb.WriteString("\n" + styles.SectionDivider(row.label, innerW) + "\n")

		case pickerRowSaved, pickerRowRecent:
			selected := i == m.cursor
			cur := "  "
			if selected {
				cur = lipgloss.NewStyle().Foreground(styles.ColorBorderActive).Bold(true).Render("›") + " "
			}
			name := truncStr(row.spec.Name, 18)
			if row.kind == pickerRowRecent && row.spec.Name == row.spec.URL {
				name = "" // unnamed recent: don't repeat URL as name
			}
			url := truncStr(row.spec.URL, urlW)
			var nameSty lipgloss.Style
			if selected {
				nameSty = lipgloss.NewStyle().Foreground(styles.ColorBorderActive).Bold(true)
			} else {
				nameSty = styles.Normal
			}
			line := cur + nameSty.Render(fmt.Sprintf("%-18s", name)) + "  " + styles.Dim.Render(url)
			sb.WriteString(line + "\n")
		}
	}

	// ── add/rename/save form ──────────────────────────────────────────────────
	if m.addMode {
		heading := "Add spec:"
		switch {
		case m.renaming:
			heading = "Edit spec:"
		case m.savingURL != "":
			heading = "Save to list:"
		}
		sb.WriteString("\n" + styles.SectionDivider(heading, innerW) + "\n")
		m.nameInput.Width = innerW / 2
		sb.WriteString("  " + styles.InputLabel.Render(fmt.Sprintf("%-6s", "Name")) + "  " + m.nameInput.View() + "\n")
		if m.savingURL == "" {
			// Show URL field only when adding/editing; for "save recent" the URL is fixed.
			m.urlInput.Width = innerW - 12
			sb.WriteString("  " + styles.InputLabel.Render(fmt.Sprintf("%-6s", "URL")) + "  " + m.urlInput.View() + "\n")
		} else {
			sb.WriteString("  " + styles.Dim.Render("URL: "+truncStr(m.savingURL, innerW-8)) + "\n")
		}
		if m.err != "" {
			sb.WriteString("  " + styles.Dim.Render(m.err) + "\n")
		}
		sb.WriteString("  " + styles.Dim.Render("tab: next field  enter: confirm  esc: cancel") + "\n")
	} else {
		if m.err != "" {
			sb.WriteString("\n  " + styles.Dim.Render(m.err) + "\n")
		}

		// Context-sensitive hints.
		var hints []string
		hints = append(hints, "[↑↓] navigate", "[enter] open", "[a] add")
		row, hasRow := m.currentRow()
		if hasRow {
			if row.kind == pickerRowSaved {
				hints = append(hints, "[r] rename", "[d] delete")
			} else if row.kind == pickerRowRecent {
				hints = append(hints, "[s] save to list", "[d] remove")
			}
		}
		hints = append(hints, "[q] quit")
		sb.WriteString("\n" + styles.Dim.Render("  "+strings.Join(hints, "  ")) + "\n")
	}

	content := sb.String()

	// Wrap in a lipgloss border and center on screen.
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.ColorBorderActive).
		Padding(1, 2).
		Width(innerW).
		Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
