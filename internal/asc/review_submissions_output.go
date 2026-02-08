package asc

import (
	"fmt"
	"strconv"
)

func reviewSubmissionsRows(resp *ReviewSubmissionsResponse) ([]string, [][]string) {
	headers := []string{"ID", "State", "Platform", "Submitted Date", "App ID", "Items"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		appID := reviewSubmissionAppID(item.Relationships)
		itemCount := reviewSubmissionItemCount(item.Relationships)
		rows = append(rows, []string{
			item.ID,
			sanitizeTerminal(string(item.Attributes.SubmissionState)),
			sanitizeTerminal(string(item.Attributes.Platform)),
			sanitizeTerminal(item.Attributes.SubmittedDate),
			sanitizeTerminal(appID),
			itemCount,
		})
	}
	return headers, rows
}

func reviewSubmissionItemsRows(resp *ReviewSubmissionItemsResponse) ([]string, [][]string) {
	headers := []string{"ID", "State", "Item Type", "Item ID", "Submission ID"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		itemType, itemID := reviewSubmissionItemTarget(item.Relationships)
		submissionID := reviewSubmissionItemSubmissionID(item.Relationships)
		rows = append(rows, []string{
			item.ID,
			sanitizeTerminal(item.Attributes.State),
			sanitizeTerminal(itemType),
			sanitizeTerminal(itemID),
			sanitizeTerminal(submissionID),
		})
	}
	return headers, rows
}

func reviewSubmissionItemDeleteResultRows(result *ReviewSubmissionItemDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func reviewSubmissionAppID(rel *ReviewSubmissionRelationships) string {
	if rel == nil || rel.App == nil {
		return ""
	}
	return rel.App.Data.ID
}

func reviewSubmissionItemCount(rel *ReviewSubmissionRelationships) string {
	if rel == nil || rel.Items == nil {
		return ""
	}
	return strconv.Itoa(len(rel.Items.Data))
}

func reviewSubmissionItemTarget(rel *ReviewSubmissionItemRelationships) (string, string) {
	if rel == nil {
		return "", ""
	}
	if rel.AppStoreVersion != nil && rel.AppStoreVersion.Data.ID != "" {
		return string(rel.AppStoreVersion.Data.Type), rel.AppStoreVersion.Data.ID
	}
	if rel.AppCustomProductPage != nil && rel.AppCustomProductPage.Data.ID != "" {
		return string(rel.AppCustomProductPage.Data.Type), rel.AppCustomProductPage.Data.ID
	}
	if rel.AppEvent != nil && rel.AppEvent.Data.ID != "" {
		return string(rel.AppEvent.Data.Type), rel.AppEvent.Data.ID
	}
	if rel.AppStoreVersionExperiment != nil && rel.AppStoreVersionExperiment.Data.ID != "" {
		return string(rel.AppStoreVersionExperiment.Data.Type), rel.AppStoreVersionExperiment.Data.ID
	}
	if rel.AppStoreVersionExperimentTreatment != nil && rel.AppStoreVersionExperimentTreatment.Data.ID != "" {
		return string(rel.AppStoreVersionExperimentTreatment.Data.Type), rel.AppStoreVersionExperimentTreatment.Data.ID
	}
	return "", ""
}

func reviewSubmissionItemSubmissionID(rel *ReviewSubmissionItemRelationships) string {
	if rel == nil || rel.ReviewSubmission == nil {
		return ""
	}
	return rel.ReviewSubmission.Data.ID
}
