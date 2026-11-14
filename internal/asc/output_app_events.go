package asc

import "fmt"

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
			SanitizeTerminalText(item.ID),
			compactWhitespace(attrs.ReferenceName),
			SanitizeTerminalText(attrs.Badge),
			SanitizeTerminalText(attrs.EventState),
			SanitizeTerminalText(attrs.PrimaryLocale),
			SanitizeTerminalText(attrs.Priority),
		})
	}
	return headers, rows
}

func appEventLocalizationsRows(resp *AppEventLocalizationsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Locale", "Name", "Short Description", "Long Description"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		attrs := item.Attributes
		rows = append(rows, []string{
			SanitizeTerminalText(item.ID),
			SanitizeTerminalText(attrs.Locale),
			compactWhitespace(attrs.Name),
			compactWhitespace(attrs.ShortDescription),
			compactWhitespace(attrs.LongDescription),
		})
	}
	return headers, rows
}

func appEventScreenshotsRows(resp *AppEventScreenshotsResponse) ([]string, [][]string) {
	headers := []string{"ID", "File Name", "File Size", "Asset Type", "State"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		attrs := item.Attributes
		rows = append(rows, []string{
			SanitizeTerminalText(item.ID),
			SanitizeTerminalText(attrs.FileName),
			fmt.Sprintf("%d", attrs.FileSize),
			SanitizeTerminalText(attrs.AppEventAssetType),
			SanitizeTerminalText(formatAppMediaAssetState(attrs.AssetDeliveryState)),
		})
	}
	return headers, rows
}

func appEventVideoClipsRows(resp *AppEventVideoClipsResponse) ([]string, [][]string) {
	headers := []string{"ID", "File Name", "File Size", "Asset Type", "State"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		attrs := item.Attributes
		rows = append(rows, []string{
			SanitizeTerminalText(item.ID),
			SanitizeTerminalText(attrs.FileName),
			fmt.Sprintf("%d", attrs.FileSize),
			SanitizeTerminalText(attrs.AppEventAssetType),
			SanitizeTerminalText(formatAppMediaVideoState(attrs.VideoDeliveryState, attrs.AssetDeliveryState)),
		})
	}
	return headers, rows
}

func appEventSubmissionResultRows(result *AppEventSubmissionResult) ([]string, [][]string) {
	headers := []string{"Submission ID", "Item ID", "Event ID", "App ID", "Platform", "Submitted Date"}
	submittedDate := ""
	if result.SubmittedDate != nil {
		submittedDate = *result.SubmittedDate
	}
	rows := [][]string{{
		SanitizeTerminalText(result.SubmissionID),
		SanitizeTerminalText(result.ItemID),
		SanitizeTerminalText(result.EventID),
		SanitizeTerminalText(result.AppID),
		SanitizeTerminalText(result.Platform),
		SanitizeTerminalText(submittedDate),
	}}
	return headers, rows
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
