package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	tuimsg "github.com/shayan-shojaei/radar/internal/tui/msg"
	"github.com/shayan-shojaei/radar/internal/tui/styles"
	"github.com/shayan-shojaei/radar/pkg/models"
)

// ── data structures ────────────────────────────────────────────────────────────

type tagGroup struct {
	name      string
	endpoints []models.ParsedEndpoint
	collapsed bool
}

type listRowKind int

const (
	rowKindTag listRowKind = iota
	rowKindEndpoint
)

type listRow struct {
	kind   listRowKind
	tagIdx int
	ep     models.ParsedEndpoint
}

type summaryMode int

const (
	summaryNone    summaryMode = iota // path only, 1 line
	summarySummary                    // path + summary, 2 lines
	summaryBoth                       // path + summary + description, 3 lines
)

// ListModel is the viewport-backed endpoint list panel content.
type ListModel struct {
	groups   []tagGroup
	untagged []models.ParsedEndpoint
	total    int
	rows     []listRow
	cursor   int
	summary  summaryMode
	filter   textinput.Model
	vp       viewport.Model
	width    int
	height   int
	ready    bool
}

// NewListModel constructs a ListModel from parsed endpoints.
func NewListModel(endpoints []models.ParsedEndpoint) ListModel {
	groupMap := make(map[string]*tagGroup)
	var tagOrder []string
	var untagged []models.ParsedEndpoint

	for _, ep := range endpoints {
		if len(ep.Tags) > 0 {
			tag := ep.Tags[0]
			if _, exists := groupMap[tag]; !exists {
				groupMap[tag] = &tagGroup{name: tag}
				tagOrder = append(tagOrder, tag)
			}
			groupMap[tag].endpoints = append(groupMap[tag].endpoints, ep)
		} else {
			untagged = append(untagged, ep)
		}
	}

	groups := make([]tagGroup, 0, len(tagOrder))
	for _, t := range tagOrder {
		groups = append(groups, *groupMap[t])
	}

	fi := styles.NewStyledTextInput("filter by method, path, summary…")
	fi.Prompt = " / "

	m := ListModel{
		groups:   groups,
		untagged: untagged,
		total:    len(endpoints),
		filter:   fi,
		summary:  summarySummary,
	}
	m.buildRows()
	return m
}

// ApplyPrefs restores summary mode and collapsed tag state from saved preferences.
func (m *ListModel) ApplyPrefs(summaryModeN int, collapsedTags []string) {
	if summaryModeN >= 0 && summaryModeN <= 2 {
		m.summary = summaryMode(summaryModeN)
	}
	collSet := make(map[string]bool, len(collapsedTags))
	for _, t := range collapsedTags {
		collSet[t] = true
	}
	for i := range m.groups {
		m.groups[i].collapsed = collSet[m.groups[i].name]
	}
	m.buildRows()
}

// GetSummaryMode returns the current summary mode as an int for serialization.
func (m ListModel) GetSummaryMode() int {
	return int(m.summary)
}

// GetCollapsedTagNames returns the names of all currently collapsed tag groups.
func (m ListModel) GetCollapsedTagNames() []string {
	var names []string
	for _, g := range m.groups {
		if g.collapsed {
			names = append(names, g.name)
		}
	}
	return names
}

// Resize is called by the app when the inner panel dimensions change.
func (m *ListModel) Resize(innerW, innerH int) {
	m.width = innerW
	// Reserve 3 lines for filter input + separator + count line.
	vpH := innerH - 3
	if vpH < 1 {
		vpH = 1
	}
	m.vp = viewport.New(innerW, vpH)
	m.ready = true
	m.vp.SetContent(m.renderRows())
	m.scrollToCursor()
}

// ── row building ───────────────────────────────────────────────────────────────

func (m *ListModel) buildRows() {
	m.rows = m.rows[:0]
	q := strings.ToLower(m.filter.Value())

	if q != "" {
		for gi, g := range m.groups {
			for _, ep := range g.endpoints {
				if endpointMatches(ep, q) {
					m.rows = append(m.rows, listRow{kind: rowKindEndpoint, tagIdx: gi, ep: ep})
				}
			}
		}
		for _, ep := range m.untagged {
			if endpointMatches(ep, q) {
				m.rows = append(m.rows, listRow{kind: rowKindEndpoint, tagIdx: -1, ep: ep})
			}
		}
		return
	}

	for i, g := range m.groups {
		m.rows = append(m.rows, listRow{kind: rowKindTag, tagIdx: i})
		if !g.collapsed {
			for _, ep := range g.endpoints {
				m.rows = append(m.rows, listRow{kind: rowKindEndpoint, tagIdx: i, ep: ep})
			}
		}
	}
	for _, ep := range m.untagged {
		m.rows = append(m.rows, listRow{kind: rowKindEndpoint, tagIdx: -1, ep: ep})
	}
}

