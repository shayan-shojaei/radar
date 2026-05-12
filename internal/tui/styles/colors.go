package styles

import "github.com/charmbracelet/lipgloss"

// All color constants for the radar TUI.
// Named after their semantic role, not their visual appearance.
var (
	// Panel borders
	ColorBorderActive   = lipgloss.Color("#7aa2f7")
	ColorBorderInactive = lipgloss.Color("#3b4261")
	ColorBorderTitle    = lipgloss.Color("#bb9af7")

	// HTTP method badges
	ColorMethodGET    = lipgloss.Color("#9ece6a")
	ColorMethodPOST   = lipgloss.Color("#7aa2f7")
	ColorMethodPUT    = lipgloss.Color("#e0af68")
	ColorMethodPATCH  = lipgloss.Color("#ff9e64")
	ColorMethodDELETE = lipgloss.Color("#f7768e")

	// HTTP status code ranges
	ColorStatusSuccess  = lipgloss.Color("#9ece6a")
	ColorStatusRedirect = lipgloss.Color("#7aa2f7")
	ColorStatusClient   = lipgloss.Color("#e0af68")
	ColorStatusServer   = lipgloss.Color("#f7768e")

	// Text hierarchy
	ColorSelected     = lipgloss.Color("#2d3f76")
	ColorSelectedText = lipgloss.Color("#c0caf5")
	ColorNormalText   = lipgloss.Color("#a9b1d6")
	ColorDimText      = lipgloss.Color("#565f89")
	ColorHighlight    = lipgloss.Color("#bb9af7")

	// Status bar
	ColorStatusBarBg   = lipgloss.Color("#16161e")
	ColorStatusBarText = lipgloss.Color("#737aa2")
	ColorKeyBarBg      = lipgloss.Color("#1a1b2e")

	// Input fields
	ColorInputBorder     = lipgloss.Color("#7aa2f7")
	ColorInputBorderBlur = lipgloss.Color("#3b4261")
	ColorInputText       = lipgloss.Color("#c0caf5")
	ColorInputLabel      = lipgloss.Color("#bb9af7")
	ColorInputRequired   = lipgloss.Color("#f7768e")

	// JSON syntax highlighting
	ColorJSONKey    = lipgloss.Color("#bb9af7")
	ColorJSONString = lipgloss.Color("#9ece6a")
	ColorJSONNumber = lipgloss.Color("#e0af68")
	ColorJSONBool   = lipgloss.Color("#f7768e")
	ColorJSONNull   = lipgloss.Color("#f7768e")
)
