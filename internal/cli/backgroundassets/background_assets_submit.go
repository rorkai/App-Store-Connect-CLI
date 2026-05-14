package backgroundassets

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const backgroundAssetVersionStateComplete = "COMPLETE"

func BackgroundAssetsSubmitCommand() *ffcli.Command {
	fs := flag.NewFlagSet("submit", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	platform := fs.String("platform", "IOS", "Platform: IOS, MAC_OS, TV_OS, VISION_OS")
	all := fs.Bool("all", false, "Submit the latest COMPLETE version of every non-archived asset pack for the app")
	assetPackIdentifiers := fs.String("asset-pack-identifier", "", "Comma-separated asset pack identifiers to submit")
	backgroundAssetIDs := fs.String("background-asset-id", "", "Comma-separated background asset IDs to submit")
	versionIDs := fs.String("version-id", "", "Comma-separated background asset version IDs to submit (skips lookup)")
	submissionID := fs.String("review-submission-id", "", "Attach items to this existing review submission instead of creating a new one")
	confirm := fs.Bool("confirm", false, "Confirm submission (required unless --dry-run)")
	dryRun := fs.Bool("dry-run", false, "Preview the submission flow without mutating")
	noSubmit := fs.Bool("no-submit", false, "Create the submission and attach items but do not submit; useful when chaining additional items")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "submit",
		ShortUsage: "asc background-assets submit --app \"APP_ID\" --all --confirm",
		ShortHelp:  "Submit background asset packs for App Store review.",
		LongHelp: `Submit background asset packs for App Store review.

This is a wrapper around the generic review submission flow:
  - asc review submissions-create
  - asc review items-add (for each background asset version)
  - asc review submissions-submit

Pack selection is mutually exclusive: --all, --asset-pack-identifier,
--background-asset-id, or --version-id.

For --all, --asset-pack-identifier, and --background-asset-id the newest
version in state COMPLETE is selected per pack; an error is returned if a
pack has no COMPLETE version.

Examples:
  asc background-assets submit --app "APP_ID" --all --confirm
  asc background-assets submit --app "APP_ID" --asset-pack-identifier "com.example.stamps.us,com.example.stamps.de" --confirm
  asc background-assets submit --app "APP_ID" --version-id "VID_A,VID_B" --review-submission-id "SUB" --confirm
  asc background-assets submit --app "APP_ID" --all --dry-run`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}
			normalizedPlatform, err := shared.NormalizeAppStoreVersionPlatform(*platform)
			if err != nil {
				return shared.UsageError(err.Error())
			}

			packIDs := shared.SplitCSV(*assetPackIdentifiers)
			assetIDs := shared.SplitCSV(*backgroundAssetIDs)
			explicitVersionIDs := shared.SplitCSV(*versionIDs)

			selectionCount := 0
			if *all {
				selectionCount++
			}
			if len(packIDs) > 0 {
				selectionCount++
			}
			if len(assetIDs) > 0 {
				selectionCount++
			}
			if len(explicitVersionIDs) > 0 {
				selectionCount++
			}
			if selectionCount == 0 {
				return shared.UsageError("one of --all, --asset-pack-identifier, --background-asset-id, or --version-id is required")
			}
			if selectionCount > 1 {
				return shared.UsageError("--all, --asset-pack-identifier, --background-asset-id, and --version-id are mutually exclusive")
			}
			if !*confirm && !*dryRun {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required unless --dry-run is set")
				return flag.ErrHelp
			}
			if *noSubmit && *dryRun {
				return shared.UsageError("--no-submit and --dry-run are mutually exclusive")
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			needsClientForResolve := len(explicitVersionIDs) == 0
			var client backgroundAssetSubmitClient
			if needsClientForResolve || !*dryRun {
				realClient, err := shared.GetASCClient()
				if err != nil {
					return fmt.Errorf("background-assets submit: %w", err)
				}
				client = realClient
			}

			items, err := resolveBackgroundAssetSubmitItems(requestCtx, backgroundAssetSubmitResolver{client: client}, backgroundAssetSubmitSelection{
				appID:                resolvedAppID,
				platform:             normalizedPlatform,
				all:                  *all,
				assetPackIdentifiers: packIDs,
				backgroundAssetIDs:   assetIDs,
				explicitVersionIDs:   explicitVersionIDs,
			})
			if err != nil {
				return fmt.Errorf("background-assets submit: %w", err)
			}
			if len(items) == 0 {
				return shared.UsageError("no matching background asset versions found to submit")
			}

			result := backgroundAssetsSubmitResult{
				AppID:    resolvedAppID,
				Platform: normalizedPlatform,
				Items:    items,
				DryRun:   *dryRun,
				NoSubmit: *noSubmit,
			}

			if *dryRun {
				result.Messages = append(result.Messages, fmt.Sprintf("dry-run: would submit %d background asset version(s) for review", len(items)))
				return shared.PrintOutput(result, *output.Output, *output.Pretty)
			}

			currentSubmissionID := strings.TrimSpace(*submissionID)
			createdHere := false
			if currentSubmissionID == "" {
				createResp, err := client.CreateReviewSubmission(requestCtx, resolvedAppID, asc.Platform(normalizedPlatform))
				if err != nil {
					return fmt.Errorf("background-assets submit: create review submission: %w", err)
				}
				currentSubmissionID = createResp.Data.ID
				createdHere = true
			}
			result.SubmissionID = currentSubmissionID

			alreadyAttached := map[string]struct{}{}
			if !createdHere {
				existing, err := fetchAlreadyAttachedBackgroundAssetVersions(requestCtx, client, currentSubmissionID)
				if err != nil {
					return fmt.Errorf("background-assets submit: inspect existing items on submission %q: %w", currentSubmissionID, err)
				}
				alreadyAttached = existing
			}

			for i, item := range items {
				if _, skip := alreadyAttached[item.BackgroundAssetVersionID]; skip {
					result.SkippedAlreadyAttached = append(result.SkippedAlreadyAttached, item)
					continue
				}
				if _, err := client.CreateReviewSubmissionItem(requestCtx, currentSubmissionID, asc.ReviewSubmissionItemTypeBackgroundAssetVersion, item.BackgroundAssetVersionID); err != nil {
					if createdHere {
						if _, cancelErr := client.CancelReviewSubmission(requestCtx, currentSubmissionID); cancelErr == nil {
							return fmt.Errorf("background-assets submit: attach version %q (index %d, %d already attached) to submission %q failed; rolled back the submission: %w", item.BackgroundAssetVersionID, i, result.AttachedItems, currentSubmissionID, err)
						} else {
							return fmt.Errorf("background-assets submit: attach version %q (index %d, %d already attached) to submission %q failed; rollback also failed (submission %q is leaked with %d partial item(s)): attach err=%v; rollback err=%v", item.BackgroundAssetVersionID, i, result.AttachedItems, currentSubmissionID, currentSubmissionID, result.AttachedItems, err, cancelErr)
						}
					}
					return fmt.Errorf("background-assets submit: attach version %q (index %d) to submission %q: %w", item.BackgroundAssetVersionID, i, currentSubmissionID, err)
				}
				result.AttachedItems++
			}

			if *noSubmit {
				result.Messages = append(result.Messages, fmt.Sprintf("--no-submit set; submission %s left open with %d item(s) attached", currentSubmissionID, result.AttachedItems))
				return shared.PrintOutput(result, *output.Output, *output.Pretty)
			}

			submitResp, err := client.SubmitReviewSubmission(requestCtx, currentSubmissionID)
			if err != nil {
				return fmt.Errorf("background-assets submit: submit review submission %q: %w", currentSubmissionID, err)
			}
			if submitResp != nil {
				result.SubmittedDate = submitResp.Data.Attributes.SubmittedDate
				if submitResp.Data.Attributes.SubmissionState != "" {
					result.SubmissionState = string(submitResp.Data.Attributes.SubmissionState)
				}
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

type backgroundAssetsSubmitResult struct {
	AppID                  string                             `json:"appId"`
	Platform               string                             `json:"platform"`
	SubmissionID           string                             `json:"submissionId,omitempty"`
	SubmissionState        string                             `json:"submissionState,omitempty"`
	SubmittedDate          string                             `json:"submittedDate,omitempty"`
	AttachedItems          int                                `json:"attachedItems"`
	DryRun                 bool                               `json:"dryRun,omitempty"`
	NoSubmit               bool                               `json:"noSubmit,omitempty"`
	Items                  []backgroundAssetsSubmitResultItem `json:"items"`
	SkippedAlreadyAttached []backgroundAssetsSubmitResultItem `json:"skippedAlreadyAttached,omitempty"`
	Messages               []string                           `json:"messages,omitempty"`
}

type backgroundAssetsSubmitResultItem struct {
	BackgroundAssetID        string `json:"backgroundAssetId,omitempty"`
	AssetPackIdentifier      string `json:"assetPackIdentifier,omitempty"`
	BackgroundAssetVersionID string `json:"backgroundAssetVersionId"`
	VersionNumber            string `json:"versionNumber,omitempty"`
}

type backgroundAssetSubmitSelection struct {
	appID                string
	platform             string
	all                  bool
	assetPackIdentifiers []string
	backgroundAssetIDs   []string
	explicitVersionIDs   []string
}

type backgroundAssetSubmitClient interface {
	GetBackgroundAssets(ctx context.Context, appID string, opts ...asc.BackgroundAssetsOption) (*asc.BackgroundAssetsResponse, error)
	GetBackgroundAssetVersions(ctx context.Context, backgroundAssetID string, opts ...asc.BackgroundAssetVersionsOption) (*asc.BackgroundAssetVersionsResponse, error)
	GetReviewSubmissionItems(ctx context.Context, submissionID string, opts ...asc.ReviewSubmissionItemsOption) (*asc.ReviewSubmissionItemsResponse, error)
	CreateReviewSubmission(ctx context.Context, appID string, platform asc.Platform) (*asc.ReviewSubmissionResponse, error)
	CreateReviewSubmissionItem(ctx context.Context, submissionID string, itemType asc.ReviewSubmissionItemType, itemID string) (*asc.ReviewSubmissionItemResponse, error)
	SubmitReviewSubmission(ctx context.Context, submissionID string) (*asc.ReviewSubmissionResponse, error)
	CancelReviewSubmission(ctx context.Context, submissionID string) (*asc.ReviewSubmissionResponse, error)
}

type backgroundAssetSubmitResolver struct {
	client backgroundAssetSubmitClient
}

func resolveBackgroundAssetSubmitItems(ctx context.Context, r backgroundAssetSubmitResolver, sel backgroundAssetSubmitSelection) ([]backgroundAssetsSubmitResultItem, error) {
	if len(sel.explicitVersionIDs) > 0 {
		items := make([]backgroundAssetsSubmitResultItem, 0, len(sel.explicitVersionIDs))
		for _, vid := range sel.explicitVersionIDs {
			vid = strings.TrimSpace(vid)
			if vid == "" {
				continue
			}
			items = append(items, backgroundAssetsSubmitResultItem{BackgroundAssetVersionID: vid})
		}
		return items, nil
	}

	assets, err := r.listAssets(ctx, sel)
	if err != nil {
		return nil, err
	}
	if len(assets) == 0 {
		return nil, nil
	}

	items := make([]backgroundAssetsSubmitResultItem, 0, len(assets))
	for _, asset := range assets {
		if asset.Attributes.Archived {
			continue
		}
		version, err := r.latestCompleteVersion(ctx, asset.ID, sel.platform)
		if err != nil {
			return nil, fmt.Errorf("resolve newest COMPLETE version for asset %q (%s): %w", asset.Attributes.AssetPackIdentifier, asset.ID, err)
		}
		if version == nil {
			return nil, fmt.Errorf("asset %q (%s) has no version in state %s for platform %s; upload files and wait for processing before submitting", asset.Attributes.AssetPackIdentifier, asset.ID, backgroundAssetVersionStateComplete, sel.platform)
		}
		items = append(items, backgroundAssetsSubmitResultItem{
			BackgroundAssetID:        asset.ID,
			AssetPackIdentifier:      asset.Attributes.AssetPackIdentifier,
			BackgroundAssetVersionID: version.ID,
			VersionNumber:            version.Attributes.Version,
		})
	}
	return items, nil
}

func (r backgroundAssetSubmitResolver) listAssets(ctx context.Context, sel backgroundAssetSubmitSelection) ([]ascBackgroundAssetItem, error) {
	if len(sel.backgroundAssetIDs) > 0 {
		assets := make([]ascBackgroundAssetItem, 0, len(sel.backgroundAssetIDs))
		filter := make(map[string]struct{}, len(sel.backgroundAssetIDs))
		for _, id := range sel.backgroundAssetIDs {
			filter[strings.TrimSpace(id)] = struct{}{}
		}
		opts := []asc.BackgroundAssetsOption{
			asc.WithBackgroundAssetsLimit(backgroundAssetsMaxLimit),
			asc.WithBackgroundAssetsFilterArchived([]string{"false"}),
		}
		all, err := fetchAllBackgroundAssets(ctx, r.client, sel.appID, opts...)
		if err != nil {
			return nil, err
		}
		for _, asset := range all {
			if _, ok := filter[asset.ID]; ok {
				assets = append(assets, asset)
			}
		}
		if len(assets) != len(filter) {
			missing := make([]string, 0, len(filter)-len(assets))
			seen := make(map[string]struct{}, len(assets))
			for _, a := range assets {
				seen[a.ID] = struct{}{}
			}
			for id := range filter {
				if _, ok := seen[id]; !ok {
					missing = append(missing, id)
				}
			}
			sort.Strings(missing)
			return nil, fmt.Errorf("background asset id(s) not found for app %q: %s", sel.appID, strings.Join(missing, ", "))
		}
		return assets, nil
	}

	opts := []asc.BackgroundAssetsOption{
		asc.WithBackgroundAssetsLimit(backgroundAssetsMaxLimit),
		asc.WithBackgroundAssetsFilterArchived([]string{"false"}),
	}
	if len(sel.assetPackIdentifiers) > 0 {
		opts = append(opts, asc.WithBackgroundAssetsFilterAssetPackIdentifier(sel.assetPackIdentifiers))
	}
	assets, err := fetchAllBackgroundAssets(ctx, r.client, sel.appID, opts...)
	if err != nil {
		return nil, err
	}
	if len(sel.assetPackIdentifiers) > 0 {
		wanted := make(map[string]struct{}, len(sel.assetPackIdentifiers))
		for _, id := range sel.assetPackIdentifiers {
			wanted[strings.TrimSpace(id)] = struct{}{}
		}
		seen := make(map[string]struct{}, len(assets))
		for _, a := range assets {
			seen[a.Attributes.AssetPackIdentifier] = struct{}{}
		}
		var missing []string
		for id := range wanted {
			if _, ok := seen[id]; !ok {
				missing = append(missing, id)
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			return nil, fmt.Errorf("asset pack identifier(s) not found for app %q: %s", sel.appID, strings.Join(missing, ", "))
		}
	}
	return assets, nil
}

func (r backgroundAssetSubmitResolver) latestCompleteVersion(ctx context.Context, backgroundAssetID, platform string) (*ascBackgroundAssetVersionItem, error) {
	versions, err := fetchAllBackgroundAssetVersions(ctx, r.client, backgroundAssetID)
	if err != nil {
		return nil, err
	}
	complete := versions[:0]
	for _, v := range versions {
		if !strings.EqualFold(v.Attributes.State, backgroundAssetVersionStateComplete) {
			continue
		}
		if !versionSupportsPlatform(v, platform) {
			continue
		}
		complete = append(complete, v)
	}
	if len(complete) == 0 {
		return nil, nil
	}
	sort.Slice(complete, func(i, j int) bool {
		if complete[i].Attributes.CreatedDate != complete[j].Attributes.CreatedDate {
			return complete[i].Attributes.CreatedDate > complete[j].Attributes.CreatedDate
		}
		return complete[i].Attributes.Version > complete[j].Attributes.Version
	})
	chosen := complete[0]
	return &chosen, nil
}

func versionSupportsPlatform(v ascBackgroundAssetVersionItem, platform string) bool {
	if strings.TrimSpace(platform) == "" || len(v.Attributes.Platforms) == 0 {
		return true
	}
	for _, p := range v.Attributes.Platforms {
		if strings.EqualFold(string(p), platform) {
			return true
		}
	}
	return false
}

type ascBackgroundAssetItem = asc.Resource[asc.BackgroundAssetAttributes]
type ascBackgroundAssetVersionItem = asc.Resource[asc.BackgroundAssetVersionAttributes]

func fetchAllBackgroundAssets(ctx context.Context, client backgroundAssetSubmitClient, appID string, opts ...asc.BackgroundAssetsOption) ([]ascBackgroundAssetItem, error) {
	first, err := client.GetBackgroundAssets(ctx, appID, opts...)
	if err != nil {
		return nil, fmt.Errorf("list background assets: %w", err)
	}
	resp, err := asc.PaginateAll(ctx, first, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetBackgroundAssets(ctx, appID, asc.WithBackgroundAssetsNextURL(nextURL))
	})
	if err != nil {
		return nil, fmt.Errorf("paginate background assets: %w", err)
	}
	if aggregate, ok := resp.(*asc.BackgroundAssetsResponse); ok {
		return aggregate.Data, nil
	}
	return first.Data, nil
}

func fetchAlreadyAttachedBackgroundAssetVersions(ctx context.Context, client backgroundAssetSubmitClient, submissionID string) (map[string]struct{}, error) {
	attached := map[string]struct{}{}
	opts := []asc.ReviewSubmissionItemsOption{
		asc.WithReviewSubmissionItemsLimit(backgroundAssetsMaxLimit),
		asc.WithReviewSubmissionItemsInclude([]string{"backgroundAssetVersion"}),
	}
	first, err := client.GetReviewSubmissionItems(ctx, submissionID, opts...)
	if err != nil {
		return nil, fmt.Errorf("list submission items: %w", err)
	}
	resp, err := asc.PaginateAll(ctx, first, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetReviewSubmissionItems(ctx, submissionID, asc.WithReviewSubmissionItemsNextURL(nextURL), asc.WithReviewSubmissionItemsInclude([]string{"backgroundAssetVersion"}))
	})
	if err != nil {
		return nil, fmt.Errorf("paginate submission items: %w", err)
	}
	aggregate, ok := resp.(*asc.ReviewSubmissionItemsResponse)
	if !ok {
		aggregate = first
	}
	for _, item := range aggregate.Data {
		if item.Relationships == nil || item.Relationships.BackgroundAssetVersion == nil {
			continue
		}
		if strings.EqualFold(string(item.Attributes.State), "REMOVED") {
			continue
		}
		bgVer := item.Relationships.BackgroundAssetVersion.Data.ID
		if bgVer != "" {
			attached[bgVer] = struct{}{}
		}
	}
	return attached, nil
}

func fetchAllBackgroundAssetVersions(ctx context.Context, client backgroundAssetSubmitClient, backgroundAssetID string) ([]ascBackgroundAssetVersionItem, error) {
	opts := []asc.BackgroundAssetVersionsOption{asc.WithBackgroundAssetVersionsLimit(backgroundAssetsMaxLimit)}
	first, err := client.GetBackgroundAssetVersions(ctx, backgroundAssetID, opts...)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	resp, err := asc.PaginateAll(ctx, first, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetBackgroundAssetVersions(ctx, backgroundAssetID, asc.WithBackgroundAssetVersionsNextURL(nextURL))
	})
	if err != nil {
		return nil, fmt.Errorf("paginate versions: %w", err)
	}
	if aggregate, ok := resp.(*asc.BackgroundAssetVersionsResponse); ok {
		return aggregate.Data, nil
	}
	return first.Data, nil
}
