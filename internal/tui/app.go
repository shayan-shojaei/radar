package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/shayan-shojaei/radar/internal/config"
	"github.com/shayan-shojaei/radar/internal/openapi"
	"github.com/shayan-shojaei/radar/internal/prefs"
	"github.com/shayan-shojaei/radar/internal/requester"
	"github.com/shayan-shojaei/radar/internal/session"
	tuimsg "github.com/shayan-shojaei/radar/internal/tui/msg"
	"github.com/shayan-shojaei/radar/internal/tui/styles"
	"github.com/shayan-shojaei/radar/internal/tui/views"
	"github.com/shayan-shojaei/radar/pkg/models"
)

// ── view state ─────────────────────────────────────────────────────────────────

type viewState int

const (
	viewList viewState = iota
	viewRequest
	viewResponse
	viewLoading
	viewCookiePrompt
	viewCookieManager
)

// ── layout ────────────────────────────────────────────────────────────────────

type layout struct {
	totalW, totalH         int
	leftW                  int
	rightW                 int
	panelH                 int
	reqH                   int
	respH                  int
	listInnerW, listInnerH int
	reqInnerW, reqInnerH   int
	respInnerW, respInnerH int
}

func computeLayout(w, h int) layout {
	panelH := h - 2
	if panelH < 4 {
		panelH = 4
	}
	leftW := w * 35 / 100
	if leftW < 26 {
		leftW = 26
	}
	if leftW > 64 {
		leftW = 64
	}
	rightW := w - leftW
	reqH := panelH * 55 / 100
	if reqH < 8 {
		reqH = 8
	}
	respH := panelH - reqH
	if respH < 6 {
		respH = 6
	}
	reqH = panelH - respH
	return layout{
		totalW: w, totalH: h,
		leftW: leftW, rightW: rightW, panelH: panelH,
		reqH: reqH, respH: respH,
		listInnerW: leftW - 2, listInnerH: panelH - 2,
		reqInnerW: rightW - 2, reqInnerH: reqH - 2,
		respInnerW: rightW - 2, respInnerH: respH - 2,
	}
}

// ── model ─────────────────────────────────────────────────────────────────────

type Model struct {
	cfg            *config.Config
	endpoints      []models.ParsedEndpoint
	baseURL        string
	specURL        string
	passphrase     string
	state          viewState
	layout         layout
	listModel      views.ListModel
	requestModel   views.RequestModel
	responseModel  views.ResponseModel
	cookieManager  views.CookieManagerModel
	req            *requester.Requester
	spinner        spinner.Model
	loading        bool
	fatalErr       error
	cookieJar      map[string]models.CookieEntry
	authHeader     string
	pendingCookies []models.ReceivedCookie
	helpExpanded   bool
	prefs          *prefs.Prefs
}

func New(endpoints []models.ParsedEndpoint, baseURL, specURL string, cfg *config.Config, passphrase string) Model {
	timeout := time.Duration(cfg.Timeout) * time.Second
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = styles.SpinnerStyle

	cookieJar := make(map[string]models.CookieEntry)
	authHeader := ""
	if sess, err := session.Load(baseURL, cfg.StorageDir, passphrase); err == nil && sess != nil {
		if sess.CookieJar != nil {
			for k, v := range sess.CookieJar {
				cookieJar[k] = v
			}
		}
		authHeader = sess.AuthHeader
	}

	p, _ := prefs.Load(cfg.StorageDir)
	if p == nil {
		p = &prefs.Prefs{SummaryMode: 1, CollapsedTags: make(map[string][]string), LastBaseURLs: make(map[string]string)}
	}

	// Use the last base URL the user typed for this spec (overrides spec-defined server URL).
	if last, ok := p.LastBaseURLs[specURL]; ok && last != "" {
		baseURL = last
	}

	listM := views.NewListModel(endpoints)
	listM.ApplyPrefs(p.SummaryMode, p.CollapsedTags[specURL])

	return Model{
		cfg:           cfg,
		endpoints:     endpoints,
		baseURL:       baseURL,
		specURL:       specURL,
		passphrase:    passphrase,
		state:         viewList,
		listModel:     listM,
		responseModel: views.EmptyResponseModel(),
		req:           requester.New(timeout),
		spinner:       sp,
		cookieJar:     cookieJar,
		authHeader:    authHeader,
		prefs:         p,
	}
}

