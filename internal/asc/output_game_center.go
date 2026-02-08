package asc

import (
	"encoding/json"
	"fmt"
	"strings"
)

func gameCenterAchievementsRows(resp *GameCenterAchievementsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Reference Name", "Vendor ID", "Points", "Show Before Earned", "Repeatable", "Archived"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.ReferenceName),
			item.Attributes.VendorIdentifier,
			fmt.Sprintf("%d", item.Attributes.Points),
			fmt.Sprintf("%t", item.Attributes.ShowBeforeEarned),
			fmt.Sprintf("%t", item.Attributes.Repeatable),
			fmt.Sprintf("%t", item.Attributes.Archived),
		})
	}
	return headers, rows
}

func printGameCenterAchievementsTable(resp *GameCenterAchievementsResponse) error {
	h, r := gameCenterAchievementsRows(resp)
	RenderTable(h, r)
	return nil
}

func printGameCenterAchievementsMarkdown(resp *GameCenterAchievementsResponse) error {
	h, r := gameCenterAchievementsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterAchievementDeleteResultRows(result *GameCenterAchievementDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printGameCenterAchievementDeleteResultTable(result *GameCenterAchievementDeleteResult) error {
	h, r := gameCenterAchievementDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printGameCenterAchievementDeleteResultMarkdown(result *GameCenterAchievementDeleteResult) error {
	h, r := gameCenterAchievementDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterLeaderboardsRows(resp *GameCenterLeaderboardsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Reference Name", "Vendor ID", "Formatter", "Sort", "Submission Type", "Archived"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.ReferenceName),
			item.Attributes.VendorIdentifier,
			item.Attributes.DefaultFormatter,
			item.Attributes.ScoreSortType,
			item.Attributes.SubmissionType,
			fmt.Sprintf("%t", item.Attributes.Archived),
		})
	}
	return headers, rows
}

func printGameCenterLeaderboardsTable(resp *GameCenterLeaderboardsResponse) error {
	h, r := gameCenterLeaderboardsRows(resp)
	RenderTable(h, r)
	return nil
}

func printGameCenterLeaderboardsMarkdown(resp *GameCenterLeaderboardsResponse) error {
	h, r := gameCenterLeaderboardsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterLeaderboardDeleteResultRows(result *GameCenterLeaderboardDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printGameCenterLeaderboardDeleteResultTable(result *GameCenterLeaderboardDeleteResult) error {
	h, r := gameCenterLeaderboardDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printGameCenterLeaderboardDeleteResultMarkdown(result *GameCenterLeaderboardDeleteResult) error {
	h, r := gameCenterLeaderboardDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterLeaderboardSetsRows(resp *GameCenterLeaderboardSetsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Reference Name", "Vendor ID"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.ReferenceName),
			item.Attributes.VendorIdentifier,
		})
	}
	return headers, rows
}

func printGameCenterLeaderboardSetsTable(resp *GameCenterLeaderboardSetsResponse) error {
	h, r := gameCenterLeaderboardSetsRows(resp)
	RenderTable(h, r)
	return nil
}

func printGameCenterLeaderboardSetsMarkdown(resp *GameCenterLeaderboardSetsResponse) error {
	h, r := gameCenterLeaderboardSetsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterLeaderboardSetDeleteResultRows(result *GameCenterLeaderboardSetDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printGameCenterLeaderboardSetDeleteResultTable(result *GameCenterLeaderboardSetDeleteResult) error {
	h, r := gameCenterLeaderboardSetDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printGameCenterLeaderboardSetDeleteResultMarkdown(result *GameCenterLeaderboardSetDeleteResult) error {
	h, r := gameCenterLeaderboardSetDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterLeaderboardLocalizationsRows(resp *GameCenterLeaderboardLocalizationsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Locale", "Name", "Formatter Override", "Formatter Suffix", "Formatter Suffix Singular", "Description"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			item.Attributes.Locale,
			compactWhitespace(item.Attributes.Name),
			formatOptionalString(item.Attributes.FormatterOverride),
			formatOptionalString(item.Attributes.FormatterSuffix),
			formatOptionalString(item.Attributes.FormatterSuffixSingular),
			formatOptionalString(item.Attributes.Description),
		})
	}
	return headers, rows
}

func printGameCenterLeaderboardLocalizationsTable(resp *GameCenterLeaderboardLocalizationsResponse) error {
	h, r := gameCenterLeaderboardLocalizationsRows(resp)
	RenderTable(h, r)
	return nil
}

func printGameCenterLeaderboardLocalizationsMarkdown(resp *GameCenterLeaderboardLocalizationsResponse) error {
	h, r := gameCenterLeaderboardLocalizationsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterLeaderboardLocalizationDeleteResultRows(result *GameCenterLeaderboardLocalizationDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printGameCenterLeaderboardLocalizationDeleteResultTable(result *GameCenterLeaderboardLocalizationDeleteResult) error {
	h, r := gameCenterLeaderboardLocalizationDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printGameCenterLeaderboardLocalizationDeleteResultMarkdown(result *GameCenterLeaderboardLocalizationDeleteResult) error {
	h, r := gameCenterLeaderboardLocalizationDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterLeaderboardReleasesRows(resp *GameCenterLeaderboardReleasesResponse) ([]string, [][]string) {
	headers := []string{"ID", "Live"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			fmt.Sprintf("%t", item.Attributes.Live),
		})
	}
	return headers, rows
}

func printGameCenterLeaderboardReleasesTable(resp *GameCenterLeaderboardReleasesResponse) error {
	h, r := gameCenterLeaderboardReleasesRows(resp)
	RenderTable(h, r)
	return nil
}

func printGameCenterLeaderboardReleasesMarkdown(resp *GameCenterLeaderboardReleasesResponse) error {
	h, r := gameCenterLeaderboardReleasesRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterLeaderboardReleaseDeleteResultRows(result *GameCenterLeaderboardReleaseDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printGameCenterLeaderboardReleaseDeleteResultTable(result *GameCenterLeaderboardReleaseDeleteResult) error {
	h, r := gameCenterLeaderboardReleaseDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printGameCenterLeaderboardReleaseDeleteResultMarkdown(result *GameCenterLeaderboardReleaseDeleteResult) error {
	h, r := gameCenterLeaderboardReleaseDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterAchievementReleasesRows(resp *GameCenterAchievementReleasesResponse) ([]string, [][]string) {
	headers := []string{"ID", "Live"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			fmt.Sprintf("%t", item.Attributes.Live),
		})
	}
	return headers, rows
}

func printGameCenterAchievementReleasesTable(resp *GameCenterAchievementReleasesResponse) error {
	h, r := gameCenterAchievementReleasesRows(resp)
	RenderTable(h, r)
	return nil
}

func printGameCenterAchievementReleasesMarkdown(resp *GameCenterAchievementReleasesResponse) error {
	h, r := gameCenterAchievementReleasesRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterAchievementReleaseDeleteResultRows(result *GameCenterAchievementReleaseDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printGameCenterAchievementReleaseDeleteResultTable(result *GameCenterAchievementReleaseDeleteResult) error {
	h, r := gameCenterAchievementReleaseDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printGameCenterAchievementReleaseDeleteResultMarkdown(result *GameCenterAchievementReleaseDeleteResult) error {
	h, r := gameCenterAchievementReleaseDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterLeaderboardSetReleasesRows(resp *GameCenterLeaderboardSetReleasesResponse) ([]string, [][]string) {
	headers := []string{"ID", "Live"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			fmt.Sprintf("%t", item.Attributes.Live),
		})
	}
	return headers, rows
}

func printGameCenterLeaderboardSetReleasesTable(resp *GameCenterLeaderboardSetReleasesResponse) error {
	h, r := gameCenterLeaderboardSetReleasesRows(resp)
	RenderTable(h, r)
	return nil
}

func printGameCenterLeaderboardSetReleasesMarkdown(resp *GameCenterLeaderboardSetReleasesResponse) error {
	h, r := gameCenterLeaderboardSetReleasesRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterLeaderboardSetReleaseDeleteResultRows(result *GameCenterLeaderboardSetReleaseDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printGameCenterLeaderboardSetReleaseDeleteResultTable(result *GameCenterLeaderboardSetReleaseDeleteResult) error {
	h, r := gameCenterLeaderboardSetReleaseDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printGameCenterLeaderboardSetReleaseDeleteResultMarkdown(result *GameCenterLeaderboardSetReleaseDeleteResult) error {
	h, r := gameCenterLeaderboardSetReleaseDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterLeaderboardSetLocalizationsRows(resp *GameCenterLeaderboardSetLocalizationsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Locale", "Name"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			item.Attributes.Locale,
			compactWhitespace(item.Attributes.Name),
		})
	}
	return headers, rows
}

func printGameCenterLeaderboardSetLocalizationsTable(resp *GameCenterLeaderboardSetLocalizationsResponse) error {
	h, r := gameCenterLeaderboardSetLocalizationsRows(resp)
	RenderTable(h, r)
	return nil
}

func printGameCenterLeaderboardSetLocalizationsMarkdown(resp *GameCenterLeaderboardSetLocalizationsResponse) error {
	h, r := gameCenterLeaderboardSetLocalizationsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterLeaderboardSetLocalizationDeleteResultRows(result *GameCenterLeaderboardSetLocalizationDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printGameCenterLeaderboardSetLocalizationDeleteResultTable(result *GameCenterLeaderboardSetLocalizationDeleteResult) error {
	h, r := gameCenterLeaderboardSetLocalizationDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printGameCenterLeaderboardSetLocalizationDeleteResultMarkdown(result *GameCenterLeaderboardSetLocalizationDeleteResult) error {
	h, r := gameCenterLeaderboardSetLocalizationDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterAchievementLocalizationsRows(resp *GameCenterAchievementLocalizationsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Locale", "Name", "Before Earned Description", "After Earned Description"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			item.Attributes.Locale,
			compactWhitespace(item.Attributes.Name),
			compactWhitespace(item.Attributes.BeforeEarnedDescription),
			compactWhitespace(item.Attributes.AfterEarnedDescription),
		})
	}
	return headers, rows
}

func printGameCenterAchievementLocalizationsTable(resp *GameCenterAchievementLocalizationsResponse) error {
	h, r := gameCenterAchievementLocalizationsRows(resp)
	RenderTable(h, r)
	return nil
}

func printGameCenterAchievementLocalizationsMarkdown(resp *GameCenterAchievementLocalizationsResponse) error {
	h, r := gameCenterAchievementLocalizationsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterAchievementLocalizationDeleteResultRows(result *GameCenterAchievementLocalizationDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printGameCenterAchievementLocalizationDeleteResultTable(result *GameCenterAchievementLocalizationDeleteResult) error {
	h, r := gameCenterAchievementLocalizationDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printGameCenterAchievementLocalizationDeleteResultMarkdown(result *GameCenterAchievementLocalizationDeleteResult) error {
	h, r := gameCenterAchievementLocalizationDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterLeaderboardImageUploadResultRows(result *GameCenterLeaderboardImageUploadResult) ([]string, [][]string) {
	headers := []string{"ID", "Localization ID", "File Name", "File Size", "Delivery State", "Uploaded"}
	rows := [][]string{{
		result.ID,
		result.LocalizationID,
		result.FileName,
		fmt.Sprintf("%d", result.FileSize),
		result.AssetDeliveryState,
		fmt.Sprintf("%t", result.Uploaded),
	}}
	return headers, rows
}

func printGameCenterLeaderboardImageUploadResultTable(result *GameCenterLeaderboardImageUploadResult) error {
	h, r := gameCenterLeaderboardImageUploadResultRows(result)
	RenderTable(h, r)
	return nil
}

func printGameCenterLeaderboardImageUploadResultMarkdown(result *GameCenterLeaderboardImageUploadResult) error {
	h, r := gameCenterLeaderboardImageUploadResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterLeaderboardImageDeleteResultRows(result *GameCenterLeaderboardImageDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printGameCenterLeaderboardImageDeleteResultTable(result *GameCenterLeaderboardImageDeleteResult) error {
	h, r := gameCenterLeaderboardImageDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printGameCenterLeaderboardImageDeleteResultMarkdown(result *GameCenterLeaderboardImageDeleteResult) error {
	h, r := gameCenterLeaderboardImageDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterAchievementImageUploadResultRows(result *GameCenterAchievementImageUploadResult) ([]string, [][]string) {
	headers := []string{"ID", "Localization ID", "File Name", "File Size", "Delivery State", "Uploaded"}
	rows := [][]string{{
		result.ID,
		result.LocalizationID,
		result.FileName,
		fmt.Sprintf("%d", result.FileSize),
		result.AssetDeliveryState,
		fmt.Sprintf("%t", result.Uploaded),
	}}
	return headers, rows
}

func printGameCenterAchievementImageUploadResultTable(result *GameCenterAchievementImageUploadResult) error {
	h, r := gameCenterAchievementImageUploadResultRows(result)
	RenderTable(h, r)
	return nil
}

func printGameCenterAchievementImageUploadResultMarkdown(result *GameCenterAchievementImageUploadResult) error {
	h, r := gameCenterAchievementImageUploadResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterAchievementImageDeleteResultRows(result *GameCenterAchievementImageDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printGameCenterAchievementImageDeleteResultTable(result *GameCenterAchievementImageDeleteResult) error {
	h, r := gameCenterAchievementImageDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printGameCenterAchievementImageDeleteResultMarkdown(result *GameCenterAchievementImageDeleteResult) error {
	h, r := gameCenterAchievementImageDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterLeaderboardSetImageUploadResultRows(result *GameCenterLeaderboardSetImageUploadResult) ([]string, [][]string) {
	headers := []string{"ID", "Localization ID", "File Name", "File Size", "Delivery State", "Uploaded"}
	rows := [][]string{{
		result.ID,
		result.LocalizationID,
		result.FileName,
		fmt.Sprintf("%d", result.FileSize),
		result.AssetDeliveryState,
		fmt.Sprintf("%t", result.Uploaded),
	}}
	return headers, rows
}

func printGameCenterLeaderboardSetImageUploadResultTable(result *GameCenterLeaderboardSetImageUploadResult) error {
	h, r := gameCenterLeaderboardSetImageUploadResultRows(result)
	RenderTable(h, r)
	return nil
}

func printGameCenterLeaderboardSetImageUploadResultMarkdown(result *GameCenterLeaderboardSetImageUploadResult) error {
	h, r := gameCenterLeaderboardSetImageUploadResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterLeaderboardSetImageDeleteResultRows(result *GameCenterLeaderboardSetImageDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printGameCenterLeaderboardSetImageDeleteResultTable(result *GameCenterLeaderboardSetImageDeleteResult) error {
	h, r := gameCenterLeaderboardSetImageDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printGameCenterLeaderboardSetImageDeleteResultMarkdown(result *GameCenterLeaderboardSetImageDeleteResult) error {
	h, r := gameCenterLeaderboardSetImageDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterChallengesRows(resp *GameCenterChallengesResponse) ([]string, [][]string) {
	headers := []string{"ID", "Reference Name", "Vendor ID", "Type", "Repeatable", "Archived"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.ReferenceName),
			item.Attributes.VendorIdentifier,
			item.Attributes.ChallengeType,
			fmt.Sprintf("%t", item.Attributes.Repeatable),
			fmt.Sprintf("%t", item.Attributes.Archived),
		})
	}
	return headers, rows
}

func printGameCenterChallengesTable(resp *GameCenterChallengesResponse) error {
	h, r := gameCenterChallengesRows(resp)
	RenderTable(h, r)
	return nil
}

func printGameCenterChallengesMarkdown(resp *GameCenterChallengesResponse) error {
	h, r := gameCenterChallengesRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterChallengeDeleteResultRows(result *GameCenterChallengeDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printGameCenterChallengeDeleteResultTable(result *GameCenterChallengeDeleteResult) error {
	h, r := gameCenterChallengeDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printGameCenterChallengeDeleteResultMarkdown(result *GameCenterChallengeDeleteResult) error {
	h, r := gameCenterChallengeDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterAchievementVersionsRows(resp *GameCenterAchievementVersionsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Version", "State"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			fmt.Sprintf("%d", item.Attributes.Version),
			string(item.Attributes.State),
		})
	}
	return headers, rows
}

func printGameCenterAchievementVersionsTable(resp *GameCenterAchievementVersionsResponse) error {
	h, r := gameCenterAchievementVersionsRows(resp)
	RenderTable(h, r)
	return nil
}

func printGameCenterAchievementVersionsMarkdown(resp *GameCenterAchievementVersionsResponse) error {
	h, r := gameCenterAchievementVersionsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterLeaderboardVersionsRows(resp *GameCenterLeaderboardVersionsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Version", "State"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			fmt.Sprintf("%d", item.Attributes.Version),
			string(item.Attributes.State),
		})
	}
	return headers, rows
}

func printGameCenterLeaderboardVersionsTable(resp *GameCenterLeaderboardVersionsResponse) error {
	h, r := gameCenterLeaderboardVersionsRows(resp)
	RenderTable(h, r)
	return nil
}

func printGameCenterLeaderboardVersionsMarkdown(resp *GameCenterLeaderboardVersionsResponse) error {
	h, r := gameCenterLeaderboardVersionsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterLeaderboardSetVersionsRows(resp *GameCenterLeaderboardSetVersionsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Version", "State"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			fmt.Sprintf("%d", item.Attributes.Version),
			string(item.Attributes.State),
		})
	}
	return headers, rows
}

func printGameCenterLeaderboardSetVersionsTable(resp *GameCenterLeaderboardSetVersionsResponse) error {
	h, r := gameCenterLeaderboardSetVersionsRows(resp)
	RenderTable(h, r)
	return nil
}

func printGameCenterLeaderboardSetVersionsMarkdown(resp *GameCenterLeaderboardSetVersionsResponse) error {
	h, r := gameCenterLeaderboardSetVersionsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterChallengeVersionsRows(resp *GameCenterChallengeVersionsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Version", "State"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			fmt.Sprintf("%d", item.Attributes.Version),
			string(item.Attributes.State),
		})
	}
	return headers, rows
}

func printGameCenterChallengeVersionsTable(resp *GameCenterChallengeVersionsResponse) error {
	h, r := gameCenterChallengeVersionsRows(resp)
	RenderTable(h, r)
	return nil
}

func printGameCenterChallengeVersionsMarkdown(resp *GameCenterChallengeVersionsResponse) error {
	h, r := gameCenterChallengeVersionsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterChallengeLocalizationsRows(resp *GameCenterChallengeLocalizationsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Locale", "Name", "Description"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			item.Attributes.Locale,
			compactWhitespace(item.Attributes.Name),
			compactWhitespace(item.Attributes.Description),
		})
	}
	return headers, rows
}

func printGameCenterChallengeLocalizationsTable(resp *GameCenterChallengeLocalizationsResponse) error {
	h, r := gameCenterChallengeLocalizationsRows(resp)
	RenderTable(h, r)
	return nil
}

func printGameCenterChallengeLocalizationsMarkdown(resp *GameCenterChallengeLocalizationsResponse) error {
	h, r := gameCenterChallengeLocalizationsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterChallengeLocalizationDeleteResultRows(result *GameCenterChallengeLocalizationDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printGameCenterChallengeLocalizationDeleteResultTable(result *GameCenterChallengeLocalizationDeleteResult) error {
	h, r := gameCenterChallengeLocalizationDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printGameCenterChallengeLocalizationDeleteResultMarkdown(result *GameCenterChallengeLocalizationDeleteResult) error {
	h, r := gameCenterChallengeLocalizationDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterChallengeImagesRows(resp *GameCenterChallengeImagesResponse) ([]string, [][]string) {
	headers := []string{"ID", "File Name", "File Size", "Delivery State"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		state := ""
		if item.Attributes.AssetDeliveryState != nil {
			state = item.Attributes.AssetDeliveryState.State
		}
		rows = append(rows, []string{
			item.ID,
			item.Attributes.FileName,
			fmt.Sprintf("%d", item.Attributes.FileSize),
			state,
		})
	}
	return headers, rows
}

func printGameCenterChallengeImagesTable(resp *GameCenterChallengeImagesResponse) error {
	h, r := gameCenterChallengeImagesRows(resp)
	RenderTable(h, r)
	return nil
}

func printGameCenterChallengeImagesMarkdown(resp *GameCenterChallengeImagesResponse) error {
	h, r := gameCenterChallengeImagesRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterChallengeImageUploadResultRows(result *GameCenterChallengeImageUploadResult) ([]string, [][]string) {
	headers := []string{"ID", "Localization ID", "File Name", "File Size", "Delivery State", "Uploaded"}
	rows := [][]string{{
		result.ID,
		result.LocalizationID,
		result.FileName,
		fmt.Sprintf("%d", result.FileSize),
		result.AssetDeliveryState,
		fmt.Sprintf("%t", result.Uploaded),
	}}
	return headers, rows
}

func printGameCenterChallengeImageUploadResultTable(result *GameCenterChallengeImageUploadResult) error {
	h, r := gameCenterChallengeImageUploadResultRows(result)
	RenderTable(h, r)
	return nil
}

func printGameCenterChallengeImageUploadResultMarkdown(result *GameCenterChallengeImageUploadResult) error {
	h, r := gameCenterChallengeImageUploadResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterChallengeImageDeleteResultRows(result *GameCenterChallengeImageDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printGameCenterChallengeImageDeleteResultTable(result *GameCenterChallengeImageDeleteResult) error {
	h, r := gameCenterChallengeImageDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printGameCenterChallengeImageDeleteResultMarkdown(result *GameCenterChallengeImageDeleteResult) error {
	h, r := gameCenterChallengeImageDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterChallengeReleasesRows(resp *GameCenterChallengeVersionReleasesResponse) ([]string, [][]string) {
	headers := []string{"ID"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{item.ID})
	}
	return headers, rows
}

func printGameCenterChallengeReleasesTable(resp *GameCenterChallengeVersionReleasesResponse) error {
	h, r := gameCenterChallengeReleasesRows(resp)
	RenderTable(h, r)
	return nil
}

func printGameCenterChallengeReleasesMarkdown(resp *GameCenterChallengeVersionReleasesResponse) error {
	h, r := gameCenterChallengeReleasesRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterChallengeReleaseDeleteResultRows(result *GameCenterChallengeVersionReleaseDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printGameCenterChallengeReleaseDeleteResultTable(result *GameCenterChallengeVersionReleaseDeleteResult) error {
	h, r := gameCenterChallengeReleaseDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printGameCenterChallengeReleaseDeleteResultMarkdown(result *GameCenterChallengeVersionReleaseDeleteResult) error {
	h, r := gameCenterChallengeReleaseDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterActivitiesRows(resp *GameCenterActivitiesResponse) ([]string, [][]string) {
	headers := []string{"ID", "Reference Name", "Vendor ID", "Play Style", "Min Players", "Max Players", "Party Code", "Archived"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.ReferenceName),
			item.Attributes.VendorIdentifier,
			item.Attributes.PlayStyle,
			fmt.Sprintf("%d", item.Attributes.MinimumPlayersCount),
			fmt.Sprintf("%d", item.Attributes.MaximumPlayersCount),
			fmt.Sprintf("%t", item.Attributes.SupportsPartyCode),
			fmt.Sprintf("%t", item.Attributes.Archived),
		})
	}
	return headers, rows
}

func printGameCenterActivitiesTable(resp *GameCenterActivitiesResponse) error {
	h, r := gameCenterActivitiesRows(resp)
	RenderTable(h, r)
	return nil
}

func printGameCenterActivitiesMarkdown(resp *GameCenterActivitiesResponse) error {
	h, r := gameCenterActivitiesRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterActivityDeleteResultRows(result *GameCenterActivityDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printGameCenterActivityDeleteResultTable(result *GameCenterActivityDeleteResult) error {
	h, r := gameCenterActivityDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printGameCenterActivityDeleteResultMarkdown(result *GameCenterActivityDeleteResult) error {
	h, r := gameCenterActivityDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterActivityVersionsRows(resp *GameCenterActivityVersionsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Version", "State", "Fallback URL"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			fmt.Sprintf("%d", item.Attributes.Version),
			string(item.Attributes.State),
			item.Attributes.FallbackURL,
		})
	}
	return headers, rows
}

func printGameCenterActivityVersionsTable(resp *GameCenterActivityVersionsResponse) error {
	h, r := gameCenterActivityVersionsRows(resp)
	RenderTable(h, r)
	return nil
}

func printGameCenterActivityVersionsMarkdown(resp *GameCenterActivityVersionsResponse) error {
	h, r := gameCenterActivityVersionsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterActivityLocalizationsRows(resp *GameCenterActivityLocalizationsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Locale", "Name", "Description"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			item.Attributes.Locale,
			compactWhitespace(item.Attributes.Name),
			compactWhitespace(item.Attributes.Description),
		})
	}
	return headers, rows
}

func printGameCenterActivityLocalizationsTable(resp *GameCenterActivityLocalizationsResponse) error {
	h, r := gameCenterActivityLocalizationsRows(resp)
	RenderTable(h, r)
	return nil
}

func printGameCenterActivityLocalizationsMarkdown(resp *GameCenterActivityLocalizationsResponse) error {
	h, r := gameCenterActivityLocalizationsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterActivityLocalizationDeleteResultRows(result *GameCenterActivityLocalizationDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printGameCenterActivityLocalizationDeleteResultTable(result *GameCenterActivityLocalizationDeleteResult) error {
	h, r := gameCenterActivityLocalizationDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printGameCenterActivityLocalizationDeleteResultMarkdown(result *GameCenterActivityLocalizationDeleteResult) error {
	h, r := gameCenterActivityLocalizationDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterActivityImagesRows(resp *GameCenterActivityImagesResponse) ([]string, [][]string) {
	headers := []string{"ID", "File Name", "File Size", "Delivery State"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		state := ""
		if item.Attributes.AssetDeliveryState != nil {
			state = item.Attributes.AssetDeliveryState.State
		}
		rows = append(rows, []string{
			item.ID,
			item.Attributes.FileName,
			fmt.Sprintf("%d", item.Attributes.FileSize),
			state,
		})
	}
	return headers, rows
}

func printGameCenterActivityImagesTable(resp *GameCenterActivityImagesResponse) error {
	h, r := gameCenterActivityImagesRows(resp)
	RenderTable(h, r)
	return nil
}

func printGameCenterActivityImagesMarkdown(resp *GameCenterActivityImagesResponse) error {
	h, r := gameCenterActivityImagesRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterActivityImageUploadResultRows(result *GameCenterActivityImageUploadResult) ([]string, [][]string) {
	headers := []string{"ID", "Localization ID", "File Name", "File Size", "Delivery State", "Uploaded"}
	rows := [][]string{{
		result.ID,
		result.LocalizationID,
		result.FileName,
		fmt.Sprintf("%d", result.FileSize),
		result.AssetDeliveryState,
		fmt.Sprintf("%t", result.Uploaded),
	}}
	return headers, rows
}

func printGameCenterActivityImageUploadResultTable(result *GameCenterActivityImageUploadResult) error {
	h, r := gameCenterActivityImageUploadResultRows(result)
	RenderTable(h, r)
	return nil
}

func printGameCenterActivityImageUploadResultMarkdown(result *GameCenterActivityImageUploadResult) error {
	h, r := gameCenterActivityImageUploadResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterActivityImageDeleteResultRows(result *GameCenterActivityImageDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printGameCenterActivityImageDeleteResultTable(result *GameCenterActivityImageDeleteResult) error {
	h, r := gameCenterActivityImageDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printGameCenterActivityImageDeleteResultMarkdown(result *GameCenterActivityImageDeleteResult) error {
	h, r := gameCenterActivityImageDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterActivityReleasesRows(resp *GameCenterActivityVersionReleasesResponse) ([]string, [][]string) {
	headers := []string{"ID"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{item.ID})
	}
	return headers, rows
}

func printGameCenterActivityReleasesTable(resp *GameCenterActivityVersionReleasesResponse) error {
	h, r := gameCenterActivityReleasesRows(resp)
	RenderTable(h, r)
	return nil
}

func printGameCenterActivityReleasesMarkdown(resp *GameCenterActivityVersionReleasesResponse) error {
	h, r := gameCenterActivityReleasesRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterActivityReleaseDeleteResultRows(result *GameCenterActivityVersionReleaseDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printGameCenterActivityReleaseDeleteResultTable(result *GameCenterActivityVersionReleaseDeleteResult) error {
	h, r := gameCenterActivityReleaseDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printGameCenterActivityReleaseDeleteResultMarkdown(result *GameCenterActivityVersionReleaseDeleteResult) error {
	h, r := gameCenterActivityReleaseDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterGroupsRows(resp *GameCenterGroupsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Reference Name"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.ReferenceName),
		})
	}
	return headers, rows
}

func printGameCenterGroupsTable(resp *GameCenterGroupsResponse) error {
	h, r := gameCenterGroupsRows(resp)
	RenderTable(h, r)
	return nil
}

func printGameCenterGroupsMarkdown(resp *GameCenterGroupsResponse) error {
	h, r := gameCenterGroupsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterGroupDeleteResultRows(result *GameCenterGroupDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printGameCenterGroupDeleteResultTable(result *GameCenterGroupDeleteResult) error {
	h, r := gameCenterGroupDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printGameCenterGroupDeleteResultMarkdown(result *GameCenterGroupDeleteResult) error {
	h, r := gameCenterGroupDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterAppVersionsRows(resp *GameCenterAppVersionsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Enabled"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{item.ID, fmt.Sprintf("%t", item.Attributes.Enabled)})
	}
	return headers, rows
}

func printGameCenterAppVersionsTable(resp *GameCenterAppVersionsResponse) error {
	h, r := gameCenterAppVersionsRows(resp)
	RenderTable(h, r)
	return nil
}

func printGameCenterAppVersionsMarkdown(resp *GameCenterAppVersionsResponse) error {
	h, r := gameCenterAppVersionsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterEnabledVersionsRows(resp *GameCenterEnabledVersionsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Platform", "Version", "Icon Template URL"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		iconURL := ""
		if item.Attributes.IconAsset != nil {
			iconURL = item.Attributes.IconAsset.TemplateURL
		}
		rows = append(rows, []string{
			item.ID,
			string(item.Attributes.Platform),
			item.Attributes.VersionString,
			iconURL,
		})
	}
	return headers, rows
}

func printGameCenterEnabledVersionsTable(resp *GameCenterEnabledVersionsResponse) error {
	h, r := gameCenterEnabledVersionsRows(resp)
	RenderTable(h, r)
	return nil
}

func printGameCenterEnabledVersionsMarkdown(resp *GameCenterEnabledVersionsResponse) error {
	h, r := gameCenterEnabledVersionsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterDetailsRows(resp *GameCenterDetailsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Arcade Enabled", "Challenge Enabled", "Leaderboard Enabled", "Leaderboard Set Enabled", "Achievement Enabled", "Multiplayer Session", "Turn-Based Session"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			fmt.Sprintf("%t", item.Attributes.ArcadeEnabled),
			fmt.Sprintf("%t", item.Attributes.ChallengeEnabled),
			fmt.Sprintf("%t", item.Attributes.LeaderboardEnabled),
			fmt.Sprintf("%t", item.Attributes.LeaderboardSetEnabled),
			fmt.Sprintf("%t", item.Attributes.AchievementEnabled),
			fmt.Sprintf("%t", item.Attributes.MultiplayerSessionEnabled),
			fmt.Sprintf("%t", item.Attributes.MultiplayerTurnBasedSessionEnabled),
		})
	}
	return headers, rows
}

func printGameCenterDetailsTable(resp *GameCenterDetailsResponse) error {
	h, r := gameCenterDetailsRows(resp)
	RenderTable(h, r)
	return nil
}

func printGameCenterDetailsMarkdown(resp *GameCenterDetailsResponse) error {
	h, r := gameCenterDetailsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterMatchmakingQueuesRows(resp *GameCenterMatchmakingQueuesResponse) ([]string, [][]string) {
	headers := []string{"ID", "Reference Name", "Classic Bundle IDs"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.ReferenceName),
			formatStringList(item.Attributes.ClassicMatchmakingBundleIDs),
		})
	}
	return headers, rows
}

func printGameCenterMatchmakingQueuesTable(resp *GameCenterMatchmakingQueuesResponse) error {
	h, r := gameCenterMatchmakingQueuesRows(resp)
	RenderTable(h, r)
	return nil
}

func printGameCenterMatchmakingQueuesMarkdown(resp *GameCenterMatchmakingQueuesResponse) error {
	h, r := gameCenterMatchmakingQueuesRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterMatchmakingQueueDeleteResultRows(result *GameCenterMatchmakingQueueDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printGameCenterMatchmakingQueueDeleteResultTable(result *GameCenterMatchmakingQueueDeleteResult) error {
	h, r := gameCenterMatchmakingQueueDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printGameCenterMatchmakingQueueDeleteResultMarkdown(result *GameCenterMatchmakingQueueDeleteResult) error {
	h, r := gameCenterMatchmakingQueueDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterMatchmakingRuleSetsRows(resp *GameCenterMatchmakingRuleSetsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Reference Name", "Language", "Min Players", "Max Players"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.ReferenceName),
			fmt.Sprintf("%d", item.Attributes.RuleLanguageVersion),
			fmt.Sprintf("%d", item.Attributes.MinPlayers),
			fmt.Sprintf("%d", item.Attributes.MaxPlayers),
		})
	}
	return headers, rows
}

func printGameCenterMatchmakingRuleSetsTable(resp *GameCenterMatchmakingRuleSetsResponse) error {
	h, r := gameCenterMatchmakingRuleSetsRows(resp)
	RenderTable(h, r)
	return nil
}

func printGameCenterMatchmakingRuleSetsMarkdown(resp *GameCenterMatchmakingRuleSetsResponse) error {
	h, r := gameCenterMatchmakingRuleSetsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterMatchmakingRuleSetDeleteResultRows(result *GameCenterMatchmakingRuleSetDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printGameCenterMatchmakingRuleSetDeleteResultTable(result *GameCenterMatchmakingRuleSetDeleteResult) error {
	h, r := gameCenterMatchmakingRuleSetDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printGameCenterMatchmakingRuleSetDeleteResultMarkdown(result *GameCenterMatchmakingRuleSetDeleteResult) error {
	h, r := gameCenterMatchmakingRuleSetDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterMatchmakingRulesRows(resp *GameCenterMatchmakingRulesResponse) ([]string, [][]string) {
	headers := []string{"ID", "Reference Name", "Type", "Weight"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.ReferenceName),
			item.Attributes.Type,
			fmt.Sprintf("%g", item.Attributes.Weight),
		})
	}
	return headers, rows
}

func printGameCenterMatchmakingRulesTable(resp *GameCenterMatchmakingRulesResponse) error {
	h, r := gameCenterMatchmakingRulesRows(resp)
	RenderTable(h, r)
	return nil
}

func printGameCenterMatchmakingRulesMarkdown(resp *GameCenterMatchmakingRulesResponse) error {
	h, r := gameCenterMatchmakingRulesRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterMatchmakingRuleDeleteResultRows(result *GameCenterMatchmakingRuleDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printGameCenterMatchmakingRuleDeleteResultTable(result *GameCenterMatchmakingRuleDeleteResult) error {
	h, r := gameCenterMatchmakingRuleDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printGameCenterMatchmakingRuleDeleteResultMarkdown(result *GameCenterMatchmakingRuleDeleteResult) error {
	h, r := gameCenterMatchmakingRuleDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterMatchmakingTeamsRows(resp *GameCenterMatchmakingTeamsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Reference Name", "Min Players", "Max Players"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.ReferenceName),
			fmt.Sprintf("%d", item.Attributes.MinPlayers),
			fmt.Sprintf("%d", item.Attributes.MaxPlayers),
		})
	}
	return headers, rows
}

func printGameCenterMatchmakingTeamsTable(resp *GameCenterMatchmakingTeamsResponse) error {
	h, r := gameCenterMatchmakingTeamsRows(resp)
	RenderTable(h, r)
	return nil
}

func printGameCenterMatchmakingTeamsMarkdown(resp *GameCenterMatchmakingTeamsResponse) error {
	h, r := gameCenterMatchmakingTeamsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterMatchmakingTeamDeleteResultRows(result *GameCenterMatchmakingTeamDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printGameCenterMatchmakingTeamDeleteResultTable(result *GameCenterMatchmakingTeamDeleteResult) error {
	h, r := gameCenterMatchmakingTeamDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printGameCenterMatchmakingTeamDeleteResultMarkdown(result *GameCenterMatchmakingTeamDeleteResult) error {
	h, r := gameCenterMatchmakingTeamDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterMetricsRows(resp *GameCenterMetricsResponse) ([]string, [][]string) {
	headers := []string{"Start", "End", "Granularity", "Values", "Dimensions"}
	var rows [][]string
	for _, item := range resp.Data {
		for _, point := range item.DataPoints {
			rows = append(rows, []string{
				point.Start,
				point.End,
				formatMetricGranularity(item.Granularity),
				formatMetricJSON(point.Values),
				formatMetricJSON(item.Dimensions),
			})
		}
	}
	return headers, rows
}

func printGameCenterMetricsTable(resp *GameCenterMetricsResponse) error {
	h, r := gameCenterMetricsRows(resp)
	RenderTable(h, r)
	return nil
}

func printGameCenterMetricsMarkdown(resp *GameCenterMetricsResponse) error {
	h, r := gameCenterMetricsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterMatchmakingRuleSetTestRows(resp *GameCenterMatchmakingRuleSetTestResponse) ([]string, [][]string) {
	headers := []string{"ID"}
	rows := [][]string{{resp.Data.ID}}
	return headers, rows
}

func printGameCenterMatchmakingRuleSetTestTable(resp *GameCenterMatchmakingRuleSetTestResponse) error {
	h, r := gameCenterMatchmakingRuleSetTestRows(resp)
	RenderTable(h, r)
	return nil
}

func printGameCenterMatchmakingRuleSetTestMarkdown(resp *GameCenterMatchmakingRuleSetTestResponse) error {
	h, r := gameCenterMatchmakingRuleSetTestRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterLeaderboardEntrySubmissionRows(resp *GameCenterLeaderboardEntrySubmissionResponse) ([]string, [][]string) {
	attrs := resp.Data.Attributes
	submittedDate := ""
	if attrs.SubmittedDate != nil {
		submittedDate = *attrs.SubmittedDate
	}
	headers := []string{"ID", "Vendor ID", "Score", "Bundle ID", "Scoped Player ID", "Submitted Date"}
	rows := [][]string{{
		resp.Data.ID,
		compactWhitespace(attrs.VendorIdentifier),
		compactWhitespace(attrs.Score),
		compactWhitespace(attrs.BundleID),
		compactWhitespace(attrs.ScopedPlayerID),
		compactWhitespace(submittedDate),
	}}
	return headers, rows
}

func printGameCenterLeaderboardEntrySubmissionTable(resp *GameCenterLeaderboardEntrySubmissionResponse) error {
	h, r := gameCenterLeaderboardEntrySubmissionRows(resp)
	RenderTable(h, r)
	return nil
}

func printGameCenterLeaderboardEntrySubmissionMarkdown(resp *GameCenterLeaderboardEntrySubmissionResponse) error {
	h, r := gameCenterLeaderboardEntrySubmissionRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func gameCenterPlayerAchievementSubmissionRows(resp *GameCenterPlayerAchievementSubmissionResponse) ([]string, [][]string) {
	attrs := resp.Data.Attributes
	submittedDate := ""
	if attrs.SubmittedDate != nil {
		submittedDate = *attrs.SubmittedDate
	}
	headers := []string{"ID", "Vendor ID", "Percent", "Bundle ID", "Scoped Player ID", "Submitted Date"}
	rows := [][]string{{
		resp.Data.ID,
		compactWhitespace(attrs.VendorIdentifier),
		fmt.Sprintf("%d", attrs.PercentageAchieved),
		compactWhitespace(attrs.BundleID),
		compactWhitespace(attrs.ScopedPlayerID),
		compactWhitespace(submittedDate),
	}}
	return headers, rows
}

func printGameCenterPlayerAchievementSubmissionTable(resp *GameCenterPlayerAchievementSubmissionResponse) error {
	h, r := gameCenterPlayerAchievementSubmissionRows(resp)
	RenderTable(h, r)
	return nil
}

func printGameCenterPlayerAchievementSubmissionMarkdown(resp *GameCenterPlayerAchievementSubmissionResponse) error {
	h, r := gameCenterPlayerAchievementSubmissionRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func formatStringList(items []string) string {
	if len(items) == 0 {
		return ""
	}
	return strings.Join(items, ",")
}

func formatMetricJSON(value any) string {
	if value == nil {
		return ""
	}
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(data)
}

func formatMetricGranularity(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}
