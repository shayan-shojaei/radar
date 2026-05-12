package styles

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var bd = lipgloss.RoundedBorder()

// MethodBadge returns a colored, padded HTTP method badge.
func MethodBadge(method string) string {
	padded := fmt.Sprintf("%-6s", method)
	switch method {
	case "GET":
		return MethodGET.Render(padded)
	case "POST":
		return MethodPOST.Render(padded)
	case "PUT":
		return MethodPUT.Render(padded)
	case "PATCH":
		return MethodPATCH.Render(padded)
	case "DELETE":
		return MethodDELETE.Render(padded)
	default:
		return MethodOther.Render(padded)
	}
}

// MethodStyle returns the lipgloss.Style for a given HTTP method.
func MethodStyle(method string) lipgloss.Style {
	switch method {
	case "GET":
		return MethodGET
	case "POST":
		return MethodPOST
	case "PUT":
		return MethodPUT
	case "PATCH":
		return MethodPATCH
	case "DELETE":
		return MethodDELETE
	default:
		return MethodOther
	}
}

// StatusBadge returns a colored status code + reason string.
func StatusBadge(code int) string {
	reason := http.StatusText(code)
	text := fmt.Sprintf(" %d %s ", code, reason)
	switch {
	case code >= 200 && code < 300:
		return Status2xx.Render(text)
	case code >= 300 && code < 400:
		return Status3xx.Render(text)
	case code >= 400 && code < 500:
		return Status4xx.Render(text)
	case code >= 500:
		return Status5xx.Render(text)
	default:
		return StatusDef.Render(text)
	}
}

// SectionDivider returns a full-width styled "─" rule with an optional label.
func SectionDivider(label string, width int) string {
	if label == "" {
		return SectionRule.Render(strings.Repeat("─", width))
	}
	styledLabel := " " + SectionHeader.Render(label) + " "
	labelW := lipgloss.Width(styledLabel)
	remaining := width - labelW
	if remaining < 0 {
		remaining = 0
	}
	left := SectionRule.Render(strings.Repeat("─", 1))
	right := SectionRule.Render(strings.Repeat("─", remaining-1))
	return left + styledLabel + right
}

// RenderPanel draws a rounded-border panel with an optional title embedded in
// the top border. active controls the border color.
// outerW and outerH are the full panel dimensions including the border.
func RenderPanel(title, content string, active bool, outerW, outerH int) string {
	var borderColor lipgloss.Color
	if active {
		borderColor = ColorBorderActive
	} else {
		borderColor = ColorBorderInactive
	}
	bSty := lipgloss.NewStyle().Foreground(borderColor)

	innerW := outerW - 2
	innerH := outerH - 2
	if innerW < 0 {
		innerW = 0
	}
	if innerH < 0 {
		innerH = 0
	}

	// ── top border ────────────────────────────────────────────────────────────
	var topMid string
	if title != "" {
		styledTitle := " " + PanelTitle.Render(title) + " "
		titleW := lipgloss.Width(styledTitle)
		fillW := innerW - titleW
		if fillW < 0 {
			fillW = 0
		}
		topMid = styledTitle + bSty.Render(strings.Repeat(bd.Top, fillW))
	} else {
		topMid = bSty.Render(strings.Repeat(bd.Top, innerW))
	}
	topLine := bSty.Render(bd.TopLeft) + topMid + bSty.Render(bd.TopRight)

	// ── bottom border ─────────────────────────────────────────────────────────
	bottomLine := bSty.Render(bd.BottomLeft+strings.Repeat(bd.Bottom, innerW)+bd.BottomRight)

	// ── content rows ──────────────────────────────────────────────────────────
	lines := strings.Split(content, "\n")
	// Remove trailing empty line that strings.Split adds after a terminal \n.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	var sb strings.Builder
	sb.WriteString(topLine)
	sb.WriteByte('\n')

	for i := 0; i < innerH; i++ {
		var line string
		if i < len(lines) {
			line = lines[i]
		}
		// Pad to innerW using ANSI-aware width.
		lineW := lipgloss.Width(line)
		if lineW < innerW {
			line += strings.Repeat(" ", innerW-lineW)
		}
		sb.WriteString(bSty.Render(bd.Left))
		sb.WriteString(line)
		sb.WriteString(bSty.Render(bd.Right))
		sb.WriteByte('\n')
	}

	sb.WriteString(bottomLine)
	return sb.String()
}