// ── init ──────────────────────────────────────────────────────────────────────

func (m Model) Init() tea.Cmd { return m.listModel.Init() }

// ── update ────────────────────────────────────────────────────────────────────

func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {

	case tea.KeyMsg:
		// Global intercepts before any view sees the key.
		switch message.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			if m.state == viewList {
				return m, tea.Quit
			}
		case "?":
			// Toggle help overlay from any non-modal state.
			if m.state != viewCookiePrompt {
				m.helpExpanded = !m.helpExpanded
				return m, nil
			}
		case "ctrl+R": // ctrl+shift+r in terminals with extended keyboard support
			m.helpExpanded = false
			return m, m.refreshSpecCmd()
		}
		// Close help overlay on any other key.
		if m.helpExpanded {
			m.helpExpanded = false
			return m, nil
		}
		return m.delegateKey(message)

	case tea.WindowSizeMsg:
		m.layout = computeLayout(message.Width, message.Height)
		m.listModel.Resize(m.layout.listInnerW, m.layout.listInnerH)
		m.requestModel.Resize(m.layout.reqInnerW, m.layout.reqInnerH)
		m.responseModel.Resize(m.layout.respInnerW, m.layout.respInnerH)
		m.cookieManager.Resize(m.layout.totalW-2, m.layout.panelH-2)
		return m, nil

	case tuimsg.EndpointSelectedMsg:
		// Auto-save the previous endpoint's state before switching.
		var saveCmd tea.Cmd
		if m.state == viewRequest {
			m, saveCmd = m.autoSaveCurrentRequest()
		}
		m.requestModel = views.NewRequestModel(message.Endpoint, m.baseURL)
		m.requestModel.Resize(m.layout.reqInnerW, m.layout.reqInnerH)
		m.responseModel = views.EmptyResponseModel()
		m.responseModel.Resize(m.layout.respInnerW, m.layout.respInnerH)
		m.state = viewRequest
		if m.baseURL != "" {
			if sess, err := session.Load(m.baseURL, m.cfg.StorageDir, m.passphrase); err == nil && sess != nil {
				key := fmt.Sprintf("%s %s", message.Endpoint.Method, message.Endpoint.Path)
				if rd, ok := sess.Requests[key]; ok {
					m.requestModel.ApplySession(rd)
				}
			}
		}
		return m, tea.Batch(saveCmd, m.requestModel.Init())

	case tuimsg.SessionLoadRequestMsg:
		if m.state == viewRequest && m.baseURL != "" {
			ep := m.requestModel.Endpoint()
			if ep != nil {
				if sess, err := session.Load(m.baseURL, m.cfg.StorageDir, m.passphrase); err == nil && sess != nil {
					key := fmt.Sprintf("%s %s", ep.Method, ep.Path)
					if rd, ok := sess.Requests[key]; ok {
						m.requestModel.ApplySession(rd)
					}
				}
			}
		}
		return m, nil

	case tuimsg.RequestSentMsg:
		rd := message.Request
		baseURL, methodPath := splitEndpointKey(rd.EndpointKey)
		if baseURL != "" {
			m.baseURL = baseURL
		}
		rd.EndpointKey = methodPath
		m.loading = true
		m.state = viewLoading
		return m, tea.Batch(m.doRequest(rd), m.spinner.Tick, m.saveRequestCmd(methodPath, rd))

	case tuimsg.ResponseReceivedMsg:
		m.loading = false
		if message.Err != nil {
			m.fatalErr = message.Err
			m.state = viewResponse
			return m, nil
		}
		m.responseModel = views.NewResponseModel(message.Response)
		m.responseModel.Resize(m.layout.respInnerW, m.layout.respInnerH)
		m.state = viewResponse
		if m.cookieJar == nil {
			m.cookieJar = make(map[string]models.CookieEntry)
		}
		var httpOnlyCookies []models.ReceivedCookie
		for _, c := range message.Response.SetCookies {
			if c.HTTPOnly {
				httpOnlyCookies = append(httpOnlyCookies, c)
			} else {
				m.cookieJar[c.Name] = models.CookieEntry{Value: c.Value, Enabled: true}
			}
		}
		m.pendingCookies = httpOnlyCookies
		if len(httpOnlyCookies) > 0 {
			m.state = viewCookiePrompt
		}
		return m, m.responseModel.Init()

	case tuimsg.SpecRefreshedMsg:
		if message.Err != nil {
			// Silent failure — leave TUI state untouched.
			return m, nil
		}
		prevEp := m.requestModel.Endpoint()
		m.endpoints = message.Endpoints
		if message.BaseURL != "" {
			m.baseURL = message.BaseURL
		}
		m.listModel = views.NewListModel(message.Endpoints)
		m.listModel.ApplyPrefs(m.prefs.SummaryMode, m.prefs.CollapsedTags[m.specURL])
		m.listModel.Resize(m.layout.listInnerW, m.layout.listInnerH)
		// If an endpoint was open, check whether it still exists.
		if prevEp != nil && (m.state == viewRequest || m.state == viewResponse) {
			found := false
			for _, ep := range message.Endpoints {
				if ep.Method == prevEp.Method && ep.Path == prevEp.Path {
					found = true
					break
				}
			}
			if !found {
				m.state = viewList
			}
			// If found, keep requestModel as-is (fields untouched).
		}
		return m, nil

	case tuimsg.CookieJarUpdatedMsg:
		m.cookieJar = message.Jar
		m.authHeader = message.AuthHeader
		m.state = viewList
		return m, m.saveJarCmd()

	case tuimsg.BackMsg:
		switch m.state {
		case viewResponse:
			m.state = viewRequest
			return m, m.requestModel.Init()
		default:
			var saveCmd tea.Cmd
			if m.state == viewRequest {
				m, saveCmd = m.autoSaveCurrentRequest()
			}
			m.state = viewList
			return m, tea.Batch(saveCmd, m.listModel.Init())
		}

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(message)
			return m, cmd
		}
	}

	return m, nil
}