func endpointMatches(ep models.ParsedEndpoint, q string) bool {
	return strings.Contains(strings.ToLower(ep.Method), q) ||
		strings.Contains(strings.ToLower(ep.Path), q) ||
		strings.Contains(strings.ToLower(ep.Summary), q) ||
		strings.Contains(strings.ToLower(ep.Description), q)
}

// ── cursor + scroll helpers ───────────────────────────────────────────────────

func (m *ListModel) clampCursor() {
	if len(m.rows) == 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *ListModel) rowLineCount(row listRow) int {
	if row.kind == rowKindTag {
		return 1
	}
	switch m.summary {
	case summaryNone:
		return 1
	case summarySummary:
		return 2
	case summaryBoth:
		return 3
	}
	return 1
}

func (m *ListModel) lineOffset(rowIdx int) int {
	offset := 0
	for i := range rowIdx {
		if i < len(m.rows) {
			offset += m.rowLineCount(m.rows[i])
		}
	}
	return offset
}

func (m *ListModel) scrollToCursor() {
	if m.vp.Height == 0 || len(m.rows) == 0 {
		return
	}
	cursorLine := m.lineOffset(m.cursor)
	cursorLines := m.rowLineCount(m.rows[m.cursor])
	if cursorLine < m.vp.YOffset {
		m.vp.SetYOffset(cursorLine)
	} else if cursorLine+cursorLines > m.vp.YOffset+m.vp.Height {
		m.vp.SetYOffset(cursorLine + cursorLines - m.vp.Height)
	}
}

func (m *ListModel) refresh() {
	m.buildRows()
	m.clampCursor()
	if m.ready {
		m.vp.SetContent(m.renderRows())
		m.scrollToCursor()
	}
}

func (m *ListModel) redraw() {
	if m.ready {
		m.vp.SetContent(m.renderRows())
		m.scrollToCursor()
	}
}

// ── rendering ─────────────────────────────────────────────────────────────────

func (m *ListModel) renderRows() string {
	w := m.width
	if w < 20 {
		w = 40
	}
	if len(m.rows) == 0 {
		return styles.Dim.Render("  no endpoints match")
	}
	var sb strings.Builder
	for i, row := range m.rows {
		sb.WriteString(m.renderRow(row, i == m.cursor, w))
		sb.WriteByte('\n')
	}
	return sb.String()
}

func (m *ListModel) renderRow(row listRow, selected bool, w int) string {
	if row.kind == rowKindTag {
		return m.renderTagRow(row.tagIdx, selected, w)
	}
	return m.renderEndpointRow(row.ep, selected, w)
}

// renderTagRow: [cursor] [▼/▶] [TAG NAME] [──fill──] [count]
func (m *ListModel) renderTagRow(tagIdx int, selected bool, w int) string {
	g := m.groups[tagIdx]

	cur := "  "
	if selected {
		cur = lipgloss.NewStyle().Foreground(styles.ColorBorderActive).Bold(true).Render("›") + " "
	}

	arrow := "▼ "
	var nameSty lipgloss.Style
	if g.collapsed {
		arrow = "▶ "
		nameSty = styles.Dim
	} else {
		nameSty = styles.Highlight
	}
	if selected {
		nameSty = lipgloss.NewStyle().Foreground(styles.ColorBorderActive).Bold(true)
	}

	name := strings.ToUpper(g.name)
	countStr := fmt.Sprintf(" %d", len(g.endpoints))

	// Visual width accounting: cur=2, arrow=2, countStr, right margin=1
	fixedW := 2 + 2 + lipgloss.Width(countStr) + 1
	nameAvail := w - fixedW
	if nameAvail < 1 {
		nameAvail = 1
	}
	runes := []rune(name)
	if len(runes) > nameAvail {
		name = string(runes[:nameAvail])
	}

	fillW := nameAvail - len([]rune(name))
	if fillW < 0 {
		fillW = 0
	}
	fill := styles.SectionRule.Render(strings.Repeat("─", fillW))

	return cur + nameSty.Render(arrow+name) + fill + styles.Dim.Render(countStr)
}

// renderEndpointRow: line 1: [cursor] [METHOD] [full-width path]
//
//	line 2+: indented summary / description (per summary mode)
func (m *ListModel) renderEndpointRow(ep models.ParsedEndpoint, selected bool, w int) string {
	cur := "    "
	if selected {
		cur = "  " + lipgloss.NewStyle().Foreground(styles.ColorBorderActive).Bold(true).Render("›") + " "
	}

	method := styles.MethodBadge(ep.Method)

	// Path gets full remaining width (summary moves to its own line).
	remaining := w - 4 - 6 - 2
	if remaining < 12 {
		remaining = 12
	}

	path := truncStr(ep.Path, remaining)
	paddedPath := fmt.Sprintf("%-*s", remaining, path)

	var pathRender string
	if selected {
		pathRender = styles.ListPathFocused.Render(paddedPath)
	} else {
		pathRender = styles.ListPath.Render(paddedPath)
	}

	line1 := cur + method + "  " + pathRender

	if m.summary == summaryNone {
		return line1
	}

	// Lines 2+ are indented to align under the path text (12 chars: 4+6+2).
	const pathIndent = "            "
	subW := w - len(pathIndent)
	if subW < 0 {
		subW = 0
	}

	line2 := pathIndent + styles.ListSummary.Render(truncStr(ep.Summary, subW))

	if m.summary == summarySummary {
		return line1 + "\n" + line2
	}

	// summaryBoth: add description line.
	line3 := pathIndent + styles.Dim.Render(truncStr(ep.Description, subW))
	return line1 + "\n" + line2 + "\n" + line3
}

