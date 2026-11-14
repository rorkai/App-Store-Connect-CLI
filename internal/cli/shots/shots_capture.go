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
	provider := fs.String("provider", screenshots.ProviderAXe, fmt.Sprintf("Capture provider: %s (iOS simulator), %s (macOS, app must be running)", screenshots.ProviderAXe, screenshots.ProviderMacOS))
	bundleID := fs.String("bundle-id", "", "App bundle ID (required)")
	udid := fs.String("udid", "booted", "Simulator UDID (default: booted)")
	name := fs.String("name", "", "Screenshot name for output file (required)")
	outputDir := fs.String("output-dir", "./screenshots/raw", "Output directory for captured PNG")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "capture",
		ShortUsage: "asc screenshots capture --bundle-id BUNDLE_ID --name NAME [flags]",
		ShortHelp:  "Capture a single screenshot from a simulator or running macOS app.",
		LongHelp: `Capture one screenshot from a running app.

iOS/simulator (default): app must be installed; simulator must be booted or --udid set.

macOS: app must be running. Captures the frontmost visible window by bundle ID.
  Requires: Screen Recording permission for your terminal app, and Xcode Command Line Tools (swift).
  asc screenshots capture --provider macos --bundle-id com.example.MyApp --name home`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			bundleIDVal := strings.TrimSpace(*bundleID)
			if bundleIDVal == "" {
				return shared.RequiredFlag("--bundle-id")
			}
			nameVal := strings.TrimSpace(*name)
			if nameVal == "" {
				return shared.RequiredFlag("--name")
			}
			if nameVal == "." || nameVal == ".." || strings.ContainsAny(nameVal, `/\`) {
				fmt.Fprintln(os.Stderr, "Error: --name must be a file name without path separators")
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticInvalidInput, "--name")
			}
			providerVal := strings.TrimSpace(strings.ToLower(*provider))
			if providerVal != screenshots.ProviderAXe && providerVal != screenshots.ProviderMacOS {
				fmt.Fprintf(os.Stderr, "Error: --provider must be %q or %q\n", screenshots.ProviderAXe, screenshots.ProviderMacOS)
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticInvalidInput, "--provider")
			}

			outputDirVal := *outputDir
			if strings.TrimSpace(outputDirVal) == "" {
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

			timeoutCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			result, err := screenshots.Capture(timeoutCtx, req)
			if err != nil {
				return fmt.Errorf("screenshots capture: %w", err)
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
