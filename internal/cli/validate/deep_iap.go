package validate

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

type iapAttachmentCandidate struct {
	productID, versionID string
	editable             bool
}
type iapAttachmentType struct {
	products            []validation.IAP
	candidates          []iapAttachmentCandidate
	approved, uncertain bool
}

func collectIAPAttachmentDeepCheck(ctx context.Context, client *asc.Client, report validation.Report, catalog []validation.IAP, fetchSkipReason string) (validation.DeepCheck, *validation.CheckResult) {
	ctx, cancel := shared.ContextWithTimeout(withReadinessRequestGate(ctx))
	defer cancel()
	check := validation.DeepCheck{ID: validation.DeepCheckIAPAttachment, Source: validation.DeepSourcePublicAPI, Status: validation.DeepStatusNotApplicable, Message: "No ready first-of-type IAP requires attachment"}
	resolution := &validation.Resolution{Fixability: validation.FixabilityManual, AppStoreConnectURL: appStoreConnectURL(report.AppID, "appstore/review")}
	finish := func(status validation.DeepStatus, message string) (validation.DeepCheck, *validation.CheckResult) {
		check.Status, check.Message, check.Resolution = status, message, resolution
		if status != validation.DeepStatusBlocked && status != validation.DeepStatusUnverified {
			return check, nil
		}
		severity := validation.SeverityWarning
		if status == validation.DeepStatusBlocked {
			severity = validation.SeverityError
		}
		return check, &validation.CheckResult{ID: check.ID, Severity: severity, Message: message, Remediation: "Inspect the selected app version and IAP versions in the same App Review submission", ResourceType: "inAppPurchases", Resolution: resolution}
	}
	if fetchSkipReason != "" {
		return finish(validation.DeepStatusUnverified, fetchSkipReason)
	}
	groups := map[string]*iapAttachmentType{}
	for _, kind := range []string{"CONSUMABLE", "NON_CONSUMABLE", "NON_RENEWING_SUBSCRIPTION"} {
		groups[kind] = &iapAttachmentType{}
	}
	catalogUnknown := false
	for _, product := range catalog {
		group := groups[product.Type]
		if group == nil {
			catalogUnknown = true
			continue
		}
		if product.ID == "" || product.ProductID == "" || !knownIAPCatalogState(product.State) {
			catalogUnknown = true
		}
		group.products = append(group.products, product)
		group.approved = group.approved || (product.State == "APPROVED" && product.ID != "" && product.ProductID != "")
	}
	needed := false
	for _, group := range groups {
		for _, p := range group.products {
			if p.State == "READY_TO_SUBMIT" {
				needed = true
			}
		}
	}
	if !needed {
		if catalogUnknown {
			return finish(validation.DeepStatusUnverified, "Apple returned incomplete IAP identity, type, or state")
		}
		return check, nil
	}
	if report.VersionState == "" {
		return finish(validation.DeepStatusUnverified, "The selected app version state is unavailable")
	}
	switch report.VersionState {
	case "READY_FOR_SALE", "READY_FOR_DISTRIBUTION", "PENDING_DEVELOPER_RELEASE", "PENDING_APPLE_RELEASE", "PROCESSING_FOR_APP_STORE", "DEVELOPER_REMOVED_FROM_SALE", "REMOVED_FROM_SALE":
		return check, nil
	}
	if !isSubscriptionAttachmentVersionState(report.VersionState) {
		return finish(validation.DeepStatusUnverified, "The selected app version state is not recognized for IAP attachment")
	}
	if report.VersionID == "" || report.Platform == "" {
		return finish(validation.DeepStatusUnverified, "The selected app version identity or platform is unavailable")
	}
	versions := map[string]*asc.InAppPurchaseVersionsResponse{}
	for _, group := range groups {
		hasReady := false
		for _, p := range group.products {
			hasReady = hasReady || p.State == "READY_TO_SUBMIT"
		}
		if !hasReady || group.approved {
			continue
		}
		for _, p := range group.products {
			response, ok := versions[p.ID]
			if !ok {
				first, err := doReadinessRequest(ctx, func(c context.Context) (*asc.InAppPurchaseVersionsResponse, error) {
					return client.GetInAppPurchaseVersionsStrict(c, p.ID, asc.WithIAPVersionsFields([]string{"version", "state"}), asc.WithIAPVersionsLimit(200))
				})
				if err != nil {
					group.uncertain = true
					continue
				}
				total, hasTotal := asc.ParsePagingTotalOK(first.Meta)
				all, err := asc.PaginateAll(ctx, first, func(c context.Context, next string) (asc.PaginatedResponse, error) {
					return doReadinessRequest(c, func(c context.Context) (asc.PaginatedResponse, error) {
						page, err := client.GetInAppPurchaseVersionsStrict(c, p.ID, asc.WithIAPVersionsNextURL(next))
						if page != nil {
							if n, ok := asc.ParsePagingTotalOK(page.Meta); ok && (!hasTotal || n > total) {
								total, hasTotal = n, true
							}
						}
						return page, err
					})
				})
				if err != nil {
					group.uncertain = true
					continue
				}
				response = all.(*asc.InAppPurchaseVersionsResponse)
				ids := map[string]bool{}
				for _, v := range response.Data {
					ids[v.ID] = true
				}
				if hasTotal && total > len(ids) {
					group.uncertain = true
					continue
				}
				versions[p.ID] = response
			}
			versionIDs := map[string]asc.InAppPurchaseVersionAttributes{}
			versionConflict := false
			for _, version := range response.Data {
				if previous, ok := versionIDs[version.ID]; ok && previous != version.Attributes {
					versionConflict = true
				}
				versionIDs[version.ID] = version.Attributes
			}
			if versionConflict {
				group.uncertain = true
				continue
			}
			current := []asc.Resource[asc.InAppPurchaseVersionAttributes]{}
			seenVersions := map[string]bool{}
			currentKnown := true
			for _, v := range response.Data {
				if seenVersions[v.ID] {
					continue
				}
				seenVersions[v.ID] = true
				if v.ID == "" || v.Attributes.Version <= 0 {
					currentKnown = false
					group.uncertain = true
					continue
				}
				switch v.Attributes.State {
				case "APPROVED":
					group.approved = true
				case "ACCEPTED":
					group.uncertain = true
					current = append(current, v)
				case "PREPARE_FOR_SUBMISSION", "READY_FOR_REVIEW", "WAITING_FOR_REVIEW", "IN_REVIEW", "REJECTED", "DEVELOPER_REJECTED":
					current = append(current, v)
				case "REPLACED_WITH_NEW_VERSION":
					group.uncertain = true
				default:
					currentKnown = false
					group.uncertain = true
				}
			}
			if p.State == "READY_TO_SUBMIT" || p.State == "WAITING_FOR_REVIEW" || p.State == "IN_REVIEW" || p.State == "PENDING_BINARY_APPROVAL" {
				if !currentKnown || len(current) != 1 || current[0].Attributes.State == "REJECTED" || current[0].Attributes.State == "DEVELOPER_REJECTED" {
					group.uncertain = true
				} else {
					group.candidates = append(group.candidates, iapAttachmentCandidate{p.ProductID, current[0].ID, p.State == "READY_TO_SUBMIT" && (current[0].Attributes.State == "PREPARE_FOR_SUBMISSION" || current[0].Attributes.State == "READY_FOR_REVIEW")})
				}
			}
			if p.State == "REMOVED_FROM_SALE" || p.State == "DEVELOPER_REMOVED_FROM_SALE" {
				group.uncertain = true
			}
		}
	}
	allApproved := true
	for _, group := range groups {
		for _, p := range group.products {
			if p.State == "READY_TO_SUBMIT" && !group.approved {
				allApproved = false
			}
		}
	}
	if allApproved && !catalogUnknown {
		return finish(validation.DeepStatusPassed, "Each ready IAP type has an explicitly approved product or version")
	}
	first, err := doReadinessRequest(ctx, func(c context.Context) (*asc.ReviewSubmissionsResponse, error) {
		return client.GetReviewSubmissionsStrict(c, report.AppID, asc.WithReviewSubmissionsInclude([]string{"appStoreVersionForReview"}), asc.Limit[asc.ReviewSubmissionsOption](200))
	})
	if err != nil {
		return finish(validation.DeepStatusUnverified, "App Review submissions could not be verified through the public API")
	}
	total, hasTotal := asc.ParsePagingTotalOK(first.Meta)
	all, err := asc.PaginateAll(ctx, first, func(c context.Context, next string) (asc.PaginatedResponse, error) {
		return doReadinessRequest(c, func(c context.Context) (asc.PaginatedResponse, error) {
			page, err := client.GetReviewSubmissionsStrict(c, report.AppID, asc.NextURL[asc.ReviewSubmissionsOption](next))
			if page != nil {
				if n, ok := asc.ParsePagingTotalOK(page.Meta); ok && (!hasTotal || n > total) {
					total, hasTotal = n, true
				}
			}
			return page, err
		})
	})
	if err != nil {
		return finish(validation.DeepStatusUnverified, "The complete App Review submission history could not be verified")
	}
	submissions := all.(*asc.ReviewSubmissionsResponse)
	ids := map[string]bool{}
	for _, s := range submissions.Data {
		ids[s.ID] = true
	}
	if hasTotal && total > len(ids) {
		return finish(validation.DeepStatusUnverified, "The advertised App Review submission total exceeds the complete response")
	}
	uncertain := catalogUnknown
	attached := map[string]bool{}
	editableDrafts := []string{}
	bySubmissionID := map[string]asc.ReviewSubmissionResource{}
	conflictingSubmissions := map[string]bool{}
	for _, submission := range submissions.Data {
		if previous, ok := bySubmissionID[submission.ID]; ok && (previous.Attributes.Platform != submission.Attributes.Platform || previous.Attributes.SubmissionState != submission.Attributes.SubmissionState || !reflect.DeepEqual(previous.Relationships, submission.Relationships)) {
			conflictingSubmissions[submission.ID] = true
		}
		bySubmissionID[submission.ID] = submission
	}
	seenSubmissions := map[string]bool{}
	for _, s := range submissions.Data {
		if conflictingSubmissions[s.ID] {
			uncertain = true
			continue
		}
		if seenSubmissions[s.ID] {
			continue
		}
		seenSubmissions[s.ID] = true
		if !s.Relationships.AppStoreVersionLinkageKnown() {
			uncertain = true
			continue
		}
		link := s.Relationships.AppStoreVersionForReview
		if link == nil || link.Data.ID != report.VersionID {
			continue
		}
		if string(s.Attributes.Platform) != report.Platform {
			uncertain = true
			continue
		}
		switch s.Attributes.SubmissionState {
		case asc.ReviewSubmissionStateComplete, asc.ReviewSubmissionStateCanceling:
			continue
		case asc.ReviewSubmissionStateReadyForReview, asc.ReviewSubmissionStateWaitingForReview, asc.ReviewSubmissionStateInReview, asc.ReviewSubmissionStateUnresolvedIssues, asc.ReviewSubmissionStateCompleting:
		default:
			uncertain = true
			continue
		}
		items, err := fetchIAPSubmissionItems(ctx, client, s.ID)
		if err != nil {
			uncertain = true
			continue
		}
		appPresent, itemUnknown := false, false
		present := map[string]bool{}
		for _, item := range items {
			if item.Attributes.State == "REMOVED" {
				continue
			}
			switch item.Attributes.State {
			case "READY_FOR_REVIEW", "ACCEPTED", "APPROVED", "REJECTED":
			default:
				itemUnknown = true
				continue
			}
			if !item.Relationships.VersionLinkagesKnown() || item.Relationships.HasConflictingContentLinkages() {
				itemUnknown = true
				continue
			}
			if a := item.Relationships.AppStoreVersion; a != nil && a.Data.ID != "" {
				if a.Data.ID == report.VersionID {
					appPresent = true
				} else {
					itemUnknown = true
				}
			}
			if v := item.Relationships.InAppPurchaseVersion; v != nil && item.Attributes.State != "REJECTED" {
				present[v.Data.ID] = true
			}
		}
		uncertain = uncertain || itemUnknown
		if appPresent && !itemUnknown {
			for id := range present {
				attached[id] = true
			}
			if s.Attributes.SubmissionState == asc.ReviewSubmissionStateReadyForReview {
				editableDrafts = append(editableDrafts, s.ID)
			}
		}
	}
	missing, unknown := []string{}, []string{}
	missingCandidates := []iapAttachmentCandidate{}
	for _, kind := range []string{"CONSUMABLE", "NON_CONSUMABLE", "NON_RENEWING_SUBSCRIPTION"} {
		group := groups[kind]
		ready := []string{}
		for _, p := range group.products {
			if p.State == "READY_TO_SUBMIT" {
				ready = append(ready, p.ProductID)
			}
		}
		if len(ready) == 0 || group.approved {
			continue
		}
		satisfied := false
		for _, candidate := range group.candidates {
			satisfied = satisfied || attached[candidate.versionID]
		}
		if satisfied {
			continue
		}
		label := fmt.Sprintf("%s (%s)", kind, strings.Join(ready, ", "))
		if uncertain || group.uncertain {
			unknown = append(unknown, label)
		} else {
			missing = append(missing, label)
			missingCandidates = append(missingCandidates, group.candidates...)
		}
	}
	if len(missing) > 0 {
		if len(missingCandidates) == 1 && len(editableDrafts) == 1 && missingCandidates[0].editable && isSubscriptionAttachmentEditableVersionState(report.VersionState) {
			resolution.Fixability = validation.FixabilityWebFixable
			resolution.Commands = []string{fmt.Sprintf("asc web review iaps attach --app %q --iap-id %q --submission %q --confirm", report.AppID, missingCandidates[0].productID, editableDrafts[0])}
		}
		message := "First-of-type IAP attachment is missing from the selected app version submission: " + strings.Join(missing, "; ")
		if len(unknown) > 0 {
			message += ". Unverified: " + strings.Join(unknown, "; ")
		}
		return finish(validation.DeepStatusBlocked, message)
	}
	if catalogUnknown && len(unknown) == 0 {
		return finish(validation.DeepStatusUnverified, "Apple returned incomplete IAP identity, type, or state for additional catalog products")
	}
	if len(unknown) > 0 {
		return finish(validation.DeepStatusUnverified, "First-of-type IAP attachment could not be verified: "+strings.Join(unknown, "; "))
	}
	return finish(validation.DeepStatusPassed, "Each ready IAP type has prior approval or an exact current version in the selected app version submission")
}

