package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

const (
	developerWebsitePushIDResourceType         = "websitepushIds"
	developerWebsitePushIDCapabilityType       = "websitepushIdCapabilities"
	developerWebsitePushIDCapabilitiesPath     = "/capabilities?filter[referenceType]=websitepushId"
	developerWebsitePushIDDetailInclude        = "websitepushIdCapabilities"
	developerWebsitePushIDUnexpectedStatusHint = "the write outcome must be verified before retrying"
)

var (
	websitePushIDIdentifierPattern = regexp.MustCompile(`^[A-Za-z0-9.\-]+$`)
	websitePushIDNamePattern       = regexp.MustCompile(`^[0-9A-Za-z ]+$`)
)

// DeveloperWebsitePushIDResource is one modern Developer Portal Website Push
// ID JSON:API resource. Attributes and relationships remain open-ended so the
// CLI can preserve fields Apple adds without guessing their meaning.
type DeveloperWebsitePushIDResource struct {
	ID            string                                        `json:"id"`
	Type          string                                        `json:"type"`
	Attributes    map[string]any                                `json:"attributes,omitempty"`
	Relationships map[string]DeveloperWebsitePushIDRelationship `json:"relationships,omitempty"`
	Links         map[string]any                                `json:"links,omitempty"`
}

// DeveloperWebsitePushIDRelationship preserves a JSON:API relationship's
// raw data, links, and metadata. Data may be an array, object, or null.
type DeveloperWebsitePushIDRelationship struct {
	Data  json.RawMessage `json:"data,omitempty"`
	Links map[string]any  `json:"links,omitempty"`
	Meta  map[string]any  `json:"meta,omitempty"`
}

// DeveloperWebsitePushIDGetResult is the modern single-resource response.
// Raw preserves Apple's complete JSON:API envelope for JSON output.
type DeveloperWebsitePushIDGetResult struct {
	Data     DeveloperWebsitePushIDResource   `json:"data"`
	Included []DeveloperWebsitePushIDResource `json:"included,omitempty"`
	Links    map[string]any                   `json:"links,omitempty"`
	Meta     map[string]any                   `json:"meta,omitempty"`
	Raw      json.RawMessage                  `json:"-"`
}

// MarshalJSON preserves Apple's complete detail envelope when available.
func (r DeveloperWebsitePushIDGetResult) MarshalJSON() ([]byte, error) {
	if len(r.Raw) > 0 {
		return append([]byte(nil), r.Raw...), nil
	}
	type resultWithoutMethods DeveloperWebsitePushIDGetResult
	return json.Marshal(resultWithoutMethods(r))
}

// DeveloperWebsitePushIDCreateRequest contains the user-controlled values for
// a Website Push ID registration. Capability configuration is intentionally not
// exposed until Apple's capability graph has a captured writable contract.
type DeveloperWebsitePushIDCreateRequest struct {
	Name       string
	Identifier string
}

// DeveloperWebsitePushIDDeleteRequest identifies an opaque modern resource ID.
type DeveloperWebsitePushIDDeleteRequest struct {
	WebsitePushID string
}

// DeveloperWebsitePushIDUnverifiedError means Apple may have applied the
// mutation but the follow-up read could not establish its final state. Callers
// must inspect the resource before retrying; this client never retries writes.
type DeveloperWebsitePushIDUnverifiedError struct {
	Err error
}

func (e *DeveloperWebsitePushIDUnverifiedError) Error() string {
	const warning = "Website Push ID mutation may have succeeded; inspect the selected team before retrying"
	if e == nil || e.Err == nil {
		return warning
	}
	return warning + ": " + e.Err.Error()
}

