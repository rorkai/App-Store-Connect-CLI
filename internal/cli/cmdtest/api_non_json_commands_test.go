package cmdtest

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var nonJSONAdviceCommand = regexp.MustCompile("use `(asc [^`]+)` instead")

// TestAPINonJSONRejectionsRecommendRealCommands walks every operation that
// `asc api` refuses for its response media type, extracts the command its
// diagnostic recommends, and resolves that command through the real root so
// the advice cannot rot into an unknown-command error.
func TestAPINonJSONRejectionsRecommendRealCommands(t *testing.T) {
	paths := []string{
		"/v1/salesReports",
		"/v1/financeReports",
		"/v1/subscriptionOfferCodeOneTimeUseCodes/CODE_ID/values",
		"/v1/inAppPurchaseOfferCodeOneTimeUseCodes/CODE_ID/values",
		"/v1/apps/APP_ID/perfPowerMetrics",
		"/v1/builds/BUILD_ID/perfPowerMetrics",
		"/v1/diagnosticSignatures/SIGNATURE_ID/logs",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
				return nil, errors.New("client factory must not run during validation")
			})
			defer restore()

			_, stderr := captureOutput(t, func() {
				if code := rootcmd.Run([]string{"api", "GET", path}, "1.2.3"); code != rootcmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
				}
			})
			restore()

			match := nonJSONAdviceCommand.FindStringSubmatch(stderr)
			if match == nil {
				t.Fatalf("stderr = %q, want a recommended `asc ...` command", stderr)
			}
			recommended := strings.Fields(match[1])[1:]
			if len(recommended) == 0 {
				t.Fatalf("recommended command %q has no subcommand", match[1])
			}

			helpArgs := append(append([]string{}, recommended...), "--help")
			var helpCode int
			helpOut, helpErr := captureOutput(t, func() {
				helpCode = rootcmd.Run(helpArgs, "1.2.3")
			})
			if helpCode != rootcmd.ExitSuccess {
				t.Fatalf("`asc %s --help` exit code = %d, want %d (stderr: %s)",
					strings.Join(recommended, " "), helpCode, rootcmd.ExitSuccess, helpErrHint(helpErr))
			}
			if !strings.Contains(helpOut+helpErr, recommended[len(recommended)-1]) {
				t.Fatalf("`asc %s --help` output does not mention the command:\n%s%s",
					strings.Join(recommended, " "), helpOut, helpErr)
			}
		})
	}
}

func helpErrHint(stderr string) string {
	if stderr == "" {
		return "<empty>"
	}
	return stderr
}
