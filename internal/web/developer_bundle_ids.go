package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	developerPortalBaseURL    = "https://developer.apple.com"
	developerPortalAccountURL = developerPortalBaseURL + "/account"
	developerServicesBaseURL  = developerPortalBaseURL + "/services-account/v1"
	privateCloudCompute       = "PRIVATE_CLOUD_COMPUTE"
)

var supportedDeveloperBundleIDCapabilities = map[string]struct{}{
	privateCloudCompute: {},
}

var developerBundleIDIncludes = []string{
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

// DeveloperBundleIDCapabilityEnableRequest enables one supported Developer
// Portal-only capability on an existing Bundle ID resource.
type DeveloperBundleIDCapabilityEnableRequest struct {
	BundleID   string
	Capability string
}

// DeveloperBundleIDCapabilityEnableResult summarizes a Developer Portal
// capability enable operation. Changed is false when the capability was already
// enabled and no PATCH was sent.
type DeveloperBundleIDCapabilityEnableResult struct {
	BundleID   string `json:"bundleId"`
	Capability string `json:"capability"`
	Enabled    bool   `json:"enabled"`
	Changed    bool   `json:"changed"`
	Status     string `json:"status"`
}

type developerCapabilityMetadataResponse struct {
	Data []struct {
		ID         string                                `json:"id"`
		Type       string                                `json:"type"`
		Attributes developerCapabilityMetadataAttributes `json:"attributes"`
	} `json:"data"`
}

type developerCapabilityMetadataAttributes struct {
	Name         string `json:"name"`
	Entitlement  string `json:"entitlement"`
	IsPublic     bool   `json:"isPublic"`
	Editable     bool   `json:"editable"`
	CanRequest   bool   `json:"canRequestFromPortal"`
	EnabledByDef bool   `json:"enabledByDefault"`
}

type developerResource struct {
	ID            string                     `json:"id,omitempty"`
	Type          string                     `json:"type"`
	Attributes    json.RawMessage            `json:"attributes,omitempty"`
	Relationships map[string]json.RawMessage `json:"relationships,omitempty"`
}

type developerResourceRelationship struct {
	Data []developerResource `json:"data"`
}

type developerBundleIDResponse struct {
	Data struct {
		ID            string                     `json:"id"`
		Type          string                     `json:"type"`
		Attributes    json.RawMessage            `json:"attributes"`
		Relationships map[string]json.RawMessage `json:"relationships"`
	} `json:"data"`
	Included []developerResource `json:"included"`
}

type developerBundleIDPatchRequest struct {
	Data struct {
		ID            string                     `json:"id"`
		Type          string                     `json:"type"`
		Attributes    json.RawMessage            `json:"attributes"`
		Relationships map[string]json.RawMessage `json:"relationships"`
	} `json:"data"`
}

func normalizeDeveloperBundleIDCapabilityEnableRequest(req DeveloperBundleIDCapabilityEnableRequest) (DeveloperBundleIDCapabilityEnableRequest, error) {
	req.BundleID = strings.TrimSpace(req.BundleID)
	req.Capability = strings.ToUpper(strings.TrimSpace(req.Capability))
	if req.BundleID == "" {
		return req, fmt.Errorf("bundle id is required")
	}
	if req.Capability == "" {
		return req, fmt.Errorf("capability is required")
	}
	if _, ok := supportedDeveloperBundleIDCapabilities[req.Capability]; !ok {
		return req, fmt.Errorf("unsupported Developer Portal capability %q (supported: %s)", req.Capability, privateCloudCompute)
	}
	return req, nil
}

// EnableDeveloperBundleIDCapability enables a supported Developer Portal-only
// Bundle ID capability while preserving Apple's complete current capability
// relationship payload.
func (c *Client) EnableDeveloperBundleIDCapability(ctx context.Context, req DeveloperBundleIDCapabilityEnableRequest) (*DeveloperBundleIDCapabilityEnableResult, error) {
	req, err := normalizeDeveloperBundleIDCapabilityEnableRequest(req)
	if err != nil {
		return nil, err
	}
	if err := c.ensureDeveloperPortalSession(ctx); err != nil {
		return nil, err
	}

	metadata, err := c.loadDeveloperCapabilityMetadata(ctx, req.BundleID)
	if err != nil {
		return nil, err
	}
	capabilityMetadata, ok := findDeveloperCapability(metadata, req.Capability)
	if !ok {
		return nil, fmt.Errorf("capability %q is not available in Developer Portal for this account", req.Capability)
	}
	if !capabilityMetadata.Editable {
		return nil, fmt.Errorf("capability %q is not editable in Developer Portal for this account", req.Capability)
	}

	current, err := c.loadDeveloperBundleID(ctx, req.BundleID)
	if err != nil {
		return nil, err
	}
	payload, alreadyEnabled, err := buildDeveloperBundleIDCapabilityPatchRequest(current, req)
	if err != nil {
		return nil, err
	}
	if alreadyEnabled {
		return &DeveloperBundleIDCapabilityEnableResult{
			BundleID:   req.BundleID,
			Capability: req.Capability,
			Enabled:    true,
			Changed:    false,
			Status:     "already-enabled",
		}, nil
	}

	csrf, csrfTS := c.developerCSRFTokens()
	if csrf == "" || csrfTS == "" {
		return nil, fmt.Errorf("missing Developer Portal CSRF headers; run 'asc web auth login --apple-id EMAIL --reauthenticate' and try again")
	}
	path := "/bundleIds/" + url.PathEscape(req.BundleID)
	if _, err := c.doDeveloperPortalRequest(ctx, http.MethodPatch, path, payload, developerPortalHeaders(req.BundleID), true); err != nil {
		return nil, err
	}

	return &DeveloperBundleIDCapabilityEnableResult{
		BundleID:   req.BundleID,
		Capability: req.Capability,
		Enabled:    true,
		Changed:    true,
		Status:     "enabled",
	}, nil
}

func (c *Client) ensureDeveloperPortalSession(ctx context.Context) error {
	headers := developerPortalHeaders("")
	headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	body, response, err := c.doDeveloperPortalHTTP(ctx, http.MethodGet, developerPortalAccountURL, nil, headers)
	if err != nil {
		return err
	}
	_ = body
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return developerPortalSessionError(response.StatusCode)
	}
	if response.StatusCode < 200 || response.StatusCode >= 400 {
		return &APIError{Status: response.StatusCode, AppleRequestID: extractAppleRequestID(response.Header), rawBody: body}
	}
	if response.Request != nil && response.Request.URL != nil && !strings.EqualFold(response.Request.URL.Hostname(), "developer.apple.com") {
		return fmt.Errorf("authentication redirected to %s instead of Developer Portal; run 'asc web auth login --apple-id EMAIL --reauthenticate' and try again", response.Request.URL.Hostname())
	}
	return nil
}

