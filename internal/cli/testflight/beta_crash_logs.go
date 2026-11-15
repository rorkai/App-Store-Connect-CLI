package testflight

import (
	"context"
	"fmt"

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
