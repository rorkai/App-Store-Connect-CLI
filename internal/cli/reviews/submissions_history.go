package reviews

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// SubmissionHistoryEntry is the assembled result for one submission.
type SubmissionHistoryEntry struct {
	SubmissionID  string                  `json:"submissionId"`
	VersionString string                  `json:"versionString"`
	Platform      string                  `json:"platform"`
	State         string                  `json:"state"`
	SubmittedDate string                  `json:"submittedDate"`
	Outcome       string                  `json:"outcome"`
	Items         []SubmissionHistoryItem `json:"items"`
}

// SubmissionHistoryItem is a summary of one item in a submission.
type SubmissionHistoryItem struct {
	ID         string `json:"id"`
	State      string `json:"state"`
	Type       string `json:"type"`
	ResourceID string `json:"resourceId"`
}

type submissionVersionContext struct {
	VersionString string
	Platform      string
}

// ReviewHistoryCommand returns the history subcommand.
func ReviewHistoryCommand() *ffcli.Command {
	fs := flag.NewFlagSet("history", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	platform := fs.String("platform", "", "Filter by platform: IOS, MAC_OS, TV_OS, VISION_OS (comma-separated)")
	state := fs.String("state", "", "Filter by state (comma-separated)")
	version := fs.String("version", "", "Filter by version string (e.g. 1.2.0)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "history",
		ShortUsage: "asc review history [flags]",
		ShortHelp:  "Show enriched review submission history for an app.",
		LongHelp: `Show enriched review submission history for an app.

Each entry includes the submission state, platform, version string, submitted
date, and a derived outcome (approved, rejected, or the raw state).

Examples:
  asc review history --app "123456789"
  asc review history --app "123456789" --platform IOS --state COMPLETE
  asc review history --app "123456789" --version "1.2.0"
  asc review history --app "123456789" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return shared.UsageError("--limit must be between 1 and 200")
			}

			platforms, err := shared.NormalizeAppStoreVersionPlatforms(shared.SplitCSVUpper(*platform))
			if err != nil {
				return shared.UsageError(err.Error())
			}
			states, err := shared.NormalizeReviewSubmissionStates(shared.SplitCSVUpper(*state))
			if err != nil {
				return shared.UsageError(err.Error())
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("review history: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.ReviewSubmissionsOption{
				asc.WithReviewSubmissionsLimit(*limit),
				asc.WithReviewSubmissionsPlatforms(platforms),
				asc.WithReviewSubmissionsStates(states),
				asc.WithReviewSubmissionsInclude([]string{"appStoreVersionForReview"}),
			}
			if *paginate {
				opts = append(opts, asc.WithReviewSubmissionsLimit(200))
			}

			submissions, versionContexts, err := fetchReviewSubmissions(requestCtx, client, resolvedAppID, opts, *paginate)
			if err != nil {
				return fmt.Errorf("review history: %w", err)
			}

			entries, err := enrichSubmissions(requestCtx, client, submissions, versionContexts, strings.TrimSpace(*version))
			if err != nil {
				return fmt.Errorf("review history: %w", err)
			}

			tableFunc := func() error { return printHistoryTable(entries) }
			markdownFunc := func() error { return printHistoryMarkdown(entries) }
			return shared.PrintOutputWithRenderers(entries, *output.Output, *output.Pretty, tableFunc, markdownFunc)
		},
	}
}

func fetchReviewSubmissions(ctx context.Context, client *asc.Client, appID string, opts []asc.ReviewSubmissionsOption, paginate bool) ([]asc.ReviewSubmissionResource, map[string]submissionVersionContext, error) {
	submissions := make([]asc.ReviewSubmissionResource, 0)
	versionContexts := make(map[string]submissionVersionContext)

	collectPage := func(resp *asc.ReviewSubmissionsResponse) error {
		if resp == nil {
			return nil
		}
		submissions = append(submissions, resp.Data...)
		pageContexts, err := submissionVersionContexts(resp)
		if err != nil {
			return err
		}
		for submissionID, ctx := range pageContexts {
			versionContexts[submissionID] = ctx
		}
		return nil
	}

	fetchAll := func() error {
		resp, err := client.GetReviewSubmissions(ctx, appID, opts...)
		if err != nil {
			return err
		}
		if err := collectPage(resp); err != nil {
			return err
		}
		if !paginate {
			return nil
		}

		current := resp
		seenNext := make(map[string]struct{})
		page := 1
		for current != nil && strings.TrimSpace(current.Links.Next) != "" {
			nextURL := strings.TrimSpace(current.Links.Next)
			if _, ok := seenNext[nextURL]; ok {
				return fmt.Errorf("page %d: %w", page+1, asc.ErrRepeatedPaginationURL)
			}
			seenNext[nextURL] = struct{}{}

			nextResp, err := client.GetReviewSubmissions(ctx, appID, asc.WithReviewSubmissionsNextURL(nextURL))
			if err != nil {
				return fmt.Errorf("page %d: %w", page+1, err)
			}
			if err := collectPage(nextResp); err != nil {
				return fmt.Errorf("page %d: %w", page+1, err)
			}

			current = nextResp
			page++
		}
		return nil
	}

	if paginate {
		if err := shared.WithSpinner("", fetchAll); err != nil {
			return nil, nil, err
		}
	} else if err := fetchAll(); err != nil {
		return nil, nil, err
	}

	return submissions, versionContexts, nil
}

func submissionVersionContexts(resp *asc.ReviewSubmissionsResponse) (map[string]submissionVersionContext, error) {
	contexts := make(map[string]submissionVersionContext)
	if resp == nil || len(resp.Data) == 0 {
		return contexts, nil
	}

	versionByID := make(map[string]submissionVersionContext)
	if len(resp.Included) != 0 {
		var included []asc.Resource[asc.AppStoreVersionAttributes]
		if err := json.Unmarshal(resp.Included, &included); err != nil {
			return nil, fmt.Errorf("failed to parse included review submission versions: %w", err)
		}
		for _, version := range included {
			if version.Type != asc.ResourceTypeAppStoreVersions {
				continue
			}
			versionByID[version.ID] = submissionVersionContext{
				VersionString: version.Attributes.VersionString,
				Platform:      string(version.Attributes.Platform),
			}
		}
	}

	for _, sub := range resp.Data {
		if sub.Relationships == nil || sub.Relationships.AppStoreVersionForReview == nil {
			continue
		}
		versionID := strings.TrimSpace(sub.Relationships.AppStoreVersionForReview.Data.ID)
		if versionID == "" {
			continue
		}
		if ctx, ok := versionByID[versionID]; ok {
			contexts[sub.ID] = ctx
		}
	}

	return contexts, nil
}

// enrichSubmissions takes already-fetched submissions and enriches each with
// item states and version strings. Applies client-side version filtering and
// sorts by submittedDate descending.
func enrichSubmissions(ctx context.Context, client *asc.Client, submissions []asc.ReviewSubmissionResource, versionContexts map[string]submissionVersionContext, versionFilter string) ([]SubmissionHistoryEntry, error) {
	entries := make([]SubmissionHistoryEntry, 0, len(submissions))
	for _, sub := range submissions {
		// Skip pre-submission drafts (no submittedDate)
		if strings.TrimSpace(sub.Attributes.SubmittedDate) == "" {
			continue
		}

		entry := SubmissionHistoryEntry{
			SubmissionID:  sub.ID,
			Platform:      string(sub.Attributes.Platform),
			State:         string(sub.Attributes.SubmissionState),
			SubmittedDate: sub.Attributes.SubmittedDate,
		}
		if versionCtx, ok := versionContexts[sub.ID]; ok {
			entry.VersionString = strings.TrimSpace(versionCtx.VersionString)
			if entry.Platform == "" {
				entry.Platform = strings.TrimSpace(versionCtx.Platform)
			}
		}

		items, err := fetchAllSubmissionItems(ctx, client, sub.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch items for submission %s: %w", sub.ID, err)
		}

		itemStates := make([]string, 0, len(items))
		entry.Items = make([]SubmissionHistoryItem, 0, len(items))
		for _, item := range items {
			histItem := SubmissionHistoryItem{
				ID:    item.ID,
				State: item.Attributes.State,
			}
			populateSubmissionHistoryItem(&histItem, item)

			itemStates = append(itemStates, item.Attributes.State)
			entry.Items = append(entry.Items, histItem)
		}

		entry.Outcome = deriveOutcome(entry.State, itemStates)

		if entry.VersionString == "" {
			entry.VersionString = "unknown"
		}

		entries = append(entries, entry)
	}

	// Client-side version filter
	if versionFilter != "" {
		filtered := make([]SubmissionHistoryEntry, 0, len(entries))
		for _, e := range entries {
			if e.VersionString == versionFilter {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	// Sort by submittedDate descending
	sort.Slice(entries, func(i, j int) bool {
		cmp := compareRFC3339DateStrings(entries[i].SubmittedDate, entries[j].SubmittedDate)
		if cmp == 0 {
			return entries[i].SubmissionID > entries[j].SubmissionID
		}
		return cmp > 0
	})

	return entries, nil
}

func fetchAllSubmissionItems(ctx context.Context, client *asc.Client, submissionID string) ([]asc.ReviewSubmissionItemResource, error) {
	firstPage, err := client.GetReviewSubmissionItems(
		ctx,
		submissionID,
		asc.WithReviewSubmissionItemsLimit(200),
		asc.WithReviewSubmissionItemsFields(reviewSubmissionItemHistoryFields()),
		asc.WithReviewSubmissionItemsInclude(reviewSubmissionItemHistoryIncludes()),
	)
	if err != nil {
		return nil, err
	}
	if firstPage == nil {
		return []asc.ReviewSubmissionItemResource{}, nil
	}

	resp, err := asc.PaginateAll(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetReviewSubmissionItems(ctx, submissionID, asc.WithReviewSubmissionItemsNextURL(nextURL))
	})
	if err != nil {
		return nil, err
	}

	aggResp, ok := resp.(*asc.ReviewSubmissionItemsResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected review submission items pagination response type %T", resp)
	}
	if aggResp == nil || aggResp.Data == nil {
		return []asc.ReviewSubmissionItemResource{}, nil
	}

	return aggResp.Data, nil
}

func reviewSubmissionItemHistoryFields() []string {
	return []string{
		"state",
		"appStoreVersion",
		"appCustomProductPageVersion",
		"appStoreVersionExperiment",
		"appStoreVersionExperimentV2",
		"appEvent",
		"backgroundAssetVersion",
		"gameCenterAchievementVersion",
		"gameCenterActivityVersion",
		"gameCenterChallengeVersion",
		"gameCenterLeaderboardSetVersion",
		"gameCenterLeaderboardVersion",
	}
}

func reviewSubmissionItemHistoryIncludes() []string {
	return []string{
		"appStoreVersion",
		"appCustomProductPageVersion",
		"appStoreVersionExperiment",
		"appEvent",
		"backgroundAssetVersion",
		"gameCenterAchievementVersion",
		"gameCenterActivityVersion",
		"gameCenterChallengeVersion",
		"gameCenterLeaderboardSetVersion",
		"gameCenterLeaderboardVersion",
	}
}

func populateSubmissionHistoryItem(histItem *SubmissionHistoryItem, item asc.ReviewSubmissionItemResource) {
	if histItem == nil || item.Relationships == nil {
		return
	}

	switch {
	case item.Relationships.AppStoreVersion != nil:
		histItem.Type = "appStoreVersion"
		histItem.ResourceID = item.Relationships.AppStoreVersion.Data.ID
	case item.Relationships.AppCustomProductPageVersion != nil:
		histItem.Type = "appCustomProductPageVersion"
		histItem.ResourceID = item.Relationships.AppCustomProductPageVersion.Data.ID
	case item.Relationships.AppCustomProductPage != nil:
		histItem.Type = "appCustomProductPage"
		histItem.ResourceID = item.Relationships.AppCustomProductPage.Data.ID
	case item.Relationships.AppStoreVersionExperimentV2 != nil:
		histItem.Type = "appStoreVersionExperimentV2"
		histItem.ResourceID = item.Relationships.AppStoreVersionExperimentV2.Data.ID
	case item.Relationships.AppEvent != nil:
		histItem.Type = "appEvent"
		histItem.ResourceID = item.Relationships.AppEvent.Data.ID
	case item.Relationships.BackgroundAssetVersion != nil:
		histItem.Type = "backgroundAssetVersion"
		histItem.ResourceID = item.Relationships.BackgroundAssetVersion.Data.ID
	case item.Relationships.GameCenterAchievementVersion != nil:
		histItem.Type = "gameCenterAchievementVersion"
		histItem.ResourceID = item.Relationships.GameCenterAchievementVersion.Data.ID
	case item.Relationships.GameCenterActivityVersion != nil:
		histItem.Type = "gameCenterActivityVersion"
		histItem.ResourceID = item.Relationships.GameCenterActivityVersion.Data.ID
	case item.Relationships.GameCenterChallengeVersion != nil:
		histItem.Type = "gameCenterChallengeVersion"
		histItem.ResourceID = item.Relationships.GameCenterChallengeVersion.Data.ID
	case item.Relationships.GameCenterLeaderboardSetVersion != nil:
		histItem.Type = "gameCenterLeaderboardSetVersion"
		histItem.ResourceID = item.Relationships.GameCenterLeaderboardSetVersion.Data.ID
	case item.Relationships.GameCenterLeaderboardVersion != nil:
		histItem.Type = "gameCenterLeaderboardVersion"
		histItem.ResourceID = item.Relationships.GameCenterLeaderboardVersion.Data.ID
	case item.Relationships.AppStoreVersionExperiment != nil:
		histItem.Type = "appStoreVersionExperiment"
		histItem.ResourceID = item.Relationships.AppStoreVersionExperiment.Data.ID
	case item.Relationships.AppStoreVersionExperimentTreatment != nil:
		histItem.Type = "appStoreVersionExperimentTreatment"
		histItem.ResourceID = item.Relationships.AppStoreVersionExperimentTreatment.Data.ID
	}
}

func printHistoryTable(entries []SubmissionHistoryEntry) error {
	headers := []string{"VERSION", "PLATFORM", "STATE", "SUBMITTED", "OUTCOME", "ITEMS"}
	rows := make([][]string, 0, len(entries))
	for _, e := range entries {
		rows = append(rows, []string{
			e.VersionString,
			e.Platform,
			e.State,
			e.SubmittedDate,
			e.Outcome,
			formatItemsSummary(e.Items),
		})
	}
	asc.RenderTable(headers, rows)
	return nil
}

func printHistoryMarkdown(entries []SubmissionHistoryEntry) error {
	headers := []string{"VERSION", "PLATFORM", "STATE", "SUBMITTED", "OUTCOME", "ITEMS"}
	rows := make([][]string, 0, len(entries))
	for _, e := range entries {
		rows = append(rows, []string{
			e.VersionString,
			e.Platform,
			e.State,
			e.SubmittedDate,
			e.Outcome,
			formatItemsSummary(e.Items),
		})
	}
	asc.RenderMarkdown(headers, rows)
	return nil
}

func formatItemsSummary(items []SubmissionHistoryItem) string {
	if len(items) == 0 {
		return "0 items"
	}
	counts := map[string]int{}
	for _, item := range items {
		counts[strings.ToLower(item.State)]++
	}
	var parts []string
	for state, count := range counts {
		parts = append(parts, fmt.Sprintf("%d %s", count, state))
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

// deriveOutcome computes a human-readable outcome from submission and item states.
// Priority order:
// 1. Any item REJECTED → "rejected"
// 2. All items APPROVED → "approved"
// 3. Submission state UNRESOLVED_ISSUES → "rejected"
// 4. Fallback → lowercase submission state
func deriveOutcome(submissionState string, itemStates []string) string {
	hasRejected := false
	allApproved := len(itemStates) > 0

	for _, s := range itemStates {
		if s == "REJECTED" {
			hasRejected = true
		}
		if s != "APPROVED" {
			allApproved = false
		}
	}

	if hasRejected {
		return "rejected"
	}
	if allApproved {
		return "approved"
	}
	if submissionState == "UNRESOLVED_ISSUES" {
		return "rejected"
	}
	return strings.ToLower(submissionState)
}

func compareRFC3339DateStrings(current, best string) int {
	currentTime, currentValid := parseRFC3339Date(current)
	bestTime, bestValid := parseRFC3339Date(best)

	switch {
	case currentValid && bestValid:
		if currentTime.After(bestTime) {
			return 1
		}
		if currentTime.Before(bestTime) {
			return -1
		}
		return 0
	case currentValid:
		return 1
	case bestValid:
		return -1
	default:
		current = strings.TrimSpace(current)
		best = strings.TrimSpace(best)
		if current > best {
			return 1
		}
		if current < best {
			return -1
		}
		return 0
	}
}

func parseRFC3339Date(value string) (time.Time, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, false
	}
	if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return parsed, true
	}
	if parsed, err := time.Parse(time.RFC3339Nano, trimmed); err == nil {
		return parsed, true
	}
	return time.Time{}, false
}
