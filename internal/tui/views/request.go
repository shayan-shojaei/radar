package views

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	tuimsg "github.com/shayan-shojaei/radar/internal/tui/msg"
	"github.com/shayan-shojaei/radar/internal/tui/styles"
	"github.com/shayan-shojaei/radar/pkg/models"
)

var pathParamRe = regexp.MustCompile(`\{([^}]+)\}`)

type fieldKind int

const (
	kindTextInput fieldKind = iota
	kindTextarea
)

type bodyMode int

const (
	bodyModeRaw bodyMode = iota
	bodyModeFields
)

var contentTypeOptions = []string{
	"application/json",
	"text/plain",
	"application/x-www-form-urlencoded",
	"multipart/form-data",
	"text/xml",
	"application/xml",
}

type field struct {
	kind         fieldKind
	label        string
	required     bool
	paramKey     string
	paramIn      string
	textInput    textinput.Model
	textarea     textarea.Model
	cycleOptions []string // non-nil = supports cycling with ctrl+n/ctrl+p
	cycleIdx     int      // index into cycleOptions; -1 = custom typed value
}

// RequestModel is the Bubble Tea model for the request editor panel content.
type RequestModel struct {
	endpoint        models.ParsedEndpoint
	fields          []field
	bodyFieldInputs []field // spec-defined body fields (fields mode)
	extraBodyFields []field // free-form key-value pairs (fields mode)
	bodyMode        bodyMode
	focused         int
	vp              viewport.Model
	width           int
	height          int
	ready           bool
}

// NewRequestModel creates a RequestModel for the given endpoint.
// initialBaseURL pre-populates the Base URL field if provided.
func NewRequestModel(ep models.ParsedEndpoint, initialBaseURL string) RequestModel {
	var fields []field

	// Base URL.
	ti := styles.NewStyledTextInput("https://api.example.com")
	ti.CharLimit = 512
	if initialBaseURL != "" {
		ti.SetValue(initialBaseURL)
	}
	fields = append(fields, field{kind: kindTextInput, label: "Base URL", paramKey: "__baseURL", textInput: ti})

	// Path params from the path template.
	for _, match := range pathParamRe.FindAllStringSubmatch(ep.Path, -1) {
		ti := styles.NewStyledTextInput(match[1])
		ti.CharLimit = 256
		fields = append(fields, field{
			kind: kindTextInput, label: match[1], paramKey: match[1], paramIn: "path",
			required: true, textInput: ti,
		})
	}

	// Content-Type field (appears in HEADERS section, before other spec params).
	// Skip if the spec already defines a content-type header parameter.
	hasContentType := false
	for _, p := range ep.Parameters {
		if strings.EqualFold(p.In, "header") && strings.EqualFold(p.Name, "content-type") {
			hasContentType = true
			break
		}
	}
	if ep.HasBody && !hasContentType {
		cti := styles.NewStyledTextInput("application/json")
		cti.CharLimit = 256
		cti.SetValue("application/json")
		fields = append(fields, field{
			kind:         kindTextInput,
			label:        "Content-Type",
			paramKey:     "content-type", // lowercase key for dedup
			paramIn:      "header",
			cycleOptions: contentTypeOptions,
			cycleIdx:     0,
			textInput:    cti,
		})
	}

	// Query / header / cookie params from spec.
	for _, p := range ep.Parameters {
		switch strings.ToLower(p.In) {
		case "query", "header", "cookie":
			ti := styles.NewStyledTextInput(p.Name)
			ti.CharLimit = 512
			// Normalize header keys to lowercase to avoid duplicates.
			paramKey := p.Name
			var cycleOpts []string
			cycleIdx := 0
			if strings.EqualFold(p.In, "header") {
				paramKey = strings.ToLower(p.Name)
				// When the spec defines content-type, attach the same cycle options
				// that the auto-inserted field would have had.
				if paramKey == "content-type" && ep.HasBody {
					cycleOpts = contentTypeOptions
					ti.SetValue(contentTypeOptions[0])
				}
			}
			fields = append(fields, field{
				kind: kindTextInput, label: p.Name, required: p.Required,
				paramKey: paramKey, paramIn: p.In, textInput: ti,
				cycleOptions: cycleOpts, cycleIdx: cycleIdx,
			})
		}
	}

	// Body textarea.
	if ep.HasBody {
		ta := textarea.New()
		ta.Placeholder = "{ }"
		ta.CharLimit = 0
		ta.ShowLineNumbers = false
		ta.FocusedStyle.Base = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.ColorInputBorder)
		ta.BlurredStyle.Base = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.ColorInputBorderBlur)
		ta.FocusedStyle.Text = lipgloss.NewStyle().Foreground(styles.ColorInputText)
		ta.BlurredStyle.Text = lipgloss.NewStyle().Foreground(styles.ColorDimText)
		ta.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(styles.ColorDimText)
		ta.BlurredStyle.Placeholder = lipgloss.NewStyle().Foreground(styles.ColorDimText)
		fields = append(fields, field{kind: kindTextarea, label: "Body", paramKey: "__body", textarea: ta})
	}

	if len(fields) > 0 {
		if fields[0].kind == kindTextInput {
			fields[0].textInput.Focus()
		} else {
			fields[0].textarea.Focus()
		}
	}

	m := RequestModel{endpoint: ep, fields: fields}

	// Build structured body field inputs when spec provides body schema.
	if len(ep.BodyFields) > 0 {
		for _, bf := range ep.BodyFields {
			placeholder := bf.Name
			if bf.Example != "" {
				placeholder = bf.Example
			}
			ti := styles.NewStyledTextInput(placeholder)
			ti.CharLimit = 512
			m.bodyFieldInputs = append(m.bodyFieldInputs, field{
				kind:      kindTextInput,
				label:     bf.Name,
				required:  bf.Required,
				paramKey:  bf.Name,
				paramIn:   "__bodyfield",
				textInput: ti,
			})
		}
		// Two free-form key-value pairs for extra fields.
		for i := range 2 {
			kti := styles.NewStyledTextInput(fmt.Sprintf("key%d", i+1))
			kti.CharLimit = 256
			m.extraBodyFields = append(m.extraBodyFields, field{
				kind: kindTextInput, label: "Key", paramKey: "", paramIn: "__bodyextra", textInput: kti,
			})
			vti := styles.NewStyledTextInput("value")
			vti.CharLimit = 512
			m.extraBodyFields = append(m.extraBodyFields, field{
				kind: kindTextInput, label: "Value", paramKey: "", paramIn: "__bodyextraval", textInput: vti,
			})
		}
	}

	return m
}

