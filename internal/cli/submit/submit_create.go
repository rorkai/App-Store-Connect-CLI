package submit

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func addVersionToSubmissionOrRecover(
	ctx context.Context,
	client *asc.Client,
	submissionID, versionID, appID, platform string,
	emit func(string),
) (string, error) {
	alreadyAttached, preflightErr := validateReviewSubmissionBeforeAdd(ctx, client, submissionID, appID, platform, versionID)
	if preflightErr != nil {
		return "", preflightErr
	}
	if alreadyAttached {
		message := fmt.Sprintf("Version already attached to review submission %s; skipping duplicate add.", strings.TrimSpace(submissionID))
		if emit != nil {
			emit(message)
		} else {
			fmt.Fprintln(os.Stderr, message)
		}
		return strings.TrimSpace(submissionID), nil
	}

	_, err := client.AddReviewSubmissionItem(ctx, submissionID, versionID)
	if err == nil {
		return submissionID, nil
	}

	conflict := extractSubmissionConflict(err)
	conflictSubmissionID := strings.TrimSpace(conflict.SubmissionID)
	if conflict.Kind != submissionConflictAlreadyAttached || conflictSubmissionID == "" {
		return "", err
	}
	if verifyErr := verifyReviewSubmissionForSubmit(ctx, client, conflictSubmissionID, appID, platform, versionID); verifyErr != nil {
		return "", fmt.Errorf(
			"conflict review submission %s could not be safely reused: %w",
			conflictSubmissionID,
			verifyErr,
		)
	}

	message := fmt.Sprintf("Version already in review submission %s, reusing it.", conflictSubmissionID)
	if emit != nil {
		emit(message)
	} else {
		fmt.Fprintln(os.Stderr, message)
	}
	return conflictSubmissionID, nil
}

// validateReviewSubmissionBeforeAdd re-reads the submission immediately before
// adding an item. The discovery response is only a candidate; the detail and
// item read establish that the submission is still ready, belongs to this app,
// and cannot be turned into a mixed submission by the add below.
func validateReviewSubmissionBeforeAdd(
	ctx context.Context,
	client *asc.Client,
	submissionID, appID, platform, versionID string,
) (bool, error) {
	submissionID = strings.TrimSpace(submissionID)
	appID = strings.TrimSpace(appID)
	platform = strings.ToUpper(strings.TrimSpace(platform))
	versionID = strings.TrimSpace(versionID)
	if submissionID == "" || appID == "" || platform == "" || versionID == "" {
		return false, fmt.Errorf("submission, app, platform, and version IDs are required")
	}

	refreshed, err := client.GetReviewSubmissionStrict(
		ctx,
		submissionID,
		asc.WithReviewSubmissionInclude([]string{"app"}),
	)
	if err != nil {
		return false, fmt.Errorf("refresh review submission before adding item: %w", err)
	}
	if refreshed == nil {
		return false, fmt.Errorf("app store connect did not return review submission %s", submissionID)
	}
	if err := validateReviewSubmissionForMutation(&refreshed.Data, submissionID, appID, platform); err != nil {
		return false, err
	}

	summary, err := summarizeReviewSubmissionItems(ctx, client, submissionID, versionID)
	if err != nil {
		return false, fmt.Errorf("inspect review submission items before adding item: %w", err)
	}
	if summary.hasOtherItems {
		return false, fmt.Errorf("review submission %s contains unrelated review items", submissionID)
	}
	return summary.hasTargetVersion, nil
}

