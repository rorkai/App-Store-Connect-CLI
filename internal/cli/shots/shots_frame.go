package shots

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
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
	"font",
	"frame-color",
	"input-dir",
	"name",
	"output-dir",
	"output-path",
	"overlay-config",
	"parallel-workers",
	"resume",
	"subtitle",
	"subtitle-color",
	"text-box",
	"text-box-color",
	"text-box-padding",
	"text-box-radius",
	"text-position",
	"title",
	"title-color",
}

// shotsFrameFn frames one --input, anchoring the output directory itself.
// shotsFrameIntoFn frames into an output root the caller retains, so every
// --input-dir file publishes into the directory selected at the start.
var (
	shotsFrameFn     = screenshots.Frame
	shotsFrameIntoFn = screenshots.FrameIntoOutputRoot
)

// NewShotsFrameCommand returns the screenshots frame subcommand.
func NewShotsFrameCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("frame", flag.ExitOnError)
	inputPath := fs.String("input", "", "Path to raw screenshot PNG (required unless --input-dir or --config is set)")
	inputDir := fs.String("input-dir", "", "Directory of raw screenshot PNGs to frame in one batch (non-recursive)")
	configPath := fs.String("config", "", "Path to Koubou YAML config (optional)")
	outputPath := fs.String("output-path", "", "Exact output file path for framed PNG (optional)")
	outputDir := fs.String("output-dir", defaultShotsFrameOutputDir, "Output directory when --output-path is not set")
	name := fs.String("name", "", "Output file name without extension (defaults to input base name)")
	device := fs.String(
		"device",
		string(screenshots.DefaultFrameDevice()),
		fmt.Sprintf("Frame device: %s", strings.Join(screenshots.FrameDeviceValues(), ", ")),
	)
	frameColor := fs.String("frame-color", "", "Frame color variant for --device (see list-frame-devices); defaults to the device's first color")
	title := fs.String("title", "", "Title text overlay")
	subtitle := fs.String("subtitle", "", "Subtitle or keyword text overlay")
	bgColor := fs.String("bg-color", "", "Solid background color (e.g. #1a1a2e); defaults to a dark gradient behind text overlays")
	titleColor := fs.String("title-color", "", "Title text color (e.g. #000000); defaults to #ffffff")
	subtitleColor := fs.String("subtitle-color", "", "Subtitle text color (e.g. #333333); defaults to #aaaaaa")
	font := fs.String("font", "", "Font for title and subtitle overlays: an installed family name or a path to a .ttf, .otf, or .ttc file (defaults to Arial)")
	textPosition := fs.String("text-position", string(screenshots.TextPositionTop), "Title and subtitle placement: "+strings.Join(screenshots.TextPositionValues(), ", "))
	textBox := fs.Bool("text-box", false, "Draw a filled box behind the title and subtitle overlays")
	textBoxColor := fs.String("text-box-color", "", "Text box fill color as #RGB, #RRGGBB, or #RRGGBBAA; defaults to "+screenshots.DefaultTextBoxColor+" (requires --text-box or an overlay textBox entry)")
	textBoxPadding := fs.Int("text-box-padding", 0, "Text box padding in pixels; defaults to a quarter of each overlay's font size")
	textBoxRadius := fs.Int("text-box-radius", 0, "Text box corner radius in pixels; defaults to the padding (0 draws square corners)")
	overlayConfig := fs.String("overlay-config", "", "JSON overlay config with default and data[] title, keyword, background, and textBox entries")
	resume := fs.Bool("resume", false, "Skip inputs whose source hash and render settings match .asc/reports/screenshots-frame/state.json")
	parallelWorkers := fs.Int("parallel-workers", 1, "Frame up to N --input-dir screenshots at once (1 to the CPU count)")
	output := d.BindOutputFlags(fs)
	watch := fs.Bool("watch", false, "Watch config and asset files for changes, auto-regenerate (requires --config)")
	watchDebounce := fs.Duration("watch-debounce", 500*time.Millisecond, "Debounce delay between change detection and regeneration")
	watchReviewDir := fs.String("watch-review-dir", "", "Auto-regenerate review HTML in this directory on each watch cycle")
	watchRawDir := fs.String("watch-raw-dir", "", "Raw screenshots directory for review generation (defaults to config asset dir)")

	return &ffcli.Command{
		Name:       "frame",
		ShortUsage: "asc screenshots frame (--input ./screenshots/raw/home.png | --input-dir ./screenshots/raw | --config ./koubou.yaml) [flags]",
		ShortHelp:  "Compose a screenshot into an Apple device frame.",
		LongHelp: `Compose screenshots using Koubou's YAML-based rendering flow.

Install Koubou v0.21.0 (pip install -U koubou==0.21.0), then kou setup-frames.
Koubou v0.20.0 remains supported for existing frames; iPhone Duo requires v0.21.0.

Use --input for one screenshot, --input-dir to frame every PNG in a
directory, or --config for an explicit Koubou YAML config.

Devices cover iPhone, iPad, Apple Watch, Apple TV, and Mac. Run
asc screenshots list-frame-devices to see each device's --frame-color values.
Use iphone-duo-outer or iphone-duo-inner for Duo portrait frames; append
-landscape to either device value for landscape. Both offer night-sky and
star-white frame colors.
Title and subtitle overlays, --font, --text-position, --text-box, and
--bg-color apply to every device. --font takes an installed family name or a
.ttf, .otf, or .ttc file; asc copies the file into its private Koubou work
directory, so the font does not need to be installed on the rendering host.

With --input-dir, a failed input does not stop the batch: the receipt lists
each input's status and the command exits 1 if any input failed. Add --resume
to record completed inputs and skip them on the next run. Add
--parallel-workers N to run up to N independent renders at once; receipts stay
in input order.

Use --watch with --config to start a live watcher that auto-regenerates
framed screenshots whenever the YAML config or referenced raw assets change.`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			// Keep path values literal after validating emptiness. Trimming a
			// valid path can select a different filesystem entry.
			configVal := *configPath
			inputVal := *inputPath
			inputDirVal := *inputDir
			configSet := strings.TrimSpace(configVal) != ""
			inputSet := strings.TrimSpace(inputVal) != ""
			inputDirSet := strings.TrimSpace(inputDirVal) != ""
			watchDebounceSet := false
			watchReviewDirSet := false
			watchRawDirSet := false
			textPositionSet := false
			textBoxColorSet := false
			textBoxPaddingSet := false
			textBoxRadiusSet := false
			parallelWorkersSet := false
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
				case "text-position":
					textPositionSet = true
				case "text-box-color":
					textBoxColorSet = true
				case "text-box-padding":
					textBoxPaddingSet = true
				case "text-box-radius":
					textBoxRadiusSet = true
				case "parallel-workers":
					parallelWorkersSet = true
				}
				if slices.Contains(watchUnsupportedFrameFlags, flagValue.Name) {
					watchUnsupportedFlags = append(watchUnsupportedFlags, "--"+flagValue.Name)
				}
			})
			if !configSet && !inputSet && !inputDirSet {
				fmt.Fprintln(d.IO.ErrOut, "Error: --input is required when --config is not set")
				return shared.MissingRequiredUsageError("--input")
			}
			if configSet && inputSet {
				fmt.Fprintln(d.IO.ErrOut, "Error: use either --input or --config, not both")
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticConflictingInput, "--config")
			}
			if *watch && !configSet {
				fmt.Fprintln(d.IO.ErrOut, "Error: --watch requires --config")
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticConflictingInput, "--watch")
			}
			if *watch && len(watchUnsupportedFlags) > 0 {
				parameter := ""
				if len(watchUnsupportedFlags) == 1 {
					parameter = watchUnsupportedFlags[0]
				}
				return shared.WithDiagnostic(d.UsageError(fmt.Sprintf(
					"%s cannot be used with --watch; watch mode regenerates from the Koubou YAML config",
					strings.Join(watchUnsupportedFlags, ", "),
				)), shared.DiagnosticConflictingInput, parameter)
			}
			if inputDirSet && (inputSet || configSet) {
				return shared.WithDiagnostic(d.UsageError("use either --input, --input-dir, or --config"), shared.DiagnosticConflictingInput, "--input-dir")
			}
			if parallelWorkersSet && !inputDirSet {
				return shared.WithDiagnostic(d.UsageError("--parallel-workers requires --input-dir"), shared.DiagnosticConflictingInput, "--parallel-workers")
			}
			if maxWorkers := runtime.NumCPU(); *parallelWorkers < 1 || *parallelWorkers > maxWorkers {
				return shared.WithDiagnostic(d.UsageError(fmt.Sprintf("--parallel-workers must be between 1 and %d", maxWorkers)), shared.DiagnosticInvalidInput, "--parallel-workers")
			}
			if inputDirSet && (strings.TrimSpace(*outputPath) != "" || strings.TrimSpace(*name) != "") {
				return shared.WithDiagnostic(d.UsageError("--output-path and --name cannot be used with --input-dir; outputs are named <input>-<device>.png in --output-dir"), shared.DiagnosticConflictingInput, "--input-dir")
			}
			if !*watch {
				switch {
				case watchDebounceSet:
					return shared.WithDiagnostic(d.UsageError("--watch-debounce requires --watch"), shared.DiagnosticConflictingInput, "--watch-debounce")
				case watchReviewDirSet:
					return shared.WithDiagnostic(d.UsageError("--watch-review-dir requires --watch"), shared.DiagnosticConflictingInput, "--watch-review-dir")
				case watchRawDirSet:
					return shared.WithDiagnostic(d.UsageError("--watch-raw-dir requires --watch"), shared.DiagnosticConflictingInput, "--watch-raw-dir")
				}
			}
			if watchRawDirSet && !watchReviewDirSet {
				return shared.WithDiagnostic(d.UsageError("--watch-raw-dir requires --watch-review-dir"), shared.DiagnosticConflictingInput, "--watch-raw-dir")
			}
			if watchReviewDirSet && strings.TrimSpace(*watchReviewDir) == "" {
				return shared.WithDiagnostic(d.UsageError("--watch-review-dir must not be empty"), shared.DiagnosticInvalidInput, "--watch-review-dir")
			}
			if watchDebounceSet && *watchDebounce <= 0 {
				return shared.WithDiagnostic(d.UsageError("--watch-debounce must be greater than 0"), shared.DiagnosticInvalidInput, "--watch-debounce")
			}
			if configSet {
				absConfig, err := d.Abs(configVal)
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
				opts := &screenshots.WatchOptions{Stderr: d.IO.ErrOut}
				if reviewDir := *watchReviewDir; strings.TrimSpace(reviewDir) != "" {
					opts.ReviewOutputDir = userPath(d, reviewDir)
					opts.ReviewRawDir = userPath(d, *watchRawDir)
				}
				return screenshots.WatchAndRegenerate(watchCtx, configVal, *watchDebounce, nil, opts)
			}

			deviceVal, err := screenshots.ParseFrameDevice(*device)
			if err != nil {
				fmt.Fprintf(
					d.IO.ErrOut,
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
			fontSet := strings.TrimSpace(*font) != ""
			frameColorSet := strings.TrimSpace(*frameColor) != ""
			if hasCanvasFlags && configSet {
				fmt.Fprintf(d.IO.ErrOut, "Error: --title, --subtitle, --bg-color, --title-color, --subtitle-color cannot be used with --config; set these in the YAML config instead\n")
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticConflictingInput, "--config")
			}
			if configSet && (frameColorSet || fontSet || textPositionSet) {
				return shared.WithDiagnostic(d.UsageError("--frame-color, --font, and --text-position cannot be used with --config; set the frame, font, and text positions in the YAML config instead"), shared.DiagnosticConflictingInput, "--config")
			}
			textBoxDetailSet := textBoxColorSet || textBoxPaddingSet || textBoxRadiusSet
			if configSet && (*textBox || textBoxDetailSet) {
				return shared.WithDiagnostic(d.UsageError("--text-box, --text-box-color, --text-box-padding, and --text-box-radius cannot be used with --config; set box on the text items in the YAML config instead"), shared.DiagnosticConflictingInput, "--config")
			}
			boxFlags, err := parseTextBoxFlags(d, *textBox, textBoxFlagValues{
				color:      *textBoxColor,
				colorSet:   textBoxColorSet,
				padding:    *textBoxPadding,
				paddingSet: textBoxPaddingSet,
				radius:     *textBoxRadius,
				radiusSet:  textBoxRadiusSet,
			})
			if err != nil {
				return err
			}

			frameColorID, err := screenshots.ResolveFrameColor(deviceVal, *frameColor)
			if err != nil {
				return shared.WithDiagnostic(d.UsageError("--frame-color: "+err.Error()), shared.DiagnosticInvalidInput, "--frame-color")
			}
			textPositionVal, err := screenshots.ParseTextPosition(*textPosition)
			if err != nil {
				return shared.WithDiagnostic(d.UsageError("--text-position: "+err.Error()), shared.DiagnosticInvalidInput, "--text-position")
			}
			hasTextSource := strings.TrimSpace(*title) != "" || strings.TrimSpace(*subtitle) != "" || strings.TrimSpace(*overlayConfig) != ""
			if fontSet && !hasTextSource {
				return shared.WithDiagnostic(d.UsageError("--font requires --title, --subtitle, or --overlay-config"), shared.DiagnosticConflictingInput, "--font")
			}
			if textPositionSet && !hasTextSource {
				return shared.WithDiagnostic(d.UsageError("--text-position requires --title, --subtitle, or --overlay-config"), shared.DiagnosticConflictingInput, "--text-position")
			}
			if *textBox && !hasTextSource {
				return shared.WithDiagnostic(d.UsageError("--text-box requires --title, --subtitle, or --overlay-config"), shared.DiagnosticConflictingInput, "--text-box")
			}
			if textBoxDetailSet && !*textBox && strings.TrimSpace(*overlayConfig) == "" {
				parameter := boxFlags.firstDetailFlag()
				return shared.WithDiagnostic(d.UsageError(parameter+" requires --text-box or an --overlay-config textBox entry"), shared.DiagnosticConflictingInput, parameter)
			}
			fontFamily := strings.TrimSpace(*font)
			var fontFile *screenshots.FontFile
			if fontSet && screenshots.IsFontFilePath(fontFamily) {
				// Keep the path literal; only the family-name form is trimmed.
				fontFile, err = screenshots.LoadFontFile(userPath(d, *font))
				if err != nil {
					return shared.WithDiagnostic(d.UsageError("--font: "+err.Error()), shared.DiagnosticInvalidInput, "--font")
				}
				fontFamily = ""
			}

			var baseCanvas *screenshots.CanvasOptions
			if hasCanvasFlags || fontSet || textPositionSet || *textBox {
				baseCanvas = &screenshots.CanvasOptions{
					Title:         strings.TrimSpace(*title),
					Subtitle:      strings.TrimSpace(*subtitle),
					BGColor:       strings.TrimSpace(*bgColor),
					TitleColor:    strings.TrimSpace(*titleColor),
					SubtitleColor: strings.TrimSpace(*subtitleColor),
					Font:          fontFamily,
					FontFile:      fontFile,
					TextPosition:  textPositionVal,
				}
			}

			settings := frameRenderSettings{
				deps:            d,
				device:          deviceVal,
				frameColor:      frameColorID,
				base:            baseCanvas,
				fontSet:         fontSet,
				textPositionSet: textPositionSet,
				textBox:         boxFlags,
			}
			if strings.TrimSpace(*overlayConfig) != "" {
				if configSet {
					fmt.Fprintln(d.IO.ErrOut, "Error: --overlay-config cannot be used with --config")
					return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticConflictingInput, "--overlay-config")
				}
				loaded, hash, err := screenshots.LoadOverlayConfig(userPath(d, *overlayConfig))
				if err != nil {
					return fmt.Errorf("screenshots frame: %w", err)
				}
				if err := screenshots.ValidateOverlayTextBoxes(loaded, *textBox); err != nil {
					return shared.WithDiagnostic(d.UsageError("--overlay-config: "+err.Error()), shared.DiagnosticInvalidInput, "--overlay-config")
				}
				if textBoxDetailSet && !*textBox && !screenshots.OverlayEnablesTextBox(loaded) {
					parameter := boxFlags.firstDetailFlag()
					return shared.WithDiagnostic(d.UsageError(parameter+" requires --text-box or an --overlay-config textBox entry"), shared.DiagnosticConflictingInput, parameter)
				}
				settings.overlay = &loaded
				settings.overlayHash = hash
			}
			if *resume && configSet {
				fmt.Fprintln(d.IO.ErrOut, "Error: --resume cannot be used with --config")
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticConflictingInput, "--resume")
			}

			if inputDirSet {
				return runFrameBatch(ctx, frameBatchOptions{
					inputDir:  inputDirVal,
					outputDir: *outputDir,
					resume:    *resume,
					workers:   *parallelWorkers,
					settings:  settings,
					output:    *output.Output,
					pretty:    *output.Pretty,
				})
			}

			absInput := ""
			if inputSet {
				var err error
				absInput, err = d.Abs(inputVal)
				if err != nil {
					return fmt.Errorf("screenshots frame: resolve input path: %w", err)
				}
			}

			outputDevice := string(deviceVal)
			if configSet && strings.TrimSpace(*outputPath) == "" {
				outputDevice = screenshots.ResolveFrameDeviceFromConfig(configVal, outputDevice)
			}

			outPath, err := resolveOutputPath(d, *outputPath, *outputDir, *name, absInput, outputDevice)
			if err != nil {
				return fmt.Errorf("screenshots frame: %w", err)
			}

			canvasOpts, err := settings.canvasFor(absInput)
			if err != nil {
				return err
			}
			if err := settings.requireStyledText(canvasOpts != nil && canvasOpts.Title+canvasOpts.Subtitle != ""); err != nil {
				return err
			}

			timeoutCtx, cancel := d.Timeout(ctx)
			defer cancel()

			request := screenshots.FrameRequest{
				InputPath:  absInput,
				OutputPath: outPath,
				Device:     string(deviceVal),
				ConfigPath: configVal,
				Canvas:     canvasOpts,
			}
			if !configSet {
				request.FrameColor = frameColorID
			}

			if *resume && inputSet {
				root, err := frameResumeRoot(d)
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
					openInput := func(ctx context.Context) (*screenshots.FrameInputSnapshot, error) {
						return screenshots.OpenFrameInputSnapshot(ctx, request.InputPath)
					}
					store := &frameResumeStore{state: &state, root: root}
					result, frameErr := frameSnapshot(timeoutCtx, openInput, request, settings, store, nil)
					if frameErr != nil {
						return fmt.Errorf("screenshots frame: %w", frameErr)
					}
					framed = result
					return nil
				})
				if err != nil {
					return classifyFrameError(err)
				}
				return d.PrintOutput(framed, *output.Output, *output.Pretty)
			}

			if inputSet {
				warnFrameAspectMismatch(d, absInput, absInput, deviceVal)
			}
			result, err := shotsFrameFn(timeoutCtx, request)
			if err != nil {
				return classifyFrameError(fmt.Errorf("screenshots frame: %w", err))
			}

			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// classifyFrameError marks failures caused by the caller's frame inputs or a
