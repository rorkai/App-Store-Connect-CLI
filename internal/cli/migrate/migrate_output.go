package migrate

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func printMigrateImportResultMarkdown(result *MigrateImportResult) error {
	defer renderMigrateStoreAssets(result, asc.RenderMarkdown)
	if result.DryRun {
		fmt.Println("## Dry Run - No changes made")
		fmt.Println()
	}
	if result.Status == migratePartialStatus || result.Status == "failed" {
		if result.Status == migratePartialStatus {
			fmt.Printf("## Partial Import - failed at %s\n\n", result.FailureStage)
		} else {
			fmt.Printf("## Failed Import - failed at %s\n\n", result.FailureStage)
		}
		if result.Failure != "" {
			fmt.Printf("**Failure:** %s\n\n", result.Failure)
		}
		if len(result.CompletedStages) > 0 {
			fmt.Printf("**Completed Stages:** %s\n\n", strings.Join(result.CompletedStages, ", "))
		}
	}
	fmt.Printf("**Version ID:** %s\n\n", result.VersionID)
	if result.AppID != "" {
		fmt.Printf("**App ID:** %s\n\n", result.AppID)
	}
	if result.DeliverfilePath != "" {
		fmt.Printf("**Deliverfile:** %s\n\n", result.DeliverfilePath)
	}
	if result.MetadataDir != "" {
		fmt.Printf("**Metadata Dir:** %s\n\n", result.MetadataDir)
	}
	if result.ScreenshotsDir != "" {
		fmt.Printf("**Screenshots Dir:** %s\n\n", result.ScreenshotsDir)
	}
	if len(result.Locales) > 0 {
		fmt.Printf("**Locales:** %s\n\n", strings.Join(result.Locales, ", "))
	}

	if len(result.MetadataFiles) > 0 {
		fmt.Println("### Version Metadata Files")
		fmt.Println()
		headers := []string{"Locale", "Files"}
		rows := make([][]string, 0, len(result.MetadataFiles))
		for _, item := range result.MetadataFiles {
			rows = append(rows, []string{item.Locale, strings.Join(item.Files, "<br>")})
		}
		asc.RenderMarkdown(headers, rows)
	}

	if len(result.AppInfoFiles) > 0 {
		fmt.Println()
		fmt.Println("### App Info Metadata Files")
		fmt.Println()
		headers := []string{"Locale", "Files"}
		rows := make([][]string, 0, len(result.AppInfoFiles))
		for _, item := range result.AppInfoFiles {
			rows = append(rows, []string{item.Locale, strings.Join(item.Files, "<br>")})
		}
		asc.RenderMarkdown(headers, rows)
	}

	if result.ReviewInformation != nil {
		fmt.Println()
		fmt.Println("### Review Information")
		fmt.Println()
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
			asc.RenderMarkdown(headers, rows)
		}
	}

	if len(result.ScreenshotPlan) > 0 {
		fmt.Println()
		fmt.Println("### Screenshot Plan")
		fmt.Println()
		headers := []string{"Locale", "Display Type", "Files"}
		rows := make([][]string, 0, len(result.ScreenshotPlan))
		for _, plan := range result.ScreenshotPlan {
			rows = append(rows, []string{plan.Locale, plan.DisplayType, strings.Join(plan.Files, "<br>")})
		}
		asc.RenderMarkdown(headers, rows)
	}

	if len(result.Skipped) > 0 {
		fmt.Println()
		fmt.Println("### Skipped")
		fmt.Println()
		headers := []string{"Path", "Reason"}
		rows := make([][]string, 0, len(result.Skipped))
		for _, item := range result.Skipped {
			rows = append(rows, []string{item.Path, item.Reason})
		}
		asc.RenderMarkdown(headers, rows)
	}

	if len(result.Uploaded) > 0 {
		fmt.Println()
		fmt.Println("### Version Metadata Uploaded")
		fmt.Println()
		headers := []string{"Locale", "Fields", "Action"}
		rows := make([][]string, 0, len(result.Uploaded))
		for _, item := range result.Uploaded {
			rows = append(rows, []string{item.Locale, fmt.Sprintf("%d", item.Fields), item.Action})
		}
		asc.RenderMarkdown(headers, rows)
	}

	if len(result.AppInfoUploaded) > 0 {
		fmt.Println()
		fmt.Println("### App Info Uploaded")
		fmt.Println()
		headers := []string{"Locale", "Fields", "Action"}
		rows := make([][]string, 0, len(result.AppInfoUploaded))
		for _, item := range result.AppInfoUploaded {
			rows = append(rows, []string{item.Locale, fmt.Sprintf("%d", item.Fields), item.Action})
		}
		asc.RenderMarkdown(headers, rows)
	}

	if result.ReviewInfoResult != nil {
		fmt.Println()
		fmt.Println("### Review Information Result")
		fmt.Println()
		fmt.Printf("- %s (%s)\n", result.ReviewInfoResult.Action, result.ReviewInfoResult.DetailID)
	}

	if len(result.ScreenshotResults) > 0 {
		fmt.Println()
		fmt.Println("### Screenshot Uploads")
		fmt.Println()
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
		asc.RenderMarkdown(headers, rows)
	}

	return nil
}

