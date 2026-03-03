package screenshots

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

// WatchOptions configures optional review regeneration after each watch cycle.
type WatchOptions struct {
	// ReviewOutputDir, when non-empty, triggers automatic review HTML/manifest
	// regeneration after each successful kou generate cycle.
	ReviewOutputDir string
	// ReviewRawDir is the raw screenshots directory for review generation.
	ReviewRawDir string
}

// WatchAndRegenerate watches a Koubou YAML config file (and the raw asset
// directories it references) for changes, then re-runs kou generate on each
// change.  It blocks until ctx is cancelled.
func WatchAndRegenerate(ctx context.Context, configPath string, debounce time.Duration, onCycle func(results []WatchCycleResult, err error), opts *WatchOptions) error {
	absConfig, err := filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("watch: resolve config path: %w", err)
	}
	if _, err := os.Stat(absConfig); err != nil {
		return fmt.Errorf("watch: config file not found: %w", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("watch: create watcher: %w", err)
	}
	defer func() { _ = watcher.Close() }()

	// Watch the config file's parent directory (fsnotify needs directories on
	// some platforms, and this also catches renames/re-creates of the file).
	configDir := filepath.Dir(absConfig)
	if err := watcher.Add(configDir); err != nil {
		return fmt.Errorf("watch: add config dir %q: %w", configDir, err)
	}

	// Also watch every unique raw-asset directory referenced by the config.
	assetDirs := collectAssetDirs(absConfig)
	for _, dir := range assetDirs {
		if err := watcher.Add(dir); err != nil {
			fmt.Fprintf(os.Stderr, "watch: could not add asset dir %q: %v\n", dir, err)
		}
	}

	if debounce <= 0 {
		debounce = 500 * time.Millisecond
	}

	fmt.Fprintf(os.Stderr, "Watching %s for changes (debounce %s)…\n", absConfig, debounce)
	fmt.Fprintf(os.Stderr, "Press Ctrl-C to stop.\n")

	// Resolve review options once up front.
	var reviewReq *ReviewRequest
	if opts != nil && opts.ReviewOutputDir != "" {
		framedDir := resolveKoubouOutputDir(absConfig)
		rawDir := opts.ReviewRawDir
		if rawDir == "" {
			// Fall back to the first asset dir collected from the config.
			if len(assetDirs) > 0 {
				rawDir = assetDirs[0]
			}
		}
		reviewReq = &ReviewRequest{
			FramedDir: framedDir,
			RawDir:    rawDir,
			OutputDir: opts.ReviewOutputDir,
		}
		fmt.Fprintf(os.Stderr, "Review HTML will auto-regenerate in %s\n", opts.ReviewOutputDir)
	}

	// Run one initial generation so the user sees output immediately.
	runGeneration(ctx, absConfig, reviewReq, onCycle)
	coalescer := newGenerationCoalescer(func() {
		runGeneration(ctx, absConfig, reviewReq, onCycle)
	})

	var timer *time.Timer
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if !isRelevantChange(event, absConfig, assetDirs) {
				continue
			}
			// Debounce: reset the timer on every qualifying event so rapid
			// saves trigger only one generation.
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(debounce, func() {
				fmt.Fprintf(os.Stderr, "\n--- change detected: %s ---\n", event.Name)
				coalescer.Trigger()
			})
		case watchErr, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Fprintf(os.Stderr, "watch error: %v\n", watchErr)
		}
	}
}

type generationCoalescer struct {
	mu      sync.Mutex
	running bool
	pending bool
	run     func()
}

func newGenerationCoalescer(run func()) *generationCoalescer {
	return &generationCoalescer{run: run}
}

func (coalescer *generationCoalescer) Trigger() {
	coalescer.mu.Lock()
	if coalescer.running {
		coalescer.pending = true
		coalescer.mu.Unlock()
		return
	}
	coalescer.running = true
	coalescer.mu.Unlock()

	for {
		coalescer.run()

		coalescer.mu.Lock()
		if !coalescer.pending {
			coalescer.running = false
			coalescer.mu.Unlock()
			return
		}
		coalescer.pending = false
		coalescer.mu.Unlock()
	}
}

