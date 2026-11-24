package asc

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// ValidateNoRedirectReadContinuation binds a scoped pagination cursor to the
// collection that produced it before the next request is sent. It returns a
// canonical URL for marked contexts and leaves unmarked URLs untouched.
func (c *Client) ValidateNoRedirectReadContinuation(ctx context.Context, nextURL, expectedPath string) (string, error) {
	if !noRedirectReads(ctx) {
		return nextURL, nil
	}

	nextURL = strings.TrimSpace(nextURL)
	expectedPath = strings.TrimSpace(expectedPath)
	if nextURL == "" {
		return nextURL, nil
	}

	parsed, err := url.Parse(nextURL)
	if err != nil {
		return "", fmt.Errorf("invalid scoped pagination URL: %w", err)
	}
	if parsed.Opaque != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("rejected scoped pagination URL with opaque or fragment component")
	}

	if parsed.IsAbs() {
		base, err := url.Parse(c.apiBaseURL())
		if err != nil {
			return "", fmt.Errorf("invalid API base URL: %w", err)
		}
		if !strings.EqualFold(parsed.Scheme, "https") || !strings.EqualFold(base.Scheme, "https") || parsed.Host == "" || parsed.User != nil || !strings.EqualFold(parsed.Host, base.Host) {
			return "", fmt.Errorf("rejected scoped pagination URL origin %q", parsed.Host)
		}
		parsed.Scheme = "https"
		parsed.Host = strings.ToLower(parsed.Host)
	} else if parsed.Scheme != "" || parsed.Host != "" || !strings.HasPrefix(parsed.Path, "/") {
		return "", fmt.Errorf("rejected scoped pagination URL %q: expected root-relative or same-origin HTTPS", nextURL)
	}

	if parsed.EscapedPath() != expectedPath {
		return "", fmt.Errorf("rejected scoped pagination URL path %q: expected %q", parsed.EscapedPath(), expectedPath)
	}
	return parsed.String(), nil
}
