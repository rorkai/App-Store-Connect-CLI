package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	developerPortalLegacyBaseURL     = developerPortalBaseURL + "/services-account/QH65B2"
	developerAppGroupsListPath       = "/account/ios/identifiers/listApplicationGroups.action"
	developerAppGroupsCreatePath     = "/account/ios/identifiers/addApplicationGroup.action"
	developerAppGroupsPageSize       = 500
	developerAppGroupsCapabilityType = "APP_GROUPS"
)

// DeveloperAppGroup is an App Group identifier returned by Apple Developer Portal.
type DeveloperAppGroup struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Identifier string `json:"identifier"`
	Prefix     string `json:"prefix,omitempty"`
	Status     string `json:"status,omitempty"`
}

// DeveloperAppGroupsListResult contains every App Group visible to the selected team.
type DeveloperAppGroupsListResult struct {
	Data []DeveloperAppGroup `json:"data"`
}

// DeveloperAppGroupCreateRequest registers an App Group identifier.
type DeveloperAppGroupCreateRequest struct {
	Name       string
	Identifier string
}

// DeveloperAppGroupAssignRequest associates an App Group with a Bundle ID.
type DeveloperAppGroupAssignRequest struct {
	BundleID string
	GroupID  string
}

// DeveloperAppGroupAssignResult summarizes an App Group assignment.
type DeveloperAppGroupAssignResult struct {
	BundleID string `json:"bundleId"`
	GroupID  string `json:"groupId"`
	Changed  bool   `json:"changed"`
	Status   string `json:"status"`
}

type developerPortalLegacyResponse struct {
	ResultCode   *int   `json:"resultCode"`
	ResultString string `json:"resultString"`
	UserString   string `json:"userString"`
	RequestID    string `json:"requestId"`
}

type developerAppGroupPayload struct {
	Name             string `json:"name"`
	Prefix           string `json:"prefix"`
	Identifier       string `json:"identifier"`
	Status           string `json:"status"`
	ApplicationGroup string `json:"applicationGroup"`
}

type developerAppGroupsListResponse struct {
	developerPortalLegacyResponse
	PageNumber           int                        `json:"pageNumber"`
	PageSize             int                        `json:"pageSize"`
	TotalRecords         int                        `json:"totalRecords"`
	ApplicationGroupList []developerAppGroupPayload `json:"applicationGroupList"`
}

type developerAppGroupCreateResponse struct {
	developerPortalLegacyResponse
	ApplicationGroup developerAppGroupPayload `json:"applicationGroup"`
}

// ListDeveloperAppGroups lists App Groups through the selected Developer Portal team.
func (c *Client) ListDeveloperAppGroups(ctx context.Context) (*DeveloperAppGroupsListResult, error) {
	if err := c.ensureDeveloperPortalSession(ctx); err != nil {
		return nil, err
	}
	teamID := c.developerPortalTeamID()
	if teamID == "" {
		return nil, fmt.Errorf("developer portal team is not selected; %s", developerPortalAuthHint)
	}

	result := &DeveloperAppGroupsListResult{Data: []DeveloperAppGroup{}}
	for pageNumber := 1; ; pageNumber++ {
		body, err := c.doDeveloperPortalLegacyFormRequest(ctx, developerAppGroupsListPath, url.Values{
			"teamId":     {teamID},
			"pageNumber": {strconv.Itoa(pageNumber)},
			"pageSize":   {strconv.Itoa(developerAppGroupsPageSize)},
			"sort":       {"name=asc"},
		}, false)
		if err != nil {
			return nil, err
		}

		var page developerAppGroupsListResponse
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("failed to parse Developer Portal App Groups response: %w", err)
		}
		if err := validateDeveloperPortalLegacyResponse(page.developerPortalLegacyResponse); err != nil {
			return nil, err
		}
		for _, group := range page.ApplicationGroupList {
			decoded, err := decodeDeveloperAppGroup(group)
			if err != nil {
				return nil, err
			}
			result.Data = append(result.Data, decoded)
		}

		if len(page.ApplicationGroupList) == 0 || page.TotalRecords <= len(result.Data) {
			break
		}
	}
	return result, nil
}

// CreateDeveloperAppGroup registers an App Group through Developer Portal.
func (c *Client) CreateDeveloperAppGroup(ctx context.Context, request DeveloperAppGroupCreateRequest) (*DeveloperAppGroup, error) {
	request.Name = strings.TrimSpace(request.Name)
	request.Identifier = strings.TrimSpace(request.Identifier)
	if request.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if err := validateDeveloperAppGroupIdentifier(request.Identifier); err != nil {
		return nil, err
	}
	if err := c.ensureDeveloperPortalSession(ctx); err != nil {
		return nil, err
	}
	teamID := c.developerPortalTeamID()
	if teamID == "" {
		return nil, fmt.Errorf("developer portal team is not selected; %s", developerPortalAuthHint)
	}

	body, err := c.doDeveloperPortalLegacyFormRequest(ctx, developerAppGroupsCreatePath, url.Values{
		"teamId":     {teamID},
		"name":       {request.Name},
		"identifier": {request.Identifier},
	}, true)
	if err != nil {
		return nil, err
	}
	var response developerAppGroupCreateResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse Developer Portal App Group create response: %w", err)
	}
	if err := validateDeveloperPortalLegacyResponse(response.developerPortalLegacyResponse); err != nil {
		return nil, err
	}
	group, err := decodeDeveloperAppGroup(response.ApplicationGroup)
	if err != nil {
		return nil, err
	}
	return &group, nil
}