// WatchCycleResult describes one screenshot generated per watch cycle.
type WatchCycleResult struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

func runGeneration(ctx context.Context, configPath string, reviewReq *ReviewRequest, onCycle func([]WatchCycleResult, error)) {
	results, err := runKoubouGenerate(ctx, configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "generation error: %v\n", err)
		if onCycle != nil {
			onCycle(nil, err)
		}
		return
	}

	cycleResults := make([]WatchCycleResult, 0, len(results))
	anySuccess := false
	for _, r := range results {
		cr := WatchCycleResult(r)
		cycleResults = append(cycleResults, cr)
		if r.Success {
			anySuccess = true
			fmt.Fprintf(os.Stderr, "  ✓ %s → %s\n", r.Name, r.Path)
		} else {
			fmt.Fprintf(os.Stderr, "  ✗ %s: %s\n", r.Name, r.Error)
		}
	}

	// Auto-regenerate review HTML/manifest when at least one screenshot succeeded.
	if anySuccess && reviewReq != nil {
		reviewResult, reviewErr := GenerateReview(ctx, *reviewReq)
		if reviewErr != nil {
			fmt.Fprintf(os.Stderr, "  review error: %v\n", reviewErr)
		} else {
			fmt.Fprintf(os.Stderr, "  ✓ review → %s (%d ready)\n", reviewResult.HTMLPath, reviewResult.Ready)
		}
	}

	if onCycle != nil {
		onCycle(cycleResults, nil)
	}
}

// isRelevantChange returns true when the fsnotify event affects either the
// config file itself or a .png/.jpg/.jpeg file inside a watched asset dir.
func isRelevantChange(event fsnotify.Event, configPath string, assetDirs []string) bool {
	if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
		return false
	}
	// Config file changed.
	absEvent, err := filepath.Abs(event.Name)
	if err != nil {
		return false
	}
	if absEvent == configPath {
		return true
	}
	// Image file in an asset dir changed.
	ext := strings.ToLower(filepath.Ext(absEvent))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
		return false
	}
	eventDir := filepath.Dir(absEvent)
	for _, dir := range assetDirs {
		if eventDir == dir {
			return true
		}
	}
	return false
}

// resolveKoubouOutputDir reads the project.output_dir from a Koubou YAML config.
func resolveKoubouOutputDir(configPath string) string {
	type project struct {
		OutputDir string `yaml:"output_dir"`
	}
	type parsed struct {
		Project project `yaml:"project"`
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}
	var cfg parsed
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return ""
	}
	dir := strings.TrimSpace(cfg.Project.OutputDir)
	if dir == "" {
		return ""
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(filepath.Dir(configPath), dir)
	}
	return dir
}

// collectAssetDirs parses a Koubou YAML config and returns the unique parent
// directories of every referenced asset path.
func collectAssetDirs(configPath string) []string {
	type contentItem struct {
		Type  string `yaml:"type"`
		Asset string `yaml:"asset"`
	}
	type screenshot struct {
		Content []contentItem `yaml:"content"`
	}
	type parsed struct {
		Screenshots map[string]screenshot `yaml:"screenshots"`
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil
	}
	var cfg parsed
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil
	}

	seen := make(map[string]bool)
	var dirs []string
	for _, ss := range cfg.Screenshots {
		for _, item := range ss.Content {
			if item.Type != "image" || strings.TrimSpace(item.Asset) == "" {
				continue
			}
			assetPath := strings.TrimSpace(item.Asset)
			if !filepath.IsAbs(assetPath) {
				assetPath = filepath.Join(filepath.Dir(configPath), assetPath)
			}
			dir := filepath.Dir(assetPath)
			abs, err := filepath.Abs(dir)
			if err != nil {
				continue
			}
			if seen[abs] {
				continue
			}
			seen[abs] = true
			dirs = append(dirs, abs)
		}
	}
	return dirs
}