func fetchIAPSubmissionItems(ctx context.Context, client *asc.Client, submissionID string) ([]asc.ReviewSubmissionItemResource, error) {
	first, err := doReadinessRequest(ctx, func(c context.Context) (*asc.ReviewSubmissionItemsResponse, error) {
		return client.GetReviewSubmissionItemsStrict(c, submissionID, asc.WithReviewSubmissionItemsFields([]string{"state", "appStoreVersion", "inAppPurchaseVersion"}), asc.WithReviewSubmissionItemsInclude([]string{"appStoreVersion", "inAppPurchaseVersion"}), asc.Limit[asc.ReviewSubmissionItemsOption](200))
	})
	if err != nil {
		return nil, err
	}
	total, hasTotal := asc.ParsePagingTotalOK(first.Meta)
	all, err := asc.PaginateAll(ctx, first, func(c context.Context, next string) (asc.PaginatedResponse, error) {
		return doReadinessRequest(c, func(c context.Context) (asc.PaginatedResponse, error) {
			page, err := client.GetReviewSubmissionItemsStrict(c, submissionID, asc.NextURL[asc.ReviewSubmissionItemsOption](next))
			if page != nil {
				if n, ok := asc.ParsePagingTotalOK(page.Meta); ok && (!hasTotal || n > total) {
					total, hasTotal = n, true
				}
			}
			return page, err
		})
	})
	if err != nil {
		return nil, err
	}
	items := all.(*asc.ReviewSubmissionItemsResponse).Data
	ids := map[string]asc.ReviewSubmissionItemResource{}
	unique := make([]asc.ReviewSubmissionItemResource, 0, len(items))
	for _, item := range items {
		if previous, ok := ids[item.ID]; ok {
			if previous.Attributes.State != item.Attributes.State || !reflect.DeepEqual(previous.Relationships, item.Relationships) {
				return nil, fmt.Errorf("conflicting submission item identity")
			}
			continue
		}
		ids[item.ID] = item
		unique = append(unique, item)
	}
	if hasTotal && total > len(ids) {
		return nil, fmt.Errorf("incomplete submission items")
	}
	return unique, nil
}

func knownIAPCatalogState(state string) bool {
	switch state {
	case "MISSING_METADATA", "WAITING_FOR_UPLOAD", "PROCESSING_CONTENT", "READY_TO_SUBMIT", "WAITING_FOR_REVIEW", "IN_REVIEW", "DEVELOPER_ACTION_NEEDED", "PENDING_BINARY_APPROVAL", "APPROVED", "DEVELOPER_REMOVED_FROM_SALE", "REMOVED_FROM_SALE", "REJECTED":
		return true
	}
	return false
}
