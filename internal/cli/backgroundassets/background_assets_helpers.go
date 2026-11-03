package backgroundassets

import (
	"fmt"
	"slices"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const (
	backgroundAssetsMaxLimit                      = 200
	backgroundAssetsReviewSubmissionItemsMaxPages = 1000
)

var backgroundAssetUploadFileAssetTypeValues = []string{
	string(asc.BackgroundAssetUploadFileAssetTypeAsset),
	string(asc.BackgroundAssetUploadFileAssetTypeManifest),
}

var (
	backgroundAssetSortValues        = []string{"assetPackIdentifier", "-assetPackIdentifier", "createdDate", "-createdDate"}
	backgroundAssetVersionSortValues = []string{"version", "-version"}
	backgroundAssetVersionStates     = []string{"AWAITING_UPLOAD", "PROCESSING", "FAILED", "COMPLETE"}
	backgroundAssetInternalBetaState = []string{"READY_FOR_TESTING", "SUPERSEDED"}
	backgroundAssetExternalBetaState = []string{
		"READY_FOR_BETA_SUBMISSION",
		"WAITING_FOR_REVIEW",
		"IN_REVIEW",
		"REJECTED",
		"PROCESSING_FOR_TESTING",
		"READY_FOR_TESTING",
		"SUPERSEDED",
	}
	backgroundAssetAppStoreState = []string{
		"PREPARE_FOR_SUBMISSION",
		"READY_FOR_REVIEW",
		"WAITING_FOR_REVIEW",
		"IN_REVIEW",
		"ACCEPTED",
		"REJECTED",
		"PROCESSING_FOR_DISTRIBUTION",
		"READY_FOR_DISTRIBUTION",
		"SUPERSEDED",
	}
)

// parseBackgroundAssetEnumCSV uppercases a comma-separated flag value and
// rejects anything outside allowed, returning nil when the flag is blank.
func parseBackgroundAssetEnumCSV(flagName, value string, allowed []string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	values := shared.SplitCSVUpper(value)
	if len(values) == 0 {
		return nil, fmt.Errorf("%s must be one of: %s", flagName, strings.Join(allowed, ", "))
	}
	for _, item := range values {
		if !slices.Contains(allowed, item) {
			return nil, fmt.Errorf("%s must be one of: %s", flagName, strings.Join(allowed, ", "))
		}
	}
	return values, nil
}

func normalizeBackgroundAssetUploadFileAssetType(value string) (asc.BackgroundAssetUploadFileAssetType, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")
	switch normalized {
	case string(asc.BackgroundAssetUploadFileAssetTypeAsset):
		return asc.BackgroundAssetUploadFileAssetTypeAsset, nil
	case string(asc.BackgroundAssetUploadFileAssetTypeManifest):
		return asc.BackgroundAssetUploadFileAssetTypeManifest, nil
	default:
		return "", fmt.Errorf("--asset-type must be one of: %s", strings.Join(backgroundAssetUploadFileAssetTypeValues, ", "))
	}
}
