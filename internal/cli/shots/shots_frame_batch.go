package shots

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/screenshots"
)

type frameBatchOptions struct {
	inputDir  string
	outputDir string
	resume    bool
	workers   int
	settings  frameRenderSettings
	output    string
	pretty    bool
}

type frameBatchJob struct {
	input  string
	output string
	canvas *screenshots.CanvasOptions
}

// runFrameBatch frames every PNG directly inside opts.inputDir. A failed input
// is recorded in the receipt and the remaining inputs still render; the
// command then exits 1. With --resume, each completed input is recorded as
// soon as it is published so a rerun skips it.
func runFrameBatch(ctx context.Context, opts frameBatchOptions) error {
	d := opts.settings.deps
	absInputDir, err := d.Abs(opts.inputDir)
	if err != nil {
		return fmt.Errorf("screenshots frame: resolve --input-dir: %w", err)
	}
	absOutputDir, err := d.Abs(defaultIfBlank(opts.outputDir, defaultShotsFrameOutputDir))
	if err != nil {
		return fmt.Errorf("screenshots frame: resolve --output-dir: %w", err)
	}
	inputRoot, err := rootfs.New(absInputDir)
	if err != nil {
		return fmt.Errorf("screenshots frame: read --input-dir: %w", err)
	}
	defer inputRoot.Close()
	same, err := sameRootedDirectory(inputRoot, absOutputDir)
	if err != nil {
		return fmt.Errorf("screenshots frame: inspect --output-dir: %w", err)
	}
	if same {
		// A later run would pick up framed outputs as new inputs.
		return shared.WithDiagnostic(d.UsageError("--output-dir must differ from --input-dir"), shared.DiagnosticConflictingInput, "--output-dir")
	}
	// Anchor --output-dir once, before any render, so replacing it or a
	// parent with a symlink mid-batch cannot redirect later outputs. A
	// missing directory is created on the first publish.
	outputRoot, err := rootfs.New(absOutputDir)
	if err != nil {
		return fmt.Errorf("screenshots frame: open --output-dir: %w", err)
	}
	defer outputRoot.Close()
	inputs, err := listFramePNGs(inputRoot, absInputDir)
	if err != nil {
		return fmt.Errorf("screenshots frame: read --input-dir: %w", err)
	}
	if len(inputs) == 0 {
		return shared.WithDiagnostic(d.UsageError("--input-dir contains no PNG files: "+absInputDir), shared.DiagnosticInvalidInput, "--input-dir")
	}

	// Resolve every output and overlay before rendering anything so usage
	// errors never leave a partially framed batch behind.
	device := string(opts.settings.device)
	jobs := make([]frameBatchJob, 0, len(inputs))
	claimed := make(map[string]string, len(inputs))
	// Inputs that differ only by case collide when the output directory
	// resolves names case-insensitively, so fold case there.
	foldCase := frameOutputFoldsCaseFn(absOutputDir)
	anyText := false
	for _, input := range inputs {
		outPath, err := resolveOutputPath(d, "", opts.outputDir, "", input, device)
		if err != nil {
			return fmt.Errorf("screenshots frame: %w", err)
		}
		key := outPath
		if foldCase {
			key = strings.ToLower(outPath)
		}
		if previous, ok := claimed[key]; ok {
			return shared.WithDiagnostic(d.UsageError(fmt.Sprintf(
				"--input-dir inputs %s and %s map to the same output %s",
				filepath.Base(previous), filepath.Base(input), outPath,
			)), shared.DiagnosticConflictingInput, "--input-dir")
		}
		claimed[key] = input
		canvas, err := opts.settings.canvasFor(input)
		if err != nil {
			return err
		}
		jobs = append(jobs, frameBatchJob{input: input, output: outPath, canvas: canvas})
		if canvas != nil && canvas.Title+canvas.Subtitle != "" {
			anyText = true
		}
	}
	if err := opts.settings.requireStyledText(anyText); err != nil {
		return err
	}

	receipt := asc.ScreenshotFrameBatchResult{
		InputDir:   absInputDir,
		OutputDir:  absOutputDir,
		Device:     device,
		FrameColor: opts.settings.frameColor,
		Resume:     opts.resume,
		Total:      len(jobs),
		Files:      make([]asc.ScreenshotFrameBatchFile, 0, len(jobs)),
	}

	render := func(store *frameResumeStore) {
		receipt.Files = renderFrameBatch(ctx, inputRoot, outputRoot, jobs, opts.settings, store, opts.workers)
	}
	if opts.resume {
		root, err := frameResumeRoot(d)
		if err != nil {
			return err
		}
		defer root.Close()
		// Bound only the wait for another run's lock; each input below gets
		// its own render timeout.
		lockCtx, cancelLock := d.Timeout(ctx)
		defer cancelLock()
		err = screenshots.WithFrameResumeLock(lockCtx, root, func() error {
			state, loadErr := screenshots.LoadFrameResumeState(root, screenshots.FrameResumeStateRel)
			if loadErr != nil {
				return fmt.Errorf("screenshots frame: read resume state: %w", loadErr)
			}
			render(&frameResumeStore{state: &state, root: root})
			return nil
		})
		if err != nil {
			return err
		}
	} else {
		render(nil)
	}

	for _, file := range receipt.Files {
		switch file.Status {
		case asc.ScreenshotFrameBatchStatusFramed:
			receipt.Framed++
		case asc.ScreenshotFrameBatchStatusSkipped:
			receipt.Skipped++
		default:
			receipt.Failed++
		}
	}
	if err := d.PrintOutput(&receipt, opts.output, opts.pretty); err != nil {
		return err
	}
	if receipt.Failed == 0 {
		return nil
	}
	message := fmt.Sprintf("screenshots frame: %d of %d screenshots failed to frame", receipt.Failed, receipt.Total)
	if opts.resume {
		message += "; rerun the same command to retry only the failed inputs"
	}
	fmt.Fprintf(d.IO.ErrOut, "Error: %s\n", message)
	return shared.NewStderrReportedError(errors.New(message))
}

