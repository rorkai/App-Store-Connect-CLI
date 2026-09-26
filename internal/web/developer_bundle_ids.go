package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

const (
	privateCloudCompute = "PRIVATE_CLOUD_COMPUTE"
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

// DeveloperBundleIDCapabilityDisableRequest disables one supported Developer
// Portal-only capability on an existing Bundle ID resource.
type DeveloperBundleIDCapabilityDisableRequest struct {
	BundleID   string
	Capability string
}

// DeveloperBundleIDCapabilityUnverifiedError is returned when a capability
// PATCH may have been applied but the requested disabled state could not be
// proven. Callers should inspect the Bundle ID before retrying.
type DeveloperBundleIDCapabilityUnverifiedError struct {
	Err error
}

func (e *DeveloperBundleIDCapabilityUnverifiedError) Error() string { return e.Err.Error() }

func (e *DeveloperBundleIDCapabilityUnverifiedError) Unwrap() error { return e.Err }

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

const developerBundleIDCapabilitiesIncludeLimit = 50

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

	current, err := c.loadDeveloperBundleIDExact(ctx, req.BundleID)
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

// DisableDeveloperBundleIDCapability disables a supported Developer
// Portal-only Bundle ID capability and verifies the resulting graph. The
// private endpoint does not provide a reliable mutation response body, so a
// successful PATCH is accepted only after a fresh exact-resource read proves
// that no matching capability remains enabled.
func (c *Client) DisableDeveloperBundleIDCapability(ctx context.Context, req DeveloperBundleIDCapabilityDisableRequest) (*asc.DeveloperBundleIDCapabilityDisableResult, error) {
	normalized, err := normalizeDeveloperBundleIDCapabilityEnableRequest(DeveloperBundleIDCapabilityEnableRequest(req))
	if err != nil {
		return nil, err
	}
	req.BundleID = normalized.BundleID
	req.Capability = normalized.Capability
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

	current, err := c.loadDeveloperBundleIDExact(ctx, req.BundleID)
	if err != nil {
		return nil, err
	}
	before, err := developerBundleIDCapabilityDisableState(current, req.Capability)
	if err != nil {
		return nil, err
	}
	if !before.hasEnabled() {
		return &asc.DeveloperBundleIDCapabilityDisableResult{
			BundleID:   req.BundleID,
			Capability: req.Capability,
			Enabled:    false,
			Changed:    false,
			Status:     "already-disabled",
		}, nil
	}

	payload, alreadyDisabled, err := buildDeveloperBundleIDCapabilityPatchRequestForState(current, DeveloperBundleIDCapabilityEnableRequest(req), false, false)
	if err != nil {
		return nil, err
	}
	if alreadyDisabled {
		return &asc.DeveloperBundleIDCapabilityDisableResult{
			BundleID:   req.BundleID,
			Capability: req.Capability,
			Enabled:    false,
			Changed:    false,
			Status:     "already-disabled",
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
	_, writeErr := c.doDeveloperPortalRequest(ctx, http.MethodPatch, path, payload, developerPortalHeaders(req.BundleID), true)
	if writeErr == nil {
		verified, verifyErr := c.loadDeveloperBundleIDExact(ctx, req.BundleID)
		if verifyErr != nil {
			return nil, newDeveloperBundleIDUnverifiedError("Developer Portal accepted disabling %q but verification failed: %v", req.Capability, verifyErr)
		}
		after, stateErr := developerBundleIDCapabilityDisableState(verified, req.Capability)
		if stateErr != nil {
			return nil, newDeveloperBundleIDUnverifiedError("Developer Portal accepted disabling %q but verification failed: %v", req.Capability, stateErr)
		}
		if !after.isDisabled() || !before.retainsNonTargetResources(after) || (!after.sameTargetIDs(before) && !after.targetsRemoved(before)) {
			return nil, newDeveloperBundleIDUnverifiedError("Developer Portal accepted disabling %q but verification did not prove the same target resources are disabled or that Apple removed all target resources", req.Capability)
		}
		return developerBundleIDCapabilityDisableResult(req), nil
	}

	if !isAmbiguousDeveloperBundleIDCapabilityWriteFailure(writeErr) {
		return nil, writeErr
	}
	verified, verifyErr := c.loadDeveloperBundleIDExact(ctx, req.BundleID)
	if verifyErr != nil {
		return nil, newDeveloperBundleIDUnverifiedError("disabling %q may have been applied but verification also failed: %v", req.Capability, verifyErr)
	}
	after, stateErr := developerBundleIDCapabilityDisableState(verified, req.Capability)
	if stateErr != nil {
		return nil, newDeveloperBundleIDUnverifiedError("disabling %q may have been applied but verification failed: %v", req.Capability, stateErr)
	}
	switch {
	case after.isDisabled() && before.retainsNonTargetResources(after) && (after.sameTargetIDs(before) || after.targetsRemoved(before)):
		return developerBundleIDCapabilityDisableResult(req), nil
	default:
		return nil, newDeveloperBundleIDUnverifiedError("disabling %q may have been applied but verification did not prove the same target resources are disabled or that Apple removed all target resources (write error: %v)", req.Capability, writeErr)
	}
}

func developerBundleIDCapabilityDisableResult(req DeveloperBundleIDCapabilityDisableRequest) *asc.DeveloperBundleIDCapabilityDisableResult {
	return &asc.DeveloperBundleIDCapabilityDisableResult{
		BundleID:   req.BundleID,
		Capability: req.Capability,
		Enabled:    false,
		Changed:    true,
		Status:     "disabled",
	}
}

func (c *Client) loadDeveloperBundleIDExact(ctx context.Context, bundleID string) (developerBundleIDResponse, error) {
	response, err := c.loadDeveloperBundleID(ctx, bundleID)
	if err != nil {
		return developerBundleIDResponse{}, err
	}
	if response.Data.ID != bundleID {
		return developerBundleIDResponse{}, fmt.Errorf("cannot safely update Bundle ID %q: Developer Portal returned resource %q instead", bundleID, response.Data.ID)
	}
	return response, nil
}

type developerBundleIDCapabilityDisableSnapshot struct {
	EnabledByID map[string]bool
	ResourceIDs map[string]struct{}
}

func (s developerBundleIDCapabilityDisableSnapshot) hasEnabled() bool {
	for _, enabled := range s.EnabledByID {
		if enabled {
			return true
		}
	}
	return false
}

func (s developerBundleIDCapabilityDisableSnapshot) isDisabled() bool { return !s.hasEnabled() }

func (s developerBundleIDCapabilityDisableSnapshot) sameTargetIDs(other developerBundleIDCapabilityDisableSnapshot) bool {
	if len(s.EnabledByID) != len(other.EnabledByID) {
		return false
	}
	for id := range s.EnabledByID {
		if _, ok := other.EnabledByID[id]; !ok {
			return false
		}
	}
	return true
}

func (s developerBundleIDCapabilityDisableSnapshot) retainsNonTargetResources(other developerBundleIDCapabilityDisableSnapshot) bool {
	for id := range s.ResourceIDs {
		if _, target := s.EnabledByID[id]; target {
			continue
		}
		if _, retained := other.ResourceIDs[id]; !retained {
			return false
		}
	}
	return true
}

func (s developerBundleIDCapabilityDisableSnapshot) targetsRemoved(other developerBundleIDCapabilityDisableSnapshot) bool {
	return len(s.EnabledByID) == 0 && len(other.EnabledByID) > 0
}

func developerBundleIDCapabilityDisableState(current developerBundleIDResponse, capabilityID string) (developerBundleIDCapabilityDisableSnapshot, error) {
	capabilities, err := developerBundleIDCapabilitiesForDisable(current)
	if err != nil {
		return developerBundleIDCapabilityDisableSnapshot{}, err
	}
	state := developerBundleIDCapabilityDisableSnapshot{
		EnabledByID: make(map[string]bool),
		ResourceIDs: make(map[string]struct{}, len(capabilities)),
	}
	for _, capability := range capabilities {
		if strings.TrimSpace(capability.ID) == "" {
			return developerBundleIDCapabilityDisableSnapshot{}, fmt.Errorf("cannot safely verify Bundle ID capability graph without a resource id")
		}
		state.ResourceIDs[capability.ID] = struct{}{}
		id, err := developerBundleIDCapabilityID(capability)
		if err != nil {
			return developerBundleIDCapabilityDisableSnapshot{}, err
		}
		if id != capabilityID {
			continue
		}
		if strings.TrimSpace(capability.ID) == "" {
			return developerBundleIDCapabilityDisableSnapshot{}, fmt.Errorf("cannot safely update Bundle ID capability %q without a resource id", capabilityID)
		}
		enabled, present, err := developerBundleIDCapabilityEnabledValue(capability)
		if err != nil {
			return developerBundleIDCapabilityDisableSnapshot{}, err
		}
		if !present {
			return developerBundleIDCapabilityDisableSnapshot{}, fmt.Errorf("bundle ID capability %q resource %q is missing an enabled state", capabilityID, capability.ID)
		}
		state.EnabledByID[capability.ID] = enabled
	}
	return state, nil
}

func newDeveloperBundleIDUnverifiedError(format string, args ...any) error {
	return &DeveloperBundleIDCapabilityUnverifiedError{Err: fmt.Errorf(format+"; no automatic retry was sent; inspect the Bundle ID before retrying", args...)}
}

func isAmbiguousDeveloperBundleIDCapabilityWriteFailure(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Status == http.StatusRequestTimeout || apiErr.Status >= http.StatusInternalServerError
	}
	return isAmbiguousDeveloperPortalWriteFailure(err) || errors.Is(err, io.ErrUnexpectedEOF)
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
	query.Set("fields[bundleIds]", "name,identifier,platform,seedId,wildcard,bundleIdCapabilities,~permissions.delete,~permissions.edit")
	query.Set("include", strings.Join(developerBundleIDIncludes, ","))
	query.Set("limit[bundleIdCapabilities]", strconv.Itoa(developerBundleIDCapabilitiesIncludeLimit))
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
	return buildDeveloperBundleIDCapabilityPatchRequestForState(current, req, true, true)
}

// buildDeveloperBundleIDCapabilityPatchRequestForState builds the shared
// capability graph patch. The enable path keeps its historical first-target
// duplicate behavior; disabling updates every matching enabled resource and
// never synthesizes or drops a disabled resource.
func buildDeveloperBundleIDCapabilityPatchRequestForState(current developerBundleIDResponse, req DeveloperBundleIDCapabilityEnableRequest, desiredEnabled, addIfMissing bool) (developerBundleIDPatchRequest, bool, error) {
	capabilities, err := developerBundleIDCapabilities(current)
	if err != nil {
		return developerBundleIDPatchRequest{}, false, err
	}

	capabilityIDs := make([]string, len(capabilities))
	targetIndex := -1
	targetEnabled := false
	hasTarget := false
	hasEnabledTarget := false
	for index, capability := range capabilities {
		capabilityID, err := developerBundleIDCapabilityID(capability)
		if err != nil {
			return developerBundleIDPatchRequest{}, false, err
		}
		capabilityIDs[index] = capabilityID
		if capabilityID != req.Capability {
			continue
		}
		hasTarget = true
		var enabled bool
		if desiredEnabled {
			enabled, err = developerBundleIDCapabilityEnabled(capability)
			if err != nil {
				return developerBundleIDPatchRequest{}, false, err
			}
		} else {
			var present bool
			enabled, present, err = developerBundleIDCapabilityEnabledValue(capability)
			if err != nil {
				return developerBundleIDPatchRequest{}, false, err
			}
			if !present {
				return developerBundleIDPatchRequest{}, false, fmt.Errorf("bundle ID capability %q resource %q is missing an enabled state", req.Capability, capability.ID)
			}
		}
		if enabled {
			hasEnabledTarget = true
		}
		if targetIndex == -1 || (enabled && !targetEnabled) {
			targetIndex = index
			targetEnabled = enabled
		}
	}
	if desiredEnabled && targetEnabled {
		return developerBundleIDPatchRequest{}, true, nil
	}
	if !desiredEnabled && (!hasTarget || !hasEnabledTarget) {
		return developerBundleIDPatchRequest{}, true, nil
	}

	updated := make([]developerResource, 0, len(capabilities)+1)
	for index, capability := range capabilities {
		if capabilityIDs[index] != req.Capability {
			updated = append(updated, capability)
			continue
		}
		if desiredEnabled {
			if index != targetIndex {
				continue
			}
			var err error
			capability.Attributes, err = setDeveloperCapabilityEnabledValue(capability.Attributes, desiredEnabled)
			if err != nil {
				return developerBundleIDPatchRequest{}, false, err
			}
			updated = append(updated, capability)
			continue
		}
		enabled, present, err := developerBundleIDCapabilityEnabledValue(capability)
		if err != nil {
			return developerBundleIDPatchRequest{}, false, err
		}
		if !present {
			return developerBundleIDPatchRequest{}, false, fmt.Errorf("bundle ID capability %q resource %q is missing an enabled state", req.Capability, capability.ID)
		}
		if enabled {
			capability.Attributes, err = setDeveloperCapabilityEnabledValue(capability.Attributes, desiredEnabled)
			if err != nil {
				return developerBundleIDPatchRequest{}, false, err
			}
		}
		updated = append(updated, capability)
	}
	if targetIndex == -1 && addIfMissing {
		updated = append(updated, newDeveloperBundleIDCapability(req.Capability))
	}

	relationshipBody, err := marshalDeveloperBundleIDCapabilitiesForPatch(updated)
	if err != nil {
		return developerBundleIDPatchRequest{}, false, err
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
	for key := range attributes {
		if key == "permissions" || strings.HasPrefix(key, "~permissions.") {
			delete(attributes, key)
		}
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

func marshalDeveloperBundleIDCapabilitiesForPatch(capabilities []developerResource) (json.RawMessage, error) {
	for index := range capabilities {
		if len(capabilities[index].Attributes) == 0 {
			continue
		}
		var attributes map[string]json.RawMessage
		if err := json.Unmarshal(capabilities[index].Attributes, &attributes); err != nil {
			return nil, fmt.Errorf("failed to parse Bundle ID capability %q attributes for patch: %w", capabilities[index].ID, err)
		}
		writable := make(map[string]json.RawMessage, 2)
		for _, key := range []string{"enabled", "settings"} {
			if value, ok := attributes[key]; ok {
				writable[key] = append(json.RawMessage(nil), value...)
			}
		}
		encoded, err := json.Marshal(writable)
		if err != nil {
			return nil, fmt.Errorf("failed to encode Bundle ID capability %q attributes for patch: %w", capabilities[index].ID, err)
		}
		capabilities[index].Attributes = encoded
	}

	encoded, err := json.Marshal(developerResourceRelationship{Data: capabilities})
	if err != nil {
		return nil, fmt.Errorf("failed to build Bundle ID capability relationships: %w", err)
	}
	return encoded, nil
}

func developerBundleIDCapabilities(current developerBundleIDResponse) ([]developerResource, error) {
	references, err := developerBundleIDCapabilityReferencesForReplacement(current)
	if err != nil {
		return nil, err
	}
	includedByID, err := indexDeveloperCapabilityResources(current.Included)
	if err != nil {
		return nil, fmt.Errorf("cannot safely update Bundle ID %q: %w", current.Data.ID, err)
	}
	referenced := make(map[string]struct{}, len(references))
	for _, reference := range references {
		referenced[reference.ID] = struct{}{}
	}
	for _, resource := range current.Included {
		if resource.Type != "bundleIdCapabilities" {
			continue
		}
		if strings.TrimSpace(resource.ID) == "" {
			return nil, fmt.Errorf("cannot safely update Bundle ID %q: included capability has no id", current.Data.ID)
		}
		if _, ok := referenced[resource.ID]; !ok {
			return nil, fmt.Errorf("cannot safely update Bundle ID %q: Developer Portal included capability %q that the Bundle ID does not reference", current.Data.ID, resource.ID)
		}
	}

	capabilities := make([]developerResource, 0, len(references))
	seen := make(map[string]struct{}, len(references))
	for _, reference := range references {
		if _, duplicate := seen[reference.ID]; duplicate {
			continue
		}
		seen[reference.ID] = struct{}{}
		resource := reference
		if included, ok := includedByID[reference.ID]; ok {
			resource = included
		}
		if _, err := developerBundleIDCapabilityID(resource); err != nil {
			return nil, fmt.Errorf("cannot safely preserve Bundle ID capability %q: relationship resource is incomplete: %w", resource.ID, err)
		}
		capabilities = append(capabilities, resource)
	}
	return capabilities, nil
}

// developerBundleIDCapabilitiesForDisable applies the same complete ownership
// boundary used to build replacement PATCHes. A disable verification cannot
// trust included resources that the selected relationship does not own.
func developerBundleIDCapabilitiesForDisable(current developerBundleIDResponse) ([]developerResource, error) {
	return developerBundleIDCapabilities(current)
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

func developerBundleIDCapabilityEnabledValue(resource developerResource) (bool, bool, error) {
	if len(resource.Attributes) == 0 {
		return false, false, fmt.Errorf("bundle ID capability %q is missing attributes", resource.ID)
	}
	var attributes map[string]json.RawMessage
	if err := json.Unmarshal(resource.Attributes, &attributes); err != nil {
		return false, false, fmt.Errorf("failed to parse Bundle ID capability %q attributes: %w", resource.ID, err)
	}
	raw, ok := attributes["enabled"]
	if !ok {
		return false, false, nil
	}
	if strings.TrimSpace(string(raw)) == "null" {
		return false, true, fmt.Errorf("failed to parse Bundle ID capability %q enabled state: value is null", resource.ID)
	}
	var enabled bool
	if err := json.Unmarshal(raw, &enabled); err != nil {
		return false, true, fmt.Errorf("failed to parse Bundle ID capability %q enabled state: %w", resource.ID, err)
	}
	return enabled, true, nil
}

func setDeveloperCapabilityEnabledValue(raw json.RawMessage, enabled bool) (json.RawMessage, error) {
	var attributes map[string]json.RawMessage
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &attributes); err != nil {
			return nil, fmt.Errorf("failed to parse existing capability attributes: %w", err)
		}
	}
	if attributes == nil {
		attributes = make(map[string]json.RawMessage)
	}
	attributes["enabled"] = json.RawMessage(strconv.FormatBool(enabled))
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