func (c *Client) loadDeveloperCapabilityMetadata(ctx context.Context, bundleID string) (developerCapabilityMetadataResponse, error) {
	query := make(url.Values)
	query.Set("filter[capabilityType]", "capability,service")
	query.Set("filter[includeRequestable]", "true")
	body, err := c.doDeveloperPortalRequest(ctx, http.MethodPost, "/capabilities?"+query.Encode(), nil, developerPortalHeaders(bundleID), false)
	if err != nil {
		return developerCapabilityMetadataResponse{}, err
	}
	var response developerCapabilityMetadataResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return response, fmt.Errorf("failed to parse Developer Portal capabilities response: %w", err)
	}
	return response, nil
}

func findDeveloperCapability(response developerCapabilityMetadataResponse, capabilityID string) (developerCapabilityMetadataAttributes, bool) {
	for _, capability := range response.Data {
		if capability.Type == "capabilities" && strings.EqualFold(strings.TrimSpace(capability.ID), capabilityID) {
			return capability.Attributes, true
		}
	}
	return developerCapabilityMetadataAttributes{}, false
}

func (c *Client) loadDeveloperBundleID(ctx context.Context, bundleID string) (developerBundleIDResponse, error) {
	query := make(url.Values)
	query.Set("fields[bundleIds]", "name,identifier,platform,seedId,wildcard,~permissions.delete,~permissions.edit")
	query.Set("include", strings.Join(developerBundleIDIncludes, ","))
	path := "/bundleIds/" + url.PathEscape(bundleID) + "?" + query.Encode()
	body, err := c.doDeveloperPortalRequest(ctx, http.MethodPost, path, nil, developerPortalHeaders(bundleID), false)
	if err != nil {
		return developerBundleIDResponse{}, err
	}
	var response developerBundleIDResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return response, fmt.Errorf("failed to parse Developer Portal Bundle ID response: %w", err)
	}
	if strings.TrimSpace(response.Data.ID) == "" || response.Data.Type != "bundleIds" || len(response.Data.Attributes) == 0 {
		return response, fmt.Errorf("incomplete Bundle ID resource returned by Developer Portal")
	}
	return response, nil
}

