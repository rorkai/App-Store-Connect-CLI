package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
)

const (
	developerAppGroupsListPath        = "/account/ios/identifiers/listApplicationGroups.action"
	developerAppGroupsCreatePath      = "/account/ios/identifiers/addApplicationGroup.action"
	developerAppGroupsDeletePath      = "/account/ios/identifiers/deleteApplicationGroup.action"
	developerAppGroupsPageSize        = 500
	developerAppGroupsCapabilityType  = "APP_GROUPS"
	developerAppGroupIdentifierPrefix = "group."
	developerBundleIDsListPageSize    = 200
	developerBundleIDsListMaxPages    = 100
)

var developerBundleIDsListIncludes = []string{
	"bundleIdCapabilities",
	"bundleIdCapabilities.capability",
	"bundleIdCapabilities.appGroups",
}

// DeveloperAppGroup is an App Group identifier returned by Apple Developer Portal.
type DeveloperAppGroup struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Identifier string `json:"identifier"`
	Prefix     string `json:"prefix,omitempty"`
	Status     string `json:"status,omitempty"`
}

// DeveloperAppGroupsListResult contains App Groups visible to the selected team.
type DeveloperAppGroupsListResult struct {
	Data []DeveloperAppGroup `json:"data"`
}

// DeveloperAppGroupsListOptions controls App Group list pagination.
type DeveloperAppGroupsListOptions struct {
	Paginate bool
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

// DeveloperAppGroupUnassignRequest removes one App Group from a Bundle ID.
type DeveloperAppGroupUnassignRequest struct {
	BundleID string
	GroupID  string
}

// DeveloperAppGroupSetRequest replaces a Bundle ID's complete App Group set.
type DeveloperAppGroupSetRequest struct {
	BundleID string
	GroupIDs []string
}

// DeveloperAppGroupDeleteRequest deletes an App Group registration.
type DeveloperAppGroupDeleteRequest struct {
	GroupID string
}

// DeveloperAppGroupAssignment names a Bundle ID that references an App Group.
type DeveloperAppGroupAssignment struct {
	BundleID   string `json:"bundleId"`
	Identifier string `json:"identifier,omitempty"`
	Name       string `json:"name,omitempty"`
}

// DeveloperAppGroupInUseError is returned when a delete is refused because the
// App Group is still referenced by at least one Bundle ID.
type DeveloperAppGroupInUseError struct {
	GroupID     string
	Identifier  string
	Assignments []DeveloperAppGroupAssignment
}

func (e *DeveloperAppGroupInUseError) Error() string {
	group := e.GroupID
	if e.Identifier != "" {
		group = fmt.Sprintf("%s (%s)", e.GroupID, e.Identifier)
	}
	names := make([]string, 0, len(e.Assignments))
	for _, assignment := range e.Assignments {
		label := assignment.Identifier
		if label == "" {
			label = assignment.Name
		}
		if label == "" {
			names = append(names, assignment.BundleID)
			continue
		}
		names = append(names, fmt.Sprintf("%s (%s)", label, assignment.BundleID))
	}
	noun := "Bundle IDs"
	if len(e.Assignments) == 1 {
		noun = "Bundle ID"
	}
	return fmt.Sprintf("App Group %s is still assigned to %d %s: %s; run 'asc web app-groups unassign --bundle-id BUNDLE_RESOURCE_ID --group-id %s --confirm' for each Bundle ID before deleting",
		group, len(e.Assignments), noun, strings.Join(names, ", "), e.GroupID)
}

// DeveloperAppGroupUnverifiedError is returned when the Developer Portal
// accepted an App Group mutation but the follow-up read could not confirm it.
// Callers should assume the write may have been applied.
type DeveloperAppGroupUnverifiedError struct {
	Err error
}

func (e *DeveloperAppGroupUnverifiedError) Error() string { return e.Err.Error() }

func (e *DeveloperAppGroupUnverifiedError) Unwrap() error { return e.Err }

// ErrDeveloperPortalTeamNotSelected reports that no Developer Portal team is
// selected for the session, which the operator fixes by authenticating or by
// naming a team.
var ErrDeveloperPortalTeamNotSelected = errors.New("developer portal team is not selected")

// DeveloperPortalResultError reports that a Developer Portal legacy endpoint
// answered with an explicit non-zero result code: the request reached the
// portal and was refused.
type DeveloperPortalResultError struct {
	ResultCode int
	RequestID  string
	Message    string
}

func (e *DeveloperPortalResultError) Error() string {
	requestID := strings.TrimSpace(asc.SanitizeTerminalText(e.RequestID))
	message := strings.TrimSpace(asc.SanitizeTerminalText(e.Message))
	if message == "" {
		message = "unknown Developer Portal error"
	}
	if requestID != "" {
		return fmt.Sprintf("developer portal request failed (result code %d, request ID %s): %s", e.ResultCode, requestID, message)
	}
	return fmt.Sprintf("developer portal request failed (result code %d): %s", e.ResultCode, message)
}

// DeveloperAppGroupNotFoundError reports that the named App Group does not
// exist in the selected Developer Portal team.
type DeveloperAppGroupNotFoundError struct {
	GroupID string
}

func (e *DeveloperAppGroupNotFoundError) Error() string {
	return fmt.Sprintf("app group %q not found in the selected Developer Portal team", e.GroupID)
}

// developerAppGroupResponseError marks a Developer Portal response the client
// could not read completely, so telemetry separates an unusable upstream
// response from a CLI defect. Transport failures, HTTP statuses, explicit
// portal refusals, and failures that already classify themselves keep their
// own meaning.
func developerAppGroupResponseError(err error) error {
	if err == nil {
		return nil
	}
	var apiErr *APIError
	var resultErr *DeveloperPortalResultError
	var unreadable *DeveloperAppGroupUnreadableResponseError
	var unverified *DeveloperAppGroupUnverifiedError
	var inUse *DeveloperAppGroupInUseError
	var notFound *DeveloperAppGroupNotFoundError
	var identifier *DeveloperAppGroupIdentifierError
	switch {
	case errors.As(err, &apiErr),
		errors.As(err, &resultErr),
		errors.As(err, &unreadable),
		errors.As(err, &unverified),
		errors.As(err, &inUse),
		errors.As(err, &notFound),
		errors.As(err, &identifier),
		errors.Is(err, ErrDeveloperPortalTeamNotSelected),
		isAmbiguousDeveloperPortalWriteFailure(err):
		return err
	default:
		return &DeveloperAppGroupUnreadableResponseError{Err: err}
	}
}

// DeveloperAppGroupUnreadableResponseError marks an App Group mutation that
// was abandoned before any write because Apple's Bundle ID response could not
// be read completely. Nothing was sent, so the operator can retry once the
// response is understood.
type DeveloperAppGroupUnreadableResponseError struct {
	Err error
}

func (e *DeveloperAppGroupUnreadableResponseError) Error() string { return e.Err.Error() }

func (e *DeveloperAppGroupUnreadableResponseError) Unwrap() error { return e.Err }

// DeveloperAppGroupIdentifierError is returned when an App Group identifier
// was passed where Apple's opaque App Group resource ID is required. GroupID
// carries the resource ID of the matching group when the team's listing
// resolved one.
type DeveloperAppGroupIdentifierError struct {
	Given      string
	GroupID    string
	Identifier string
}

func (e *DeveloperAppGroupIdentifierError) Error() string {
	if e.GroupID != "" {
		return fmt.Sprintf("%q is the App Group identifier, not the opaque App Group resource ID; use %q instead (the ID column of 'asc web app-groups list')", e.Given, e.GroupID)
	}
	return fmt.Sprintf("%q is not an App Group resource ID in the selected Developer Portal team; pass the opaque ID from the ID column of 'asc web app-groups list'", e.Given)
}

// developerAppGroupsState is the raw APP_GROUPS capability state of a Bundle
// ID. GroupIDs lists every group in the relationship data even when Apple
// reports the capability disabled, because the Developer Portal still treats
// those groups as referenced (and refuses to delete them).
type developerAppGroupsState struct {
	Enabled  bool
	GroupIDs []string
}

func (s developerAppGroupsState) matches(enabled bool, groupIDs []string) bool {
	return s.Enabled == enabled && len(differenceStrings(groupIDs, s.GroupIDs)) == 0 && len(differenceStrings(s.GroupIDs, groupIDs)) == 0
}

type developerBundleIDListResponse struct {
	Data     []developerResource `json:"data"`
	Included []developerResource `json:"included"`
	Links    struct {
		Next string `json:"next"`
	} `json:"links"`
	Meta struct {
		Paging struct {
			Total *int `json:"total"`
		} `json:"paging"`
	} `json:"meta"`
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
	PageNumber           *int                       `json:"pageNumber"`
	PageSize             int                        `json:"pageSize"`
	TotalRecords         *int                       `json:"totalRecords"`
	ApplicationGroupList []developerAppGroupPayload `json:"applicationGroupList"`
}

type developerAppGroupCreateResponse struct {
	developerPortalLegacyResponse
	ApplicationGroup developerAppGroupPayload `json:"applicationGroup"`
}

// ListDeveloperAppGroups lists App Groups through the selected Developer Portal team.
func (c *Client) ListDeveloperAppGroups(ctx context.Context, options DeveloperAppGroupsListOptions) (*DeveloperAppGroupsListResult, error) {
	if err := c.ensureDeveloperPortalSession(ctx); err != nil {
		return nil, developerAppGroupResponseError(err)
	}
	teamID := c.developerPortalTeamID()
	if teamID == "" {
		return nil, fmt.Errorf("%w; %s", ErrDeveloperPortalTeamNotSelected, developerPortalAuthHint)
	}
	result, err := c.listDeveloperAppGroupPages(ctx, teamID, options.Paginate, options.Paginate)
	if err != nil {
		return nil, developerAppGroupResponseError(err)
	}
	return result, nil
}

// listDeveloperAppGroupPages reads the team's App Groups. With requireCollection
// set, a success envelope whose applicationGroupList is absent or null, or whose
// totalRecords or pageNumber is absent, null, or inconsistent with the request
// and the records returned, is an error instead of a short team; the delete path
// needs that to stay fail-closed while the default one-page list command keeps
// tolerating a sparse envelope. Explicit pagination requires complete page
// metadata so it cannot report a truncated team as complete.
func (c *Client) listDeveloperAppGroupPages(ctx context.Context, teamID string, paginate bool, requireCollection bool) (*DeveloperAppGroupsListResult, error) {
	result := &DeveloperAppGroupsListResult{Data: []DeveloperAppGroup{}}
	seenGroupIDs := make(map[string]struct{})
	firstTotalRecords := 0
	for pageNumber := 1; ; pageNumber++ {
		body, err := c.doDeveloperPortalLegacyFormRequest(readonly.WithReadIntent(ctx), developerAppGroupsListPath, url.Values{
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
		if requireCollection && page.ApplicationGroupList == nil {
			return nil, fmt.Errorf("developer portal App Groups response has no applicationGroupList collection")
		}
		if requireCollection && page.TotalRecords == nil {
			return nil, fmt.Errorf("developer portal App Groups response has no totalRecords count")
		}
		// A stale or repeated page can make the record count line up with
		// totalRecords while the page that holds the target was never read.
		if requireCollection && (page.PageNumber == nil || *page.PageNumber != pageNumber) {
			return nil, fmt.Errorf("developer portal App Groups response did not return the requested page %d", pageNumber)
		}
		for _, group := range page.ApplicationGroupList {
			decoded, err := decodeDeveloperAppGroup(group)
			if err != nil {
				return nil, err
			}
			if requireCollection {
				if _, duplicate := seenGroupIDs[decoded.ID]; duplicate {
					return nil, fmt.Errorf("developer portal App Groups response repeated App Group %q across pages", decoded.ID)
				}
				seenGroupIDs[decoded.ID] = struct{}{}
			}
			result.Data = append(result.Data, decoded)
		}

		totalRecords := 0
		if page.TotalRecords != nil {
			totalRecords = *page.TotalRecords
		}
		// One listing has one total; a strict caller treats a page that
		// reports a different count as a listing that shifted mid-read.
		if pageNumber == 1 {
			firstTotalRecords = totalRecords
		} else if requireCollection && totalRecords != firstTotalRecords {
			return nil, fmt.Errorf("developer portal App Groups response changed totalRecords from %d to %d between pages", firstTotalRecords, totalRecords)
		}
		if requireCollection && totalRecords < len(result.Data) {
			return nil, fmt.Errorf("developer portal App Groups response reported %d total records but returned %d", totalRecords, len(result.Data))
		}
		if !paginate || totalRecords <= len(result.Data) {
			break
		}
		if len(page.ApplicationGroupList) == 0 {
			// The portal announced more records than it delivered; a strict
			// caller cannot treat this truncated listing as complete.
			if requireCollection {
				return nil, fmt.Errorf("developer portal App Groups response reported %d total records but stopped after %d", totalRecords, len(result.Data))
			}
			break
		}
	}
	return result, nil
}

// DeleteDeveloperAppGroup deletes an App Group registration. It fails closed
// when the group is still referenced by any Bundle ID and verifies the deletion
// by re-reading the team's App Group list.
func (c *Client) DeleteDeveloperAppGroup(ctx context.Context, request DeveloperAppGroupDeleteRequest) (*asc.WebAppGroupDeleteResult, error) {
	request.GroupID = strings.TrimSpace(request.GroupID)
	if request.GroupID == "" {
		return nil, fmt.Errorf("group id is required")
	}
	if err := c.ensureDeveloperPortalSession(ctx); err != nil {
		return nil, developerAppGroupResponseError(err)
	}
	teamID := c.developerPortalTeamID()
	if teamID == "" {
		return nil, fmt.Errorf("%w; %s", ErrDeveloperPortalTeamNotSelected, developerPortalAuthHint)
	}

	groups, err := c.listDeveloperAppGroupPages(ctx, teamID, true, true)
	if err != nil {
		return nil, developerAppGroupResponseError(err)
	}
	group, found := findDeveloperAppGroup(groups, request.GroupID)
	if !found {
		return nil, &DeveloperAppGroupNotFoundError{GroupID: request.GroupID}
	}

	assignments, err := c.listDeveloperAppGroupAssignments(ctx, request.GroupID)
	if err != nil {
		return nil, developerAppGroupResponseError(err)
	}
	if len(assignments) > 0 {
		return nil, &DeveloperAppGroupInUseError{GroupID: group.ID, Identifier: group.Identifier, Assignments: assignments}
	}

	if err := c.primeDeveloperAppGroupCSRF(ctx); err != nil {
		return nil, err
	}
	body, err := c.doDeveloperPortalLegacyFormRequest(ctx, developerAppGroupsDeletePath, url.Values{
		"teamId":           {teamID},
		"applicationGroup": {request.GroupID},
	}, true)
	if err != nil {
		if !isAmbiguousDeveloperPortalWriteFailure(err) {
			return nil, err
		}
		// The request may have reached the portal; settle by re-reading before
		// telling the operator whether a retry is safe.
		remaining, readErr := c.listDeveloperAppGroupPages(ctx, teamID, true, true)
		if readErr != nil {
			return nil, &DeveloperAppGroupUnverifiedError{Err: fmt.Errorf("%w; the delete may have been applied but verification also failed: %w", err, readErr)}
		}
		if _, stillListed := findDeveloperAppGroup(remaining, request.GroupID); stillListed {
			return nil, err
		}
		return developerAppGroupDeleteReceipt(group), nil
	}
	var response developerPortalLegacyResponse
	if err := json.Unmarshal(body, &response); err != nil {
		// The portal returned success but nothing readable; the group may
		// already be gone, so a retry is not safe.
		return nil, &DeveloperAppGroupUnverifiedError{Err: fmt.Errorf("developer portal accepted the delete but its response could not be parsed: %w", err)}
	}
	if response.ResultCode == nil {
		// Valid JSON without a verdict is the same ambiguity as an unparseable
		// body; only an explicit non-zero resultCode is a refused delete.
		return nil, &DeveloperAppGroupUnverifiedError{Err: fmt.Errorf("developer portal accepted the delete but its response is missing resultCode")}
	}
	if err := validateDeveloperPortalLegacyResponse(response); err != nil {
		return nil, err
	}

	remaining, err := c.listDeveloperAppGroupPages(ctx, teamID, true, true)
	if err != nil {
		return nil, &DeveloperAppGroupUnverifiedError{Err: fmt.Errorf("developer portal accepted the delete but verification failed: %w", err)}
	}
	if _, stillListed := findDeveloperAppGroup(remaining, request.GroupID); stillListed {
		return nil, &DeveloperAppGroupUnverifiedError{Err: fmt.Errorf("developer portal accepted the delete but App Group %q is still listed; re-run 'asc web app-groups list' before retrying", request.GroupID)}
	}
	return developerAppGroupDeleteReceipt(group), nil
}

func developerAppGroupDeleteReceipt(group DeveloperAppGroup) *asc.WebAppGroupDeleteResult {
	return &asc.WebAppGroupDeleteResult{
		GroupID:    group.ID,
		Identifier: group.Identifier,
		Name:       group.Name,
		Deleted:    true,
		Status:     "deleted",
	}
}

// isAmbiguousDeveloperPortalWriteFailure reports whether a write failed in a
// way that leaves it unknown whether the portal applied it: the request was
// handed to the transport but no verdict came back. Explicit HTTP statuses and
// pre-send failures such as missing CSRF headers are not ambiguous.
func isAmbiguousDeveloperPortalWriteFailure(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return false
	}
	var urlErr *url.Error
	return errors.As(err, &urlErr) ||
		errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, io.EOF) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, context.Canceled)
}