func (e *DeveloperWebsitePushIDUnverifiedError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Validate checks the source-backed Website Push ID input constraints.
func (r DeveloperWebsitePushIDCreateRequest) Validate() error {
	_, err := r.validate()
	return err
}

func (r DeveloperWebsitePushIDCreateRequest) validate() (DeveloperWebsitePushIDCreateRequest, error) {
	r.Name = strings.TrimSpace(r.Name)
	r.Identifier = strings.TrimSpace(r.Identifier)
	switch {
	case r.Name == "":
		return r, fmt.Errorf("name is required")
	case len(r.Name) > 50:
		return r, fmt.Errorf("name must be at most 50 characters")
	case !websitePushIDNamePattern.MatchString(r.Name):
		return r, fmt.Errorf("name may contain only letters, numbers, and spaces")
	case r.Identifier == "":
		return r, fmt.Errorf("identifier is required")
	case len(r.Identifier) > 155:
		return r, fmt.Errorf("identifier must be at most 155 characters")
	case !websitePushIDIdentifierPattern.MatchString(r.Identifier):
		return r, fmt.Errorf("identifier may contain only letters, numbers, periods, and hyphens")
	default:
		return r, nil
	}
}

// Validate checks that a delete request names an opaque resource ID.
func (r DeveloperWebsitePushIDDeleteRequest) Validate() error {
	_, err := r.validate()
	return err
}

func (r DeveloperWebsitePushIDDeleteRequest) validate() (DeveloperWebsitePushIDDeleteRequest, error) {
	r.WebsitePushID = strings.TrimSpace(r.WebsitePushID)
	if r.WebsitePushID == "" {
		return r, fmt.Errorf("website push id is required")
	}
	return r, nil
}

// GetDeveloperWebsitePushID reads one modern Website Push ID resource and its
// captured capability relationship through the Developer Portal web session.
func (c *Client) GetDeveloperWebsitePushID(ctx context.Context, websitePushID string) (*DeveloperWebsitePushIDGetResult, error) {
	websitePushID = strings.TrimSpace(websitePushID)
	if websitePushID == "" {
		return nil, fmt.Errorf("website push id is required")
	}
	if err := c.ensureDeveloperPortalSession(ctx); err != nil {
		return nil, err
	}
	return c.getDeveloperWebsitePushIDAfterSession(ctx, websitePushID)
}

func (c *Client) getDeveloperWebsitePushIDAfterSession(ctx context.Context, websitePushID string) (*DeveloperWebsitePushIDGetResult, error) {
	teamID := c.developerPortalTeamID()
	if teamID == "" {
		return nil, fmt.Errorf("developer portal team is not selected; %s", developerPortalAuthHint)
	}
	path := "/websitepushIds/" + url.PathEscape(websitePushID) + "?include=" + developerWebsitePushIDDetailInclude
	headers := developerPortalHeaders("")
	headers.Set("Referer", developerPortalBaseURL+"/account/resources/identifiers/websitePushId/edit/"+url.PathEscape(websitePushID))
	headers.Set("X-HTTP-Method-Override", http.MethodGet)
	body, err := c.doDeveloperWebsitePushIDRequest(ctx, path, developerPortalTeamIDRequest{TeamID: teamID}, headers, false, http.StatusOK)
	if err != nil {
		return nil, err
	}
	result, err := parseDeveloperWebsitePushIDGetResponse(body)
	if err != nil {
		return nil, err
	}
	if err := validateDeveloperWebsitePushIDResource(result.Data, websitePushID); err != nil {
		return nil, fmt.Errorf("invalid Developer Portal Website Push ID response: %w", err)
	}
	return result, nil
}

// CreateDeveloperWebsitePushID registers one Website Push ID using the modern
// JSON:API endpoint. It sends an explicitly empty capability relationship and
// refuses to write when the account's capability catalog or response graph is
// not empty and therefore cannot be preserved safely by this slice.
func (c *Client) CreateDeveloperWebsitePushID(ctx context.Context, request DeveloperWebsitePushIDCreateRequest) (*asc.WebWebsitePushIDMutationResult, error) {
	request, err := request.validate()
	if err != nil {
		return nil, err
	}
	if err := c.ensureDeveloperPortalSession(ctx); err != nil {
		return nil, err
	}
	teamID := c.developerPortalTeamID()
	if teamID == "" {
		return nil, fmt.Errorf("developer portal team is not selected; %s", developerPortalAuthHint)
	}

	before, err := c.listDeveloperWebsitePushIDsAfterSession(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot safely preflight Website Push ID creation: %w", err)
	}
	if err := validateWebsitePushIDLegacyListForWrite(before); err != nil {
		return nil, err
	}
	for _, entry := range before.WebsitePushIDList {
		if websitePushIDLegacyString(entry, "identifier", "websitePushId") == "" {
			return nil, fmt.Errorf("developer portal Website Push ID list contains an unrecognized identifier; refusing to create without a complete collision read")
		}
	}
	if matches := findWebsitePushIDLegacyMatches(before, "", request.Identifier); len(matches) > 0 {
		return nil, fmt.Errorf("website push ID identifier %q already exists in the selected Developer Portal team", request.Identifier)
	}
	if err := c.requireEmptyWebsitePushIDCapabilityCatalog(ctx); err != nil {
		return nil, err
	}

	payload := developerWebsitePushIDCreatePayload{
		Data: developerWebsitePushIDCreateData{
			Type: developerWebsitePushIDResourceType,
			Attributes: map[string]string{
				"name":       request.Name,
				"identifier": request.Identifier,
				"teamId":     teamID,
			},
			Relationships: map[string]developerWebsitePushIDCreateRelationship{
				developerWebsitePushIDCapabilityType: {Data: []json.RawMessage{}},
			},
		},
	}
	body, err := c.doDeveloperWebsitePushIDRequest(ctx, "/websitepushIds", payload, developerPortalHeaders(""), true, http.StatusCreated)
	if err != nil {
		if !isAmbiguousDeveloperPortalWriteFailure(err) && !isDeveloperWebsitePushIDUnexpectedStatus(err) && !isDeveloperWebsitePushIDAmbiguousHTTPError(err) {
			return nil, err
		}
		return c.settleCreatedWebsitePushID(ctx, before, request, err)
	}

	createdID, parseErr := parseDeveloperWebsitePushIDCreateID(body)
	if parseErr != nil {
		return c.settleCreatedWebsitePushID(ctx, before, request, parseErr)
	}
	if createdID == "" {
		createdID, err = c.discoverCreatedWebsitePushID(ctx, before, request.Identifier)
		if err != nil {
			return nil, &DeveloperWebsitePushIDUnverifiedError{Err: err}
		}
	}
	return c.verifyCreatedWebsitePushID(ctx, createdID, request)
}

// DeleteDeveloperWebsitePushID deletes one modern Website Push ID only when
// its detail response proves it is deletable and has no attached capability
// references. A successful 204 is followed by a canonical detail read and the
// legacy list read used by the existing Website Push list command.
func (c *Client) DeleteDeveloperWebsitePushID(ctx context.Context, request DeveloperWebsitePushIDDeleteRequest) (*asc.WebWebsitePushIDMutationResult, error) {
	request, err := request.validate()
	if err != nil {
		return nil, err
	}
	if err := c.ensureDeveloperPortalSession(ctx); err != nil {
		return nil, err
	}
	current, err := c.getDeveloperWebsitePushIDAfterSession(ctx, request.WebsitePushID)
	if err != nil {
		return nil, err
	}
	if err := validateWebsitePushIDDeletePreflight(current); err != nil {
		return nil, err
	}
	identifier := websitePushIDStringAttribute(current.Data.Attributes, "identifier")
	teamID := c.developerPortalTeamID()
	headers := developerPortalHeaders("")
	headers.Set("Referer", developerPortalBaseURL+"/account/resources/identifiers/websitePushId/edit/"+url.PathEscape(request.WebsitePushID))
	headers.Set("X-HTTP-Method-Override", http.MethodDelete)
	body, err := c.doDeveloperWebsitePushIDRequest(ctx, "/websitepushIds/"+url.PathEscape(request.WebsitePushID), developerPortalTeamIDRequest{TeamID: teamID}, headers, true, http.StatusNoContent)
	if err != nil {
		if !isAmbiguousDeveloperPortalWriteFailure(err) && !isDeveloperWebsitePushIDUnexpectedStatus(err) && !isDeveloperWebsitePushIDAmbiguousHTTPError(err) {
			return nil, err
		}
		return c.settleDeletedWebsitePushID(ctx, request.WebsitePushID, identifier, websitePushIDStringAttribute(current.Data.Attributes, "name"), err)
	}
	if len(bytes.TrimSpace(body)) != 0 {
		return c.settleDeletedWebsitePushID(ctx, request.WebsitePushID, identifier, websitePushIDStringAttribute(current.Data.Attributes, "name"), fmt.Errorf("developer portal returned a body with HTTP 204; %s", developerWebsitePushIDUnexpectedStatusHint))
	}
	if err := c.verifyDeletedWebsitePushID(ctx, request.WebsitePushID, identifier); err != nil {
		return nil, &DeveloperWebsitePushIDUnverifiedError{Err: fmt.Errorf("developer portal accepted the Website Push ID delete but verification failed: %w", err)}
	}
	return websitePushIDMutationReceipt("delete", request.WebsitePushID, identifier, websitePushIDStringAttribute(current.Data.Attributes, "name"), "deleted"), nil
}

type developerWebsitePushIDCreatePayload struct {
	Data developerWebsitePushIDCreateData `json:"data"`
}

type developerWebsitePushIDCreateData struct {
	Type          string                                              `json:"type"`
	Attributes    map[string]string                                   `json:"attributes"`
	Relationships map[string]developerWebsitePushIDCreateRelationship `json:"relationships"`
}

type developerWebsitePushIDCreateRelationship struct {
	Data []json.RawMessage `json:"data"`
}

type developerWebsitePushIDCreateResponse struct {
	Data json.RawMessage `json:"data"`
}

type developerWebsitePushIDCapabilityResponse struct {
	Data  json.RawMessage `json:"data"`
	Meta  map[string]any  `json:"meta"`
	Links map[string]any  `json:"links"`
}

type developerWebsitePushIDUnexpectedStatusError struct {
	Status   int
	Expected int
}

func (e *developerWebsitePushIDUnexpectedStatusError) Error() string {
	return fmt.Sprintf("Developer Portal Website Push ID request returned HTTP %d, want HTTP %d; %s", e.Status, e.Expected, developerWebsitePushIDUnexpectedStatusHint)
}

func isDeveloperWebsitePushIDUnexpectedStatus(err error) bool {
	var statusErr *developerWebsitePushIDUnexpectedStatusError
	return errors.As(err, &statusErr)
}

func isDeveloperWebsitePushIDAmbiguousHTTPError(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && (apiErr.Status == http.StatusRequestTimeout || (apiErr.Status >= http.StatusInternalServerError && apiErr.Status < 600))
}

func (c *Client) doDeveloperWebsitePushIDRequest(ctx context.Context, path string, body any, headers http.Header, requireCSRF bool, expectedStatus int) ([]byte, error) {
	if err := c.applyDeveloperPortalCSRF(headers, requireCSRF); err != nil {
		return nil, err
	}
	responseBody, response, err := c.doDeveloperPortalHTTP(ctx, http.MethodPost, c.developerPortalOrigin()+developerServicesPath+path, body, headers)
	if err != nil {
		return nil, err
	}
	c.captureDeveloperCSRFTokens(response.Header)
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return nil, developerPortalSessionError(response, responseBody)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, &APIError{
			Status:         response.StatusCode,
			AppleRequestID: extractAppleRequestID(response.Header),
			CorrelationKey: strings.TrimSpace(response.Header.Get("X-Apple-Jingle-Correlation-Key")),
			rawBody:        responseBody,
		}
	}
	if expectedStatus != 0 && response.StatusCode != expectedStatus {
		return nil, &developerWebsitePushIDUnexpectedStatusError{Status: response.StatusCode, Expected: expectedStatus}
	}
	return responseBody, nil
}