// renderFrameBatch frames jobs with up to workers renders in flight and
// returns one receipt row per job in input order, whatever order the renders
// finish in. Each render is an independent Koubou run with its own snapshot,
// timeout, and rooted publish, so a failure stays confined to its row.
func renderFrameBatch(ctx context.Context, inputRoot, outputRoot rootfs.Root, jobs []frameBatchJob, settings frameRenderSettings, store *frameResumeStore, workers int) []asc.ScreenshotFrameBatchFile {
	files := make([]asc.ScreenshotFrameBatchFile, len(jobs))
	workers = max(1, min(workers, len(jobs)))
	next := make(chan int)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range next {
				files[index] = frameBatchFile(ctx, inputRoot, outputRoot, jobs[index], settings, store)
			}
		}()
	}
	for index := range jobs {
		next <- index
	}
	close(next)
	wg.Wait()
	return files
}

// frameBatchFile renders one batch input with its own timeout and converts the
// outcome into a receipt row. The input is snapshotted through the retained
// --input-dir root, so replacing it after enumeration (for example with a
// symlink) cannot redirect the render outside that directory. The image is
// published through the retained outputRoot for the same reason.
func frameBatchFile(ctx context.Context, inputRoot, outputRoot rootfs.Root, job frameBatchJob, settings frameRenderSettings, store *frameResumeStore) asc.ScreenshotFrameBatchFile {
	file := asc.ScreenshotFrameBatchFile{Input: job.input, Path: job.output}
	if err := ctx.Err(); err != nil {
		file.Status = asc.ScreenshotFrameBatchStatusFailed
		file.Error = err.Error()
		return file
	}
	fileCtx, cancel := settings.deps.Timeout(ctx)
	defer cancel()

	request := screenshots.FrameRequest{
		InputPath:  job.input,
		OutputPath: job.output,
		Device:     string(settings.device),
		FrameColor: settings.frameColor,
		Canvas:     job.canvas,
	}
	openInput := func(ctx context.Context) (*screenshots.FrameInputSnapshot, error) {
		return screenshots.OpenFrameInputSnapshotInRoot(ctx, inputRoot, filepath.Base(job.input))
	}
	result, err := frameSnapshot(fileCtx, openInput, request, settings, store, &outputRoot)
	if err != nil {
		file.Status = asc.ScreenshotFrameBatchStatusFailed
		file.Error = err.Error()
		return file
	}
	file.Status = asc.ScreenshotFrameBatchStatusFramed
	if result.Skipped {
		file.Status = asc.ScreenshotFrameBatchStatusSkipped
	}
	if result.Path != "" {
		file.Path = result.Path
	}
	file.FramePath = result.FramePath
	file.DisplayType = result.DisplayType
	file.Width = result.Width
	file.Height = result.Height
	return file
}