func findDeveloperAppGroup(result *DeveloperAppGroupsListResult, groupID string) (DeveloperAppGroup, bool) {
	if result == nil {
		return DeveloperAppGroup{}, false
	}
	for _, group := range result.Data {
		if group.ID == groupID {
			return group, true
		}
	}
	return DeveloperAppGroup{}, false
}

// listDeveloperAppGroupAssignments walks every Bundle ID in the selected team
// and returns the ones whose APP_GROUPS capability references groupID. Any
// Bundle ID whose capability graph cannot be resolved is an error so callers
// never treat an unreadable graph as "unassigned".
func (c *Client) listDeveloperAppGroupAssignments(ctx context.Context, groupID string) ([]DeveloperAppGroupAssignment, error) {
	query := make(url.Values)
	query.Set("fields[bundleIds]", "name,identifier,platform,bundleIdCapabilities")
	query.Set("include", strings.Join(developerBundleIDsListIncludes, ","))
	query.Set("limit", strconv.Itoa(developerBundleIDsListPageSize))
	query.Set("limit[bundleIdCapabilities]", strconv.Itoa(developerBundleIDCapabilitiesIncludeLimit))

	assignments := []DeveloperAppGroupAssignment{}
	seenNext := make(map[string]struct{})
	seenBundleIDs := make(map[string]struct{})
	collected := 0
	var total *int
	for page := 0; ; page++ {
		if page >= developerBundleIDsListMaxPages {
			return nil, fmt.Errorf("developer portal Bundle ID listing exceeded %d pages while checking App Group assignments", developerBundleIDsListMaxPages)
		}
		body, err := c.doDeveloperPortalProxyRead(ctx, "/bundleIds", query, developerPortalHeaders(""))
		if err != nil {
			return nil, err
		}
		var response developerBundleIDListResponse
		if err := json.Unmarshal(body, &response); err != nil {
			return nil, fmt.Errorf("failed to parse Developer Portal Bundle ID list response: %w", err)
		}
		// A missing or null collection is not an empty team; treat it as
		// unreadable so the delete preflight stays fail-closed.
		if response.Data == nil {
			return nil, fmt.Errorf("cannot determine App Group assignments: Developer Portal Bundle ID list response has no data collection")
		}
		includedByID, err := indexDeveloperCapabilityResources(response.Included)
		if err != nil {
			return nil, fmt.Errorf("cannot determine App Group assignments: %w", err)
		}
		referencedCapabilities := make(map[string]struct{}, len(includedByID))
		for _, bundle := range response.Data {
			if bundle.Type != "bundleIds" || strings.TrimSpace(bundle.ID) == "" {
				return nil, fmt.Errorf("cannot determine App Group assignments: Developer Portal Bundle ID list contains a non-Bundle-ID entry (type %q, id %q)", bundle.Type, bundle.ID)
			}
			if _, duplicate := seenBundleIDs[bundle.ID]; duplicate {
				return nil, fmt.Errorf("cannot determine App Group assignments: Developer Portal Bundle ID list repeated Bundle ID %q across pages", bundle.ID)
			}
			seenBundleIDs[bundle.ID] = struct{}{}
			assignment, referenced, err := developerBundleIDReferencesAppGroup(bundle, includedByID, groupID, referencedCapabilities)
			if err != nil {
				return nil, err
			}
			if referenced {
				assignments = append(assignments, assignment)
			}
		}
		// An included capability no Bundle ID on the page references has an
		// unknown owner; if it lists the target group, the assignment cannot be
		// attributed and the page is unreadable rather than "unused".
		for capabilityID := range includedByID {
			if _, ok := referencedCapabilities[capabilityID]; !ok {
				return nil, fmt.Errorf("cannot determine App Group assignments: Developer Portal included capability %q that no listed Bundle ID references", capabilityID)
			}
		}
		collected += len(response.Data)

		// The paging total describes one listing, so a later page that reports
		// a different value is evidence the listing shifted underneath us.
		if response.Meta.Paging.Total != nil {
			if total != nil && *total != *response.Meta.Paging.Total {
				return nil, fmt.Errorf("cannot determine App Group assignments: Developer Portal Bundle ID list changed its paging total from %d to %d between pages", *total, *response.Meta.Paging.Total)
			}
			total = response.Meta.Paging.Total
		}
		if total != nil && *total < collected {
			return nil, fmt.Errorf("cannot determine App Group assignments: Developer Portal Bundle ID list reported %d total records but returned %d", *total, collected)
		}
		next := strings.TrimSpace(response.Links.Next)
		if next == "" {
			// Without a next link the listing is complete only if the paging
			// total agrees, or, when no total is provided, the last page was
			// short; a full final page with no terminal indicator is ambiguous.
			switch {
			case total != nil && *total != collected:
				return nil, fmt.Errorf("cannot determine App Group assignments: Developer Portal Bundle ID list reported %d total records but ended after %d", *total, collected)
			case total == nil && len(response.Data) >= developerBundleIDsListPageSize:
				return nil, fmt.Errorf("cannot determine App Group assignments: Developer Portal Bundle ID list ended on a full page of %d without a next link or paging total", len(response.Data))
			}
			break
		}
		if _, repeated := seenNext[next]; repeated {
			return nil, fmt.Errorf("developer portal Bundle ID listing repeated pagination cursor while checking App Group assignments")
		}
		seenNext[next] = struct{}{}
		nextURL, err := url.Parse(next)
		if err != nil {
			return nil, fmt.Errorf("invalid Developer Portal Bundle ID pagination link: %w", err)
		}
		// Overlay the cursor link on the original query so the include and
		// field selections survive even if Apple returns a cursor-only link.
		for key, values := range nextURL.Query() {
			query[key] = values
		}
	}
	return assignments, nil
}

