package testflight

import (
	"context"
	"fmt"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func runCrashLogByCrashLogID(ctx context.Context, crashLogID string, output shared.OutputFlags) error {
	client, err := shared.GetASCClient()
	if err != nil {
		return fmt.Errorf("testflight crashes log: %w", err)
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	resp, err := client.GetBetaCrashLog(requestCtx, crashLogID)
	if err != nil {
		return fmt.Errorf("testflight crashes log: failed to fetch: %w", err)
	}

	return shared.PrintOutput(resp, *output.Output, *output.Pretty)
}
