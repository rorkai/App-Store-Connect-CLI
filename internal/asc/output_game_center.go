package asc

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func printGameCenterAchievementsTable(resp *GameCenterAchievementsResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printGameCenterAchievementsMarkdown(resp *GameCenterAchievementsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Reference Name | Vendor ID | Points | Show Before Earned | Repeatable | Archived |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %d | %t | %t | %t |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.ReferenceName),
			escapeMarkdown(item.Attributes.VendorIdentifier),
			item.Attributes.Points,
			item.Attributes.ShowBeforeEarned,
			item.Attributes.Repeatable,
			item.Attributes.Archived,
		)
	}
	return nil
}

func printGameCenterAchievementDeleteResultTable(result *GameCenterAchievementDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterAchievementDeleteResultMarkdown(result *GameCenterAchievementDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}

func printGameCenterLeaderboardsTable(resp *GameCenterLeaderboardsResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printGameCenterLeaderboardsMarkdown(resp *GameCenterLeaderboardsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Reference Name | Vendor ID | Formatter | Sort | Submission Type | Archived |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s | %s | %t |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.ReferenceName),
			escapeMarkdown(item.Attributes.VendorIdentifier),
			escapeMarkdown(item.Attributes.DefaultFormatter),
			escapeMarkdown(item.Attributes.ScoreSortType),
			escapeMarkdown(item.Attributes.SubmissionType),
			item.Attributes.Archived,
		)
	}
	return nil
}

func printGameCenterLeaderboardDeleteResultTable(result *GameCenterLeaderboardDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterLeaderboardDeleteResultMarkdown(result *GameCenterLeaderboardDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}

func printGameCenterLeaderboardSetsTable(resp *GameCenterLeaderboardSetsResponse) error {
	headers := []string{"ID", "Reference Name", "Vendor ID"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.ReferenceName),
			item.Attributes.VendorIdentifier,
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterLeaderboardSetsMarkdown(resp *GameCenterLeaderboardSetsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Reference Name | Vendor ID |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.ReferenceName),
			escapeMarkdown(item.Attributes.VendorIdentifier),
		)
	}
	return nil
}

func printGameCenterLeaderboardSetDeleteResultTable(result *GameCenterLeaderboardSetDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterLeaderboardSetDeleteResultMarkdown(result *GameCenterLeaderboardSetDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}

func printGameCenterLeaderboardLocalizationsTable(resp *GameCenterLeaderboardLocalizationsResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printGameCenterLeaderboardLocalizationsMarkdown(resp *GameCenterLeaderboardLocalizationsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Locale | Name | Formatter Override | Formatter Suffix | Formatter Suffix Singular | Description |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.Locale),
			escapeMarkdown(item.Attributes.Name),
			escapeMarkdown(formatOptionalString(item.Attributes.FormatterOverride)),
			escapeMarkdown(formatOptionalString(item.Attributes.FormatterSuffix)),
			escapeMarkdown(formatOptionalString(item.Attributes.FormatterSuffixSingular)),
			escapeMarkdown(formatOptionalString(item.Attributes.Description)),
		)
	}
	return nil
}