func verifyReviewSubmissionForSubmit(
	ctx context.Context,
	client *asc.Client,
	submissionID, appID, platform, versionID string,
) error {
	submissionID = strings.TrimSpace(submissionID)
	appID = strings.TrimSpace(appID)
	platform = strings.ToUpper(strings.TrimSpace(platform))
	versionID = strings.TrimSpace(versionID)
	if submissionID == "" || appID == "" || platform == "" || versionID == "" {
		return fmt.Errorf("submission, app, platform, and version IDs are required")
	}

	refreshed, err := client.GetReviewSubmissionStrict(
		ctx,
		submissionID,
		asc.WithReviewSubmissionInclude([]string{"app"}),
	)
	if err != nil {
		return fmt.Errorf("refresh review submission: %w", err)
	}
	if refreshed == nil || strings.TrimSpace(refreshed.Data.ID) != submissionID {
		return fmt.Errorf("app store connect did not return review submission %s", submissionID)
	}
	if err := validateReviewSubmissionForMutation(&refreshed.Data, submissionID, appID, platform); err != nil {
		return err
	}

	summary, err := summarizeReviewSubmissionItems(ctx, client, submissionID, versionID)
	if err != nil {
		return fmt.Errorf("inspect review submission items: %w", err)
	}
	if !summary.hasTargetVersion {
		return fmt.Errorf("review submission %s does not contain target version %s", submissionID, versionID)
	}
	if summary.hasOtherItems {
		return fmt.Errorf("review submission %s contains unrelated review items", submissionID)
	}
	return nil
}

func validateReviewSubmissionForMutation(submission *asc.ReviewSubmissionResource, expectedID, appID, platform string) error {
	expectedID = strings.TrimSpace(expectedID)
	appID = strings.TrimSpace(appID)
	platform = strings.ToUpper(strings.TrimSpace(platform))
	if submission == nil {
		return fmt.Errorf("review submission response is required")
	}
	if submission.Type != asc.ResourceTypeReviewSubmissions {
		return fmt.Errorf("review submission %s returned resource type %q, not %q", expectedID, submission.Type, asc.ResourceTypeReviewSubmissions)
	}
	actualID := strings.TrimSpace(submission.ID)
	if actualID == "" {
		return fmt.Errorf("review submission %s returned an empty ID", expectedID)
	}
	if expectedID != "" && actualID != expectedID {
		return fmt.Errorf("app store connect returned review submission %s instead of %s", actualID, expectedID)
	}
	if submission.Attributes.SubmissionState != asc.ReviewSubmissionStateReadyForReview {
		return fmt.Errorf(
			"review submission %s is in state %q, not %q",
			actualID,
			submission.Attributes.SubmissionState,
			asc.ReviewSubmissionStateReadyForReview,
		)
	}
	if !strings.EqualFold(string(submission.Attributes.Platform), platform) {
		return fmt.Errorf(
			"review submission %s is for platform %q, not %q",
			actualID,
			submission.Attributes.Platform,
			platform,
		)
	}
	if submission.Relationships == nil || submission.Relationships.App == nil {
		return fmt.Errorf("review submission %s did not prove its app relationship", actualID)
	}
	appRelationship := submission.Relationships.App
	if appRelationship.Data.Type != asc.ResourceTypeApps {
		return fmt.Errorf("review submission %s returned app relationship type %q, not %q", actualID, appRelationship.Data.Type, asc.ResourceTypeApps)
	}
	actualAppID := strings.TrimSpace(appRelationship.Data.ID)
	if actualAppID == "" {
		return fmt.Errorf("review submission %s did not prove its app relationship", actualID)
	}
	if appID != "" && actualAppID != appID {
		return fmt.Errorf("review submission %s belongs to app %q, not %q", actualID, actualAppID, appID)
	}
	return nil
}

