package asc

import (
	"context"
	"fmt"
	"reflect"
)

// PaginatedResponse represents a response that supports pagination
type PaginatedResponse interface {
	GetLinks() *Links
	GetData() interface{}
}

// GetLinks returns the links field for pagination
func (r *Response[T]) GetLinks() *Links {
	return &r.Links
}

// GetData returns the data field for aggregation
func (r *Response[T]) GetData() interface{} {
	return r.Data
}

// GetLinks returns the links field for pagination.
func (r *LinkagesResponse) GetLinks() *Links {
	return &r.Links
}

// GetData returns the data field for aggregation.
func (r *LinkagesResponse) GetData() interface{} {
	return r.Data
}

// GetLinks returns the links field for pagination.
func (r *PreReleaseVersionsResponse) GetLinks() *Links {
	return &r.Links
}

// GetData returns the data field for aggregation.
func (r *PreReleaseVersionsResponse) GetData() interface{} {
	return r.Data
}

// PaginateFunc is a function that fetches a page of results
type PaginateFunc func(ctx context.Context, nextURL string) (PaginatedResponse, error)

// PaginateAll fetches all pages and aggregates results
func PaginateAll(ctx context.Context, firstPage PaginatedResponse, fetchNext PaginateFunc) (PaginatedResponse, error) {
	if firstPage == nil {
		return nil, nil
	}

	// Determine the response type from the first page
	var result PaginatedResponse
	switch firstPage.(type) {
	case *FeedbackResponse:
		result = &FeedbackResponse{Links: Links{}}
	case *CrashesResponse:
		result = &CrashesResponse{Links: Links{}}
	case *ReviewsResponse:
		result = &ReviewsResponse{Links: Links{}}
	case *AppsResponse:
		result = &AppsResponse{Links: Links{}}
	case *AppTagsResponse:
		result = &AppTagsResponse{Links: Links{}}
	case *LinkagesResponse:
		result = &LinkagesResponse{Links: Links{}}
	case *BundleIDsResponse:
		result = &BundleIDsResponse{Links: Links{}}
	case *MerchantIDsResponse:
		result = &MerchantIDsResponse{Links: Links{}}
	case *PassTypeIDsResponse:
		result = &PassTypeIDsResponse{Links: Links{}}
	case *InAppPurchasesV2Response:
		result = &InAppPurchasesV2Response{Links: Links{}}
	case *TerritoriesResponse:
		result = &TerritoriesResponse{Links: Links{}}
	case *DiagnosticSignaturesResponse:
		result = &DiagnosticSignaturesResponse{Links: Links{}}
	case *TerritoryAvailabilitiesResponse:
		result = &TerritoryAvailabilitiesResponse{Links: Links{}}
	case *AppPricePointsV3Response:
		result = &AppPricePointsV3Response{Links: Links{}}
	case *BuildsResponse:
		result = &BuildsResponse{Links: Links{}}
	case *BuildBundleFileSizesResponse:
		result = &BuildBundleFileSizesResponse{Links: Links{}}
	case *BetaAppClipInvocationsResponse:
		result = &BetaAppClipInvocationsResponse{Links: Links{}}
	case *SubscriptionOfferCodeOneTimeUseCodesResponse:
		result = &SubscriptionOfferCodeOneTimeUseCodesResponse{Links: Links{}}
	case *AppStoreVersionsResponse:
		result = &AppStoreVersionsResponse{Links: Links{}}
	case *ReviewSubmissionsResponse:
		result = &ReviewSubmissionsResponse{Links: Links{}}
	case *ReviewSubmissionItemsResponse:
		result = &ReviewSubmissionItemsResponse{Links: Links{}}
	case *NominationsResponse:
		result = &NominationsResponse{Links: Links{}}
	case *PreReleaseVersionsResponse:
		result = &PreReleaseVersionsResponse{Links: Links{}}
	case *AccessibilityDeclarationsResponse:
		result = &AccessibilityDeclarationsResponse{Links: Links{}}
	case *AppEncryptionDeclarationsResponse:
		result = &AppEncryptionDeclarationsResponse{Links: Links{}}
	case *AppStoreReviewAttachmentsResponse:
		result = &AppStoreReviewAttachmentsResponse{Links: Links{}}
	case *AppStoreVersionLocalizationsResponse:
		result = &AppStoreVersionLocalizationsResponse{Links: Links{}}
	case *BetaBuildLocalizationsResponse:
		result = &BetaBuildLocalizationsResponse{Links: Links{}}
	case *AppInfoLocalizationsResponse:
		result = &AppInfoLocalizationsResponse{Links: Links{}}
	case *InAppPurchaseLocalizationsResponse:
		result = &InAppPurchaseLocalizationsResponse{Links: Links{}}
	case *SubscriptionGroupsResponse:
		result = &SubscriptionGroupsResponse{Links: Links{}}
	case *SubscriptionsResponse:
		result = &SubscriptionsResponse{Links: Links{}}
	case *BetaGroupsResponse:
		result = &BetaGroupsResponse{Links: Links{}}
	case *BetaTestersResponse:
		result = &BetaTestersResponse{Links: Links{}}
	case *BundleIDCapabilitiesResponse:
		result = &BundleIDCapabilitiesResponse{Links: Links{}}
	case *CertificatesResponse:
		result = &CertificatesResponse{Links: Links{}}
	case *DevicesResponse:
		result = &DevicesResponse{Links: Links{}}
	case *ProfilesResponse:
		result = &ProfilesResponse{Links: Links{}}
	case *UsersResponse:
		result = &UsersResponse{Links: Links{}}
	case *UserInvitationsResponse:
		result = &UserInvitationsResponse{Links: Links{}}
	case *SandboxTestersResponse:
		result = &SandboxTestersResponse{Links: Links{}}
	case *AnalyticsReportRequestsResponse:
		result = &AnalyticsReportRequestsResponse{Links: Links{}}
	case *CiProductsResponse:
		result = &CiProductsResponse{Links: Links{}}
	case *CiWorkflowsResponse:
		result = &CiWorkflowsResponse{Links: Links{}}
	case *ScmGitReferencesResponse:
		result = &ScmGitReferencesResponse{Links: Links{}}
	case *ScmRepositoriesResponse:
		result = &ScmRepositoriesResponse{Links: Links{}}
	case *CiBuildRunsResponse:
		result = &CiBuildRunsResponse{Links: Links{}}
	case *CiBuildActionsResponse:
		result = &CiBuildActionsResponse{Links: Links{}}
	case *CiArtifactsResponse:
		result = &CiArtifactsResponse{Links: Links{}}
	case *CiTestResultsResponse:
		result = &CiTestResultsResponse{Links: Links{}}
	case *CiIssuesResponse:
		result = &CiIssuesResponse{Links: Links{}}
	case *CiMacOsVersionsResponse:
		result = &CiMacOsVersionsResponse{Links: Links{}}
	case *CiXcodeVersionsResponse:
		result = &CiXcodeVersionsResponse{Links: Links{}}
	default:
		return nil, fmt.Errorf("unsupported response type for pagination")
	}

	page := 1
	seenNext := make(map[string]struct{})
	for {
		// Aggregate data from current page using reflection over the Data field.
		// This keeps aggregation generic while still validating type compatibility.
		if err := aggregatePageData(result, firstPage); err != nil {
			return nil, fmt.Errorf("page %d: %w", page, err)
		}

		// Check for next page
		links := firstPage.GetLinks()
		if links == nil || links.Next == "" {
			break
		}

		if _, ok := seenNext[links.Next]; ok {
			return result, fmt.Errorf("page %d: detected repeated pagination URL", page+1)
		}
		seenNext[links.Next] = struct{}{}
		page++

		// Fetch next page
		nextPage, err := fetchNext(ctx, links.Next)
		if err != nil {
			return result, fmt.Errorf("page %d: %w", page, err)
		}

		// Validate that the response type matches
		if typeOf(nextPage) != typeOf(firstPage) {
			return result, fmt.Errorf("page %d: unexpected response type (expected %T, got %T)", page, firstPage, nextPage)
		}

		firstPage = nextPage
	}

	return result, nil
}