// ── field navigation helpers ──────────────────────────────────────────────────

// activeFieldsLen returns the count of fields visible in the current body mode.
func (m RequestModel) activeFieldsLen() int {
	if m.bodyMode == bodyModeRaw || len(m.endpoint.BodyFields) == 0 {
		return len(m.fields)
	}
	return m.nonBodyFieldCount() + len(m.bodyFieldInputs) + len(m.extraBodyFields)
}

// nonBodyFieldCount counts fields in m.fields that are not the __body textarea.
func (m RequestModel) nonBodyFieldCount() int {
	count := 0
	for _, f := range m.fields {
		if f.paramKey != "__body" {
			count++
		}
	}
	return count
}

// activeFields returns the slice of fields to use for rendering in the current body mode.
func (m RequestModel) activeFields() []field {
	if m.bodyMode == bodyModeRaw || len(m.endpoint.BodyFields) == 0 {
		return m.fields
	}
	var result []field
	for _, f := range m.fields {
		if f.paramKey != "__body" {
			result = append(result, f)
		}
	}
	result = append(result, m.bodyFieldInputs...)
	result = append(result, m.extraBodyFields...)
	return result
}

// activeFocusedField returns a pointer to the focused field in its underlying slice.
func (m *RequestModel) activeFocusedField() *field {
	if m.bodyMode == bodyModeRaw || len(m.endpoint.BodyFields) == 0 {
		if m.focused >= 0 && m.focused < len(m.fields) {
			return &m.fields[m.focused]
		}
		return nil
	}
	// fields mode: [non-body fields | bodyFieldInputs | extraBodyFields]
	nonBodyCount := m.nonBodyFieldCount()
	if m.focused < nonBodyCount {
		idx := 0
		for i := range m.fields {
			if m.fields[i].paramKey == "__body" {
				continue
			}
			if idx == m.focused {
				return &m.fields[i]
			}
			idx++
		}
		return nil
	}
	bodyIdx := m.focused - nonBodyCount
	if bodyIdx < len(m.bodyFieldInputs) {
		return &m.bodyFieldInputs[bodyIdx]
	}
	extraIdx := bodyIdx - len(m.bodyFieldInputs)
	if extraIdx >= 0 && extraIdx < len(m.extraBodyFields) {
		return &m.extraBodyFields[extraIdx]
	}
	return nil
}

func (m *RequestModel) clampFocus() {
	n := m.activeFieldsLen()
	if n == 0 {
		m.focused = 0
		return
	}
	if m.focused >= n {
		m.focused = n - 1
	}
	if m.focused < 0 {
		m.focused = 0
	}
}