func (c *Client) requireEmptyWebsitePushIDCapabilityCatalog(ctx context.Context) error {
	teamID := c.developerPortalTeamID()
	if teamID == "" {
		return fmt.Errorf("developer portal team is not selected; %s", developerPortalAuthHint)
	}
	headers := developerPortalHeaders("")
	headers.Set("X-HTTP-Method-Override", http.MethodGet)
	body, err := c.doDeveloperWebsitePushIDRequest(ctx, developerWebsitePushIDCapabilitiesPath, developerPortalTeamIDRequest{TeamID: teamID}, headers, false, http.StatusOK)
	if err != nil {
		return fmt.Errorf("cannot safely determine Website Push ID capabilities: %w", err)
	}
	var response developerWebsitePushIDCapabilityResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to parse Developer Portal Website Push ID capabilities response: %w", err)
	}
	if err := validateEmptyWebsitePushIDCapabilityPaging(response.Meta, response.Links); err != nil {
		return err
	}
	if missingJSONValue(response.Data) {
		return fmt.Errorf("cannot safely determine Website Push ID capabilities: response has no data collection")
	}
	var capabilities []json.RawMessage
	if err := json.Unmarshal(response.Data, &capabilities); err != nil || capabilities == nil {
		return fmt.Errorf("cannot safely determine Website Push ID capabilities: response data is not an array")
	}
	if len(capabilities) > 0 {
		return fmt.Errorf("website push ID capability graph is non-empty; this command refuses to discard or guess capability configuration")
	}
	return nil
}

