package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

const (
	customAppUserResourceType = "customAppUsers"
	customAppUserPageSize     = "100"
)

// CustomAppUser is one Apple Account recipient in an app-scoped custom app
// user collection. Raw preserves the complete resource when a caller needs to
// inspect fields this client does not model.
type CustomAppUser struct {
	ID      string
	Type    string
	AppleID string
	Raw     json.RawMessage `json:"-"`
}

// MarshalJSON preserves Apple's resource object when it was decoded from a
// response. The fallback is useful for callers constructing a resource in
// tests or request diagnostics.
func (u CustomAppUser) MarshalJSON() ([]byte, error) {
	if len(u.Raw) > 0 {
		return append([]byte(nil), u.Raw...), nil
	}
	return json.Marshal(struct {
		Type       string            `json:"type"`
		ID         string            `json:"id"`
		Attributes map[string]string `json:"attributes"`
	}{
		Type: u.Type,
		ID:   u.ID,
		Attributes: map[string]string{
			"appleId": u.AppleID,
		},
	})
}

// CustomAppUsersListResult is the raw Apple JSON:API collection returned for
// one selected app. JSON output uses Raw verbatim for a single page; when
// pagination is requested, the first envelope is retained and its data array
// is replaced with the validated aggregate of all pages.
type CustomAppUsersListResult struct {
	Data []CustomAppUser `json:"data"`
	Raw  json.RawMessage `json:"-"`

	// These fields are intentionally kept separate from Raw so the common CLI
	// pagination warning can inspect links and totals without changing JSON.
	links map[string]any
	meta  json.RawMessage
	// rawData is populated only after a successful aggregate. A nil slice means
	// that the original one-page envelope should be emitted byte-for-byte.
	rawData []json.RawMessage
}

var _ interface {
	MarshalJSON() ([]byte, error)
} = (*CustomAppUsersListResult)(nil)

var _ asc.PaginatedResponse = (*CustomAppUsersListResult)(nil)

// GetLinks exposes collection links for shared output diagnostics.
func (r *CustomAppUsersListResult) GetLinks() *asc.Links {
	if r == nil {
		return nil
	}
	return &asc.Links{
		Self:  customAppUserLinkString(r.links, "self"),
		Next:  customAppUserLinkString(r.links, "next"),
		First: customAppUserLinkString(r.links, "first"),
		Prev:  customAppUserLinkString(r.links, "prev"),
	}
}

// GetData exposes the collection to shared output diagnostics.
func (r *CustomAppUsersListResult) GetData() any {
	if r == nil {
		return nil
	}
	return r.Data
}

// GetMeta exposes the raw paging metadata to shared output diagnostics.
func (r *CustomAppUsersListResult) GetMeta() json.RawMessage {
	if r == nil || len(r.meta) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), r.meta...)
}

// MarshalJSON preserves the first Apple envelope and only changes its data
// member after an explicit, validated --paginate aggregation.
func (r CustomAppUsersListResult) MarshalJSON() ([]byte, error) {
	if len(r.rawData) == 0 && len(r.Raw) > 0 {
		return append([]byte(nil), r.Raw...), nil
	}
	if len(r.Raw) > 0 && r.rawData != nil {
		var envelope map[string]json.RawMessage
		if err := json.Unmarshal(r.Raw, &envelope); err != nil {
			return nil, fmt.Errorf("failed to preserve custom app users envelope: %w", err)
		}
		data, err := json.Marshal(r.rawData)
		if err != nil {
			return nil, fmt.Errorf("failed to encode custom app users data: %w", err)
		}
		envelope["data"] = data
		if r.links != nil {
			links, err := json.Marshal(r.links)
			if err != nil {
				return nil, fmt.Errorf("failed to preserve custom app users links: %w", err)
			}
			envelope["links"] = links
		}
		return json.Marshal(envelope)
	}
	type resultWithoutMethods CustomAppUsersListResult
	return json.Marshal(resultWithoutMethods(r))
}

