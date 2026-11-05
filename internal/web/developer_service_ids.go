package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

const (
	developerServiceIDsListPlatform = "SERVICES"
	developerServiceIDsListLimit    = "1000"
	developerServiceIDsListSort     = "name"
)

var developerServiceIDDetailIncludes = []string{
	"bundleIdCapabilities",
	"bundleIdCapabilities.capability",
	"bundleIdCapabilities.appConsentBundleId",
}

// DeveloperServiceIDCreateRequest contains the writable fields accepted by
// the private Developer Portal Services ID registration form.
type DeveloperServiceIDCreateRequest struct {
	Identifier string
	Name       string
}

// DeveloperServiceIDRenameRequest changes the display name of one Services
// ID. The identifier and capability graph are read from the portal first.
type DeveloperServiceIDRenameRequest struct {
	ServiceID string
	Name      string
}

// DeveloperServiceIDDeleteRequest identifies one Services ID to remove.
type DeveloperServiceIDDeleteRequest struct {
	ServiceID string
}

// DeveloperServiceIDsListResult and DeveloperServiceIDGetResult intentionally
// reuse the open-ended JSON:API read types. Their MarshalJSON methods return
// Apple's original response envelope, including unknown members and included
// capability resources.
type (
	DeveloperServiceIDsListResult = DeveloperBundleIDsListResult
	DeveloperServiceIDGetResult   = DeveloperBundleIDGetResult
)

// DeveloperServiceIDUnverifiedError reports a write whose final state cannot
// be established. Callers must inspect the resource before retrying; the
// client never retries an ambiguous Services ID mutation automatically.
type DeveloperServiceIDUnverifiedError struct {
	Err error
}

func (e *DeveloperServiceIDUnverifiedError) Error() string {
	if e == nil || e.Err == nil {
		return "developer portal Services ID mutation outcome is unknown"
	}
	return e.Err.Error()
}