func parseDeveloperWebsitePushIDGetResponse(body []byte) (*DeveloperWebsitePushIDGetResult, error) {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("failed to parse Developer Portal Website Push ID response: %w", err)
	}
	if missingJSONValue(envelope.Data) {
		return nil, fmt.Errorf("developer portal Website Push ID response has no data resource")
	}
	var result DeveloperWebsitePushIDGetResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse Developer Portal Website Push ID response: %w", err)
	}
	if result.Included == nil {
		result.Included = []DeveloperWebsitePushIDResource{}
	}
	result.Raw = append(json.RawMessage(nil), body...)
	return &result, nil
}

func parseDeveloperWebsitePushIDCreateID(body []byte) (string, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return "", nil
	}
	var response developerWebsitePushIDCreateResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to parse Developer Portal Website Push ID create response: %w", err)
	}
	if missingJSONValue(response.Data) {
		return "", nil
	}
	var resource DeveloperWebsitePushIDResource
	if err := json.Unmarshal(response.Data, &resource); err != nil {
		return "", fmt.Errorf("failed to parse Developer Portal Website Push ID create resource: %w", err)
	}
	if err := validateDeveloperWebsitePushIDResource(resource, ""); err != nil {
		return "", fmt.Errorf("invalid Developer Portal Website Push ID create resource: %w", err)
	}
	return resource.ID, nil
}

