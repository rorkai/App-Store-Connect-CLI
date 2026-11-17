package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

const developerICloudContainersListQuery = "limit=1000&offset=0&sort=name"

// DeveloperICloudContainer is an iCloud container resource returned by the
// modern Developer Portal web-session endpoint.
type DeveloperICloudContainer struct {
	ID            string                             `json:"id"`
	Type          string                             `json:"type"`
	Attributes    DeveloperICloudContainerAttributes `json:"attributes"`
	Links         map[string]any                     `json:"links,omitempty"`
	Relationships map[string]any                     `json:"relationships,omitempty"`
}

// DeveloperICloudContainerAttributes contains the fields Apple returns for a
// Developer Portal iCloud container list resource.
type DeveloperICloudContainerAttributes struct {
	Identifier string `json:"identifier"`
	Hidden     bool   `json:"hidden"`
	Prefix     string `json:"prefix"`
	CanEdit    bool   `json:"canEdit"`
	Name       string `json:"name"`
	CanDelete  bool   `json:"canDelete"`
	ResponseID string `json:"responseId"`
}

// DeveloperICloudContainersListResult is the read-only iCloud container
// collection returned by the Developer Portal web session.
//
// Raw keeps Apple's complete JSON:API envelope authoritative for JSON output,
// including fields this client does not model yet.
type DeveloperICloudContainersListResult struct {
	Data  []DeveloperICloudContainer `json:"data"`
	Links map[string]any             `json:"links,omitempty"`
	Meta  map[string]any             `json:"meta,omitempty"`
	Raw   json.RawMessage            `json:"-"`
}

var _ asc.PaginatedResponse = (*DeveloperICloudContainersListResult)(nil)

// GetLinks exposes actual continuation links for formatted-output diagnostics.
func (r *DeveloperICloudContainersListResult) GetLinks() *asc.Links {
	if r == nil {
		return nil
	}
	return &asc.Links{Self: developerBundleIDLinkString(r.Links, "self"), Next: developerBundleIDLinkString(r.Links, "next"), First: developerBundleIDLinkString(r.Links, "first"), Prev: developerBundleIDLinkString(r.Links, "prev")}
}

// GetData exposes the bounded collection to shared pagination diagnostics.
func (r *DeveloperICloudContainersListResult) GetData() any {
	if r == nil {
		return nil
	}
	return r.Data
}

// GetMeta exposes paging totals without changing the original JSON envelope.
func (r *DeveloperICloudContainersListResult) GetMeta() json.RawMessage {
	if r == nil || r.Meta == nil {
		return nil
	}
	encoded, err := json.Marshal(r.Meta)
	if err != nil {
		return nil
	}
	return encoded
}

// MarshalJSON preserves Apple's full collection envelope for JSON output.
func (r DeveloperICloudContainersListResult) MarshalJSON() ([]byte, error) {
	if len(r.Raw) > 0 {
		return append([]byte(nil), r.Raw...), nil
	}
	type resultWithoutMethods DeveloperICloudContainersListResult
	return json.Marshal(resultWithoutMethods(r))
}

// ListDeveloperICloudContainers reads the modern Developer Portal iCloud
// container collection. Apple exposes this logical GET as a POST with
// X-HTTP-Method-Override and keeps the hidden filter in the URL while the
// selected team and bounded query are sent in the JSON body.
func (c *Client) ListDeveloperICloudContainers(ctx context.Context, hidden bool) (*DeveloperICloudContainersListResult, error) {
	if err := c.ensureDeveloperPortalSession(ctx); err != nil {
		return nil, err
	}
	return c.listDeveloperICloudContainersAfterSession(ctx, hidden)
}

func (c *Client) listDeveloperICloudContainersAfterSession(ctx context.Context, hidden bool) (*DeveloperICloudContainersListResult, error) {
	teamID := c.developerPortalTeamID()
	if teamID == "" {
		return nil, fmt.Errorf("developer portal team is not selected; %s", developerPortalAuthHint())
	}
	query := url.Values{}
	query.Set("filter[AND][hidden]", strconv.FormatBool(hidden))
	path := queryPath("/cloudContainers", query)
	headers := developerPortalHeaders("")
	headers.Set("X-HTTP-Method-Override", http.MethodGet)
	body, err := c.doDeveloperPortalRequest(ctx, http.MethodPost, path, developerPortalProxyReadRequest{
		URLEncodedQueryParams: developerICloudContainersListQuery,
		TeamID:                teamID,
	}, headers, false)
	if err != nil {
		return nil, err
	}
	return parseDeveloperICloudContainersListResponse(body)
}

func parseDeveloperICloudContainersListResponse(body []byte) (*DeveloperICloudContainersListResult, error) {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("failed to parse Developer Portal iCloud container list response: %w", err)
	}
	if missingJSONValue(envelope.Data) {
		return nil, fmt.Errorf("developer portal iCloud container list response has no data collection")
	}

	var result DeveloperICloudContainersListResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse Developer Portal iCloud container list response: %w", err)
	}
	if result.Data == nil {
		return nil, fmt.Errorf("developer portal iCloud container list response has no data collection")
	}
	for _, resource := range result.Data {
		if err := validateDeveloperICloudContainerResource(resource); err != nil {
			return nil, fmt.Errorf("invalid Developer Portal iCloud container list response: %w", err)
		}
	}
	result.Raw = append(json.RawMessage(nil), body...)
	return &result, nil
}

func validateDeveloperICloudContainerResource(resource DeveloperICloudContainer) error {
	if strings.TrimSpace(resource.ID) == "" {
		return fmt.Errorf("cloud container resource is missing id")
	}
	if strings.TrimSpace(resource.Type) != "cloudContainers" {
		return fmt.Errorf("cloud container resource %q has type %q, want cloudContainers", resource.ID, resource.Type)
	}
	return nil
}
