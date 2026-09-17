package shots

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/screenshots"
)

const defaultShotsFrameOutputDir = "./screenshots/framed"

// watchUnsupportedFrameFlags lists the single-shot flags that --watch cannot
// honor because watch mode renders from the Koubou YAML config on every cycle.
var watchUnsupportedFrameFlags = []string{
	"bg-color",
	"device",
	"name",
	"output-dir",
	"output-path",
	"overlay-config",
	"resume",
	"subtitle",
	"subtitle-color",
	"title",
	"title-color",
}

var shotsFrameFn = screenshots.Frame

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
	title := fs.String("title", "", "Title text overlay")
	subtitle := fs.String("subtitle", "", "Subtitle or keyword text overlay")
	bgColor := fs.String("bg-color", "", "Solid background color in canvas mode (e.g. #1a1a2e); defaults to dark gradient")
	titleColor := fs.String("title-color", "", "Title text color (e.g. #000000); defaults to #ffffff")
	subtitleColor := fs.String("subtitle-color", "", "Subtitle text color (e.g. #333333); defaults to #aaaaaa")
	overlayConfig := fs.String("overlay-config", "", "JSON overlay config with default and data[] title, keyword, and background entries")
	resume := fs.Bool("resume", false, "Skip framing when the source hash matches .asc/reports/screenshots-frame/state.json")
	output := shared.BindOutputFlags(fs)
	watch := fs.Bool("watch", false, "Watch config and asset files for changes, auto-regenerate (requires --config)")
	watchDebounce := fs.Duration("watch-debounce", 500*time.Millisecond, "Debounce delay between change detection and regeneration")
	watchReviewDir := fs.String("watch-review-dir", "", "Auto-regenerate review HTML in this directory on each watch cycle")
	watchRawDir := fs.String("watch-raw-dir", "", "Raw screenshots directory for review generation (defaults to config asset dir)")

	return &ffcli.Command{
		Name:       "frame",
		ShortUsage: "asc screenshots frame (--input ./screenshots/raw/home.png | --config ./koubou.yaml) [flags]",
		ShortHelp:  "Compose a screenshot into an Apple device frame.",
		LongHelp: `Compose screenshots using Koubou's YAML-based rendering flow.

Requires Koubou v0.18.1 (pip install koubou==0.18.1).

Use either --input (auto-generated Koubou config) or --config (explicit Koubou YAML).

Use --watch with --config to start a live watcher that auto-regenerates
framed screenshots whenever the YAML config or referenced raw assets change.`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			// Keep path values literal after validating emptiness. Trimming a
			// valid path can select a different filesystem entry.
			configVal := *configPath
			inputVal := *inputPath
			configSet := strings.TrimSpace(configVal) != ""
			inputSet := strings.TrimSpace(inputVal) != ""
			watchDebounceSet := false
			watchReviewDirSet := false
			watchRawDirSet := false
			// Watch mode regenerates straight from the Koubou YAML config, so
			// the single-shot device, canvas and output flags have nowhere to
			// apply. Collect the ones the caller set so they are rejected
			// instead of silently dropped. fs.Visit reports flags in name
			// order, so the message is stable.
			watchUnsupportedFlags := make([]string, 0, len(watchUnsupportedFrameFlags))
			fs.Visit(func(flagValue *flag.Flag) {
				switch flagValue.Name {
				case "watch-debounce":
					watchDebounceSet = true
				case "watch-review-dir":
					watchReviewDirSet = true
				case "watch-raw-dir":
					watchRawDirSet = true
				default:
					if slices.Contains(watchUnsupportedFrameFlags, flagValue.Name) {
						watchUnsupportedFlags = append(watchUnsupportedFlags, "--"+flagValue.Name)
					}
				}
			})
			if !configSet && !inputSet {
				fmt.Fprintln(os.Stderr, "Error: --input is required when --config is not set")
				return shared.MissingRequiredUsageError("--input")
			}
			if configSet && inputSet {
				fmt.Fprintln(os.Stderr, "Error: use either --input or --config, not both")
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticConflictingInput, "--config")
			}
			if *watch && !configSet {
				fmt.Fprintln(os.Stderr, "Error: --watch requires --config")
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticConflictingInput, "--watch")
			}
			if *watch && len(watchUnsupportedFlags) > 0 {
				parameter := ""
				if len(watchUnsupportedFlags) == 1 {
					parameter = watchUnsupportedFlags[0]
				}
				return shared.WithDiagnostic(shared.UsageError(fmt.Sprintf(
					"%s cannot be used with --watch; watch mode regenerates from the Koubou YAML config",
					strings.Join(watchUnsupportedFlags, ", "),
				)), shared.DiagnosticConflictingInput, parameter)
			}
			if !*watch {
				switch {
				case watchDebounceSet:
					return shared.WithDiagnostic(shared.UsageError("--watch-debounce requires --watch"), shared.DiagnosticConflictingInput, "--watch-debounce")
				case watchReviewDirSet:
					return shared.WithDiagnostic(shared.UsageError("--watch-review-dir requires --watch"), shared.DiagnosticConflictingInput, "--watch-review-dir")
				case watchRawDirSet:
					return shared.WithDiagnostic(shared.UsageError("--watch-raw-dir requires --watch"), shared.DiagnosticConflictingInput, "--watch-raw-dir")
				}
			}
			if watchRawDirSet && !watchReviewDirSet {
				return shared.WithDiagnostic(shared.UsageError("--watch-raw-dir requires --watch-review-dir"), shared.DiagnosticConflictingInput, "--watch-raw-dir")
			}
			if watchReviewDirSet && strings.TrimSpace(*watchReviewDir) == "" {
				return shared.WithDiagnostic(shared.UsageError("--watch-review-dir must not be empty"), shared.DiagnosticInvalidInput, "--watch-review-dir")
			}
			if watchDebounceSet && *watchDebounce <= 0 {
				return shared.WithDiagnostic(shared.UsageError("--watch-debounce must be greater than 0"), shared.DiagnosticInvalidInput, "--watch-debounce")
			}
			if configSet {
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
				if reviewDir := *watchReviewDir; strings.TrimSpace(reviewDir) != "" {
					opts = &screenshots.WatchOptions{
						ReviewOutputDir: reviewDir,
						ReviewRawDir:    *watchRawDir,
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
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticInvalidInput, "--device")
			}

			canvasParameters := make([]string, 0, 5)
			if strings.TrimSpace(*title) != "" {
				canvasParameters = append(canvasParameters, "--title")
			}
			if strings.TrimSpace(*subtitle) != "" {
				canvasParameters = append(canvasParameters, "--subtitle")
			}
			if strings.TrimSpace(*bgColor) != "" {
				canvasParameters = append(canvasParameters, "--bg-color")
			}
			if strings.TrimSpace(*titleColor) != "" {
				canvasParameters = append(canvasParameters, "--title-color")
			}
			if strings.TrimSpace(*subtitleColor) != "" {
				canvasParameters = append(canvasParameters, "--subtitle-color")
			}
			hasCanvasFlags := len(canvasParameters) > 0
			hasTextFlags := strings.TrimSpace(*title) != "" || strings.TrimSpace(*subtitle) != "" || strings.TrimSpace(*titleColor) != "" || strings.TrimSpace(*subtitleColor) != ""
			if hasCanvasFlags && configSet {
				fmt.Fprintf(os.Stderr, "Error: --title, --subtitle, --bg-color, --title-color, --subtitle-color cannot be used with --config; set these in the YAML config instead\n")
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticConflictingInput, "--config")
			}
			if strings.TrimSpace(*bgColor) != "" && !screenshots.IsCanvasDevice(deviceVal) {
				fmt.Fprintln(os.Stderr, "Error: --bg-color only applies to canvas devices (e.g. --device mac)")
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticConflictingInput, "--bg-color")
			}

			absInput := ""
			if inputSet {
				var err error
				absInput, err = filepath.Abs(inputVal)
				if err != nil {
					return fmt.Errorf("screenshots frame: resolve input path: %w", err)
				}
			}

			outputDevice := string(deviceVal)
			if configSet && strings.TrimSpace(*outputPath) == "" {
				outputDevice = screenshots.ResolveFrameDeviceFromConfig(configVal, outputDevice)
			}

			outPath, err := resolveOutputPath(*outputPath, *outputDir, *name, absInput, outputDevice)
			if err != nil {
				return fmt.Errorf("screenshots frame: %w", err)
			}

			timeoutCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			var canvasOpts *screenshots.CanvasOptions
			if hasTextFlags || (strings.TrimSpace(*bgColor) != "" && screenshots.IsCanvasDevice(deviceVal)) {
				canvasOpts = &screenshots.CanvasOptions{
					Title:         strings.TrimSpace(*title),
					Subtitle:      strings.TrimSpace(*subtitle),
					BGColor:       strings.TrimSpace(*bgColor),
					TitleColor:    strings.TrimSpace(*titleColor),
					SubtitleColor: strings.TrimSpace(*subtitleColor),
				}
			}

			overlayHash := ""
			if strings.TrimSpace(*overlayConfig) != "" {
				if configSet {
					fmt.Fprintln(os.Stderr, "Error: --overlay-config cannot be used with --config")
					return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticConflictingInput, "--overlay-config")
				}
				loaded, hash, err := screenshots.LoadOverlayConfig(*overlayConfig)
				if err != nil {
					return fmt.Errorf("screenshots frame: %w", err)
				}
				overlayHash = hash
				matched := screenshots.OverlayToCanvas(screenshots.MatchOverlay(loaded, absInput))
				if canvasOpts == nil {
					canvasOpts = &screenshots.CanvasOptions{}
				}
				if canvasOpts.Title == "" {
					canvasOpts.Title = matched.Title
				}
				if canvasOpts.Subtitle == "" {
					canvasOpts.Subtitle = matched.Subtitle
				}
				if canvasOpts.BGColor == "" {
					canvasOpts.BGColor = matched.BGColor
				}
			}

			if canvasOpts != nil && strings.TrimSpace(canvasOpts.BGColor) != "" && !screenshots.IsCanvasDevice(deviceVal) {
				fmt.Fprintln(os.Stderr, "Error: background overlays only apply to canvas devices (e.g. --device mac)")
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticConflictingInput, "--overlay-config")
			}

			if *resume && configSet {
				fmt.Fprintln(os.Stderr, "Error: --resume cannot be used with --config")
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticConflictingInput, "--resume")
			}
			if *resume && inputSet {
				hash, err := screenshots.HashFile(absInput)
				if err != nil {
					return fmt.Errorf("screenshots frame: hash input: %w", err)
				}
				fingerprint := frameResumeFingerprint(hash, string(deviceVal), overlayHash, canvasOpts)
				root, err := frameResumeRoot()
				if err != nil {
					return err
				}
				defer root.Close()
				var framed *screenshots.FrameResult
				err = screenshots.WithFrameResumeLock(timeoutCtx, root, func() error {
					state, loadErr := screenshots.LoadFrameResumeState(root, screenshots.FrameResumeStateRel)
					if loadErr != nil {
						return fmt.Errorf("screenshots frame: read resume state: %w", loadErr)
					}
					if result, ok := screenshots.ResumeEntry(state, outPath, fingerprint); ok {
						framed = &result
						return nil
					}
					result, frameErr := shotsFrameFn(timeoutCtx, screenshots.FrameRequest{
						InputPath:  absInput,
						OutputPath: outPath,
						Device:     string(deviceVal),
						ConfigPath: configVal,
						Canvas:     canvasOpts,
					})
					if frameErr != nil {
						return fmt.Errorf("screenshots frame: %w", frameErr)
					}
					stored := *result
					stored.Skipped = false
					state.Files[outPath] = screenshots.FrameResumeEntry{
						Fingerprint: fingerprint,
						Result:      stored,
					}
					if saveErr := screenshots.SaveFrameResumeState(root, screenshots.FrameResumeStateRel, state); saveErr != nil {
						return fmt.Errorf("screenshots frame: write resume state: %w", saveErr)
					}
					framed = result
					return nil
				})
				if err != nil {
					return err
				}
				return shared.PrintOutput(framed, *output.Output, *output.Pretty)
			}

			result, err := shotsFrameFn(timeoutCtx, screenshots.FrameRequest{
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

func frameResumeFingerprint(sourceHash, device, overlayHash string, canvas *screenshots.CanvasOptions) string {
	fp := screenshots.FrameResumeFingerprint{
		SourceHash:  sourceHash,
		Device:      device,
		OverlayHash: overlayHash,
	}
	if canvas != nil {
		fp.Title = canvas.Title
		fp.Subtitle = canvas.Subtitle
		fp.TitleColor = canvas.TitleColor
		fp.SubtitleColor = canvas.SubtitleColor
		fp.Background = canvas.BGColor
	}
	return screenshots.FingerprintFrameResume(fp)
}

func frameResumeRoot() (rootfs.Root, error) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return rootfs.Root{}, fmt.Errorf("screenshots frame: resolve resume root: %w", err)
	}
	root, err := rootfs.New(workingDirectory)
	if err != nil {
		return rootfs.Root{}, fmt.Errorf("screenshots frame: resolve resume root: %w", err)
	}
	return root, nil
}

func resolveOutputPath(explicitPath, outputDir, name, inputPath, device string) (string, error) {
	explicit := explicitPath
	if strings.TrimSpace(explicit) != "" {
		absPath, err := filepath.Abs(explicit)
		if err != nil {
			return "", fmt.Errorf("resolve output path: %w", err)
		}
		return absPath, nil
	}

	dir := outputDir
	if strings.TrimSpace(dir) == "" {
		dir = defaultShotsFrameOutputDir
	}
	baseName := strings.TrimSpace(name)
	if baseName != "" && (baseName == "." || baseName == ".." || strings.ContainsAny(baseName, `/\`)) {
		return "", shared.WithDiagnostic(
			shared.NewValidationError(fmt.Errorf("--name must be a file name without path separators")),
			shared.DiagnosticInvalidInput,
			"--name",
		)
	}
	if baseName == "" {
		if strings.TrimSpace(inputPath) != "" {
			baseName = strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
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