func validateDeveloperWebsitePushIDResource(resource DeveloperWebsitePushIDResource, expectedID string) error {
	if strings.TrimSpace(resource.ID) == "" {
		return fmt.Errorf("website push ID resource is missing id")
	}
	if strings.TrimSpace(resource.Type) != developerWebsitePushIDResourceType {
		return fmt.Errorf("website push ID resource %q has type %q, want %s", resource.ID, resource.Type, developerWebsitePushIDResourceType)
	}
	if expectedID != "" && resource.ID != expectedID {
		return fmt.Errorf("website push ID response returned resource %q, want %q", resource.ID, expectedID)
	}
	return nil
}

func validateWebsitePushIDDeletePreflight(result *DeveloperWebsitePushIDGetResult) error {
	if result == nil {
		return fmt.Errorf("developer portal Website Push ID response is empty")
	}
	if identifier := websitePushIDStringAttribute(result.Data.Attributes, "identifier"); identifier == "" {
		return fmt.Errorf("cannot safely delete Website Push ID %q: response is missing identifier", result.Data.ID)
	}
	canDelete, ok := websitePushIDBoolAttribute(result.Data.Attributes, "canDelete")
	if !ok || !canDelete {
		return fmt.Errorf("website push ID %q is not deletable by the selected Developer Portal session", result.Data.ID)
	}
	return validateEmptyWebsitePushIDCapabilityGraph(result)
}

func validateEmptyWebsitePushIDCapabilityGraph(result *DeveloperWebsitePushIDGetResult) error {
	if result == nil {
		return fmt.Errorf("website push ID response is empty")
	}
	if len(result.Included) > 0 {
		return fmt.Errorf("website push ID capability graph is present; this command refuses to discard or guess capability configuration")
	}
	relationship, ok := result.Data.Relationships[developerWebsitePushIDCapabilityType]
	if !ok || missingJSONValue(relationship.Data) {
		return fmt.Errorf("website push ID capability relationship is missing or unreadable; refusing to assume it is empty")
	}
	if err := validateEmptyWebsitePushIDCapabilityPaging(relationship.Meta, relationship.Links); err != nil {
		return err
	}
	var references []json.RawMessage
	if err := json.Unmarshal(relationship.Data, &references); err != nil || references == nil {
		return fmt.Errorf("website push ID capability relationship is not an array; refusing to assume it is empty")
	}
	if len(references) > 0 {
		return fmt.Errorf("website push ID capability graph is non-empty; remove attached capabilities before deleting")
	}
	return nil
}

