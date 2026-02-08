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

func backgroundAssetVersionStateRows(id string, state string) ([]string, [][]string) {
	headers := []string{"ID", "State"}
	rows := [][]string{{id, state}}
	return headers, rows
}
