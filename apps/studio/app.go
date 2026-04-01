package main

import (
	"context"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/apps/studio/internal/studio/acp"
	"github.com/rudrankriyam/App-Store-Connect-CLI/apps/studio/internal/studio/approvals"
	"github.com/rudrankriyam/App-Store-Connect-CLI/apps/studio/internal/studio/environment"
	"github.com/rudrankriyam/App-Store-Connect-CLI/apps/studio/internal/studio/settings"
	"github.com/rudrankriyam/App-Store-Connect-CLI/apps/studio/internal/studio/threads"
)

type App struct {
	ctx         context.Context
	rootDir     string
	settings    *settings.Store
	threads     *threads.Store
	approvals   *approvals.Queue
	environment *environment.Service

	mu           sync.Mutex
	sessions     map[string]*threadSession
	sessionInits map[string]*sessionInit
	startAgent   func(context.Context, acp.LaunchSpec) (agentClient, error)

	// Cached auth credentials — read once from config on startup, immune to wipes
	cachedKeyID          string
	cachedIssuerID       string
	cachedPrivateKeyPath string
}

var allowedStudioCommandPaths = map[string]struct{}{
	"accessibility list":                    {},
	"account status":                        {},
	"age-rating view":                       {},
	"agreements list":                       {},
	"alternative-distribution domains list": {},
	"analytics requests":                    {},
	"android-ios-mapping list":              {},
	"app-clips list":                        {},
	"app-events list":                       {},
	"app-setup info list":                   {},
	"app-tags list":                         {},
	"background-assets list":                {},
	"build-bundles list":                    {},
	"build-localizations list":              {},
	"builds list":                           {},
	"bundle-ids create":                     {},
	"bundle-ids list":                       {},
	"categories list":                       {},
	"certificates list":                     {},
	"devices list":                          {},
	"devices register":                      {},
	"encryption declarations list":          {},
	"eula list":                             {},
	"eula view":                             {},
	"game-center achievements list":         {},
	"iap list":                              {},
	"insights weekly":                       {},
	"localizations list":                    {},
	"localizations preview-sets list":       {},
	"marketplace search-details view":       {},
	"merchant-ids list":                     {},
	"metadata pull":                         {},
	"nominations list":                      {},
	"pass-type-ids list":                    {},
	"performance diagnostics list":          {},
	"performance metrics list":              {},
	"pre-orders view":                       {},
	"pricing view":                          {},
	"product-pages custom-pages list":       {},
	"product-pages experiments list":        {},
	"profiles list":                         {},
	"review submissions-list":               {},
	"reviews list":                          {},
	"routing-coverage list":                 {},
	"sandbox list":                          {},
	"schema index":                          {},
	"status":                                {},
	"users list":                            {},
	"versions list":                         {},
	"webhooks list":                         {},
	"workflow list":                         {},
	"xcode-cloud workflows list":            {},
}

type threadSession struct {
	client    agentClient
	sessionID string
}

type sessionInit struct {
	done       chan struct{}
	waiters    int
	err        error
	panicValue any
}

type agentClient interface {
	Bootstrap(context.Context, acp.SessionConfig) (string, error)
	Prompt(context.Context, string, string) (acp.PromptResult, []acp.Event, error)
	Close() error
}

func NewApp() (*App, error) {
	rootDir, err := settings.DefaultRoot()
	if err != nil {
		return nil, err
	}

	return &App{
		rootDir:      rootDir,
		settings:     settings.NewStore(rootDir),
		threads:      threads.NewStore(rootDir),
		approvals:    approvals.NewQueue(),
		environment:  environment.NewService(),
		sessions:     make(map[string]*threadSession),
		sessionInits: make(map[string]*sessionInit),
		startAgent:   startACPClient,
	}, nil
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.cacheAuthFromConfig()
}

func (a *App) shutdown(context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for key, session := range a.sessions {
		if session != nil && session.client != nil {
			_ = session.client.Close()
		}
		delete(a.sessions, key)
	}
}

func (a *App) Bootstrap() (BootstrapData, error) {
	cfg, err := a.settings.Load()
	if err != nil {
		return BootstrapData{}, err
	}

	snapshot, err := a.environment.Snapshot()
	if err != nil {
		return BootstrapData{}, err
	}

	existingThreads, err := a.threads.LoadAll()
	if err != nil {
		return BootstrapData{}, err
	}

	return BootstrapData{
		AppName:      "ASC Studio",
		Tagline:      "The glassy desktop workspace for App Store Connect, powered by asc.",
		GeneratedAt:  formatTimestamp(time.Now().UTC()),
		Sections:     defaultSections(),
		Settings:     cfg,
		Presets:      settings.DefaultPresets(),
		Environment:  snapshot,
		Threads:      toStudioThreads(existingThreads),
		Approvals:    toStudioApprovals(a.approvals.Pending()),
		WindowFlavor: "translucent",
	}, nil
}

func (a *App) GetSettings() (settings.StudioSettings, error) {
	return a.settings.Load()
}

func (a *App) SaveSettings(next settings.StudioSettings) (settings.StudioSettings, error) {
	next.Normalize()
	if err := a.settings.Save(next); err != nil {
		return settings.StudioSettings{}, err
	}
	return a.settings.Load()
}

func (a *App) contextOrBackground() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

func defaultSections() []WorkspaceSection {
	return []WorkspaceSection{
		{ID: "apps", Label: "Apps", Description: "Select an app and pin its release context into Studio."},
		{ID: "overview", Label: "Overview", Description: "Monitor release readiness, metadata drift, and unresolved blockers."},
		{ID: "builds", Label: "Builds", Description: "Inspect TestFlight and App Store build status in one place."},
		{ID: "submission", Label: "Submission", Description: "Preview validation and guarded mutation flows before publish."},
		{ID: "assets", Label: "Assets", Description: "Track screenshots and localization surfaces for app store readiness."},
		{ID: "threads", Label: "Threads", Description: "Keep ACP threads, approvals, and release history together."},
	}
}

var startACPClient = func(ctx context.Context, spec acp.LaunchSpec) (agentClient, error) {
	return acp.Start(ctx, spec)
}

var (
	execLookPathFunc   = func(file string) (string, error) { return exec.LookPath(file) }
	osExecutableFunc   = os.Executable
	getwdFunc          = os.Getwd
	sessionInitTimeout = 15 * time.Second
)