func validateReviewSubmissionCreateReceipt(receipt *asc.ReviewSubmissionResponse, appID, platform string) error {
	if receipt == nil {
		return fmt.Errorf("review submission create response is required")
	}
	if receipt.Data.Type != asc.ResourceTypeReviewSubmissions {
		return fmt.Errorf("created review submission returned resource type %q, not %q", receipt.Data.Type, asc.ResourceTypeReviewSubmissions)
	}
	if strings.TrimSpace(receipt.Data.ID) == "" {
		return fmt.Errorf("created review submission returned an empty ID")
	}
	// Create responses may be sparse. Reject contradictory fields immediately;
	// the mandatory detail read before adding the item validates omitted fields.
	if state := strings.TrimSpace(string(receipt.Data.Attributes.SubmissionState)); state != "" && receipt.Data.Attributes.SubmissionState != asc.ReviewSubmissionStateReadyForReview {
		return fmt.Errorf("created review submission %s is in state %q, not %q", strings.TrimSpace(receipt.Data.ID), receipt.Data.Attributes.SubmissionState, asc.ReviewSubmissionStateReadyForReview)
	}
	if receiptPlatform := strings.TrimSpace(string(receipt.Data.Attributes.Platform)); receiptPlatform != "" && !strings.EqualFold(receiptPlatform, strings.TrimSpace(platform)) {
		return fmt.Errorf("created review submission %s is for platform %q, not %q", strings.TrimSpace(receipt.Data.ID), receiptPlatform, strings.TrimSpace(platform))
	}
	if relationships := receipt.Data.Relationships; relationships != nil && relationships.App != nil {
		if relationships.App.Data.Type != asc.ResourceTypeApps {
			return fmt.Errorf("created review submission %s returned app relationship type %q, not %q", strings.TrimSpace(receipt.Data.ID), relationships.App.Data.Type, asc.ResourceTypeApps)
		}
		if actualAppID := strings.TrimSpace(relationships.App.Data.ID); actualAppID == "" {
			return fmt.Errorf("created review submission %s did not prove its app relationship", strings.TrimSpace(receipt.Data.ID))
		} else if expectedAppID := strings.TrimSpace(appID); expectedAppID != "" && actualAppID != expectedAppID {
			return fmt.Errorf("created review submission %s belongs to app %q, not %q", strings.TrimSpace(receipt.Data.ID), actualAppID, expectedAppID)
		}
	}
	return nil
}

type submitCreateReviewSubmissionPreparation struct {
	reuseSubmissionID         string
	reuseSubmissionHasVersion bool
}

func prepareReviewSubmissionForCreate(
	ctx context.Context,
	client *asc.Client,
	appID, platform, versionID string,
	emit func(string),
) (submitCreateReviewSubmissionPreparation, error) {
	return prepareReviewSubmissionForCreateWithPageLimit(ctx, client, appID, platform, versionID, emit, reviewSubmissionDiscoveryMaxPages)
}

// reviewSubmissionDiscoveryMaxPages bounds untrusted links.next chains while
// allowing up to 200,000 discovered submissions at the requested page size.
const reviewSubmissionDiscoveryMaxPages = 1000

