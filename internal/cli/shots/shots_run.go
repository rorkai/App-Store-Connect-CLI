package shots

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/screenshots"
)

// NewShotsRunCommand returns the screenshots run subcommand.
func NewShotsRunCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("run", flag.ExitOnError)
	planPath := fs.String("plan", ".asc/screenshots.json", "Path to screenshot run plan JSON")
	bundleID := fs.String("bundle-id", "", "Override app bundle ID from plan")
	udid := fs.String("udid", "", "Override simulator UDID from plan")
	outputDir := fs.String("output-dir", "", "Override output directory from plan")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "run",
		ShortUsage: "asc screenshots run [--plan .asc/screenshots.json] [flags]",
		ShortHelp:  "Run a deterministic screenshot sequence from JSON.",
		LongHelp: `Run a deterministic screenshot automation sequence.

By default it loads .asc/screenshots.json from the current project root.
Supported actions: launch, tap, type, key_sequence, wait, wait_for (polling), scroll, swipe, long_press, button, pinch, rotate, double_tap, screenshot.
scroll takes direction up, down, left, or right, the way the finger moves: up drags the content up to show what is below it.
long_press takes label, id, or x and y, and duration_ms from 1 to 10000 (default 1000).
button takes button apple-pay, home, lock, side-button, or siri, and optional duration_ms from 1 to 10000.
pinch takes scale above 0 and not 1 (above 1 zooms in), and optional x and y (default: the screen centre) and radius.
rotate takes x, y, radius, and degrees from -360 to 360, not 0 (clockwise).
double_tap takes x and y; it does not accept label or id.
scroll and swipe need AXe 2.0 or later.
Set defaults.status_bar to screenshot, low-battery, no-service, or charging to
override the simulator status bar during the run (needs AXe 2.0+ and Xcode 27+).
When the run ends, every status bar override on that simulator is cleared,
including any that were set before the run.
Set defaults.location to {"latitude": 37.7749, "longitude": -122.4194} to apply
a simulated location before the steps. The location receipt reports command
acknowledgement; use app readback to verify delivery. This setting persists
after the run; clear it with xcrun simctl location <UDID> clear when wanted.`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			planPathVal := *planPath
			if strings.TrimSpace(planPathVal) == "" {
				return d.RequiredFlag("--plan")
			}

			absPlanPath, err := d.Abs(planPathVal)
			if err != nil {
				return fmt.Errorf("screenshots run: resolve plan path: %w", err)
			}

			plan, err := screenshots.LoadPlanUnvalidated(absPlanPath)
			if err != nil {
				return fmt.Errorf("screenshots run: %w", err)
			}

			if override := strings.TrimSpace(*bundleID); override != "" {
				plan.App.BundleID = override
			}
			if override := strings.TrimSpace(*udid); override != "" {
				plan.App.UDID = override
			}
			if override := *outputDir; strings.TrimSpace(override) != "" {
				plan.App.OutputDir = override
			}
			if strings.TrimSpace(plan.App.OutputDir) == "" {
				plan.App.OutputDir = screenshots.DefaultPlanOutputDir
			}
			plan.App.OutputDir = d.OSPath(plan.App.OutputDir)

			result, err := screenshots.RunPlan(ctx, plan)
			if err != nil {
				return fmt.Errorf("screenshots run: %w", err)
			}

			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