func truncStr(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxW {
		return s
	}
	if maxW <= 1 {
		return string(runes[:maxW])
	}
	return string(runes[:maxW-1]) + "…"
}

// ── Bubble Tea ────────────────────────────────────────────────────────────────

func (m ListModel) Init() tea.Cmd { return nil }

func (m ListModel) Update(message tea.Msg) (ListModel, tea.Cmd) {
	switch message := message.(type) {
	case tea.KeyMsg:
		if m.filter.Focused() {
			return m.handleFilterKey(message)
		}
		return m.handleNavKey(message)
	}
	return m, nil
}

func (m ListModel) handleFilterKey(km tea.KeyMsg) (ListModel, tea.Cmd) {
	switch km.String() {
	case "esc":
		m.filter.Blur()
		m.filter.SetValue("")
		m.refresh()
		return m, nil
	case "enter":
		if len(m.rows) == 1 && m.rows[0].kind == rowKindEndpoint {
			ep := m.rows[0].ep
			return m, func() tea.Msg { return tuimsg.EndpointSelectedMsg{Endpoint: ep} }
		}
		m.filter.Blur()
		m.clampCursor()
		m.redraw()
		return m, nil
	}
	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(km)
	m.cursor = 0
	m.buildRows()
	m.redraw()
	return m, cmd
}

func (m ListModel) handleNavKey(km tea.KeyMsg) (ListModel, tea.Cmd) {
	switch km.String() {
	case "/":
		cmd := m.filter.Focus()
		return m, cmd
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			m.redraw()
		}
	case "down", "j":
		if m.cursor < len(m.rows)-1 {
			m.cursor++
			m.redraw()
		}
	case "pgup", "ctrl+u":
		m.cursor = max(0, m.cursor-m.vp.Height)
		m.redraw()
	case "pgdown", "ctrl+d":
		if len(m.rows) > 0 {
			m.cursor = min(len(m.rows)-1, m.cursor+m.vp.Height)
		}
		m.redraw()
	case "g":
		m.cursor = 0
		m.redraw()
	case "G":
		if len(m.rows) > 0 {
			m.cursor = len(m.rows) - 1
		}
		m.redraw()
	case "d":
		m.summary = (m.summary + 1) % 3
		m.redraw()
	case "z", " ":
		if m.cursor < len(m.rows) && m.rows[m.cursor].kind == rowKindTag {
			ti := m.rows[m.cursor].tagIdx
			m.groups[ti].collapsed = !m.groups[ti].collapsed
			m.buildRows()
			m.clampCursor()
			m.redraw()
		}
	case "C":
		for i := range m.groups {
			m.groups[i].collapsed = true
		}
		m.refresh()
	case "E":
		for i := range m.groups {
			m.groups[i].collapsed = false
		}
		m.refresh()
	case "enter":
		if m.cursor >= len(m.rows) {
			break
		}
		row := m.rows[m.cursor]
		if row.kind == rowKindTag {
			m.groups[row.tagIdx].collapsed = !m.groups[row.tagIdx].collapsed
			m.buildRows()
			m.clampCursor()
			m.redraw()
		} else {
			ep := row.ep
			return m, func() tea.Msg { return tuimsg.EndpointSelectedMsg{Endpoint: ep} }
		}
	}
	return m, nil
}

// View renders the inner panel content (no border).
func (m ListModel) View() string {
	if !m.ready {
		return styles.Dim.Render("  Loading…")
	}

	w := m.width

	// Count / match line.
	var countLine string
	if q := m.filter.Value(); q != "" {
		countLine = styles.Dim.Render(fmt.Sprintf("  %d / %d", len(m.rows), m.total))
	} else {
		countLine = styles.Dim.Render(fmt.Sprintf("  %d endpoints", m.total))
	}

	// Filter input.
	var filterLine string
	if m.filter.Focused() || m.filter.Value() != "" {
		filterLine = m.filter.View()
	} else {
		filterLine = styles.Dim.Render("  / to filter")
	}

	sep := styles.SectionRule.Render(strings.Repeat("─", w))

	return countLine + "\n" +
		filterLine + "\n" +
		sep + "\n" +
		m.vp.View()
}