func printGameCenterLeaderboardLocalizationDeleteResultTable(result *GameCenterLeaderboardLocalizationDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterLeaderboardLocalizationDeleteResultMarkdown(result *GameCenterLeaderboardLocalizationDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}

func printGameCenterLeaderboardReleasesTable(resp *GameCenterLeaderboardReleasesResponse) error {
	headers := []string{"ID", "Live"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			fmt.Sprintf("%t", item.Attributes.Live),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterLeaderboardReleasesMarkdown(resp *GameCenterLeaderboardReleasesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Live |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %t |\n",
			escapeMarkdown(item.ID),
			item.Attributes.Live,
		)
	}
	return nil
}

func printGameCenterLeaderboardReleaseDeleteResultTable(result *GameCenterLeaderboardReleaseDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterLeaderboardReleaseDeleteResultMarkdown(result *GameCenterLeaderboardReleaseDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}

func printGameCenterAchievementReleasesTable(resp *GameCenterAchievementReleasesResponse) error {
	headers := []string{"ID", "Live"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			fmt.Sprintf("%t", item.Attributes.Live),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterAchievementReleasesMarkdown(resp *GameCenterAchievementReleasesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Live |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %t |\n",
			escapeMarkdown(item.ID),
			item.Attributes.Live,
		)
	}
	return nil
}

func printGameCenterAchievementReleaseDeleteResultTable(result *GameCenterAchievementReleaseDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterAchievementReleaseDeleteResultMarkdown(result *GameCenterAchievementReleaseDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}

func printGameCenterLeaderboardSetReleasesTable(resp *GameCenterLeaderboardSetReleasesResponse) error {
	headers := []string{"ID", "Live"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			fmt.Sprintf("%t", item.Attributes.Live),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterLeaderboardSetReleasesMarkdown(resp *GameCenterLeaderboardSetReleasesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Live |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %t |\n",
			escapeMarkdown(item.ID),
			item.Attributes.Live,
		)
	}
	return nil
}

func printGameCenterLeaderboardSetReleaseDeleteResultTable(result *GameCenterLeaderboardSetReleaseDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterLeaderboardSetReleaseDeleteResultMarkdown(result *GameCenterLeaderboardSetReleaseDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}

func printGameCenterLeaderboardSetLocalizationsTable(resp *GameCenterLeaderboardSetLocalizationsResponse) error {
	headers := []string{"ID", "Locale", "Name"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			item.Attributes.Locale,
			compactWhitespace(item.Attributes.Name),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterLeaderboardSetLocalizationsMarkdown(resp *GameCenterLeaderboardSetLocalizationsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Locale | Name |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.Locale),
			escapeMarkdown(item.Attributes.Name),
		)
	}
	return nil
}

func printGameCenterLeaderboardSetLocalizationDeleteResultTable(result *GameCenterLeaderboardSetLocalizationDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterLeaderboardSetLocalizationDeleteResultMarkdown(result *GameCenterLeaderboardSetLocalizationDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}

func printGameCenterAchievementLocalizationsTable(resp *GameCenterAchievementLocalizationsResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printGameCenterAchievementLocalizationsMarkdown(resp *GameCenterAchievementLocalizationsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Locale | Name | Before Earned Description | After Earned Description |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.Locale),
			escapeMarkdown(item.Attributes.Name),
			escapeMarkdown(item.Attributes.BeforeEarnedDescription),
			escapeMarkdown(item.Attributes.AfterEarnedDescription),
		)
	}
	return nil
}

func printGameCenterAchievementLocalizationDeleteResultTable(result *GameCenterAchievementLocalizationDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterAchievementLocalizationDeleteResultMarkdown(result *GameCenterAchievementLocalizationDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}

func printGameCenterLeaderboardImageUploadResultTable(result *GameCenterLeaderboardImageUploadResult) error {
	headers := []string{"ID", "Localization ID", "File Name", "File Size", "Delivery State", "Uploaded"}
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

func printGameCenterLeaderboardImageUploadResultMarkdown(result *GameCenterLeaderboardImageUploadResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Localization ID | File Name | File Size | Delivery State | Uploaded |")
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

func printGameCenterLeaderboardImageDeleteResultTable(result *GameCenterLeaderboardImageDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterLeaderboardImageDeleteResultMarkdown(result *GameCenterLeaderboardImageDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}

func printGameCenterAchievementImageUploadResultTable(result *GameCenterAchievementImageUploadResult) error {
	headers := []string{"ID", "Localization ID", "File Name", "File Size", "Delivery State", "Uploaded"}
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

func printGameCenterAchievementImageUploadResultMarkdown(result *GameCenterAchievementImageUploadResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Localization ID | File Name | File Size | Delivery State | Uploaded |")
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

func printGameCenterAchievementImageDeleteResultTable(result *GameCenterAchievementImageDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterAchievementImageDeleteResultMarkdown(result *GameCenterAchievementImageDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}

func printGameCenterLeaderboardSetImageUploadResultTable(result *GameCenterLeaderboardSetImageUploadResult) error {
	headers := []string{"ID", "Localization ID", "File Name", "File Size", "Delivery State", "Uploaded"}
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

func printGameCenterLeaderboardSetImageUploadResultMarkdown(result *GameCenterLeaderboardSetImageUploadResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Localization ID | File Name | File Size | Delivery State | Uploaded |")
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

func printGameCenterLeaderboardSetImageDeleteResultTable(result *GameCenterLeaderboardSetImageDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterLeaderboardSetImageDeleteResultMarkdown(result *GameCenterLeaderboardSetImageDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.ID),
		result.Deleted,
	)
	return nil
}

func printGameCenterChallengesTable(resp *GameCenterChallengesResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printGameCenterChallengesMarkdown(resp *GameCenterChallengesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Reference Name | Vendor ID | Type | Repeatable | Archived |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %t | %t |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.ReferenceName),
			escapeMarkdown(item.Attributes.VendorIdentifier),
			escapeMarkdown(item.Attributes.ChallengeType),
			item.Attributes.Repeatable,
			item.Attributes.Archived,
		)
	}
	return nil
}

func printGameCenterChallengeDeleteResultTable(result *GameCenterChallengeDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterChallengeDeleteResultMarkdown(result *GameCenterChallengeDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n", escapeMarkdown(result.ID), result.Deleted)
	return nil
}

func printGameCenterAchievementVersionsTable(resp *GameCenterAchievementVersionsResponse) error {
	headers := []string{"ID", "Version", "State"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			fmt.Sprintf("%d", item.Attributes.Version),
			string(item.Attributes.State),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterAchievementVersionsMarkdown(resp *GameCenterAchievementVersionsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Version | State |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %d | %s |\n",
			escapeMarkdown(item.ID),
			item.Attributes.Version,
			escapeMarkdown(string(item.Attributes.State)),
		)
	}
	return nil
}

func printGameCenterLeaderboardVersionsTable(resp *GameCenterLeaderboardVersionsResponse) error {
	headers := []string{"ID", "Version", "State"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			fmt.Sprintf("%d", item.Attributes.Version),
			string(item.Attributes.State),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterLeaderboardVersionsMarkdown(resp *GameCenterLeaderboardVersionsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Version | State |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %d | %s |\n",
			escapeMarkdown(item.ID),
			item.Attributes.Version,
			escapeMarkdown(string(item.Attributes.State)),
		)
	}
	return nil
}

func printGameCenterLeaderboardSetVersionsTable(resp *GameCenterLeaderboardSetVersionsResponse) error {
	headers := []string{"ID", "Version", "State"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			fmt.Sprintf("%d", item.Attributes.Version),
			string(item.Attributes.State),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterLeaderboardSetVersionsMarkdown(resp *GameCenterLeaderboardSetVersionsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Version | State |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %d | %s |\n",
			escapeMarkdown(item.ID),
			item.Attributes.Version,
			escapeMarkdown(string(item.Attributes.State)),
		)
	}
	return nil
}

func printGameCenterChallengeVersionsTable(resp *GameCenterChallengeVersionsResponse) error {
	headers := []string{"ID", "Version", "State"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			fmt.Sprintf("%d", item.Attributes.Version),
			string(item.Attributes.State),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterChallengeVersionsMarkdown(resp *GameCenterChallengeVersionsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Version | State |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %d | %s |\n",
			escapeMarkdown(item.ID),
			item.Attributes.Version,
			escapeMarkdown(string(item.Attributes.State)),
		)
	}
	return nil
}

func printGameCenterChallengeLocalizationsTable(resp *GameCenterChallengeLocalizationsResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printGameCenterChallengeLocalizationsMarkdown(resp *GameCenterChallengeLocalizationsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Locale | Name | Description |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.Locale),
			escapeMarkdown(item.Attributes.Name),
			escapeMarkdown(item.Attributes.Description),
		)
	}
	return nil
}

func printGameCenterChallengeLocalizationDeleteResultTable(result *GameCenterChallengeLocalizationDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterChallengeLocalizationDeleteResultMarkdown(result *GameCenterChallengeLocalizationDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n", escapeMarkdown(result.ID), result.Deleted)
	return nil
}

func printGameCenterChallengeImagesTable(resp *GameCenterChallengeImagesResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printGameCenterChallengeImagesMarkdown(resp *GameCenterChallengeImagesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | File Name | File Size | Delivery State |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- |")
	for _, item := range resp.Data {
		state := ""
		if item.Attributes.AssetDeliveryState != nil {
			state = item.Attributes.AssetDeliveryState.State
		}
		fmt.Fprintf(os.Stdout, "| %s | %s | %d | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.FileName),
			item.Attributes.FileSize,
			escapeMarkdown(state),
		)
	}
	return nil
}

func printGameCenterChallengeImageUploadResultTable(result *GameCenterChallengeImageUploadResult) error {
	headers := []string{"ID", "Localization ID", "File Name", "File Size", "Delivery State", "Uploaded"}
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

func printGameCenterChallengeImageUploadResultMarkdown(result *GameCenterChallengeImageUploadResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Localization ID | File Name | File Size | Delivery State | Uploaded |")
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

func printGameCenterChallengeImageDeleteResultTable(result *GameCenterChallengeImageDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterChallengeImageDeleteResultMarkdown(result *GameCenterChallengeImageDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n", escapeMarkdown(result.ID), result.Deleted)
	return nil
}

func printGameCenterChallengeReleasesTable(resp *GameCenterChallengeVersionReleasesResponse) error {
	headers := []string{"ID"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{item.ID})
	}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterChallengeReleasesMarkdown(resp *GameCenterChallengeVersionReleasesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID |")
	fmt.Fprintln(os.Stdout, "| --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s |\n", escapeMarkdown(item.ID))
	}
	return nil
}

func printGameCenterChallengeReleaseDeleteResultTable(result *GameCenterChallengeVersionReleaseDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterChallengeReleaseDeleteResultMarkdown(result *GameCenterChallengeVersionReleaseDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n", escapeMarkdown(result.ID), result.Deleted)
	return nil
}

func printGameCenterActivitiesTable(resp *GameCenterActivitiesResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printGameCenterActivitiesMarkdown(resp *GameCenterActivitiesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Reference Name | Vendor ID | Play Style | Min Players | Max Players | Party Code | Archived |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %d | %d | %t | %t |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.ReferenceName),
			escapeMarkdown(item.Attributes.VendorIdentifier),
			escapeMarkdown(item.Attributes.PlayStyle),
			item.Attributes.MinimumPlayersCount,
			item.Attributes.MaximumPlayersCount,
			item.Attributes.SupportsPartyCode,
			item.Attributes.Archived,
		)
	}
	return nil
}

func printGameCenterActivityDeleteResultTable(result *GameCenterActivityDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterActivityDeleteResultMarkdown(result *GameCenterActivityDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n", escapeMarkdown(result.ID), result.Deleted)
	return nil
}

func printGameCenterActivityVersionsTable(resp *GameCenterActivityVersionsResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printGameCenterActivityVersionsMarkdown(resp *GameCenterActivityVersionsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Version | State | Fallback URL |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %d | %s | %s |\n",
			escapeMarkdown(item.ID),
			item.Attributes.Version,
			escapeMarkdown(string(item.Attributes.State)),
			escapeMarkdown(item.Attributes.FallbackURL),
		)
	}
	return nil
}

func printGameCenterActivityLocalizationsTable(resp *GameCenterActivityLocalizationsResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printGameCenterActivityLocalizationsMarkdown(resp *GameCenterActivityLocalizationsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Locale | Name | Description |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.Locale),
			escapeMarkdown(item.Attributes.Name),
			escapeMarkdown(item.Attributes.Description),
		)
	}
	return nil
}

func printGameCenterActivityLocalizationDeleteResultTable(result *GameCenterActivityLocalizationDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterActivityLocalizationDeleteResultMarkdown(result *GameCenterActivityLocalizationDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n", escapeMarkdown(result.ID), result.Deleted)
	return nil
}

func printGameCenterActivityImagesTable(resp *GameCenterActivityImagesResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printGameCenterActivityImagesMarkdown(resp *GameCenterActivityImagesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | File Name | File Size | Delivery State |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- |")
	for _, item := range resp.Data {
		state := ""
		if item.Attributes.AssetDeliveryState != nil {
			state = item.Attributes.AssetDeliveryState.State
		}
		fmt.Fprintf(os.Stdout, "| %s | %s | %d | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.FileName),
			item.Attributes.FileSize,
			escapeMarkdown(state),
		)
	}
	return nil
}

func printGameCenterActivityImageUploadResultTable(result *GameCenterActivityImageUploadResult) error {
	headers := []string{"ID", "Localization ID", "File Name", "File Size", "Delivery State", "Uploaded"}
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

func printGameCenterActivityImageUploadResultMarkdown(result *GameCenterActivityImageUploadResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Localization ID | File Name | File Size | Delivery State | Uploaded |")
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

func printGameCenterActivityImageDeleteResultTable(result *GameCenterActivityImageDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterActivityImageDeleteResultMarkdown(result *GameCenterActivityImageDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n", escapeMarkdown(result.ID), result.Deleted)
	return nil
}

func printGameCenterActivityReleasesTable(resp *GameCenterActivityVersionReleasesResponse) error {
	headers := []string{"ID"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{item.ID})
	}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterActivityReleasesMarkdown(resp *GameCenterActivityVersionReleasesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID |")
	fmt.Fprintln(os.Stdout, "| --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s |\n", escapeMarkdown(item.ID))
	}
	return nil
}

func printGameCenterActivityReleaseDeleteResultTable(result *GameCenterActivityVersionReleaseDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterActivityReleaseDeleteResultMarkdown(result *GameCenterActivityVersionReleaseDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n", escapeMarkdown(result.ID), result.Deleted)
	return nil
}

func printGameCenterGroupsTable(resp *GameCenterGroupsResponse) error {
	headers := []string{"ID", "Reference Name"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.ReferenceName),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterGroupsMarkdown(resp *GameCenterGroupsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Reference Name |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.ReferenceName),
		)
	}
	return nil
}

func printGameCenterGroupDeleteResultTable(result *GameCenterGroupDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterGroupDeleteResultMarkdown(result *GameCenterGroupDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n", escapeMarkdown(result.ID), result.Deleted)
	return nil
}

func printGameCenterAppVersionsTable(resp *GameCenterAppVersionsResponse) error {
	headers := []string{"ID", "Enabled"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{item.ID, fmt.Sprintf("%t", item.Attributes.Enabled)})
	}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterAppVersionsMarkdown(resp *GameCenterAppVersionsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Enabled |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %t |\n", escapeMarkdown(item.ID), item.Attributes.Enabled)
	}
	return nil
}

func printGameCenterEnabledVersionsTable(resp *GameCenterEnabledVersionsResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printGameCenterEnabledVersionsMarkdown(resp *GameCenterEnabledVersionsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Platform | Version | Icon Template URL |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- |")
	for _, item := range resp.Data {
		iconURL := ""
		if item.Attributes.IconAsset != nil {
			iconURL = item.Attributes.IconAsset.TemplateURL
		}
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(string(item.Attributes.Platform)),
			escapeMarkdown(item.Attributes.VersionString),
			escapeMarkdown(iconURL),
		)
	}
	return nil
}

func printGameCenterDetailsTable(resp *GameCenterDetailsResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printGameCenterDetailsMarkdown(resp *GameCenterDetailsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Arcade Enabled | Challenge Enabled | Leaderboard Enabled | Leaderboard Set Enabled | Achievement Enabled | Multiplayer Session | Turn-Based Session |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %t | %t | %t | %t | %t | %t | %t |\n",
			escapeMarkdown(item.ID),
			item.Attributes.ArcadeEnabled,
			item.Attributes.ChallengeEnabled,
			item.Attributes.LeaderboardEnabled,
			item.Attributes.LeaderboardSetEnabled,
			item.Attributes.AchievementEnabled,
			item.Attributes.MultiplayerSessionEnabled,
			item.Attributes.MultiplayerTurnBasedSessionEnabled,
		)
	}
	return nil
}

func printGameCenterMatchmakingQueuesTable(resp *GameCenterMatchmakingQueuesResponse) error {
	headers := []string{"ID", "Reference Name", "Classic Bundle IDs"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			compactWhitespace(item.Attributes.ReferenceName),
			formatStringList(item.Attributes.ClassicMatchmakingBundleIDs),
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterMatchmakingQueuesMarkdown(resp *GameCenterMatchmakingQueuesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Reference Name | Classic Bundle IDs |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.ReferenceName),
			escapeMarkdown(formatStringList(item.Attributes.ClassicMatchmakingBundleIDs)),
		)
	}
	return nil
}

func printGameCenterMatchmakingQueueDeleteResultTable(result *GameCenterMatchmakingQueueDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterMatchmakingQueueDeleteResultMarkdown(result *GameCenterMatchmakingQueueDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n", escapeMarkdown(result.ID), result.Deleted)
	return nil
}

func printGameCenterMatchmakingRuleSetsTable(resp *GameCenterMatchmakingRuleSetsResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printGameCenterMatchmakingRuleSetsMarkdown(resp *GameCenterMatchmakingRuleSetsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Reference Name | Language | Min Players | Max Players |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %d | %d | %d |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.ReferenceName),
			item.Attributes.RuleLanguageVersion,
			item.Attributes.MinPlayers,
			item.Attributes.MaxPlayers,
		)
	}
	return nil
}

func printGameCenterMatchmakingRuleSetDeleteResultTable(result *GameCenterMatchmakingRuleSetDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterMatchmakingRuleSetDeleteResultMarkdown(result *GameCenterMatchmakingRuleSetDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n", escapeMarkdown(result.ID), result.Deleted)
	return nil
}

func printGameCenterMatchmakingRulesTable(resp *GameCenterMatchmakingRulesResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printGameCenterMatchmakingRulesMarkdown(resp *GameCenterMatchmakingRulesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Reference Name | Type | Weight |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %g |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.ReferenceName),
			escapeMarkdown(item.Attributes.Type),
			item.Attributes.Weight,
		)
	}
	return nil
}

func printGameCenterMatchmakingRuleDeleteResultTable(result *GameCenterMatchmakingRuleDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterMatchmakingRuleDeleteResultMarkdown(result *GameCenterMatchmakingRuleDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n", escapeMarkdown(result.ID), result.Deleted)
	return nil
}

func printGameCenterMatchmakingTeamsTable(resp *GameCenterMatchmakingTeamsResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printGameCenterMatchmakingTeamsMarkdown(resp *GameCenterMatchmakingTeamsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Reference Name | Min Players | Max Players |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %d | %d |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.ReferenceName),
			item.Attributes.MinPlayers,
			item.Attributes.MaxPlayers,
		)
	}
	return nil
}

func printGameCenterMatchmakingTeamDeleteResultTable(result *GameCenterMatchmakingTeamDeleteResult) error {
	headers := []string{"ID", "Deleted"}
	rows := [][]string{{result.ID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterMatchmakingTeamDeleteResultMarkdown(result *GameCenterMatchmakingTeamDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n", escapeMarkdown(result.ID), result.Deleted)
	return nil
}

func printGameCenterMetricsTable(resp *GameCenterMetricsResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printGameCenterMetricsMarkdown(resp *GameCenterMetricsResponse) error {
	fmt.Fprintln(os.Stdout, "| Start | End | Granularity | Values | Dimensions |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		for _, point := range item.DataPoints {
			fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s |\n",
				escapeMarkdown(point.Start),
				escapeMarkdown(point.End),
				escapeMarkdown(formatMetricGranularity(item.Granularity)),
				escapeMarkdown(formatMetricJSON(point.Values)),
				escapeMarkdown(formatMetricJSON(item.Dimensions)),
			)
		}
	}
	return nil
}

func printGameCenterMatchmakingRuleSetTestTable(resp *GameCenterMatchmakingRuleSetTestResponse) error {
	headers := []string{"ID"}
	rows := [][]string{{resp.Data.ID}}
	RenderTable(headers, rows)
	return nil
}

func printGameCenterMatchmakingRuleSetTestMarkdown(resp *GameCenterMatchmakingRuleSetTestResponse) error {
	fmt.Fprintln(os.Stdout, "| ID |")
	fmt.Fprintln(os.Stdout, "| --- |")
	fmt.Fprintf(os.Stdout, "| %s |\n", escapeMarkdown(resp.Data.ID))
	return nil
}

func printGameCenterLeaderboardEntrySubmissionTable(resp *GameCenterLeaderboardEntrySubmissionResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printGameCenterLeaderboardEntrySubmissionMarkdown(resp *GameCenterLeaderboardEntrySubmissionResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Vendor ID | Score | Bundle ID | Scoped Player ID | Submitted Date |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- |")
	attrs := resp.Data.Attributes
	submittedDate := ""
	if attrs.SubmittedDate != nil {
		submittedDate = *attrs.SubmittedDate
	}
	fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s | %s |\n",
		escapeMarkdown(resp.Data.ID),
		escapeMarkdown(attrs.VendorIdentifier),
		escapeMarkdown(attrs.Score),
		escapeMarkdown(attrs.BundleID),
		escapeMarkdown(attrs.ScopedPlayerID),
		escapeMarkdown(submittedDate),
	)
	return nil
}

func printGameCenterPlayerAchievementSubmissionTable(resp *GameCenterPlayerAchievementSubmissionResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printGameCenterPlayerAchievementSubmissionMarkdown(resp *GameCenterPlayerAchievementSubmissionResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Vendor ID | Percent | Bundle ID | Scoped Player ID | Submitted Date |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- |")
	attrs := resp.Data.Attributes
	submittedDate := ""
	if attrs.SubmittedDate != nil {
		submittedDate = *attrs.SubmittedDate
	}
	fmt.Fprintf(os.Stdout, "| %s | %s | %d | %s | %s | %s |\n",
		escapeMarkdown(resp.Data.ID),
		escapeMarkdown(attrs.VendorIdentifier),
		attrs.PercentageAchieved,
		escapeMarkdown(attrs.BundleID),
		escapeMarkdown(attrs.ScopedPlayerID),
		escapeMarkdown(submittedDate),
	)
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