// Resize is called by the app when inner panel dimensions change.
func (m *RequestModel) Resize(innerW, innerH int) {
	m.width = innerW
	m.height = innerH
	m.vp = viewport.New(innerW, innerH)
	m.ready = true
	// Resize textarea width if present.
	for i := range m.fields {
		if m.fields[i].kind == kindTextarea {
			m.fields[i].textarea.SetWidth(innerW - 4)
			bodyH := innerH / 2
			if bodyH < 3 {
				bodyH = 3
			}
			m.fields[i].textarea.SetHeight(bodyH)
		}
	}
	m.vp.SetContent(m.renderFields())
}

// ApplySession populates fields from saved RequestData.
func (m *RequestModel) ApplySession(rd models.RequestData) {
	for i := range m.fields {
		f := &m.fields[i]
		switch f.paramKey {
		case "__baseURL":
			if idx := strings.Index(rd.EndpointKey, "|"); idx >= 0 {
				f.textInput.SetValue(rd.EndpointKey[:idx])
			} else {
				f.textInput.SetValue(rd.EndpointKey)
			}
		case "__body":
			f.textarea.SetValue(rd.Body)
		default:
			switch strings.ToLower(f.paramIn) {
			case "path":
				if v, ok := rd.PathParams[f.paramKey]; ok {
					f.textInput.SetValue(v)
				}
			case "query":
				if v, ok := rd.QueryParams[f.paramKey]; ok {
					f.textInput.SetValue(v)
				}
			case "header":
				// Headers are stored with lowercase keys.
				if v, ok := rd.Headers[strings.ToLower(f.paramKey)]; ok {
					f.textInput.SetValue(v)
				}
			case "cookie":
				if v, ok := rd.Cookies[f.paramKey]; ok {
					f.textInput.SetValue(v)
				}
			}
		}
	}
	// Restore body mode and re-populate individual field inputs from saved JSON.
	if rd.BodyMode == "fields" && len(m.endpoint.BodyFields) > 0 {
		m.bodyMode = bodyModeFields
		if rd.Body != "" {
			var obj map[string]interface{}
			if err := json.Unmarshal([]byte(rd.Body), &obj); err == nil {
				specKeys := make(map[string]bool, len(m.bodyFieldInputs))
				for i := range m.bodyFieldInputs {
					key := m.bodyFieldInputs[i].paramKey
					specKeys[key] = true
					if v, ok := obj[key]; ok {
						m.bodyFieldInputs[i].textInput.SetValue(fmt.Sprint(v))
					}
				}
				extraIdx := 0
				for k, v := range obj {
					if !specKeys[k] && extraIdx+1 < len(m.extraBodyFields) {
						m.extraBodyFields[extraIdx].textInput.SetValue(k)
						m.extraBodyFields[extraIdx+1].textInput.SetValue(fmt.Sprint(v))
						extraIdx += 2
					}
				}
			}
		}
	}
}

// CurrentRequestData returns the current form state for auto-saving on navigate-away.
func (m RequestModel) CurrentRequestData() models.RequestData {
	return m.buildRequestData()
}

func (m RequestModel) buildRequestData() models.RequestData {
	rd := models.RequestData{
		Headers:     make(map[string]string),
		QueryParams: make(map[string]string),
		PathParams:  make(map[string]string),
		Cookies:     make(map[string]string),
	}
	var baseURL string
	for _, f := range m.fields {
		val := fieldValue(f)
		switch f.paramKey {
		case "__baseURL":
			baseURL = val
		case "__body":
			if m.bodyMode == bodyModeRaw || len(m.endpoint.BodyFields) == 0 {
				rd.Body = val
			}
		default:
			switch strings.ToLower(f.paramIn) {
			case "path":
				rd.PathParams[f.paramKey] = val
			case "query":
				rd.QueryParams[f.paramKey] = val
			case "header":
				rd.Headers[strings.ToLower(f.paramKey)] = val
			case "cookie":
				rd.Cookies[f.paramKey] = val
			}
		}
	}
	if m.bodyMode == bodyModeFields && len(m.endpoint.BodyFields) > 0 {
		rd.Body = m.buildBodyJSON()
	}
	// F2: persist body mode.
	if m.bodyMode == bodyModeFields {
		rd.BodyMode = "fields"
	} else {
		rd.BodyMode = "raw"
	}
	rd.EndpointKey = baseURL + "|" + fmt.Sprintf("%s %s", m.endpoint.Method, m.endpoint.Path)
	return rd
}

