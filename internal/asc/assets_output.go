package asc

import "fmt"

// AppScreenshotSetWithScreenshots groups a set with its screenshots.
type AppScreenshotSetWithScreenshots struct {
	Set         Resource[AppScreenshotSetAttributes] `json:"set"`
	Screenshots []Resource[AppScreenshotAttributes]  `json:"screenshots"`
}

// AppScreenshotListResult represents screenshot list output by localization.
type AppScreenshotListResult struct {
	VersionLocalizationID string                            `json:"versionLocalizationId"`
	Sets                  []AppScreenshotSetWithScreenshots `json:"sets"`
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
}

// AppScreenshotUploadResult represents screenshot upload output.
type AppScreenshotUploadResult struct {
	VersionLocalizationID string                  `json:"versionLocalizationId"`
	SetID                 string                  `json:"setId"`
	DisplayType           string                  `json:"displayType"`
	Results               []AssetUploadResultItem `json:"results"`
}

// AppPreviewUploadResult represents preview upload output.
type AppPreviewUploadResult struct {
	VersionLocalizationID string                  `json:"versionLocalizationId"`
	SetID                 string                  `json:"setId"`
	PreviewType           string                  `json:"previewType"`
	Results               []AssetUploadResultItem `json:"results"`
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
		rows = append(rows, []string{item.ID, item.Attributes.ScreenshotDisplayType})
	}
	return headers, rows
}

func appScreenshotsRows(resp *AppScreenshotsResponse) ([]string, [][]string) {
	headers := []string{"ID", "File Name", "File Size", "State"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		state := ""
		if item.Attributes.AssetDeliveryState != nil {
			state = item.Attributes.AssetDeliveryState.State
		}
		rows = append(rows, []string{
			item.ID,
			item.Attributes.FileName,
			fmt.Sprintf("%d", item.Attributes.FileSize),
			state,
		})
	}
	return headers, rows
}

func appPreviewSetsRows(resp *AppPreviewSetsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Preview Type"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{item.ID, item.Attributes.PreviewType})
	}
	return headers, rows
}

func appPreviewsRows(resp *AppPreviewsResponse) ([]string, [][]string) {
	headers := []string{"ID", "File Name", "File Size", "State"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		state := ""
		if item.Attributes.AssetDeliveryState != nil {
			state = item.Attributes.AssetDeliveryState.State
		}
		rows = append(rows, []string{
			item.ID,
			item.Attributes.FileName,
			fmt.Sprintf("%d", item.Attributes.FileSize),
			state,
		})
	}
	return headers, rows
}

func appScreenshotListResultRows(result *AppScreenshotListResult) ([]string, [][]string) {
	headers := []string{"Set ID", "Display Type", "Screenshot ID", "File Name", "File Size", "State"}
	var rows [][]string
	for _, set := range result.Sets {
		displayType := set.Set.Attributes.ScreenshotDisplayType
		if len(set.Screenshots) == 0 {
			rows = append(rows, []string{set.Set.ID, displayType, "", "", "", ""})
			continue
		}
		for _, item := range set.Screenshots {
			state := ""
			if item.Attributes.AssetDeliveryState != nil {
				state = item.Attributes.AssetDeliveryState.State
			}
			rows = append(rows, []string{
				set.Set.ID,
				displayType,
				item.ID,
				item.Attributes.FileName,
				fmt.Sprintf("%d", item.Attributes.FileSize),
				state,
			})
		}
	}
	return headers, rows
}

func appPreviewListResultRows(result *AppPreviewListResult) ([]string, [][]string) {
	headers := []string{"Set ID", "Preview Type", "Preview ID", "File Name", "File Size", "State"}
	var rows [][]string
	for _, set := range result.Sets {
		previewType := set.Set.Attributes.PreviewType
		if len(set.Previews) == 0 {
			rows = append(rows, []string{set.Set.ID, previewType, "", "", "", ""})
			continue
		}
		for _, item := range set.Previews {
			state := ""
			if item.Attributes.AssetDeliveryState != nil {
				state = item.Attributes.AssetDeliveryState.State
			}
			rows = append(rows, []string{
				set.Set.ID,
				previewType,
				item.ID,
				item.Attributes.FileName,
				fmt.Sprintf("%d", item.Attributes.FileSize),
				state,
			})
		}
	}
	return headers, rows
}

func appScreenshotUploadResultMainRows(result *AppScreenshotUploadResult) ([]string, [][]string) {
	headers := []string{"Localization ID", "Set ID", "Display Type"}
	rows := [][]string{{result.VersionLocalizationID, result.SetID, result.DisplayType}}
	return headers, rows
}

func appPreviewUploadResultMainRows(result *AppPreviewUploadResult) ([]string, [][]string) {
	headers := []string{"Localization ID", "Set ID", "Preview Type"}
	rows := [][]string{{result.VersionLocalizationID, result.SetID, result.PreviewType}}
	return headers, rows
}

func assetUploadResultItemRows(results []AssetUploadResultItem) ([]string, [][]string) {
	headers := []string{"File Name", "Asset ID", "State"}
	rows := make([][]string, 0, len(results))
	for _, item := range results {
		rows = append(rows, []string{item.FileName, item.AssetID, item.State})
	}
	return headers, rows
}

func assetDeleteResultRows(result *AssetDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}