// missing Koubou install as expected negatives, without changing the message
// or exit code.
func classifyFrameError(err error) error {
	if errors.Is(err, screenshots.ErrKoubouNotFound) {
		return shared.WithDiagnostic(shared.NewValidationError(err), shared.DiagnosticDependencyFailed, "")
	}
	var inputErr *screenshots.FrameInputError
	if !errors.As(err, &inputErr) {
		return err
	}
	parameter := "--input"
	if inputErr.Input == screenshots.FrameInputConfig {
		parameter = "--config"
	}
	return shared.ClassifyLocalFileError(err, parameter, shared.DiagnosticFileInvalidFormat)
}

// frameRenderSettings are the render inputs shared by every framed file in
// one invocation. Per-file overlays are resolved by canvasFor.
type frameRenderSettings struct {
	deps            *shared.Deps
	device          screenshots.FrameDevice
	frameColor      string
	base            *screenshots.CanvasOptions
	overlay         *screenshots.OverlayConfig
	overlayHash     string
	fontSet         bool
	textPositionSet bool
	textBox         textBoxFlags
}

// requireStyledText rejects --font, --text-position, and --text-box flags when
// no framed input resolves to title or subtitle text (or, for box details, to
// a box), so the flags never go unused. With
// --input-dir it is enough for one input to carry text.
func (settings frameRenderSettings) requireStyledText(hasText bool) error {
	d := settings.deps
	if err := settings.textBox.requireUsed(d); err != nil {
		return err
	}
	if hasText || settings.overlay == nil {
		return nil
	}
	switch {
	case settings.fontSet:
		return shared.WithDiagnostic(d.UsageError("--font has no effect: --overlay-config supplies no title or keyword for the framed input"), shared.DiagnosticConflictingInput, "--font")
	case settings.textPositionSet:
		return shared.WithDiagnostic(d.UsageError("--text-position has no effect: --overlay-config supplies no title or keyword for the framed input"), shared.DiagnosticConflictingInput, "--text-position")
	case settings.textBox.enabled:
		return shared.WithDiagnostic(d.UsageError("--text-box has no effect: --overlay-config supplies no title or keyword for the framed input"), shared.DiagnosticConflictingInput, "--text-box")
	}
	return nil
}

