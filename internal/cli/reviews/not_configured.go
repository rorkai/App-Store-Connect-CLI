package reviews

import (
	"fmt"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type reviewDetailNotConfiguredResult struct {
	VersionID  string `json:"versionId"`
	Configured bool   `json:"configured"`
	Message    string `json:"message"`
}

type reviewResponseNotConfiguredResult struct {
	ReviewID   string `json:"reviewId"`
	Configured bool   `json:"configured"`
	Message    string `json:"message"`
}

func reviewDetailNotConfiguredMessage(versionID string) string {
	return fmt.Sprintf("App Store review detail is not configured for version %q.", versionID)
}

func reviewResponseNotConfiguredMessage(reviewID string) string {
	return fmt.Sprintf("Customer review response is not configured for review %q.", reviewID)
}

func warnNotConfigured(d *shared.Deps, message string) {
	fmt.Fprintf(d.IO.ErrOut, "Warning: %s\n", message)
}

func renderNotConfiguredState(d *shared.Deps, title, referenceLabel, referenceValue, message string, markdown bool) {
	rows := [][]string{
		{referenceLabel, referenceValue},
		{"configured", "false"},
		{"message", message},
	}
	d.RenderSection(title, []string{"field", "value"}, rows, markdown)
}

func reviewConfiguredLabel(configured bool) string {
	if configured {
		return "configured"
	}
	return "not configured"
}