// developerBundleIDReferencesAppGroup reports whether one Bundle ID lists
// groupID and records every capability it references in referenced so the
// caller can detect included capabilities that no Bundle ID owns.
func developerBundleIDReferencesAppGroup(bundle developerResource, includedByID map[string]developerResource, groupID string, referenced map[string]struct{}) (DeveloperAppGroupAssignment, bool, error) {
	assignment := DeveloperAppGroupAssignment{BundleID: strings.TrimSpace(bundle.ID)}
	if len(bundle.Attributes) > 0 {
		var attributes struct {
			Name       string `json:"name"`
			Identifier string `json:"identifier"`
		}
		if err := json.Unmarshal(bundle.Attributes, &attributes); err != nil {
			return assignment, false, fmt.Errorf("failed to parse Bundle ID %q attributes: %w", bundle.ID, err)
		}
		assignment.Name = strings.TrimSpace(attributes.Name)
		assignment.Identifier = strings.TrimSpace(attributes.Identifier)
	}
	label := assignment.Identifier
	if label == "" {
		label = assignment.BundleID
	}

	rawRelationship, ok := bundle.Relationships["bundleIdCapabilities"]
	if !ok {
		return assignment, false, fmt.Errorf("cannot determine App Group assignments for Bundle ID %q: Developer Portal omitted its capability relationships", label)
	}
	capabilityReferences, err := decodeStrictDeveloperRelationship(rawRelationship)
	if err != nil {
		return assignment, false, fmt.Errorf("cannot determine App Group assignments for Bundle ID %q: capability relationship %w", label, err)
	}
	if err := validateDeveloperRelationshipCompleteness(rawRelationship, len(capabilityReferences), developerBundleIDCapabilitiesIncludeLimit, "capability relationship"); err != nil {
		return assignment, false, fmt.Errorf("cannot determine App Group assignments for Bundle ID %q: %w", label, err)
	}
	for _, reference := range capabilityReferences {
		if reference.Type != "bundleIdCapabilities" || strings.TrimSpace(reference.ID) == "" {
			return assignment, false, fmt.Errorf("cannot determine App Group assignments for Bundle ID %q: capability relationship contains an invalid reference (type %q, id %q)", label, reference.Type, reference.ID)
		}
	}
	// Record every reference before inspecting any, since a match returns
	// early and must not leave this Bundle ID's other capabilities unclaimed.
	for _, reference := range capabilityReferences {
		referenced[reference.ID] = struct{}{}
	}
	for _, reference := range capabilityReferences {
		capability, included := includedByID[reference.ID]
		if !included {
			return assignment, false, fmt.Errorf("cannot determine App Group assignments for Bundle ID %q: capability %q missing from Developer Portal response", label, reference.ID)
		}
		capabilityID, err := developerBundleIDCapabilityID(capability)
		if err != nil {
			return assignment, false, fmt.Errorf("cannot determine App Group assignments for Bundle ID %q: %w", label, err)
		}
		if capabilityID != developerAppGroupsCapabilityType {
			continue
		}
		// The preflight requested include=bundleIdCapabilities.appGroups, so an
		// APP_GROUPS capability without a readable appGroups collection is
		// unreadable rather than empty.
		rawGroups, exists := capability.Relationships["appGroups"]
		if !exists {
			return assignment, false, fmt.Errorf("cannot determine App Group assignments for Bundle ID %q: Developer Portal omitted the appGroups relationship of capability %q", label, reference.ID)
		}
		if _, err := decodeStrictDeveloperRelationship(rawGroups); err != nil {
			return assignment, false, fmt.Errorf("cannot determine App Group assignments for Bundle ID %q: appGroups relationship %w", label, err)
		}
		groups, err := developerAppGroupRelationships(capability)
		if err != nil {
			return assignment, false, fmt.Errorf("cannot determine App Group assignments for Bundle ID %q: %w", label, err)
		}
		if err := validateDeveloperRelationshipCompleteness(rawGroups, len(groups), 0, "appGroups relationship"); err != nil {
			return assignment, false, fmt.Errorf("cannot determine App Group assignments for Bundle ID %q: %w", label, err)
		}
		if containsDeveloperResource(groups, "appGroups", groupID) {
			return assignment, true, nil
		}
	}
	return assignment, false, nil
}

