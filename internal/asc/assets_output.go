package asc

import (
	"fmt"
	"sort"
	"strings"
)

// AppScreenshotSetWithScreenshots groups a set with its screenshots.
type AppScreenshotSetWithScreenshots struct {
	Set         Resource[AppScreenshotSetAttributes] `json:"set"`
	Screenshots []Resource[AppScreenshotAttributes]  `json:"screenshots"`
}

// AppScreenshotLocalizationListResult groups one App Store version
// localization with its screenshot sets.
type AppScreenshotLocalizationListResult struct {
	Locale                string                            `json:"locale"`
	VersionLocalizationID string                            `json:"versionLocalizationId"`
	Sets                  []AppScreenshotSetWithScreenshots `json:"sets"`
}

// AppScreenshotListResult represents screenshot list output by localization.
// Localizations is populated instead of VersionLocalizationID and Sets when a
// caller lists every localization of a version without selecting a locale.
type AppScreenshotListResult struct {
	VersionLocalizationID string                                `json:"versionLocalizationId"`
	Locale                string                                `json:"locale,omitempty"`
	Sets                  []AppScreenshotSetWithScreenshots     `json:"sets"`
	Localizations         []AppScreenshotLocalizationListResult `json:"localizations,omitempty"`
}

// AppScreenshotSetListResult represents screenshot sets and their screenshots
// for a non-version localization, such as a custom product page or treatment.
type AppScreenshotSetListResult struct {
	LocalizationID string                            `json:"localizationId"`
	Sets           []AppScreenshotSetWithScreenshots `json:"sets"`
}

// AppPreviewSetWithPreviews groups a set with its previews.
type AppPreviewSetWithPreviews struct {
	Set      Resource[AppPreviewSetAttributes] `json:"set"`
	Previews []Resource[AppPreviewAttributes]  `json:"previews"`
}

// AppPreviewListResult represents preview list output by localization.
type AppPreviewListResult struct {
	VersionLocalizationID string                      `json:"versionLocalizationId"`
	Sets                  []AppPreviewSetWithPreviews `json:"sets"`
}

// AssetUploadResultItem represents a single uploaded asset.
type AssetUploadResultItem struct {
	FileName string `json:"fileName"`
	FilePath string `json:"filePath"`
	AssetID  string `json:"assetId"`
	State    string `json:"state,omitempty"`
	Skipped  bool   `json:"skipped,omitempty"`
}

// AssetUploadFailureItem represents a failed upload item.
type AssetUploadFailureItem struct {
	FileName string `json:"fileName,omitempty"`
	FilePath string `json:"filePath,omitempty"`
	Error    string `json:"error"`
}

// AppScreenshotUploadResult represents screenshot upload output.
type AppScreenshotUploadResult struct {
	VersionLocalizationID string                   `json:"versionLocalizationId"`
	SetID                 string                   `json:"setId"`
	DisplayType           string                   `json:"displayType"`
	DryRun                bool                     `json:"dryRun,omitempty"`
	Resumed               bool                     `json:"resumed,omitempty"`
	Total                 int                      `json:"total,omitempty"`
	Uploaded              int                      `json:"uploaded,omitempty"`
	Skipped               int                      `json:"skipped,omitempty"`
	Pending               int                      `json:"pending,omitempty"`
	Failed                int                      `json:"failed,omitempty"`
	FailureArtifactPath   string                   `json:"failureArtifactPath,omitempty"`
	Results               []AssetUploadResultItem  `json:"results"`
	Failures              []AssetUploadFailureItem `json:"failures,omitempty"`
}