func prepareReviewSubmissionForCreateWithPageLimit(
	ctx context.Context,
	client *asc.Client,
	appID, platform, versionID string,
	emit func(string),
	maxPages int,
) (submitCreateReviewSubmissionPreparation, error) {
	if ctx == nil {
		return submitCreateReviewSubmissionPreparation{}, fmt.Errorf("review submission preparation context is required")
	}
	if client == nil {
		return submitCreateReviewSubmissionPreparation{}, fmt.Errorf("review submission preparation client is required")
	}
	appID = strings.TrimSpace(appID)
	platform = strings.TrimSpace(platform)
	versionID = strings.TrimSpace(versionID)
	if appID == "" || platform == "" || versionID == "" {
		return submitCreateReviewSubmissionPreparation{}, fmt.Errorf("app, platform, and version IDs are required")
	}
	if maxPages <= 0 {
		return submitCreateReviewSubmissionPreparation{}, fmt.Errorf("review submission discovery page limit must be positive")
	}

	emitMessage := func(format string, args ...any) {
		message := fmt.Sprintf(format, args...)
		if emit != nil {
			emit(message)
			return
		}
		fmt.Fprintln(os.Stderr, message)
	}

	existing, err := client.GetReviewSubmissionsStrict(
		ctx,
		appID,
		asc.WithReviewSubmissionsStates([]string{string(asc.ReviewSubmissionStateReadyForReview)}),
		asc.WithReviewSubmissionsPlatforms([]string{platform}),
		asc.WithReviewSubmissionsInclude([]string{"appStoreVersionForReview"}),
		asc.WithReviewSubmissionsLimit(200),
	)
	if err != nil {
		return submitCreateReviewSubmissionPreparation{}, fmt.Errorf("query ready review submissions: %w", err)
	}
	if existing == nil {
		return submitCreateReviewSubmissionPreparation{}, fmt.Errorf("query ready review submissions: response is required")
	}
	if existing.Data == nil {
		return submitCreateReviewSubmissionPreparation{}, fmt.Errorf("query ready review submissions: response data is required")
	}
	expectedSubmissions, submissionsTotalKnown := asc.ParsePagingTotalOK(existing.Meta)
	existing.Links.Next = strings.TrimSpace(existing.Links.Next)
	submissionPages := 1

	paginated, err := asc.PaginateAll(ctx, existing, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		if submissionPages >= maxPages {
			return nil, fmt.Errorf("review submission discovery pagination must not exceed %d pages", maxPages)
		}
		submissionPages++
		next, nextErr := client.GetReviewSubmissionsStrict(ctx, appID, asc.WithReviewSubmissionsNextURL(nextURL))
		if nextErr != nil {
			return nil, nextErr
		}
		if next == nil {
			return nil, fmt.Errorf("response is required")
		}
		if next.Data == nil {
			return nil, fmt.Errorf("response data is required")
		}
		if nextTotal, ok := asc.ParsePagingTotalOK(next.Meta); ok {
			if submissionsTotalKnown && nextTotal != expectedSubmissions {
				return nil, fmt.Errorf("paging total changed from %d to %d", expectedSubmissions, nextTotal)
			}
			expectedSubmissions = nextTotal
			submissionsTotalKnown = true
		}
		next.Links.Next = strings.TrimSpace(next.Links.Next)
		return next, nil
	})
	if err != nil {
		return submitCreateReviewSubmissionPreparation{}, fmt.Errorf("query ready review submissions: %w", err)
	}
	all, ok := paginated.(*asc.ReviewSubmissionsResponse)
	if !ok || all == nil {
		return submitCreateReviewSubmissionPreparation{}, fmt.Errorf("query ready review submissions: unexpected response type %T", paginated)
	}
	submissions := all.Data
	if submissionsTotalKnown && len(submissions) != expectedSubmissions {
		return submitCreateReviewSubmissionPreparation{}, fmt.Errorf(
			"query ready review submissions: incomplete response: received %d of %d submissions",
			len(submissions),
			expectedSubmissions,
		)
	}

	if len(submissions) == 0 {
		return submitCreateReviewSubmissionPreparation{}, nil
	}

	result := submitCreateReviewSubmissionPreparation{}
	normalizedPlatform := strings.ToUpper(strings.TrimSpace(platform))
	targetVersionID := strings.TrimSpace(versionID)
	inspector := newReviewSubmissionInspector(client, targetVersionID)

	for i := range submissions {
		if err := validateReviewSubmissionForPreparation(&submissions[i]); err != nil {
			return submitCreateReviewSubmissionPreparation{}, fmt.Errorf("query ready review submissions: response data[%d]: %w", i, err)
		}
	}

	for i := range submissions {
		sub := submissions[i]
		if sub.Attributes.SubmissionState != asc.ReviewSubmissionStateReadyForReview {
			continue
		}
		if normalizedPlatform != "" && !strings.EqualFold(string(sub.Attributes.Platform), normalizedPlatform) {
			continue
		}
		if strings.TrimSpace(sub.ID) == "" {
			return submitCreateReviewSubmissionPreparation{}, fmt.Errorf("ready review submission is missing an ID")
		}
		reusable, hasVersion, reuseErr := inspector.canReuse(ctx, &sub)
		if reuseErr != nil {
			return submitCreateReviewSubmissionPreparation{}, fmt.Errorf("inspect ready review submission %q: %w", strings.TrimSpace(sub.ID), reuseErr)
		}
		if reusable {
			if hasVersion {
				emitMessage("Reusing existing review submission %s because the target version is already attached.", sub.ID)
			} else {
				emitMessage("Reusing existing review submission %s because it has no review items.", sub.ID)
			}
			result.reuseSubmissionID = strings.TrimSpace(sub.ID)
			result.reuseSubmissionHasVersion = hasVersion
			return result, nil
		}
		emitMessage(
			"Skipped stale review submission %s because it is not exclusively usable for version %s. Cancel it explicitly with `asc submit cancel --id %s --confirm` if you intend to replace it.",
			sub.ID,
			targetVersionID,
			sub.ID,
		)
	}
	return result, nil
}