// indexDeveloperCapabilityResources maps included bundleIdCapabilities
// resources by ID. Two different representations of the same ID would let
// whichever one wins hide an assignment, so only identical repeats are
// tolerated.
func indexDeveloperCapabilityResources(included []developerResource) (map[string]developerResource, error) {
	indexed := make(map[string]developerResource, len(included))
	for _, resource := range included {
		if resource.Type != "bundleIdCapabilities" || strings.TrimSpace(resource.ID) == "" {
			continue
		}
		if previous, duplicate := indexed[resource.ID]; duplicate && !reflect.DeepEqual(previous, resource) {
			return nil, fmt.Errorf("developer portal returned conflicting representations of capability %q", resource.ID)
		}
		indexed[resource.ID] = resource
	}
	return indexed, nil
}

// decodeStrictDeveloperRelationship decodes a JSON:API relationship object and
// rejects an absent or null data collection, which destructive preflights must
// treat as unreadable rather than as an empty set.
func decodeStrictDeveloperRelationship(raw json.RawMessage) ([]developerResource, error) {
	var relationship developerResourceRelationship
	if err := json.Unmarshal(raw, &relationship); err != nil {
		return nil, fmt.Errorf("could not be parsed: %w", err)
	}
	if relationship.Data == nil {
		return nil, fmt.Errorf("has no data collection")
	}
	return relationship.Data, nil
}

