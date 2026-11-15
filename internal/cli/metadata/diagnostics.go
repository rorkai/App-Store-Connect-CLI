package metadata

import (
	"fmt"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func metadataRequiredInputError(d *shared.Deps, parameter, message string) error {
	trimmed := strings.TrimSpace(message)
	fmt.Fprintf(d.IO.ErrOut, "Error: %s\n", trimmed)
	return shared.WithDiagnostic(
		shared.NewReportedUsageError(shared.UsageErrorMissingRequired, trimmed),
		shared.DiagnosticRequiredInputMissing,
		parameter,
	)
}
