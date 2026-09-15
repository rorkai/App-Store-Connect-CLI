package shared

import (
	"flag"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// selfLinkHost is the only host whose URLs are treated as resource self-links.
const selfLinkHost = "api.appstoreconnect.apple.com"

var selfLinkVersionSegment = regexp.MustCompile(`^v[0-9]+$`)

// ResourceIDFromValue accepts a bare App Store Connect resource ID or the
// resource's API self-link (for example the links.self value from JSON output
// or a webhook delivery) and returns the bare ID.
//
// Only URLs on api.appstoreconnect.apple.com whose path is exactly
// /v<n>/<type>/<id> are accepted. When resourceType is non-empty the <type>
// segment must match it. Values that are not http(s) URLs are returned
// trimmed and otherwise unchanged so bundle identifiers, product IDs, names,
// and other selector shapes keep working.
func ResourceIDFromValue(value, resourceType string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if !looksLikeHTTPURL(trimmed) {
		return trimmed, nil
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid resource URL: %w", err)
	}
	if !strings.EqualFold(parsed.Hostname(), selfLinkHost) {
		return "", fmt.Errorf("only App Store Connect API self-links on %s are accepted as resource IDs", selfLinkHost)
	}

	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) != 3 || !selfLinkVersionSegment.MatchString(segments[0]) || segments[1] == "" || segments[2] == "" {
		return "", fmt.Errorf("self-link must point at a single resource, /v1/<type>/<id>; relationship and related-resource paths are not accepted")
	}

	linkType, id := segments[1], segments[2]
	if resourceType != "" && linkType != resourceType {
		return "", fmt.Errorf("expected a self-link of type %s, got %s", resourceType, linkType)
	}
	return id, nil
}

func looksLikeHTTPURL(value string) bool {
	lower := strings.ToLower(value)
	return strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://")
}

// resourceIDValue is a flag.Value that stores a bare resource ID, accepting
// either the ID itself or its API self-link.
type resourceIDValue struct {
	target       *string
	resourceType string
}

// BindResourceIDFlag registers a string flag that accepts a bare App Store
// Connect resource ID or the resource's API self-link. Self-links of another
// resource type, on another host, or with a longer path are rejected during
// flag parsing so the command exits with usage code 2 before any request.
func BindResourceIDFlag(fs *flag.FlagSet, name, resourceType, usage string) *string {
	target := new(string)
	fs.Var(&resourceIDValue{target: target, resourceType: resourceType}, name, usage)
	return target
}

func (v *resourceIDValue) String() string {
	if v == nil || v.target == nil {
		return ""
	}
	return *v.target
}

func (v *resourceIDValue) Set(raw string) error {
	id, err := ResourceIDFromValue(raw, v.resourceType)
	if err != nil {
		return err
	}
	*v.target = id
	return nil
}

// Get returns the stored bare ID so the flag satisfies flag.Getter like the
// other custom flag values in this package.
func (v *resourceIDValue) Get() any {
	return v.String()
}