// CustomAppUserUnverifiedError reports a user mutation whose provider outcome
// cannot be established. The caller must inspect the selected app before
// retrying; this client never retries POST or DELETE.
type CustomAppUserUnverifiedError struct {
	Err error
}

func (e *CustomAppUserUnverifiedError) Error() string {
	if e == nil || e.Err == nil {
		return "custom app user mutation outcome is uncertain"
	}
	return fmt.Sprintf("custom app user mutation outcome is uncertain: %v", e.Err)
}

func (e *CustomAppUserUnverifiedError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type customAppUserValidationError struct {
	Err error
}

func (e *customAppUserValidationError) Error() string {
	if e == nil || e.Err == nil {
		return "invalid custom app user input"
	}
	return e.Err.Error()
}

func (e *customAppUserValidationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// IsCustomAppUserWriteUncertain reports whether an error arose after a user
// mutation may have reached Apple. Deterministic client-side validation and
// ordinary 4xx responses do not get an uncertain receipt; transport errors,
// 408, and 5xx responses do.
func IsCustomAppUserWriteUncertain(err error) bool {
	if err == nil {
		return false
	}
	var validationErr *customAppUserValidationError
	if errors.As(err, &validationErr) {
		return false
	}
	var uncertainErr *CustomAppUserUnverifiedError
	if errors.As(err, &uncertainErr) {
		return true
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr != nil {
		return apiErr.Status == http.StatusRequestTimeout || apiErr.Status >= 500
	}
	return true
}

// ListCustomAppUsersPaginated reads and validates the complete selected-app
// customAppUsers collection. It follows only same-host, same-app collection
// links and never performs a write.
func (c *Client) ListCustomAppUsersPaginated(ctx context.Context, appID string) (*CustomAppUsersListResult, error) {
	return c.listCustomAppUsers(ctx, appID, true)
}

// ListCustomAppUsersWithPagination exposes one method for command callers that
// map the --paginate flag directly while retaining the convenient first-page
// method for programmatic users.
func (c *Client) ListCustomAppUsersWithPagination(ctx context.Context, appID string, paginate bool) (*CustomAppUsersListResult, error) {
	return c.listCustomAppUsers(ctx, appID, paginate)
}

func (c *Client) listCustomAppUsers(ctx context.Context, appID string, paginate bool) (*CustomAppUsersListResult, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("app id is required")
	}
	firstPath := customAppUsersCollectionPath(appID)
	currentPath := firstPath
	visited := make(map[string]struct{})
	var result *CustomAppUsersListResult
	var allData []json.RawMessage
	seenIDs := make(map[string]struct{})
	var expectedTotal *int
	pageCount := 0

	for {
		if _, seen := visited[currentPath]; seen {
			return nil, fmt.Errorf("custom app users pagination loop detected")
		}
		visited[currentPath] = struct{}{}

		body, err := c.doJSONAPIRequest(ctx, c.baseURL, currentPath)
		if err != nil {
			return nil, err
		}
		page, err := parseCustomAppUsersPage(body, appID, c.baseURL)
		if err != nil {
			return nil, err
		}
		pageCount++
		if expectedTotal == nil && page.total != nil {
			value := *page.total
			expectedTotal = &value
		} else if expectedTotal != nil && page.total != nil && *expectedTotal != *page.total {
			return nil, fmt.Errorf("custom app users response changed paging total from %d to %d", *expectedTotal, *page.total)
		}
		for _, user := range page.users {
			if _, seen := seenIDs[user.ID]; seen {
				return nil, fmt.Errorf("custom app users response contains duplicate resource id %q across pages", user.ID)
			}
			seenIDs[user.ID] = struct{}{}
		}

		if result == nil {
			result = &CustomAppUsersListResult{
				Data:  append([]CustomAppUser(nil), page.users...),
				Raw:   append(json.RawMessage(nil), body...),
				links: cloneCustomAppUserLinks(page.links),
				meta:  append(json.RawMessage(nil), page.meta...),
			}
			allData = append(allData, page.dataRaw...)
		} else {
			result.Data = append(result.Data, page.users...)
			allData = append(allData, page.dataRaw...)
		}

		if expectedTotal != nil && len(allData) > *expectedTotal {
			return nil, fmt.Errorf("custom app users response contains %d resources but paging total is %d", len(allData), *expectedTotal)
		}
		if page.next == "" || !paginate {
			if expectedTotal != nil && page.next == "" && len(allData) != *expectedTotal {
				return nil, fmt.Errorf("custom app users response contains %d resources but paging total is %d", len(allData), *expectedTotal)
			}
			if result == nil {
				return nil, fmt.Errorf("custom app users response is empty")
			}
			if paginate && pageCount > 1 {
				result.rawData = make([]json.RawMessage, len(allData))
				copy(result.rawData, allData)
				// An aggregate has no remaining page. Keep the first envelope's
				// other fields but clear the continuation link in the diagnostic
				// view; MarshalJSON still retains all unknown link members.
				delete(result.links, "next")
			}
			if page.next != "" {
				if _, err := resolveCustomAppUsersNextPath(page.next, currentPath, c.baseURL, appID); err != nil {
					return nil, fmt.Errorf("invalid custom app users response pagination links: %w", err)
				}
			}
			return result, nil
		}

		nextPath, err := resolveCustomAppUsersNextPath(page.next, currentPath, c.baseURL, appID)
		if err != nil {
			return nil, err
		}
		currentPath = nextPath
	}
}

// CreateCustomAppUser sends the observed JSON:API create request once and
// validates the accepted resource's type, opaque ID, and exact account.
func (c *Client) CreateCustomAppUser(ctx context.Context, appID, appleID string) (*CustomAppUser, error) {
	appID = strings.TrimSpace(appID)
	appleID = strings.TrimSpace(appleID)
	if appID == "" {
		return nil, &customAppUserValidationError{Err: fmt.Errorf("app id is required")}
	}
	if appleID == "" {
		return nil, &customAppUserValidationError{Err: fmt.Errorf("apple account is required")}
	}
	payload := map[string]any{
		"data": map[string]any{
			"type": customAppUserResourceType,
			"attributes": map[string]string{
				"appleId": appleID,
			},
			"relationships": map[string]any{
				"app": map[string]any{
					"data": map[string]string{
						"type": "apps",
						"id":   appID,
					},
				},
			},
		},
	}
	body, err := c.doCustomAppUserMutationRequest(ctx, http.MethodPost, "/"+customAppUserResourceType, payload, appID)
	if err != nil {
		return nil, err
	}
	user, err := parseCustomAppUserWriteResponse(body, appleID)
	if err != nil {
		return user, &CustomAppUserUnverifiedError{Err: err}
	}
	return user, nil
}

// DeleteCustomAppUser sends the observed JSON:API delete request once. Apple
// returns an empty 204 body; an empty body is accepted, while a present body
// must identify the deleted customAppUsers resource exactly.
func (c *Client) DeleteCustomAppUser(ctx context.Context, appID, recipientID string) error {
	appID = strings.TrimSpace(appID)
	recipientID = strings.TrimSpace(recipientID)
	if appID == "" {
		return &customAppUserValidationError{Err: fmt.Errorf("app id is required")}
	}
	if recipientID == "" {
		return &customAppUserValidationError{Err: fmt.Errorf("recipient id is required")}
	}
	payload := map[string]any{
		"data": map[string]string{
			"type": customAppUserResourceType,
			"id":   recipientID,
		},
	}
	body, err := c.doCustomAppUserMutationRequest(ctx, http.MethodDelete, "/"+customAppUserResourceType+"/"+url.PathEscape(recipientID), payload, appID)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return &CustomAppUserUnverifiedError{Err: fmt.Errorf("parse custom app user delete response: %w", err)}
	}
	var resource struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	}
	if err := json.Unmarshal(envelope.Data, &resource); err != nil || resource.Type != customAppUserResourceType || strings.TrimSpace(resource.ID) != recipientID {
		return &CustomAppUserUnverifiedError{Err: fmt.Errorf("custom app user delete response has unexpected resource identity")}
	}
	return nil
}

