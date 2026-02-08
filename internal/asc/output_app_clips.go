package asc

import (
	"fmt"
	"os"
	"strings"
)

func printAppClipsTable(resp *AppClipsResponse) error {
	headers := []string{"ID", "Bundle ID"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{item.ID, item.Attributes.BundleID})
	}
	RenderTable(headers, rows)
	return nil
}

func printAppClipsMarkdown(resp *AppClipsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Bundle ID |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.BundleID),
		)
	}
	return nil
}

func printAppClipDefaultExperiencesTable(resp *AppClipDefaultExperiencesResponse) error {
	headers := []string{"ID", "Action"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{item.ID, string(item.Attributes.Action)})
	}
	RenderTable(headers, rows)
	return nil
}

func printAppClipDefaultExperiencesMarkdown(resp *AppClipDefaultExperiencesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Action |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(string(item.Attributes.Action)),
		)
	}
	return nil
}

func printAppClipDefaultExperienceLocalizationsTable(resp *AppClipDefaultExperienceLocalizationsResponse) error {
	headers := []string{"ID", "Locale", "Subtitle"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			item.Attributes.Locale,
			compactWhitespace(item.Attributes.Subtitle),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printAppClipDefaultExperienceLocalizationsMarkdown(resp *AppClipDefaultExperienceLocalizationsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Locale | Subtitle |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.Locale),
			escapeMarkdown(item.Attributes.Subtitle),
		)
	}
	return nil
}

func printAppClipAdvancedExperiencesTable(resp *AppClipAdvancedExperiencesResponse) error {
	headers := []string{"ID", "Action", "Status", "Business Category", "Default Language", "Powered By", "Link"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			string(item.Attributes.Action),
			item.Attributes.Status,
			string(item.Attributes.BusinessCategory),
			string(item.Attributes.DefaultLanguage),
			fmt.Sprintf("%t", item.Attributes.IsPoweredBy),
			item.Attributes.Link,
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printAppClipAdvancedExperiencesMarkdown(resp *AppClipAdvancedExperiencesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Action | Status | Business Category | Default Language | Powered By | Link |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s | %t | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(string(item.Attributes.Action)),
			escapeMarkdown(item.Attributes.Status),
			escapeMarkdown(string(item.Attributes.BusinessCategory)),
			escapeMarkdown(string(item.Attributes.DefaultLanguage)),
			item.Attributes.IsPoweredBy,
			escapeMarkdown(item.Attributes.Link),
		)
	}
	return nil
}

func printBetaAppClipInvocationLocalizationsTable(resp *BetaAppClipInvocationLocalizationsResponse) error {
	headers := []string{"ID", "Locale", "Title"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			item.Attributes.Locale,
			compactWhitespace(item.Attributes.Title),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printBetaAppClipInvocationLocalizationsMarkdown(resp *BetaAppClipInvocationLocalizationsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Locale | Title |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.Locale),
			escapeMarkdown(item.Attributes.Title),
		)
	}
	return nil
}

func printAppClipAdvancedExperienceImageUploadResultTable(result *AppClipAdvancedExperienceImageUploadResult) error {
	headers := []string{"ID", "Experience ID", "File Name", "File Size", "State", "Uploaded"}
	rows := [][]string{{
		result.ID,
		result.ExperienceID,
		result.FileName,
		fmt.Sprintf("%d", result.FileSize),
		result.AssetDeliveryState,
		fmt.Sprintf("%t", result.Uploaded),
	}}
	RenderTable(headers, rows)
	return nil
}

func printAppClipAdvancedExperienceImageUploadResultMarkdown(result *AppClipAdvancedExperienceImageUploadResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Experience ID | File Name | File Size | State | Uploaded |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %s | %s | %d | %s | %t |\n",
		escapeMarkdown(result.ID),
		escapeMarkdown(result.ExperienceID),
		escapeMarkdown(result.FileName),
		result.FileSize,
		escapeMarkdown(result.AssetDeliveryState),
		result.Uploaded,
	)
	return nil
}

func printAppClipAdvancedExperienceImageTable(resp *AppClipAdvancedExperienceImageResponse) error {
	headers := []string{"ID", "File Name", "File Size", "State"}
	state := ""
	if resp.Data.Attributes.AssetDeliveryState != nil {
		state = resp.Data.Attributes.AssetDeliveryState.State
	}
	rows := [][]string{{
		resp.Data.ID,
		resp.Data.Attributes.FileName,
		fmt.Sprintf("%d", resp.Data.Attributes.FileSize),
		state,
	}}
	RenderTable(headers, rows)
	return nil
}

