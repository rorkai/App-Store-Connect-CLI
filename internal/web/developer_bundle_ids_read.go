package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

const (
	developerBundleIDsListLimit     = "1000"
	developerBundleIDsListSort      = "name"
	developerBundleIDsListPlatforms = "IOS,MACOS"
)

var developerBundleIDDetailIncludes = []string{
	"bundleIdCapabilities",
	"bundleIdCapabilities.capability",
	"bundleIdCapabilities.associatedBundleIds",
	"bundleIdCapabilities.appGroups",
	"bundleIdCapabilities.merchantIds",
	"bundleIdCapabilities.cloudContainers",
	"bundleIdCapabilities.certificates",
	"bundleIdCapabilities.appConsentBundleId",
	"bundleIdCapabilities.macBundleId",
	"bundleIdCapabilities.relatedAppConsentBundleIds",
	"bundleIdCapabilities.parentBundleId",
	"bundleIdCapabilities.mediaSharingProtocolIds",
}

const developerBundleIDDetailFields = "name,identifier,platform,seedId,wildcard,~permissions.delete,~permissions.edit"

// DeveloperBundleID is one JSON:API Bundle ID resource returned by the
// Developer Portal web session. The Portal adds fields that are not present in
// the public App Store Connect Bundle ID resource, so attributes and
// relationships intentionally remain open-ended while preserving Apple's
// response shape for JSON output.
type DeveloperBundleID struct {
	ID            string                                   `json:"id"`
	Type          string                                   `json:"type"`
	Attributes    map[string]any                           `json:"attributes,omitempty"`
	Relationships map[string]DeveloperBundleIDRelationship `json:"relationships,omitempty"`
	Links         map[string]any                           `json:"links,omitempty"`
}

// DeveloperBundleIDRelationship preserves the relationship data and links
// returned by Apple's JSON:API response. Data is raw because it can be either
// a to-one object, a to-many array, or null depending on the relationship.
type DeveloperBundleIDRelationship struct {
	Data  json.RawMessage `json:"data,omitempty"`
	Links map[string]any  `json:"links,omitempty"`
	Meta  map[string]any  `json:"meta,omitempty"`
}

// DeveloperBundleIDsListResult is the read-only collection response for the
// Developer Portal Bundle ID endpoint.
type DeveloperBundleIDsListResult struct {
	Data     []DeveloperBundleID `json:"data"`
	Included []DeveloperBundleID `json:"included,omitempty"`
	Links    map[string]any      `json:"links,omitempty"`
	Meta     map[string]any      `json:"meta,omitempty"`
	// Raw preserves the complete JSON:API envelope returned by Apple. It is
	// omitted from the encoded shape because MarshalJSON emits it verbatim when
	// available, retaining unknown top-level members and explicitly empty ones.
	Raw json.RawMessage `json:"-"`
}

var _ asc.PaginatedResponse = (*DeveloperBundleIDsListResult)(nil)

// GetLinks exposes the collection continuation link to shared output and
// pagination helpers while retaining the open-ended JSON:API links map for
// raw JSON output.
func (r *DeveloperBundleIDsListResult) GetLinks() *asc.Links {
	if r == nil {
		return nil
	}
	return &asc.Links{
		Self:  developerBundleIDLinkString(r.Links, "self"),
		First: developerBundleIDLinkString(r.Links, "first"),
		Next:  developerBundleIDLinkString(r.Links, "next"),
		Prev:  developerBundleIDLinkString(r.Links, "prev"),
	}
}

// GetData exposes the collection items to shared pagination diagnostics.
func (r *DeveloperBundleIDsListResult) GetData() any {
	if r == nil {
		return nil
	}
	return r.Data
}

// GetMeta exposes the parsed metadata to shared pagination diagnostics. The
// original response remains authoritative for JSON output through Raw.
func (r *DeveloperBundleIDsListResult) GetMeta() json.RawMessage {
	if r == nil || r.Meta == nil {
		return nil
	}
	encoded, err := json.Marshal(r.Meta)
	if err != nil {
		return nil
	}
	return encoded
}

