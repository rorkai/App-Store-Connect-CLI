package asc

import (
	"fmt"
	"io"
	"strconv"
)

// PublishedApp describes an App Store Connect app with live published territory coverage.
type PublishedApp struct {
	ID                      string `json:"id"`
	Name                    string `json:"name"`
	BundleID                string `json:"bundleId"`
	SKU                     string `json:"sku"`
	PrimaryLocale           string `json:"primaryLocale,omitempty"`
	AvailabilityID          string `json:"availabilityId"`
	PublishedTerritoryCount int    `json:"publishedTerritoryCount"`
}

// PublishedAppFailure describes an app whose availability audit could not be completed.
type PublishedAppFailure struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Error string `json:"error"`
}

// AppsPublishedReport summarizes an account-wide published-app audit, retaining
// successful rows when individual app audits fail.
type AppsPublishedReport struct {
	AuditedAppCount   int                   `json:"auditedAppCount"`
	PublishedAppCount int                   `json:"publishedAppCount"`
	Apps              []PublishedApp        `json:"apps"`
	Failures          []PublishedAppFailure `json:"failures,omitempty"`
}

func appsPublishedReportTables(w io.Writer, report *AppsPublishedReport, render func([]string, [][]string)) error {
	headers := []string{"ID", "Name", "Bundle ID", "SKU", "Availability ID", "Published Territories"}
	rows := make([][]string, 0, len(report.Apps))
	for _, app := range report.Apps {
		rows = append(rows, []string{
			app.ID,
			app.Name,
			app.BundleID,
			app.SKU,
			app.AvailabilityID,
			strconv.Itoa(app.PublishedTerritoryCount),
		})
	}
	render(headers, rows)

	if len(report.Failures) > 0 {
		failureRows := make([][]string, 0, len(report.Failures))
		for _, failure := range report.Failures {
			failureRows = append(failureRows, []string{failure.ID, failure.Name, failure.Error})
		}
		fmt.Fprintln(w, "\nFailed app audits:")
		render([]string{"ID", "Name", "Error"}, failureRows)
	}
	fmt.Fprintf(w, "\n%s\n", PublishedAppsSummary(*report))
	return nil
}

// PublishedAppsSummary returns the human-readable audit totals.
func PublishedAppsSummary(report AppsPublishedReport) string {
	summary := fmt.Sprintf(
		"Audited %d app records; found %d published %s.",
		report.AuditedAppCount,
		report.PublishedAppCount,
		publishedAppLabel(report.PublishedAppCount),
	)
	if len(report.Failures) > 0 {
		summary += fmt.Sprintf(" %d app audit(s) failed.", len(report.Failures))
	}
	return summary
}

func publishedAppLabel(count int) string {
	if count == 1 {
		return "app"
	}
	return "apps"
}
