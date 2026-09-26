package backgroundassets

import (
	"context"
	"errors"
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
	all := fs.Bool("all", false, "Submit the latest COMPLETE version of every non-archived background asset for the app")
	assetPackIdentifiers := fs.String("asset-pack-identifier", "", "Comma-separated asset pack identifiers to submit")
	backgroundAssetIDs := fs.String("background-asset-id", "", "Comma-separated background asset IDs to submit")
	versionIDs := fs.String("version-id", "", "Comma-separated background asset version IDs to submit (skips lookup)")
	submissionID := shared.BindResourceIDFlag(fs, "review-submission-id", "reviewSubmissions", "Attach items to this existing review submission instead of creating a new one")
	confirm := fs.Bool("confirm", false, "Confirm submission (required unless --dry-run)")
	dryRun := fs.Bool("dry-run", false, "Preview the submission flow without mutating")
	noSubmit := fs.Bool("no-submit", false, "Create the submission and attach items but do not submit; useful when chaining additional items")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "submit",
		ShortUsage: "asc background-assets submit --app \"APP_ID\" --background-asset-id \"ASSET_ID\" --confirm",
		ShortHelp:  "Submit background asset versions for App Store review.",
		LongHelp: `Submit one or more background asset versions for App Store review.

This is a wrapper around the generic review submission flow:
  - asc review submissions-create
  - asc review items-add (for each background asset version)
  - asc review submissions-submit

Selection is mutually exclusive: --all, --asset-pack-identifier,
--background-asset-id, or --version-id.

For --all, --asset-pack-identifier, and --background-asset-id the newest
version in state COMPLETE is selected per background asset; an error is
returned if a selected background asset has no COMPLETE version.

Examples:
  asc background-assets submit --app "APP_ID" --background-asset-id "ASSET_ID" --confirm
  asc background-assets submit --app "APP_ID" --asset-pack-identifier "com.example.assetpack" --confirm
  asc background-assets submit --app "APP_ID" --version-id "VERSION_ID" --review-submission-id "SUBMISSION_ID" --confirm
  asc background-assets submit --app "APP_ID" --all --dry-run`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
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
				return shared.MissingRequiredUsageError("--confirm")
			}
			if *noSubmit && *dryRun {
				return shared.UsageError("--no-submit and --dry-run are mutually exclusive")
			}

			needsClientForResolve := len(explicitVersionIDs) == 0
			var client backgroundAssetSubmitClient
			if needsClientForResolve || !*dryRun {
				realClient, err := shared.GetASCClient()
				if err != nil {
					return fmt.Errorf("background-assets submit: %w", err)
				}
				client = realClient
			}

			items, err := resolveBackgroundAssetSubmitItems(ctx, backgroundAssetSubmitResolver{client: client}, backgroundAssetSubmitSelection{
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
				createCtx, createCancel := backgroundAssetSubmitRequestContext(ctx)
				createResp, err := client.CreateReviewSubmission(createCtx, resolvedAppID, asc.Platform(normalizedPlatform))
				createCancel()
				if err != nil {
					var partialErr *asc.ReviewSubmissionCreatePartialError
					if errors.As(err, &partialErr) && partialErr.Response != nil &&
						partialErr.Response.Data.Type == asc.ResourceTypeReviewSubmissions {
						createdSubmissionID := strings.TrimSpace(partialErr.Response.Data.ID)
						if createdSubmissionID != "" {
							return rollbackBackgroundAssetReviewSubmission(ctx, client, createdSubmissionID, "create review submission", err)
						}
					}
					return fmt.Errorf("background-assets submit: create review submission: %w", err)
				}
				createdSubmissionID := ""
				if createResp != nil {
					createdSubmissionID = strings.TrimSpace(createResp.Data.ID)
				}
				if err := validateBackgroundAssetReviewSubmissionCreateReceipt(createResp, resolvedAppID, normalizedPlatform); err != nil {
					if createdSubmissionID != "" {
						return rollbackBackgroundAssetReviewSubmission(ctx, client, createdSubmissionID, "validate create receipt", err)
					}
					return fmt.Errorf("background-assets submit: create review submission receipt: %w", err)
				}
				currentSubmissionID = createdSubmissionID
				createdHere = true
			}
			result.SubmissionID = currentSubmissionID

			if err := validateBackgroundAssetReviewSubmissionBeforeAdd(ctx, client, currentSubmissionID, resolvedAppID, normalizedPlatform); err != nil {
				if createdHere {
					return rollbackBackgroundAssetReviewSubmission(ctx, client, currentSubmissionID, "validate before adding items", err)
				}
				return fmt.Errorf("background-assets submit: validate review submission %q before adding items: %w", currentSubmissionID, err)
			}
			alreadyAttached, err := fetchAlreadyAttachedBackgroundAssetVersions(ctx, client, currentSubmissionID)
			if err != nil {
				if createdHere {
					return rollbackBackgroundAssetReviewSubmission(ctx, client, currentSubmissionID, "inspect existing items", err)
				}
				return fmt.Errorf("background-assets submit: inspect existing items on submission %q: %w", currentSubmissionID, err)
			}

			for i, item := range items {
				if _, skip := alreadyAttached[item.BackgroundAssetVersionID]; skip {
					result.SkippedAlreadyAttached = append(result.SkippedAlreadyAttached, item)
					continue
				}
				attachCtx, attachCancel := backgroundAssetSubmitRequestContext(ctx)
				_, err := client.CreateReviewSubmissionItem(attachCtx, currentSubmissionID, asc.ReviewSubmissionItemTypeBackgroundAssetVersion, item.BackgroundAssetVersionID)
				attachCancel()
				if err != nil {
					if createdHere {
						cancelErr := cancelBackgroundAssetReviewSubmission(ctx, client, currentSubmissionID)
						if cancelErr == nil {
							return fmt.Errorf("background-assets submit: attach version %q (index %d, %d already attached) to submission %q failed; rolled back the submission: %w", item.BackgroundAssetVersionID, i, result.AttachedItems, currentSubmissionID, err)
						} else {
							return fmt.Errorf("background-assets submit: attach version %q (index %d, %d already attached) to submission %q failed; rollback also failed (submission %q is leaked with %d partial item(s)): %w", item.BackgroundAssetVersionID, i, result.AttachedItems, currentSubmissionID, currentSubmissionID, result.AttachedItems, errors.Join(err, cancelErr))
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

			submitCtx, submitCancel := backgroundAssetSubmitRequestContext(ctx)
			submitResp, err := client.SubmitReviewSubmission(submitCtx, currentSubmissionID)
			submitCancel()
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

// backgroundAssetSubmitRequestContext gives each ASC request an independent
// timeout while retaining the caller's cancellation and any earlier deadline.
func backgroundAssetSubmitRequestContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return shared.ContextWithTimeout(ctx)
}

func cancelBackgroundAssetReviewSubmission(ctx context.Context, client backgroundAssetSubmitClient, submissionID string) error {
	rollbackParent := context.WithoutCancel(shared.ContextWithoutTimeout(ctx))
	rollbackCtx, cancel := shared.ContextWithTimeout(rollbackParent)
	defer cancel()
	_, err := client.CancelReviewSubmission(rollbackCtx, submissionID)
	return err
}

func rollbackBackgroundAssetReviewSubmission(ctx context.Context, client backgroundAssetSubmitClient, submissionID, stage string, cause error) error {
	if cancelErr := cancelBackgroundAssetReviewSubmission(ctx, client, submissionID); cancelErr == nil {
		return fmt.Errorf("background-assets submit: %s on newly created submission %q failed; rolled back the submission: %w", stage, submissionID, cause)
	} else {
		return fmt.Errorf("background-assets submit: %s on newly created submission %q failed; rollback also failed (submission %q is leaked): %w", stage, submissionID, submissionID, errors.Join(cause, cancelErr))
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
	GetReviewSubmissionStrict(ctx context.Context, submissionID string, opts ...asc.ReviewSubmissionOption) (*asc.ReviewSubmissionResponse, error)
	GetReviewSubmissionItemsStrict(ctx context.Context, submissionID string, opts ...asc.ReviewSubmissionItemsOption) (*asc.ReviewSubmissionItemsResponse, error)
	CreateReviewSubmission(ctx context.Context, appID string, platform asc.Platform) (*asc.ReviewSubmissionResponse, error)
	CreateReviewSubmissionItem(ctx context.Context, submissionID string, itemType asc.ReviewSubmissionItemType, itemID string) (*asc.ReviewSubmissionItemResponse, error)
	SubmitReviewSubmission(ctx context.Context, submissionID string) (*asc.ReviewSubmissionResponse, error)
	CancelReviewSubmission(ctx context.Context, submissionID string) (*asc.ReviewSubmissionResponse, error)
}

func validateBackgroundAssetReviewSubmissionCreateReceipt(response *asc.ReviewSubmissionResponse, appID, platform string) error {
	if response == nil {
		return fmt.Errorf("created review submission response is required")
	}
	resource := &response.Data
	if resource.Type != asc.ResourceTypeReviewSubmissions {
		return fmt.Errorf("created review submission returned resource type %q, not %q", resource.Type, asc.ResourceTypeReviewSubmissions)
	}
	resourceID := strings.TrimSpace(resource.ID)
	if resourceID == "" {
		return fmt.Errorf("created review submission returned an empty ID")
	}
	if state := resource.Attributes.SubmissionState; state != "" && state != asc.ReviewSubmissionStateReadyForReview {
		return fmt.Errorf("created review submission %s is in state %q, not %q", resourceID, state, asc.ReviewSubmissionStateReadyForReview)
	}
	if actualPlatform := strings.TrimSpace(string(resource.Attributes.Platform)); actualPlatform != "" && !strings.EqualFold(actualPlatform, strings.TrimSpace(platform)) {
		return fmt.Errorf("created review submission %s is for platform %q, not %q", resourceID, actualPlatform, strings.TrimSpace(platform))
	}
	if resource.Relationships == nil || resource.Relationships.App == nil {
		return nil
	}
	appRelationship := resource.Relationships.App.Data
	if appRelationship.Type != asc.ResourceTypeApps {
		return fmt.Errorf("created review submission %s returned app relationship type %q, not %q", resourceID, appRelationship.Type, asc.ResourceTypeApps)
	}
	actualAppID := strings.TrimSpace(appRelationship.ID)
	if actualAppID == "" {
		return fmt.Errorf("created review submission %s returned an empty app relationship ID", resourceID)
	}
	if actualAppID != strings.TrimSpace(appID) {
		return fmt.Errorf("created review submission %s belongs to app %q, not %q", resourceID, actualAppID, strings.TrimSpace(appID))
	}
	return nil
}

func validateBackgroundAssetReviewSubmissionBeforeAdd(ctx context.Context, client backgroundAssetSubmitClient, submissionID, appID, platform string) error {
	submissionID = strings.TrimSpace(submissionID)
	appID = strings.TrimSpace(appID)
	platform = strings.TrimSpace(platform)
	requestCtx, cancel := backgroundAssetSubmitRequestContext(ctx)
	defer cancel()
	response, err := client.GetReviewSubmissionStrict(requestCtx, submissionID, asc.WithReviewSubmissionInclude([]string{"app"}))
	if err != nil {
		return err
	}
	if response == nil {
		return fmt.Errorf("review submission response is required")
	}
	resource := &response.Data
	if resource.Type != asc.ResourceTypeReviewSubmissions {
		return fmt.Errorf("review submission %s returned resource type %q, not %q", submissionID, resource.Type, asc.ResourceTypeReviewSubmissions)
	}
	actualID := strings.TrimSpace(resource.ID)
	if actualID == "" {
		return fmt.Errorf("review submission %s returned an empty ID", submissionID)
	}
	if actualID != submissionID {
		return fmt.Errorf("app store connect returned review submission %s instead of %s", actualID, submissionID)
	}
	if resource.Attributes.SubmissionState != asc.ReviewSubmissionStateReadyForReview {
		return fmt.Errorf("review submission %s is in state %q, not %q", actualID, resource.Attributes.SubmissionState, asc.ReviewSubmissionStateReadyForReview)
	}
	if !strings.EqualFold(string(resource.Attributes.Platform), platform) {
		return fmt.Errorf("review submission %s is for platform %q, not %q", actualID, resource.Attributes.Platform, platform)
	}
	if resource.Relationships == nil || resource.Relationships.App == nil {
		return fmt.Errorf("review submission %s did not prove its app relationship", actualID)
	}
	appRelationship := resource.Relationships.App.Data
	if appRelationship.Type != asc.ResourceTypeApps {
		return fmt.Errorf("review submission %s returned app relationship type %q, not %q", actualID, appRelationship.Type, asc.ResourceTypeApps)
	}
	actualAppID := strings.TrimSpace(appRelationship.ID)
	if actualAppID == "" {
		return fmt.Errorf("review submission %s did not prove its app relationship", actualID)
	}
	if actualAppID != appID {
		return fmt.Errorf("review submission %s belongs to app %q, not %q", actualID, actualAppID, appID)
	}
	return nil
}

type backgroundAssetSubmitResolver struct {
	client backgroundAssetSubmitClient
}

func resolveBackgroundAssetSubmitItems(ctx context.Context, r backgroundAssetSubmitResolver, sel backgroundAssetSubmitSelection) ([]backgroundAssetsSubmitResultItem, error) {
	if len(sel.explicitVersionIDs) > 0 {
		items := make([]backgroundAssetsSubmitResultItem, 0, len(sel.explicitVersionIDs))
		seen := make(map[string]struct{}, len(sel.explicitVersionIDs))
		for _, vid := range sel.explicitVersionIDs {
			vid = strings.TrimSpace(vid)
			if vid == "" {
				continue
			}
			if _, ok := seen[vid]; ok {
				continue
			}
			seen[vid] = struct{}{}
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

type (
	ascBackgroundAssetItem        = asc.Resource[asc.BackgroundAssetAttributes]
	ascBackgroundAssetVersionItem = asc.Resource[asc.BackgroundAssetVersionAttributes]
)

func fetchAllBackgroundAssets(ctx context.Context, client backgroundAssetSubmitClient, appID string, opts ...asc.BackgroundAssetsOption) ([]ascBackgroundAssetItem, error) {
	firstCtx, firstCancel := backgroundAssetSubmitRequestContext(ctx)
	first, err := client.GetBackgroundAssets(firstCtx, appID, opts...)
	firstCancel()
	if err != nil {
		return nil, fmt.Errorf("list background assets: %w", err)
	}
	resp, err := asc.PaginateAll(ctx, first, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
		pageCtx, pageCancel := backgroundAssetSubmitRequestContext(ctx)
		defer pageCancel()
		return client.GetBackgroundAssets(pageCtx, appID, asc.WithBackgroundAssetsNextURL(nextURL))
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
	firstCtx, firstCancel := backgroundAssetSubmitRequestContext(ctx)
	first, err := client.GetReviewSubmissionItemsStrict(firstCtx, submissionID, opts...)
	firstCancel()
	if err != nil {
		return nil, fmt.Errorf("list submission items: %w", err)
	}
	page := first
	seenNext := make(map[string]struct{})
	for pageNumber := 1; ; pageNumber++ {
		for _, item := range page.Data {
			if item.Relationships == nil || item.Relationships.BackgroundAssetVersion == nil {
				continue
			}
			if strings.EqualFold(string(item.Attributes.State), "REMOVED") {
				continue
			}
			backgroundAssetVersion := item.Relationships.BackgroundAssetVersion.Data
			if backgroundAssetVersion.Type != asc.ResourceTypeBackgroundAssetVersions {
				return nil, fmt.Errorf("review submission item %q returned background asset version relationship type %q, not %q", strings.TrimSpace(item.ID), backgroundAssetVersion.Type, asc.ResourceTypeBackgroundAssetVersions)
			}
			bgVer := strings.TrimSpace(backgroundAssetVersion.ID)
			if bgVer == "" {
				return nil, fmt.Errorf("review submission item %q returned an empty background asset version relationship ID", strings.TrimSpace(item.ID))
			}
			attached[bgVer] = struct{}{}
		}

		nextURL := page.Links.Next
		if nextURL == "" {
			break
		}
		if pageNumber >= backgroundAssetsReviewSubmissionItemsMaxPages {
			return nil, fmt.Errorf("paginate submission items: page %d exceeds the maximum of %d pages", pageNumber+1, backgroundAssetsReviewSubmissionItemsMaxPages)
		}
		if _, exists := seenNext[nextURL]; exists {
			return nil, fmt.Errorf("paginate submission items: page %d: %w", pageNumber+1, asc.ErrRepeatedPaginationURL)
		}
		seenNext[nextURL] = struct{}{}
		pageCtx, pageCancel := backgroundAssetSubmitRequestContext(ctx)
		page, err = client.GetReviewSubmissionItemsStrict(pageCtx, submissionID, asc.WithReviewSubmissionItemsNextURL(nextURL), asc.WithReviewSubmissionItemsInclude([]string{"backgroundAssetVersion"}))
		pageCancel()
		if err != nil {
			return nil, fmt.Errorf("paginate submission items: page %d: %w", pageNumber+1, err)
		}
	}
	return attached, nil
}

func fetchAllBackgroundAssetVersions(ctx context.Context, client backgroundAssetSubmitClient, backgroundAssetID string) ([]ascBackgroundAssetVersionItem, error) {
	opts := []asc.BackgroundAssetVersionsOption{asc.WithBackgroundAssetVersionsLimit(backgroundAssetsMaxLimit)}
	firstCtx, firstCancel := backgroundAssetSubmitRequestContext(ctx)
	first, err := client.GetBackgroundAssetVersions(firstCtx, backgroundAssetID, opts...)
	firstCancel()
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	resp, err := asc.PaginateAll(ctx, first, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
		pageCtx, pageCancel := backgroundAssetSubmitRequestContext(ctx)
		defer pageCancel()
		return client.GetBackgroundAssetVersions(pageCtx, backgroundAssetID, asc.WithBackgroundAssetVersionsNextURL(nextURL))
	})
	if err != nil {
		return nil, fmt.Errorf("paginate versions: %w", err)
	}
	if aggregate, ok := resp.(*asc.BackgroundAssetVersionsResponse); ok {
		return aggregate.Data, nil
	}
	return first.Data, nil
}