func reviewSubmissionAppStoreVersionID(submission *asc.ReviewSubmissionResource) string {
	if submission == nil || submission.Relationships == nil || submission.Relationships.AppStoreVersionForReview == nil {
		return ""
	}
	if submission.Relationships.AppStoreVersionForReview.Data.Type != asc.ResourceTypeAppStoreVersions {
		return ""
	}
	return strings.TrimSpace(submission.Relationships.AppStoreVersionForReview.Data.ID)
}

func validateReviewSubmissionForPreparation(submission *asc.ReviewSubmissionResource) error {
	if submission == nil {
		return fmt.Errorf("resource is required")
	}
	if submission.Type != asc.ResourceTypeReviewSubmissions {
		return fmt.Errorf("resource type must be %q, got %q", asc.ResourceTypeReviewSubmissions, submission.Type)
	}
	if strings.TrimSpace(submission.ID) == "" {
		return fmt.Errorf("resource ID must not be empty")
	}
	if strings.TrimSpace(string(submission.Attributes.SubmissionState)) == "" {
		return fmt.Errorf("state must not be empty")
	}
	if strings.TrimSpace(string(submission.Attributes.Platform)) == "" {
		return fmt.Errorf("platform must not be empty")
	}
	if submission.Relationships == nil {
		return nil
	}
	for name, relationship := range map[string]struct {
		value        *asc.Relationship
		expectedType asc.ResourceType
	}{
		"app":                      {value: submission.Relationships.App, expectedType: asc.ResourceTypeApps},
		"appStoreVersionForReview": {value: submission.Relationships.AppStoreVersionForReview, expectedType: asc.ResourceTypeAppStoreVersions},
		"submittedByActor":         {value: submission.Relationships.SubmittedByActor, expectedType: asc.ResourceTypeActors},
		"lastUpdatedByActor":       {value: submission.Relationships.LastUpdatedByActor, expectedType: asc.ResourceTypeActors},
	} {
		if relationship.value == nil {
			continue
		}
		resourceType := strings.TrimSpace(string(relationship.value.Data.Type))
		resourceID := strings.TrimSpace(relationship.value.Data.ID)
		if resourceType == "" && resourceID == "" {
			continue
		}
		if resourceType != string(relationship.expectedType) {
			return fmt.Errorf("relationship %s type must be %q, got %q", name, relationship.expectedType, resourceType)
		}
		if resourceID == "" {
			return fmt.Errorf("relationship %s ID must not be empty", name)
		}
	}
	return nil
}

type reviewSubmissionItemSummary struct {
	hasItems         bool
	hasTargetVersion bool
	hasOtherItems    bool
}

// reviewSubmissionInspector answers reuse and cancellation questions about the
// app's ready-for-review submissions. Item membership does not change while
// preparation runs, so each submission is inspected at most once per pass.
type reviewSubmissionInspector struct {
	client          *asc.Client
	targetVersionID string
	summaries       map[string]reviewSubmissionItemSummary
	summaryErrors   map[string]error
}

func newReviewSubmissionInspector(client *asc.Client, targetVersionID string) *reviewSubmissionInspector {
	return &reviewSubmissionInspector{
		client:          client,
		targetVersionID: strings.TrimSpace(targetVersionID),
		summaries:       make(map[string]reviewSubmissionItemSummary),
		summaryErrors:   make(map[string]error),
	}
}