// canvasFor merges the --overlay-config entry matching inputPath under the
// explicit text flags and validates the resulting text options.
func (settings frameRenderSettings) canvasFor(inputPath string) (*screenshots.CanvasOptions, error) {
	d := settings.deps
	var canvasOpts *screenshots.CanvasOptions
	if settings.base != nil {
		copied := *settings.base
		canvasOpts = &copied
	}
	var entry screenshots.OverlayEntry
	if settings.overlay != nil {
		entry = screenshots.MatchOverlay(*settings.overlay, inputPath)
		matched := screenshots.OverlayToCanvas(entry)
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
	if canvasOpts != nil && canvasOpts.Title+canvasOpts.Subtitle != "" {
		canvasOpts.TextBox = settings.textBox.resolve(entry)
	}
	if canvasOpts != nil && canvasOpts.TextPosition == "" {
		canvasOpts.TextPosition = screenshots.TextPositionTop
	}
	if canvasOpts != nil && canvasOpts.TitleColor != "" && canvasOpts.Title == "" {
		fmt.Fprintln(d.IO.ErrOut, "Error: --title-color requires --title")
		return nil, shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticConflictingInput, "--title-color")
	}
	if canvasOpts != nil && canvasOpts.SubtitleColor != "" && canvasOpts.Subtitle == "" {
		fmt.Fprintln(d.IO.ErrOut, "Error: --subtitle-color requires --subtitle")
		return nil, shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticConflictingInput, "--subtitle-color")
	}
	return canvasOpts, nil
}