func (c *Client) doCustomAppUserMutationRequest(ctx context.Context, method, path string, body any, appID string) ([]byte, error) {
	headers := make(http.Header)
	headers.Set("Accept", "application/json")
	headers.Set("Content-Type", "application/json")
	headers.Set("X-Requested-With", "XMLHttpRequest")
	headers.Set("X-CSRF-ITC", "[asc-ui]")
	headers.Set("Origin", appStoreBaseURL)
	headers.Set("Referer", appStoreBaseURL+"/apps/"+url.PathEscape(strings.TrimSpace(appID))+"/distribution/pricing")
	return c.doRequestBase(ctx, c.baseURL, method, path, body, headers)
}

type customAppUsersPage struct {
	users   []CustomAppUser
	dataRaw []json.RawMessage
	links   map[string]any
	meta    json.RawMessage
	next    string
	total   *int
}

func parseCustomAppUsersPage(body []byte, appID, baseURL string) (customAppUsersPage, error) {
	var page customAppUsersPage
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		return page, fmt.Errorf("failed to parse custom app users response: %w", err)
	}
	rawData, ok := envelope["data"]
	trimmedData := bytes.TrimSpace(rawData)
	if !ok || len(trimmedData) == 0 || bytes.Equal(trimmedData, []byte("null")) || trimmedData[0] != '[' {
		return page, fmt.Errorf("custom app users response missing non-null data collection")
	}
	if err := json.Unmarshal(trimmedData, &page.dataRaw); err != nil || page.dataRaw == nil {
		return page, fmt.Errorf("custom app users response data must be an array")
	}
	page.users = make([]CustomAppUser, 0, len(page.dataRaw))
	seenIDs := make(map[string]struct{}, len(page.dataRaw))
	for _, raw := range page.dataRaw {
		user, err := parseCustomAppUserResource(raw)
		if err != nil {
			return page, err
		}
		if _, seen := seenIDs[user.ID]; seen {
			return page, fmt.Errorf("custom app users response contains duplicate resource id %q", user.ID)
		}
		seenIDs[user.ID] = struct{}{}
		page.users = append(page.users, user)
	}

	rawLinks, ok := envelope["links"]
	trimmedLinks := bytes.TrimSpace(rawLinks)
	if !ok || len(trimmedLinks) == 0 || bytes.Equal(trimmedLinks, []byte("null")) || trimmedLinks[0] != '{' {
		return page, fmt.Errorf("custom app users response missing non-null links")
	}
	if err := json.Unmarshal(trimmedLinks, &page.links); err != nil || page.links == nil {
		return page, fmt.Errorf("custom app users response links must be an object")
	}
	self, err := customAppUserLinkValue(page.links, "self")
	if err != nil || strings.TrimSpace(self) == "" {
		return page, fmt.Errorf("custom app users response links.self must be a non-empty string")
	}
	if _, err := resolveCustomAppUsersNextPath(self, "", baseURL, appID); err != nil {
		return page, fmt.Errorf("invalid custom app users response links.self: %w", err)
	}
	page.next, err = customAppUserNextLink(page.links)
	if err != nil {
		return page, fmt.Errorf("invalid custom app users response pagination links: %w", err)
	}

	if rawMeta, present := envelope["meta"]; present {
		page.meta = append(json.RawMessage(nil), rawMeta...)
		page.total, err = customAppUserPagingTotal(rawMeta)
		if err != nil {
			return page, err
		}
	}
	return page, nil
}