func (i *reviewSubmissionInspector) summarize(ctx context.Context, submissionID string) (reviewSubmissionItemSummary, error) {
	submissionID = strings.TrimSpace(submissionID)
	if cached, ok := i.summaries[submissionID]; ok {
		return cached, nil
	}
	if cachedErr, ok := i.summaryErrors[submissionID]; ok {
		return reviewSubmissionItemSummary{}, cachedErr
	}
	summary, err := summarizeReviewSubmissionItems(ctx, i.client, submissionID, i.targetVersionID)
	if err != nil {
		i.summaryErrors[submissionID] = err
		return reviewSubmissionItemSummary{}, err
	}
	i.summaries[submissionID] = summary
	return summary, nil
}

func (i *reviewSubmissionInspector) canReuse(ctx context.Context, submission *asc.ReviewSubmissionResource) (reusable bool, hasVersion bool, err error) {
	if submission == nil {
		return false, false, nil
	}

	submissionID := strings.TrimSpace(submission.ID)
	if submissionID == "" {
		return false, false, nil
	}
	if linkedVersionID := reviewSubmissionAppStoreVersionID(submission); linkedVersionID != "" && linkedVersionID != i.targetVersionID {
		return false, false, nil
	}

	itemSummary, err := i.summarize(ctx, submissionID)
	if err != nil {
		return false, false, err
	}
	if itemSummary.hasItems {
		if itemSummary.hasTargetVersion && !itemSummary.hasOtherItems {
			return true, true, nil
		}
		return false, false, nil
	}

	return true, false, nil
}

// reviewSubmissionItemsMaxPages bounds untrusted links.next chains while
// allowing a large realistic review-submission item collection (200 items per
// page, up to 200,000 items).
const reviewSubmissionItemsMaxPages = 1000

func summarizeReviewSubmissionItems(
	ctx context.Context,
	client *asc.Client,
	submissionID, targetVersionID string,
) (reviewSubmissionItemSummary, error) {
	return summarizeReviewSubmissionItemsWithPageLimit(ctx, client, submissionID, targetVersionID, reviewSubmissionItemsMaxPages)
}

func summarizeReviewSubmissionItemsWithPageLimit(
	ctx context.Context,
	client *asc.Client,
	submissionID, targetVersionID string,
	maxPages int,
) (reviewSubmissionItemSummary, error) {
	var summary reviewSubmissionItemSummary

	if ctx == nil {
		return summary, fmt.Errorf("review submission item context is required")
	}
	submissionID = strings.TrimSpace(submissionID)
	targetVersionID = strings.TrimSpace(targetVersionID)
	if client == nil {
		return summary, fmt.Errorf("review submission item client is required")
	}
	if submissionID == "" || targetVersionID == "" {
		return summary, fmt.Errorf("submission and target version IDs are required")
	}
	if maxPages <= 0 {
		return summary, fmt.Errorf("review submission item page limit must be positive")
	}

	// appStoreVersion must be INCLUDED, not merely named in fields[]. fields[] is a sparse-fieldset
	// selector: it narrows what comes back, and the API answers it with items that carry "links" only
	// and no "relationships" key at all. Every item then reads as an empty version id,
	// hasTargetVersion never becomes true, and a correctly prepared submission is rejected as not
	// containing its version. include= is what makes App Store Connect materialise the linkage;
	// fields[] rides along to keep the item payload narrow.
	resp, err := client.GetReviewSubmissionItemsStrict(
		ctx,
		submissionID,
		asc.WithReviewSubmissionItemsInclude([]string{"appStoreVersion"}),
		asc.WithReviewSubmissionItemsFields([]string{"appStoreVersion"}),
		asc.WithReviewSubmissionItemsLimit(200),
	)
	if err != nil {
		return summary, err
	}
	if resp == nil {
		return summary, fmt.Errorf("review submission items response is required")
	}
	if resp.Data == nil {
		return summary, fmt.Errorf("review submission items response data is required")
	}

	page := 0
	itemsSeen := 0
	expectedItems, itemsTotalKnown := asc.ParsePagingTotalOK(resp.Meta)
	consume := func(pageResponse asc.PaginatedResponse) error {
		page++
		if page > maxPages {
			return fmt.Errorf("review submission items pagination must not exceed %d pages", maxPages)
		}
		pageItems, ok := pageResponse.(*asc.ReviewSubmissionItemsResponse)
		if !ok || pageItems == nil {
			return fmt.Errorf("unexpected review submission items response type %T", pageResponse)
		}
		if pageItems.Data == nil {
			return fmt.Errorf("review submission items response data is required")
		}
		pageItems.Links.Next = strings.TrimSpace(pageItems.Links.Next)
		for index := range pageItems.Data {
			if err := validateReviewSubmissionItemForInspection(&pageItems.Data[index]); err != nil {
				return fmt.Errorf("review submission item[%d]: %w", index, err)
			}
		}
		if nextTotal, ok := asc.ParsePagingTotalOK(pageItems.Meta); ok {
			if itemsTotalKnown && nextTotal != expectedItems {
				return fmt.Errorf("review submission items paging total changed from %d to %d", expectedItems, nextTotal)
			}
			expectedItems = nextTotal
			itemsTotalKnown = true
		}
		accumulateReviewSubmissionItemSummary(&summary, pageItems.Data, targetVersionID)
		itemsSeen += len(pageItems.Data)
		if itemsTotalKnown && itemsSeen > expectedItems {
			return fmt.Errorf("review submission items response exceeded total: received %d of %d items", itemsSeen, expectedItems)
		}
		if strings.TrimSpace(pageItems.Links.Next) == "" && itemsTotalKnown && itemsSeen != expectedItems {
			return fmt.Errorf("review submission items response is incomplete: received %d of %d items", itemsSeen, expectedItems)
		}
		return nil
	}
	fetchNext := func(fetchCtx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		if page >= maxPages {
			return nil, fmt.Errorf("review submission items pagination must not exceed %d pages", maxPages)
		}
		return client.GetReviewSubmissionItemsStrict(fetchCtx, submissionID, asc.WithReviewSubmissionItemsNextURL(strings.TrimSpace(nextURL)))
	}
	if err := asc.PaginateEach(ctx, resp, fetchNext, consume); err != nil {
		return summary, err
	}
	return summary, nil
}