func validateEmptyWebsitePushIDCapabilityPaging(meta, links map[string]any) error {
	if next, present := links["next"]; present && next != nil {
		if value, ok := next.(string); !ok || strings.TrimSpace(value) != "" {
			return fmt.Errorf("website push ID capability page has a next link; cannot establish an empty graph")
		}
	}
	if rawPaging, present := meta["paging"]; present {
		paging, ok := rawPaging.(map[string]any)
		if !ok {
			return fmt.Errorf("website push ID capability paging metadata is unreadable")
		}
		total, ok := paging["total"].(float64)
		if !ok || total != 0 {
			return fmt.Errorf("website push ID capability paging total does not establish an empty graph")
		}
	}
	return nil
}

func (c *Client) verifyCreatedWebsitePushID(ctx context.Context, websitePushID string, request DeveloperWebsitePushIDCreateRequest) (*asc.WebWebsitePushIDMutationResult, error) {
	result, err := c.getDeveloperWebsitePushIDAfterSession(ctx, websitePushID)
	if err != nil {
		return nil, &DeveloperWebsitePushIDUnverifiedError{Err: fmt.Errorf("developer portal accepted the Website Push ID create but verification read failed: %w", err)}
	}
	if got := websitePushIDStringAttribute(result.Data.Attributes, "identifier"); got != request.Identifier {
		return nil, &DeveloperWebsitePushIDUnverifiedError{Err: fmt.Errorf("developer portal created Website Push ID %q with identifier %q, want %q", websitePushID, got, request.Identifier)}
	}
	if got := websitePushIDStringAttribute(result.Data.Attributes, "name"); got != request.Name {
		return nil, &DeveloperWebsitePushIDUnverifiedError{Err: fmt.Errorf("developer portal created Website Push ID %q with name %q, want %q", websitePushID, got, request.Name)}
	}
	if err := validateEmptyWebsitePushIDCapabilityGraph(result); err != nil {
		return nil, &DeveloperWebsitePushIDUnverifiedError{Err: fmt.Errorf("developer portal created Website Push ID %q with an unreadable capability graph: %w", websitePushID, err)}
	}
	return websitePushIDMutationReceipt("create", websitePushID, request.Identifier, request.Name, "created"), nil
}

func (c *Client) settleCreatedWebsitePushID(ctx context.Context, before *DeveloperWebsitePushIDsListResult, request DeveloperWebsitePushIDCreateRequest, cause error) (*asc.WebWebsitePushIDMutationResult, error) {
	websitePushID, err := c.discoverCreatedWebsitePushID(ctx, before, request.Identifier)
	if err != nil {
		return nil, &DeveloperWebsitePushIDUnverifiedError{Err: fmt.Errorf("%w; could not settle the create outcome: %w", cause, err)}
	}
	result, err := c.verifyCreatedWebsitePushID(ctx, websitePushID, request)
	if err != nil {
		return nil, &DeveloperWebsitePushIDUnverifiedError{Err: fmt.Errorf("%w; could not verify the created resource: %w", cause, err)}
	}
	return result, nil
}

func (c *Client) discoverCreatedWebsitePushID(ctx context.Context, before *DeveloperWebsitePushIDsListResult, identifier string) (string, error) {
	after, err := c.listDeveloperWebsitePushIDsAfterSession(ctx)
	if err != nil {
		return "", fmt.Errorf("post-create Website Push ID list failed: %w", err)
	}
	if err := validateWebsitePushIDLegacyListForWrite(after); err != nil {
		return "", err
	}
	beforeMatches := findWebsitePushIDLegacyMatches(before, "", identifier)
	afterMatches := findWebsitePushIDLegacyMatches(after, "", identifier)
	if len(afterMatches) != 1 || len(beforeMatches) != 0 {
		return "", fmt.Errorf("post-create Website Push ID list did not identify exactly one new resource for identifier %q", identifier)
	}
	if afterMatches[0].ID == "" {
		return "", fmt.Errorf("post-create Website Push ID list identified %q but did not expose an opaque resource ID", identifier)
	}
	return afterMatches[0].ID, nil
}

