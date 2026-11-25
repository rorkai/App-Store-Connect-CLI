package shared

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type scopedContinuationClient interface {
	ValidateNoRedirectReadContinuation(ctx context.Context, nextURL, expectedPath string) (string, error)
}

// BindCollectionNextURL retains collection scope and returns a path on the selected client origin.
func BindCollectionNextURL(ctx context.Context, client scopedContinuationClient, next, collectionPath string, requiredQuery url.Values) (string, error) {
	next, err := client.ValidateNoRedirectReadContinuation(ctx, next, collectionPath)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(next)
	if err != nil || parsed == nil || parsed.User != nil || parsed.Fragment != "" || parsed.Opaque != "" {
		return "", fmt.Errorf("invalid collection pagination URL")
	}
	if parsed.IsAbs() {
		if parsed.Scheme != "https" || !asc.IsAPIHost(parsed.Host) {
			return "", fmt.Errorf("collection pagination URL has an unexpected origin")
		}
	} else if parsed.Host != "" || !strings.HasPrefix(next, "/") {
		return "", fmt.Errorf("collection pagination URL must retain its collection path")
	}
	if parsed.Path != collectionPath || parsed.EscapedPath() != collectionPath {
		return "", fmt.Errorf("collection pagination URL changed the collection path")
	}
	values, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		return "", fmt.Errorf("invalid collection pagination query")
	}
	for key, entries := range values {
		if (key != "cursor" && key != "limit" && !requiredQuery.Has(key)) || len(entries) != 1 {
			return "", fmt.Errorf("collection pagination URL changed the collection query")
		}
	}
	for key, expected := range requiredQuery {
		actual := values[key]
		if len(expected) != 1 || len(actual) != 1 || actual[0] != expected[0] {
			return "", fmt.Errorf("collection pagination URL changed a required scope filter")
		}
	}
	return parsed.RequestURI(), nil
}