// AppScreenshotLocalizationUploadResult represents one localization in a fan-out screenshot upload.
type AppScreenshotLocalizationUploadResult struct {
	Locale                string                   `json:"locale"`
	VersionLocalizationID string                   `json:"versionLocalizationId"`
	SetID                 string                   `json:"setId"`
	DisplayType           string                   `json:"displayType"`
	DryRun                bool                     `json:"dryRun,omitempty"`
	Total                 int                      `json:"total,omitempty"`
	Uploaded              int                      `json:"uploaded,omitempty"`
	Skipped               int                      `json:"skipped,omitempty"`
	Pending               int                      `json:"pending,omitempty"`
	Failed                int                      `json:"failed,omitempty"`
	FailureArtifactPath   string                   `json:"failureArtifactPath,omitempty"`
	Results               []AssetUploadResultItem  `json:"results"`
	Failures              []AssetUploadFailureItem `json:"failures,omitempty"`
}

// AppScreenshotFanoutUploadResult represents an app/version-scoped screenshot upload fan-out.
type AppScreenshotFanoutUploadResult struct {
	AppID         string                                  `json:"appId"`
	Version       string                                  `json:"version"`
	VersionID     string                                  `json:"versionId"`
	Platform      string                                  `json:"platform"`
	DisplayType   string                                  `json:"displayType"`
	DryRun        bool                                    `json:"dryRun,omitempty"`
	Localizations []AppScreenshotLocalizationUploadResult `json:"localizations"`
}

// AppPreviewUploadResult represents preview upload output.
type AppPreviewUploadResult struct {
	VersionLocalizationID string                   `json:"versionLocalizationId"`
	SetID                 string                   `json:"setId"`
	PreviewType           string                   `json:"previewType"`
	DryRun                bool                     `json:"dryRun,omitempty"`
	Results               []AssetUploadResultItem  `json:"results"`
	Failures              []AssetUploadFailureItem `json:"failures,omitempty"`
}

// CustomProductPageScreenshotUploadResult represents custom product page screenshot upload output.
type CustomProductPageScreenshotUploadResult struct {
	CustomProductPageLocalizationID string                  `json:"customProductPageLocalizationId"`
	SetID                           string                  `json:"setId"`
	DisplayType                     string                  `json:"displayType"`
	Results                         []AssetUploadResultItem `json:"results"`
}

// ExperimentTreatmentLocalizationScreenshotUploadResult represents PPO treatment localization screenshot upload output.
type ExperimentTreatmentLocalizationScreenshotUploadResult struct {
	ExperimentTreatmentLocalizationID string                  `json:"experimentTreatmentLocalizationId"`
	SetID                             string                  `json:"setId"`
	DisplayType                       string                  `json:"displayType"`
	Results                           []AssetUploadResultItem `json:"results"`
}

// CustomProductPagePreviewUploadResult represents custom product page preview upload output.
type CustomProductPagePreviewUploadResult struct {
	CustomProductPageLocalizationID string                  `json:"customProductPageLocalizationId"`
	SetID                           string                  `json:"setId"`
	PreviewType                     string                  `json:"previewType"`
	Results                         []AssetUploadResultItem `json:"results"`
}

// AssetDeleteResult represents deletion output for assets.
type AssetDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

func appScreenshotSetsRows(resp *AppScreenshotSetsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Display Type"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			SanitizeTerminalText(item.ID),
			SanitizeTerminalText(item.Attributes.ScreenshotDisplayType),
		})
	}
	return headers, rows
}

func appScreenshotsRows(resp *AppScreenshotsResponse) ([]string, [][]string) {
	headers := []string{"ID", "File Name", "File Size", "State"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			SanitizeTerminalText(item.ID),
			SanitizeTerminalText(item.Attributes.FileName),
			fmt.Sprintf("%d", item.Attributes.FileSize),
			SanitizeTerminalText(assetDeliveryStateValue(item.Attributes.AssetDeliveryState)),
		})
	}
	return headers, rows
}

func appPreviewSetsRows(resp *AppPreviewSetsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Preview Type"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			SanitizeTerminalText(item.ID),
			SanitizeTerminalText(item.Attributes.PreviewType),
		})
	}
	return headers, rows
}

