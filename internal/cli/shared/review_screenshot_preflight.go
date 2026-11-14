package shared

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// PreflightReviewScreenshot checks the bytes of an in-app purchase or
// subscription App Review screenshot that are about to be uploaded. A format
// problem App Store Connect would reject at delivery is returned as an error
// before any request. Documented but unconfirmed constraints (size and alpha)
// and a payload that does not fully decode are printed to stderr as warnings.
func PreflightReviewScreenshot(path string, file io.ReaderAt, size int64) error {
	return preflightReviewScreenshotTo(os.Stderr, path, file, size)
}

// PreflightReviewScreenshot is PreflightReviewScreenshot with the warnings
// written to d's stderr.
func (d *Deps) PreflightReviewScreenshot(path string, file io.ReaderAt, size int64) error {
	return preflightReviewScreenshotTo(d.IO.ErrOut, path, file, size)
}

func preflightReviewScreenshotTo(stderr io.Writer, path string, file io.ReaderAt, size int64) error {
	warnings, err := asc.CheckReviewScreenshotImage(path, io.NewSectionReader(file, 0, size))
	if err != nil {
		return err
	}
	if warning := asc.ReviewScreenshotDecodeWarning(path, file, size); warning != "" {
		warnings = append(warnings, warning)
	}
	for _, warning := range warnings {
		fmt.Fprintf(stderr, "Warning: %s\n", warning)
	}
	return nil
}

// ReviewScreenshotUsageError reports a review screenshot rejected before any
// request. It prints message as one "Error:" line and returns a usage error
// (exit 2) that does not wrap flag.ErrHelp, so the command's full usage page
// does not follow and bury the one actionable line. parameter names the flag
// that supplied the screenshot for diagnostics. Commands under a re-parented
// tree pass message through RewriteUsageMessage first.
func ReviewScreenshotUsageError(parameter, message string) error {
	return reviewScreenshotUsageErrorTo(os.Stderr, parameter, message)
}

// ReviewScreenshotUsageError is ReviewScreenshotUsageError with the "Error:"
// line written to d's stderr.
func (d *Deps) ReviewScreenshotUsageError(parameter, message string) error {
	return reviewScreenshotUsageErrorTo(d.IO.ErrOut, parameter, message)
}

func reviewScreenshotUsageErrorTo(stderr io.Writer, parameter, message string) error {
	message = strings.TrimSpace(SanitizeTerminal(message))
	fmt.Fprintf(stderr, "Error: %s\n", message)
	return WithDiagnostic(NewReportedUsageError(UsageErrorInvalidValue, message), DiagnosticInvalidInput, parameter)
}

// ReviewScreenshotDeliveryError reports a review screenshot that App Store
// Connect rejected after upload as invalid --file input. The failed screenshot
// stays attached and blocks a new upload, so the message names the delete
// command of group.
func ReviewScreenshotDeliveryError(group, screenshotID string, details []asc.StateDetail) error {
	parts := make([]string, 0, len(details))
	for _, detail := range details {
		code := strings.TrimSpace(detail.Code)
		description := strings.TrimSpace(detail.Description)
		switch {
		case code != "" && description != "":
			parts = append(parts, fmt.Sprintf("%s (%s)", code, description))
		case code != "":
			parts = append(parts, code)
		case description != "":
			parts = append(parts, description)
		}
	}
	reason := strings.Join(parts, ", ")
	if reason == "" {
		reason = "unknown error"
	}
	quotedID, ok := ShellQuote(screenshotID)
	if !ok {
		quotedID = "SCREENSHOT_ID"
	}
	message := fmt.Sprintf("screenshot %s delivery failed: %s; run %s delete --screenshot-id %s --confirm, then upload a corrected file", screenshotID, reason, group, quotedID)
	return WithDiagnostic(NewValidationError(errors.New(SanitizeTerminal(message))), DiagnosticInvalidInput, "--file")
}
