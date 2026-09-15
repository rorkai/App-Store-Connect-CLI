package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
)

const (
	developerWebsitePushIDsListPath     = "/account/ios/identifiers/listWebsitePushIds.action"
	developerWebsitePushIDsListPageSize = 1000
)

// DeveloperWebsitePushID is an open-ended legacy Website Push ID list entry.
// The current account had no rows when this contract was captured, so the
// provider-owned entry shape remains open rather than being guessed into a
// closed struct.
type DeveloperWebsitePushID map[string]any

// DeveloperWebsitePushIDsListResult is the legacy Website Push ID collection
// returned by the Developer Portal. Raw preserves Apple's complete root-level
// response for JSON output, including fields that this client does not model.
type DeveloperWebsitePushIDsListResult struct {
	ResultCode        *int                     `json:"resultCode,omitempty"`
	PageNumber        *int                     `json:"pageNumber,omitempty"`
	PageSize          int                      `json:"pageSize,omitempty"`
	WebsitePushIDList []DeveloperWebsitePushID `json:"websitePushIdList"`
	Raw               json.RawMessage          `json:"-"`
}

// ListDeveloperWebsitePushIDs reads the captured first Website Push ID page
// for the selected Developer Portal team. This legacy response does not yet
// have a verified continuation contract, so the command intentionally makes a
// single fixed page request.
func (c *Client) ListDeveloperWebsitePushIDs(ctx context.Context) (*DeveloperWebsitePushIDsListResult, error) {
	if err := c.ensureDeveloperPortalSession(ctx); err != nil {
		return nil, err
	}
	return c.listDeveloperWebsitePushIDsAfterSession(ctx)
}

// listDeveloperWebsitePushIDsAfterSession reads the captured first page after
// the caller has already established the Developer Portal session. Mutation
// flows use this seam so their preflight and postcondition reads do not
// bootstrap the account session again between requests.
func (c *Client) listDeveloperWebsitePushIDsAfterSession(ctx context.Context) (*DeveloperWebsitePushIDsListResult, error) {
	teamID := c.developerPortalTeamID()
	if teamID == "" {
		return nil, fmt.Errorf("developer portal team is not selected; %s", developerPortalAuthHint)
	}

	body, err := c.doDeveloperPortalLegacyFormRequest(readonly.WithReadIntent(ctx), developerWebsitePushIDsListPath, url.Values{
		"onlyCountLists": {"true"},
		"pageSize":       {strconv.Itoa(developerWebsitePushIDsListPageSize)},
		"pageNumber":     {"1"},
		"sort":           {"name=asc"},
		"teamId":         {teamID},
	}, false)
	if err != nil {
		return nil, err
	}

	result, err := parseDeveloperWebsitePushIDsListResponse(body)
	if err != nil {
		return nil, err
	}
	return result, nil
}

type developerWebsitePushIDsListResponse struct {
	developerPortalLegacyResponse
	PageNumber        *int            `json:"pageNumber"`
	PageSize          int             `json:"pageSize"`
	WebsitePushIDList json.RawMessage `json:"websitePushIdList"`
}

func parseDeveloperWebsitePushIDsListResponse(body []byte) (*DeveloperWebsitePushIDsListResult, error) {
	var response developerWebsitePushIDsListResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse Developer Portal Website Push ID response: %w", err)
	}
	if err := validateDeveloperPortalLegacyResponse(response.developerPortalLegacyResponse); err != nil {
		return nil, err
	}
	if missingJSONValue(response.WebsitePushIDList) {
		return nil, fmt.Errorf("developer portal Website Push ID response has no websitePushIdList collection")
	}

	var entries []DeveloperWebsitePushID
	if err := json.Unmarshal(response.WebsitePushIDList, &entries); err != nil {
		return nil, fmt.Errorf("failed to parse Developer Portal websitePushIdList collection: %w", err)
	}
	if entries == nil {
		return nil, fmt.Errorf("developer portal Website Push ID response has no websitePushIdList collection")
	}
	result := &DeveloperWebsitePushIDsListResult{
		ResultCode:        response.ResultCode,
		PageNumber:        response.PageNumber,
		PageSize:          response.PageSize,
		WebsitePushIDList: entries,
		Raw:               append(json.RawMessage(nil), body...),
	}
	return result, nil
}

// MarshalJSON preserves Apple's complete legacy response envelope when the
// result came from the service. A result constructed by a caller falls back
// to its modeled fields for tests and other in-process callers.
func (r DeveloperWebsitePushIDsListResult) MarshalJSON() ([]byte, error) {
	if len(r.Raw) > 0 {
		return append([]byte(nil), r.Raw...), nil
	}
	type resultWithoutMethods DeveloperWebsitePushIDsListResult
	return json.Marshal(resultWithoutMethods(r))
}