func developerBundleIDLinkString(links map[string]any, key string) string {
	switch value := links[key].(type) {
	case string:
		return value
	case map[string]any:
		href, _ := value["href"].(string)
		return href
	default:
		return ""
	}
}

// MarshalJSON preserves Apple's full collection envelope for JSON output.
// Table renderers use the parsed fields above, while JSON callers receive the
// original response rather than a lossy re-encoding of the known fields.
func (r DeveloperBundleIDsListResult) MarshalJSON() ([]byte, error) {
	if len(r.Raw) > 0 {
		return append([]byte(nil), r.Raw...), nil
	}
	type resultWithoutMethods DeveloperBundleIDsListResult
	return json.Marshal(resultWithoutMethods(r))
}

// DeveloperBundleIDGetResult is the read-only single-resource response for
// the Developer Portal Bundle ID endpoint.
type DeveloperBundleIDGetResult struct {
	Data     DeveloperBundleID   `json:"data"`
	Included []DeveloperBundleID `json:"included,omitempty"`
	Links    map[string]any      `json:"links,omitempty"`
	Meta     map[string]any      `json:"meta,omitempty"`
	// Raw preserves the complete JSON:API envelope returned by Apple. It is
	// omitted from the encoded shape because MarshalJSON emits it verbatim when
	// available, retaining unknown top-level members and explicitly empty ones.
	Raw json.RawMessage `json:"-"`
}

// MarshalJSON preserves Apple's full single-resource envelope for JSON output.
func (r DeveloperBundleIDGetResult) MarshalJSON() ([]byte, error) {
	if len(r.Raw) > 0 {
		return append([]byte(nil), r.Raw...), nil
	}
	type resultWithoutMethods DeveloperBundleIDGetResult
	return json.Marshal(resultWithoutMethods(r))
}

// DeveloperBundleIDListResult is retained as a singular spelling alias for
// callers that use the resource name in the result type.
type DeveloperBundleIDListResult = DeveloperBundleIDsListResult

// DeveloperBundleIDResponse is an alias matching the public API naming style.
type DeveloperBundleIDResponse = DeveloperBundleIDGetResult

