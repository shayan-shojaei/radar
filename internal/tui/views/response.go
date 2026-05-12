package views

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	tuimsg "github.com/shayan-shojaei/radar/internal/tui/msg"
	"github.com/shayan-shojaei/radar/internal/tui/styles"
	"github.com/shayan-shojaei/radar/pkg/models"
)

// ResponseModel is the Bubble Tea model for the response viewer panel content.
type ResponseModel struct {
	response        models.ResponseData
	vp              viewport.Model
	headersExpanded bool
	width           int
	height          int
	ready           bool
	hasResponse     bool
}

// NewResponseModel creates a ResponseModel from a ResponseData.
func NewResponseModel(resp models.ResponseData) ResponseModel {
	return ResponseModel{
		response:    resp,
		hasResponse: true,
	}
}

// EmptyResponseModel returns a blank placeholder model.
func EmptyResponseModel() ResponseModel {
	return ResponseModel{}
}

// Resize is called by the app when inner panel dimensions change.
func (m *ResponseModel) Resize(innerW, innerH int) {
	m.width = innerW
	m.height = innerH
	// Reserve 2 lines: status+duration row + divider.
	vpH := innerH - 2
	if vpH < 1 {
		vpH = 1
	}
	m.vp = viewport.New(innerW, vpH)
	m.ready = true
	if m.hasResponse {
		m.vp.SetContent(m.bodyContent())
	}
}

// HasResponse reports whether a response has been received.
func (m ResponseModel) HasResponse() bool { return m.hasResponse }

// isHTML reports whether the response content-type is text/html.
func (m ResponseModel) isHTML() bool {
	ct := m.response.Headers["Content-Type"]
	if ct == "" {
		ct = m.response.Headers["content-type"]
	}
	return strings.Contains(strings.ToLower(ct), "text/html")
}

// openInBrowserCmd writes the HTML body to a temp file and opens it in the default browser.
func openInBrowserCmd(body string) tea.Cmd {
	return func() tea.Msg {
		f, err := os.CreateTemp("", "radar-*.html")
		if err != nil {
			return nil
		}
		if _, err := f.WriteString(body); err != nil {
			f.Close()
			return nil
		}
		f.Close()
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", f.Name())
		case "windows":
			cmd = exec.Command("cmd", "/c", "start", f.Name())
		default:
			cmd = exec.Command("xdg-open", f.Name())
		}
		cmd.Start() //nolint:errcheck
		return nil
	}
}

// StatusTitle returns a compact title suffix for the panel border
// (e.g. "200 OK  142ms").
func (m ResponseModel) StatusTitle() string {
	if !m.hasResponse {
		return ""
	}
	status := styles.StatusBadge(m.response.StatusCode)
	dur := styles.Dim.Render(fmt.Sprintf(" %dms ", m.response.DurationMs))
	return status + dur
}

// ── rendering ─────────────────────────────────────────────────────────────────

func (m ResponseModel) bodyContent() string {
	var sb strings.Builder

	if m.headersExpanded {
		sb.WriteString(styles.SectionDivider("HEADERS", m.width))
		sb.WriteByte('\n')
		for k, v := range m.response.Headers {
			sb.WriteString(styles.Highlight.Render(k) + styles.Dim.Render(": ") + styles.Normal.Render(v))
			sb.WriteByte('\n')
		}
		sb.WriteByte('\n')
	}

	sb.WriteString(styles.SectionDivider("BODY", m.width))
	sb.WriteByte('\n')
	sb.WriteString(styles.ColorizeJSON(m.response.Body))
	if m.isHTML() {
		sb.WriteByte('\n')
		sb.WriteString(styles.Dim.Render("  ctrl+o: open in browser"))
		sb.WriteByte('\n')
	}
	return sb.String()
}

// ── Bubble Tea ────────────────────────────────────────────────────────────────

func (m ResponseModel) Init() tea.Cmd { return nil }

func (m ResponseModel) Update(message tea.Msg) (ResponseModel, tea.Cmd) {
	switch message := message.(type) {
	case tea.KeyMsg:
		switch message.String() {
		case "esc", "q":
			return m, func() tea.Msg { return tuimsg.BackMsg{} }
		case "h":
			m.headersExpanded = !m.headersExpanded
			if m.ready && m.hasResponse {
				m.vp.SetContent(m.bodyContent())
			}
			return m, nil
		case "ctrl+o":
			if m.hasResponse && m.isHTML() {
				return m, openInBrowserCmd(m.response.Body)
			}
		}
	}
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(message)
	return m, cmd
}

// View renders inner panel content (no border).
func (m ResponseModel) View() string {
	if !m.ready {
		return styles.Dim.Render("  Loading…")
	}
	if !m.hasResponse {
		return "\n" + styles.Dim.Render("  send a request with ctrl+s")
	}

	code := m.response.StatusCode
	statusStr := styles.StatusBadge(code)
	durStr := styles.Dim.Render(fmt.Sprintf("  %dms", m.response.DurationMs))

	div := styles.SectionRule.Render(strings.Repeat("─", m.width))

	return statusStr + durStr + "\n" +
		div + "\n" +
		m.vp.View()
}
