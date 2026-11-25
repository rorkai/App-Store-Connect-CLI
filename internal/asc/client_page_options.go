package asc

import (
	"fmt"
	"net/url"
	"strings"
)

type pager interface{ page() *listQuery }

func (q *listQuery) page() *listQuery { return q }

// Limit returns an option O that sets the page size. Values below 1 keep the
// endpoint's default. Write it as Limit[DevicesOption](n); the query type is inferred.
func Limit[O ~func(Q), Q pager](limit int) O {
	return func(q Q) {
		if limit > 0 {
			q.page().limit = limit
		}
	}
}

// NextURL returns an option O that fetches the given links.next URL instead
// of the first page. The URL is trimmed; a blank URL is ignored.
func NextURL[O ~func(Q), Q pager](next string) O {
	next = strings.TrimSpace(next)
	return func(q Q) {
		if next != "" {
			q.page().nextURL = next
		}
	}
}

// ValidateVersionLocalizationNextURL validates a pagination cursor and binds it to the
// resource path that produced it.
func ValidateVersionLocalizationNextURL(nextURL, expectedPath string) error {
	nextURL = strings.TrimSpace(nextURL)
	expectedPath = strings.TrimSpace(expectedPath)
	if nextURL == "" {
		return nil
	}
	if err := validateNextURL(nextURL); err != nil {
		return err
	}
	parsed, err := url.Parse(nextURL)
	if err != nil {
		return fmt.Errorf("invalid pagination URL: %w", err)
	}
	if parsed.Path != expectedPath {
		return fmt.Errorf("pagination URL path %q does not match expected resource path %q", parsed.Path, expectedPath)
	}
	return nil
}
