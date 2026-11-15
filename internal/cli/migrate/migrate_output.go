package migrate

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func printMigrateImportResultMarkdown(d *shared.Deps, result *MigrateImportResult) error {
	defer renderMigrateStoreAssets(d, result, asc.RenderMarkdownTo)
	if result.DryRun {
		fmt.Fprintln(d.IO.Out, "## Dry Run - No changes made")
		fmt.Fprintln(d.IO.Out)
	}
	if result.Status == migratePartialStatus || result.Status == "failed" {
		if result.Status == migratePartialStatus {
			fmt.Fprintf(d.IO.Out, "## Partial Import - failed at %s\n\n", result.FailureStage)
		} else {
			fmt.Fprintf(d.IO.Out, "## Failed Import - failed at %s\n\n", result.FailureStage)
		}
		if result.Failure != "" {
			fmt.Fprintf(d.IO.Out, "**Failure:** %s\n\n", result.Failure)
		}
		if len(result.CompletedStages) > 0 {
			fmt.Fprintf(d.IO.Out, "**Completed Stages:** %s\n\n", strings.Join(result.CompletedStages, ", "))
		}
	}
	fmt.Fprintf(d.IO.Out, "**Version ID:** %s\n\n", result.VersionID)
	if result.AppID != "" {
		fmt.Fprintf(d.IO.Out, "**App ID:** %s\n\n", result.AppID)
	}
	if result.Copyright != "" {
		fmt.Fprintf(d.IO.Out, "**Copyright:** %s\n\n", result.Copyright)
	}
	if result.DeliverfilePath != "" {
		fmt.Fprintf(d.IO.Out, "**Deliverfile:** %s\n\n", result.DeliverfilePath)
	}
	if result.MetadataDir != "" {
		fmt.Fprintf(d.IO.Out, "**Metadata Dir:** %s\n\n", result.MetadataDir)
	}
	if result.ScreenshotsDir != "" {
		fmt.Fprintf(d.IO.Out, "**Screenshots Dir:** %s\n\n", result.ScreenshotsDir)
	}
	if len(result.Locales) > 0 {
		fmt.Fprintf(d.IO.Out, "**Locales:** %s\n\n", strings.Join(result.Locales, ", "))
	}

	if len(result.MetadataFiles) > 0 {
		fmt.Fprintln(d.IO.Out, "### Version Metadata Files")
		fmt.Fprintln(d.IO.Out)
		headers := []string{"Locale", "Files"}
		rows := make([][]string, 0, len(result.MetadataFiles))
		for _, item := range result.MetadataFiles {
			rows = append(rows, []string{item.Locale, strings.Join(item.Files, "<br>")})
		}
		asc.RenderMarkdownTo(d.IO.Out, headers, rows)
	}

	if len(result.AppInfoFiles) > 0 {
		fmt.Fprintln(d.IO.Out)
		fmt.Fprintln(d.IO.Out, "### App Info Metadata Files")
		fmt.Fprintln(d.IO.Out)
		headers := []string{"Locale", "Files"}
		rows := make([][]string, 0, len(result.AppInfoFiles))
		for _, item := range result.AppInfoFiles {
			rows = append(rows, []string{item.Locale, strings.Join(item.Files, "<br>")})
		}
		asc.RenderMarkdownTo(d.IO.Out, headers, rows)
	}

	if result.ReviewInformation != nil {
		fmt.Fprintln(d.IO.Out)
		fmt.Fprintln(d.IO.Out, "### Review Information")
		fmt.Fprintln(d.IO.Out)
		headers := []string{"Field", "Value"}
		rows := make([][]string, 0, 8)
		addReviewRow := func(label string, value *string) {
			if value == nil {
				return
			}
			rows = append(rows, []string{label, *value})
		}
		addReviewRow("Contact First Name", result.ReviewInformation.ContactFirstName)
		addReviewRow("Contact Last Name", result.ReviewInformation.ContactLastName)
		addReviewRow("Contact Phone", result.ReviewInformation.ContactPhone)
		addReviewRow("Contact Email", result.ReviewInformation.ContactEmail)
		addReviewRow("Demo Account Name", result.ReviewInformation.DemoAccountName)
		addReviewRow("Demo Account Password", result.ReviewInformation.DemoAccountPassword)
		addReviewRow("Notes", result.ReviewInformation.Notes)
		if result.ReviewInformation.DemoAccountRequired != nil {
			rows = append(rows, []string{"Demo Account Required", fmt.Sprintf("%t", *result.ReviewInformation.DemoAccountRequired)})
		}
		if len(rows) > 0 {
			asc.RenderMarkdownTo(d.IO.Out, headers, rows)
		}
	}

	if len(result.ScreenshotPlan) > 0 {
		fmt.Fprintln(d.IO.Out)
		fmt.Fprintln(d.IO.Out, "### Screenshot Plan")
		fmt.Fprintln(d.IO.Out)
		headers := []string{"Locale", "Display Type", "Files"}
		rows := make([][]string, 0, len(result.ScreenshotPlan))
		for _, plan := range result.ScreenshotPlan {
			rows = append(rows, []string{plan.Locale, plan.DisplayType, strings.Join(plan.Files, "<br>")})
		}
		asc.RenderMarkdownTo(d.IO.Out, headers, rows)
	}

	if len(result.Skipped) > 0 {
		fmt.Fprintln(d.IO.Out)
		fmt.Fprintln(d.IO.Out, "### Skipped")
		fmt.Fprintln(d.IO.Out)
		headers := []string{"Path", "Reason"}
		rows := make([][]string, 0, len(result.Skipped))
		for _, item := range result.Skipped {
			rows = append(rows, []string{item.Path, item.Reason})
		}
		asc.RenderMarkdownTo(d.IO.Out, headers, rows)
	}

	if len(result.Uploaded) > 0 {
		fmt.Fprintln(d.IO.Out)
		fmt.Fprintln(d.IO.Out, "### Version Metadata Uploaded")
		fmt.Fprintln(d.IO.Out)
		headers := []string{"Locale", "Fields", "Action"}
		rows := make([][]string, 0, len(result.Uploaded))
		for _, item := range result.Uploaded {
			rows = append(rows, []string{item.Locale, fmt.Sprintf("%d", item.Fields), item.Action})
		}
		asc.RenderMarkdownTo(d.IO.Out, headers, rows)
	}

	if len(result.AppInfoUploaded) > 0 {
		fmt.Fprintln(d.IO.Out)
		fmt.Fprintln(d.IO.Out, "### App Info Uploaded")
		fmt.Fprintln(d.IO.Out)
		headers := []string{"Locale", "Fields", "Action"}
		rows := make([][]string, 0, len(result.AppInfoUploaded))
		for _, item := range result.AppInfoUploaded {
			rows = append(rows, []string{item.Locale, fmt.Sprintf("%d", item.Fields), item.Action})
		}
		asc.RenderMarkdownTo(d.IO.Out, headers, rows)
	}

	if result.ReviewInfoResult != nil {
		fmt.Fprintln(d.IO.Out)
		fmt.Fprintln(d.IO.Out, "### Review Information Result")
		fmt.Fprintln(d.IO.Out)
		fmt.Fprintf(d.IO.Out, "- %s (%s)\n", result.ReviewInfoResult.Action, result.ReviewInfoResult.DetailID)
	}

	if len(result.ScreenshotResults) > 0 {
		fmt.Fprintln(d.IO.Out)
		fmt.Fprintln(d.IO.Out, "### Screenshot Uploads")
		fmt.Fprintln(d.IO.Out)
		headers := []string{"Locale", "Display Type", "Uploaded", "Skipped"}
		rows := make([][]string, 0, len(result.ScreenshotResults))
		for _, item := range result.ScreenshotResults {
			rows = append(rows, []string{
				item.Locale,
				item.DisplayType,
				fmt.Sprintf("%d", len(item.Uploaded)),
				fmt.Sprintf("%d", len(item.Skipped)),
			})
		}
		asc.RenderMarkdownTo(d.IO.Out, headers, rows)
	}

	return nil
}