func printAppClipAdvancedExperienceImageMarkdown(resp *AppClipAdvancedExperienceImageResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | File Name | File Size | State |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- |")
	state := ""
	if resp.Data.Attributes.AssetDeliveryState != nil {
		state = resp.Data.Attributes.AssetDeliveryState.State
	}
	fmt.Fprintf(os.Stdout, "| %s | %s | %d | %s |\n",
		escapeMarkdown(resp.Data.ID),
		escapeMarkdown(resp.Data.Attributes.FileName),
		resp.Data.Attributes.FileSize,
		escapeMarkdown(state),
	)
	return nil
}

func printAppClipHeaderImageUploadResultTable(result *AppClipHeaderImageUploadResult) error {
	headers := []string{"ID", "Localization ID", "File Name", "File Size", "State", "Uploaded"}
	rows := [][]string{{
		result.ID,
		result.LocalizationID,
		result.FileName,
		fmt.Sprintf("%d", result.FileSize),
		result.AssetDeliveryState,
		fmt.Sprintf("%t", result.Uploaded),
	}}
	RenderTable(headers, rows)
	return nil
}

func printAppClipHeaderImageUploadResultMarkdown(result *AppClipHeaderImageUploadResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Localization ID | File Name | File Size | State | Uploaded |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %s | %s | %d | %s | %t |\n",
		escapeMarkdown(result.ID),
		escapeMarkdown(result.LocalizationID),
		escapeMarkdown(result.FileName),
		result.FileSize,
		escapeMarkdown(result.AssetDeliveryState),
		result.Uploaded,
	)
	return nil
}

func printAppClipHeaderImageTable(resp *AppClipHeaderImageResponse) error {
	headers := []string{"ID", "File Name", "File Size", "State"}
	state := ""
	if resp.Data.Attributes.AssetDeliveryState != nil {
		state = resp.Data.Attributes.AssetDeliveryState.State
	}
	rows := [][]string{{
		resp.Data.ID,
		resp.Data.Attributes.FileName,
		fmt.Sprintf("%d", resp.Data.Attributes.FileSize),
		state,
	}}
	RenderTable(headers, rows)
	return nil
}

func printAppClipHeaderImageMarkdown(resp *AppClipHeaderImageResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | File Name | File Size | State |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- |")
	state := ""
	if resp.Data.Attributes.AssetDeliveryState != nil {
		state = resp.Data.Attributes.AssetDeliveryState.State
	}
	fmt.Fprintf(os.Stdout, "| %s | %s | %d | %s |\n",
		escapeMarkdown(resp.Data.ID),
		escapeMarkdown(resp.Data.Attributes.FileName),
		resp.Data.Attributes.FileSize,
		escapeMarkdown(state),
	)
	return nil
}

func printAppClipDefaultExperienceDeleteResultTable(result *AppClipDefaultExperienceDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printAppClipDefaultExperienceDeleteResultMarkdown(result *AppClipDefaultExperienceDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}

func printAppClipDefaultExperienceLocalizationDeleteResultTable(result *AppClipDefaultExperienceLocalizationDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printAppClipDefaultExperienceLocalizationDeleteResultMarkdown(result *AppClipDefaultExperienceLocalizationDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}

func printAppClipAdvancedExperienceDeleteResultTable(result *AppClipAdvancedExperienceDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printAppClipAdvancedExperienceDeleteResultMarkdown(result *AppClipAdvancedExperienceDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}

func printAppClipAdvancedExperienceImageDeleteResultTable(result *AppClipAdvancedExperienceImageDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printAppClipAdvancedExperienceImageDeleteResultMarkdown(result *AppClipAdvancedExperienceImageDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}

func printAppClipHeaderImageDeleteResultTable(result *AppClipHeaderImageDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printAppClipHeaderImageDeleteResultMarkdown(result *AppClipHeaderImageDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}

func printBetaAppClipInvocationDeleteResultTable(result *BetaAppClipInvocationDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printBetaAppClipInvocationDeleteResultMarkdown(result *BetaAppClipInvocationDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}

func printBetaAppClipInvocationLocalizationDeleteResultTable(result *BetaAppClipInvocationLocalizationDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printBetaAppClipInvocationLocalizationDeleteResultMarkdown(result *BetaAppClipInvocationLocalizationDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}

func printAppClipAppStoreReviewDetailTable(resp *AppClipAppStoreReviewDetailResponse) error {
	headers := []string{"ID", "Invocation URLs"}
	urls := strings.Join(resp.Data.Attributes.InvocationURLs, ", ")
	rows := [][]string{{resp.Data.ID, compactWhitespace(urls)}}
	RenderTable(headers, rows)
	return nil
}

func printAppClipAppStoreReviewDetailMarkdown(resp *AppClipAppStoreReviewDetailResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Invocation URLs |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	urls := strings.Join(resp.Data.Attributes.InvocationURLs, ", ")
	fmt.Fprintf(os.Stdout, "| %s | %s |\n",
		escapeMarkdown(resp.Data.ID),
		escapeMarkdown(urls),
	)
	return nil
}