func parseCustomAppUserResource(raw json.RawMessage) (CustomAppUser, error) {
	var resource map[string]json.RawMessage
	if err := json.Unmarshal(raw, &resource); err != nil {
		return CustomAppUser{}, fmt.Errorf("custom app users response contains an invalid resource: %w", err)
	}
	var identity struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	}
	if err := json.Unmarshal(raw, &identity); err != nil {
		return CustomAppUser{}, fmt.Errorf("custom app users response contains an invalid resource identity: %w", err)
	}
	identity.Type = strings.TrimSpace(identity.Type)
	identity.ID = strings.TrimSpace(identity.ID)
	if identity.Type != customAppUserResourceType || identity.ID == "" {
		return CustomAppUser{}, fmt.Errorf("custom app users response contains resource type %q and id %q; want type %q and a non-empty id", identity.Type, identity.ID, customAppUserResourceType)
	}
	attributesRaw, ok := resource["attributes"]
	trimmedAttributes := bytes.TrimSpace(attributesRaw)
	if !ok || len(trimmedAttributes) == 0 || bytes.Equal(trimmedAttributes, []byte("null")) || trimmedAttributes[0] != '{' {
		return CustomAppUser{}, fmt.Errorf("custom app user %q is missing attributes", identity.ID)
	}
	var attributes map[string]json.RawMessage
	if err := json.Unmarshal(trimmedAttributes, &attributes); err != nil || attributes == nil {
		return CustomAppUser{}, fmt.Errorf("custom app user %q has invalid attributes", identity.ID)
	}
	var appleID string
	appleIDRaw, ok := attributes["appleId"]
	if !ok || json.Unmarshal(appleIDRaw, &appleID) != nil || strings.TrimSpace(appleID) == "" {
		return CustomAppUser{}, fmt.Errorf("custom app user %q is missing a non-empty appleId", identity.ID)
	}
	return CustomAppUser{
		ID:      identity.ID,
		Type:    identity.Type,
		AppleID: appleID,
		Raw:     append(json.RawMessage(nil), raw...),
	}, nil
}

