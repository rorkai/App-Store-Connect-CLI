package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/rudrankriyam/App-Store-Connect-CLI/apps/studio/internal/studio/acp"
	"github.com/rudrankriyam/App-Store-Connect-CLI/apps/studio/internal/studio/approvals"
	"github.com/rudrankriyam/App-Store-Connect-CLI/apps/studio/internal/studio/ascbin"
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

	mu       sync.Mutex
	sessions map[string]*threadSession
}

type threadSession struct {
	client    *acp.Client
	sessionID string
}

type BootstrapData struct {
	AppName      string                    `json:"appName"`
	Tagline      string                    `json:"tagline"`
	GeneratedAt  time.Time                 `json:"generatedAt"`
	Sections     []WorkspaceSection        `json:"sections"`
	Settings     settings.StudioSettings   `json:"settings"`
	Presets      []settings.ProviderPreset `json:"presets"`
	Environment  environment.Snapshot      `json:"environment"`
	Threads      []threads.Thread          `json:"threads"`
	Approvals    []approvals.Action        `json:"approvals"`
	WindowFlavor string                    `json:"windowFlavor"`
}

type WorkspaceSection struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

type PromptRequest struct {
	ThreadID string `json:"threadId"`
	Prompt   string `json:"prompt"`
}

type PromptResponse struct {
	Thread threads.Thread `json:"thread"`
}

type ApprovalRequest struct {
	ThreadID        string   `json:"threadId"`
	Title           string   `json:"title"`
	Summary         string   `json:"summary"`
	CommandPreview  []string `json:"commandPreview"`
	MutationSurface string   `json:"mutationSurface"`
}

type ResolutionResponse struct {
	ascbin.Resolution
	AvailablePresets []settings.ProviderPreset `json:"availablePresets"`
}

type AuthStatus struct {
	Authenticated bool   `json:"authenticated"`
	Storage       string `json:"storage"`
	Profile       string `json:"profile"`
	RawOutput     string `json:"rawOutput"`
}

func NewApp() (*App, error) {
	rootDir, err := settings.DefaultRoot()
	if err != nil {
		return nil, err
	}

	return &App{
		rootDir:     rootDir,
		settings:    settings.NewStore(rootDir),
		threads:     threads.NewStore(rootDir),
		approvals:   approvals.NewQueue(),
		environment: environment.NewService(),
		sessions:    make(map[string]*threadSession),
	}, nil
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
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
		GeneratedAt:  time.Now().UTC(),
		Sections:     defaultSections(),
		Settings:     cfg,
		Presets:      settings.DefaultPresets(),
		Environment:  snapshot,
		Threads:      existingThreads,
		Approvals:    a.approvals.Pending(),
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

func (a *App) CheckAuthStatus() (AuthStatus, error) {
	ascPath, err := a.resolveASCPath()
	if err != nil {
		return AuthStatus{RawOutput: "Could not find asc binary: " + err.Error()}, nil
	}

	ctx, cancel := context.WithTimeout(a.contextOrBackground(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, ascPath, "auth", "status", "--output", "json")
	cmd.Env = append(os.Environ(), "ASC_BYPASS_KEYCHAIN=1")
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))

	status := AuthStatus{RawOutput: output}

	if err != nil {
		status.Authenticated = false
		return status, nil
	}

	// Exit 0 means credentials exist. Try to parse JSON output.
	status.Authenticated = true

	var jsonStatus struct {
		StorageBackend  string `json:"storageBackend"`
		StorageLocation string `json:"storageLocation"`
		Credentials     []struct {
			Name      string `json:"name"`
			KeyID     string `json:"keyId"`
			IsDefault bool   `json:"isDefault"`
		} `json:"credentials"`
	}
	if json.Unmarshal([]byte(output), &jsonStatus) == nil {
		status.Storage = jsonStatus.StorageBackend
		for _, cred := range jsonStatus.Credentials {
			if cred.IsDefault {
				status.Profile = cred.Name
				break
			}
		}
		if status.Profile == "" && len(jsonStatus.Credentials) > 0 {
			status.Profile = jsonStatus.Credentials[0].Name
		}
	}

	return status, nil
}

func (a *App) resolveASCPath() (string, error) {
	cfg, err := a.settings.Load()
	if err != nil {
		return "", err
	}

	bundled := filepath.Join(a.rootDir, "bin", "asc")
	resolution, err := ascbin.Resolve(ascbin.ResolveOptions{
		BundledPath:    bundled,
		SystemOverride: cfg.SystemASCPath,
		PreferBundled:  cfg.PreferBundledASC,
		LookPath:       execLookPath,
	})
	if err != nil {
		return "", err
	}
	return resolution.Path, nil
}

func (a *App) ListThreads() ([]threads.Thread, error) {
	return a.threads.LoadAll()
}