// developerAppGroupAcceptedCreateError keeps an explicit portal refusal a
// refusal while reporting a 2xx envelope that carries no verdict as an
// accepted but unverified create: the group may already exist, so a blind
// retry is unsafe.
func developerAppGroupAcceptedCreateError(err error) error {
	var resultErr *DeveloperPortalResultError
	if errors.As(err, &resultErr) {
		return err
	}
	return &DeveloperAppGroupUnverifiedError{Err: fmt.Errorf("developer portal accepted the create but %w", err)}
}

// CreateDeveloperAppGroup registers an App Group through Developer Portal.
func (c *Client) CreateDeveloperAppGroup(ctx context.Context, request DeveloperAppGroupCreateRequest) (*DeveloperAppGroup, error) {
	request.Name = strings.TrimSpace(request.Name)
	request.Identifier = strings.TrimSpace(request.Identifier)
	if request.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if err := ValidateDeveloperAppGroupIdentifier(request.Identifier); err != nil {
		return nil, err
	}
	if err := c.ensureDeveloperPortalSession(ctx); err != nil {
		return nil, developerAppGroupResponseError(err)
	}
	if err := c.primeDeveloperAppGroupCSRF(ctx); err != nil {
		return nil, err
	}
	teamID := c.developerPortalTeamID()
	if teamID == "" {
		return nil, fmt.Errorf("%w; %s", ErrDeveloperPortalTeamNotSelected, developerPortalAuthHint)
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
		// The portal answered 2xx, so the group may already be registered; a
		// retry is not safe until the operator re-reads the team's list.
		return nil, &DeveloperAppGroupUnverifiedError{Err: fmt.Errorf("developer portal accepted the create but failed to parse Developer Portal App Group create response: %w", err)}
	}
	if err := validateDeveloperPortalLegacyResponse(response.developerPortalLegacyResponse); err != nil {
		return nil, developerAppGroupAcceptedCreateError(err)
	}
	group, err := decodeDeveloperAppGroup(response.ApplicationGroup)
	if err != nil {
		return nil, &DeveloperAppGroupUnverifiedError{Err: fmt.Errorf("developer portal accepted the create but its receipt could not be read: %w", err)}
	}
	return &group, nil
}

// rejectDeveloperAppGroupIdentifier refuses the most common assignment
// mistake before any write: passing the App Group identifier
// ("group.com.example.shared") where Apple's opaque App Group resource ID is
// required, which the portal can only answer with an unhelpful refusal.
//
// Only a value that cannot be confused with an opaque ID is examined, and only
// against the team's own listing, so a team whose resource IDs happen to look
// like identifiers is never refused. A listing that cannot be read leaves the
// assignment to proceed exactly as before.
func (c *Client) rejectDeveloperAppGroupIdentifier(ctx context.Context, groupID string) error {
	if !strings.HasPrefix(groupID, developerAppGroupIdentifierPrefix) {
		return nil
	}
	teamID := c.developerPortalTeamID()
	if teamID == "" {
		return nil
	}
	// Only a complete listing can prove the value is not a resource ID: a
	// success envelope that is short or sparse may omit the very group that
	// was named, so the strict read is required and any failure leaves the
	// assignment to proceed.
	groups, err := c.listDeveloperAppGroupPages(ctx, teamID, true, true)
	if err != nil || groups == nil {
		return nil
	}
	for _, group := range groups.Data {
		if group.ID == groupID {
			return nil
		}
	}
	for _, group := range groups.Data {
		if group.Identifier == groupID {
			return &DeveloperAppGroupIdentifierError{Given: groupID, GroupID: group.ID, Identifier: group.Identifier}
		}
	}
	return &DeveloperAppGroupIdentifierError{Given: groupID}
}

// AssignDeveloperAppGroup associates an App Group with a Bundle ID while
// preserving Apple's complete current capability graph. The result is verified
// by re-reading the Bundle ID.
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
		return nil, developerAppGroupResponseError(err)
	}
	if err := c.rejectDeveloperAppGroupIdentifier(ctx, request.GroupID); err != nil {
		return nil, err
	}
	current, state, err := c.loadDeveloperBundleIDAppGroups(ctx, request.BundleID)
	if err != nil {
		return nil, err
	}
	if state.Enabled && slices.Contains(state.GroupIDs, request.GroupID) {
		return &DeveloperAppGroupAssignResult{BundleID: request.BundleID, GroupID: request.GroupID, Changed: false, Status: "already-assigned"}, nil
	}
	desired := append([]string{}, state.GroupIDs...)
	if !slices.Contains(desired, request.GroupID) {
		desired = append(desired, request.GroupID)
	}
	if err := c.applyDeveloperAppGroups(ctx, current, state, true, desired); err != nil {
		return nil, err
	}
	return &DeveloperAppGroupAssignResult{BundleID: request.BundleID, GroupID: request.GroupID, Changed: true, Status: "assigned"}, nil
}

// UnassignDeveloperAppGroup removes one App Group from a Bundle ID while
// preserving every other capability. It operates on the raw relationship data
// so a group listed under a disabled APP_GROUPS capability can still be
// cleared (the delete preflight counts such groups as in use). Removing the
// last group disables the capability; a capability Apple already reports
// disabled stays disabled. The result is verified by re-reading the Bundle ID.
func (c *Client) UnassignDeveloperAppGroup(ctx context.Context, request DeveloperAppGroupUnassignRequest) (*asc.WebAppGroupUnassignResult, error) {
	request.BundleID = strings.TrimSpace(request.BundleID)
	request.GroupID = strings.TrimSpace(request.GroupID)
	if request.BundleID == "" {
		return nil, fmt.Errorf("bundle id is required")
	}
	if request.GroupID == "" {
		return nil, fmt.Errorf("group id is required")
	}
	if err := c.ensureDeveloperPortalSession(ctx); err != nil {
		return nil, developerAppGroupResponseError(err)
	}
	current, state, err := c.loadDeveloperBundleIDAppGroups(ctx, request.BundleID)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(state.GroupIDs, request.GroupID) {
		return &asc.WebAppGroupUnassignResult{BundleID: request.BundleID, GroupID: request.GroupID, RemainingGroupIDs: append([]string{}, state.GroupIDs...), Changed: false, Status: "not-assigned"}, nil
	}
	desired := make([]string, 0, len(state.GroupIDs))
	for _, id := range state.GroupIDs {
		if id != request.GroupID {
			desired = append(desired, id)
		}
	}
	enabled := state.Enabled && len(desired) > 0
	if err := c.applyDeveloperAppGroups(ctx, current, state, enabled, desired); err != nil {
		return nil, err
	}
	return &asc.WebAppGroupUnassignResult{BundleID: request.BundleID, GroupID: request.GroupID, RemainingGroupIDs: desired, Changed: true, Status: "unassigned"}, nil
}