// warnFrameAspectMismatch writes a stderr warning when the PNG at readPath
// does not match device's screen aspect ratio, because Koubou then letterboxes
// it inside the frame. displayPath names the input as the operator selected
// it. Unreadable inputs are left for the render to report.
func warnFrameAspectMismatch(d *shared.Deps, displayPath, readPath string, device screenshots.FrameDevice) {
	mismatch, err := screenshots.CheckFrameInputAspect(readPath, device)
	if err != nil || mismatch == nil {
		return
	}
	fmt.Fprintf(
		d.IO.ErrOut,
		"Warning: %s is %dx%d, but %s screenshots are %dx%d; the image will be letterboxed inside the device screen\n",
		displayPath,
		mismatch.InputWidth,
		mismatch.InputHeight,
		device,
		mismatch.ScreenWidth,
		mismatch.ScreenHeight,
	)
}

// frameInputOpener snapshots one framed input so hashing and rendering read
// the same bytes.
type frameInputOpener func(context.Context) (*screenshots.FrameInputSnapshot, error)

// userPath returns path as the operating system should open it relative to
// d's working directory; a blank path stays blank so callers keep their
// defaults.
func userPath(d *shared.Deps, path string) string {
	if strings.TrimSpace(path) == "" {
		return path
	}
	return d.OSPath(path)
}