func (m Model) delegateKey(km tea.KeyMsg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.state {
	case viewList:
		if km.String() == "K" {
			m.cookieManager = views.NewCookieManagerModel(m.cookieJar, m.authHeader)
			m.cookieManager.Resize(m.layout.totalW-2, m.layout.panelH-2)
			m.state = viewCookieManager
			return m, m.cookieManager.Init()
		}
		m.listModel, cmd = m.listModel.Update(km)
		switch km.String() {
		case "d", "z", " ", "C", "E", "enter":
			cmd = tea.Batch(cmd, m.savePrefsCmd())
		}
	case viewRequest:
		m.requestModel, cmd = m.requestModel.Update(km)
	case viewResponse:
		m.responseModel, cmd = m.responseModel.Update(km)
	case viewCookieManager:
		m.cookieManager, cmd = m.cookieManager.Update(km)
	case viewCookiePrompt:
		return m.handleCookiePromptKey(km)
	}
	return m, cmd
}

func (m Model) handleCookiePromptKey(km tea.KeyMsg) (Model, tea.Cmd) {
	if len(m.pendingCookies) == 0 {
		m.state = viewResponse
		return m, nil
	}
	cookie := m.pendingCookies[0]
	switch km.String() {
	case "y", "enter":
		if m.cookieJar == nil {
			m.cookieJar = make(map[string]models.CookieEntry)
		}
		m.cookieJar[cookie.Name] = models.CookieEntry{Value: cookie.Value, Enabled: true, HTTPOnly: true}
		m.pendingCookies = m.pendingCookies[1:]
	case "n", "esc":
		m.pendingCookies = m.pendingCookies[1:]
	}
	if len(m.pendingCookies) == 0 {
		m.state = viewResponse
		return m, m.saveJarCmd()
	}
	return m, nil
}