// SetDeveloperAppGroups converges a Bundle ID on exactly the requested App
// Group set, reports the added and removed groups, and skips the write when the
// current set already matches. The result is verified by re-reading the Bundle ID.
func (c *Client) SetDeveloperAppGroups(ctx context.Context, request DeveloperAppGroupSetRequest) (*asc.WebAppGroupSetResult, error) {
	request.BundleID = strings.TrimSpace(request.BundleID)
	if request.BundleID == "" {
		return nil, fmt.Errorf("bundle id is required")
	}
	desired := dedupeTrimmedStrings(request.GroupIDs)
	if len(desired) == 0 {
		return nil, fmt.Errorf("at least one group id is required")
	}
	if err := c.ensureDeveloperPortalSession(ctx); err != nil {
		return nil, developerAppGroupResponseError(err)
	}
	current, state, err := c.loadDeveloperBundleIDAppGroups(ctx, request.BundleID)
	if err != nil {
		return nil, err
	}
	added := differenceStrings(desired, state.GroupIDs)
	removed := differenceStrings(state.GroupIDs, desired)
	result := &asc.WebAppGroupSetResult{BundleID: request.BundleID, GroupIDs: desired, Added: added, Removed: removed}
	// A disabled capability that already lists the desired groups still needs a
	// write so the groups become effective.
	if state.matches(true, desired) {
		result.Changed = false
		result.Status = "unchanged"
		return result, nil
	}
	if err := c.applyDeveloperAppGroups(ctx, current, state, true, desired); err != nil {
		return nil, err
	}
	result.Changed = true
	result.Status = "updated"
	return result, nil
}

// applyDeveloperAppGroups PATCHes the desired APP_GROUPS state and verifies it
// by re-reading the Bundle ID. A write that fails without a verdict is settled
// by the same read: the desired state means it applied, the prior state means
// it did not and the original error is safe to retry, anything else is
// unverified.
func (c *Client) applyDeveloperAppGroups(ctx context.Context, current developerBundleIDResponse, before developerAppGroupsState, enabled bool, desired []string) error {
	bundleID := current.Data.ID
	payload, err := c.prepareDeveloperAppGroupsPatch(ctx, current, enabled, desired)
	if err != nil {
		// Nothing was sent yet, so this failure is plainly retry-safe and must
		// not be settled as an ambiguous write.
		return err
	}
	_, writeErr := c.doDeveloperPortalRequest(ctx, http.MethodPatch, "/bundleIds/"+url.PathEscape(bundleID), payload, developerPortalHeaders(bundleID), true)
	if writeErr == nil {
		return c.verifyDeveloperAppGroups(ctx, bundleID, enabled, desired)
	}
	if !isAmbiguousDeveloperPortalWriteFailure(writeErr) {
		return writeErr
	}
	_, state, readErr := c.loadDeveloperBundleIDAppGroups(ctx, bundleID)
	if readErr != nil {
		return &DeveloperAppGroupUnverifiedError{Err: fmt.Errorf("%w; the update may have been applied but verification also failed: %w", writeErr, readErr)}
	}
	switch {
	case state.matches(enabled, desired):
		return nil
	case state.matches(before.Enabled, before.GroupIDs):
		return writeErr
	default:
		return &DeveloperAppGroupUnverifiedError{Err: fmt.Errorf("%w; Bundle ID %q now reports APP_GROUPS enabled=%t with groups [%s], which is neither the previous nor the requested state", writeErr, bundleID, state.Enabled, strings.Join(state.GroupIDs, ", "))}
	}
}

// prepareDeveloperAppGroupsPatch builds the PATCH body and primes the CSRF
// tokens it needs without sending the write itself.
func (c *Client) prepareDeveloperAppGroupsPatch(ctx context.Context, current developerBundleIDResponse, enabled bool, desired []string) (developerBundleIDPatchRequest, error) {
	payload, err := buildDeveloperAppGroupsPatchRequest(current, enabled, desired)
	if err != nil {
		return developerBundleIDPatchRequest{}, err
	}
	if err := c.primeDeveloperAppGroupCSRF(ctx); err != nil {
		return developerBundleIDPatchRequest{}, err
	}
	return addDeveloperPortalTeamID(payload, c.developerPortalTeamID())
}

// loadDeveloperBundleIDAppGroups reads one Bundle ID for an App Group mutation
// and rejects a response whose resource ID is not exactly the requested one, so
// the follow-up PATCH can never target a Bundle ID the caller did not name.
func (c *Client) loadDeveloperBundleIDAppGroups(ctx context.Context, bundleID string) (developerBundleIDResponse, developerAppGroupsState, error) {
	current, err := c.loadDeveloperBundleID(ctx, bundleID)
	if err != nil {
		return developerBundleIDResponse{}, developerAppGroupsState{}, developerAppGroupResponseError(err)
	}
	if current.Data.ID != bundleID {
		return developerBundleIDResponse{}, developerAppGroupsState{}, &DeveloperAppGroupUnreadableResponseError{Err: fmt.Errorf("cannot safely update Bundle ID %q: Developer Portal returned resource %q instead", bundleID, current.Data.ID)}
	}
	state, err := developerBundleIDAppGroupsState(current)
	if err != nil {
		// Classify the fail-closed preflight so telemetry separates an
		// unreadable Developer Portal response from an unclassified internal
		// failure. No write was sent.
		return developerBundleIDResponse{}, developerAppGroupsState{}, &DeveloperAppGroupUnreadableResponseError{Err: err}
	}
	return current, state, nil
}

func (c *Client) verifyDeveloperAppGroups(ctx context.Context, bundleID string, enabled bool, desired []string) error {
	_, state, err := c.loadDeveloperBundleIDAppGroups(ctx, bundleID)
	if err != nil {
		return &DeveloperAppGroupUnverifiedError{Err: fmt.Errorf("developer portal accepted the update but verification failed: %w", err)}
	}
	if !state.matches(enabled, desired) {
		return &DeveloperAppGroupUnverifiedError{Err: fmt.Errorf("developer portal accepted the update but Bundle ID %q still reports APP_GROUPS enabled=%t with groups [%s] instead of enabled=%t with [%s]", bundleID, state.Enabled, strings.Join(state.GroupIDs, ", "), enabled, strings.Join(desired, ", "))}
	}
	return nil
}

func (c *Client) primeDeveloperAppGroupCSRF(ctx context.Context) error {
	teamID := c.developerPortalTeamID()
	if teamID == "" {
		return fmt.Errorf("%w; %s", ErrDeveloperPortalTeamNotSelected, developerPortalAuthHint)
	}
	c.clearDeveloperCSRFTokens()
	body, err := c.doDeveloperPortalLegacyFormRequest(readonly.WithReadIntent(ctx), developerAppGroupsListPath, url.Values{
		"teamId":     {teamID},
		"pageNumber": {"1"},
		"pageSize":   {strconv.Itoa(developerAppGroupsPageSize)},
		"sort":       {"name=asc"},
	}, false)
	if err != nil {
		return err
	}
	var response developerAppGroupsListResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return &DeveloperAppGroupUnreadableResponseError{Err: fmt.Errorf("failed to parse Developer Portal App Groups response while priming CSRF: %w", err)}
	}
	if err := validateDeveloperPortalLegacyResponse(response.developerPortalLegacyResponse); err != nil {
		return err
	}
	csrf, csrfTS := c.developerCSRFTokens()
	if csrf == "" || csrfTS == "" {
		return fmt.Errorf("missing Developer Portal CSRF headers after App Groups lookup; %s", developerPortalAuthHint)
	}
	return nil
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