// frameResumeStore is the loaded resume state for one invocation. The caller
// holds the cross-process resume lock; mu serializes the renders of one
// --parallel-workers batch that read, record, and save it.
type frameResumeStore struct {
	mu    sync.Mutex
	state *screenshots.FrameResumeState
	root  rootfs.Root
}

// lookup returns the stored entry for outputPath as a one-entry state, so the
// output can be verified without holding mu.
func (store *frameResumeStore) lookup(outputPath string) screenshots.FrameResumeState {
	store.mu.Lock()
	defer store.mu.Unlock()
	lookup := screenshots.FrameResumeState{Files: map[string]screenshots.FrameResumeEntry{}}
	if entry, ok := store.state.Files[outputPath]; ok {
		lookup.Files[outputPath] = entry
	}
	return lookup
}

// record stores a completed render and saves the whole state.
func (store *frameResumeStore) record(outputPath string, entry screenshots.FrameResumeEntry) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.state.Files[outputPath] = entry
	return screenshots.SaveFrameResumeState(store.root, screenshots.FrameResumeStateRel, *store.state)
}

// frameSnapshot renders a protected snapshot of the input opened by open.
// With a non-nil store it skips the render when the state records the same
// fingerprint and output bytes, and saves the state after a successful
// render. A non-nil outputRoot is the retained --output-dir the image is
// published into.
func frameSnapshot(ctx context.Context, open frameInputOpener, request screenshots.FrameRequest, settings frameRenderSettings, store *frameResumeStore, outputRoot *rootfs.Root) (*screenshots.FrameResult, error) {
	snapshot, err := open(ctx)
	if err != nil {
		return nil, fmt.Errorf("snapshot input: %w", err)
	}
	finish := func(result *screenshots.FrameResult, primary error) (*screenshots.FrameResult, error) {
		if closeErr := snapshot.Close(); closeErr != nil {
			return nil, errors.Join(primary, closeErr)
		}
		if primary != nil {
			return nil, primary
		}
		return result, nil
	}
	fingerprint := ""
	if store != nil {
		fingerprint = frameResumeFingerprint(snapshot.SourceHash(), settings, request.Canvas)
		if result, ok := screenshots.ResumeEntry(ctx, store.lookup(request.OutputPath), request.OutputPath, fingerprint); ok {
			return finish(&result, nil)
		}
	}
	warnFrameAspectMismatch(settings.deps, request.InputPath, snapshot.Path(), settings.device)
	request.InputPath = snapshot.Path()
	var result *screenshots.FrameResult
	if outputRoot != nil {
		result, err = shotsFrameIntoFn(ctx, request, *outputRoot)
	} else {
		result, err = shotsFrameFn(ctx, request)
	}
	if err != nil {
		return finish(nil, err)
	}
	if store == nil {
		return finish(result, nil)
	}
	stored := *result
	stored.Skipped = false
	entry := screenshots.FrameResumeEntry{
		Fingerprint: fingerprint,
		OutputHash:  result.OutputHash,
		Result:      stored,
	}
	if err := store.record(request.OutputPath, entry); err != nil {
		return finish(nil, fmt.Errorf("write resume state: %w", err))
	}
	return finish(result, nil)
}

