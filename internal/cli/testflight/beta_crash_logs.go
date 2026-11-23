package testflight

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const crashLogNotFoundHint = "Crash logs are fetched by crash submission ID: list them with `asc testflight crashes list --app APP_ID` and pass one to --submission-id."

func crashLogFetchError(err error) error {
	err = fmt.Errorf("testflight crashes log: failed to fetch: %w", err)
	if asc.IsNotFound(err) {
		return &shared.HintedError{Err: err, Hint: crashLogNotFoundHint}
	}
	return err
}

// crashLogBySubmissionFetchError tells a submission without a crash log from
// an unknown submission. Apple answers GET
// /v1/betaFeedbackCrashSubmissions/{id}/crashLog for a submission that exists
// but has no crash log with a 404 naming the betaCrashLogs type, and for an
// unknown submission with one naming betaFeedbackCrashSubmissions.
func crashLogBySubmissionFetchError(err error, submissionID string) error {
	var apiErr *asc.APIError
	if !asc.IsNotFound(err) || !errors.As(err, &apiErr) || !strings.Contains(apiErr.Detail, "'betaCrashLogs'") {
		return crashLogFetchError(err)
	}
	quoted, ok := shared.ShellQuote(submissionID)
	if !ok {
		quoted = "SUBMISSION_ID"
	}
	return &shared.HintedError{
		Err:  fmt.Errorf("testflight crashes log: failed to fetch: %w", err),
		Hint: fmt.Sprintf("This crash submission has no crash log; App Store Connect stores none for it, so retrying will not help. `asc testflight crashes view --submission-id %s` shows the rest of the report.", quoted),
	}
}

func runCrashLogByCrashLogID(ctx context.Context, d *shared.Deps, crashLogID string, output shared.OutputFlags) error {
	client, err := d.Client()
	if err != nil {
		return fmt.Errorf("testflight crashes log: %w", err)
	}

	requestCtx, cancel := d.Timeout(ctx)
	defer cancel()

	resp, err := client.GetBetaCrashLog(requestCtx, crashLogID)
	if err != nil {
		return crashLogFetchError(err)
	}

	return d.PrintOutput(resp, *output.Output, *output.Pretty)
}