func buildDeveloperBundleIDCapabilityPatchRequest(current developerBundleIDResponse, req DeveloperBundleIDCapabilityEnableRequest) (developerBundleIDPatchRequest, bool, error) {
	capabilities, err := developerBundleIDCapabilities(current)
	if err != nil {
		return developerBundleIDPatchRequest{}, false, err
	}

	updated := make([]developerResource, 0, len(capabilities)+1)
	foundTarget := false
	alreadyEnabled := false
	for _, capability := range capabilities {
		capabilityID, err := developerBundleIDCapabilityID(capability)
		if err != nil {
			return developerBundleIDPatchRequest{}, false, err
		}
		if capabilityID != req.Capability {
			updated = append(updated, capability)
			continue
		}
		if foundTarget {
			continue
		}
		foundTarget = true
		enabled, err := developerBundleIDCapabilityEnabled(capability)
		if err != nil {
			return developerBundleIDPatchRequest{}, false, err
		}
		if enabled {
			alreadyEnabled = true
			updated = append(updated, capability)
			continue
		}
		capability.Attributes, err = setDeveloperCapabilityEnabled(capability.Attributes)
		if err != nil {
			return developerBundleIDPatchRequest{}, false, err
		}
		updated = append(updated, capability)
	}
	if alreadyEnabled {
		return developerBundleIDPatchRequest{}, true, nil
	}
	if !foundTarget {
		updated = append(updated, newDeveloperBundleIDCapability(req.Capability))
	}

	relationshipBody, err := json.Marshal(developerResourceRelationship{Data: updated})
	if err != nil {
		return developerBundleIDPatchRequest{}, false, fmt.Errorf("failed to build Bundle ID capability relationships: %w", err)
	}
	relationships := cloneRawMessageMap(current.Data.Relationships)
	if relationships == nil {
		relationships = make(map[string]json.RawMessage)
	}
	relationships["bundleIdCapabilities"] = relationshipBody

	var payload developerBundleIDPatchRequest
	payload.Data.ID = current.Data.ID
	payload.Data.Type = current.Data.Type
	payload.Data.Attributes = append(json.RawMessage(nil), current.Data.Attributes...)
	payload.Data.Relationships = relationships
	return payload, false, nil
}

