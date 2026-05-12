package msg

import "github.com/shayan-shojaei/radar/pkg/models"

// EndpointSelectedMsg is sent when the user selects an endpoint from the list.
type EndpointSelectedMsg struct {
	Endpoint models.ParsedEndpoint
}

// RequestSentMsg is sent when the user triggers a request.
type RequestSentMsg struct {
	Request models.RequestData
}

// ResponseReceivedMsg carries the HTTP response back to the TUI.
type ResponseReceivedMsg struct {
	Response models.ResponseData
	Err      error
}

// BackMsg signals the active view should return to the previous one.
type BackMsg struct{}

// SessionLoadedMsg is sent when a saved session is loaded for an endpoint.
type SessionLoadedMsg struct {
	Request models.RequestData
}

// SessionLoadRequestMsg is sent by ctrl+l to trigger reloading the saved session.
type SessionLoadRequestMsg struct{}

// ClearFieldsMsg triggers clearing all input fields.
type ClearFieldsMsg struct{}

// OpenCookieManagerMsg opens the cookie manager view.
type OpenCookieManagerMsg struct{}

// CookieJarUpdatedMsg is sent when the cookie manager closes with changes.
type CookieJarUpdatedMsg struct {
	Jar        map[string]models.CookieEntry
	AuthHeader string
}

// CookiePromptAnswerMsg is sent when the user responds to an HttpOnly cookie prompt.
type CookiePromptAnswerMsg struct {
	Accept bool
	Cookie models.ReceivedCookie
}

// SpecRefreshedMsg carries the result of a background spec re-parse.
type SpecRefreshedMsg struct {
	Endpoints []models.ParsedEndpoint
	BaseURL   string
	Err       error
}
