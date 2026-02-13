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

const defaultShotsFrameOutputDir = "./screenshots/framed"

// ShotsFrameCommand returns the shots frame subcommand.
func ShotsFrameCommand() *ffcli.Command {
	fs := flag.NewFlagSet("frame", flag.ExitOnError)
	inputPath := fs.String("input", "", "Path to raw screenshot PNG (required)")
	outputPath := fs.String("output-path", "", "Exact output file path for framed PNG (optional)")
	outputDir := fs.String("output-dir", defaultShotsFrameOutputDir, "Output directory when --output-path is not set")
	name := fs.String("name", "", "Output file name without extension (defaults to input base name)")
	device := fs.String(
		"device",
		string(screenshots.DefaultFrameDevice()),
		fmt.Sprintf("Frame device: %s", strings.Join(screenshots.FrameDeviceValues(), ", ")),
	)
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "frame",
		ShortUsage: "asc shots frame --input ./screenshots/raw/home.png [flags]",
		ShortHelp:  "Compose a screenshot into an Apple device frame.",
		LongHelp: `Compose one raw screenshot into a cached Apple device frame.

By default this uses --device iphone-air and writes to ./screenshots/framed.`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			inputVal := strings.TrimSpace(*inputPath)
			if inputVal == "" {
				fmt.Fprintln(os.Stderr, "Error: --input is required")
				return flag.ErrHelp
			}

			deviceVal, err := screenshots.ParseFrameDevice(*device)
			if err != nil {
				fmt.Fprintf(
					os.Stderr,
					"Error: --device must be one of: %s\n",
					strings.Join(screenshots.FrameDeviceValues(), ", "),
				)
				return flag.ErrHelp
			}

			absInput, err := filepath.Abs(inputVal)
			if err != nil {
				return fmt.Errorf("shots frame: resolve input path: %w", err)
			}

			outPath, err := resolveOutputPath(*outputPath, *outputDir, *name, absInput, string(deviceVal))
			if err != nil {
				return fmt.Errorf("shots frame: %w", err)
			}

			result, err := screenshots.Frame(ctx, screenshots.FrameRequest{
				InputPath:  absInput,
				OutputPath: outPath,
				Device:     string(deviceVal),
			})
			if err != nil {
				return fmt.Errorf("shots frame: %w", err)
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}

func resolveOutputPath(explicitPath, outputDir, name, inputPath, device string) (string, error) {
	explicit := strings.TrimSpace(explicitPath)
	if explicit != "" {
		absPath, err := filepath.Abs(explicit)
		if err != nil {
			return "", fmt.Errorf("resolve output path: %w", err)
		}
		return absPath, nil
	}

	dir := strings.TrimSpace(outputDir)
	if dir == "" {
		dir = defaultShotsFrameOutputDir
	}
	baseName := strings.TrimSpace(name)
	if baseName == "" {
		baseName = strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	}
	if baseName == "" {
		baseName = "screenshot"
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve output directory: %w", err)
	}
	return filepath.Join(absDir, fmt.Sprintf("%s-%s.png", baseName, device)), nil
}