func developerBundleIDCapabilities(current developerBundleIDResponse) ([]developerResource, error) {
	var relationship developerResourceRelationship
	rawRelationship, ok := current.Data.Relationships["bundleIdCapabilities"]
	if !ok {
		return []developerResource{}, nil
	}
	if err := json.Unmarshal(rawRelationship, &relationship); err != nil {
		return nil, fmt.Errorf("failed to parse current Bundle ID capability relationships: %w", err)
	}

	includedByID := make(map[string]developerResource)
	includedOrder := make([]string, 0)
	for _, resource := range current.Included {
		if resource.Type != "bundleIdCapabilities" || strings.TrimSpace(resource.ID) == "" {
			continue
		}
		if _, exists := includedByID[resource.ID]; !exists {
			includedOrder = append(includedOrder, resource.ID)
		}
		includedByID[resource.ID] = resource
	}

	capabilities := make([]developerResource, 0, len(relationship.Data))
	seen := make(map[string]struct{})
	for _, resource := range relationship.Data {
		if resource.Type != "bundleIdCapabilities" {
			continue
		}
		if resource.ID != "" {
			if _, duplicate := seen[resource.ID]; duplicate {
				continue
			}
			seen[resource.ID] = struct{}{}
			if included, ok := includedByID[resource.ID]; ok {
				resource = included
			}
		}
		if _, err := developerBundleIDCapabilityID(resource); err != nil {
			return nil, fmt.Errorf("cannot safely preserve Bundle ID capability %q: %w", resource.ID, err)
		}
		capabilities = append(capabilities, resource)
	}
	for _, id := range includedOrder {
		if _, ok := seen[id]; ok {
			continue
		}
		resource := includedByID[id]
		if _, err := developerBundleIDCapabilityID(resource); err != nil {
			return nil, fmt.Errorf("cannot safely preserve Bundle ID capability %q: %w", resource.ID, err)
		}
		seen[id] = struct{}{}
		capabilities = append(capabilities, resource)
	}
	return capabilities, nil
}

func developerBundleIDCapabilityID(resource developerResource) (string, error) {
	raw, ok := resource.Relationships["capability"]
	if !ok {
		return "", fmt.Errorf("missing capability relationship")
	}
	var relationship struct {
		Data relationshipData `json:"data"`
	}
	if err := json.Unmarshal(raw, &relationship); err != nil {
		return "", fmt.Errorf("invalid capability relationship: %w", err)
	}
	id := strings.ToUpper(strings.TrimSpace(relationship.Data.ID))
	if relationship.Data.Type != "capabilities" || id == "" {
		return "", fmt.Errorf("invalid capability relationship data")
	}
	return id, nil
}

func developerBundleIDCapabilityEnabled(resource developerResource) (bool, error) {
	if len(resource.Attributes) == 0 {
		return false, fmt.Errorf("bundle ID capability %q is missing attributes", resource.ID)
	}
	var attributes map[string]json.RawMessage
	if err := json.Unmarshal(resource.Attributes, &attributes); err != nil {
		return false, fmt.Errorf("failed to parse Bundle ID capability %q attributes: %w", resource.ID, err)
	}
	var enabled bool
	raw, ok := attributes["enabled"]
	if !ok {
		return false, nil
	}
	if err := json.Unmarshal(raw, &enabled); err != nil {
		return false, fmt.Errorf("failed to parse Bundle ID capability %q enabled state: %w", resource.ID, err)
	}
	return enabled, nil
}

func setDeveloperCapabilityEnabled(raw json.RawMessage) (json.RawMessage, error) {
	var attributes map[string]json.RawMessage
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &attributes); err != nil {
			return nil, fmt.Errorf("failed to parse existing capability attributes: %w", err)
		}
	}
	if attributes == nil {
		attributes = make(map[string]json.RawMessage)
	}
	attributes["enabled"] = json.RawMessage("true")
	if _, ok := attributes["settings"]; !ok {
		attributes["settings"] = json.RawMessage("[]")
	}
	updated, err := json.Marshal(attributes)
	if err != nil {
		return nil, fmt.Errorf("failed to encode capability attributes: %w", err)
	}
	return updated, nil
}

func newDeveloperBundleIDCapability(capability string) developerResource {
	capabilityRelationship, _ := json.Marshal(struct {
		Data relationshipData `json:"data"`
	}{Data: relationshipData{Type: "capabilities", ID: capability}})
	return developerResource{
		Type:       "bundleIdCapabilities",
		Attributes: json.RawMessage(`{"enabled":true,"settings":[]}`),
		Relationships: map[string]json.RawMessage{
			"capability": capabilityRelationship,
		},
	}
}

func cloneRawMessageMap(source map[string]json.RawMessage) map[string]json.RawMessage {
	if source == nil {
		return nil
	}
	cloned := make(map[string]json.RawMessage, len(source))
	for key, value := range source {
		cloned[key] = append(json.RawMessage(nil), value...)
	}
	return cloned
}

