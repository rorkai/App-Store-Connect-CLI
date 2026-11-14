package shared

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// ErrBundleIDNotFound reports that no bundle ID has the requested identifier.
var ErrBundleIDNotFound = errors.New("bundle ID not found")

// FindBundleID returns the bundle ID whose identifier equals identifier.
// App Store Connect's filter[identifier] is a prefix/substring match, so the
// filtered list is read in full and the exact identifier is selected instead
// of trusting the first result.
func FindBundleID(ctx context.Context, client *asc.Client, identifier string) (*asc.BundleIDResponse, error) {
	want := strings.TrimSpace(identifier)
	next := ""
	page := 1
	seenNext := make(map[string]struct{})
	for {
		opts := []asc.BundleIDsOption{asc.WithBundleIDsFilterIdentifier(identifier), asc.Limit[asc.BundleIDsOption](200)}
		if next != "" {
			opts = []asc.BundleIDsOption{asc.NextURL[asc.BundleIDsOption](next)}
		}
		resp, err := client.GetBundleIDs(ctx, opts...)
		if err != nil {
			return nil, err
		}
		for _, item := range resp.Data {
			if strings.EqualFold(strings.TrimSpace(item.Attributes.Identifier), want) {
				return &asc.BundleIDResponse{Data: item}, nil
			}
		}
		if strings.TrimSpace(resp.Links.Next) == "" {
			return nil, fmt.Errorf("%w: %s", ErrBundleIDNotFound, identifier)
		}
		if _, repeated := seenNext[resp.Links.Next]; repeated {
			return nil, fmt.Errorf("list bundle IDs page %d: %w", page+1, asc.ErrRepeatedPaginationURL)
		}
		seenNext[resp.Links.Next] = struct{}{}
		page++
		next = resp.Links.Next
	}
}
