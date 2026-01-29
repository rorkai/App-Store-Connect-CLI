package server

import (
	"context"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func DefaultUsageFunc(c *ffcli.Command) string {
	return shared.DefaultUsageFunc(c)
}

func getServerClient() (*asc.ServerAPIClient, error) {
	return shared.GetServerAPIClient()
}

func contextWithTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return shared.ContextWithTimeout(ctx)
}

func printOutput(data interface{}, format string, pretty bool) error {
	return shared.PrintOutput(data, format, pretty)
}

func splitCSV(value string) []string {
	return shared.SplitCSV(value)
}