// AssignDeveloperAppGroup associates an App Group with a Bundle ID while
// preserving Apple's complete current capability graph.
func (c *Client) AssignDeveloperAppGroup(ctx context.Context, request DeveloperAppGroupAssignRequest) (*DeveloperAppGroupAssignResult, error) {
	request.BundleID = strings.TrimSpace(request.BundleID)
	request.GroupID = strings.TrimSpace(request.GroupID)
	if request.BundleID == "" {
		return nil, fmt.Errorf("bundle id is required")
	}
	if request.GroupID == "" {
		return nil, fmt.Errorf("group id is required")
	}
	if err := c.ensureDeveloperPortalSession(ctx); err != nil {
		return nil, err
	}
	current, err := c.loadDeveloperBundleID(ctx, request.BundleID)
	if err != nil {
		return nil, err
	}
	payload, alreadyAssigned, err := buildDeveloperAppGroupAssignmentPatchRequest(current, request.GroupID)
	if err != nil {
		return nil, err
	}
	if alreadyAssigned {
		return &DeveloperAppGroupAssignResult{BundleID: request.BundleID, GroupID: request.GroupID, Changed: false, Status: "already-assigned"}, nil
	}
	payload, err = addDeveloperPortalTeamID(payload, c.developerPortalTeamID())
	if err != nil {
		return nil, err
	}
	if _, err := c.doDeveloperPortalRequest(ctx, http.MethodPatch, "/bundleIds/"+url.PathEscape(request.BundleID), payload, developerPortalHeaders(request.BundleID), true); err != nil {
		return nil, err
	}
	return &DeveloperAppGroupAssignResult{BundleID: request.BundleID, GroupID: request.GroupID, Changed: true, Status: "assigned"}, nil
}

func decodeDeveloperAppGroup(payload developerAppGroupPayload) (DeveloperAppGroup, error) {
	group := DeveloperAppGroup{
		ID:         strings.TrimSpace(payload.ApplicationGroup),
		Name:       strings.TrimSpace(payload.Name),
		Identifier: strings.TrimSpace(payload.Identifier),
		Prefix:     strings.TrimSpace(payload.Prefix),
		Status:     strings.TrimSpace(payload.Status),
	}
	if group.ID == "" || group.Identifier == "" {
		return DeveloperAppGroup{}, fmt.Errorf("incomplete App Group resource returned by Developer Portal")
	}
	return group, nil
}

func validateDeveloperAppGroupIdentifier(identifier string) error {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return fmt.Errorf("identifier is required")
	}
	if !strings.HasPrefix(identifier, "group.") {
		return fmt.Errorf("App Group identifier must start with group.")
	}
	if strings.ContainsAny(identifier, " \t\r\n") {
		return fmt.Errorf("App Group identifier must not contain whitespace")
	}
	return nil
}

func validateDeveloperPortalLegacyResponse(response developerPortalLegacyResponse) error {
	if response.ResultCode == nil {
		return fmt.Errorf("Developer Portal response is missing resultCode")
	}
	if *response.ResultCode == 0 {
		return nil
	}
	message := strings.TrimSpace(response.UserString)
	if message == "" {
		message = strings.TrimSpace(response.ResultString)
	}
	if message == "" {
		message = "unknown Developer Portal error"
	}
	if response.RequestID != "" {
		return fmt.Errorf("Developer Portal request failed (result code %d, request ID %s): %s", *response.ResultCode, response.RequestID, message)
	}
	return fmt.Errorf("Developer Portal request failed (result code %d): %s", *response.ResultCode, message)
}

func (c *Client) doDeveloperPortalLegacyFormRequest(ctx context.Context, path string, values url.Values, requireCSRF bool) ([]byte, error) {
	headers := developerPortalHeaders("")
	headers.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	headers.Set("Content-Type", "application/x-www-form-urlencoded")
	csrf, csrfTS := c.developerCSRFTokens()
	if csrf != "" {
		headers.Set("csrf", csrf)
	}
	if csrfTS != "" {
		headers.Set("csrf_ts", csrfTS)
	}
	if requireCSRF && (csrf == "" || csrfTS == "") {
		return nil, fmt.Errorf("missing Developer Portal CSRF headers; %s", developerPortalAuthHint)
	}
	body, response, err := c.doDeveloperPortalHTTP(ctx, http.MethodPost, developerPortalLegacyBaseURL+path, values, headers)
	if err != nil {
		return nil, err
	}
	c.captureDeveloperCSRFTokens(response.Header)
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return nil, developerPortalSessionError(response.StatusCode)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, &APIError{Status: response.StatusCode, AppleRequestID: extractAppleRequestID(response.Header), rawBody: body}
	}
	return body, nil
}