func accumulateReviewSubmissionItemSummary(summary *reviewSubmissionItemSummary, items []asc.ReviewSubmissionItemResource, targetVersionID string) {
	if summary == nil {
		return
	}

	targetVersionID = strings.TrimSpace(targetVersionID)
	for _, item := range items {
		summary.hasItems = true

		versionID := reviewSubmissionItemVersionID(item)
		switch {
		case targetVersionID != "" && versionID == targetVersionID:
			summary.hasTargetVersion = true
		case versionID != "":
			summary.hasOtherItems = true
		default:
			// If the item is not the target version, treat it as unrelated and
			// avoid reusing the submission implicitly.
			summary.hasOtherItems = true
		}
	}
}

func reviewSubmissionItemVersionID(item asc.ReviewSubmissionItemResource) string {
	if item.Relationships == nil || item.Relationships.AppStoreVersion == nil {
		return ""
	}
	if item.Relationships.AppStoreVersion.Data.Type != asc.ResourceTypeAppStoreVersions {
		return ""
	}
	return strings.TrimSpace(item.Relationships.AppStoreVersion.Data.ID)
}

func validateReviewSubmissionItemForInspection(item *asc.ReviewSubmissionItemResource) error {
	if item == nil {
		return fmt.Errorf("resource is required")
	}
	if item.Type != asc.ResourceTypeReviewSubmissionItems {
		return fmt.Errorf("resource type must be %q, got %q", asc.ResourceTypeReviewSubmissionItems, item.Type)
	}
	if strings.TrimSpace(item.ID) == "" {
		return fmt.Errorf("resource ID must not be empty")
	}
	if item.Relationships == nil {
		return nil
	}
	for name, relationship := range map[string]struct {
		value        *asc.Relationship
		expectedType asc.ResourceType
	}{
		"appStoreVersion":                 {value: item.Relationships.AppStoreVersion, expectedType: asc.ResourceTypeAppStoreVersions},
		"appCustomProductPageVersion":     {value: item.Relationships.AppCustomProductPageVersion, expectedType: asc.ResourceTypeAppCustomProductPageVersions},
		"appEvent":                        {value: item.Relationships.AppEvent, expectedType: asc.ResourceTypeAppEvents},
		"appStoreVersionExperiment":       {value: item.Relationships.AppStoreVersionExperiment, expectedType: asc.ResourceTypeAppStoreVersionExperiments},
		"appStoreVersionExperimentV2":     {value: item.Relationships.AppStoreVersionExperimentV2, expectedType: asc.ResourceTypeAppStoreVersionExperiments},
		"backgroundAssetVersion":          {value: item.Relationships.BackgroundAssetVersion, expectedType: asc.ResourceTypeBackgroundAssetVersions},
		"gameCenterAchievementVersion":    {value: item.Relationships.GameCenterAchievementVersion, expectedType: asc.ResourceTypeGameCenterAchievementVersions},
		"gameCenterActivityVersion":       {value: item.Relationships.GameCenterActivityVersion, expectedType: asc.ResourceTypeGameCenterActivityVersions},
		"gameCenterChallengeVersion":      {value: item.Relationships.GameCenterChallengeVersion, expectedType: asc.ResourceTypeGameCenterChallengeVersions},
		"gameCenterLeaderboardSetVersion": {value: item.Relationships.GameCenterLeaderboardSetVersion, expectedType: asc.ResourceTypeGameCenterLeaderboardSetVersions},
		"gameCenterLeaderboardVersion":    {value: item.Relationships.GameCenterLeaderboardVersion, expectedType: asc.ResourceTypeGameCenterLeaderboardVersions},
		"inAppPurchaseVersion":            {value: item.Relationships.InAppPurchaseVersion, expectedType: asc.ResourceTypeInAppPurchaseVersions},
		"subscriptionVersion":             {value: item.Relationships.SubscriptionVersion, expectedType: asc.ResourceTypeSubscriptionVersions},
		"subscriptionGroupVersion":        {value: item.Relationships.SubscriptionGroupVersion, expectedType: asc.ResourceTypeSubscriptionGroupVersions},
	} {
		if relationship.value == nil {
			continue
		}
		resourceType := strings.TrimSpace(string(relationship.value.Data.Type))
		resourceID := strings.TrimSpace(relationship.value.Data.ID)
		if resourceType == "" && resourceID == "" {
			continue
		}
		if resourceType != string(relationship.expectedType) {
			return fmt.Errorf("relationship %s type must be %q, got %q", name, relationship.expectedType, resourceType)
		}
		if resourceID == "" {
			return fmt.Errorf("relationship %s ID must not be empty", name)
		}
	}
	return nil
}

