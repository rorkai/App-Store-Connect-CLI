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

func gameCenterAchievementDeleteResultRows(result *GameCenterAchievementDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
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

func gameCenterLeaderboardDeleteResultRows(result *GameCenterLeaderboardDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
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

func gameCenterLeaderboardSetDeleteResultRows(result *GameCenterLeaderboardSetDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
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

func gameCenterLeaderboardLocalizationDeleteResultRows(result *GameCenterLeaderboardLocalizationDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
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

func gameCenterLeaderboardReleaseDeleteResultRows(result *GameCenterLeaderboardReleaseDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
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

func gameCenterAchievementReleaseDeleteResultRows(result *GameCenterAchievementReleaseDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
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

func gameCenterLeaderboardSetReleaseDeleteResultRows(result *GameCenterLeaderboardSetReleaseDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
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

func gameCenterLeaderboardSetLocalizationDeleteResultRows(result *GameCenterLeaderboardSetLocalizationDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
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

func gameCenterAchievementLocalizationDeleteResultRows(result *GameCenterAchievementLocalizationDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
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

func gameCenterLeaderboardImageDeleteResultRows(result *GameCenterLeaderboardImageDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
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

func gameCenterAchievementImageDeleteResultRows(result *GameCenterAchievementImageDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
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

func gameCenterLeaderboardSetImageDeleteResultRows(result *GameCenterLeaderboardSetImageDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
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

func gameCenterChallengeDeleteResultRows(result *GameCenterChallengeDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
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

func gameCenterChallengeLocalizationDeleteResultRows(result *GameCenterChallengeLocalizationDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
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

func gameCenterChallengeImageDeleteResultRows(result *GameCenterChallengeImageDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func gameCenterChallengeReleasesRows(resp *GameCenterChallengeVersionReleasesResponse) ([]string, [][]string) {
	headers := []string{"ID"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{item.ID})
	}
	return headers, rows
}

func gameCenterChallengeReleaseDeleteResultRows(result *GameCenterChallengeVersionReleaseDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
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

func gameCenterActivityDeleteResultRows(result *GameCenterActivityDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
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

func gameCenterActivityLocalizationDeleteResultRows(result *GameCenterActivityLocalizationDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
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

func gameCenterActivityImageDeleteResultRows(result *GameCenterActivityImageDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func gameCenterActivityReleasesRows(resp *GameCenterActivityVersionReleasesResponse) ([]string, [][]string) {
	headers := []string{"ID"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{item.ID})
	}
	return headers, rows
}

func gameCenterActivityReleaseDeleteResultRows(result *GameCenterActivityVersionReleaseDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
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

func gameCenterGroupDeleteResultRows(result *GameCenterGroupDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func gameCenterAppVersionsRows(resp *GameCenterAppVersionsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Enabled"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{item.ID, fmt.Sprintf("%t", item.Attributes.Enabled)})
	}
	return headers, rows
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

func gameCenterMatchmakingQueueDeleteResultRows(result *GameCenterMatchmakingQueueDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
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

func gameCenterMatchmakingRuleSetDeleteResultRows(result *GameCenterMatchmakingRuleSetDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
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

func gameCenterMatchmakingRuleDeleteResultRows(result *GameCenterMatchmakingRuleDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
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

func gameCenterMatchmakingTeamDeleteResultRows(result *GameCenterMatchmakingTeamDeleteResult) ([]string, [][]string) {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
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

func gameCenterMatchmakingRuleSetTestRows(resp *GameCenterMatchmakingRuleSetTestResponse) ([]string, [][]string) {
	headers := []string{"ID"}
	rows := [][]string{{resp.Data.ID}}
	return headers, rows
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
