package asc

import (
	"fmt"
	"os"
	"strings"
)

func printBackgroundAssetsTable(resp *BackgroundAssetsResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printBackgroundAssetsMarkdown(resp *BackgroundAssetsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Asset Pack Identifier | Archived | Created Date |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %t | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.AssetPackIdentifier),
			item.Attributes.Archived,
			escapeMarkdown(item.Attributes.CreatedDate),
		)
	}
	return nil
}

func printBackgroundAssetVersionsTable(resp *BackgroundAssetVersionsResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printBackgroundAssetVersionsMarkdown(resp *BackgroundAssetVersionsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Version | State | Platforms | Created Date |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.Version),
			escapeMarkdown(item.Attributes.State),
			escapeMarkdown(formatPlatforms(item.Attributes.Platforms)),
			escapeMarkdown(item.Attributes.CreatedDate),
		)
	}
	return nil
}

func printBackgroundAssetUploadFilesTable(resp *BackgroundAssetUploadFilesResponse) error {
	headers := []string{"ID", "File Name", "Asset Type", "File Size", "State"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		state := ""
		if item.Attributes.AssetDeliveryState != nil && item.Attributes.AssetDeliveryState.State != nil {
			state = *item.Attributes.AssetDeliveryState.State
		}
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.FileName),
			string(item.Attributes.AssetType),
			fmt.Sprintf("%d", item.Attributes.FileSize),
			state,
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printBackgroundAssetUploadFilesMarkdown(resp *BackgroundAssetUploadFilesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | File Name | Asset Type | File Size | State |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		state := ""
		if item.Attributes.AssetDeliveryState != nil && item.Attributes.AssetDeliveryState.State != nil {
			state = *item.Attributes.AssetDeliveryState.State
		}
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %d | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.FileName),
			escapeMarkdown(string(item.Attributes.AssetType)),
			item.Attributes.FileSize,
			escapeMarkdown(strings.TrimSpace(state)),
		)
	}
	return nil
}

func printBackgroundAssetVersionStateTable(id string, state string) error {
	headers := []string{"ID", "State"}
	rows := [][]string{{id, state}}
	RenderTable(headers, rows)
	return nil
}

func printBackgroundAssetVersionStateMarkdown(id string, state string) error {
	fmt.Fprintln(os.Stdout, "| ID | State |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %s |\n",
		escapeMarkdown(id),
		escapeMarkdown(state),
	)
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