func printMigrateImportResultTable(d *shared.Deps, result *MigrateImportResult) error {
	defer renderMigrateStoreAssets(d, result, asc.RenderTableTo)
	if result.DryRun {
		fmt.Fprintln(d.IO.Out, "DRY RUN - No changes made")
		fmt.Fprintln(d.IO.Out)
	}
	if result.Status == migratePartialStatus || result.Status == "failed" {
		if result.Status == migratePartialStatus {
			fmt.Fprintf(d.IO.Out, "PARTIAL IMPORT - failed at %s\n", result.FailureStage)
		} else {
			fmt.Fprintf(d.IO.Out, "FAILED IMPORT - failed at %s\n", result.FailureStage)
		}
		if result.Failure != "" {
			fmt.Fprintf(d.IO.Out, "Failure: %s\n", result.Failure)
		}
		if len(result.CompletedStages) > 0 {
			fmt.Fprintf(d.IO.Out, "Completed Stages: %s\n", strings.Join(result.CompletedStages, ", "))
		}
		fmt.Fprintln(d.IO.Out)
	}
	fmt.Fprintf(d.IO.Out, "Version ID: %s\n\n", result.VersionID)
	if result.AppID != "" {
		fmt.Fprintf(d.IO.Out, "App ID: %s\n\n", result.AppID)
	}
	if result.Copyright != "" {
		fmt.Fprintf(d.IO.Out, "Copyright: %s\n\n", result.Copyright)
	}

	if len(result.MetadataFiles) > 0 {
		fmt.Fprintln(d.IO.Out, "Version Metadata Files:")
		headers := []string{"Locale", "Files"}
		rows := make([][]string, 0, len(result.MetadataFiles))
		for _, item := range result.MetadataFiles {
			rows = append(rows, []string{item.Locale, strings.Join(item.Files, ", ")})
		}
		asc.RenderTableTo(d.IO.Out, headers, rows)
	}

	if len(result.AppInfoFiles) > 0 {
		fmt.Fprintln(d.IO.Out)
		fmt.Fprintln(d.IO.Out, "App Info Metadata Files:")
		headers := []string{"Locale", "Files"}
		rows := make([][]string, 0, len(result.AppInfoFiles))
		for _, item := range result.AppInfoFiles {
			rows = append(rows, []string{item.Locale, strings.Join(item.Files, ", ")})
		}
		asc.RenderTableTo(d.IO.Out, headers, rows)
	}

	if result.ReviewInformation != nil {
		fmt.Fprintln(d.IO.Out)
		fmt.Fprintln(d.IO.Out, "Review Information:")
		headers := []string{"Field", "Value"}
		rows := make([][]string, 0, 8)
		addReviewRow := func(label string, value *string) {
			if value == nil {
				return
			}
			rows = append(rows, []string{label, *value})
		}
		addReviewRow("Contact First Name", result.ReviewInformation.ContactFirstName)
		addReviewRow("Contact Last Name", result.ReviewInformation.ContactLastName)
		addReviewRow("Contact Phone", result.ReviewInformation.ContactPhone)
		addReviewRow("Contact Email", result.ReviewInformation.ContactEmail)
		addReviewRow("Demo Account Name", result.ReviewInformation.DemoAccountName)
		addReviewRow("Demo Account Password", result.ReviewInformation.DemoAccountPassword)
		addReviewRow("Notes", result.ReviewInformation.Notes)
		if result.ReviewInformation.DemoAccountRequired != nil {
			rows = append(rows, []string{"Demo Account Required", fmt.Sprintf("%t", *result.ReviewInformation.DemoAccountRequired)})
		}
		if len(rows) > 0 {
			asc.RenderTableTo(d.IO.Out, headers, rows)
		}
	}

	if len(result.ScreenshotPlan) > 0 {
		fmt.Fprintln(d.IO.Out)
		fmt.Fprintln(d.IO.Out, "Screenshot Plan:")
		headers := []string{"Locale", "Display Type", "Files"}
		rows := make([][]string, 0, len(result.ScreenshotPlan))
		for _, plan := range result.ScreenshotPlan {
			rows = append(rows, []string{plan.Locale, plan.DisplayType, strings.Join(plan.Files, ", ")})
		}
		asc.RenderTableTo(d.IO.Out, headers, rows)
	}

	if len(result.Skipped) > 0 {
		fmt.Fprintln(d.IO.Out)
		fmt.Fprintln(d.IO.Out, "Skipped:")
		headers := []string{"Path", "Reason"}
		rows := make([][]string, 0, len(result.Skipped))
		for _, item := range result.Skipped {
			rows = append(rows, []string{item.Path, item.Reason})
		}
		asc.RenderTableTo(d.IO.Out, headers, rows)
	}

	if len(result.Uploaded) > 0 {
		fmt.Fprintln(d.IO.Out)
		fmt.Fprintln(d.IO.Out, "Version Metadata Uploaded:")
		headers := []string{"Locale", "Fields", "Action"}
		rows := make([][]string, 0, len(result.Uploaded))
		for _, item := range result.Uploaded {
			rows = append(rows, []string{item.Locale, fmt.Sprintf("%d", item.Fields), item.Action})
		}
		asc.RenderTableTo(d.IO.Out, headers, rows)
	}

	if len(result.AppInfoUploaded) > 0 {
		fmt.Fprintln(d.IO.Out)
		fmt.Fprintln(d.IO.Out, "App Info Uploaded:")
		headers := []string{"Locale", "Fields", "Action"}
		rows := make([][]string, 0, len(result.AppInfoUploaded))
		for _, item := range result.AppInfoUploaded {
			rows = append(rows, []string{item.Locale, fmt.Sprintf("%d", item.Fields), item.Action})
		}
		asc.RenderTableTo(d.IO.Out, headers, rows)
	}

	if result.ReviewInfoResult != nil {
		fmt.Fprintln(d.IO.Out)
		fmt.Fprintln(d.IO.Out, "Review Information Result:")
		fmt.Fprintf(d.IO.Out, "%s (%s)\n", result.ReviewInfoResult.Action, result.ReviewInfoResult.DetailID)
	}

	if len(result.ScreenshotResults) > 0 {
		fmt.Fprintln(d.IO.Out)
		fmt.Fprintln(d.IO.Out, "Screenshot Uploads:")
		headers := []string{"Locale", "Display Type", "Uploaded", "Skipped"}
		rows := make([][]string, 0, len(result.ScreenshotResults))
		for _, item := range result.ScreenshotResults {
			rows = append(rows, []string{
				item.Locale,
				item.DisplayType,
				fmt.Sprintf("%d", len(item.Uploaded)),
				fmt.Sprintf("%d", len(item.Skipped)),
			})
		}
		asc.RenderTableTo(d.IO.Out, headers, rows)
	}

	return nil
}

