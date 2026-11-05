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
			SanitizeTerminalText(item.ID),
			SanitizeTerminalText(string(item.Attributes.SubmissionState)),
			SanitizeTerminalText(string(item.Attributes.Platform)),
			SanitizeTerminalText(item.Attributes.SubmittedDate),
			SanitizeTerminalText(appID),
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
			SanitizeTerminalText(item.ID),
			SanitizeTerminalText(item.Attributes.State),
			SanitizeTerminalText(itemType),
			SanitizeTerminalText(itemID),
			SanitizeTerminalText(submissionID),
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

	for _, relationship := range []*Relationship{
		rel.InAppPurchaseVersion,
		rel.SubscriptionVersion,
		rel.SubscriptionGroupVersion,
		rel.RetentionMessageVersion,
		rel.AppStoreVersion,
		rel.AppCustomProductPageVersion,
		rel.AppAssetLibraryImage,
		rel.AppAssetLibraryVideo,
		rel.AppEvent,
		rel.AppStoreVersionExperiment,
		rel.AppStoreVersionExperimentV2,
		rel.BackgroundAssetVersion,
		rel.GameCenterAchievementVersion,
		rel.GameCenterActivityVersion,
		rel.GameCenterChallengeVersion,
		rel.GameCenterLeaderboardSetVersion,
		rel.GameCenterLeaderboardVersion,
	} {
		if relationship == nil || relationship.Data.ID == "" {
			continue
		}
		return string(relationship.Data.Type), relationship.Data.ID
	}
	return "", ""
}

// reviewSubmissionItemSubmissionID preserves the existing table column even though
// API 4.4.1 no longer exposes a reviewSubmission relationship on item responses.
func reviewSubmissionItemSubmissionID(_ *ReviewSubmissionItemRelationships) string {
	return ""
}