func (e *DeveloperServiceIDUnverifiedError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type developerServiceIDCreatePayload struct {
	Data developerServiceIDCreateData `json:"data"`
}

type developerServiceIDCreateData struct {
	Type          string                                `json:"type"`
	Attributes    map[string]string                     `json:"attributes"`
	Relationships developerServiceIDCreateRelationships `json:"relationships"`
}

type developerServiceIDCreateRelationships struct {
	BundleIDCapabilities developerServiceIDCreateRelationship `json:"bundleIdCapabilities"`
}

type developerServiceIDCreateRelationship struct {
	Data []any `json:"data"`
}

// ListDeveloperServiceIDs lists Services IDs through the private Developer
// Portal bundleIds resource. The endpoint is a logical GET; the cookie-auth
// transport sends the captured POST plus X-HTTP-Method-Override: GET.
func (c *Client) ListDeveloperServiceIDs(ctx context.Context) (*DeveloperServiceIDsListResult, error) {
	if err := c.ensureDeveloperPortalSession(ctx); err != nil {
		return nil, err
	}
	return c.listDeveloperServiceIDsAfterSession(ctx)
}

func (c *Client) listDeveloperServiceIDsAfterSession(ctx context.Context) (*DeveloperServiceIDsListResult, error) {
	teamID := c.developerPortalTeamID()
	if teamID == "" {
		return nil, fmt.Errorf("developer portal team is not selected; %s", developerPortalAuthHint)
	}
	// Keep the captured source order in the proxy body. The query semantics are
	// the same as url.Values.Encode, but the live frontend emits this order.
	query := "limit=" + developerServiceIDsListLimit + "&sort=" + developerServiceIDsListSort + "&filter[platform]=" + developerServiceIDsListPlatform
	headers := developerPortalHeaders("")
	headers.Set("X-HTTP-Method-Override", http.MethodGet)
	body, err := c.doDeveloperPortalRequest(ctx, http.MethodPost, "/bundleIds", developerPortalProxyReadRequest{
		URLEncodedQueryParams: query,
		TeamID:                teamID,
	}, headers, false)
	if err != nil {
		return nil, err
	}
	result, err := parseDeveloperBundleIDsListResponse(body)
	if err != nil {
		return nil, err
	}
	if err := validateDeveloperServiceIDCollection(result.Data); err != nil {
		return nil, err
	}
	return result, nil
}

// GetDeveloperServiceID reads one Services ID and its captured capability
// graph. A resource returned with another platform or another ID is rejected
// before it can be used by a mutation.
func (c *Client) GetDeveloperServiceID(ctx context.Context, serviceID string) (*DeveloperServiceIDGetResult, error) {
	serviceID = strings.TrimSpace(serviceID)
	if serviceID == "" {
		return nil, fmt.Errorf("service id is required")
	}
	if err := c.ensureDeveloperPortalSession(ctx); err != nil {
		return nil, err
	}
	return c.getDeveloperServiceIDAfterSession(ctx, serviceID)
}

func (c *Client) getDeveloperServiceIDAfterSession(ctx context.Context, serviceID string) (*DeveloperServiceIDGetResult, error) {
	query := url.Values{}
	query.Set("include", strings.Join(developerServiceIDDetailIncludes, ","))
	body, err := c.doDeveloperPortalBundleIDDetailRead(ctx, "/bundleIds/"+url.PathEscape(serviceID), serviceID, query)
	if err != nil {
		return nil, err
	}
	result, err := parseDeveloperBundleIDGetResponse(body)
	if err != nil {
		return nil, err
	}
	if err := validateDeveloperServiceIDResource(result.Data, serviceID); err != nil {
		return nil, err
	}
	return result, nil
}

// CreateDeveloperServiceID registers a minimal Services ID. Capability and
// sign-in settings are deliberately not synthesized in this lifecycle slice.
// The successful response is followed by a detail read; if Apple omits the
// created resource from the response, the list is used to converge by exact
// identifier.
func (c *Client) CreateDeveloperServiceID(ctx context.Context, request DeveloperServiceIDCreateRequest) (*asc.WebServiceIDMutationResult, error) {
	request.Identifier = strings.TrimSpace(request.Identifier)
	request.Name = strings.TrimSpace(request.Name)
	if request.Identifier == "" {
		return nil, fmt.Errorf("identifier is required")
	}
	if request.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if err := c.ensureDeveloperPortalSession(ctx); err != nil {
		return nil, err
	}
	teamID := c.developerPortalTeamID()
	if teamID == "" {
		return nil, fmt.Errorf("developer portal team is not selected; %s", developerPortalAuthHint)
	}

	payload := developerServiceIDCreatePayload{
		Data: developerServiceIDCreateData{
			Type: "bundleIds",
			Attributes: map[string]string{
				"identifier": request.Identifier,
				"platform":   developerServiceIDsListPlatform,
				"seedId":     teamID,
				"teamId":     teamID,
				"name":       request.Name,
			},
			Relationships: developerServiceIDCreateRelationships{
				BundleIDCapabilities: developerServiceIDCreateRelationship{Data: []any{}},
			},
		},
	}
	body, err := c.doDeveloperPortalRequest(ctx, http.MethodPost, "/bundleIds", payload, developerPortalHeaders(""), true)
	if err != nil {
		return nil, developerServiceIDWriteError("create", err)
	}

	serviceID, err := developerServiceIDFromCreateResponse(body)
	if err != nil {
		return nil, &DeveloperServiceIDUnverifiedError{Err: fmt.Errorf("developer portal accepted the Services ID create but its response could not be used for verification: %w", err)}
	}
	if serviceID == "" {
		serviceID, err = c.findDeveloperServiceIDByIdentifier(ctx, request.Identifier)
		if err != nil {
			return nil, &DeveloperServiceIDUnverifiedError{Err: fmt.Errorf("developer portal accepted the Services ID create but convergence by identifier failed: %w", err)}
		}
	}

	view, err := c.getDeveloperServiceIDAfterSession(ctx, serviceID)
	if err != nil {
		return nil, &DeveloperServiceIDUnverifiedError{Err: fmt.Errorf("developer portal accepted the Services ID create but verification failed: %w", err)}
	}
	if err := verifyDeveloperServiceIDIdentity(view.Data, serviceID, request.Identifier, request.Name); err != nil {
		return nil, &DeveloperServiceIDUnverifiedError{Err: fmt.Errorf("developer portal accepted the Services ID create but verification disagreed: %w", err)}
	}
	return &asc.WebServiceIDMutationResult{
		Operation:  "create",
		ServiceID:  serviceID,
		Identifier: request.Identifier,
		Name:       request.Name,
		Changed:    true,
		Verified:   true,
		Status:     "created",
	}, nil
}

// RenameDeveloperServiceID changes only the name and required private team
// attribute. It carries every relationship from the preflight detail forward,
// including the capability graph, without enabling or disabling anything.
func (c *Client) RenameDeveloperServiceID(ctx context.Context, request DeveloperServiceIDRenameRequest) (*asc.WebServiceIDMutationResult, error) {
	request.ServiceID = strings.TrimSpace(request.ServiceID)
	request.Name = strings.TrimSpace(request.Name)
	if request.ServiceID == "" {
		return nil, fmt.Errorf("service id is required")
	}
	if request.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if err := c.ensureDeveloperPortalSession(ctx); err != nil {
		return nil, err
	}
	current, err := c.loadDeveloperServiceIDResourceAfterSession(ctx, request.ServiceID)
	if err != nil {
		return nil, err
	}
	identifier, err := developerServiceIDRequiredRawAttribute(current.Data.Attributes, request.ServiceID, "identifier")
	if err != nil {
		return nil, err
	}
	currentName, err := developerServiceIDRequiredRawAttribute(current.Data.Attributes, request.ServiceID, "name")
	if err != nil {
		return nil, err
	}
	if currentName == request.Name {
		return &asc.WebServiceIDMutationResult{
			Operation:  "rename",
			ServiceID:  request.ServiceID,
			Identifier: identifier,
			Name:       request.Name,
			Changed:    false,
			Verified:   true,
			Status:     "unchanged",
		}, nil
	}
	payload, err := buildDeveloperServiceIDRenamePayload(current, request.Name, c.developerPortalTeamID())
	if err != nil {
		return nil, err
	}
	expectedCapabilityGraph, err := developerServiceIDCapabilityGraph(current)
	if err != nil {
		return nil, err
	}
	if _, err := c.doDeveloperPortalRequest(ctx, http.MethodPatch, "/bundleIds/"+url.PathEscape(request.ServiceID), payload, developerPortalHeaders(request.ServiceID), true); err != nil {
		return nil, developerServiceIDWriteError("rename", err)
	}
	view, err := c.getDeveloperServiceIDAfterSession(ctx, request.ServiceID)
	if err != nil {
		return nil, &DeveloperServiceIDUnverifiedError{Err: fmt.Errorf("developer portal accepted the Services ID rename but verification failed: %w", err)}
	}
	if err := verifyDeveloperServiceIDIdentity(view.Data, request.ServiceID, identifier, request.Name); err != nil {
		return nil, &DeveloperServiceIDUnverifiedError{Err: fmt.Errorf("developer portal accepted the Services ID rename but verification disagreed: %w", err)}
	}
	if err := verifyDeveloperServiceIDCapabilityGraph(expectedCapabilityGraph, view.Raw); err != nil {
		return nil, &DeveloperServiceIDUnverifiedError{Err: fmt.Errorf("developer portal accepted the Services ID rename but capability verification disagreed: %w", err)}
	}
	return &asc.WebServiceIDMutationResult{
		Operation:  "rename",
		ServiceID:  request.ServiceID,
		Identifier: identifier,
		Name:       request.Name,
		Changed:    true,
		Verified:   true,
		Status:     "renamed",
	}, nil
}

// DeleteDeveloperServiceID removes a Services ID after proving the requested
// resource is a SERVICES platform resource. A 404 on the post-delete detail
// read is the only successful convergence signal.
func (c *Client) DeleteDeveloperServiceID(ctx context.Context, request DeveloperServiceIDDeleteRequest) (*asc.WebServiceIDMutationResult, error) {
	request.ServiceID = strings.TrimSpace(request.ServiceID)
	if request.ServiceID == "" {
		return nil, fmt.Errorf("service id is required")
	}
	if err := c.ensureDeveloperPortalSession(ctx); err != nil {
		return nil, err
	}
	current, err := c.loadDeveloperServiceIDResourceAfterSession(ctx, request.ServiceID)
	if err != nil {
		return nil, err
	}
	identifier := developerServiceIDRawAttribute(current.Data.Attributes, "identifier")
	name := developerServiceIDRawAttribute(current.Data.Attributes, "name")
	headers := developerPortalHeaders(request.ServiceID)
	headers.Set("X-HTTP-Method-Override", http.MethodDelete)
	if _, err := c.doDeveloperPortalRequest(ctx, http.MethodPost, "/bundleIds/"+url.PathEscape(request.ServiceID), map[string]string{"teamId": c.developerPortalTeamID()}, headers, true); err != nil {
		return nil, developerServiceIDWriteError("delete", err)
	}

	if _, err := c.getDeveloperServiceIDAfterSession(ctx, request.ServiceID); err == nil {
		return nil, &DeveloperServiceIDUnverifiedError{Err: fmt.Errorf("developer portal accepted the Services ID delete but %q is still listed; inspect it before retrying", request.ServiceID)}
	} else if !isDeveloperPortalNotFound(err) {
		return nil, &DeveloperServiceIDUnverifiedError{Err: fmt.Errorf("developer portal accepted the Services ID delete but verification failed: %w", err)}
	}
	return &asc.WebServiceIDMutationResult{
		Operation:  "delete",
		ServiceID:  request.ServiceID,
		Identifier: identifier,
		Name:       name,
		Changed:    true,
		Verified:   true,
		Status:     "deleted",
	}, nil
}

func (c *Client) loadDeveloperServiceIDResourceAfterSession(ctx context.Context, serviceID string) (developerBundleIDResponse, error) {
	query := url.Values{}
	query.Set("include", strings.Join(developerServiceIDDetailIncludes, ","))
	body, err := c.doDeveloperPortalBundleIDDetailRead(ctx, "/bundleIds/"+url.PathEscape(serviceID), serviceID, query)
	if err != nil {
		return developerBundleIDResponse{}, err
	}
	var current developerBundleIDResponse
	if err := json.Unmarshal(body, &current); err != nil {
		return current, fmt.Errorf("failed to parse Developer Portal Services ID response: %w", err)
	}
	if strings.TrimSpace(current.Data.ID) == "" || current.Data.ID != serviceID || current.Data.Type != "bundleIds" {
		return current, fmt.Errorf("cannot safely use Services ID %q: Developer Portal returned resource %q of type %q", serviceID, current.Data.ID, current.Data.Type)
	}
	if err := validateDeveloperServiceIDRawAttributes(current.Data.Attributes, serviceID); err != nil {
		return current, err
	}
	return current, nil
}

func buildDeveloperServiceIDRenamePayload(current developerBundleIDResponse, name, teamID string) (developerBundleIDPatchRequest, error) {
	rawRelationship, ok := current.Data.Relationships["bundleIdCapabilities"]
	if !ok {
		return developerBundleIDPatchRequest{}, fmt.Errorf("cannot safely rename Services ID %q: Developer Portal omitted its bundleIdCapabilities relationship", current.Data.ID)
	}
	references, err := decodeStrictDeveloperRelationship(rawRelationship)
	if err != nil {
		return developerBundleIDPatchRequest{}, fmt.Errorf("cannot safely rename Services ID %q: bundleIdCapabilities relationship %w", current.Data.ID, err)
	}
	for _, reference := range references {
		if reference.Type != "bundleIdCapabilities" || strings.TrimSpace(reference.ID) == "" || reference.ID != strings.TrimSpace(reference.ID) {
			return developerBundleIDPatchRequest{}, fmt.Errorf("cannot safely rename Services ID %q: bundleIdCapabilities relationship contains an invalid reference (type %q, id %q)", current.Data.ID, reference.Type, reference.ID)
		}
	}
	// The PATCH echoes this relationship back as the complete capability set,
	// so a truncated read would silently remove capabilities.
	if err := validateServiceIDCapabilityCompleteness(rawRelationship, len(references)); err != nil {
		return developerBundleIDPatchRequest{}, fmt.Errorf("cannot safely rename Services ID %q: %w", current.Data.ID, err)
	}
	var attributes map[string]json.RawMessage
	if err := json.Unmarshal(current.Data.Attributes, &attributes); err != nil {
		return developerBundleIDPatchRequest{}, fmt.Errorf("failed to parse Services ID %q attributes for rename: %w", current.Data.ID, err)
	}
	if attributes == nil {
		return developerBundleIDPatchRequest{}, fmt.Errorf("cannot safely rename Services ID %q: attributes are missing", current.Data.ID)
	}
	encodedName, err := json.Marshal(strings.TrimSpace(name))
	if err != nil {
		return developerBundleIDPatchRequest{}, fmt.Errorf("failed to encode Services ID name: %w", err)
	}
	attributes["name"] = encodedName
	encodedAttributes, err := json.Marshal(attributes)
	if err != nil {
		return developerBundleIDPatchRequest{}, fmt.Errorf("failed to encode Services ID %q attributes: %w", current.Data.ID, err)
	}
	payload := developerBundleIDPatchRequest{}
	payload.Data.ID = current.Data.ID
	payload.Data.Type = current.Data.Type
	payload.Data.Attributes = encodedAttributes
	payload.Data.Relationships = cloneRawMessageMap(current.Data.Relationships)
	return addDeveloperPortalTeamID(payload, teamID)
}

func developerServiceIDFromCreateResponse(body []byte) (string, error) {
	if len(strings.TrimSpace(string(body))) == 0 {
		return "", nil
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return "", err
	}
	if missingJSONValue(envelope.Data) {
		return "", nil
	}
	result, err := parseDeveloperBundleIDGetResponse(body)
	if err != nil {
		return "", err
	}
	// The create response only supplies the opaque resource identity on some
	// portal responses. The authoritative platform and attributes check happens
	// on the post-create detail read below.
	return result.Data.ID, nil
}

func (c *Client) findDeveloperServiceIDByIdentifier(ctx context.Context, identifier string) (string, error) {
	result, err := c.listDeveloperServiceIDsAfterSession(ctx)
	if err != nil {
		return "", err
	}
	var match string
	for _, resource := range result.Data {
		value, ok := developerServiceIDAttribute(resource.Attributes, "identifier")
		if !ok || value != identifier {
			continue
		}
		if match != "" {
			return "", fmt.Errorf("convergence found more than one Services ID with identifier %q", identifier)
		}
		match = resource.ID
	}
	if match == "" {
		return "", fmt.Errorf("convergence found no Services ID with identifier %q", identifier)
	}
	return match, nil
}

func validateDeveloperServiceIDCollection(resources []DeveloperBundleID) error {
	for _, resource := range resources {
		if err := validateDeveloperServiceIDResource(resource, ""); err != nil {
			return fmt.Errorf("invalid Developer Portal Services ID list response: %w", err)
		}
	}
	return nil
}

func validateDeveloperServiceIDResource(resource DeveloperBundleID, requestedID string) error {
	if err := validateDeveloperBundleIDResource(resource); err != nil {
		return err
	}
	if requestedID != "" && resource.ID != requestedID {
		return fmt.Errorf("developer portal returned resource %q for requested Services ID %q", resource.ID, requestedID)
	}
	platform, ok := developerServiceIDAttribute(resource.Attributes, "platform")
	if !ok || platform != developerServiceIDsListPlatform {
		return fmt.Errorf("services ID resource %q has platform %q, want %q", resource.ID, platform, developerServiceIDsListPlatform)
	}
	return nil
}

func validateDeveloperServiceIDRawAttributes(attributes json.RawMessage, serviceID string) error {
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(attributes, &decoded); err != nil || decoded == nil {
		if err != nil {
			return fmt.Errorf("failed to parse Services ID %q attributes: %w", serviceID, err)
		}
		return fmt.Errorf("cannot safely use Services ID %q: attributes are missing", serviceID)
	}
	var platform string
	if err := json.Unmarshal(decoded["platform"], &platform); err != nil || platform != developerServiceIDsListPlatform {
		return fmt.Errorf("services ID resource %q has platform %q, want %q", serviceID, platform, developerServiceIDsListPlatform)
	}
	return nil
}

func developerServiceIDRawAttribute(attributes json.RawMessage, key string) string {
	var decoded map[string]json.RawMessage
	if json.Unmarshal(attributes, &decoded) != nil {
		return ""
	}
	var value string
	if json.Unmarshal(decoded[key], &value) != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func developerServiceIDRequiredRawAttribute(attributes json.RawMessage, serviceID, key string) (string, error) {
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(attributes, &decoded); err != nil {
		return "", fmt.Errorf("failed to parse services ID %q attributes: %w", serviceID, err)
	}
	if decoded == nil {
		return "", fmt.Errorf("services ID %q has no attributes", serviceID)
	}
	raw, ok := decoded[key]
	if !ok {
		return "", fmt.Errorf("services ID %q is missing its %s attribute", serviceID, key)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("services ID %q has a non-string %s attribute: %w", serviceID, key, err)
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("services ID %q has an empty %s attribute", serviceID, key)
	}
	return value, nil
}

func developerServiceIDAttribute(attributes map[string]any, key string) (string, bool) {
	value, ok := attributes[key].(string)
	return strings.TrimSpace(value), ok
}

type developerServiceIDCapabilitySnapshot struct {
	Enabled          string
	Settings         string
	Capability       string
	AppConsentBundle string
}

// verifyDeveloperServiceIDCapabilityGraph compares only the capability graph
// carried by the Services ID resource. Response links and meta are transport
// metadata. The capability references and settings are compared as sets, while
// the selected relationship data is compared after JSON object-key ordering is
// normalized. The original raw relationship map is still used for the PATCH.
func verifyDeveloperServiceIDCapabilityGraph(expected map[string]developerServiceIDCapabilitySnapshot, postReadRaw json.RawMessage) error {
	var postRead developerBundleIDResponse
	if err := json.Unmarshal(postReadRaw, &postRead); err != nil {
		return fmt.Errorf("failed to parse post-write Services ID response for capability verification: %w", err)
	}

	got, err := developerServiceIDCapabilityGraph(postRead)
	if err != nil {
		return fmt.Errorf("cannot inspect post-write capability graph: %w", err)
	}
	gotReferences, err := decodeStrictDeveloperRelationship(postRead.Data.Relationships["bundleIdCapabilities"])
	if err != nil {
		return fmt.Errorf("cannot inspect post-write capability graph: bundleIdCapabilities relationship %w", err)
	}
	if err := validateServiceIDCapabilityCompleteness(postRead.Data.Relationships["bundleIdCapabilities"], len(gotReferences)); err != nil {
		return fmt.Errorf("cannot inspect post-write capability graph: %w", err)
	}
	if len(expected) != len(got) {
		return fmt.Errorf("post-write capability reference count is %d, want %d", len(got), len(expected))
	}
	for key, expectedCapability := range expected {
		actual, ok := got[key]
		if !ok {
			return fmt.Errorf("post-write capability graph is missing %q", key)
		}
		if expectedCapability.Enabled != actual.Enabled {
			return fmt.Errorf("post-write capability %q enabled state changed", key)
		}
		if expectedCapability.Settings != actual.Settings {
			return fmt.Errorf("post-write capability %q settings changed", key)
		}
		if expectedCapability.Capability != actual.Capability {
			return fmt.Errorf("post-write capability %q capability linkage changed", key)
		}
		if expectedCapability.AppConsentBundle != actual.AppConsentBundle {
			return fmt.Errorf("post-write capability %q app consent linkage changed", key)
		}
	}
	return nil
}

func developerServiceIDCapabilityGraph(response developerBundleIDResponse) (map[string]developerServiceIDCapabilitySnapshot, error) {
	rawRelationship, ok := response.Data.Relationships["bundleIdCapabilities"]
	if !ok {
		return nil, fmt.Errorf("bundleIdCapabilities relationship is missing")
	}
	references, err := decodeStrictDeveloperRelationship(rawRelationship)
	if err != nil {
		return nil, fmt.Errorf("bundleIdCapabilities relationship %w", err)
	}

	includedByID := make(map[string]developerResource, len(response.Included))
	for _, resource := range response.Included {
		if resource.Type != "bundleIdCapabilities" || strings.TrimSpace(resource.ID) == "" {
			continue
		}
		if _, duplicate := includedByID[resource.ID]; duplicate {
			return nil, fmt.Errorf("included capability %q appears more than once", resource.ID)
		}
		includedByID[resource.ID] = resource
	}

	graph := make(map[string]developerServiceIDCapabilitySnapshot, len(references))
	for _, reference := range references {
		if reference.Type != "bundleIdCapabilities" || strings.TrimSpace(reference.ID) == "" || reference.ID != strings.TrimSpace(reference.ID) {
			return nil, fmt.Errorf("bundleIdCapabilities relationship contains an invalid reference (type %q, id %q)", reference.Type, reference.ID)
		}
		key := reference.Type + "#" + reference.ID
		if _, duplicate := graph[key]; duplicate {
			return nil, fmt.Errorf("bundleIdCapabilities relationship contains duplicate reference %q", reference.ID)
		}
		resource, ok := includedByID[reference.ID]
		if !ok {
			return nil, fmt.Errorf("included capability %q is missing", reference.ID)
		}
		snapshot, err := developerServiceIDCapabilitySnapshotFor(resource)
		if err != nil {
			return nil, fmt.Errorf("capability %q: %w", reference.ID, err)
		}
		graph[key] = snapshot
	}
	return graph, nil
}

func developerServiceIDCapabilitySnapshotFor(resource developerResource) (developerServiceIDCapabilitySnapshot, error) {
	enabled, err := canonicalDeveloperServiceIDAttribute(resource.Attributes, "enabled", false)
	if err != nil {
		return developerServiceIDCapabilitySnapshot{}, fmt.Errorf("enabled attribute %w", err)
	}
	settings, err := canonicalDeveloperServiceIDAttribute(resource.Attributes, "settings", true)
	if err != nil {
		return developerServiceIDCapabilitySnapshot{}, fmt.Errorf("settings attribute %w", err)
	}
	capability, err := canonicalDeveloperServiceIDRelationshipData(resource.Relationships, "capability")
	if err != nil {
		return developerServiceIDCapabilitySnapshot{}, fmt.Errorf("capability relationship %w", err)
	}
	appConsentBundle, err := canonicalDeveloperServiceIDRelationshipData(resource.Relationships, "appConsentBundleId")
	if err != nil {
		return developerServiceIDCapabilitySnapshot{}, fmt.Errorf("app consent relationship %w", err)
	}
	return developerServiceIDCapabilitySnapshot{
		Enabled:          enabled,
		Settings:         settings,
		Capability:       capability,
		AppConsentBundle: appConsentBundle,
	}, nil
}

func canonicalDeveloperServiceIDAttribute(raw json.RawMessage, key string, sortArray bool) (string, error) {
	var attributes map[string]json.RawMessage
	if err := json.Unmarshal(raw, &attributes); err != nil {
		return "", fmt.Errorf("could not be parsed: %w", err)
	}
	if attributes == nil {
		return "", fmt.Errorf("are missing")
	}
	value, ok := attributes[key]
	if !ok {
		return "<missing>", nil
	}
	encoded, err := canonicalDeveloperServiceIDValue(value, sortArray)
	if err != nil {
		return "", fmt.Errorf("could not be normalized: %w", err)
	}
	return encoded, nil
}

func canonicalDeveloperServiceIDRelationshipData(relationships map[string]json.RawMessage, key string) (string, error) {
	raw, ok := relationships[key]
	if !ok {
		return "<missing>", nil
	}
	var relationship struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &relationship); err != nil {
		return "", fmt.Errorf("could not be parsed: %w", err)
	}
	encoded, err := canonicalDeveloperServiceIDValue(relationship.Data, false)
	if err != nil {
		return "", fmt.Errorf("data could not be normalized: %w", err)
	}
	return encoded, nil
}

func canonicalDeveloperServiceIDValue(raw json.RawMessage, sortArray bool) (string, error) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return `"<missing>"`, nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("could not be parsed: %w", err)
	}
	if sortArray {
		if values, ok := value.([]any); ok {
			encodedValues := make([]string, len(values))
			for index, item := range values {
				encoded, err := json.Marshal(item)
				if err != nil {
					return "", fmt.Errorf("array item %d could not be encoded: %w", index, err)
				}
				encodedValues[index] = string(encoded)
			}
			sort.Strings(encodedValues)
			return "[" + strings.Join(encodedValues, ",") + "]", nil
		}
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("could not be encoded: %w", err)
	}
	return string(encoded), nil
}