// KeyBinding renders a single key+action hint for the keybinding bar.
func KeyBinding(key, action string) string {
	return KeyBarKey.Render("["+key+"]") + " " + KeyBar.Render(action)
}

// StatusBarLine renders the full-width status bar.
func StatusBarLine(left, right string, width int) string {
	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	pad := width - leftW - rightW
	if pad < 0 {
		pad = 0
	}
	fill := StatusBar.Render(strings.Repeat(" ", pad))
	return left + fill + right
}

// ColorizeJSON pretty-prints and syntax-highlights a JSON string.
// Returns the raw string unchanged if it is not valid JSON.
func ColorizeJSON(raw string) string {
	var v interface{}
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return raw
	}
	// Re-indent first for consistent formatting.
	indented, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return raw
	}

	dec := json.NewDecoder(bytes.NewReader(indented))
	dec.UseNumber()

	var out strings.Builder
	var prevToken json.Token
	var inKey bool

	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}

		switch t := tok.(type) {
		case json.Delim:
			out.WriteString(Normal.Render(t.String()))
			inKey = t == '{' || t == '['
		case string:
			if inKey {
				out.WriteString(JSONKey.Render(`"` + t + `"`))
				inKey = false
			} else {
				out.WriteString(JSONString.Render(`"` + t + `"`))
			}
		case json.Number:
			out.WriteString(JSONNumber.Render(t.String()))
		case bool:
			if t {
				out.WriteString(JSONBool.Render("true"))
			} else {
				out.WriteString(JSONBool.Render("false"))
			}
		case nil:
			out.WriteString(JSONNull.Render("null"))
		}

		// Detect if next token after a colon is a key.
		if s, ok := prevToken.(string); ok {
			_ = s
		}
		prevToken = tok
	}

	// The simple token-based approach loses whitespace; fall back to a
	// line-by-line colorizer for the indented output.
	return colorizeJSONLines(string(indented))
}

// colorizeJSONLines applies token-level coloring line by line to pre-indented JSON.
func colorizeJSONLines(src string) string {
	var sb strings.Builder
	lines := strings.Split(src, "\n")
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		indent := line[:len(line)-len(trimmed)]

		switch {
		case strings.HasPrefix(trimmed, `"`) && strings.Contains(trimmed, `":`):
			// Key: value line
			colonIdx := strings.Index(trimmed, `":`)
			if colonIdx >= 0 {
				key := trimmed[:colonIdx+1]
				rest := trimmed[colonIdx+1:]
				sb.WriteString(indent)
				sb.WriteString(JSONKey.Render(key))
				sb.WriteString(Normal.Render(":"))
				sb.WriteString(colorizeValue(strings.TrimPrefix(rest, " ")))
			} else {
				sb.WriteString(Dim.Render(line))
			}
		case strings.HasPrefix(trimmed, `"`):
			sb.WriteString(indent + JSONString.Render(trimmed))
		case trimmed == "true" || trimmed == "false":
			sb.WriteString(indent + JSONBool.Render(trimmed))
		case trimmed == "null":
			sb.WriteString(indent + JSONNull.Render(trimmed))
		case len(trimmed) > 0 && (trimmed[0] == '-' || (trimmed[0] >= '0' && trimmed[0] <= '9')):
			// number (possibly with trailing comma)
			core, suffix := splitTrailingComma(trimmed)
			sb.WriteString(indent + JSONNumber.Render(core) + Normal.Render(suffix))
		default:
			sb.WriteString(Normal.Render(line))
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

func colorizeValue(val string) string {
	val = strings.TrimSpace(val)
	core, suffix := splitTrailingComma(val)
	switch {
	case len(core) > 0 && core[0] == '"':
		return " " + JSONString.Render(core) + Normal.Render(suffix)
	case core == "true" || core == "false":
		return " " + JSONBool.Render(core) + Normal.Render(suffix)
	case core == "null":
		return " " + JSONNull.Render(core) + Normal.Render(suffix)
	case len(core) > 0 && (core[0] == '-' || (core[0] >= '0' && core[0] <= '9')):
		return " " + JSONNumber.Render(core) + Normal.Render(suffix)
	default:
		return " " + Normal.Render(val)
	}
}

func splitTrailingComma(s string) (core, suffix string) {
	s = strings.TrimRight(s, " \t")
	if strings.HasSuffix(s, ",") {
		return s[:len(s)-1], ","
	}
	return s, ""
}
