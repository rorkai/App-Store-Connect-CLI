package shots

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/screenshots"
)

const defaultShotsFrameOutputDir = "./screenshots/framed"

// ShotsFrameCommand returns the screenshots frame subcommand.
func ShotsFrameCommand() *ffcli.Command {
	fs := flag.NewFlagSet("frame", flag.ExitOnError)
	inputPath := fs.String("input", "", "Path to raw screenshot PNG (required)")
	configPath := fs.String("config", "", "Path to Koubou YAML config (optional)")
	outputPath := fs.String("output-path", "", "Exact output file path for framed PNG (optional)")
	outputDir := fs.String("output-dir", defaultShotsFrameOutputDir, "Output directory when --output-path is not set")
	name := fs.String("name", "", "Output file name without extension (defaults to input base name)")
	device := fs.String(
		"device",
		string(screenshots.DefaultFrameDevice()),
		fmt.Sprintf("Frame device: %s", strings.Join(screenshots.FrameDeviceValues(), ", ")),
	)
	title := fs.String("title", "", "Title text overlay (canvas mode only, e.g. --device mac)")
	subtitle := fs.String("subtitle", "", "Subtitle text overlay (canvas mode only, e.g. --device mac)")
	bgColor := fs.String("bg-color", "", "Solid background color in canvas mode (e.g. #1a1a2e); defaults to dark gradient")
	titleColor := fs.String("title-color", "", "Title text color in canvas mode (e.g. #000000); defaults to #ffffff")
	subtitleColor := fs.String("subtitle-color", "", "Subtitle text color in canvas mode (e.g. #333333); defaults to #aaaaaa")
	output := shared.BindOutputFlags(fs)
	watch := fs.Bool("watch", false, "Watch config and asset files for changes, auto-regenerate (requires --config)")
	watchDebounce := fs.Duration("watch-debounce", 500*time.Millisecond, "Debounce delay between change detection and regeneration")
	watchReviewDir := fs.String("watch-review-dir", "", "Auto-regenerate review HTML in this directory on each watch cycle")
	watchRawDir := fs.String("watch-raw-dir", "", "Raw screenshots directory for review generation (defaults to config asset dir)")

	return &ffcli.Command{
		Name:       "frame",
		ShortUsage: "asc screenshots frame (--input ./screenshots/raw/home.png | --config ./koubou.yaml) [flags]",
		ShortHelp:  "Compose a screenshot into an Apple device frame (experimental).",
		LongHelp: `Compose screenshots using Koubou's YAML-based rendering flow (experimental).

Requires Koubou v0.13.0 (pip install koubou==0.13.0).

Use either --input (auto-generated Koubou config) or --config (explicit Koubou YAML).

Use --watch with --config to start a live watcher that auto-regenerates
framed screenshots whenever the YAML config or referenced raw assets change.`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			configVal := strings.TrimSpace(*configPath)
			inputVal := strings.TrimSpace(*inputPath)
			if configVal == "" && inputVal == "" {
				fmt.Fprintln(os.Stderr, "Error: --input is required when --config is not set")
				return flag.ErrHelp
			}
			if configVal != "" && inputVal != "" {
				fmt.Fprintln(os.Stderr, "Error: use either --input or --config, not both")
				return flag.ErrHelp
			}
			if *watch && configVal == "" {
				fmt.Fprintln(os.Stderr, "Error: --watch requires --config")
				return flag.ErrHelp
			}
			if configVal != "" {
				absConfig, err := filepath.Abs(configVal)
				if err != nil {
					return fmt.Errorf("screenshots frame: resolve config path: %w", err)
				}
				configVal = absConfig
			}

			// Watch mode: start a long-running watcher that re-generates on
			// every config/asset change, then blocks until Ctrl-C.
			if *watch {
				watchCtx, stop := signal.NotifyContext(ctx, os.Interrupt)
				defer stop()
				var opts *screenshots.WatchOptions
				if reviewDir := strings.TrimSpace(*watchReviewDir); reviewDir != "" {
					opts = &screenshots.WatchOptions{
						ReviewOutputDir: reviewDir,
						ReviewRawDir:    strings.TrimSpace(*watchRawDir),
					}
				}
				return screenshots.WatchAndRegenerate(watchCtx, configVal, *watchDebounce, nil, opts)
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

			hasCanvasFlags := strings.TrimSpace(*title) != "" ||
				strings.TrimSpace(*subtitle) != "" ||
				strings.TrimSpace(*bgColor) != "" ||
				strings.TrimSpace(*titleColor) != "" ||
				strings.TrimSpace(*subtitleColor) != ""
			if hasCanvasFlags && configVal != "" {
				fmt.Fprintf(os.Stderr, "Error: --title, --subtitle, --bg-color, --title-color, --subtitle-color cannot be used with --config; set these in the YAML config instead\n")
				return flag.ErrHelp
			}
			if hasCanvasFlags && !screenshots.IsCanvasDevice(deviceVal) {
				fmt.Fprintf(os.Stderr, "Error: --title, --subtitle, --bg-color, --title-color, --subtitle-color only apply to canvas devices (e.g. --device mac)\n")
				return flag.ErrHelp
			}

			absInput := ""
			if inputVal != "" {
				var err error
				absInput, err = filepath.Abs(inputVal)
				if err != nil {
					return fmt.Errorf("screenshots frame: resolve input path: %w", err)
				}
			}

			outputDevice := string(deviceVal)
			if configVal != "" && strings.TrimSpace(*outputPath) == "" {
				outputDevice = screenshots.ResolveFrameDeviceFromConfig(configVal, outputDevice)
			}

			outPath, err := resolveOutputPath(*outputPath, *outputDir, *name, absInput, outputDevice)
			if err != nil {
				return fmt.Errorf("screenshots frame: %w", err)
			}

			timeoutCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			var canvasOpts *screenshots.CanvasOptions
			if hasCanvasFlags && screenshots.IsCanvasDevice(deviceVal) {
				canvasOpts = &screenshots.CanvasOptions{
					Title:         strings.TrimSpace(*title),
					Subtitle:      strings.TrimSpace(*subtitle),
					BGColor:       strings.TrimSpace(*bgColor),
					TitleColor:    strings.TrimSpace(*titleColor),
					SubtitleColor: strings.TrimSpace(*subtitleColor),
				}
			}

			result, err := screenshots.Frame(timeoutCtx, screenshots.FrameRequest{
				InputPath:  absInput,
				OutputPath: outPath,
				Device:     string(deviceVal),
				ConfigPath: configVal,
				Canvas:     canvasOpts,
			})
			if err != nil {
				return fmt.Errorf("screenshots frame: %w", err)
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
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
	if baseName != "" && (baseName == "." || baseName == ".." || strings.ContainsAny(baseName, `/\`)) {
		return "", fmt.Errorf("--name must be a file name without path separators")
	}
	if baseName == "" {
		trimmedInputPath := strings.TrimSpace(inputPath)
		if trimmedInputPath != "" {
			baseName = strings.TrimSuffix(filepath.Base(trimmedInputPath), filepath.Ext(trimmedInputPath))
		}
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