// ValidateDeveloperAppGroupIdentifier validates an App Group identifier before
// any Developer Portal request is attempted.
func ValidateDeveloperAppGroupIdentifier(identifier string) error {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return fmt.Errorf("identifier is required")
	}
	if !strings.HasPrefix(identifier, "group.") {
		return fmt.Errorf("identifier must start with \"group.\"")
	}
	suffix := strings.TrimPrefix(identifier, "group.")
	if suffix == "" {
		return fmt.Errorf("identifier must include a name after \"group.\"")
	}
	for _, character := range suffix {
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '-' || character == '.' {
			continue
		}
		return fmt.Errorf("identifier may contain only letters, numbers, hyphens, and periods")
	}
	return nil
}

func validateDeveloperPortalLegacyResponse(response developerPortalLegacyResponse) error {
	if response.ResultCode == nil {
		return &DeveloperAppGroupUnreadableResponseError{Err: fmt.Errorf("developer portal response is missing resultCode")}
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
	return &DeveloperPortalResultError{ResultCode: *response.ResultCode, RequestID: response.RequestID, Message: message}
}

// developerBundleIDCapabilityReferencesForReplacement resolves the capability
// references that any destructive Bundle ID mutation has to PATCH back.
//
// Every App Group mutation replaces the complete bundleIdCapabilities
// relationship, so a graph that cannot be read must abort before any write
// rather than be rewritten as "no other capabilities". The Bundle ID detail
// request explicitly selects bundleIdCapabilities and asks for the maximum
// supported included count. A missing or links-only relationship therefore
// cannot prove which capabilities belong to the Bundle ID and must abort. The
// included array alone is not a completeness boundary: it has no independent
// pagination metadata and a replacement PATCH built from a truncated array
// could silently detach unrelated capabilities.
func developerBundleIDCapabilityReferencesForReplacement(current developerBundleIDResponse) ([]developerResource, error) {
	rawRelationship, hasRelationship := current.Data.Relationships["bundleIdCapabilities"]
	if !hasRelationship || developerRelationshipUnresolved(rawRelationship) {
		return nil, fmt.Errorf("cannot safely update Bundle ID %q: Developer Portal did not resolve the selected bundleIdCapabilities relationship", current.Data.ID)
	}
	references, err := decodeStrictDeveloperRelationship(rawRelationship)
	if err != nil {
		return nil, fmt.Errorf("cannot safely update Bundle ID %q: capability graph %w", current.Data.ID, err)
	}
	if err := validateDeveloperRelationshipCompleteness(rawRelationship, len(references), developerBundleIDCapabilitiesIncludeLimit, "Bundle ID capability graph"); err != nil {
		return nil, fmt.Errorf("cannot safely update Bundle ID %q: %w", current.Data.ID, err)
	}
	// developerBundleIDCapabilities drops references it cannot resolve; a
	// PATCH built from that filtered graph would silently detach them, so
	// reject every invalid reference before any write is computed.
	for _, reference := range references {
		if reference.Type != "bundleIdCapabilities" || strings.TrimSpace(reference.ID) == "" {
			return nil, fmt.Errorf("cannot safely update Bundle ID %q: capability graph contains an invalid reference (type %q, id %q)", current.Data.ID, reference.Type, reference.ID)
		}
	}
	return references, nil
}

// validateDeveloperRelationshipCompleteness rejects pagination evidence that
// would make a destructive relationship replacement partial. When a maximum
// include limit was requested, a full page without a matching total is also
// ambiguous: Apple may have truncated the included resources at that limit.
func validateDeveloperRelationshipCompleteness(raw json.RawMessage, returned, requestedLimit int, label string) error {
	var members map[string]json.RawMessage
	if err := json.Unmarshal(raw, &members); err != nil || members == nil {
		return fmt.Errorf("%s could not be parsed", label)
	}
	hasExactTotal := false
	if rawLinks, ok := members["links"]; ok && string(rawLinks) != "null" {
		var links map[string]json.RawMessage
		if err := json.Unmarshal(rawLinks, &links); err != nil || links == nil {
			return fmt.Errorf("%s links are unreadable", label)
		}
		if rawNext, ok := links["next"]; ok && string(rawNext) != "null" {
			var next string
			if err := json.Unmarshal(rawNext, &next); err != nil {
				return fmt.Errorf("%s next link is unreadable", label)
			}
			if strings.TrimSpace(next) != "" {
				return fmt.Errorf("%s is paginated and therefore incomplete", label)
			}
		}
	}
	if rawMeta, ok := members["meta"]; ok && string(rawMeta) != "null" {
		var meta map[string]json.RawMessage
		if err := json.Unmarshal(rawMeta, &meta); err != nil || meta == nil {
			return fmt.Errorf("%s metadata is unreadable", label)
		}
		if rawPaging, ok := meta["paging"]; ok && string(rawPaging) != "null" {
			var paging map[string]json.RawMessage
			if err := json.Unmarshal(rawPaging, &paging); err != nil || paging == nil {
				return fmt.Errorf("%s paging metadata is unreadable", label)
			}
			if rawTotal, ok := paging["total"]; ok && string(rawTotal) != "null" {
				var total int
				if err := json.Unmarshal(rawTotal, &total); err != nil || total < 0 {
					return fmt.Errorf("%s paging total is unreadable", label)
				}
				if total != returned {
					return fmt.Errorf("%s returned %d of %d resources", label, returned, total)
				}
				hasExactTotal = true
			}
		}
	}
	if requestedLimit > 0 && returned >= requestedLimit && !hasExactTotal {
		return fmt.Errorf("%s may be truncated at the requested limit of %d", label, requestedLimit)
	}
	return nil
}

// developerRelationshipUnresolved reports whether a JSON:API relationship
// object states no linkage because Apple did not select it: a well-formed
// object that carries links and no data member. A null value, a non-object, an
// empty object, and anything carrying a data member are all handled by the
// strict decoder instead, so a malformed relationship can never hand the
// capability graph over to included.
func developerRelationshipUnresolved(raw json.RawMessage) bool {
	var members map[string]json.RawMessage
	if err := json.Unmarshal(raw, &members); err != nil || members == nil {
		return false
	}
	if _, resolved := members["data"]; resolved {
		return false
	}
	links, hasLinks := members["links"]
	if !hasLinks {
		return false
	}
	// A links member that is not itself a readable links object makes the
	// whole relationship malformed rather than merely unselected, so the
	// strict decoder handles it instead of the included fallback.
	var linkMembers map[string]json.RawMessage
	return json.Unmarshal(links, &linkMembers) == nil && linkMembers != nil
}

// developerBundleIDAppGroupsState reads the APP_GROUPS capability of a Bundle
// ID: whether it is enabled and which groups it currently lists.
func developerBundleIDAppGroupsState(current developerBundleIDResponse) (developerAppGroupsState, error) {
	references, err := developerBundleIDCapabilityReferencesForReplacement(current)
	if err != nil {
		return developerAppGroupsState{}, err
	}
	// developerBundleIDCapabilities lets the last included copy of an ID win, so
	// conflicting duplicates must be rejected before the graph is rebuilt.
	if _, err := indexDeveloperCapabilityResources(current.Included); err != nil {
		return developerAppGroupsState{}, fmt.Errorf("cannot safely update Bundle ID %q: %w", current.Data.ID, err)
	}
	// developerBundleIDCapabilities also carries included capabilities the
	// primary resource never referenced into the replacement PATCH; for this
	// Bundle ID that would attach a capability it does not own.
	referenced := make(map[string]struct{}, len(references))
	for _, reference := range references {
		referenced[reference.ID] = struct{}{}
	}
	for _, resource := range current.Included {
		if resource.Type != "bundleIdCapabilities" || strings.TrimSpace(resource.ID) == "" {
			continue
		}
		if _, ok := referenced[resource.ID]; !ok {
			return developerAppGroupsState{}, fmt.Errorf("cannot safely update Bundle ID %q: Developer Portal included capability %q that the Bundle ID does not reference", current.Data.ID, resource.ID)
		}
	}
	capabilities, err := developerBundleIDCapabilities(current)
	if err != nil {
		return developerAppGroupsState{}, err
	}
	state := developerAppGroupsState{GroupIDs: []string{}}
	found := false
	for _, capability := range capabilities {
		capabilityID, err := developerBundleIDCapabilityID(capability)
		if err != nil {
			return developerAppGroupsState{}, err
		}
		if capabilityID != developerAppGroupsCapabilityType {
			continue
		}
		if found {
			return developerAppGroupsState{}, fmt.Errorf("cannot safely update duplicate APP_GROUPS capability resources")
		}
		found = true
		state.Enabled, err = developerAppGroupsCapabilityEnabled(capability)
		if err != nil {
			return developerAppGroupsState{}, fmt.Errorf("cannot safely update Bundle ID %q: %w", current.Data.ID, err)
		}
		// The read requested include=bundleIdCapabilities.appGroups; the PATCH
		// replaces this collection wholesale, so it must be readable first.
		rawGroups, exists := capability.Relationships["appGroups"]
		if !exists {
			return developerAppGroupsState{}, fmt.Errorf("cannot safely update Bundle ID %q: Developer Portal omitted the appGroups relationship of its APP_GROUPS capability", current.Data.ID)
		}
		if _, err := decodeStrictDeveloperRelationship(rawGroups); err != nil {
			return developerAppGroupsState{}, fmt.Errorf("cannot safely update Bundle ID %q: APP_GROUPS appGroups relationship %w", current.Data.ID, err)
		}
		groups, err := developerAppGroupRelationships(capability)
		if err != nil {
			return developerAppGroupsState{}, err
		}
		if err := validateDeveloperRelationshipCompleteness(rawGroups, len(groups), 0, "APP_GROUPS appGroups relationship"); err != nil {
			return developerAppGroupsState{}, fmt.Errorf("cannot safely update Bundle ID %q: %w", current.Data.ID, err)
		}
		for _, group := range groups {
			state.GroupIDs = append(state.GroupIDs, group.ID)
		}
	}
	return state, nil
}

// developerAppGroupsCapabilityEnabled reads the enabled state of the APP_GROUPS
// capability strictly. unassign carries the current state forward into the
// PATCH, so an omitted or null value must be unreadable rather than "false",
// which would disable every remaining assignment.
func developerAppGroupsCapabilityEnabled(capability developerResource) (bool, error) {
	if len(capability.Attributes) == 0 {
		return false, fmt.Errorf("APP_GROUPS capability %q is missing attributes", capability.ID)
	}
	var attributes map[string]json.RawMessage
	if err := json.Unmarshal(capability.Attributes, &attributes); err != nil {
		return false, fmt.Errorf("failed to parse APP_GROUPS capability %q attributes: %w", capability.ID, err)
	}
	raw, ok := attributes["enabled"]
	if !ok || string(raw) == "null" {
		return false, fmt.Errorf("APP_GROUPS capability %q has no readable enabled state", capability.ID)
	}
	var enabled bool
	if err := json.Unmarshal(raw, &enabled); err != nil {
		return false, fmt.Errorf("failed to parse APP_GROUPS capability %q enabled state: %w", capability.ID, err)
	}
	return enabled, nil
}

// buildDeveloperAppGroupsPatchRequest rewrites only the APP_GROUPS capability
// so it lists exactly the desired groups with the requested enabled state,
// while preserving every other capability and relationship Apple returned. A
// missing capability is only created when it should be enabled.
func buildDeveloperAppGroupsPatchRequest(current developerBundleIDResponse, enabled bool, desired []string) (developerBundleIDPatchRequest, error) {
	capabilities, err := developerBundleIDCapabilities(current)
	if err != nil {
		return developerBundleIDPatchRequest{}, err
	}
	groups := make([]developerResource, 0, len(desired))
	for _, groupID := range desired {
		groups = append(groups, developerResource{Type: "appGroups", ID: groupID})
	}

	updated := make([]developerResource, 0, len(capabilities)+1)
	foundAppGroups := false
	for _, capability := range capabilities {
		capabilityID, err := developerBundleIDCapabilityID(capability)
		if err != nil {
			return developerBundleIDPatchRequest{}, err
		}
		if capabilityID != developerAppGroupsCapabilityType {
			updated = append(updated, capability)
			continue
		}
		if foundAppGroups {
			return developerBundleIDPatchRequest{}, fmt.Errorf("cannot safely update duplicate APP_GROUPS capability resources")
		}
		foundAppGroups = true
		capability.Attributes, err = setDeveloperCapabilityEnabledValue(capability.Attributes, enabled)
		if err != nil {
			return developerBundleIDPatchRequest{}, err
		}
		if err := setDeveloperAppGroupRelationships(&capability, groups); err != nil {
			return developerBundleIDPatchRequest{}, err
		}
		updated = append(updated, capability)
	}
	if !foundAppGroups && enabled {
		capability := newDeveloperBundleIDCapability(developerAppGroupsCapabilityType)
		if err := setDeveloperAppGroupRelationships(&capability, groups); err != nil {
			return developerBundleIDPatchRequest{}, err
		}
		updated = append(updated, capability)
	}

	relationship, err := marshalDeveloperBundleIDCapabilitiesForPatch(updated)
	if err != nil {
		return developerBundleIDPatchRequest{}, err
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
	return payload, nil
}

func dedupeTrimmedStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || slices.Contains(result, trimmed) {
			continue
		}
		result = append(result, trimmed)
	}
	return result
}

// differenceStrings returns the values of left that are absent from right,
// preserving left's order.
func differenceStrings(left, right []string) []string {
	result := []string{}
	for _, value := range left {
		if !slices.Contains(right, value) {
			result = append(result, value)
		}
	}
	return result
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
	// IDs are compared exactly everywhere, so a padded ID would silently miss
	// its canonical form; reject it rather than guess at a normalization.
	for _, group := range relationship.Data {
		if group.Type != "appGroups" || group.ID == "" || group.ID != strings.TrimSpace(group.ID) {
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
