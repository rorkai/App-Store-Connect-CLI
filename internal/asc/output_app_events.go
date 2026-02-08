package asc

import (
	"fmt"
	"os"
)

// AppEventDeleteResult represents CLI output for app event deletions.
type AppEventDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// AppEventLocalizationDeleteResult represents CLI output for localization deletions.
type AppEventLocalizationDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// AppEventSubmissionResult represents CLI output for app event submissions.
type AppEventSubmissionResult struct {
	SubmissionID  string  `json:"submissionId"`
	ItemID        string  `json:"itemId,omitempty"`
	EventID       string  `json:"eventId"`
	AppID         string  `json:"appId"`
	Platform      string  `json:"platform,omitempty"`
	SubmittedDate *string `json:"submittedDate,omitempty"`
}

func printAppEventsTable(resp *AppEventsResponse) error {
	headers := []string{"ID", "Reference Name", "Type", "State", "Primary Locale", "Priority"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		attrs := item.Attributes
		rows = append(rows, []string{
			sanitizeTerminal(item.ID),
			compactWhitespace(attrs.ReferenceName),
			sanitizeTerminal(attrs.Badge),
			sanitizeTerminal(attrs.EventState),
			sanitizeTerminal(attrs.PrimaryLocale),
			sanitizeTerminal(attrs.Priority),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printAppEventsMarkdown(resp *AppEventsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Reference Name | Type | State | Primary Locale | Priority |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		attrs := item.Attributes
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(attrs.ReferenceName),
			escapeMarkdown(attrs.Badge),
			escapeMarkdown(attrs.EventState),
			escapeMarkdown(attrs.PrimaryLocale),
			escapeMarkdown(attrs.Priority),
		)
	}
	return nil
}

func printAppEventLocalizationsTable(resp *AppEventLocalizationsResponse) error {
	headers := []string{"ID", "Locale", "Name", "Short Description", "Long Description"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		attrs := item.Attributes
		rows = append(rows, []string{
			sanitizeTerminal(item.ID),
			sanitizeTerminal(attrs.Locale),
			compactWhitespace(attrs.Name),
			compactWhitespace(attrs.ShortDescription),
			compactWhitespace(attrs.LongDescription),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printAppEventLocalizationsMarkdown(resp *AppEventLocalizationsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Locale | Name | Short Description | Long Description |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		attrs := item.Attributes
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(attrs.Locale),
			escapeMarkdown(attrs.Name),
			escapeMarkdown(attrs.ShortDescription),
			escapeMarkdown(attrs.LongDescription),
		)
	}
	return nil
}

func printAppEventScreenshotsTable(resp *AppEventScreenshotsResponse) error {
	headers := []string{"ID", "File Name", "File Size", "Asset Type", "State"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		attrs := item.Attributes
		rows = append(rows, []string{
			sanitizeTerminal(item.ID),
			sanitizeTerminal(attrs.FileName),
			fmt.Sprintf("%d", attrs.FileSize),
			sanitizeTerminal(attrs.AppEventAssetType),
			sanitizeTerminal(formatAppMediaAssetState(attrs.AssetDeliveryState)),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printAppEventScreenshotsMarkdown(resp *AppEventScreenshotsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | File Name | File Size | Asset Type | State |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		attrs := item.Attributes
		fmt.Fprintf(os.Stdout, "| %s | %s | %d | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(attrs.FileName),
			attrs.FileSize,
			escapeMarkdown(attrs.AppEventAssetType),
			escapeMarkdown(formatAppMediaAssetState(attrs.AssetDeliveryState)),
		)
	}
	return nil
}

func printAppEventVideoClipsTable(resp *AppEventVideoClipsResponse) error {
	headers := []string{"ID", "File Name", "File Size", "Asset Type", "State"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		attrs := item.Attributes
		rows = append(rows, []string{
			sanitizeTerminal(item.ID),
			sanitizeTerminal(attrs.FileName),
			fmt.Sprintf("%d", attrs.FileSize),
			sanitizeTerminal(attrs.AppEventAssetType),
			sanitizeTerminal(formatAppMediaVideoState(attrs.VideoDeliveryState, attrs.AssetDeliveryState)),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printAppEventVideoClipsMarkdown(resp *AppEventVideoClipsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | File Name | File Size | Asset Type | State |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		attrs := item.Attributes
		fmt.Fprintf(os.Stdout, "| %s | %s | %d | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(attrs.FileName),
			attrs.FileSize,
			escapeMarkdown(attrs.AppEventAssetType),
			escapeMarkdown(formatAppMediaVideoState(attrs.VideoDeliveryState, attrs.AssetDeliveryState)),
		)
	}
	return nil
}

func printAppEventDeleteResultTable(result *AppEventDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printAppEventDeleteResultMarkdown(result *AppEventDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}

func printAppEventLocalizationDeleteResultTable(result *AppEventLocalizationDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printAppEventLocalizationDeleteResultMarkdown(result *AppEventLocalizationDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}

func printAppEventSubmissionResultTable(result *AppEventSubmissionResult) error {
	headers := []string{"Submission ID", "Item ID", "Event ID", "App ID", "Platform", "Submitted Date"}
	submittedDate := ""
	if result.SubmittedDate != nil {
		submittedDate = *result.SubmittedDate
	}
	rows := [][]string{{
		sanitizeTerminal(result.SubmissionID),
		sanitizeTerminal(result.ItemID),
		sanitizeTerminal(result.EventID),
		sanitizeTerminal(result.AppID),
		sanitizeTerminal(result.Platform),
		sanitizeTerminal(submittedDate),
	}}
	RenderTable(headers, rows)
	return nil
}

func printAppEventSubmissionResultMarkdown(result *AppEventSubmissionResult) error {
	fmt.Fprintln(os.Stdout, "| Submission ID | Item ID | Event ID | App ID | Platform | Submitted Date |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- |")
	submittedDate := ""
	if result.SubmittedDate != nil {
		submittedDate = *result.SubmittedDate
	}
	fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s | %s |\n",
		escapeMarkdown(result.SubmissionID),
		escapeMarkdown(result.ItemID),
		escapeMarkdown(result.EventID),
		escapeMarkdown(result.AppID),
		escapeMarkdown(result.Platform),
		escapeMarkdown(submittedDate),
	)
	return nil
}

func formatAppMediaAssetState(state *AppMediaAssetState) string {
	if state == nil || state.State == nil {
		return ""
	}
	return *state.State
}

func formatAppMediaVideoState(videoState *AppMediaVideoState, assetState *AppMediaAssetState) string {
	if videoState != nil && videoState.State != nil {
		return *videoState.State
	}
	return formatAppMediaAssetState(assetState)
}