func printMigrateExportResultMarkdown(d *shared.Deps, result *MigrateExportResult) error {
	if result.Failure != "" {
		fmt.Fprintf(d.IO.Out, "Failure: %s\n", result.Failure)
	}
	if len(result.AssetFiles) > 0 {
		rows := [][]string{}
		for _, path := range result.AssetFiles {
			rows = append(rows, []string{path})
		}
		asc.RenderMarkdownTo(d.IO.Out, []string{"Asset file"}, rows)
	}
	fmt.Fprintf(d.IO.Out, "**Version ID:** %s\n\n", result.VersionID)
	fmt.Fprintf(d.IO.Out, "**Output Directory:** %s\n\n", result.OutputDir)
	fmt.Fprintln(d.IO.Out, "### Exported Locales")
	fmt.Fprintln(d.IO.Out)
	for _, locale := range result.Locales {
		fmt.Fprintf(d.IO.Out, "- %s\n", locale)
	}
	fmt.Fprintf(d.IO.Out, "\n**Total Files:** %d\n", result.TotalFiles)
	return nil
}

func printMigrateExportResultTable(d *shared.Deps, result *MigrateExportResult) error {
	if result.Failure != "" {
		fmt.Fprintf(d.IO.Out, "Failure: %s\n", result.Failure)
	}
	if len(result.AssetFiles) > 0 {
		rows := [][]string{}
		for _, path := range result.AssetFiles {
			rows = append(rows, []string{path})
		}
		asc.RenderTableTo(d.IO.Out, []string{"Asset file"}, rows)
	}
	fmt.Fprintf(d.IO.Out, "Version ID: %s\n", result.VersionID)
	fmt.Fprintf(d.IO.Out, "Output Dir: %s\n\n", result.OutputDir)
	headers := []string{"Locale"}
	rows := make([][]string, 0, len(result.Locales))
	for _, locale := range result.Locales {
		rows = append(rows, []string{locale})
	}
	asc.RenderTableTo(d.IO.Out, headers, rows)
	fmt.Fprintf(d.IO.Out, "\nTotal Files: %d\n", result.TotalFiles)
	return nil
}

