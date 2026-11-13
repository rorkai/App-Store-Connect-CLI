package shots

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/screenshots"
)

type shotsMatrixCommandDependencies struct {
	loadPlan  func(string) (*screenshots.MatrixPlan, error)
	runMatrix func(context.Context, string, *screenshots.MatrixPlan, screenshots.MatrixOptions) (*screenshots.MatrixResult, error)
}

// ShotsMatrixCommand returns the local screenshot matrix subcommand.
func ShotsMatrixCommand() *ffcli.Command {
	return shotsMatrixCommandWithDependencies(shotsMatrixCommandDependencies{
		loadPlan:  screenshots.LoadMatrixPlan,
		runMatrix: screenshots.RunMatrix,
	})
}

func shotsMatrixCommandWithDependencies(dependencies shotsMatrixCommandDependencies) *ffcli.Command {
	fs := flag.NewFlagSet("matrix", flag.ExitOnError)
	planPath := fs.String("plan", ".asc/screenshots-matrix.json", "Path to screenshot matrix plan JSON")
	maxConcurrency := fs.Int("max-concurrency", 0, "Override matrix worker count (1-8; 0 uses plan value)")
	maxAttempts := fs.Int("max-attempts", 0, "Override total attempts per cell (1-3; 0 uses plan value)")
	retryBackoff := fs.Duration("retry-backoff", 0, "Override retry backoff duration (0 uses plan value unless explicitly set)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "matrix",
		ShortUsage: "asc screenshots matrix [--plan .asc/screenshots-matrix.json] [flags]",
		ShortHelp:  "Capture a bounded local screenshot matrix and write an offline review.",
		LongHelp: `Capture an existing screenshot plan across device, locale, appearance,
and content-variant axes.

Target simulators must already exist and be booted. This command is local-only:
it does not upload screenshots or change App Store Connect state. Every run
writes review/manifest.json and review/index.html, including failed cells.

Use --max-concurrency, --max-attempts, and --retry-backoff to override the
execution values in the matrix plan.`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}

			maxConcurrencySet := false
			maxAttemptsSet := false
			retryBackoffSet := false
			fs.Visit(func(value *flag.Flag) {
				switch value.Name {
				case "max-concurrency":
					maxConcurrencySet = true
				case "max-attempts":
					maxAttemptsSet = true
				case "retry-backoff":
					retryBackoffSet = true
				}
			})
			planValue := *planPath
			if strings.TrimSpace(planValue) == "" {
				fmt.Fprintln(os.Stderr, "Error: --plan is required")
				return shared.MissingRequiredUsageError("--plan")
			}
			// Both flags advertise "0 uses plan value". Honor that sentinel
			// instead of rejecting it, so a script computing an override can
			// pass 0 to defer to the plan. Only a stated non-zero value counts
			// as an override, and it must fall inside the documented range.
			if *maxConcurrency == 0 {
				maxConcurrencySet = false
			}
			if *maxAttempts == 0 {
				maxAttemptsSet = false
			}
			if maxConcurrencySet && (*maxConcurrency < 1 || *maxConcurrency > 8) {
				fmt.Fprintln(os.Stderr, "Error: --max-concurrency must be between 1 and 8 when set")
				return shared.InvalidValueUsageError("--max-concurrency")
			}
			if maxAttemptsSet && (*maxAttempts < 1 || *maxAttempts > 3) {
				fmt.Fprintln(os.Stderr, "Error: --max-attempts must be between 1 and 3 when set")
				return shared.InvalidValueUsageError("--max-attempts")
			}
			if *retryBackoff < 0 {
				fmt.Fprintln(os.Stderr, "Error: --retry-backoff must be >= 0")
				return shared.InvalidValueUsageError("--retry-backoff")
			}

			absPlanPath, err := filepath.Abs(planValue)
			if err != nil {
				return fmt.Errorf("screenshots matrix: resolve plan path: %w", err)
			}
			matrixPlan, err := dependencies.loadPlan(absPlanPath)
			if err != nil {
				if errors.Is(err, screenshots.ErrMatrixPlanParseJSON) || errors.Is(err, screenshots.ErrMatrixPlanRead) {
					return reportMatrixUsageError(err, "--plan")
				}
				return fmt.Errorf("screenshots matrix: %w", err)
			}
			result, runErr := dependencies.runMatrix(ctx, absPlanPath, matrixPlan, screenshots.MatrixOptions{
				MaxConcurrency:    *maxConcurrency,
				MaxConcurrencySet: maxConcurrencySet,
				MaxAttempts:       *maxAttempts,
				MaxAttemptsSet:    maxAttemptsSet,
				RetryBackoff:      *retryBackoff,
				RetryBackoffSet:   retryBackoffSet,
			})
			if result != nil {
				if printErr := shared.PrintOutput(matrixResultOutput(result), *output.Output, *output.Pretty); printErr != nil {
					return printErr
				}
			}
			if runErr != nil {
				var validationErr *screenshots.MatrixValidationError
				if errors.As(runErr, &validationErr) {
					return reportMatrixUsageError(validationErr, "--plan")
				}
				return fmt.Errorf("screenshots matrix: %w", runErr)
			}
			return nil
		},
	}
}

func reportMatrixUsageError(err error, parameter string) error {
	fmt.Fprintf(os.Stderr, "Error: %s\n", shared.SanitizeTerminal(err.Error()))
	return shared.NewErrorWithCause(shared.InvalidValueUsageError(parameter), err)
}