// ── view ──────────────────────────────────────────────────────────────────────

func (m Model) View() string {
	if m.layout.totalW == 0 {
		return ""
	}
	if m.fatalErr != nil {
		return m.renderFatalError()
	}
	if m.helpExpanded {
		return m.renderHelpOverlay()
	}
	if m.state == viewCookiePrompt {
		return m.renderCookiePrompt()
	}
	if m.state == viewCookieManager {
		content := m.cookieManager.View()
		panel := styles.RenderPanel("Cookies & Auth", content, true, m.layout.totalW, m.layout.panelH)
		return lipgloss.JoinVertical(lipgloss.Left, panel, m.keyBar(), m.statusBar())
	}

	lyt := m.layout
	listActive := m.state == viewList
	leftPanel := styles.RenderPanel("Endpoints", m.listModel.View(), listActive, lyt.leftW, lyt.panelH)

	reqActive := m.state == viewRequest
	ep := m.requestModel.Endpoint()
	reqTitle := "Request"
	if ep != nil {
		reqTitle = "Request  " + styles.MethodBadge(ep.Method) + " " + styles.Normal.Render(ep.Path)
	}
	reqPanel := styles.RenderPanel(reqTitle, m.requestModel.View(), reqActive, lyt.rightW, lyt.reqH)

	respActive := m.state == viewResponse
	respTitle := "Response"
	if m.loading {
		respTitle = "Response  " + m.spinner.View() + styles.Dim.Render(" sending…")
	} else if m.responseModel.HasResponse() {
		respTitle = "Response " + m.responseModel.StatusTitle()
	}
	var respContent string
	if m.loading {
		indent := strings.Repeat(" ", (lyt.respInnerW-20)/2)
		respContent = "\n" + indent + m.spinner.View() + styles.Dim.Render(" sending request…")
	} else {
		respContent = m.responseModel.View()
	}
	respPanel := styles.RenderPanel(respTitle, respContent, respActive, lyt.rightW, lyt.respH)

	rightCol := lipgloss.JoinVertical(lipgloss.Left, reqPanel, respPanel)
	mainArea := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightCol)
	return lipgloss.JoinVertical(lipgloss.Left, mainArea, m.keyBar(), m.statusBar())
}

func (m Model) renderFatalError() string {
	msg := styles.ErrorBorder.Render(
		styles.ErrorTitle.Render(" Error ") + "\n\n" +
			styles.ErrorMsg.Render(m.fatalErr.Error()) + "\n\n" +
			styles.ErrorHint.Render("[q] quit"),
	)
	return lipgloss.Place(m.layout.totalW, m.layout.totalH, lipgloss.Center, lipgloss.Center, msg)
}

func (m Model) renderCookiePrompt() string {
	if len(m.pendingCookies) == 0 {
		return ""
	}
	c := m.pendingCookies[0]
	val := c.Value
	if len([]rune(val)) > 40 {
		val = string([]rune(val)[:39]) + "…"
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.ColorMethodPOST).
		Padding(1, 3).
		Render(
			styles.Highlight.Render("HttpOnly Cookie Received") + "\n\n" +
				styles.InputLabel.Render(fmt.Sprintf("%-8s", "Name:")) + styles.Normal.Render(c.Name) + "\n" +
				styles.InputLabel.Render(fmt.Sprintf("%-8s", "Value:")) + styles.Dim.Render(val) + "\n\n" +
				styles.Dim.Render("Persist this cookie?  ") +
				styles.KeyBarKey.Render("[y]") + styles.Dim.Render(" yes   ") +
				styles.KeyBarKey.Render("[n]") + styles.Dim.Render(" no"),
		)
	return lipgloss.Place(m.layout.totalW, m.layout.totalH, lipgloss.Center, lipgloss.Center, box)
}