func printMigrateImportResultTable(result *MigrateImportResult) error {
	defer renderMigrateStoreAssets(result, asc.RenderTable)
	if result.DryRun {
		fmt.Println("DRY RUN - No changes made")
		fmt.Println()
	}
	if result.Status == migratePartialStatus || result.Status == "failed" {
		if result.Status == migratePartialStatus {
			fmt.Printf("PARTIAL IMPORT - failed at %s\n", result.FailureStage)
		} else {
			fmt.Printf("FAILED IMPORT - failed at %s\n", result.FailureStage)
		}
		if result.Failure != "" {
			fmt.Printf("Failure: %s\n", result.Failure)
		}
		if len(result.CompletedStages) > 0 {
			fmt.Printf("Completed Stages: %s\n", strings.Join(result.CompletedStages, ", "))
		}
		fmt.Println()
	}
	fmt.Printf("Version ID: %s\n\n", result.VersionID)
	if result.AppID != "" {
		fmt.Printf("App ID: %s\n\n", result.AppID)
	}

	if len(result.MetadataFiles) > 0 {
		fmt.Println("Version Metadata Files:")
		headers := []string{"Locale", "Files"}
		rows := make([][]string, 0, len(result.MetadataFiles))
		for _, item := range result.MetadataFiles {
			rows = append(rows, []string{item.Locale, strings.Join(item.Files, ", ")})
		}
		asc.RenderTable(headers, rows)
	}

	if len(result.AppInfoFiles) > 0 {
		fmt.Println()
		fmt.Println("App Info Metadata Files:")
		headers := []string{"Locale", "Files"}
		rows := make([][]string, 0, len(result.AppInfoFiles))
		for _, item := range result.AppInfoFiles {
			rows = append(rows, []string{item.Locale, strings.Join(item.Files, ", ")})
		}
		asc.RenderTable(headers, rows)
	}

	if result.ReviewInformation != nil {
		fmt.Println()
		fmt.Println("Review Information:")
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
			asc.RenderTable(headers, rows)
		}
	}

	if len(result.ScreenshotPlan) > 0 {
		fmt.Println()
		fmt.Println("Screenshot Plan:")
		headers := []string{"Locale", "Display Type", "Files"}
		rows := make([][]string, 0, len(result.ScreenshotPlan))
		for _, plan := range result.ScreenshotPlan {
			rows = append(rows, []string{plan.Locale, plan.DisplayType, strings.Join(plan.Files, ", ")})
		}
		asc.RenderTable(headers, rows)
	}

	if len(result.Skipped) > 0 {
		fmt.Println()
		fmt.Println("Skipped:")
		headers := []string{"Path", "Reason"}
		rows := make([][]string, 0, len(result.Skipped))
		for _, item := range result.Skipped {
			rows = append(rows, []string{item.Path, item.Reason})
		}
		asc.RenderTable(headers, rows)
	}

	if len(result.Uploaded) > 0 {
		fmt.Println()
		fmt.Println("Version Metadata Uploaded:")
		headers := []string{"Locale", "Fields", "Action"}
		rows := make([][]string, 0, len(result.Uploaded))
		for _, item := range result.Uploaded {
			rows = append(rows, []string{item.Locale, fmt.Sprintf("%d", item.Fields), item.Action})
		}
		asc.RenderTable(headers, rows)
	}

	if len(result.AppInfoUploaded) > 0 {
		fmt.Println()
		fmt.Println("App Info Uploaded:")
		headers := []string{"Locale", "Fields", "Action"}
		rows := make([][]string, 0, len(result.AppInfoUploaded))
		for _, item := range result.AppInfoUploaded {
			rows = append(rows, []string{item.Locale, fmt.Sprintf("%d", item.Fields), item.Action})
		}
		asc.RenderTable(headers, rows)
	}

	if result.ReviewInfoResult != nil {
		fmt.Println()
		fmt.Println("Review Information Result:")
		fmt.Printf("%s (%s)\n", result.ReviewInfoResult.Action, result.ReviewInfoResult.DetailID)
	}

	if len(result.ScreenshotResults) > 0 {
		fmt.Println()
		fmt.Println("Screenshot Uploads:")
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
		asc.RenderTable(headers, rows)
	}

	return nil
}

