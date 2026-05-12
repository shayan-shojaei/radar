package requester

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/shayan-shojaei/radar/pkg/models"
)

// Requester executes HTTP requests built from RequestData.
type Requester struct {
	client *http.Client
}

// New creates a Requester with the given timeout.
func New(timeout time.Duration) *Requester {
	return &Requester{
		client: &http.Client{Timeout: timeout},
	}
}

// Do builds and fires an HTTP request from rd, returning the response.
func (r *Requester) Do(baseURL string, rd models.RequestData) (models.ResponseData, error) {
	fullPath := buildPath(rd.EndpointKey, rd.PathParams)
	reqURL, err := buildURL(baseURL, fullPath, rd.QueryParams)
	if err != nil {
		return models.ResponseData{}, fmt.Errorf("requester: build URL: %w", err)
	}

	method := extractMethod(rd.EndpointKey)
	var bodyReader io.Reader
	if rd.Body != "" {
		bodyReader = strings.NewReader(rd.Body)
	}

	req, err := http.NewRequest(method, reqURL, bodyReader)
	if err != nil {
		return models.ResponseData{}, fmt.Errorf("requester: create request: %w", err)
	}

	for k, v := range rd.Headers {
		req.Header.Set(k, v)
	}
	for k, v := range rd.Cookies {
		req.AddCookie(&http.Cookie{Name: k, Value: v})
	}

	start := time.Now()
	resp, err := r.client.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		return models.ResponseData{}, fmt.Errorf("requester: execute request: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return models.ResponseData{}, fmt.Errorf("requester: read body: %w", err)
	}

	headers := make(map[string]string, len(resp.Header))
	for k := range resp.Header {
		headers[k] = resp.Header.Get(k)
	}

	var setCookies []models.ReceivedCookie
	for _, c := range resp.Cookies() {
		setCookies = append(setCookies, models.ReceivedCookie{
			Name:     c.Name,
			Value:    c.Value,
			HTTPOnly: c.HttpOnly,
			Domain:   c.Domain,
			Path:     c.Path,
		})
	}

	return models.ResponseData{
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       string(rawBody),
		DurationMs: elapsed.Milliseconds(),
		SetCookies: setCookies,
	}, nil
}

// buildPath substitutes path parameters into the path portion of an endpoint key.
func buildPath(endpointKey string, pathParams map[string]string) string {
	// endpointKey format: "METHOD /path"
	parts := strings.SplitN(endpointKey, " ", 2)
	if len(parts) != 2 {
		return ""
	}
	path := parts[1]
	for k, v := range pathParams {
		path = strings.ReplaceAll(path, "{"+k+"}", url.PathEscape(v))
	}
	return path
}

// buildURL assembles the full request URL with query parameters.
func buildURL(baseURL, path string, queryParams map[string]string) (string, error) {
	base := strings.TrimRight(baseURL, "/")
	u, err := url.Parse(base + path)
	if err != nil {
		return "", fmt.Errorf("parse URL: %w", err)
	}
	if len(queryParams) > 0 {
		q := u.Query()
		for k, v := range queryParams {
			q.Set(k, v)
		}
		u.RawQuery = q.Encode()
	}
	return u.String(), nil
}

// extractMethod returns the HTTP method from an endpoint key ("METHOD /path").
func extractMethod(endpointKey string) string {
	parts := strings.SplitN(endpointKey, " ", 2)
	if len(parts) == 0 {
		return "GET"
	}
	return parts[0]
}