func appPreviewsRows(resp *AppPreviewsResponse) ([]string, [][]string) {
	headers := []string{"ID", "File Name", "File Size", "Poster Frame", "State"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			SanitizeTerminalText(item.ID),
			SanitizeTerminalText(item.Attributes.FileName),
			fmt.Sprintf("%d", item.Attributes.FileSize),
			SanitizeTerminalText(item.Attributes.PreviewFrameTimeCode),
			SanitizeTerminalText(assetDeliveryStateValue(item.Attributes.AssetDeliveryState)),
		})
	}
	return headers, rows
}

func appScreenshotListResultRows(result *AppScreenshotListResult) ([]string, [][]string) {
	if len(result.Localizations) == 0 {
		return appScreenshotSetsWithScreenshotsRows(result.Sets)
	}
	return appScreenshotLocalizationListResultRows(result.Localizations)
}

func appScreenshotLocalizationListResultRows(localizations []AppScreenshotLocalizationListResult) ([]string, [][]string) {
	setHeaders, _ := appScreenshotSetsWithScreenshotsRows(nil)
	headers := append([]string{"Locale"}, setHeaders...)
	var rows [][]string
	for _, localization := range localizations {
		locale := SanitizeTerminalText(localization.Locale)
		_, setRows := appScreenshotSetsWithScreenshotsRows(localization.Sets)
		if len(setRows) == 0 {
			placeholder := make([]string, len(headers))
			placeholder[0] = locale
			rows = append(rows, placeholder)
			continue
		}
		for _, setRow := range setRows {
			rows = append(rows, append([]string{locale}, setRow...))
		}
	}
	return headers, rows
}

func appScreenshotSetListResultRows(result *AppScreenshotSetListResult) ([]string, [][]string) {
	return appScreenshotSetsWithScreenshotsRows(result.Sets)
}

func appScreenshotSetsWithScreenshotsRows(sets []AppScreenshotSetWithScreenshots) ([]string, [][]string) {
	headers := []string{"Set ID", "Display Type", "Screenshot ID", "File Name", "File Size", "State"}
	var rows [][]string
	for _, set := range sets {
		setID := SanitizeTerminalText(set.Set.ID)
		displayType := SanitizeTerminalText(set.Set.Attributes.ScreenshotDisplayType)
		if len(set.Screenshots) == 0 {
			rows = append(rows, []string{setID, displayType, "", "", "", ""})
			continue
		}
		for _, item := range set.Screenshots {
			rows = append(rows, []string{
				setID,
				displayType,
				SanitizeTerminalText(item.ID),
				SanitizeTerminalText(item.Attributes.FileName),
				fmt.Sprintf("%d", item.Attributes.FileSize),
				SanitizeTerminalText(assetDeliveryStateValue(item.Attributes.AssetDeliveryState)),
			})
		}
	}
	return headers, rows
}

func appPreviewListResultRows(result *AppPreviewListResult) ([]string, [][]string) {
	headers := []string{"Set ID", "Preview Type", "Preview ID", "File Name", "File Size", "Poster Frame", "State"}
	var rows [][]string
	for _, set := range result.Sets {
		setID := SanitizeTerminalText(set.Set.ID)
		previewType := SanitizeTerminalText(set.Set.Attributes.PreviewType)
		if len(set.Previews) == 0 {
			rows = append(rows, []string{setID, previewType, "", "", "", "", ""})
			continue
		}
		for _, item := range set.Previews {
			rows = append(rows, []string{
				setID,
				previewType,
				SanitizeTerminalText(item.ID),
				SanitizeTerminalText(item.Attributes.FileName),
				fmt.Sprintf("%d", item.Attributes.FileSize),
				SanitizeTerminalText(item.Attributes.PreviewFrameTimeCode),
				SanitizeTerminalText(assetDeliveryStateValue(item.Attributes.AssetDeliveryState)),
			})
		}
	}
	return headers, rows
}