// buildBodyJSON serializes bodyFieldInputs + extraBodyFields into indented JSON.
func (m RequestModel) buildBodyJSON() string {
	obj := make(map[string]interface{})
	for i, f := range m.bodyFieldInputs {
		val := f.textInput.Value()
		if val == "" {
			continue
		}
		typ := ""
		if i < len(m.endpoint.BodyFields) {
			typ = m.endpoint.BodyFields[i].Type
		}
		obj[f.paramKey] = convertBodyValue(val, typ)
	}
	for i := 0; i+1 < len(m.extraBodyFields); i += 2 {
		key := m.extraBodyFields[i].textInput.Value()
		val := m.extraBodyFields[i+1].textInput.Value()
		if key != "" {
			obj[key] = val
		}
	}
	if len(obj) == 0 {
		return ""
	}
	data, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return ""
	}
	return string(data)
}

func convertBodyValue(val, typ string) interface{} {
	switch typ {
	case "integer":
		if n, err := strconv.Atoi(val); err == nil {
			return n
		}
	case "boolean":
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
	}
	return val
}

// serializeBodyFieldsToJSON writes the current fields-mode body to the __body textarea.
func (m *RequestModel) serializeBodyFieldsToJSON() {
	body := m.buildBodyJSON()
	for i := range m.fields {
		if m.fields[i].paramKey == "__body" {
			m.fields[i].textarea.SetValue(body)
			return
		}
	}
}

func fieldValue(f field) string {
	if f.kind == kindTextInput {
		return f.textInput.Value()
	}
	return f.textarea.Value()
}

// ── rendering ─────────────────────────────────────────────────────────────────

func (m RequestModel) renderFields() string {
	w := m.width
	if w < 20 {
		w = 60
	}

	fields := m.activeFields()
	var sb strings.Builder
	var lastSection string

	for i, f := range fields {
		// Emit section header when the param group changes.
		section := sectionFor(f)
		if section != lastSection {
			if lastSection != "" {
				sb.WriteByte('\n')
			}
			sb.WriteString(styles.SectionDivider(section, w))
			sb.WriteByte('\n')
			lastSection = section
		}

		focused := i == m.focused

		if f.kind == kindTextarea {
			sb.WriteString(renderTextarea(f, focused, w))
			sb.WriteByte('\n')
			// Hint: switch to fields mode when spec fields are available.
			if f.paramKey == "__body" && len(m.endpoint.BodyFields) > 0 {
				sb.WriteString(styles.Dim.Render("  ctrl+t: switch to fields mode"))
				sb.WriteByte('\n')
			}
			continue
		}

		// Label column (18 chars wide).
		label := styles.InputLabel.Render(fmt.Sprintf("%-16s", f.label))
		if f.required {
			label += styles.InputRequired.Render("*")
		} else {
			label += " "
		}
		label += " "

		// Style the textinput border based on focus.
		inputW := w - 20
		if len(f.cycleOptions) > 0 {
			inputW -= 16 // reserve room for the cycle hint
		}
		if inputW < 10 {
			inputW = 10
		}
		f.textInput.Width = inputW

		if focused {
			f.textInput.PromptStyle = lipgloss.NewStyle().Foreground(styles.ColorBorderActive)
		} else {
			f.textInput.PromptStyle = lipgloss.NewStyle().Foreground(styles.ColorDimText)
		}

		line := label + f.textInput.View()
		if len(f.cycleOptions) > 0 {
			line += styles.Dim.Render("  ↕ ctrl+n/p")
		}
		sb.WriteString(line)
		sb.WriteByte('\n')
	}

	// Hint at the bottom of the BODY section in fields mode.
	if m.bodyMode == bodyModeFields && len(m.endpoint.BodyFields) > 0 {
		sb.WriteByte('\n')
		sb.WriteString(styles.Dim.Render("  ctrl+t: switch to raw mode"))
		sb.WriteByte('\n')
	}

	return sb.String()
}

func renderTextarea(f field, focused bool, w int) string {
	_ = focused // textarea handles its own focus style
	return f.textarea.View()
}

func sectionFor(f field) string {
	switch f.paramKey {
	case "__baseURL":
		return "BASE URL"
	case "__body":
		return "BODY"
	}
	switch strings.ToLower(f.paramIn) {
	case "path":
		return "PATH PARAMS"
	case "query":
		return "QUERY PARAMS"
	case "header":
		return "HEADERS"
	case "cookie":
		return "COOKIES"
	case "__bodyfield", "__bodyextra", "__bodyextraval":
		return "BODY"
	}
	return "PARAMS"
}

// ── Bubble Tea ────────────────────────────────────────────────────────────────

func (m RequestModel) Init() tea.Cmd { return textinput.Blink }

