// Package client is a thin, generic HTTP client for the Jira Cloud REST API
// v3, reached through Atlassian's OAuth gateway
// (api.atlassian.com/ex/jira/{cloudId}/rest/api/3/...). It knows nothing
// about specific Jira resources — every endpoint is reachable via Do.
package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const maxRetries = 3

type Client struct {
	HTTP    *http.Client
	BaseURL string
}

// New builds a client for a resolved Jira Cloud id, using hc (expected to
// inject a fresh OAuth access token per request, e.g. via oauth2.NewClient).
func New(hc *http.Client, cloudID string) *Client {
	return &Client{
		HTTP:    hc,
		BaseURL: fmt.Sprintf("https://api.atlassian.com/ex/jira/%s/rest/api/3", cloudID),
	}
}

type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// Do issues a single Jira REST API request. path must start with "/" and is
// relative to /rest/api/3 (e.g. "/issue/PROJ-1"). It retries on 429 and
// transient 5xx responses with backoff, honoring Retry-After when present.
func (c *Client) Do(ctx context.Context, method, path string, query url.Values, body []byte) (*Response, error) {
	if !strings.HasPrefix(path, "/") {
		return nil, fmt.Errorf("path must start with \"/\", got %q", path)
	}
	full := c.BaseURL + path
	if len(query) > 0 {
		full += "?" + query.Encode()
	}

	var lastErr error
	var wait time.Duration
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		req, err := http.NewRequestWithContext(ctx, method, full, bytesReader(body))
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		if len(body) > 0 {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.HTTP.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request %s %s: %w", method, path, err)
			wait = backoff(attempt + 1)
			continue
		}
		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("read response body: %w", err)
			wait = backoff(attempt + 1)
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests || (resp.StatusCode >= 500 && resp.StatusCode < 600) {
			lastErr = fmt.Errorf("%s %s: %s: %s", method, path, resp.Status, string(respBody))
			if attempt < maxRetries {
				if ra, ok := parseRetryAfter(resp.Header); ok {
					wait = ra
				} else {
					wait = backoff(attempt + 1)
				}
				continue
			}
		}

		return &Response{StatusCode: resp.StatusCode, Header: resp.Header, Body: respBody}, nil
	}
	return nil, lastErr
}

func backoff(attempt int) time.Duration {
	return time.Duration(1<<uint(attempt)) * 500 * time.Millisecond
}

func bytesReader(b []byte) io.Reader {
	if len(b) == 0 {
		return nil
	}
	return strings.NewReader(string(b))
}

// parseRetryAfter reads a Retry-After header value (seconds) if present.
func parseRetryAfter(h http.Header) (time.Duration, bool) {
	v := h.Get("Retry-After")
	if v == "" {
		return 0, false
	}
	secs, err := strconv.Atoi(v)
	if err != nil {
		return 0, false
	}
	return time.Duration(secs) * time.Second, true
}
