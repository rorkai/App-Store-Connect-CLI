package offercodes

import (
	"context"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func DefaultUsageFunc(c *ffcli.Command) string {
	return shared.DefaultUsageFunc(c)
}

func getASCClient() (*asc.Client, error) {
	return shared.GetASCClient()
}

func contextWithTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return shared.ContextWithTimeout(ctx)
}

func printOutput(data interface{}, format string, pretty bool) error {
	return shared.PrintOutput(data, format, pretty)
}

func validateNextURL(next string) error {
	return shared.ValidateNextURL(next)
}

func normalizeDate(value, flagName string) (string, error) {
	return shared.NormalizeDate(value, flagName)
}

func splitCSV(value string) []string {
	return shared.SplitCSV(value)
}

func parseCommaSeparatedIDs(value string) []string {
	return shared.SplitCSV(value)
}

func parseOptionalBoolFlag(name, value string) (*bool, error) {
	return shared.ParseOptionalBoolFlag(name, value)
}
