package asc

import (
	"context"
	"fmt"
	"net/url"
	"slices"
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

// ValidateNoRedirectReadContinuationQuery applies the same collection and
// origin checks as ValidateNoRedirectReadContinuation, then preserves any
// scope filters present on the continuation. App Store Connect may return
// opaque cursors without repeating the original filters (2026-11-25), so
// absent expected keys are accepted; a repeated key must retain its initial
// value.
func (c *Client) ValidateNoRedirectReadContinuationQuery(ctx context.Context, nextURL, expectedPath string, expectedQuery url.Values) (string, error) {
	canonical, err := c.ValidateNoRedirectReadContinuation(ctx, nextURL, expectedPath)
	if err != nil || !noRedirectReads(ctx) || len(expectedQuery) == 0 {
		return canonical, err
	}

	parsed, err := url.Parse(canonical)
	if err != nil {
		return "", fmt.Errorf("invalid scoped pagination URL: %w", err)
	}
	actualQuery, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		return "", fmt.Errorf("rejected scoped pagination URL query: invalid encoding")
	}
	for key, expectedValues := range expectedQuery {
		actualValues, present := actualQuery[key]
		if !present || slices.Equal(actualValues, expectedValues) {
			continue
		}
		return "", fmt.Errorf("rejected scoped pagination URL query %q for %q: expected %q, got %q", key, expectedPath, expectedValues, actualValues)
	}
	return canonical, nil
}