func frameResumeFingerprint(sourceHash string, settings frameRenderSettings, canvas *screenshots.CanvasOptions) string {
	fp := screenshots.FrameResumeFingerprint{
		SourceHash:  sourceHash,
		Device:      string(settings.device),
		OverlayHash: settings.overlayHash,
		FrameColor:  settings.frameColor,
	}
	if canvas != nil {
		fp.Title = canvas.Title
		fp.Subtitle = canvas.Subtitle
		fp.TitleColor = canvas.TitleColor
		fp.SubtitleColor = canvas.SubtitleColor
		fp.Background = canvas.BGColor
		fp.Font = canvas.Font
		fp.TextPosition = string(canvas.TextPosition)
		fp.TextBox = screenshots.TextBoxFingerprint(canvas)
		fp.FontFileHash = canvas.FontFile.Hash()
	}
	return screenshots.FingerprintFrameResume(fp)
}

func frameResumeRoot(d *shared.Deps) (rootfs.Root, error) {
	workingDirectory, err := d.Getwd()
	if err != nil {
		return rootfs.Root{}, fmt.Errorf("screenshots frame: resolve resume root: %w", err)
	}
	root, err := rootfs.New(workingDirectory)
	if err != nil {
		return rootfs.Root{}, fmt.Errorf("screenshots frame: resolve resume root: %w", err)
	}
	return root, nil
}

func resolveOutputPath(d *shared.Deps, explicitPath, outputDir, name, inputPath, device string) (string, error) {
	explicit := explicitPath
	if strings.TrimSpace(explicit) != "" {
		absPath, err := d.Abs(explicit)
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

	absDir, err := d.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve output directory: %w", err)
	}
	return filepath.Join(absDir, fmt.Sprintf("%s-%s.png", baseName, device)), nil
}
