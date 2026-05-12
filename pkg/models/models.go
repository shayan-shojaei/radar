package models

// Parameter represents a single parameter for an endpoint.
type Parameter struct {
	Name     string
	In       string // path, query, header, cookie
	Required bool
	Schema   string // JSON schema as string
}

// BodyField represents a single field in a request body schema.
type BodyField struct {
	Name     string
	Type     string // "string", "integer", "boolean", "object", "array", ""
	Required bool
	Example  string
}

// ParsedEndpoint holds the normalized representation of a single API endpoint.
type ParsedEndpoint struct {
	Method      string
	Path        string
	Summary     string
	Description string
	Tags        []string
	Parameters  []Parameter
	HasBody     bool
	BodyFields  []BodyField // schema properties of the request body, if available
}

// RequestData holds everything needed to replay a request.
type RequestData struct {
	EndpointKey string            // "METHOD /path"
	Headers     map[string]string
	QueryParams map[string]string
	PathParams  map[string]string
	Cookies     map[string]string
	Body        string
	BodyMode    string // "raw" or "fields"; empty = raw
}

// ReceivedCookie is parsed from a Set-Cookie response header.
type ReceivedCookie struct {
	Name     string
	Value    string
	HTTPOnly bool
	Domain   string
	Path     string
}

// CookieEntry is a persisted cookie in the jar.
type CookieEntry struct {
	Value    string
	Enabled  bool
	HTTPOnly bool
}

// ResponseData holds the result of an HTTP request.
type ResponseData struct {
	StatusCode int
	Headers    map[string]string
	Body       string
	DurationMs int64
	SetCookies []ReceivedCookie // cookies received in the response
}

// Session holds all saved state for a given API base URL.
type Session struct {
	BaseURL    string
	Requests   map[string]RequestData  // keyed by "METHOD /path"
	CookieJar  map[string]CookieEntry  // name → entry
	AuthHeader string                  // e.g. "Bearer abc123"
}
