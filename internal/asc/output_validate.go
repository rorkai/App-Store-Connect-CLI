package asc

import (
	"fmt"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

func init() {
	registerDirect(func(v *validation.Report, render func([]string, [][]string)) error {
		h, r := validationSummaryRows(v)
		render(h, r)
		oh, or := validationCheckRows(v)
		render(oh, or)
		return nil
	})

	registerDirect(func(v *validation.TestFlightReport, render func([]string, [][]string)) error {
		h, r := testflightValidationSummaryRows(v)
		render(h, r)
		oh, or := testflightValidationCheckRows(v)
		render(oh, or)
		return nil
	})

	registerDirect(func(v *validation.IAPReport, render func([]string, [][]string)) error {
		h, r := iapValidationSummaryRows(v)
		render(h, r)
		oh, or := iapValidationCheckRows(v)
		render(oh, or)
		return nil
	})

	registerDirect(func(v *validation.SubscriptionsReport, render func([]string, [][]string)) error {
		h, r := subscriptionsValidationSummaryRows(v)
		render(h, r)
		oh, or := subscriptionsValidationCheckRows(v)
		render(oh, or)
		dh, dr := subscriptionsValidationDiagnosticRows(v)
		render(dh, dr)
		return nil
	})
}

func validationSummaryRows(report *validation.Report) ([]string, [][]string) {
	headers := []string{"App ID", "Version ID", "Version", "Platform", "Errors", "Warnings", "Infos", "Blocking", "Strict"}
	rows := [][]string{{
		report.AppID,
		report.VersionID,
		report.VersionString,
		report.Platform,
		fmt.Sprintf("%d", report.Summary.Errors),
		fmt.Sprintf("%d", report.Summary.Warnings),
		fmt.Sprintf("%d", report.Summary.Infos),
		fmt.Sprintf("%d", report.Summary.Blocking),
		formatBool(report.Strict),
	}}
	return headers, rows
}

func validationCheckRows(report *validation.Report) ([]string, [][]string) {
	headers := []string{"Severity", "Check ID", "Locale", "Field", "Resource", "Message", "Remediation"}
	if report == nil || len(report.Checks) == 0 {
		return headers, [][]string{{"info", "validation.ok", "", "", "", "No issues found", ""}}
	}

	rows := make([][]string, 0, len(report.Checks))
	for _, check := range report.Checks {
		rows = append(rows, []string{
			string(check.Severity),
			check.ID,
			check.Locale,
			check.Field,
			formatResource(check.ResourceType, check.ResourceID),
			check.Message,
			check.Remediation,
		})
	}
	return headers, rows
}

func testflightValidationSummaryRows(report *validation.TestFlightReport) ([]string, [][]string) {
	headers := []string{"App ID", "Build ID", "Build Version", "Errors", "Warnings", "Infos", "Blocking", "Strict"}
	rows := [][]string{{
		report.AppID,
		report.BuildID,
		report.BuildVersion,
		fmt.Sprintf("%d", report.Summary.Errors),
		fmt.Sprintf("%d", report.Summary.Warnings),
		fmt.Sprintf("%d", report.Summary.Infos),
		fmt.Sprintf("%d", report.Summary.Blocking),
		formatBool(report.Strict),
	}}
	return headers, rows
}

func testflightValidationCheckRows(report *validation.TestFlightReport) ([]string, [][]string) {
	headers := []string{"Severity", "Check ID", "Locale", "Field", "Resource", "Message", "Remediation"}
	if report == nil || len(report.Checks) == 0 {
		return headers, [][]string{{"info", "validation.ok", "", "", "", "No issues found", ""}}
	}

	rows := make([][]string, 0, len(report.Checks))
	for _, check := range report.Checks {
		rows = append(rows, []string{
			string(check.Severity),
			check.ID,
			check.Locale,
			check.Field,
			formatResource(check.ResourceType, check.ResourceID),
			check.Message,
			check.Remediation,
		})
	}
	return headers, rows
}

func iapValidationSummaryRows(report *validation.IAPReport) ([]string, [][]string) {
	headers := []string{"App ID", "IAPs", "Errors", "Warnings", "Infos", "Blocking", "Strict"}
	rows := [][]string{{
		report.AppID,
		fmt.Sprintf("%d", report.IAPCount),
		fmt.Sprintf("%d", report.Summary.Errors),
		fmt.Sprintf("%d", report.Summary.Warnings),
		fmt.Sprintf("%d", report.Summary.Infos),
		fmt.Sprintf("%d", report.Summary.Blocking),
		formatBool(report.Strict),
	}}
	return headers, rows
}

func iapValidationCheckRows(report *validation.IAPReport) ([]string, [][]string) {
	headers := []string{"Severity", "Check ID", "Locale", "Field", "Resource", "Message", "Remediation"}
	if report == nil || len(report.Checks) == 0 {
		return headers, [][]string{{"info", "validation.ok", "", "", "", "No issues found", ""}}
	}

	rows := make([][]string, 0, len(report.Checks))
	for _, check := range report.Checks {
		rows = append(rows, []string{
			string(check.Severity),
			check.ID,
			check.Locale,
			check.Field,
			formatResource(check.ResourceType, check.ResourceID),
			check.Message,
			check.Remediation,
		})
	}
	return headers, rows
}

func subscriptionsValidationSummaryRows(report *validation.SubscriptionsReport) ([]string, [][]string) {
	headers := []string{"App ID", "Subscriptions", "Errors", "Warnings", "Infos", "Blocking", "Strict"}
	rows := [][]string{{
		report.AppID,
		fmt.Sprintf("%d", report.SubscriptionCount),
		fmt.Sprintf("%d", report.Summary.Errors),
		fmt.Sprintf("%d", report.Summary.Warnings),
		fmt.Sprintf("%d", report.Summary.Infos),
		fmt.Sprintf("%d", report.Summary.Blocking),
		formatBool(report.Strict),
	}}
	return headers, rows
}

func subscriptionsValidationCheckRows(report *validation.SubscriptionsReport) ([]string, [][]string) {
	headers := []string{"Severity", "Check ID", "Locale", "Field", "Resource", "Message", "Remediation"}
	if report == nil || len(report.Checks) == 0 {
		return headers, [][]string{{"info", "validation.ok", "", "", "", "No issues found", ""}}
	}

	rows := make([][]string, 0, len(report.Checks))
	for _, check := range report.Checks {
		rows = append(rows, []string{
			string(check.Severity),
			check.ID,
			check.Locale,
			check.Field,
			formatResource(check.ResourceType, check.ResourceID),
			check.Message,
			check.Remediation,
		})
	}
	return headers, rows
}

func subscriptionsValidationDiagnosticRows(report *validation.SubscriptionsReport) ([]string, [][]string) {
	headers := []string{"Subscription", "State", "Conclusion", "Check", "Status", "Source", "Blocking", "Evidence", "Remediation"}
	if report == nil || len(report.Diagnostics) == 0 {
		return headers, [][]string{{"", "", "", "diagnostics.none", "info", "", "", "No detailed subscription diagnostics collected", ""}}
	}

	rows := make([][]string, 0)
	for _, diagnostic := range report.Diagnostics {
		subscriptionLabel := diagnostic.SubscriptionID
		switch {
		case strings.TrimSpace(diagnostic.Name) != "" && strings.TrimSpace(diagnostic.ProductID) != "":
			subscriptionLabel = fmt.Sprintf("%s (%s)", strings.TrimSpace(diagnostic.Name), strings.TrimSpace(diagnostic.ProductID))
		case strings.TrimSpace(diagnostic.Name) != "":
			subscriptionLabel = strings.TrimSpace(diagnostic.Name)
		case strings.TrimSpace(diagnostic.ProductID) != "":
			subscriptionLabel = strings.TrimSpace(diagnostic.ProductID)
		}
		for _, row := range diagnostic.Rows {
			rows = append(rows, []string{
				subscriptionLabel,
				diagnostic.State,
				diagnostic.Conclusion,
				row.Key,
				string(row.Status),
				row.Source,
				formatBool(row.Blocking),
				row.Evidence,
				row.Remediation,
			})
		}
	}
	return headers, rows
}

func formatResource(resourceType, resourceID string) string {
	if resourceType == "" && resourceID == "" {
		return ""
	}
	if resourceID == "" {
		return resourceType
	}
	if resourceType == "" {
		return resourceID
	}
	return strings.TrimSpace(resourceType) + ":" + strings.TrimSpace(resourceID)
}

func formatBool(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
