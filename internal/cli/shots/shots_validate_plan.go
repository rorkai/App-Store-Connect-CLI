package shots

import (
	"context"
	"flag"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/screenshots"
)

// NewShotsValidatePlanCommand returns the offline capture-plan validator.
func NewShotsValidatePlanCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("validate-plan", flag.ExitOnError)
	planPath := fs.String("plan", ".asc/screenshots.json", "Path to screenshot run plan JSON")
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "validate-plan",
		ShortUsage: "asc screenshots validate-plan [--plan .asc/screenshots.json] [flags]",
		ShortHelp:  "Validate screenshot capture plan fields and steps offline.",
		LongHelp: `Validate a screenshot capture plan offline before running it.

Read JSON or JSONC, reject unknown fields, and check step values.
Field paths use zero-based array indexes; step numbers are one-based.
Independent unknown fields are reported together.

Examples:
  asc screenshots validate-plan --plan .asc/screenshots.json
  asc screenshots validate-plan --plan .asc/screenshots.json --output table`,
		FlagSet: fs, UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return d.UsageError(err.Error())
			}
			if strings.TrimSpace(*planPath) == "" {
				return d.RequiredFlag("--plan")
			}
			path, err := d.Abs(*planPath)
			if err != nil {
				return d.UsageErrorf("screenshots validate-plan: %v", err)
			}
			plan, err := screenshots.LoadPlanStrict(path)
			if err != nil {
				return d.UsageErrorf("screenshots validate-plan: %v", err)
			}
			return d.PrintOutput(&asc.ScreenshotPlanValidationResult{PlanPath: path, Valid: true, TotalSteps: len(plan.Steps)}, *output.Output, *output.Pretty)
		},
	}
}