func (c *Client) verifyDeletedWebsitePushID(ctx context.Context, websitePushID, identifier string) error {
	_, err := c.getDeveloperWebsitePushIDAfterSession(ctx, websitePushID)
	if err == nil {
		return fmt.Errorf("website push ID %q is still readable after delete", websitePushID)
	}
	if !isDeveloperPortalNotFound(err) {
		return err
	}
	list, err := c.listDeveloperWebsitePushIDsAfterSession(ctx)
	if err != nil {
		return err
	}
	if err := validateWebsitePushIDLegacyListForWrite(list); err != nil {
		return err
	}
	for _, entry := range list.WebsitePushIDList {
		identity := websitePushIDLegacyIdentityForEntry(entry)
		if websitePushIDLegacyString(entry, "identifier", "websitePushId") == "" {
			return fmt.Errorf("post-delete Website Push ID list contains an entry with no recognized identifier; cannot establish absence")
		}
		if identity.ID == websitePushID || identity.Identifier == identifier {
			return fmt.Errorf("website push ID %q remains in the legacy Website Push ID list", websitePushID)
		}
	}
	return nil
}

func (c *Client) settleDeletedWebsitePushID(ctx context.Context, websitePushID, identifier, name string, cause error) (*asc.WebWebsitePushIDMutationResult, error) {
	if err := c.verifyDeletedWebsitePushID(ctx, websitePushID, identifier); err != nil {
		return nil, &DeveloperWebsitePushIDUnverifiedError{Err: fmt.Errorf("%w; delete outcome could not be verified: %w", cause, err)}
	}
	return websitePushIDMutationReceipt("delete", websitePushID, identifier, name, "deleted"), nil
}

func isDeveloperPortalNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound
}

type websitePushIDLegacyIdentity struct {
	ID         string
	Identifier string
	Name       string
}

func websitePushIDLegacyIdentityForEntry(entry DeveloperWebsitePushID) websitePushIDLegacyIdentity {
	identity := websitePushIDLegacyIdentity{
		// The legacy list's websitePushId field is an identifier in the
		// captured response. Only an explicit id field is safe to pass to the
		// modern /websitepushIds/{id} detail route.
		ID:         websitePushIDLegacyString(entry, "id"),
		Identifier: websitePushIDLegacyString(entry, "identifier", "websitePushId"),
		Name:       websitePushIDLegacyString(entry, "name"),
	}
	if identity.Identifier == "" {
		identity.Identifier = identity.ID
	}
	return identity
}

func websitePushIDLegacyString(entry DeveloperWebsitePushID, keys ...string) string {
	for _, key := range keys {
		value, ok := entry[key]
		if !ok || value == nil {
			continue
		}
		if text, ok := value.(string); ok {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

func findWebsitePushIDLegacyMatches(result *DeveloperWebsitePushIDsListResult, resourceID, identifier string) []websitePushIDLegacyIdentity {
	if result == nil {
		return nil
	}
	matches := make([]websitePushIDLegacyIdentity, 0)
	for _, entry := range result.WebsitePushIDList {
		identity := websitePushIDLegacyIdentityForEntry(entry)
		if (resourceID != "" && identity.ID == resourceID) || (identifier != "" && identity.Identifier == identifier) {
			matches = append(matches, identity)
		}
	}
	return matches
}

func validateWebsitePushIDLegacyListForWrite(result *DeveloperWebsitePushIDsListResult) error {
	if result == nil {
		return fmt.Errorf("developer portal Website Push ID list response is empty")
	}
	if result.PageNumber == nil || *result.PageNumber != 1 || result.PageSize <= 0 {
		return fmt.Errorf("developer portal Website Push ID list has missing or invalid first-page metadata; refusing to mutate without a complete collision/postcondition read")
	}
	if len(result.WebsitePushIDList) >= result.PageSize {
		return fmt.Errorf("developer portal Website Push ID list is a full first page; refusing to mutate without a complete collision/postcondition read")
	}
	return nil
}

func websitePushIDStringAttribute(attributes map[string]any, key string) string {
	if attributes == nil {
		return ""
	}
	value, ok := attributes[key]
	if !ok || value == nil {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func websitePushIDBoolAttribute(attributes map[string]any, key string) (bool, bool) {
	if attributes == nil {
		return false, false
	}
	value, ok := attributes[key]
	if !ok {
		return false, false
	}
	boolean, ok := value.(bool)
	return boolean, ok
}

func websitePushIDMutationReceipt(operation, websitePushID, identifier, name, status string) *asc.WebWebsitePushIDMutationResult {
	return &asc.WebWebsitePushIDMutationResult{
		Operation:     operation,
		WebsitePushID: websitePushID,
		Identifier:    identifier,
		Name:          name,
		Changed:       true,
		Verified:      true,
		Status:        status,
	}
}