func printMigrateValidateResultMarkdown(d *shared.Deps, result *MigrateValidateResult) error {
	fmt.Fprintf(d.IO.Out, "**Fastlane Directory:** %s\n\n", result.FastlaneDir)

	// Summary
	if result.Valid {
		fmt.Fprintln(d.IO.Out, "## ✓ Validation Passed")
	} else {
		fmt.Fprintln(d.IO.Out, "## ✗ Validation Failed")
	}
	fmt.Fprintln(d.IO.Out)
	fmt.Fprintf(d.IO.Out, "- **Locales:** %d\n", len(result.Locales))
	fmt.Fprintf(d.IO.Out, "- **Errors:** %d\n", result.ErrorCount)
	fmt.Fprintf(d.IO.Out, "- **Warnings:** %d\n", result.WarnCount)

	if len(result.Issues) > 0 {
		fmt.Fprintln(d.IO.Out)
		fmt.Fprintln(d.IO.Out, "### Issues")
		fmt.Fprintln(d.IO.Out)
		headers := []string{"Locale", "Field", "Severity", "Message", "Length", "Limit"}
		rows := make([][]string, 0, len(result.Issues))
		for _, issue := range result.Issues {
			length := "-"
			limit := "-"
			if issue.Length > 0 {
				length = fmt.Sprintf("%d", issue.Length)
			}
			if issue.Limit > 0 {
				limit = fmt.Sprintf("%d", issue.Limit)
			}
			rows = append(rows, []string{issue.Locale, issue.Field, issue.Severity, issue.Message, length, limit})
		}
		asc.RenderMarkdownTo(d.IO.Out, headers, rows)
	}

	if len(result.Skipped) > 0 {
		fmt.Fprintln(d.IO.Out)
		fmt.Fprintln(d.IO.Out, "### Skipped")
		fmt.Fprintln(d.IO.Out)
		headers := []string{"Path", "Reason"}
		rows := make([][]string, 0, len(result.Skipped))
		for _, item := range result.Skipped {
			rows = append(rows, []string{item.Path, item.Reason})
		}
		asc.RenderMarkdownTo(d.IO.Out, headers, rows)
	}

	return nil
}