func parseCustomAppUserWriteResponse(body []byte, requestedAppleID string) (*CustomAppUser, error) {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("parse custom app user create response: %w", err)
	}
	if len(bytes.TrimSpace(envelope.Data)) == 0 || bytes.Equal(bytes.TrimSpace(envelope.Data), []byte("null")) {
		return nil, fmt.Errorf("custom app user create response missing data")
	}
	var identity struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	}
	_ = json.Unmarshal(envelope.Data, &identity)
	identity.Type = strings.TrimSpace(identity.Type)
	identity.ID = strings.TrimSpace(identity.ID)
	var available *CustomAppUser
	if identity.Type == customAppUserResourceType && identity.ID != "" {
		available = &CustomAppUser{ID: identity.ID, Type: identity.Type}
	}
	user, err := parseCustomAppUserResource(envelope.Data)
	if err != nil {
		return available, fmt.Errorf("invalid custom app user create response: %w", err)
	}
	if user.AppleID != requestedAppleID {
		return &user, fmt.Errorf("custom app user create response returned a different appleId")
	}
	return &user, nil
}

func customAppUsersCollectionPath(appID string) string {
	return "/apps/" + url.PathEscape(strings.TrimSpace(appID)) + "/" + customAppUserResourceType + "?limit=" + customAppUserPageSize
}

func customAppUserLinkValue(links map[string]any, name string) (string, error) {
	value, ok := links[name]
	if !ok || value == nil {
		return "", nil
	}
	if result, ok := value.(string); ok {
		return strings.TrimSpace(result), nil
	}
	return "", fmt.Errorf("links.%s must be a string", name)
}

func customAppUserNextLink(links map[string]any) (string, error) {
	value, ok := links["next"]
	if !ok || value == nil {
		return "", nil
	}
	switch next := value.(type) {
	case string:
		return strings.TrimSpace(next), nil
	case map[string]any:
		if href, ok := next["href"].(string); ok {
			return strings.TrimSpace(href), nil
		}
		if value, ok := next["url"].(string); ok {
			return strings.TrimSpace(value), nil
		}
		return "", fmt.Errorf("links.next object does not contain href/url")
	default:
		return "", fmt.Errorf("links.next has unsupported type %T", value)
	}
}