func (m Model) renderHelpOverlay() string {
	type row struct{ key, action string }
	var rows []row

	switch m.state {
	case viewList:
		rows = []row{
			{"↑↓ / j/k", "navigate endpoints"},
			{"enter", "select endpoint / toggle tag"},
			{"z / space", "toggle tag collapse"},
			{"C / E", "collapse all / expand all tags"},
			{"d", "cycle summary detail level"},
			{"/", "filter endpoints"},
			{"K", "cookie jar & auth header"},
			{"ctrl+R", "refresh spec from URL"},
			{"?", "close this help"},
			{"q", "quit"},
		}
	case viewRequest:
		rows = []row{
			{"tab / shift+tab", "next / previous field"},
			{"ctrl+s", "send request"},
			{"ctrl+n / ctrl+p", "cycle content-type"},
			{"ctrl+t", "toggle body mode (raw ↔ fields)"},
			{"ctrl+l", "reload saved session"},
			{"ctrl+r", "clear all fields"},
			{"ctrl+R", "refresh spec from URL"},
			{"?", "close this help"},
			{"esc", "back to list"},
		}
	case viewResponse:
		rows = []row{
			{"↑↓", "scroll response body"},
			{"h", "toggle response headers"},
			{"ctrl+R", "refresh spec from URL"},
			{"?", "close this help"},
			{"esc / q", "back to request editor"},
		}
	case viewCookieManager:
		rows = []row{
			{"↑↓ / j/k", "navigate cookies"},
			{"space", "toggle cookie enabled/disabled"},
			{"a", "add a new cookie"},
			{"d", "delete selected cookie"},
			{"e / enter", "edit Authorization header"},
			{"?", "close this help"},
			{"esc", "save changes and back"},
		}
	default:
		rows = []row{{"?", "close this help"}, {"ctrl+c", "quit"}}
	}

	// Build a two-column table: key (right-aligned) · separator · action.
	maxKeyW := 0
	for _, r := range rows {
		if len(r.key) > maxKeyW {
			maxKeyW = len(r.key)
		}
	}
	var sb strings.Builder
	for _, r := range rows {
		keyStr := styles.KeyBarKey.Render(fmt.Sprintf("%*s", maxKeyW, r.key))
		sb.WriteString("  " + keyStr + "  " + styles.Dim.Render(r.action) + "\n")
	}

	stateLabel := map[viewState]string{
		viewList:          "LIST",
		viewRequest:       "REQUEST EDITOR",
		viewResponse:      "RESPONSE",
		viewCookieManager: "COOKIE MANAGER",
	}[m.state]

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.ColorBorderActive).
		Padding(1, 2).
		Render(
			styles.Highlight.Render("  Key Bindings — "+stateLabel) + "\n\n" +
				strings.TrimRight(sb.String(), "\n"),
		)

	return lipgloss.Place(m.layout.totalW, m.layout.totalH, lipgloss.Center, lipgloss.Center, box)
}

// ── bars ──────────────────────────────────────────────────────────────────────

func (m Model) statusBar() string {
	w := m.layout.totalW
	var modeStr string
	switch m.state {
	case viewList:
		modeStr = styles.StatusBarMode.Background(styles.ColorStatusBarBg).Foreground(styles.ColorMethodGET).Render(" BROWSING ")
	case viewRequest:
		modeStr = styles.StatusBarMode.Background(styles.ColorStatusBarBg).Foreground(styles.ColorMethodPUT).Render(" EDITING ")
	case viewLoading:
		modeStr = styles.StatusBarMode.Background(styles.ColorStatusBarBg).Foreground(styles.ColorMethodPOST).Render(" LOADING ")
	case viewResponse:
		modeStr = styles.StatusBarMode.Background(styles.ColorStatusBarBg).Foreground(styles.ColorStatusSuccess).Render(" RESPONSE ")
	case viewCookieManager:
		modeStr = styles.StatusBarMode.Background(styles.ColorStatusBarBg).Foreground(styles.ColorMethodPUT).Render(" COOKIES ")
	case viewCookiePrompt:
		modeStr = styles.StatusBarMode.Background(styles.ColorStatusBarBg).Foreground(styles.ColorMethodPOST).Render(" COOKIE PROMPT ")
	}
	specStr := styles.StatusBar.Render(" " + truncStr(m.specURL, w-20) + " ")
	modeW := lipgloss.Width(modeStr)
	specW := lipgloss.Width(specStr)
	padW := w - modeW - specW
	if padW < 0 {
		padW = 0
	}
	return modeStr + styles.StatusBar.Render(strings.Repeat(" ", padW)) + specStr
}