func (a *App) CreateThread(title string) (threads.Thread, error) {
	if strings.TrimSpace(title) == "" {
		title = "New Studio Thread"
	}

	now := time.Now().UTC()
	thread := threads.Thread{
		ID:        uuid.NewString(),
		Title:     title,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := a.threads.SaveThread(thread); err != nil {
		return threads.Thread{}, err
	}
	return thread, nil
}

func (a *App) ResolveASC() (ResolutionResponse, error) {
	cfg, err := a.settings.Load()
	if err != nil {
		return ResolutionResponse{}, err
	}

	bundled := filepath.Join(a.rootDir, "bin", "asc")
	resolution, err := ascbin.Resolve(ascbin.ResolveOptions{
		BundledPath:    bundled,
		SystemOverride: cfg.SystemASCPath,
		PreferBundled:  cfg.PreferBundledASC,
		LookPath:       execLookPath,
	})
	if err != nil {
		return ResolutionResponse{}, err
	}

	return ResolutionResponse{
		Resolution:       resolution,
		AvailablePresets: settings.DefaultPresets(),
	}, nil
}

func (a *App) QueueMutation(req ApprovalRequest) (approvals.Action, error) {
	if strings.TrimSpace(req.ThreadID) == "" {
		return approvals.Action{}, errors.New("thread ID is required")
	}
	if strings.TrimSpace(req.Title) == "" {
		return approvals.Action{}, errors.New("title is required")
	}

	action := approvals.Action{
		ID:              uuid.NewString(),
		ThreadID:        req.ThreadID,
		Title:           req.Title,
		Summary:         req.Summary,
		CommandPreview:  req.CommandPreview,
		MutationSurface: req.MutationSurface,
		Status:          approvals.StatusPending,
		CreatedAt:       time.Now().UTC(),
	}
	return a.approvals.Enqueue(action), nil
}

func (a *App) ListApprovals() []approvals.Action {
	return a.approvals.Pending()
}

func (a *App) ApproveAction(id string) (approvals.Action, error) {
	return a.approvals.Approve(id)
}

func (a *App) RejectAction(id string) (approvals.Action, error) {
	return a.approvals.Reject(id)
}

func (a *App) SendPrompt(req PromptRequest) (PromptResponse, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return PromptResponse{}, errors.New("prompt is required")
	}

	thread, err := a.ensureThread(req.ThreadID)
	if err != nil {
		return PromptResponse{}, err
	}

	thread.Messages = append(thread.Messages, threads.Message{
		ID:        uuid.NewString(),
		Role:      threads.RoleUser,
		Kind:      threads.KindMessage,
		Content:   req.Prompt,
		CreatedAt: time.Now().UTC(),
	})
	thread.UpdatedAt = time.Now().UTC()
	if err := a.threads.SaveThread(thread); err != nil {
		return PromptResponse{}, err
	}

	session, err := a.ensureSession(thread)
	if err != nil {
		return PromptResponse{}, err
	}

	ctx, cancel := context.WithTimeout(a.contextOrBackground(), 15*time.Second)
	defer cancel()

	result, events, err := session.client.Prompt(ctx, session.sessionID, req.Prompt)
	if err != nil {
		return PromptResponse{}, err
	}

	for _, event := range events {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "studio:agent:update", event)
		}
	}

	assistantMessage := result.Summary()
	if assistantMessage == "" {
		assistantMessage = "ASC Studio captured the prompt and is waiting for the agent response stream."
	}

	thread.Messages = append(thread.Messages, threads.Message{
		ID:        uuid.NewString(),
		Role:      threads.RoleAssistant,
		Kind:      threads.KindMessage,
		Content:   assistantMessage,
		CreatedAt: time.Now().UTC(),
	})
	thread.SessionID = session.sessionID
	thread.UpdatedAt = time.Now().UTC()
	if err := a.threads.SaveThread(thread); err != nil {
		return PromptResponse{}, err
	}

	return PromptResponse{Thread: thread}, nil
}

func (a *App) ensureThread(id string) (threads.Thread, error) {
	if strings.TrimSpace(id) == "" {
		return a.CreateThread("New Studio Thread")
	}
	return a.threads.Get(id)
}

func (a *App) ensureSession(thread threads.Thread) (*threadSession, error) {
	a.mu.Lock()
	existing := a.sessions[thread.ID]
	a.mu.Unlock()
	if existing != nil {
		return existing, nil
	}

	cfg, err := a.settings.Load()
	if err != nil {
		return nil, err
	}
	launch, err := cfg.ResolveAgentLaunch()
	if err != nil {
		return nil, err
	}

	client, err := acp.Start(a.contextOrBackground(), acp.LaunchSpec{
		Command: launch.Command,
		Args:    launch.Args,
		Dir:     launch.Dir,
		Env:     launch.Env,
	})
	if err != nil {
		return nil, err
	}

	workspaceRoot := cfg.WorkspaceRoot
	if strings.TrimSpace(workspaceRoot) == "" {
		workspaceRoot, _ = os.Getwd()
	}

	sessionID, err := client.Bootstrap(a.contextOrBackground(), acp.SessionConfig{
		CWD:       workspaceRoot,
		SessionID: thread.SessionID,
	})
	if err != nil {
		_ = client.Close()
		return nil, err
	}

	session := &threadSession{
		client:    client,
		sessionID: sessionID,
	}

	a.mu.Lock()
	a.sessions[thread.ID] = session
	a.mu.Unlock()

	return session, nil
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

func execLookPath(file string) (string, error) {
	return execLookPathFunc(file)
}

var execLookPathFunc = func(file string) (string, error) {
	return exec.LookPath(file)
}