func customAppUserPagingTotal(rawMeta json.RawMessage) (*int, error) {
	trimmed := bytes.TrimSpace(rawMeta)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}
	var meta map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &meta); err != nil || meta == nil {
		return nil, fmt.Errorf("custom app users response meta must be an object")
	}
	pagingRaw, ok := meta["paging"]
	if !ok || bytes.Equal(bytes.TrimSpace(pagingRaw), []byte("null")) {
		return nil, nil
	}
	var paging map[string]json.RawMessage
	if err := json.Unmarshal(pagingRaw, &paging); err != nil || paging == nil {
		return nil, fmt.Errorf("custom app users response meta.paging must be an object")
	}
	totalRaw, ok := paging["total"]
	if !ok {
		return nil, nil
	}
	trimmedTotal := bytes.TrimSpace(totalRaw)
	if len(trimmedTotal) == 0 || bytes.Equal(trimmedTotal, []byte("null")) {
		return nil, fmt.Errorf("custom app users response meta.paging.total must be a non-negative integer")
	}
	var total int
	if err := json.Unmarshal(totalRaw, &total); err != nil || total < 0 {
		return nil, fmt.Errorf("custom app users response meta.paging.total must be a non-negative integer")
	}
	return &total, nil
}

func cloneCustomAppUserLinks(source map[string]any) map[string]any {
	if source == nil {
		return nil
	}
	clone := make(map[string]any, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func customAppUserLinkString(links map[string]any, name string) string {
	if name == "next" {
		value, _ := customAppUserNextLink(links)
		return value
	}
	value, _ := customAppUserLinkValue(links, name)
	return value
}

func resolveCustomAppUsersNextPath(nextLink, currentPath, baseURL, appID string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid client base URL: %w", err)
	}
	if base.Scheme == "" || base.Host == "" {
		return "", fmt.Errorf("invalid client base URL")
	}
	if strings.TrimSpace(nextLink) == "" {
		return "", nil
	}
	ref, err := url.Parse(strings.TrimSpace(nextLink))
	if err != nil {
		return "", fmt.Errorf("invalid collection link: %w", err)
	}
	baseReference := *base
	if !strings.HasSuffix(baseReference.Path, "/") {
		baseReference.Path += "/"
		baseReference.RawPath = ""
	}
	currentURL := &baseReference
	if strings.TrimSpace(currentPath) != "" {
		current, err := url.Parse(strings.TrimRight(baseURL, "/") + currentPath)
		if err != nil {
			return "", fmt.Errorf("invalid current collection link: %w", err)
		}
		currentURL = current
	}
	resolved := currentURL.ResolveReference(ref)
	if !strings.EqualFold(resolved.Scheme, base.Scheme) || !strings.EqualFold(resolved.Host, base.Host) {
		return "", fmt.Errorf("collection link host %q does not match client host %q", resolved.Host, base.Host)
	}
	basePath := strings.TrimSuffix(base.EscapedPath(), "/")
	canonical := basePath + "/apps/" + url.PathEscape(strings.TrimSpace(appID)) + "/" + customAppUserResourceType
	if resolved.EscapedPath() != canonical {
		return "", fmt.Errorf("collection link path %q is outside selected app %q", resolved.EscapedPath(), appID)
	}
	if resolved.Fragment != "" {
		return "", fmt.Errorf("collection link must not contain a fragment")
	}
	path := strings.TrimPrefix(resolved.EscapedPath(), basePath)
	if path == "" {
		path = "/"
	}
	if resolved.RawQuery != "" {
		query, err := url.ParseQuery(resolved.RawQuery)
		if err != nil {
			return "", fmt.Errorf("invalid collection link query: %w", err)
		}
		path += "?" + query.Encode()
	}
	return path, nil
}