// aggregatePageData appends page data to result by reflecting on the shared Data field.
// This keeps pagination aggregation generic while still validating type compatibility.
func aggregatePageData(result, page PaginatedResponse) error {
	if result == nil || page == nil {
		return fmt.Errorf("page aggregation received nil result or page")
	}

	resultValue := reflect.ValueOf(result)
	pageValue := reflect.ValueOf(page)
	if resultValue.Kind() != reflect.Ptr || pageValue.Kind() != reflect.Ptr {
		return fmt.Errorf("page aggregation expects pointer types (got %T and %T)", result, page)
	}

	if resultValue.Type() != pageValue.Type() {
		return fmt.Errorf("type mismatch: page is %T but result is %T", page, result)
	}

	resultElem := resultValue.Elem()
	pageElem := pageValue.Elem()
	resultData := resultElem.FieldByName("Data")
	pageData := pageElem.FieldByName("Data")
	if !resultData.IsValid() || !pageData.IsValid() {
		return fmt.Errorf("missing Data field for %T", page)
	}
	if resultData.Kind() != reflect.Slice || pageData.Kind() != reflect.Slice {
		return fmt.Errorf("Data field is not a slice for %T", page)
	}
	if resultData.Type() != pageData.Type() {
		return fmt.Errorf("Data field type mismatch: %s vs %s", resultData.Type(), pageData.Type())
	}

	resultData.Set(reflect.AppendSlice(resultData, pageData))
	return nil
}

