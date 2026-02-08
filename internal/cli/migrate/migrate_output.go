package migrate

import (
	"fmt"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func printMigrateImportResultMarkdown(result *MigrateImportResult) error {
	if result.DryRun {
		fmt.Println("## Dry Run - No changes made")
		fmt.Println()
	}
	fmt.Printf("**Version ID:** %s\n\n", result.VersionID)

	// Version localizations
	fmt.Println("### Version Localizations Found")
	fmt.Println()
	{
		headers := []string{"Locale", "Fields"}
		rows := make([][]string, 0, len(result.Localizations))
		for _, loc := range result.Localizations {
			rows = append(rows, []string{loc.Locale, fmt.Sprintf("%d", countNonEmptyFields(loc))})
		}
		asc.RenderMarkdown(headers, rows)
	}

	// App Info localizations
	if len(result.AppInfoLocalizations) > 0 {
		fmt.Println()
		fmt.Println("### App Info Localizations Found")
		fmt.Println()
		headers := []string{"Locale", "Name", "Subtitle"}
		rows := make([][]string, 0, len(result.AppInfoLocalizations))
		for _, loc := range result.AppInfoLocalizations {
			name := "-"
			if loc.Name != "" {
				name = "✓"
			}
			subtitle := "-"
			if loc.Subtitle != "" {
				subtitle = "✓"
			}
			rows = append(rows, []string{loc.Locale, name, subtitle})
		}
		asc.RenderMarkdown(headers, rows)
	}

	if len(result.Uploaded) > 0 {
		fmt.Println()
		fmt.Println("### Uploaded")
		fmt.Println()
		for _, u := range result.Uploaded {
			fmt.Printf("- %s (%d fields)\n", u.Locale, u.Fields)
		}
	}

	if len(result.AppInfoUploaded) > 0 {
		fmt.Println()
		fmt.Println("### App Info Uploaded")
		fmt.Println()
		for _, u := range result.AppInfoUploaded {
			fmt.Printf("- %s (%d fields)\n", u.Locale, u.Fields)
		}
	}

	return nil
}

func printMigrateImportResultTable(result *MigrateImportResult) error {
	if result.DryRun {
		fmt.Println("DRY RUN - No changes made")
		fmt.Println()
	}
	fmt.Printf("Version ID: %s\n\n", result.VersionID)

	// Version localizations
	fmt.Println("Version Localizations:")
	{
		headers := []string{"Locale", "Fields", "Status"}
		rows := make([][]string, 0, len(result.Localizations))
		for _, loc := range result.Localizations {
			status := "found"
			for _, u := range result.Uploaded {
				if u.Locale == loc.Locale {
					status = "uploaded"
					break
				}
			}
			rows = append(rows, []string{loc.Locale, fmt.Sprintf("%d", countNonEmptyFields(loc)), status})
		}
		asc.RenderTable(headers, rows)
	}

	// App Info localizations
	if len(result.AppInfoLocalizations) > 0 {
		fmt.Println()
		fmt.Println("App Info Localizations:")
		headers := []string{"Locale", "Name", "Subtitle", "Status"}
		rows := make([][]string, 0, len(result.AppInfoLocalizations))
		for _, loc := range result.AppInfoLocalizations {
			status := "found"
			for _, u := range result.AppInfoUploaded {
				if u.Locale == loc.Locale {
					status = "uploaded"
					break
				}
			}
			name := "-"
			if loc.Name != "" {
				name = "yes"
			}
			subtitle := "-"
			if loc.Subtitle != "" {
				subtitle = "yes"
			}
			rows = append(rows, []string{loc.Locale, name, subtitle, status})
		}
		asc.RenderTable(headers, rows)
	}

	return nil
}

func printMigrateExportResultMarkdown(result *MigrateExportResult) error {
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

	return nil
}