func refreshReviewSubmission(ctx context.Context, client *asc.Client, submissionID string) (*asc.ReviewSubmissionResource, error) {
	submissionID = strings.TrimSpace(submissionID)
	if submissionID == "" || client == nil {
		return nil, nil
	}
	resp, err := client.GetReviewSubmissionStrict(ctx, submissionID)
	if err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func reviewSubmissionIsState(ctx context.Context, client *asc.Client, submissionID string, wantState asc.ReviewSubmissionState) (bool, error) {
	refreshed, err := refreshReviewSubmission(ctx, client, submissionID)
	if err != nil || refreshed == nil {
		return false, err
	}
	return refreshed.Attributes.SubmissionState == wantState, nil
}

func preserveCreatedReviewSubmission(submissionID string, emit func(string)) {
	submissionID = strings.TrimSpace(submissionID)
	if submissionID == "" {
		return
	}
	message := fmt.Sprintf(
		"Preserved newly created review submission %s instead of canceling it automatically because it may now contain review items. Retry the command if this run reports an error; it will reuse the draft when safe. To cancel it intentionally, run `asc submit cancel --id %s --confirm`.",
		submissionID,
		submissionID,
	)
	if emit != nil {
		emit(message)
	} else {
		fmt.Fprintln(os.Stderr, message)
	}
}
