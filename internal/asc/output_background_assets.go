package asc

import (
	"fmt"
	"strings"
)

func backgroundAssetsRows(resp *BackgroundAssetsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Asset Pack Identifier", "Archived", "Created Date"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.AssetPackIdentifier),
			fmt.Sprintf("%t", item.Attributes.Archived),
			item.Attributes.CreatedDate,
		})
	}
	return headers, rows
}

func printBackgroundAssetsTable(resp *BackgroundAssetsResponse) error {
	h, r := backgroundAssetsRows(resp)
	RenderTable(h, r)
	return nil
}

func printBackgroundAssetsMarkdown(resp *BackgroundAssetsResponse) error {
	h, r := backgroundAssetsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func backgroundAssetVersionsRows(resp *BackgroundAssetVersionsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Version", "State", "Platforms", "Created Date"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.Version),
			compactWhitespace(item.Attributes.State),
			formatPlatforms(item.Attributes.Platforms),
			item.Attributes.CreatedDate,
		})
	}
	return headers, rows
}

func printBackgroundAssetVersionsTable(resp *BackgroundAssetVersionsResponse) error {
	h, r := backgroundAssetVersionsRows(resp)
	RenderTable(h, r)
	return nil
}

func printBackgroundAssetVersionsMarkdown(resp *BackgroundAssetVersionsResponse) error {
	h, r := backgroundAssetVersionsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func backgroundAssetUploadFilesRows(resp *BackgroundAssetUploadFilesResponse) ([]string, [][]string) {
	headers := []string{"ID", "File Name", "Asset Type", "File Size", "State"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		state := ""
		if item.Attributes.AssetDeliveryState != nil && item.Attributes.AssetDeliveryState.State != nil {
			state = strings.TrimSpace(*item.Attributes.AssetDeliveryState.State)
		}
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.FileName),
			string(item.Attributes.AssetType),
			fmt.Sprintf("%d", item.Attributes.FileSize),
			state,
		})
	}
	return headers, rows
}

func printBackgroundAssetUploadFilesTable(resp *BackgroundAssetUploadFilesResponse) error {
	h, r := backgroundAssetUploadFilesRows(resp)
	RenderTable(h, r)
	return nil
}

func printBackgroundAssetUploadFilesMarkdown(resp *BackgroundAssetUploadFilesResponse) error {
	h, r := backgroundAssetUploadFilesRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func backgroundAssetVersionStateRows(id string, state string) ([]string, [][]string) {
	headers := []string{"ID", "State"}
	rows := [][]string{{id, state}}
	return headers, rows
}

func printBackgroundAssetVersionStateTable(id string, state string) error {
	h, r := backgroundAssetVersionStateRows(id, state)
	RenderTable(h, r)
	return nil
}

func printBackgroundAssetVersionStateMarkdown(id string, state string) error {
	h, r := backgroundAssetVersionStateRows(id, state)
	RenderMarkdown(h, r)
	return nil
}

func printBackgroundAssetVersionAppStoreReleaseTable(resp *BackgroundAssetVersionAppStoreReleaseResponse) error {
	return printBackgroundAssetVersionStateTable(resp.Data.ID, resp.Data.Attributes.State)
}

func printBackgroundAssetVersionAppStoreReleaseMarkdown(resp *BackgroundAssetVersionAppStoreReleaseResponse) error {
	return printBackgroundAssetVersionStateMarkdown(resp.Data.ID, resp.Data.Attributes.State)
}

func printBackgroundAssetVersionExternalBetaReleaseTable(resp *BackgroundAssetVersionExternalBetaReleaseResponse) error {
	return printBackgroundAssetVersionStateTable(resp.Data.ID, resp.Data.Attributes.State)
}

func printBackgroundAssetVersionExternalBetaReleaseMarkdown(resp *BackgroundAssetVersionExternalBetaReleaseResponse) error {
	return printBackgroundAssetVersionStateMarkdown(resp.Data.ID, resp.Data.Attributes.State)
}

func printBackgroundAssetVersionInternalBetaReleaseTable(resp *BackgroundAssetVersionInternalBetaReleaseResponse) error {
	return printBackgroundAssetVersionStateTable(resp.Data.ID, resp.Data.Attributes.State)
}

func printBackgroundAssetVersionInternalBetaReleaseMarkdown(resp *BackgroundAssetVersionInternalBetaReleaseResponse) error {
	return printBackgroundAssetVersionStateMarkdown(resp.Data.ID, resp.Data.Attributes.State)
}
