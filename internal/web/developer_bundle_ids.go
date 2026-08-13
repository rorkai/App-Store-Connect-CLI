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
	developerPortalBaseURL   = "https://developer.apple.com"
	developerPortalTeamsPath = "/services-account/QH65B2/account/listTeams.action"
	developerServicesPath    = "/services-account/v1"
	privateCloudCompute      = "PRIVATE_CLOUD_COMPUTE"
	developerPortalAuthHint  = "run 'asc web auth logout --apple-id EMAIL', then 'asc web auth login --apple-id EMAIL', and try again"
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

type developerPortalTeam struct {
	TeamID string `json:"teamId"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type developerPortalTeamsResponse struct {
	Teams []developerPortalTeam `json:"teams"`
	Data  struct {
		Teams []developerPortalTeam `json:"teams"`
	} `json:"data"`
}

type developerPortalProxyReadRequest struct {
	URLEncodedQueryParams string `json:"urlEncodedQueryParams"`
	TeamID                string `json:"teamId"`
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
	teamID := c.developerPortalTeamID()
	if teamID == "" {
		return nil, fmt.Errorf("developer portal team is not selected; %s", developerPortalAuthHint)
	}
	payload, err = addDeveloperPortalTeamID(payload, teamID)
	if err != nil {
		return nil, err
	}

	csrf, csrfTS := c.developerCSRFTokens()
	if csrf == "" || csrfTS == "" {
		return nil, fmt.Errorf("missing Developer Portal CSRF headers; %s", developerPortalAuthHint)
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
	// The App Store Connect SRP session becomes usable by Developer Portal only
	// after its legacy team endpoint establishes Portal team and CSRF context.
	headers := developerPortalHeaders("")
	headers.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	headers.Set("Content-Type", "application/x-www-form-urlencoded")
	body, response, err := c.doDeveloperPortalHTTP(ctx, http.MethodPost, c.developerPortalOrigin()+developerPortalTeamsPath, nil, headers)
	if err != nil {
		return err
	}
	c.captureDeveloperCSRFTokens(response.Header)
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return developerPortalSessionError(response.StatusCode)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &APIError{Status: response.StatusCode, AppleRequestID: extractAppleRequestID(response.Header), rawBody: body}
	}
	portalURL, parseErr := url.Parse(c.developerPortalOrigin())
	if parseErr != nil {
		return fmt.Errorf("invalid Developer Portal base URL: %w", parseErr)
	}
	if response.Request != nil && response.Request.URL != nil && !strings.EqualFold(response.Request.URL.Hostname(), portalURL.Hostname()) {
		return fmt.Errorf("authentication redirected to %s instead of Developer Portal; %s", response.Request.URL.Hostname(), developerPortalAuthHint)
	}

	var payload developerPortalTeamsResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("failed to parse Developer Portal teams response: %w", err)
	}
	teams := payload.Teams
	if len(teams) == 0 {
		teams = payload.Data.Teams
	}
	team, err := selectDeveloperPortalTeam(teams, c.publicProviderID, c.providerName)
	if err != nil {
		return err
	}
	c.developerSessionMu.Lock()
	c.developerTeamID = team.TeamID
	c.developerSessionMu.Unlock()
	return nil
}

func selectDeveloperPortalTeam(teams []developerPortalTeam, publicProviderID, providerName string) (developerPortalTeam, error) {
	valid := make([]developerPortalTeam, 0, len(teams))
	for _, team := range teams {
		team.TeamID = strings.TrimSpace(team.TeamID)
		team.Name = strings.TrimSpace(team.Name)
		if team.TeamID != "" {
			valid = append(valid, team)
		}
	}
	if len(valid) == 0 {
		return developerPortalTeam{}, fmt.Errorf("apple account has no Developer Portal team; a paid Apple Developer Program membership may be required")
	}
	publicProviderID = strings.TrimSpace(publicProviderID)
	if publicProviderID != "" {
		for _, team := range valid {
			if strings.EqualFold(publicProviderID, team.TeamID) {
				return team, nil
			}
		}
	}
	providerName = strings.TrimSpace(providerName)
	if providerName != "" {
		for _, team := range valid {
			if strings.EqualFold(providerName, team.Name) {
				return team, nil
			}
		}
		var prefixMatch developerPortalTeam
		for _, team := range valid {
			if team.Name != "" && strings.HasPrefix(strings.ToLower(providerName), strings.ToLower(team.Name)) && len(team.Name) > len(prefixMatch.Name) {
				prefixMatch = team
			}
		}
		if prefixMatch.TeamID != "" {
			return prefixMatch, nil
		}
	}
	if len(valid) == 1 {
		return valid[0], nil
	}
	return developerPortalTeam{}, fmt.Errorf("could not match App Store Connect provider %q to one of %d Developer Portal teams", providerName, len(valid))
}

func (c *Client) loadDeveloperCapabilityMetadata(ctx context.Context, bundleID string) (developerCapabilityMetadataResponse, error) {
	query := make(url.Values)
	query.Set("filter[capabilityType]", "capability,service")
	query.Set("filter[includeRequestable]", "true")
	body, err := c.doDeveloperPortalProxyRead(ctx, "/capabilities", query, developerPortalHeaders(bundleID))
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
	path := "/bundleIds/" + url.PathEscape(bundleID)
	body, err := c.doDeveloperPortalProxyRead(ctx, path, query, developerPortalHeaders(bundleID))
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

	capabilityIDs := make([]string, len(capabilities))
	targetIndex := -1
	targetEnabled := false
	for index, capability := range capabilities {
		capabilityID, err := developerBundleIDCapabilityID(capability)
		if err != nil {
			return developerBundleIDPatchRequest{}, false, err
		}
		capabilityIDs[index] = capabilityID
		if capabilityID != req.Capability {
			continue
		}
		enabled, err := developerBundleIDCapabilityEnabled(capability)
		if err != nil {
			return developerBundleIDPatchRequest{}, false, err
		}
		if targetIndex == -1 || (enabled && !targetEnabled) {
			targetIndex = index
			targetEnabled = enabled
		}
	}
	if targetEnabled {
		return developerBundleIDPatchRequest{}, true, nil
	}

	updated := make([]developerResource, 0, len(capabilities)+1)
	for index, capability := range capabilities {
		if capabilityIDs[index] != req.Capability {
			updated = append(updated, capability)
			continue
		}
		if index != targetIndex {
			continue
		}
		var err error
		capability.Attributes, err = setDeveloperCapabilityEnabled(capability.Attributes)
		if err != nil {
			return developerBundleIDPatchRequest{}, false, err
		}
		updated = append(updated, capability)
	}
	if targetIndex == -1 {
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

func addDeveloperPortalTeamID(payload developerBundleIDPatchRequest, teamID string) (developerBundleIDPatchRequest, error) {
	var attributes map[string]json.RawMessage
	if len(payload.Data.Attributes) > 0 {
		if err := json.Unmarshal(payload.Data.Attributes, &attributes); err != nil {
			return payload, fmt.Errorf("failed to parse Bundle ID attributes for Developer Portal team: %w", err)
		}
	}
	if attributes == nil {
		attributes = make(map[string]json.RawMessage)
	}
	encodedTeamID, err := json.Marshal(strings.TrimSpace(teamID))
	if err != nil {
		return payload, fmt.Errorf("failed to encode Developer Portal team: %w", err)
	}
	attributes["teamId"] = encodedTeamID
	encodedAttributes, err := json.Marshal(attributes)
	if err != nil {
		return payload, fmt.Errorf("failed to encode Bundle ID attributes for Developer Portal team: %w", err)
	}
	payload.Data.Attributes = encodedAttributes
	return payload, nil
}

func developerBundleIDCapabilities(current developerBundleIDResponse) ([]developerResource, error) {
	var relationship developerResourceRelationship
	rawRelationship, ok := current.Data.Relationships["bundleIdCapabilities"]
	if ok {
		if err := json.Unmarshal(rawRelationship, &relationship); err != nil {
			return nil, fmt.Errorf("failed to parse current Bundle ID capability relationships: %w", err)
		}
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

func (c *Client) developerPortalOrigin() string {
	if c != nil && strings.TrimSpace(c.developerPortalURL) != "" {
		return strings.TrimRight(strings.TrimSpace(c.developerPortalURL), "/")
	}
	return developerPortalBaseURL
}

func (c *Client) doDeveloperPortalProxyRead(ctx context.Context, path string, query url.Values, headers http.Header) ([]byte, error) {
	// Developer Portal's cookie-authenticated v1 API proxies logical GETs as
	// POSTs carrying the team and encoded query in the request body.
	teamID := c.developerPortalTeamID()
	if teamID == "" {
		return nil, fmt.Errorf("developer portal team is not selected; %s", developerPortalAuthHint)
	}
	headers.Set("X-HTTP-Method-Override", http.MethodGet)
	return c.doDeveloperPortalRequest(ctx, http.MethodPost, path, developerPortalProxyReadRequest{
		URLEncodedQueryParams: query.Encode(),
		TeamID:                teamID,
	}, headers, false)
}

func (c *Client) doDeveloperPortalRequest(ctx context.Context, method, path string, body any, headers http.Header, requireCSRF bool) ([]byte, error) {
	csrf, csrfTS := c.developerCSRFTokens()
	if csrf != "" {
		headers.Set("csrf", csrf)
	}
	if csrfTS != "" {
		headers.Set("csrf_ts", csrfTS)
	}
	if requireCSRF {
		if csrf == "" || csrfTS == "" {
			return nil, fmt.Errorf("missing Developer Portal CSRF headers; %s", developerPortalAuthHint)
		}
	}
	responseBody, response, err := c.doDeveloperPortalHTTP(ctx, method, c.developerPortalOrigin()+developerServicesPath+path, body, headers)
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
		switch typed := body.(type) {
		case url.Values:
			requestBody = strings.NewReader(typed.Encode())
		default:
			encoded, err := json.Marshal(body)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal Developer Portal request: %w", err)
			}
			requestBody = bytes.NewReader(encoded)
		}
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

func (c *Client) developerPortalTeamID() string {
	c.developerSessionMu.Lock()
	defer c.developerSessionMu.Unlock()
	return c.developerTeamID
}

func developerPortalSessionError(status int) error {
	return fmt.Errorf("web session is unauthorized or expired for Developer Portal (status %d); %s", status, developerPortalAuthHint)
}