func buildDeveloperAppGroupAssignmentPatchRequest(current developerBundleIDResponse, groupID string) (developerBundleIDPatchRequest, bool, error) {
	capabilities, err := developerBundleIDCapabilities(current)
	if err != nil {
		return developerBundleIDPatchRequest{}, false, err
	}
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return developerBundleIDPatchRequest{}, false, fmt.Errorf("group id is required")
	}

	updated := make([]developerResource, 0, len(capabilities)+1)
	foundAppGroups := false
	for _, capability := range capabilities {
		capabilityID, err := developerBundleIDCapabilityID(capability)
		if err != nil {
			return developerBundleIDPatchRequest{}, false, err
		}
		if capabilityID != developerAppGroupsCapabilityType {
			updated = append(updated, capability)
			continue
		}
		if foundAppGroups {
			return developerBundleIDPatchRequest{}, false, fmt.Errorf("cannot safely update duplicate APP_GROUPS capability resources")
		}
		foundAppGroups = true
		enabled, err := developerBundleIDCapabilityEnabled(capability)
		if err != nil {
			return developerBundleIDPatchRequest{}, false, err
		}
		groups, err := developerAppGroupRelationships(capability)
		if err != nil {
			return developerBundleIDPatchRequest{}, false, err
		}
		for _, group := range groups {
			if group.ID == groupID && enabled {
				return developerBundleIDPatchRequest{}, true, nil
			}
		}
		capability.Attributes, err = setDeveloperCapabilityEnabled(capability.Attributes)
		if err != nil {
			return developerBundleIDPatchRequest{}, false, err
		}
		if !containsDeveloperResource(groups, "appGroups", groupID) {
			groups = append(groups, developerResource{Type: "appGroups", ID: groupID})
		}
		if err := setDeveloperAppGroupRelationships(&capability, groups); err != nil {
			return developerBundleIDPatchRequest{}, false, err
		}
		updated = append(updated, capability)
	}
	if !foundAppGroups {
		capability := newDeveloperBundleIDCapability(developerAppGroupsCapabilityType)
		if err := setDeveloperAppGroupRelationships(&capability, []developerResource{{Type: "appGroups", ID: groupID}}); err != nil {
			return developerBundleIDPatchRequest{}, false, err
		}
		updated = append(updated, capability)
	}

	relationship, err := json.Marshal(developerResourceRelationship{Data: updated})
	if err != nil {
		return developerBundleIDPatchRequest{}, false, fmt.Errorf("failed to build Bundle ID capability relationships: %w", err)
	}
	relationships := cloneRawMessageMap(current.Data.Relationships)
	if relationships == nil {
		relationships = make(map[string]json.RawMessage)
	}
	relationships["bundleIdCapabilities"] = relationship

	var payload developerBundleIDPatchRequest
	payload.Data.ID = current.Data.ID
	payload.Data.Type = current.Data.Type
	payload.Data.Attributes = append(json.RawMessage(nil), current.Data.Attributes...)
	payload.Data.Relationships = relationships
	return payload, false, nil
}

func developerAppGroupRelationships(capability developerResource) ([]developerResource, error) {
	raw, exists := capability.Relationships["appGroups"]
	if !exists {
		return []developerResource{}, nil
	}
	var relationship developerResourceRelationship
	if err := json.Unmarshal(raw, &relationship); err != nil {
		return nil, fmt.Errorf("failed to parse current App Group relationships: %w", err)
	}
	for _, group := range relationship.Data {
		if group.Type != "appGroups" || strings.TrimSpace(group.ID) == "" {
			return nil, fmt.Errorf("invalid App Group relationship returned by Developer Portal")
		}
	}
	return relationship.Data, nil
}

func setDeveloperAppGroupRelationships(capability *developerResource, groups []developerResource) error {
	encoded, err := json.Marshal(developerResourceRelationship{Data: groups})
	if err != nil {
		return fmt.Errorf("failed to encode App Group relationships: %w", err)
	}
	if capability.Relationships == nil {
		capability.Relationships = make(map[string]json.RawMessage)
	}
	capability.Relationships["appGroups"] = encoded
	return nil
}

func containsDeveloperResource(resources []developerResource, resourceType, id string) bool {
	for _, resource := range resources {
		if resource.Type == resourceType && resource.ID == id {
			return true
		}
	}
	return false
}