func verifyDeveloperServiceIDIdentity(resource DeveloperBundleID, serviceID, identifier, name string) error {
	if err := validateDeveloperServiceIDResource(resource, serviceID); err != nil {
		return err
	}
	gotIdentifier, ok := developerServiceIDAttribute(resource.Attributes, "identifier")
	if !ok || gotIdentifier != identifier {
		return fmt.Errorf("identifier is %q, want %q", gotIdentifier, identifier)
	}
	gotName, ok := developerServiceIDAttribute(resource.Attributes, "name")
	if !ok || gotName != name {
		return fmt.Errorf("name is %q, want %q", gotName, name)
	}
	return nil
}

func developerServiceIDWriteError(operation string, err error) error {
	if err == nil {
		return nil
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		if apiErr.Status == http.StatusRequestTimeout || apiErr.Status >= http.StatusInternalServerError {
			return &DeveloperServiceIDUnverifiedError{Err: fmt.Errorf("developer portal Services ID %s outcome is unknown after status %d; inspect the resource before retrying: %w", operation, apiErr.Status, err)}
		}
		return err
	}
	if isAmbiguousDeveloperPortalWriteFailure(err) {
		return &DeveloperServiceIDUnverifiedError{Err: fmt.Errorf("developer portal Services ID %s outcome is unknown; inspect the resource before retrying: %w", operation, err)}
	}
	return err
}