func appScreenshotUploadResultMainRows(result *AppScreenshotUploadResult) ([]string, [][]string) {
	headers := []string{"Localization ID", "Set ID", "Display Type", "Dry Run", "Resumed", "Total", "Uploaded", "Skipped", "Pending", "Failed", "Failure Artifact"}
	rows := [][]string{{
		SanitizeTerminalText(result.VersionLocalizationID),
		SanitizeTerminalText(result.SetID),
		SanitizeTerminalText(result.DisplayType),
		fmt.Sprintf("%t", result.DryRun),
		fmt.Sprintf("%t", result.Resumed),
		fmt.Sprintf("%d", result.Total),
		fmt.Sprintf("%d", result.Uploaded),
		fmt.Sprintf("%d", result.Skipped),
		fmt.Sprintf("%d", result.Pending),
		fmt.Sprintf("%d", result.Failed),
		SanitizeTerminalText(result.FailureArtifactPath),
	}}
	return headers, rows
}

func appScreenshotFanoutUploadResultMainRows(result *AppScreenshotFanoutUploadResult) ([]string, [][]string) {
	headers := []string{"App ID", "Version", "Version ID", "Platform", "Display Type", "Dry Run", "Localizations"}
	rows := [][]string{{
		SanitizeTerminalText(result.AppID),
		SanitizeTerminalText(result.Version),
		SanitizeTerminalText(result.VersionID),
		SanitizeTerminalText(result.Platform),
		SanitizeTerminalText(result.DisplayType),
		fmt.Sprintf("%t", result.DryRun),
		fmt.Sprintf("%d", len(result.Localizations)),
	}}
	return headers, rows
}

func appScreenshotFanoutUploadLocalizationRows(result *AppScreenshotFanoutUploadResult) ([]string, [][]string) {
	headers := []string{"Locale", "Localization ID", "Set ID", "Files", "Uploaded", "Skipped", "Pending", "Failed", "Failure Artifact", "States"}
	rows := make([][]string, 0, len(result.Localizations))
	for _, item := range result.Localizations {
		total := item.Total
		if total == 0 {
			total = len(item.Results) + item.Pending
		}
		rows = append(rows, []string{
			SanitizeTerminalText(item.Locale),
			SanitizeTerminalText(item.VersionLocalizationID),
			SanitizeTerminalText(item.SetID),
			fmt.Sprintf("%d", total),
			fmt.Sprintf("%d", item.Uploaded),
			fmt.Sprintf("%d", item.Skipped),
			fmt.Sprintf("%d", item.Pending),
			fmt.Sprintf("%d", item.Failed),
			SanitizeTerminalText(item.FailureArtifactPath),
			summarizeAssetUploadStates(item.Results),
		})
	}
	return headers, rows
}

func appScreenshotFanoutUploadResultItemRows(result *AppScreenshotFanoutUploadResult) ([]string, [][]string) {
	headers := []string{"Locale", "File Name", "Asset ID", "State"}
	rows := make([][]string, 0)
	for _, localization := range result.Localizations {
		locale := SanitizeTerminalText(localization.Locale)
		if len(localization.Results) == 0 {
			rows = append(rows, []string{locale, "", "", ""})
			continue
		}
		for _, item := range localization.Results {
			rows = append(rows, []string{
				locale,
				SanitizeTerminalText(item.FileName),
				SanitizeTerminalText(item.AssetID),
				SanitizeTerminalText(assetUploadItemState(item)),
			})
		}
	}
	return headers, rows
}

func appScreenshotFanoutUploadFailureRows(result *AppScreenshotFanoutUploadResult) ([]string, [][]string) {
	headers := []string{"Locale", "File Name", "File Path", "Error"}
	rows := make([][]string, 0)
	for _, localization := range result.Localizations {
		locale := SanitizeTerminalText(localization.Locale)
		for _, item := range localization.Failures {
			rows = append(rows, []string{
				locale,
				SanitizeTerminalText(item.FileName),
				SanitizeTerminalText(item.FilePath),
				SanitizeTerminalText(item.Error),
			})
		}
	}
	return headers, rows
}

func appPreviewUploadResultMainRows(result *AppPreviewUploadResult) ([]string, [][]string) {
	headers := []string{"Localization ID", "Set ID", "Preview Type", "Dry Run"}
	rows := [][]string{{
		SanitizeTerminalText(result.VersionLocalizationID),
		SanitizeTerminalText(result.SetID),
		SanitizeTerminalText(result.PreviewType),
		fmt.Sprintf("%t", result.DryRun),
	}}
	return headers, rows
}

