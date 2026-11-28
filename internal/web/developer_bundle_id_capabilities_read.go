package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// DeveloperCapability is a capability catalog resource returned by Apple.
type DeveloperCapability struct {
	ID         string                                `json:"id"`
	Type       string                                `json:"type"`
	Attributes developerCapabilityMetadataAttributes `json:"attributes,omitempty"`
}

// DeveloperBundleIDCapabilitiesListResult retains Apple's complete capability catalog envelope.
type DeveloperBundleIDCapabilitiesListResult struct {
	Data []DeveloperCapability `json:"data"`
	Raw  json.RawMessage       `json:"-"`
}

// MarshalJSON preserves unknown catalog metadata and envelope members.
func (r DeveloperBundleIDCapabilitiesListResult) MarshalJSON() ([]byte, error) {
	if len(r.Raw) > 0 {
		return append([]byte(nil), r.Raw...), nil
	}
	type resultWithoutMethods DeveloperBundleIDCapabilitiesListResult
	return json.Marshal(resultWithoutMethods(r))
}

// ListDeveloperBundleIDCapabilities reads the team's capability catalog using the
// selected Bundle ID as portal context. Requestable entries may need approval.
func (c *Client) ListDeveloperBundleIDCapabilities(ctx context.Context, bundleID string) (*DeveloperBundleIDCapabilitiesListResult, error) {
	bundleID = strings.TrimSpace(bundleID)
	if bundleID == "" {
		return nil, fmt.Errorf("bundle id is required")
	}
	if err := c.ensureDeveloperPortalSession(ctx); err != nil {
		return nil, err
	}
	query := make(url.Values)
	query.Set("filter[capabilityType]", "capability,service")
	query.Set("filter[includeRequestable]", "true")
	bodyQuery := make(url.Values)
	bodyQuery.Set("filter[bundleId]", bundleID)
	bodyQuery.Set("filter[platform]", "IOS,MACOS")
	body, err := c.doDeveloperPortalProxyRead(ctx, queryPath("/capabilities", query), bodyQuery, developerPortalHeaders(bundleID))
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Data  []DeveloperCapability      `json:"data"`
		Links map[string]json.RawMessage `json:"links"`
		Meta  struct {
			Paging struct {
				Total *int `json:"total"`
			} `json:"paging"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("failed to parse Developer Portal capabilities response: %w", err)
	}
	if envelope.Data == nil {
		return nil, fmt.Errorf("developer portal capability catalog must contain a data array")
	}
	if next := bytes.TrimSpace(envelope.Links["next"]); len(next) > 0 && string(next) != "null" && string(next) != `""` {
		return nil, fmt.Errorf("developer portal capability catalog is incomplete: response has a continuation link; capability availability cannot be established")
	}
	if total := envelope.Meta.Paging.Total; total != nil && (*total < 0 || *total > len(envelope.Data)) {
		return nil, fmt.Errorf("developer portal capability catalog is incomplete: returned %d resources but paging total is %d; capability availability cannot be established", len(envelope.Data), *total)
	}
	return &DeveloperBundleIDCapabilitiesListResult{Data: envelope.Data, Raw: body}, nil
}