func developerPortalHeaders(bundleID string) http.Header {
	headers := make(http.Header)
	headers.Set("Accept", "application/vnd.api+json, application/json")
	headers.Set("Content-Type", "application/vnd.api+json")
	headers.Set("Referer", developerPortalBaseURL+"/account/resources/identifiers/list")
	if strings.TrimSpace(bundleID) != "" {
		headers.Set("Referer", developerPortalBaseURL+"/account/resources/identifiers/bundleId/edit/"+url.PathEscape(bundleID))
	}
	headers.Set("User-Agent", "App-Store-Connect-CLI")
	headers.Set("X-Requested-With", "XMLHttpRequest")
	return headers
}

func (c *Client) doDeveloperPortalRequest(ctx context.Context, method, path string, body any, headers http.Header, requireCSRF bool) ([]byte, error) {
	if requireCSRF {
		csrf, csrfTS := c.developerCSRFTokens()
		headers.Set("csrf", csrf)
		headers.Set("csrf_ts", csrfTS)
	}
	responseBody, response, err := c.doDeveloperPortalHTTP(ctx, method, developerServicesBaseURL+path, body, headers)
	if err != nil {
		return nil, err
	}
	c.captureDeveloperCSRFTokens(response.Header)
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return nil, developerPortalSessionError(response.StatusCode)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, &APIError{
			Status:         response.StatusCode,
			AppleRequestID: extractAppleRequestID(response.Header),
			CorrelationKey: strings.TrimSpace(response.Header.Get("X-Apple-Jingle-Correlation-Key")),
			rawBody:        responseBody,
		}
	}
	return responseBody, nil
}

func (c *Client) doDeveloperPortalHTTP(ctx context.Context, method, requestURL string, body any, headers http.Header) ([]byte, *http.Response, error) {
	if c == nil || c.httpClient == nil {
		return nil, nil, fmt.Errorf("web client is not configured for Developer Portal")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := c.waitForRateLimit(ctx); err != nil {
		return nil, nil, err
	}

	var requestBody io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal Developer Portal request: %w", err)
		}
		requestBody = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, requestURL, requestBody)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create Developer Portal request: %w", err)
	}
	request.Header = cloneHeaders(headers)
	setModifiedCookieHeader(c.httpClient, request)

	response, err := c.httpClient.Do(request)
	if err != nil {
		logWebAuthHTTP("developer_portal_request", request, nil, nil, err)
		return nil, nil, fmt.Errorf("request to Developer Portal failed: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		logWebAuthHTTP("developer_portal_request", request, response, nil, err)
		return nil, response, fmt.Errorf("failed to read Developer Portal response: %w", err)
	}
	logWebAuthHTTP("developer_portal_request", request, response, responseBody, nil)
	return responseBody, response, nil
}

func (c *Client) captureDeveloperCSRFTokens(headers http.Header) {
	csrf := headerValueCaseInsensitive(headers, "csrf")
	csrfTS := headerValueCaseInsensitive(headers, "csrf_ts")
	if csrf == "" && csrfTS == "" {
		return
	}
	c.developerSessionMu.Lock()
	defer c.developerSessionMu.Unlock()
	if csrf != "" {
		c.developerCSRF = csrf
	}
	if csrfTS != "" {
		c.developerCSRFTS = csrfTS
	}
}

func headerValueCaseInsensitive(headers http.Header, name string) string {
	for key, values := range headers {
		if !strings.EqualFold(key, name) || len(values) == 0 {
			continue
		}
		return strings.TrimSpace(values[0])
	}
	return ""
}

func (c *Client) developerCSRFTokens() (string, string) {
	c.developerSessionMu.Lock()
	defer c.developerSessionMu.Unlock()
	return c.developerCSRF, c.developerCSRFTS
}

func developerPortalSessionError(status int) error {
	return fmt.Errorf("web session is unauthorized or expired for Developer Portal (status %d); run 'asc web auth login --apple-id EMAIL --reauthenticate' and try again", status)
}
