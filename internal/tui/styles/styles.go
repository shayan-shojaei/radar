package styles

import (
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

// Pre-built styles used across all views.
// No lipgloss.NewStyle() calls outside this package.
var (
	// ── text ──────────────────────────────────────────────────────────────────
	Normal    = lipgloss.NewStyle().Foreground(ColorNormalText)
	Dim       = lipgloss.NewStyle().Foreground(ColorDimText)
	Highlight = lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true)

	Selected = lipgloss.NewStyle().
			Background(ColorSelected).
			Foreground(ColorSelectedText).
			Bold(true)

	// ── panel titles (embedded in border) ─────────────────────────────────────
	PanelTitle = lipgloss.NewStyle().
			Foreground(ColorBorderTitle).
			Bold(true)

	// ── section dividers inside panels ────────────────────────────────────────
	SectionHeader = lipgloss.NewStyle().
			Foreground(ColorHighlight).
			Bold(true)

	SectionRule = lipgloss.NewStyle().
			Foreground(ColorBorderInactive)

	// ── method badge ──────────────────────────────────────────────────────────
	MethodGET    = lipgloss.NewStyle().Foreground(ColorMethodGET).Bold(true)
	MethodPOST   = lipgloss.NewStyle().Foreground(ColorMethodPOST).Bold(true)
	MethodPUT    = lipgloss.NewStyle().Foreground(ColorMethodPUT).Bold(true)
	MethodPATCH  = lipgloss.NewStyle().Foreground(ColorMethodPATCH).Bold(true)
	MethodDELETE = lipgloss.NewStyle().Foreground(ColorMethodDELETE).Bold(true)
	MethodOther  = lipgloss.NewStyle().Foreground(ColorNormalText).Bold(true)

	// ── status code badge ─────────────────────────────────────────────────────
	Status2xx = lipgloss.NewStyle().Foreground(ColorStatusSuccess).Bold(true)
	Status3xx = lipgloss.NewStyle().Foreground(ColorStatusRedirect).Bold(true)
	Status4xx = lipgloss.NewStyle().Foreground(ColorStatusClient).Bold(true)
	Status5xx = lipgloss.NewStyle().Foreground(ColorStatusServer).Bold(true)
	StatusDef = lipgloss.NewStyle().Foreground(ColorNormalText).Bold(true)

	// ── status / keybinding bars ──────────────────────────────────────────────
	StatusBar = lipgloss.NewStyle().
			Background(ColorStatusBarBg).
			Foreground(ColorStatusBarText)

	StatusBarMode = lipgloss.NewStyle().
			Background(ColorStatusBarBg).
			Bold(true)

	KeyBar = lipgloss.NewStyle().
		Background(ColorKeyBarBg).
		Foreground(ColorDimText)

	KeyBarKey = lipgloss.NewStyle().
			Background(ColorKeyBarBg).
			Foreground(ColorHighlight).
			Bold(true)

	// ── input fields ──────────────────────────────────────────────────────────
	InputLabel    = lipgloss.NewStyle().Foreground(ColorInputLabel)
	InputRequired = lipgloss.NewStyle().Foreground(ColorInputRequired)

	// ── list rows ─────────────────────────────────────────────────────────────
	ListPath        = lipgloss.NewStyle().Foreground(ColorNormalText)
	ListPathFocused = lipgloss.NewStyle().Foreground(ColorSelectedText).Bold(true)
	ListSummary     = lipgloss.NewStyle().Foreground(ColorDimText)

	// ── JSON highlighting ─────────────────────────────────────────────────────
	JSONKey    = lipgloss.NewStyle().Foreground(ColorJSONKey)
	JSONString = lipgloss.NewStyle().Foreground(ColorJSONString)
	JSONNumber = lipgloss.NewStyle().Foreground(ColorJSONNumber)
	JSONBool   = lipgloss.NewStyle().Foreground(ColorJSONBool)
	JSONNull   = lipgloss.NewStyle().Foreground(ColorJSONNull)

	// ── spinner ───────────────────────────────────────────────────────────────
	SpinnerStyle = lipgloss.NewStyle().Foreground(ColorBorderActive)

	// ── error box ─────────────────────────────────────────────────────────────
	ErrorBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorMethodDELETE).
			Padding(1, 2)

	ErrorTitle = lipgloss.NewStyle().Foreground(ColorMethodDELETE).Bold(true)
	ErrorMsg   = lipgloss.NewStyle().Foreground(ColorNormalText)
	ErrorHint  = lipgloss.NewStyle().Foreground(ColorDimText)
)

// NewStyledTextInput returns a textinput pre-configured with radar styles.
func NewStyledTextInput(placeholder string) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.PromptStyle = lipgloss.NewStyle().Foreground(ColorDimText)
	ti.TextStyle = lipgloss.NewStyle().Foreground(ColorInputText)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(ColorDimText)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(ColorBorderActive)
	return ti
}