func printMigrateExportResultMarkdown(result *MigrateExportResult) error {
	if result.Failure != "" {
		fmt.Printf("Failure: %s\n", result.Failure)
	}
	if len(result.AssetFiles) > 0 {
		rows := [][]string{}
		for _, path := range result.AssetFiles {
			rows = append(rows, []string{path})
		}
		asc.RenderMarkdown([]string{"Asset file"}, rows)
	}
	fmt.Printf("**Version ID:** %s\n\n", result.VersionID)
	fmt.Printf("**Output Directory:** %s\n\n", result.OutputDir)
	fmt.Println("### Exported Locales")
	fmt.Println()
	for _, locale := range result.Locales {
		fmt.Printf("- %s\n", locale)
	}
	fmt.Printf("\n**Total Files:** %d\n", result.TotalFiles)
	return nil
}

func printMigrateExportResultTable(result *MigrateExportResult) error {
	if result.Failure != "" {
		fmt.Printf("Failure: %s\n", result.Failure)
	}
	if len(result.AssetFiles) > 0 {
		rows := [][]string{}
		for _, path := range result.AssetFiles {
			rows = append(rows, []string{path})
		}
		asc.RenderTable([]string{"Asset file"}, rows)
	}
	fmt.Printf("Version ID: %s\n", result.VersionID)
	fmt.Printf("Output Dir: %s\n\n", result.OutputDir)
	headers := []string{"Locale"}
	rows := make([][]string, 0, len(result.Locales))
	for _, locale := range result.Locales {
		rows = append(rows, []string{locale})
	}
	asc.RenderTable(headers, rows)
	fmt.Printf("\nTotal Files: %d\n", result.TotalFiles)
	return nil
}

func printMigrateValidateResultMarkdown(result *MigrateValidateResult) error {
	fmt.Printf("**Fastlane Directory:** %s\n\n", result.FastlaneDir)

	// Summary
	if result.Valid {
		fmt.Println("## ✓ Validation Passed")
	} else {
		fmt.Println("## ✗ Validation Failed")
	}
	fmt.Println()
	fmt.Printf("- **Locales:** %d\n", len(result.Locales))
	fmt.Printf("- **Errors:** %d\n", result.ErrorCount)
	fmt.Printf("- **Warnings:** %d\n", result.WarnCount)

	if len(result.Issues) > 0 {
		fmt.Println()
		fmt.Println("### Issues")
		fmt.Println()
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
		asc.RenderMarkdown(headers, rows)
	}

	if len(result.Skipped) > 0 {
		fmt.Println()
		fmt.Println("### Skipped")
		fmt.Println()
		headers := []string{"Path", "Reason"}
		rows := make([][]string, 0, len(result.Skipped))
		for _, item := range result.Skipped {
			rows = append(rows, []string{item.Path, item.Reason})
		}
		asc.RenderMarkdown(headers, rows)
	}

	return nil
}

func printMigrateValidateResultTable(result *MigrateValidateResult) error {
	fmt.Printf("Fastlane Dir: %s\n\n", result.FastlaneDir)

	// Summary
	if result.Valid {
		fmt.Println("VALIDATION PASSED")
	} else {
		fmt.Println("VALIDATION FAILED")
	}
	fmt.Printf("Locales: %d  Errors: %d  Warnings: %d\n", len(result.Locales), result.ErrorCount, result.WarnCount)

	if len(result.Issues) > 0 {
		fmt.Println()
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
		asc.RenderTable(headers, rows)
	}

	if len(result.Skipped) > 0 {
		fmt.Println()
		fmt.Println("Skipped:")
		headers := []string{"Path", "Reason"}
		rows := make([][]string, 0, len(result.Skipped))
		for _, item := range result.Skipped {
			rows = append(rows, []string{item.Path, item.Reason})
		}
		asc.RenderTable(headers, rows)
	}

	return nil
}

func renderMigrateStoreAssets(result *MigrateImportResult, render func([]string, [][]string)) {
	rows := [][]string{}
	if result.DryRun {
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
		fmt.Println("Store assets:")
		render([]string{"Kind", "Locale", "File", "Action", "Status", "Details"}, rows)
	}
}
