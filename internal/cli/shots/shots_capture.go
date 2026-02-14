package shots

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/screenshots"
)

// ShotsCaptureCommand returns the screenshots capture subcommand.
func ShotsCaptureCommand() *ffcli.Command {
	fs := flag.NewFlagSet("capture", flag.ExitOnError)
	provider := fs.String("provider", screenshots.ProviderAXe, "Capture provider: axe (default)")
	bundleID := fs.String("bundle-id", "", "App bundle ID (required)")
	udid := fs.String("udid", "booted", "Simulator UDID (default: booted)")
	name := fs.String("name", "", "Screenshot name for output file (required)")
	outputDir := fs.String("output-dir", "./screenshots/raw", "Output directory for captured PNG")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "capture",
		ShortUsage: "asc screenshots capture --bundle-id BUNDLE_ID --name NAME [flags]",
		ShortHelp:  "Capture a single screenshot from the simulator.",
		LongHelp: `Capture one screenshot from the running app on the simulator.
App must already be installed; simulator must be booted or --udid set.`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			bundleIDVal := strings.TrimSpace(*bundleID)
			if bundleIDVal == "" {
				fmt.Fprintln(os.Stderr, "Error: --bundle-id is required")
				return flag.ErrHelp
			}
			nameVal := strings.TrimSpace(*name)
			if nameVal == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return flag.ErrHelp
			}
			if nameVal == "." || nameVal == ".." || strings.ContainsAny(nameVal, `/\`) {
				fmt.Fprintln(os.Stderr, "Error: --name must be a file name without path separators")
				return flag.ErrHelp
			}
			providerVal := strings.TrimSpace(strings.ToLower(*provider))
			if providerVal != screenshots.ProviderAXe {
				fmt.Fprintf(os.Stderr, "Error: --provider must be %q\n", screenshots.ProviderAXe)
				return flag.ErrHelp
			}

			outputDirVal := strings.TrimSpace(*outputDir)
			if outputDirVal == "" {
				outputDirVal = "./screenshots/raw"
			}
			absOut, err := filepath.Abs(outputDirVal)
			if err != nil {
				return fmt.Errorf("screenshots capture: resolve output dir: %w", err)
			}
			if err := os.MkdirAll(absOut, 0o755); err != nil {
				return fmt.Errorf("screenshots capture: create output dir: %w", err)
			}

			req := screenshots.CaptureRequest{
				Provider:  providerVal,
				BundleID:  bundleIDVal,
				UDID:      strings.TrimSpace(*udid),
				Name:      nameVal,
				OutputDir: absOut,
			}

			result, err := screenshots.Capture(ctx, req)
			if err != nil {
				return fmt.Errorf("screenshots capture: %w", err)
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}