// typeOf returns the runtime type of a PaginatedResponse
func typeOf(p PaginatedResponse) string {
	switch p.(type) {
	case *FeedbackResponse:
		return "FeedbackResponse"
	case *CrashesResponse:
		return "CrashesResponse"
	case *ReviewsResponse:
		return "ReviewsResponse"
	case *AppsResponse:
		return "AppsResponse"
	case *AppTagsResponse:
		return "AppTagsResponse"
	case *LinkagesResponse:
		return "LinkagesResponse"
	case *BundleIDsResponse:
		return "BundleIDsResponse"
	case *MerchantIDsResponse:
		return "MerchantIDsResponse"
	case *PassTypeIDsResponse:
		return "PassTypeIDsResponse"
	case *InAppPurchasesV2Response:
		return "InAppPurchasesV2Response"
	case *TerritoriesResponse:
		return "TerritoriesResponse"
	case *DiagnosticSignaturesResponse:
		return "DiagnosticSignaturesResponse"
	case *TerritoryAvailabilitiesResponse:
		return "TerritoryAvailabilitiesResponse"
	case *AppPricePointsV3Response:
		return "AppPricePointsV3Response"
	case *BuildsResponse:
		return "BuildsResponse"
	case *BuildBundleFileSizesResponse:
		return "BuildBundleFileSizesResponse"
	case *BetaAppClipInvocationsResponse:
		return "BetaAppClipInvocationsResponse"
	case *SubscriptionOfferCodeOneTimeUseCodesResponse:
		return "SubscriptionOfferCodeOneTimeUseCodesResponse"
	case *AppStoreVersionsResponse:
		return "AppStoreVersionsResponse"
	case *ReviewSubmissionsResponse:
		return "ReviewSubmissionsResponse"
	case *ReviewSubmissionItemsResponse:
		return "ReviewSubmissionItemsResponse"
	case *NominationsResponse:
		return "NominationsResponse"
	case *PreReleaseVersionsResponse:
		return "PreReleaseVersionsResponse"
	case *AccessibilityDeclarationsResponse:
		return "AccessibilityDeclarationsResponse"
	case *AppEncryptionDeclarationsResponse:
		return "AppEncryptionDeclarationsResponse"
	case *AppStoreReviewAttachmentsResponse:
		return "AppStoreReviewAttachmentsResponse"
	case *AppStoreVersionLocalizationsResponse:
		return "AppStoreVersionLocalizationsResponse"
	case *BetaBuildLocalizationsResponse:
		return "BetaBuildLocalizationsResponse"
	case *AppInfoLocalizationsResponse:
		return "AppInfoLocalizationsResponse"
	case *InAppPurchaseLocalizationsResponse:
		return "InAppPurchaseLocalizationsResponse"
	case *SubscriptionGroupsResponse:
		return "SubscriptionGroupsResponse"
	case *SubscriptionsResponse:
		return "SubscriptionsResponse"
	case *BetaGroupsResponse:
		return "BetaGroupsResponse"
	case *BetaTestersResponse:
		return "BetaTestersResponse"
	case *BundleIDCapabilitiesResponse:
		return "BundleIDCapabilitiesResponse"
	case *CertificatesResponse:
		return "CertificatesResponse"
	case *DevicesResponse:
		return "DevicesResponse"
	case *ProfilesResponse:
		return "ProfilesResponse"
	case *UsersResponse:
		return "UsersResponse"
	case *UserInvitationsResponse:
		return "UserInvitationsResponse"
	case *SandboxTestersResponse:
		return "SandboxTestersResponse"
	case *AnalyticsReportRequestsResponse:
		return "AnalyticsReportRequestsResponse"
	case *CiProductsResponse:
		return "CiProductsResponse"
	case *CiWorkflowsResponse:
		return "CiWorkflowsResponse"
	case *ScmGitReferencesResponse:
		return "ScmGitReferencesResponse"
	case *ScmRepositoriesResponse:
		return "ScmRepositoriesResponse"
	case *CiBuildRunsResponse:
		return "CiBuildRunsResponse"
	case *CiBuildActionsResponse:
		return "CiBuildActionsResponse"
	case *CiArtifactsResponse:
		return "CiArtifactsResponse"
	case *CiTestResultsResponse:
		return "CiTestResultsResponse"
	case *CiIssuesResponse:
		return "CiIssuesResponse"
	case *CiMacOsVersionsResponse:
		return "CiMacOsVersionsResponse"
	case *CiXcodeVersionsResponse:
		return "CiXcodeVersionsResponse"
	default:
		return "unknown"
	}
}
