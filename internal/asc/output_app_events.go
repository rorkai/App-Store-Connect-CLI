package asc

import "fmt"

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

func appEventsRows(resp *AppEventsResponse) ([]string, [][]string) {
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
	return headers, rows
}

func printAppEventsTable(resp *AppEventsResponse) error {
	h, r := appEventsRows(resp)
	RenderTable(h, r)
	return nil
}

func printAppEventsMarkdown(resp *AppEventsResponse) error {
	h, r := appEventsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func appEventLocalizationsRows(resp *AppEventLocalizationsResponse) ([]string, [][]string) {
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
	return headers, rows
}

func printAppEventLocalizationsTable(resp *AppEventLocalizationsResponse) error {
	h, r := appEventLocalizationsRows(resp)
	RenderTable(h, r)
	return nil
}

func printAppEventLocalizationsMarkdown(resp *AppEventLocalizationsResponse) error {
	h, r := appEventLocalizationsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func appEventScreenshotsRows(resp *AppEventScreenshotsResponse) ([]string, [][]string) {
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
	return headers, rows
}

func printAppEventScreenshotsTable(resp *AppEventScreenshotsResponse) error {
	h, r := appEventScreenshotsRows(resp)
	RenderTable(h, r)
	return nil
}

func printAppEventScreenshotsMarkdown(resp *AppEventScreenshotsResponse) error {
	h, r := appEventScreenshotsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func appEventVideoClipsRows(resp *AppEventVideoClipsResponse) ([]string, [][]string) {
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
	return headers, rows
}

func printAppEventVideoClipsTable(resp *AppEventVideoClipsResponse) error {
	h, r := appEventVideoClipsRows(resp)
	RenderTable(h, r)
	return nil
}

func printAppEventVideoClipsMarkdown(resp *AppEventVideoClipsResponse) error {
	h, r := appEventVideoClipsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func appEventDeleteResultRows(result *AppEventDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printAppEventDeleteResultTable(result *AppEventDeleteResult) error {
	h, r := appEventDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printAppEventDeleteResultMarkdown(result *AppEventDeleteResult) error {
	h, r := appEventDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func appEventLocalizationDeleteResultRows(result *AppEventLocalizationDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printAppEventLocalizationDeleteResultTable(result *AppEventLocalizationDeleteResult) error {
	h, r := appEventLocalizationDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printAppEventLocalizationDeleteResultMarkdown(result *AppEventLocalizationDeleteResult) error {
	h, r := appEventLocalizationDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func appEventSubmissionResultRows(result *AppEventSubmissionResult) ([]string, [][]string) {
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
	return headers, rows
}

func printAppEventSubmissionResultTable(result *AppEventSubmissionResult) error {
	h, r := appEventSubmissionResultRows(result)
	RenderTable(h, r)
	return nil
}

func printAppEventSubmissionResultMarkdown(result *AppEventSubmissionResult) error {
	h, r := appEventSubmissionResultRows(result)
	RenderMarkdown(h, r)
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
