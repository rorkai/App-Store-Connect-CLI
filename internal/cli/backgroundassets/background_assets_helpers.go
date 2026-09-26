package backgroundassets

import (
	"fmt"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

const (
	backgroundAssetsMaxLimit                      = 200
	backgroundAssetsReviewSubmissionItemsMaxPages = 1000
)

var backgroundAssetUploadFileAssetTypeValues = []string{
	string(asc.BackgroundAssetUploadFileAssetTypeAsset),
	string(asc.BackgroundAssetUploadFileAssetTypeManifest),
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
