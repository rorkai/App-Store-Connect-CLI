package merchantids

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

func splitCSV(value string) []string {
	return shared.SplitCSV(value)
}

func hasInclude(values []string, include string) bool {
	return shared.HasInclude(values, include)
}

func validateSort(value string, allowed ...string) error {
	return shared.ValidateSort(value, allowed...)
}

func certificateFieldsList() []string {
	return shared.CertificateFieldsList()
}

func certificateIncludeList() []string {
	return shared.CertificateIncludeList()
}

func certificateSortValues() []string {
	return shared.CertificateSortValues()
}

func normalizeSelection(value, flagName string, allowed []string) ([]string, error) {
	return shared.NormalizeSelection(value, flagName, allowed)
}

func normalizeCertificateFields(value, flagName string) ([]string, error) {
	return shared.NormalizeCertificateFields(value, flagName)
}

func normalizeCertificateInclude(value, flagName string) ([]string, error) {
	return shared.NormalizeCertificateInclude(value, flagName)
}
