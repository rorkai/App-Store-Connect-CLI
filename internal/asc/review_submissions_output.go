package asc

import (
	"fmt"
	"os"
	"strconv"
)

func printReviewSubmissionsTable(resp *ReviewSubmissionsResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printReviewSubmissionsMarkdown(resp *ReviewSubmissionsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | State | Platform | Submitted Date | App ID | Items |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		appID := reviewSubmissionAppID(item.Relationships)
		itemCount := reviewSubmissionItemCount(item.Relationships)
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(string(item.Attributes.SubmissionState)),
			escapeMarkdown(string(item.Attributes.Platform)),
			escapeMarkdown(item.Attributes.SubmittedDate),
			escapeMarkdown(appID),
			escapeMarkdown(itemCount),
		)
	}
	return nil
}

func printReviewSubmissionItemsTable(resp *ReviewSubmissionItemsResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printReviewSubmissionItemsMarkdown(resp *ReviewSubmissionItemsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | State | Item Type | Item ID | Submission ID |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		itemType, itemID := reviewSubmissionItemTarget(item.Relationships)
		submissionID := reviewSubmissionItemSubmissionID(item.Relationships)
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.State),
			escapeMarkdown(itemType),
			escapeMarkdown(itemID),
			escapeMarkdown(submissionID),
		)
	}
	return nil
}

func printReviewSubmissionItemDeleteResultTable(result *ReviewSubmissionItemDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printReviewSubmissionItemDeleteResultMarkdown(result *ReviewSubmissionItemDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
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