func (m RequestModel) Update(message tea.Msg) (RequestModel, tea.Cmd) {
	switch km := message.(type) {
	case tea.KeyMsg:
		switch km.String() {
		case "esc":
			return m, func() tea.Msg { return tuimsg.BackMsg{} }
		case "ctrl+s":
			rd := m.buildRequestData()
			return m, func() tea.Msg { return tuimsg.RequestSentMsg{Request: rd} }
		case "tab", "down":
			(&m).blurCurrent()
			m.focused = (m.focused + 1) % m.activeFieldsLen()
			return m, (&m).focusCurrent()
		case "shift+tab", "up":
			(&m).blurCurrent()
			n := m.activeFieldsLen()
			m.focused = (m.focused - 1 + n) % n
			return m, (&m).focusCurrent()
		case "ctrl+n":
			f := (&m).activeFocusedField()
			if f != nil && len(f.cycleOptions) > 0 {
				f.cycleIdx = (f.cycleIdx + 1) % len(f.cycleOptions)
				f.textInput.SetValue(f.cycleOptions[f.cycleIdx])
			}
			if m.ready {
				m.vp.SetContent(m.renderFields())
			}
			return m, nil
		case "ctrl+p":
			f := (&m).activeFocusedField()
			if f != nil && len(f.cycleOptions) > 0 {
				idx := f.cycleIdx - 1
				if idx < 0 {
					idx = len(f.cycleOptions) - 1
				}
				f.cycleIdx = idx
				f.textInput.SetValue(f.cycleOptions[f.cycleIdx])
			}
			if m.ready {
				m.vp.SetContent(m.renderFields())
			}
			return m, nil
		case "ctrl+l":
			return m, func() tea.Msg { return tuimsg.SessionLoadRequestMsg{} }
		case "ctrl+r":
			for i := range m.fields {
				if m.fields[i].kind == kindTextInput {
					m.fields[i].textInput.SetValue("")
				} else {
					m.fields[i].textarea.SetValue("")
				}
			}
			for i := range m.bodyFieldInputs {
				m.bodyFieldInputs[i].textInput.SetValue("")
			}
			for i := range m.extraBodyFields {
				m.extraBodyFields[i].textInput.SetValue("")
			}
			if m.ready {
				m.vp.SetContent(m.renderFields())
			}
			return m, nil
		case "ctrl+t":
			if m.endpoint.HasBody && len(m.endpoint.BodyFields) > 0 {
				if m.bodyMode == bodyModeRaw {
					m.bodyMode = bodyModeFields
				} else {
					(&m).serializeBodyFieldsToJSON()
					m.bodyMode = bodyModeRaw
				}
				(&m).clampFocus()
				if m.ready {
					m.vp.SetContent(m.renderFields())
				}
			}
			return m, nil
		}

		// Mark cycle field as custom if user types a character.
		if km.Type == tea.KeyRunes {
			f := (&m).activeFocusedField()
			if f != nil && len(f.cycleOptions) > 0 && f.cycleIdx != -1 {
				f.cycleIdx = -1
			}
		}
	}

	if m.activeFieldsLen() == 0 {
		return m, nil
	}
	var cmd tea.Cmd
	f := (&m).activeFocusedField()
	if f == nil {
		return m, nil
	}
	if f.kind == kindTextInput {
		f.textInput, cmd = f.textInput.Update(message)
	} else {
		f.textarea, cmd = f.textarea.Update(message)
	}
	if m.ready {
		m.vp.SetContent(m.renderFields())
	}
	return m, cmd
}

func (m *RequestModel) blurCurrent() {
	f := m.activeFocusedField()
	if f == nil {
		return
	}
	if f.kind == kindTextInput {
		f.textInput.Blur()
	} else {
		f.textarea.Blur()
	}
}

func (m *RequestModel) focusCurrent() tea.Cmd {
	f := m.activeFocusedField()
	if f == nil {
		return nil
	}
	if f.kind == kindTextInput {
		return f.textInput.Focus()
	}
	f.textarea.Focus()
	return nil
}

// Endpoint returns the endpoint being edited, or nil if none selected yet.
func (m RequestModel) Endpoint() *models.ParsedEndpoint {
	if m.endpoint.Method == "" {
		return nil
	}
	ep := m.endpoint
	return &ep
}

// View renders inner panel content (no border).
func (m RequestModel) View() string {
	if !m.ready {
		return styles.Dim.Render("  Loading…")
	}
	if len(m.fields) == 0 {
		return styles.Dim.Render("  no parameters")
	}
	m.vp.SetContent(m.renderFields())
	return m.vp.View()
}