// serviceIDCapabilityPlaceholderLimit is the paging limit Apple reports with
// the zero-total placeholder on Services ID detail reads with populated
// linkage (captured 2026-09-26), and on the appGroups relationship of a
// Bundle ID's APP_GROUPS capability (captured 2026-10-03).
const serviceIDCapabilityPlaceholderLimit = 2147483647

// validateServiceIDCapabilityCompleteness fails closed unless the relationship
// proves it carries every capability: no continuation link, and either an
// exact paging total, or Apple's captured zero-total placeholder with the
// maximum limit and resolved linkage. A limit below the returned count is
// contradictory. Missing metadata or a missing total could describe a page
// Apple truncated at an unstated default, so neither is accepted. A Services
// ID with no capabilities (captured 2026-09-28) reports empty linkage with
// paging total 0 and the maximum limit, which is proven by the exact total.
// Both the domains update and the rename use this check before and after
// their PATCH, because each PATCH replaces the relationship wholesale.
func validateServiceIDCapabilityCompleteness(raw json.RawMessage, returned int) error {
	const label = "Services ID capability graph"
	var relationship map[string]json.RawMessage
	if err := json.Unmarshal(raw, &relationship); err != nil || relationship == nil {
		return fmt.Errorf("%s could not be parsed", label)
	}
	// The shared validator owns continuation-link handling; paging totals are
	// judged below because the placeholder deliberately disagrees with them.
	linksOnly, err := json.Marshal(map[string]json.RawMessage{"links": relationship["links"]})
	if err != nil {
		return fmt.Errorf("%s links are unreadable", label)
	}
	if err = validateDeveloperRelationshipCompleteness(linksOnly, returned, 0, label); err != nil {
		return err
	}
	var meta map[string]json.RawMessage
	if rawMeta, ok := relationship["meta"]; !ok || json.Unmarshal(rawMeta, &meta) != nil || meta == nil {
		return fmt.Errorf("%s has no paging metadata proving it is complete", label)
	}
	var paging map[string]json.RawMessage
	if rawPaging, ok := meta["paging"]; !ok || json.Unmarshal(rawPaging, &paging) != nil || paging == nil {
		return fmt.Errorf("%s has no paging metadata proving it is complete", label)
	}
	rawTotal, ok := paging["total"]
	if !ok || string(rawTotal) == "null" {
		return fmt.Errorf("%s has no paging total proving it is complete", label)
	}
	var total int
	if err = json.Unmarshal(rawTotal, &total); err != nil || total < 0 {
		return fmt.Errorf("%s paging total is unreadable", label)
	}
	limit := -1
	if rawLimit, ok := paging["limit"]; ok && string(rawLimit) != "null" {
		if err = json.Unmarshal(rawLimit, &limit); err != nil || limit < 0 {
			return fmt.Errorf("%s paging limit is unreadable", label)
		}
		if limit < returned {
			return fmt.Errorf("%s returned %d resources beyond its paging limit of %d", label, returned, limit)
		}
	}
	if total == returned {
		return nil
	}
	if total == 0 && limit == serviceIDCapabilityPlaceholderLimit && returned > 0 {
		return nil
	}
	return fmt.Errorf("%s returned %d of %d resources", label, returned, total)
}
