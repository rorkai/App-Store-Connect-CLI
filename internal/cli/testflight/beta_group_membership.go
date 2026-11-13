package testflight

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// betaGroupMembershipReadBack records, for one group, which of the requested
// testers a post-conflict read-back found already in the group.
type betaGroupMembershipReadBack struct {
	groupID string
	present []string
	missing []string
}

// satisfied reports whether every requested tester is already a member, which
// is the end state the add asked for.
func (r betaGroupMembershipReadBack) satisfied() bool {
	return len(r.present) > 0 && len(r.missing) == 0
}

// diagnostic renders the read-back for an error message so an operator can see
// which testers still need adding.
func (r betaGroupMembershipReadBack) diagnostic() string {
	parts := make([]string, 0, 2)
	if len(r.present) > 0 {
		parts = append(parts, fmt.Sprintf("already in group %s: %s", r.groupID, strings.Join(r.present, ", ")))
	}
	if len(r.missing) > 0 {
		parts = append(parts, fmt.Sprintf("not in group %s: %s", r.groupID, strings.Join(r.missing, ", ")))
	}
	if len(parts) == 0 {
		return ""
	}
	return " (" + strings.Join(parts, "; ") + ")"
}

// isHTTPConflict reports whether err is an App Store Connect HTTP 409.
//
// Apple's code for "this tester is already in the group" is not documented and
// could not be captured: the codes observed on
// POST /v1/betaGroups/{id}/relationships/betaTesters are STATE_ERROR ("Tester(s)
// cannot be assigned", Apple Developer Forums thread 745785) and
// ENTITY_ERROR.RELATIONSHIP.INVALID, and STATE_ERROR is reported both for a
// tester the group already contains and for a tester that genuinely cannot be
// assigned. A code list would therefore either miss the real conflict or admit
// unrelated ones, so every 409 is only a candidate here and the caller's
// membership read-back stays the decisive check: the conflict is forgiven only
// when every requested tester is already a member, which is exactly the end
// state the add requested. Any other status (and any 409 whose read-back finds
// a tester missing) fails exactly as before.
//
// betaTesterGroupConflictAlreadySatisfied applies the same rule to the sibling
// POST /v1/betaTesters/{id}/relationships/betaGroups write used by the CSV
// importer.
func isHTTPConflict(err error) bool {
	var apiErr *asc.APIError
	return errors.As(err, &apiErr) && apiErr != nil && apiErr.StatusCode == http.StatusConflict
}

// readBackBetaGroupMembership resolves which requested testers are already in
// the group. It asks App Store Connect for the intersection directly
// (GET /v1/betaTesters?filter[betaGroups]=GROUP&filter[id]=TESTER,...), so the
// cost starts at one request per betaTesterResolveChunkSize requested testers,
// plus any pagination those filtered responses require, regardless of how many
// testers the group holds. It is only called after a failed add, so these reads
// never touch the success path.
func readBackBetaGroupMembership(
	ctx context.Context,
	client *asc.Client,
	groupID string,
	testerIDs []string,
) (betaGroupMembershipReadBack, error) {
	result := betaGroupMembershipReadBack{groupID: strings.TrimSpace(groupID)}
	if client == nil {
		return betaGroupMembershipReadBack{}, fmt.Errorf("client is required")
	}
	if result.groupID == "" {
		return betaGroupMembershipReadBack{}, fmt.Errorf("groupID is required")
	}

	requested := make([]string, 0, len(testerIDs))
	for _, testerID := range testerIDs {
		if trimmed := strings.TrimSpace(testerID); trimmed != "" {
			requested = append(requested, trimmed)
		}
	}
	if len(requested) == 0 {
		return result, nil
	}

	members := make(map[string]struct{}, len(requested))
	for chunk := range slices.Chunk(requested, betaTesterResolveChunkSize) {
		found, err := betaGroupMembersMatching(ctx, client, result.groupID, chunk)
		if err != nil {
			return betaGroupMembershipReadBack{}, err
		}
		for testerID := range found {
			members[testerID] = struct{}{}
		}
	}

	for _, testerID := range requested {
		if _, ok := members[testerID]; ok {
			result.present = append(result.present, testerID)
			continue
		}
		result.missing = append(result.missing, testerID)
	}
	return result, nil
}

// betaGroupMembersMatching returns the subset of testerIDs that App Store
// Connect reports as members of the group.
func betaGroupMembersMatching(
	ctx context.Context,
	client *asc.Client,
	groupID string,
	testerIDs []string,
) (map[string]struct{}, error) {
	options := []asc.BetaTestersOption{
		asc.WithBetaTestersGroupIDs([]string{groupID}),
		asc.WithBetaTestersIDs(testerIDs),
		asc.Limit[asc.BetaTestersOption](200),
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	first, err := client.GetBetaTesters(requestCtx, "", options...)
	cancel()
	if err != nil {
		return nil, err
	}

	all, err := asc.PaginateAll(ctx, first, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		requestCtx, cancel := shared.ContextWithTimeout(ctx)
		defer cancel()
		return client.GetBetaTesters(requestCtx, "", asc.NextURL[asc.BetaTestersOption](nextURL))
	})
	if err != nil {
		return nil, err
	}
	resp, ok := all.(*asc.BetaTestersResponse)
	if !ok || resp == nil {
		return nil, fmt.Errorf("unexpected beta testers response type")
	}

	members := make(map[string]struct{}, len(resp.Data))
	for _, tester := range resp.Data {
		if testerID := strings.TrimSpace(tester.ID); testerID != "" {
			members[testerID] = struct{}{}
		}
	}
	return members, nil
}