// ListDeveloperBundleIDs reads the first collection returned by Apple's
// Developer Portal Bundle ID service. Apple currently accepts a 1000-resource
// request for this web surface; links.next is returned to the caller when the
// service provides one, but this first slice deliberately does not claim to
// paginate or follow it.
func (c *Client) ListDeveloperBundleIDs(ctx context.Context) (*DeveloperBundleIDsListResult, error) {
	if err := c.ensureDeveloperPortalSession(ctx); err != nil {
		return nil, err
	}

	query := url.Values{}
	query.Set("limit", developerBundleIDsListLimit)
	query.Set("sort", developerBundleIDsListSort)
	query.Set("filter[platform]", developerBundleIDsListPlatforms)
	body, err := c.doDeveloperPortalProxyRead(ctx, "/bundleIds", query, developerPortalHeaders(""))
	if err != nil {
		return nil, err
	}

	result, err := parseDeveloperBundleIDsListResponse(body)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// GetDeveloperBundleID reads one opaque Bundle ID resource and its requested
// capability graph through the Developer Portal web session.
func (c *Client) GetDeveloperBundleID(ctx context.Context, bundleID string) (*DeveloperBundleIDGetResult, error) {
	bundleID = strings.TrimSpace(bundleID)
	if bundleID == "" {
		return nil, fmt.Errorf("bundle id is required")
	}
	if err := c.ensureDeveloperPortalSession(ctx); err != nil {
		return nil, err
	}

	query := url.Values{}
	query.Set("fields[bundleIds]", developerBundleIDDetailFields)
	query.Set("include", strings.Join(developerBundleIDDetailIncludes, ","))
	body, err := c.doDeveloperPortalBundleIDDetailRead(ctx, "/bundleIds/"+url.PathEscape(bundleID), bundleID, query)
	if err != nil {
		return nil, err
	}

	result, err := parseDeveloperBundleIDGetResponse(body)
	if err != nil {
		return nil, err
	}
	if result.Data.ID != bundleID {
		return nil, fmt.Errorf("developer portal returned Bundle ID %q, requested %q", result.Data.ID, bundleID)
	}
	return result, nil
}

type developerPortalTeamIDRequest struct {
	TeamID string `json:"teamId"`
}

// doDeveloperPortalBundleIDDetailRead is the detail variant captured from the
// web UI: the fields/include query is carried in the URL while the selected
// team is carried in a small JSON body. Like the collection proxy, it uses a
// POST with X-HTTP-Method-Override because the cookie-authenticated service
// rejects a direct browser-style GET.
func (c *Client) doDeveloperPortalBundleIDDetailRead(ctx context.Context, path, bundleID string, query url.Values) ([]byte, error) {
	teamID := c.developerPortalTeamID()
	if teamID == "" {
		return nil, fmt.Errorf("developer portal team is not selected; %s", developerPortalAuthHint())
	}
	path = queryPath(path, query)
	headers := developerPortalHeaders(bundleID)
	headers.Set("X-HTTP-Method-Override", http.MethodGet)
	return c.doDeveloperPortalRequest(ctx, http.MethodPost, path, developerPortalTeamIDRequest{TeamID: teamID}, headers, false)
}

func parseDeveloperBundleIDsListResponse(body []byte) (*DeveloperBundleIDsListResult, error) {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("failed to parse Developer Portal Bundle ID list response: %w", err)
	}
	if missingJSONValue(envelope.Data) {
		return nil, fmt.Errorf("developer portal Bundle ID list response has no data collection")
	}

	var result DeveloperBundleIDsListResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse Developer Portal Bundle ID list response: %w", err)
	}
	if result.Data == nil {
		return nil, fmt.Errorf("developer portal Bundle ID list response has no data collection")
	}
	for _, resource := range result.Data {
		if err := validateDeveloperBundleIDResource(resource); err != nil {
			return nil, fmt.Errorf("invalid Developer Portal Bundle ID list response: %w", err)
		}
	}
	if result.Included == nil {
		result.Included = []DeveloperBundleID{}
	}
	result.Raw = append(json.RawMessage(nil), body...)
	return &result, nil
}

func parseDeveloperBundleIDGetResponse(body []byte) (*DeveloperBundleIDGetResult, error) {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("failed to parse Developer Portal Bundle ID response: %w", err)
	}
	if missingJSONValue(envelope.Data) {
		return nil, fmt.Errorf("developer portal Bundle ID response has no data resource")
	}

	var result DeveloperBundleIDGetResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse Developer Portal Bundle ID response: %w", err)
	}
	if err := validateDeveloperBundleIDResource(result.Data); err != nil {
		return nil, fmt.Errorf("invalid Developer Portal Bundle ID response: %w", err)
	}
	if result.Included == nil {
		result.Included = []DeveloperBundleID{}
	}
	result.Raw = append(json.RawMessage(nil), body...)
	return &result, nil
}

func missingJSONValue(value json.RawMessage) bool {
	trimmed := bytes.TrimSpace(value)
	return len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null"))
}

func validateDeveloperBundleIDResource(resource DeveloperBundleID) error {
	if strings.TrimSpace(resource.ID) == "" {
		return fmt.Errorf("bundle ID resource is missing id")
	}
	if strings.TrimSpace(resource.Type) != "bundleIds" {
		return fmt.Errorf("bundle ID resource %q has type %q, want bundleIds", resource.ID, resource.Type)
	}
	return nil
}