// listFramePNGs returns the regular .png files directly inside the retained
// input root, sorted by name and joined to dir for reporting. Symlinks,
// directories, and other file types are skipped.
func listFramePNGs(root rootfs.Root, dir string) ([]string, error) {
	handle, err := root.OpenDir(".")
	if err != nil {
		return nil, err
	}
	entries, readErr := handle.ReadDir(-1)
	closeErr := handle.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !strings.EqualFold(filepath.Ext(entry.Name()), ".png") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	inputs := make([]string, 0, len(names))
	for _, name := range names {
		inputs = append(inputs, filepath.Join(dir, name))
	}
	return inputs, nil
}

func defaultIfBlank(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

// sameRootedDirectory reports whether outputDir names the directory anchored
// by inputRoot, including through symlinks or case-insensitive filesystems.
// Identities are compared from rooted directory handles. A missing output
// directory cannot be the input directory.
func sameRootedDirectory(inputRoot rootfs.Root, outputDir string) (bool, error) {
	if filepath.Clean(inputRoot.Path()) == filepath.Clean(outputDir) {
		return true, nil
	}
	// Existence is only a precondition; identity comes from the rooted
	// handles below.
	if _, err := os.Stat(outputDir); errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	outputRoot, err := rootfs.New(outputDir)
	if err != nil {
		return false, err
	}
	defer outputRoot.Close()
	inputInfo, err := statRootedDirectory(inputRoot)
	if err != nil {
		return false, err
	}
	outputInfo, err := statRootedDirectory(outputRoot)
	if err != nil {
		return false, err
	}
	return os.SameFile(inputInfo, outputInfo), nil
}

func statRootedDirectory(root rootfs.Root) (os.FileInfo, error) {
	handle, err := root.OpenDir(".")
	if err != nil {
		return nil, err
	}
	info, statErr := handle.Stat()
	return info, errors.Join(statErr, handle.Close())
}

// frameOutputFoldsCaseFn is replaced in tests to exercise both filesystem kinds.
var frameOutputFoldsCaseFn = pathFoldsCase

// pathFoldsCase reports whether names under dir resolve case-insensitively.
// It finds the nearest existing directory, then looks up one of its entries
// (or, when it has none to probe, the directory itself) under a case-swapped
// name. When nothing can be probed it assumes case folding, which only makes
// collision detection stricter.
func pathFoldsCase(dir string) bool {
	current := filepath.Clean(dir)
	for {
		info, err := os.Lstat(current)
		if err == nil && info.IsDir() {
			break
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return true
		}
		parent := filepath.Dir(current)
		if parent == current {
			return true
		}
		current = parent
	}
	if folds, ok := probeChildFoldsCase(current); ok {
		return folds
	}
	for {
		parent := filepath.Dir(current)
		if parent == current {
			return true
		}
		if folds, ok := probeFoldsCase(parent, filepath.Base(current)); ok {
			return folds
		}
		current = parent
	}
}

// probeChildFoldsCase probes the first entry of dir whose name has a cased
// letter. ok is false when dir has no such entry or cannot be read.
func probeChildFoldsCase(dir string) (folds bool, ok bool) {
	handle, err := os.Open(dir)
	if err != nil {
		return false, false
	}
	defer handle.Close()
	for {
		names, err := handle.Readdirnames(64)
		for _, name := range names {
			if folds, ok := probeFoldsCase(dir, name); ok {
				return folds, true
			}
		}
		if err != nil {
			return false, false
		}
	}
}

// probeFoldsCase looks up name in dir under a case-swapped spelling. ok is
// false when name has no cased letter or the lookup cannot decide.
func probeFoldsCase(dir, name string) (folds bool, ok bool) {
	swapped := swapLetterCase(name)
	if swapped == name {
		return false, false
	}
	original, err := os.Lstat(filepath.Join(dir, name))
	if err != nil {
		return false, false
	}
	probe, err := os.Lstat(filepath.Join(dir, swapped))
	if errors.Is(err, os.ErrNotExist) {
		return false, true
	}
	if err != nil {
		return false, false
	}
	return os.SameFile(original, probe), true
}

func swapLetterCase(value string) string {
	return strings.Map(func(r rune) rune {
		if upper := unicode.ToUpper(r); upper != r {
			return upper
		}
		return unicode.ToLower(r)
	}, value)
}