func customProductPageScreenshotUploadResultMainRows(result *CustomProductPageScreenshotUploadResult) ([]string, [][]string) {
	headers := []string{"Localization ID", "Set ID", "Display Type"}
	rows := [][]string{{
		SanitizeTerminalText(result.CustomProductPageLocalizationID),
		SanitizeTerminalText(result.SetID),
		SanitizeTerminalText(result.DisplayType),
	}}
	return headers, rows
}

func experimentTreatmentLocalizationScreenshotUploadResultMainRows(result *ExperimentTreatmentLocalizationScreenshotUploadResult) ([]string, [][]string) {
	headers := []string{"Localization ID", "Set ID", "Display Type"}
	rows := [][]string{{
		SanitizeTerminalText(result.ExperimentTreatmentLocalizationID),
		SanitizeTerminalText(result.SetID),
		SanitizeTerminalText(result.DisplayType),
	}}
	return headers, rows
}

func customProductPagePreviewUploadResultMainRows(result *CustomProductPagePreviewUploadResult) ([]string, [][]string) {
	headers := []string{"Localization ID", "Set ID", "Preview Type"}
	rows := [][]string{{
		SanitizeTerminalText(result.CustomProductPageLocalizationID),
		SanitizeTerminalText(result.SetID),
		SanitizeTerminalText(result.PreviewType),
	}}
	return headers, rows
}

func assetUploadResultItemRows(results []AssetUploadResultItem) ([]string, [][]string) {
	headers := []string{"File Name", "Asset ID", "State"}
	rows := make([][]string, 0, len(results))
	for _, item := range results {
		rows = append(rows, []string{
			SanitizeTerminalText(item.FileName),
			SanitizeTerminalText(item.AssetID),
			SanitizeTerminalText(assetUploadItemState(item)),
		})
	}
	return headers, rows
}

func assetDeliveryStateValue(state *AssetDeliveryState) string {
	if state == nil {
		return ""
	}
	return state.State
}

func assetUploadItemState(item AssetUploadResultItem) string {
	if item.State == "" && item.Skipped {
		return "skipped"
	}
	return item.State
}

func summarizeAssetUploadStates(results []AssetUploadResultItem) string {
	if len(results) == 0 {
		return "n/a"
	}

	counts := make(map[string]int)
	states := make([]string, 0, len(results))
	for _, item := range results {
		state := SanitizeTerminalText(assetUploadItemState(item))
		if state == "" {
			state = "uploaded"
		}
		if _, ok := counts[state]; !ok {
			states = append(states, state)
		}
		counts[state]++
	}

	sort.Strings(states)

	parts := make([]string, 0, len(states))
	for _, state := range states {
		parts = append(parts, fmt.Sprintf("%s=%d", state, counts[state]))
	}
	return strings.Join(parts, ", ")
}

func assetUploadFailureItemRows(results []AssetUploadFailureItem) ([]string, [][]string) {
	headers := []string{"File Name", "File Path", "Error"}
	rows := make([][]string, 0, len(results))
	for _, item := range results {
		rows = append(rows, []string{
			SanitizeTerminalText(item.FileName),
			SanitizeTerminalText(item.FilePath),
			SanitizeTerminalText(item.Error),
		})
	}
	return headers, rows
}

func screenshotSizesRows(result *ScreenshotSizesResult) ([]string, [][]string) {
	headers := []string{"Display Type", "Family", "Dimensions"}
	rows := make([][]string, 0, len(result.Sizes))
	for _, item := range result.Sizes {
		rows = append(rows, []string{
			SanitizeTerminalText(item.DisplayType),
			SanitizeTerminalText(item.Family),
			formatScreenshotDimensions(item.Dimensions),
		})
	}
	return headers, rows
}

func assetDeleteResultRows(result *AssetDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{SanitizeTerminalText(result.ID), fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}