// keyBar renders a compact single-line bar. Primary hints only; "?" opens full help.
func (m Model) keyBar() string {
	w := m.layout.totalW

	var primary []string
	switch m.state {
	case viewList:
		primary = []string{
			styles.KeyBinding("↑↓", "navigate"),
			styles.KeyBinding("enter", "select"),
			styles.KeyBinding("/", "filter"),
			styles.KeyBinding("K", "cookies"),
		}
	case viewRequest:
		primary = []string{
			styles.KeyBinding("tab", "next field"),
			styles.KeyBinding("ctrl+s", "send"),
			styles.KeyBinding("ctrl+t", "body mode"),
			styles.KeyBinding("esc", "back"),
		}
	case viewLoading:
		primary = []string{styles.KeyBinding("ctrl+c", "cancel")}
	case viewResponse:
		primary = []string{
			styles.KeyBinding("↑↓", "scroll"),
			styles.KeyBinding("h", "headers"),
			styles.KeyBinding("esc", "back"),
		}
	case viewCookieManager:
		primary = []string{
			styles.KeyBinding("space", "toggle"),
			styles.KeyBinding("a", "add"),
			styles.KeyBinding("esc", "save & back"),
		}
	case viewCookiePrompt:
		primary = []string{styles.KeyBinding("y", "accept"), styles.KeyBinding("n/esc", "discard")}
	}

	// Always append the help hint.
	if m.state != viewCookiePrompt {
		primary = append(primary, styles.KeyBinding("?", "help"))
	}

	sep := styles.Dim.Render("  ·  ")
	bar := " " + strings.Join(primary, sep)
	barW := lipgloss.Width(bar)
	if barW < w {
		bar += styles.KeyBar.Render(strings.Repeat(" ", w-barW))
	}
	return styles.KeyBar.Render(bar)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func (m Model) refreshSpecCmd() tea.Cmd {
	specURL := m.specURL
	return func() tea.Msg {
		eps, baseURL, err := openapi.Parse(specURL)
		return tuimsg.SpecRefreshedMsg{Endpoints: eps, BaseURL: baseURL, Err: err}
	}
}

func (m Model) doRequest(rd models.RequestData) tea.Cmd {
	baseURL := m.baseURL
	jar := m.cookieJar
	authHeader := m.authHeader
	return func() tea.Msg {
		for name, entry := range jar {
			if entry.Enabled {
				if rd.Cookies == nil {
					rd.Cookies = make(map[string]string)
				}
				if _, exists := rd.Cookies[name]; !exists {
					rd.Cookies[name] = entry.Value
				}
			}
		}
		if authHeader != "" {
			if rd.Headers == nil {
				rd.Headers = make(map[string]string)
			}
			if _, exists := rd.Headers["authorization"]; !exists {
				rd.Headers["authorization"] = authHeader
			}
		}
		resp, err := m.req.Do(baseURL, rd)
		return tuimsg.ResponseReceivedMsg{Response: resp, Err: err}
	}
}

func (m Model) saveRequestCmd(methodPath string, rd models.RequestData) tea.Cmd {
	baseURL := m.baseURL
	specURL := m.specURL
	storageDir := m.cfg.StorageDir
	passphrase := m.passphrase
	jar := m.cookieJar
	authHeader := m.authHeader
	return func() tea.Msg {
		sess, _ := session.Load(baseURL, storageDir, passphrase)
		if sess == nil {
			sess = &models.Session{BaseURL: baseURL, Requests: make(map[string]models.RequestData), CookieJar: make(map[string]models.CookieEntry)}
		}
		if sess.Requests == nil {
			sess.Requests = make(map[string]models.RequestData)
		}
		if sess.CookieJar == nil {
			sess.CookieJar = make(map[string]models.CookieEntry)
		}
		sess.Requests[methodPath] = rd
		for k, v := range jar {
			sess.CookieJar[k] = v
		}
		sess.AuthHeader = authHeader
		session.Save(sess, storageDir, passphrase) //nolint:errcheck
		// Persist the last-used base URL for this spec so it survives restarts.
		if specURL != "" && baseURL != "" {
			if p, _ := prefs.Load(storageDir); p != nil {
				if p.LastBaseURLs == nil {
					p.LastBaseURLs = make(map[string]string)
				}
				p.LastBaseURLs[specURL] = baseURL
				prefs.Save(p, storageDir) //nolint:errcheck
			}
		}
		return nil
	}
}

// autoSaveCurrentRequest saves the request editor's current state to the session
// file without requiring the user to send the request. Used on navigate-away.
func (m Model) autoSaveCurrentRequest() (Model, tea.Cmd) {
	ep := m.requestModel.Endpoint()
	if ep == nil {
		return m, nil
	}
	rd := m.requestModel.CurrentRequestData()
	baseURL, methodPath := splitEndpointKey(rd.EndpointKey)
	if baseURL == "" {
		return m, nil
	}
	m.baseURL = baseURL
	return m, m.saveRequestCmd(methodPath, rd)
}

func (m Model) savePrefsCmd() tea.Cmd {
	p := &prefs.Prefs{
		SummaryMode:   m.listModel.GetSummaryMode(),
		CollapsedTags: make(map[string][]string, len(m.prefs.CollapsedTags)+1),
	}
	for k, v := range m.prefs.CollapsedTags {
		p.CollapsedTags[k] = v
	}
	p.CollapsedTags[m.specURL] = m.listModel.GetCollapsedTagNames()
	// Update in-memory prefs so subsequent saves don't overwrite the latest state.
	m.prefs.SummaryMode = p.SummaryMode
	m.prefs.CollapsedTags = p.CollapsedTags
	storageDir := m.cfg.StorageDir
	return func() tea.Msg {
		prefs.Save(p, storageDir) //nolint:errcheck
		return nil
	}
}

func (m Model) saveJarCmd() tea.Cmd {
	baseURL := m.baseURL
	storageDir := m.cfg.StorageDir
	passphrase := m.passphrase
	jar := m.cookieJar
	authHeader := m.authHeader
	return func() tea.Msg {
		sess, _ := session.Load(baseURL, storageDir, passphrase)
		if sess == nil {
			sess = &models.Session{BaseURL: baseURL, Requests: make(map[string]models.RequestData), CookieJar: make(map[string]models.CookieEntry)}
		}
		if sess.CookieJar == nil {
			sess.CookieJar = make(map[string]models.CookieEntry)
		}
		for k, v := range jar {
			sess.CookieJar[k] = v
		}
		sess.AuthHeader = authHeader
		session.Save(sess, storageDir, passphrase) //nolint:errcheck
		return nil
	}
}

func truncStr(s string, maxW int) string {
	runes := []rune(s)
	if len(runes) <= maxW {
		return s
	}
	if maxW <= 1 {
		return string(runes[:maxW])
	}
	return string(runes[:maxW-1]) + "…"
}

func splitEndpointKey(key string) (baseURL, methodPath string) {
	idx := strings.Index(key, "|")
	if idx < 0 {
		return "", key
	}
	return key[:idx], key[idx+1:]
}