func printMigrateValidateResultTable(d *shared.Deps, result *MigrateValidateResult) error {
	fmt.Fprintf(d.IO.Out, "Fastlane Dir: %s\n\n", result.FastlaneDir)

	// Summary
	if result.Valid {
		fmt.Fprintln(d.IO.Out, "VALIDATION PASSED")
	} else {
		fmt.Fprintln(d.IO.Out, "VALIDATION FAILED")
	}
	fmt.Fprintf(d.IO.Out, "Locales: %d  Errors: %d  Warnings: %d\n", len(result.Locales), result.ErrorCount, result.WarnCount)

	if len(result.Issues) > 0 {
		fmt.Fprintln(d.IO.Out)
		headers := []string{"Locale", "Field", "Severity", "Message", "Length", "Limit"}
		rows := make([][]string, 0, len(result.Issues))
		for _, issue := range result.Issues {
			length := "-"
			limit := "-"
			if issue.Length > 0 {
				length = fmt.Sprintf("%d", issue.Length)
			}
			if issue.Limit > 0 {
				limit = fmt.Sprintf("%d", issue.Limit)
			}
			rows = append(rows, []string{issue.Locale, issue.Field, issue.Severity, issue.Message, length, limit})
		}
		asc.RenderTableTo(d.IO.Out, headers, rows)
	}

	if len(result.Skipped) > 0 {
		fmt.Fprintln(d.IO.Out)
		fmt.Fprintln(d.IO.Out, "Skipped:")
		headers := []string{"Path", "Reason"}
		rows := make([][]string, 0, len(result.Skipped))
		for _, item := range result.Skipped {
			rows = append(rows, []string{item.Path, item.Reason})
		}
		asc.RenderTableTo(d.IO.Out, headers, rows)
	}

	return nil
}

func renderMigrateStoreAssets(d *shared.Deps, result *MigrateImportResult, render func(io.Writer, []string, [][]string)) {
	rows := [][]string{}
	if result.DryRun && !result.RemoteChecked {
		if result.AppClip != nil {
			if result.AppClip.Action != "" {
				rows = append(rows, []string{"app_clip", "", "action.txt", result.AppClip.Action, "local input", ""})
			}
			locales := map[string]bool{}
			for locale := range result.AppClip.Subtitles {
				locales[locale] = true
			}
			for locale := range result.AppClip.HeaderImages {
				locales[locale] = true
			}
			ordered := make([]string, 0, len(locales))
			for locale := range locales {
				ordered = append(ordered, locale)
			}
			sort.Strings(ordered)
			for _, locale := range ordered {
				if value, exists := result.AppClip.Subtitles[locale]; exists {
					rows = append(rows, []string{"app_clip_subtitle", locale, "subtitle.txt", value, "local input", ""})
				}
				if path := result.AppClip.HeaderImages[locale]; path != "" {
					rows = append(rows, []string{"app_clip_header", locale, path, "import", "local input", ""})
				}
			}
		}
		for _, preview := range result.Previews {
			rows = append(rows, []string{"preview", preview.Locale, preview.Path, "import", "local input", ""})
		}
	} else {
		for _, item := range result.AssetResults {
			detail := item.Error
			if item.PreviousDeleted {
				detail += " previous header deleted: " + item.PreviousID
			}
			rows = append(rows, []string{item.Kind, item.Locale, item.Path, item.Action, item.Status, detail})
		}
	}
	if len(rows) > 0 {
		if result.RemoteChecked {
			fmt.Fprintln(d.IO.Out, "Remote store asset changes:")
		} else {
			fmt.Fprintln(d.IO.Out, "Store assets:")
		}
		render(d.IO.Out, []string{"Kind", "Locale", "File", "Action", "Status", "Details"}, rows)
	} else if result.RemoteChecked {
		fmt.Fprintln(d.IO.Out, "No remote store asset changes.")
	}
}
