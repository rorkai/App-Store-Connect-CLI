package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/kballard/go-shellquote"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/rudrankriyam/App-Store-Connect-CLI/apps/studio/internal/studio/acp"
	"github.com/rudrankriyam/App-Store-Connect-CLI/apps/studio/internal/studio/approvals"
	"github.com/rudrankriyam/App-Store-Connect-CLI/apps/studio/internal/studio/ascbin"
	"github.com/rudrankriyam/App-Store-Connect-CLI/apps/studio/internal/studio/environment"
	"github.com/rudrankriyam/App-Store-Connect-CLI/apps/studio/internal/studio/settings"
	"github.com/rudrankriyam/App-Store-Connect-CLI/apps/studio/internal/studio/threads"
)

var ascConfigGuardMu sync.Mutex

type App struct {
	ctx         context.Context
	rootDir     string
	settings    *settings.Store
	threads     *threads.Store
	approvals   *approvals.Queue
	environment *environment.Service

	mu           sync.Mutex
	sessions     map[string]*threadSession
	sessionInits map[string]chan struct{}
	startAgent   func(context.Context, acp.LaunchSpec) (agentClient, error)
}

type threadSession struct {
	client    agentClient
	sessionID string
}

type agentClient interface {
	Bootstrap(context.Context, acp.SessionConfig) (string, error)
	Prompt(context.Context, string, string) (acp.PromptResult, []acp.Event, error)
	Close() error
}

type BootstrapData struct {
	AppName      string                    `json:"appName"`
	Tagline      string                    `json:"tagline"`
	GeneratedAt  string                    `json:"generatedAt"`
	Sections     []WorkspaceSection        `json:"sections"`
	Settings     settings.StudioSettings   `json:"settings"`
	Presets      []settings.ProviderPreset `json:"presets"`
	Environment  environment.Snapshot      `json:"environment"`
	Threads      []StudioThread            `json:"threads"`
	Approvals    []StudioApproval          `json:"approvals"`
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
	Thread StudioThread `json:"thread"`
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

type StudioMessage struct {
	ID        string       `json:"id"`
	Role      threads.Role `json:"role"`
	Kind      threads.Kind `json:"kind"`
	Content   string       `json:"content"`
	CreatedAt string       `json:"createdAt"`
}

type StudioThread struct {
	ID        string          `json:"id"`
	Title     string          `json:"title"`
	SessionID string          `json:"sessionId,omitempty"`
	CreatedAt string          `json:"createdAt"`
	UpdatedAt string          `json:"updatedAt"`
	Messages  []StudioMessage `json:"messages"`
}

type StudioApproval struct {
	ID              string           `json:"id"`
	ThreadID        string           `json:"threadId"`
	Title           string           `json:"title"`
	Summary         string           `json:"summary"`
	CommandPreview  []string         `json:"commandPreview"`
	MutationSurface string           `json:"mutationSurface"`
	Status          approvals.Status `json:"status"`
	CreatedAt       string           `json:"createdAt"`
	ResolvedAt      string           `json:"resolvedAt,omitempty"`
}

type AuthStatus struct {
	Authenticated bool   `json:"authenticated"`
	Storage       string `json:"storage"`
	Profile       string `json:"profile"`
	RawOutput     string `json:"rawOutput"`
}

type AppInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Subtitle string `json:"subtitle"`
	BundleID string `json:"bundleId"`
	Platform string `json:"platform"`
	SKU      string `json:"sku"`
}

type ListAppsResponse struct {
	Apps  []AppInfo `json:"apps"`
	Error string    `json:"error,omitempty"`
}

type AppVersion struct {
	ID       string `json:"id"`
	Platform string `json:"platform"`
	Version  string `json:"version"`
	State    string `json:"state"`
}

type AppDetail struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	Subtitle      string       `json:"subtitle"`
	BundleID      string       `json:"bundleId"`
	SKU           string       `json:"sku"`
	PrimaryLocale string       `json:"primaryLocale"`
	Versions      []AppVersion `json:"versions"`
	Error         string       `json:"error,omitempty"`
}

type BetaGroup struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	IsInternal      bool   `json:"isInternal"`
	PublicLink      string `json:"publicLink"`
	FeedbackEnabled bool   `json:"feedbackEnabled"`
	CreatedDate     string `json:"createdDate"`
	TesterCount     int    `json:"testerCount"`
}

type BetaTester struct {
	Email      string `json:"email"`
	FirstName  string `json:"firstName"`
	LastName   string `json:"lastName"`
	InviteType string `json:"inviteType"`
	State      string `json:"state"`
}

type TestFlightResponse struct {
	Groups  []BetaGroup  `json:"groups"`
	Testers []BetaTester `json:"testers"`
	Error   string       `json:"error,omitempty"`
}

type OfferCode struct {
	SubscriptionName string   `json:"subscriptionName"`
	SubscriptionID   string   `json:"subscriptionId"`
	Name             string   `json:"name"`
	Eligibility      string   `json:"offerEligibility"`
	Customers        []string `json:"customerEligibilities"`
	Duration         string   `json:"duration"`
	OfferMode        string   `json:"offerMode"`
	Periods          int      `json:"numberOfPeriods"`
	TotalCodes       int      `json:"totalNumberOfCodes"`
	ProductionCodes  int      `json:"productionCodeCount"`
}

type OfferCodesResponse struct {
	OfferCodes []OfferCode `json:"offerCodes"`
	Error      string      `json:"error,omitempty"`
}

type FinanceRegion struct {
	Region    string `json:"reportRegion"`
	Currency  string `json:"reportCurrency"`
	Code      string `json:"regionCode"`
	Countries string `json:"countriesOrRegions"`
}

type FinanceResponse struct {
	Regions []FinanceRegion `json:"regions"`
	Error   string          `json:"error,omitempty"`
}

type ASCCommandResponse struct {
	Data  string `json:"data"`
	Error string `json:"error,omitempty"`
}

type SubscriptionItem struct {
	ID                 string `json:"id"`
	GroupName          string `json:"groupName"`
	Name               string `json:"name"`
	ProductID          string `json:"productId"`
	State              string `json:"state"`
	SubscriptionPeriod string `json:"subscriptionPeriod"`
	ReviewNote         string `json:"reviewNote"`
	GroupLevel         int    `json:"groupLevel"`
}

type SubscriptionsResponse struct {
	Subscriptions []SubscriptionItem `json:"subscriptions"`
	Error         string             `json:"error,omitempty"`
}

type SubPricingItem struct {
	Name      string `json:"name"`
	ProductID string `json:"productId"`
	Period    string `json:"subscriptionPeriod"`
	State     string `json:"state"`
	GroupName string `json:"groupName"`
	Price     string `json:"price"`
	Currency  string `json:"currency"`
	Proceeds  string `json:"proceeds"`
}

type TerritoryAvailability struct {
	Territory   string `json:"territory"`
	Available   bool   `json:"available"`
	ReleaseDate string `json:"releaseDate"`
}

type PricingOverview struct {
	AvailableInNewTerritories bool                    `json:"availableInNewTerritories"`
	CurrentPrice              string                  `json:"currentPrice"`
	CurrentProceeds           string                  `json:"currentProceeds"`
	BaseCurrency              string                  `json:"baseCurrency"`
	Territories               []TerritoryAvailability `json:"territories"`
	SubscriptionPricing       []SubPricingItem        `json:"subscriptionPricing"`
	Error                     string                  `json:"error,omitempty"`
}

type AppLocalization struct {
	LocalizationID  string `json:"localizationId"`
	Locale          string `json:"locale"`
	Description     string `json:"description"`
	Keywords        string `json:"keywords"`
	WhatsNew        string `json:"whatsNew"`
	PromotionalText string `json:"promotionalText"`
	SupportURL      string `json:"supportUrl"`
	MarketingURL    string `json:"marketingUrl"`
}

type VersionMetadataResponse struct {
	Localizations []AppLocalization `json:"localizations"`
	Error         string            `json:"error,omitempty"`
}

type AppScreenshot struct {
	ThumbnailURL string `json:"thumbnailUrl"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
}

type appPriceReference struct {
	Territory  string
	PricePoint string
}

type appPricePointLookupResult struct {
	Price    string
	Proceeds string
	Currency string
}

type ScreenshotSet struct {
	DisplayType string          `json:"displayType"`
	Screenshots []AppScreenshot `json:"screenshots"`
}

type ScreenshotsResponse struct {
	Sets  []ScreenshotSet `json:"sets"`
	Error string          `json:"error,omitempty"`
}

type rawASCApp struct {
	Type       string `json:"type"`
	ID         string `json:"id"`
	Attributes struct {
		Name     string `json:"name"`
		BundleID string `json:"bundleId"`
		SKU      string `json:"sku"`
	} `json:"attributes"`
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
		sessionInits: make(map[string]chan struct{}),
		startAgent:   startACPClient,
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

func (a *App) CheckAuthStatus() (AuthStatus, error) {
	defer configGuard()()
	ascPath, err := a.resolveASCPath()
	if err != nil {
		return AuthStatus{RawOutput: "Could not find asc binary: " + err.Error()}, nil
	}

	ctx, cancel := context.WithTimeout(a.contextOrBackground(), 10*time.Second)
	defer cancel()

	cmd := a.newASCCommand(ctx, ascPath, "auth", "status", "--output", "json")
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

func (a *App) ListApps() (ListAppsResponse, error) {
	defer configGuard()()
	ascPath, err := a.resolveASCPath()
	if err != nil {
		return ListAppsResponse{Error: "Could not find asc binary: " + err.Error()}, nil
	}

	ctx, cancel := context.WithTimeout(a.contextOrBackground(), 30*time.Second)
	defer cancel()

	cmd := a.newASCCommand(ctx, ascPath, "apps", "list", "--output", "json")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ListAppsResponse{Error: strings.TrimSpace(string(out))}, nil
	}

	rawApps, err := parseAppsListOutput(out)
	if err != nil {
		return ListAppsResponse{Error: "Failed to parse apps list: " + err.Error()}, nil
	}

	apps := make([]AppInfo, len(rawApps))
	for i, raw := range rawApps {
		apps[i] = AppInfo{
			ID:       raw.ID,
			Name:     raw.Attributes.Name,
			BundleID: raw.Attributes.BundleID,
			SKU:      raw.Attributes.SKU,
		}
	}

	// Fetch subtitles concurrently (best-effort; failures are silently skipped)
	subtitleCtx, subtitleCancel := context.WithTimeout(a.contextOrBackground(), 20*time.Second)
	defer subtitleCancel()

	type subtitleResult struct {
		index    int
		subtitle string
	}
	results := make(chan subtitleResult, len(apps))
	for i, app := range apps {
		go func(idx int, appID string) {
			subtitle := a.fetchSubtitle(subtitleCtx, ascPath, appID)
			results <- subtitleResult{index: idx, subtitle: subtitle}
		}(i, app.ID)
	}
	for range apps {
		r := <-results
		apps[r.index].Subtitle = r.subtitle
	}

	return ListAppsResponse{Apps: apps}, nil
}

func (a *App) fetchSubtitle(ctx context.Context, ascPath, appID string) string {
	cmd := a.newASCCommand(ctx, ascPath, "localizations", "list",
		"--app", appID, "--type", "app-info", "--locale", "en-US", "--output", "json")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}

	type locAttrs struct {
		Subtitle string `json:"subtitle"`
	}
	type locItem struct {
		Attributes locAttrs `json:"attributes"`
	}
	var envelope struct {
		Data []locItem `json:"data"`
	}
	if json.Unmarshal(out, &envelope) != nil || len(envelope.Data) == 0 {
		return ""
	}
	return envelope.Data[0].Attributes.Subtitle
}

// GetTestFlight fetches beta groups and tester counts concurrently.
func (a *App) GetTestFlight(appID string) (TestFlightResponse, error) {
	if strings.TrimSpace(appID) == "" {
		return TestFlightResponse{Error: "app ID is required"}, nil
	}
	defer configGuard()()
	ascPath, err := a.resolveASCPath()
	if err != nil {
		return TestFlightResponse{Error: err.Error()}, nil
	}
	ctx, cancel := context.WithTimeout(a.contextOrBackground(), 30*time.Second)
	defer cancel()

	// 1. Fetch groups
	cmd := a.newASCCommand(ctx, ascPath, "testflight", "groups", "list", "--app", appID, "--output", "json")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return TestFlightResponse{Error: strings.TrimSpace(string(out))}, nil
	}

	type rawGroup struct {
		ID         string `json:"id"`
		Attributes struct {
			Name            string `json:"name"`
			IsInternalGroup bool   `json:"isInternalGroup"`
			PublicLink      string `json:"publicLink"`
			FeedbackEnabled bool   `json:"feedbackEnabled"`
			CreatedDate     string `json:"createdDate"`
		} `json:"attributes"`
		Relationships struct {
			BetaTesters struct {
				Links struct {
					Related string `json:"related"`
				} `json:"links"`
			} `json:"betaTesters"`
		} `json:"relationships"`
	}
	var groupEnv struct {
		Data []rawGroup `json:"data"`
	}
	if json.Unmarshal(out, &groupEnv) != nil {
		return TestFlightResponse{Error: "failed to parse groups"}, nil
	}

	// 2. Fetch tester count per group concurrently (just need meta.paging.total)
	type countResult struct {
		idx   int
		count int
	}
	countCh := make(chan countResult, len(groupEnv.Data))
	for i, g := range groupEnv.Data {
		go func(idx int, groupID string) {
			// Workaround for CLI bug #1292: use relationship URL via --next
			relationshipURL := fmt.Sprintf("https://api.appstoreconnect.apple.com/v1/betaGroups/%s/betaTesters?limit=1", groupID)
			cmd := a.newASCCommand(ctx, ascPath, "testflight", "testers", "list",
				"--next", relationshipURL, "--output", "json")
			out, err := cmd.CombinedOutput()
			if err != nil {
				countCh <- countResult{idx: idx, count: 0}
				return
			}
			var env struct {
				Meta struct {
					Paging struct {
						Total int `json:"total"`
					} `json:"paging"`
				} `json:"meta"`
			}
			if json.Unmarshal(out, &env) == nil {
				countCh <- countResult{idx: idx, count: env.Meta.Paging.Total}
			} else {
				countCh <- countResult{idx: idx, count: 0}
			}
		}(i, g.ID)
	}

	groups := make([]BetaGroup, len(groupEnv.Data))
	for i, g := range groupEnv.Data {
		groups[i] = BetaGroup{
			ID:              g.ID,
			Name:            g.Attributes.Name,
			IsInternal:      g.Attributes.IsInternalGroup,
			PublicLink:      g.Attributes.PublicLink,
			FeedbackEnabled: g.Attributes.FeedbackEnabled,
			CreatedDate:     g.Attributes.CreatedDate,
		}
	}

	for range groupEnv.Data {
		r := <-countCh
		groups[r.idx].TesterCount = r.count
	}

	return TestFlightResponse{Groups: groups}, nil
}

// GetTestFlightTesters fetches ALL testers for a specific group (paginated).
func (a *App) GetTestFlightTesters(groupID string) (TestFlightResponse, error) {
	if strings.TrimSpace(groupID) == "" {
		return TestFlightResponse{Error: "group ID is required"}, nil
	}
	defer configGuard()()
	ascPath, err := a.resolveASCPath()
	if err != nil {
		return TestFlightResponse{Error: err.Error()}, nil
	}
	ctx, cancel := context.WithTimeout(a.contextOrBackground(), 120*time.Second)
	defer cancel()

	// Workaround for CLI bug #1292: --group filter fails with "Only one relationship filter".
	// Use --next with the betaTesters relationship URL and --paginate to fetch ALL testers.
	relationshipURL := fmt.Sprintf("https://api.appstoreconnect.apple.com/v1/betaGroups/%s/betaTesters?limit=200", groupID)
	cmd := a.newASCCommand(ctx, ascPath, "testflight", "testers", "list",
		"--next", relationshipURL, "--paginate", "--output", "json")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return TestFlightResponse{Error: strings.TrimSpace(string(out))}, nil
	}

	type rawTester struct {
		Attributes struct {
			Email      string `json:"email"`
			FirstName  string `json:"firstName"`
			LastName   string `json:"lastName"`
			InviteType string `json:"inviteType"`
			State      string `json:"state"`
		} `json:"attributes"`
	}
	var env struct {
		Data []rawTester `json:"data"`
	}
	if json.Unmarshal(out, &env) != nil {
		return TestFlightResponse{Error: "failed to parse testers"}, nil
	}

	testers := make([]BetaTester, 0, len(env.Data))
	for _, t := range env.Data {
		testers = append(testers, BetaTester{
			Email:      t.Attributes.Email,
			FirstName:  t.Attributes.FirstName,
			LastName:   t.Attributes.LastName,
			InviteType: t.Attributes.InviteType,
			State:      t.Attributes.State,
		})
	}
	return TestFlightResponse{Testers: testers}, nil
}

// GetSubscriptions fetches subscription groups, then subscriptions for each group concurrently.
func (a *App) GetSubscriptions(appID string) (SubscriptionsResponse, error) {
	defer configGuard()()
	if strings.TrimSpace(appID) == "" {
		return SubscriptionsResponse{Error: "app ID is required"}, nil
	}
	ascPath, err := a.resolveASCPath()
	if err != nil {
		return SubscriptionsResponse{Error: err.Error()}, nil
	}
	ctx, cancel := context.WithTimeout(a.contextOrBackground(), 30*time.Second)
	defer cancel()

	// Step 1: get groups
	cmd := a.newASCCommand(ctx, ascPath, "subscriptions", "groups", "list", "--app", appID, "--output", "json")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return SubscriptionsResponse{Error: strings.TrimSpace(string(out))}, nil
	}
	type groupItem struct {
		ID         string `json:"id"`
		Attributes struct {
			ReferenceName string `json:"referenceName"`
		} `json:"attributes"`
	}
	var groupEnv struct {
		Data []groupItem `json:"data"`
	}
	if json.Unmarshal(out, &groupEnv) != nil {
		return SubscriptionsResponse{Error: "failed to parse groups"}, nil
	}

	// Step 2: fetch subscriptions per group concurrently
	type subResult struct {
		groupName string
		subs      []SubscriptionItem
	}
	ch := make(chan subResult, len(groupEnv.Data))
	for _, g := range groupEnv.Data {
		go func(groupID, groupName string) {
			cmd := a.newASCCommand(ctx, ascPath, "subscriptions", "list", "--group-id", groupID, "--output", "json")
			out, err := cmd.CombinedOutput()
			if err != nil {
				ch <- subResult{groupName: groupName}
				return
			}
			type rawSub struct {
				ID         string `json:"id"`
				Attributes struct {
					Name               string `json:"name"`
					ProductID          string `json:"productId"`
					State              string `json:"state"`
					SubscriptionPeriod string `json:"subscriptionPeriod"`
					ReviewNote         string `json:"reviewNote"`
					GroupLevel         int    `json:"groupLevel"`
				} `json:"attributes"`
			}
			var env struct {
				Data []rawSub `json:"data"`
			}
			if json.Unmarshal(out, &env) != nil {
				ch <- subResult{groupName: groupName}
				return
			}
			items := make([]SubscriptionItem, 0, len(env.Data))
			for _, s := range env.Data {
				items = append(items, SubscriptionItem{
					ID:                 s.ID,
					GroupName:          groupName,
					Name:               s.Attributes.Name,
					ProductID:          s.Attributes.ProductID,
					State:              s.Attributes.State,
					SubscriptionPeriod: s.Attributes.SubscriptionPeriod,
					ReviewNote:         s.Attributes.ReviewNote,
					GroupLevel:         s.Attributes.GroupLevel,
				})
			}
			ch <- subResult{groupName: groupName, subs: items}
		}(g.ID, g.Attributes.ReferenceName)
	}

	var all []SubscriptionItem
	for range groupEnv.Data {
		r := <-ch
		all = append(all, r.subs...)
	}
	return SubscriptionsResponse{Subscriptions: all}, nil
}

// GetPricingOverview fetches availability + subscription pricing summary in parallel.
func (a *App) GetPricingOverview(appID string) (PricingOverview, error) {
	if strings.TrimSpace(appID) == "" {
		return PricingOverview{Error: "app ID is required"}, nil
	}
	defer configGuard()()
	ascPath, err := a.resolveASCPath()
	if err != nil {
		return PricingOverview{Error: err.Error()}, nil
	}
	ctx, cancel := context.WithTimeout(a.contextOrBackground(), 30*time.Second)
	defer cancel()

	type availResult struct {
		available   bool
		territories []TerritoryAvailability
		err         error
	}
	type pricingResult struct {
		items []SubPricingItem
	}
	type priceResult struct {
		price    string
		proceeds string
		currency string
	}
	availCh := make(chan availResult, 1)
	pricingCh := make(chan pricingResult, 1)
	priceCh := make(chan priceResult, 1)

	// Current app price (first manual price → decode base64 ID to get price point, then look up tier)
	go func() {
		scheduleID, err := a.fetchPricingScheduleID(ctx, ascPath, appID)
		if err != nil || scheduleID == "" {
			priceCh <- priceResult{}
			return
		}

		price, err := a.fetchCurrentAppPrice(ctx, ascPath, appID, scheduleID)
		if err != nil {
			priceCh <- priceResult{}
			return
		}
		priceCh <- priceResult{
			price:    price.Price,
			proceeds: price.Proceeds,
			currency: price.Currency,
		}
	}()

	// Availability + territories (sequential: need avail ID first, but it's the app ID)
	go func() {
		// 1. Get availability flag and resource ID
		cmd := a.newASCCommand(ctx, ascPath, "pricing", "availability", "view", "--app", appID, "--output", "json")
		out, err := cmd.CombinedOutput()
		if err != nil {
			availCh <- availResult{err: fmt.Errorf("%s", strings.TrimSpace(string(out)))}
			return
		}
		availabilityID, availableInNewTerritories, err := parseAvailabilityViewOutput(out)
		if err != nil {
			availCh <- availResult{err: fmt.Errorf("failed to parse availability: %w", err)}
			return
		}

		// 2. Get territory availabilities
		var territories []TerritoryAvailability
		if availabilityID != "" {
			cmd2 := a.newASCCommand(ctx, ascPath, "pricing", "availability", "territory-availabilities",
				"--availability", availabilityID, "--paginate", "--output", "json")
			out2, err := cmd2.CombinedOutput()
			if err == nil {
				type rawTerrItem struct {
					Attributes struct {
						Available   bool   `json:"available"`
						ReleaseDate string `json:"releaseDate"`
					} `json:"attributes"`
					Relationships struct {
						Territory struct {
							Data struct {
								ID string `json:"id"`
							} `json:"data"`
						} `json:"territory"`
					} `json:"relationships"`
				}
				var terrEnv struct {
					Data []rawTerrItem `json:"data"`
				}
				if json.Unmarshal(out2, &terrEnv) == nil {
					for _, t := range terrEnv.Data {
						territories = append(territories, TerritoryAvailability{
							Territory:   t.Relationships.Territory.Data.ID,
							Available:   t.Attributes.Available,
							ReleaseDate: t.Attributes.ReleaseDate,
						})
					}
				}
			}
		}

		availCh <- availResult{
			available:   availableInNewTerritories,
			territories: territories,
		}
	}()

	// Subscription pricing summary
	go func() {
		cmd := a.newASCCommand(ctx, ascPath, "subscriptions", "pricing", "summary", "--app", appID, "--output", "json")
		out, err := cmd.CombinedOutput()
		if err != nil {
			pricingCh <- pricingResult{} // not an error — app may have no subscriptions
			return
		}
		type rawSub struct {
			Name         string `json:"name"`
			ProductID    string `json:"productId"`
			Period       string `json:"subscriptionPeriod"`
			State        string `json:"state"`
			GroupName    string `json:"groupName"`
			CurrentPrice struct {
				Amount   string `json:"amount"`
				Currency string `json:"currency"`
			} `json:"currentPrice"`
			Proceeds struct {
				Amount string `json:"amount"`
			} `json:"proceeds"`
		}
		var env struct {
			Subscriptions []rawSub `json:"subscriptions"`
		}
		if json.Unmarshal(out, &env) != nil {
			pricingCh <- pricingResult{}
			return
		}
		items := make([]SubPricingItem, 0, len(env.Subscriptions))
		for _, s := range env.Subscriptions {
			items = append(items, SubPricingItem{
				Name:      s.Name,
				ProductID: s.ProductID,
				Period:    s.Period,
				State:     s.State,
				GroupName: s.GroupName,
				Price:     s.CurrentPrice.Amount,
				Currency:  s.CurrentPrice.Currency,
				Proceeds:  s.Proceeds.Amount,
			})
		}
		pricingCh <- pricingResult{items: items}
	}()

	avail := <-availCh
	pricing := <-pricingCh
	price := <-priceCh

	if avail.err != nil {
		return PricingOverview{Error: avail.err.Error()}, nil
	}

	return PricingOverview{
		AvailableInNewTerritories: avail.available,
		CurrentPrice:              price.price,
		CurrentProceeds:           price.proceeds,
		BaseCurrency:              price.currency,
		Territories:               avail.territories,
		SubscriptionPricing:       pricing.items,
	}, nil
}

// GetFinanceRegions fetches finance report region codes.
func (a *App) GetFinanceRegions() (FinanceResponse, error) {
	defer configGuard()()
	ascPath, err := a.resolveASCPath()
	if err != nil {
		return FinanceResponse{Error: err.Error()}, nil
	}
	ctx, cancel := context.WithTimeout(a.contextOrBackground(), 20*time.Second)
	defer cancel()

	cmd := a.newASCCommand(ctx, ascPath, "finance", "regions", "--output", "json")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return FinanceResponse{Error: strings.TrimSpace(string(out))}, nil
	}
	var env struct {
		Regions []FinanceRegion `json:"regions"`
	}
	if json.Unmarshal(out, &env) != nil {
		return FinanceResponse{Error: "failed to parse finance regions"}, nil
	}
	return FinanceResponse{Regions: env.Regions}, nil
}

// GetOfferCodes fetches offer codes for all subscriptions of an app concurrently.
func (a *App) GetOfferCodes(appID string) (OfferCodesResponse, error) {
	if strings.TrimSpace(appID) == "" {
		return OfferCodesResponse{Error: "app ID is required"}, nil
	}
	defer configGuard()()

	// First get subscriptions to know which sub IDs to query
	subsResp, err := a.GetSubscriptions(appID)
	if err != nil {
		return OfferCodesResponse{Error: err.Error()}, nil
	}
	if subsResp.Error != "" {
		return OfferCodesResponse{Error: subsResp.Error}, nil
	}

	ascPath, err := a.resolveASCPath()
	if err != nil {
		return OfferCodesResponse{Error: err.Error()}, nil
	}
	ctx, cancel := context.WithTimeout(a.contextOrBackground(), 30*time.Second)
	defer cancel()

	type offerResult struct {
		codes []OfferCode
	}
	ch := make(chan offerResult, len(subsResp.Subscriptions))

	for _, sub := range subsResp.Subscriptions {
		go func(subID, subName string) {
			cmd := a.newASCCommand(ctx, ascPath, "subscriptions", "offers", "offer-codes", "list",
				"--subscription-id", subID, "--output", "json")
			out, err := cmd.CombinedOutput()
			if err != nil {
				ch <- offerResult{}
				return
			}
			type rawCode struct {
				Attributes struct {
					Name                  string   `json:"name"`
					OfferEligibility      string   `json:"offerEligibility"`
					CustomerEligibilities []string `json:"customerEligibilities"`
					Duration              string   `json:"duration"`
					OfferMode             string   `json:"offerMode"`
					NumberOfPeriods       int      `json:"numberOfPeriods"`
					TotalNumberOfCodes    int      `json:"totalNumberOfCodes"`
					ProductionCodeCount   int      `json:"productionCodeCount"`
				} `json:"attributes"`
			}
			var env struct {
				Data []rawCode `json:"data"`
			}
			if json.Unmarshal(out, &env) != nil {
				ch <- offerResult{}
				return
			}
			codes := make([]OfferCode, 0, len(env.Data))
			for _, c := range env.Data {
				codes = append(codes, OfferCode{
					SubscriptionName: subName,
					SubscriptionID:   subID,
					Name:             c.Attributes.Name,
					Eligibility:      c.Attributes.OfferEligibility,
					Customers:        c.Attributes.CustomerEligibilities,
					Duration:         c.Attributes.Duration,
					OfferMode:        c.Attributes.OfferMode,
					Periods:          c.Attributes.NumberOfPeriods,
					TotalCodes:       c.Attributes.TotalNumberOfCodes,
					ProductionCodes:  c.Attributes.ProductionCodeCount,
				})
			}
			ch <- offerResult{codes: codes}
		}(sub.ID, sub.Name)
	}

	var all []OfferCode
	for range subsResp.Subscriptions {
		r := <-ch
		all = append(all, r.codes...)
	}
	return OfferCodesResponse{OfferCodes: all}, nil
}

// RunASCCommand runs an arbitrary asc CLI command and returns the raw output.
// Args is a shell-style command string, e.g. `reviews list --app "123" --limit 10 --output json`.
func (a *App) RunASCCommand(args string) (ASCCommandResponse, error) {
	defer configGuard()()
	if strings.TrimSpace(args) == "" {
		return ASCCommandResponse{Error: "args required"}, nil
	}

	ascPath, err := a.resolveASCPath()
	if err != nil {
		return ASCCommandResponse{Error: "Could not find asc binary: " + err.Error()}, nil
	}

	ctx, cancel := context.WithTimeout(a.contextOrBackground(), 30*time.Second)
	defer cancel()

	parts, err := parseASCCommandArgs(args)
	if err != nil {
		return ASCCommandResponse{Error: "Invalid command arguments: " + err.Error()}, nil
	}
	cmd := a.newASCCommand(ctx, ascPath, parts...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ASCCommandResponse{Error: strings.TrimSpace(string(out))}, nil
	}
	return ASCCommandResponse{Data: string(out)}, nil
}

func (a *App) GetAppDetail(appID string) (AppDetail, error) {
	defer configGuard()()
	if strings.TrimSpace(appID) == "" {
		return AppDetail{Error: "app ID is required"}, nil
	}

	ascPath, err := a.resolveASCPath()
	if err != nil {
		return AppDetail{Error: "Could not find asc binary: " + err.Error()}, nil
	}

	ctx, cancel := context.WithTimeout(a.contextOrBackground(), 30*time.Second)
	defer cancel()

	// Fetch app attrs and versions concurrently
	type attrsResult struct {
		name          string
		bundleID      string
		sku           string
		primaryLocale string
		err           error
	}
	type versionsResult struct {
		versions []AppVersion
		err      error
	}
	type subtitleRes struct {
		subtitle string
	}

	attrsCh := make(chan attrsResult, 1)
	versionsCh := make(chan versionsResult, 1)
	subtitleCh := make(chan subtitleRes, 1)

	go func() {
		cmd := a.newASCCommand(ctx, ascPath, "apps", "view", "--id", appID, "--output", "json")
		out, err := cmd.CombinedOutput()
		if err != nil {
			attrsCh <- attrsResult{err: err}
			return
		}
		var env struct {
			Data struct {
				Attributes struct {
					Name          string `json:"name"`
					BundleID      string `json:"bundleId"`
					SKU           string `json:"sku"`
					PrimaryLocale string `json:"primaryLocale"`
				} `json:"attributes"`
			} `json:"data"`
		}
		if json.Unmarshal(out, &env) != nil {
			attrsCh <- attrsResult{err: errors.New("failed to parse app view")}
			return
		}
		a := env.Data.Attributes
		attrsCh <- attrsResult{name: a.Name, bundleID: a.BundleID, sku: a.SKU, primaryLocale: a.PrimaryLocale}
	}()

	go func() {
		cmd := a.newASCCommand(ctx, ascPath, "versions", "list", "--app", appID, "--output", "json")
		out, err := cmd.CombinedOutput()
		if err != nil {
			versionsCh <- versionsResult{err: err}
			return
		}
		type rawVersion struct {
			ID         string `json:"id"`
			Attributes struct {
				Platform        string `json:"platform"`
				VersionString   string `json:"versionString"`
				AppVersionState string `json:"appVersionState"`
				AppStoreState   string `json:"appStoreState"`
			} `json:"attributes"`
		}
		var env struct {
			Data []rawVersion `json:"data"`
		}
		if json.Unmarshal(out, &env) != nil {
			versionsCh <- versionsResult{}
			return
		}
		vs := make([]AppVersion, 0, len(env.Data))
		for _, rv := range env.Data {
			state := rv.Attributes.AppVersionState
			if state == "" {
				state = rv.Attributes.AppStoreState
			}
			vs = append(vs, AppVersion{
				ID:       rv.ID,
				Platform: rv.Attributes.Platform,
				Version:  rv.Attributes.VersionString,
				State:    state,
			})
		}
		versionsCh <- versionsResult{versions: vs}
	}()

	go func() {
		subtitleCh <- subtitleRes{subtitle: a.fetchSubtitle(ctx, ascPath, appID)}
	}()

	attrs := <-attrsCh
	vers := <-versionsCh
	sub := <-subtitleCh

	if attrs.err != nil {
		return AppDetail{Error: attrs.err.Error()}, nil
	}

	return AppDetail{
		ID:            appID,
		Name:          attrs.name,
		Subtitle:      sub.subtitle,
		BundleID:      attrs.bundleID,
		SKU:           attrs.sku,
		PrimaryLocale: attrs.primaryLocale,
		Versions:      vers.versions,
	}, nil
}

// GetVersionMetadata returns all localizations for a given App Store version.
// Pass versionID from AppVersion.ID. Returns all locales so the frontend can
// render a picker without an extra round-trip.
func (a *App) GetVersionMetadata(versionID string) (VersionMetadataResponse, error) {
	defer configGuard()()
	if strings.TrimSpace(versionID) == "" {
		return VersionMetadataResponse{Error: "version ID is required"}, nil
	}

	ascPath, err := a.resolveASCPath()
	if err != nil {
		return VersionMetadataResponse{Error: "Could not find asc binary: " + err.Error()}, nil
	}

	ctx, cancel := context.WithTimeout(a.contextOrBackground(), 20*time.Second)
	defer cancel()

	cmd := a.newASCCommand(ctx, ascPath, "localizations", "list",
		"--version", versionID, "--output", "json")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return VersionMetadataResponse{Error: strings.TrimSpace(string(out))}, nil
	}

	type rawAttrs struct {
		Locale          string `json:"locale"`
		Description     string `json:"description"`
		Keywords        string `json:"keywords"`
		WhatsNew        string `json:"whatsNew"`
		PromotionalText string `json:"promotionalText"`
		SupportURL      string `json:"supportUrl"`
		MarketingURL    string `json:"marketingUrl"`
	}
	type rawItem struct {
		ID         string   `json:"id"`
		Attributes rawAttrs `json:"attributes"`
	}
	var envelope struct {
		Data []rawItem `json:"data"`
	}
	if json.Unmarshal(out, &envelope) != nil {
		return VersionMetadataResponse{Error: "failed to parse localizations"}, nil
	}

	locs := make([]AppLocalization, 0, len(envelope.Data))
	for _, item := range envelope.Data {
		a := item.Attributes
		locs = append(locs, AppLocalization{
			LocalizationID:  item.ID,
			Locale:          a.Locale,
			Description:     a.Description,
			Keywords:        a.Keywords,
			WhatsNew:        a.WhatsNew,
			PromotionalText: a.PromotionalText,
			SupportURL:      a.SupportURL,
			MarketingURL:    a.MarketingURL,
		})
	}
	return VersionMetadataResponse{Localizations: locs}, nil
}

// GetScreenshots returns screenshot sets for a version localization.
// Pass LocalizationID from AppLocalization.
func (a *App) GetScreenshots(localizationID string) (ScreenshotsResponse, error) {
	defer configGuard()()
	if strings.TrimSpace(localizationID) == "" {
		return ScreenshotsResponse{Error: "localization ID is required"}, nil
	}

	ascPath, err := a.resolveASCPath()
	if err != nil {
		return ScreenshotsResponse{Error: "Could not find asc binary: " + err.Error()}, nil
	}

	ctx, cancel := context.WithTimeout(a.contextOrBackground(), 20*time.Second)
	defer cancel()

	cmd := a.newASCCommand(ctx, ascPath, "screenshots", "list",
		"--version-localization", localizationID, "--output", "json")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ScreenshotsResponse{Error: strings.TrimSpace(string(out))}, nil
	}

	type rawImageAsset struct {
		TemplateURL string `json:"templateUrl"`
		Width       int    `json:"width"`
		Height      int    `json:"height"`
	}
	type rawScreenshot struct {
		Attributes struct {
			ImageAsset rawImageAsset `json:"imageAsset"`
		} `json:"attributes"`
	}
	type rawSet struct {
		Set struct {
			Attributes struct {
				DisplayType string `json:"screenshotDisplayType"`
			} `json:"attributes"`
		} `json:"set"`
		Screenshots []rawScreenshot `json:"screenshots"`
	}
	var result struct {
		Sets []rawSet `json:"sets"`
	}
	if json.Unmarshal(out, &result) != nil {
		return ScreenshotsResponse{Error: "failed to parse screenshots"}, nil
	}

	sets := make([]ScreenshotSet, 0, len(result.Sets))
	for _, rs := range result.Sets {
		if len(rs.Screenshots) == 0 {
			continue
		}
		shots := make([]AppScreenshot, 0, len(rs.Screenshots))
		for _, s := range rs.Screenshots {
			ia := s.Attributes.ImageAsset
			if ia.TemplateURL == "" {
				continue
			}
			// Build a ~400px-wide thumbnail URL from the template.
			thumbW := 400
			thumbH := thumbW
			if ia.Width > 0 && ia.Height > 0 {
				thumbH = thumbW * ia.Height / ia.Width
			}
			thumbURL := strings.NewReplacer(
				"{w}", fmt.Sprintf("%d", thumbW),
				"{h}", fmt.Sprintf("%d", thumbH),
				"{f}", "webp",
			).Replace(ia.TemplateURL)
			shots = append(shots, AppScreenshot{
				ThumbnailURL: thumbURL,
				Width:        ia.Width,
				Height:       ia.Height,
			})
		}
		if len(shots) > 0 {
			sets = append(sets, ScreenshotSet{
				DisplayType: rs.Set.Attributes.DisplayType,
				Screenshots: shots,
			})
		}
	}
	return ScreenshotsResponse{Sets: sets}, nil
}

// configGuard saves a snapshot of ~/.asc/config.json before running an asc
// command and restores it afterwards if the command mutated the file.
// This defends against CLI auth codepaths that accidentally wipe credentials
// during read-only operations (a known issue in the auth resolver).
func configGuard() func() {
	home, err := os.UserHomeDir()
	if err != nil {
		return func() {}
	}
	path := filepath.Join(home, ".asc", "config.json")

	ascConfigGuardMu.Lock()
	original, err := os.ReadFile(path)
	if err != nil {
		ascConfigGuardMu.Unlock()
		return func() {}
	}
	return func() {
		defer ascConfigGuardMu.Unlock()
		current, err := os.ReadFile(path)
		if err != nil || bytes.Equal(current, original) {
			return
		}
		_ = os.WriteFile(path, original, 0o600)
	}
}

func base64Decode(s string) ([]byte, error) {
	// ASC API uses URL-safe base64 without padding
	return base64.RawURLEncoding.DecodeString(s)
}

func parseAppsListOutput(out []byte) ([]rawASCApp, error) {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(out, &envelope); err == nil && envelope.Data != nil {
		var rawApps []rawASCApp
		if err := json.Unmarshal(envelope.Data, &rawApps); err != nil {
			return nil, err
		}
		return rawApps, nil
	}

	var rawApps []rawASCApp
	if err := json.Unmarshal(out, &rawApps); err != nil {
		return nil, err
	}
	return rawApps, nil
}

func parseResourceIDOutput(out []byte) (string, error) {
	var envelope struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &envelope); err != nil {
		return "", err
	}
	return strings.TrimSpace(envelope.Data.ID), nil
}

func parseAvailabilityViewOutput(out []byte) (string, bool, error) {
	var envelope struct {
		Data struct {
			ID         string `json:"id"`
			Attributes struct {
				AvailableInNewTerritories bool `json:"availableInNewTerritories"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &envelope); err != nil {
		return "", false, err
	}
	return strings.TrimSpace(envelope.Data.ID), envelope.Data.Attributes.AvailableInNewTerritories, nil
}

func parseFirstAppPriceReference(out []byte) (appPriceReference, bool, error) {
	type rawPrice struct {
		ID string `json:"id"`
	}
	var env struct {
		Data []rawPrice `json:"data"`
	}
	if err := json.Unmarshal(out, &env); err != nil {
		return appPriceReference{}, false, err
	}
	if len(env.Data) == 0 {
		return appPriceReference{}, false, nil
	}

	decoded, err := base64Decode(env.Data[0].ID)
	if err != nil {
		return appPriceReference{}, false, err
	}

	var ref struct {
		Territory  string `json:"t"`
		PricePoint string `json:"p"`
	}
	if err := json.Unmarshal(decoded, &ref); err != nil {
		return appPriceReference{}, false, err
	}
	if strings.TrimSpace(ref.Territory) == "" || strings.TrimSpace(ref.PricePoint) == "" {
		return appPriceReference{}, false, errors.New("missing territory or price point")
	}

	return appPriceReference{
		Territory:  strings.TrimSpace(ref.Territory),
		PricePoint: strings.TrimSpace(ref.PricePoint),
	}, true, nil
}

func parseAppPricePointLookup(out []byte, territoryID, wantedPricePoint string) (appPricePointLookupResult, bool, error) {
	type rawPricePoint struct {
		ID         string `json:"id"`
		Attributes struct {
			CustomerPrice string `json:"customerPrice"`
			Proceeds      string `json:"proceeds"`
		} `json:"attributes"`
	}
	type rawIncluded struct {
		Type       string `json:"type"`
		ID         string `json:"id"`
		Attributes struct {
			Currency string `json:"currency"`
		} `json:"attributes"`
	}

	var env struct {
		Data     []rawPricePoint `json:"data"`
		Included []rawIncluded   `json:"included"`
	}
	if err := json.Unmarshal(out, &env); err != nil {
		return appPricePointLookupResult{}, false, err
	}

	currencyByTerritory := make(map[string]string, len(env.Included))
	for _, included := range env.Included {
		if included.Type != "territories" {
			continue
		}
		currencyByTerritory[included.ID] = strings.TrimSpace(included.Attributes.Currency)
	}

	for _, point := range env.Data {
		decoded, err := base64Decode(point.ID)
		if err != nil {
			continue
		}
		var ref struct {
			PricePoint string `json:"p"`
		}
		if err := json.Unmarshal(decoded, &ref); err != nil {
			continue
		}
		if strings.TrimSpace(ref.PricePoint) != wantedPricePoint {
			continue
		}
		return appPricePointLookupResult{
			Price:    point.Attributes.CustomerPrice,
			Proceeds: point.Attributes.Proceeds,
			Currency: currencyByTerritory[territoryID],
		}, true, nil
	}

	return appPricePointLookupResult{}, false, nil
}

func parseASCCommandArgs(args string) ([]string, error) {
	return shellquote.Split(strings.TrimSpace(args))
}

func (a *App) newASCCommand(ctx context.Context, ascPath string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, ascPath, args...)
}

func (a *App) fetchPricingScheduleID(ctx context.Context, ascPath, appID string) (string, error) {
	cmd := a.newASCCommand(ctx, ascPath, "pricing", "schedule", "view", "--app", appID, "--output", "json")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return parseResourceIDOutput(out)
}

func (a *App) fetchSchedulePriceReference(ctx context.Context, ascPath, scheduleID, priceMode string) (appPriceReference, bool, error) {
	cmd := a.newASCCommand(ctx, ascPath, "pricing", "schedule", priceMode, "--schedule", scheduleID, "--output", "json")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return appPriceReference{}, false, err
	}
	return parseFirstAppPriceReference(out)
}

func (a *App) fetchCurrentAppPrice(ctx context.Context, ascPath, appID, scheduleID string) (appPricePointLookupResult, error) {
	for _, priceMode := range []string{"manual-prices", "automatic-prices"} {
		ref, found, err := a.fetchSchedulePriceReference(ctx, ascPath, scheduleID, priceMode)
		if err != nil {
			continue
		}
		if !found {
			continue
		}

		cmd := a.newASCCommand(ctx, ascPath, "pricing", "price-points", "--app", appID, "--territory", ref.Territory, "--output", "json")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return appPricePointLookupResult{}, err
		}

		price, matched, err := parseAppPricePointLookup(out, ref.Territory, ref.PricePoint)
		if err != nil {
			return appPricePointLookupResult{}, err
		}
		if matched {
			return price, nil
		}
	}

	return appPricePointLookupResult{}, nil
}

func (a *App) bundledASCPath() string {
	candidates := bundledASCCandidates()
	for _, candidate := range candidates {
		if fileExists(candidate) {
			return candidate
		}
	}
	if len(candidates) > 0 {
		return candidates[0]
	}
	return ""
}

func (a *App) resolveASCPath() (string, error) {
	cfg, err := a.settings.Load()
	if err != nil {
		return "", err
	}

	bundled := a.bundledASCPath()
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

func (a *App) ListThreads() ([]StudioThread, error) {
	all, err := a.threads.LoadAll()
	if err != nil {
		return nil, err
	}
	return toStudioThreads(all), nil
}

func (a *App) CreateThread(title string) (StudioThread, error) {
	thread, err := a.createThreadRecord(title)
	if err != nil {
		return StudioThread{}, err
	}
	return toStudioThread(thread), nil
}

func (a *App) createThreadRecord(title string) (threads.Thread, error) {
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

	bundled := a.bundledASCPath()
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

func (a *App) QueueMutation(req ApprovalRequest) (StudioApproval, error) {
	if strings.TrimSpace(req.ThreadID) == "" {
		return StudioApproval{}, errors.New("thread ID is required")
	}
	if strings.TrimSpace(req.Title) == "" {
		return StudioApproval{}, errors.New("title is required")
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
	return toStudioApproval(a.approvals.Enqueue(action)), nil
}

func (a *App) ListApprovals() []StudioApproval {
	return toStudioApprovals(a.approvals.Pending())
}

func (a *App) ApproveAction(id string) (StudioApproval, error) {
	action, err := a.approvals.Approve(id)
	if err != nil {
		return StudioApproval{}, err
	}
	return toStudioApproval(action), nil
}

func (a *App) RejectAction(id string) (StudioApproval, error) {
	action, err := a.approvals.Reject(id)
	if err != nil {
		return StudioApproval{}, err
	}
	return toStudioApproval(action), nil
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

	return PromptResponse{Thread: toStudioThread(thread)}, nil
}

func (a *App) ensureThread(id string) (threads.Thread, error) {
	if strings.TrimSpace(id) == "" {
		return a.createThreadRecord("New Studio Thread")
	}
	return a.threads.Get(id)
}

func (a *App) ensureSession(thread threads.Thread) (*threadSession, error) {
	for {
		a.mu.Lock()
		if existing := a.sessions[thread.ID]; existing != nil {
			a.mu.Unlock()
			return existing, nil
		}
		if waitCh, ok := a.sessionInits[thread.ID]; ok {
			a.mu.Unlock()
			<-waitCh
			continue
		}

		waitCh := make(chan struct{})
		a.sessionInits[thread.ID] = waitCh
		a.mu.Unlock()

		session, err := a.startThreadSession(thread)

		a.mu.Lock()
		delete(a.sessionInits, thread.ID)
		if err == nil {
			a.sessions[thread.ID] = session
		}
		close(waitCh)
		a.mu.Unlock()

		if err != nil {
			return nil, err
		}
		return session, nil
	}
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

func formatTimestamp(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return ts.UTC().Format(time.RFC3339)
}

func toStudioThreads(items []threads.Thread) []StudioThread {
	out := make([]StudioThread, 0, len(items))
	for _, item := range items {
		out = append(out, toStudioThread(item))
	}
	return out
}

func toStudioThread(item threads.Thread) StudioThread {
	messages := make([]StudioMessage, 0, len(item.Messages))
	for _, message := range item.Messages {
		messages = append(messages, StudioMessage{
			ID:        message.ID,
			Role:      message.Role,
			Kind:      message.Kind,
			Content:   message.Content,
			CreatedAt: formatTimestamp(message.CreatedAt),
		})
	}

	return StudioThread{
		ID:        item.ID,
		Title:     item.Title,
		SessionID: item.SessionID,
		CreatedAt: formatTimestamp(item.CreatedAt),
		UpdatedAt: formatTimestamp(item.UpdatedAt),
		Messages:  messages,
	}
}

func toStudioApprovals(items []approvals.Action) []StudioApproval {
	out := make([]StudioApproval, 0, len(items))
	for _, item := range items {
		out = append(out, toStudioApproval(item))
	}
	return out
}

func toStudioApproval(item approvals.Action) StudioApproval {
	return StudioApproval{
		ID:              item.ID,
		ThreadID:        item.ThreadID,
		Title:           item.Title,
		Summary:         item.Summary,
		CommandPreview:  item.CommandPreview,
		MutationSurface: item.MutationSurface,
		Status:          item.Status,
		CreatedAt:       formatTimestamp(item.CreatedAt),
		ResolvedAt:      formatTimestamp(item.ResolvedAt),
	}
}

func execLookPath(file string) (string, error) {
	return execLookPathFunc(file)
}

var execLookPathFunc = func(file string) (string, error) {
	return exec.LookPath(file)
}

var startACPClient = func(ctx context.Context, spec acp.LaunchSpec) (agentClient, error) {
	return acp.Start(ctx, spec)
}

var (
	osExecutableFunc   = os.Executable
	getwdFunc          = os.Getwd
	sessionInitTimeout = 15 * time.Second
)

func (a *App) startThreadSession(thread threads.Thread) (*threadSession, error) {
	cfg, err := a.settings.Load()
	if err != nil {
		return nil, err
	}
	launch, err := cfg.ResolveAgentLaunch()
	if err != nil {
		return nil, err
	}

	client, err := a.startAgent(a.contextOrBackground(), acp.LaunchSpec{
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
		workspaceRoot, _ = getwdFunc()
	}

	bootstrapCtx, cancel := context.WithTimeout(a.contextOrBackground(), sessionInitTimeout)
	defer cancel()

	sessionID, err := client.Bootstrap(bootstrapCtx, acp.SessionConfig{
		CWD:       workspaceRoot,
		SessionID: thread.SessionID,
	})
	if err != nil {
		_ = client.Close()
		return nil, err
	}

	return &threadSession{
		client:    client,
		sessionID: sessionID,
	}, nil
}

func bundledASCCandidates() []string {
	var candidates []string
	if executable, err := osExecutableFunc(); err == nil && strings.TrimSpace(executable) != "" {
		execDir := filepath.Dir(executable)
		candidates = append(candidates,
			filepath.Clean(filepath.Join(execDir, "..", "Resources", "bin", "asc")),
			filepath.Clean(filepath.Join(execDir, "bin", "asc")),
		)
	}

	if workingDir, err := getwdFunc(); err == nil && strings.TrimSpace(workingDir) != "" {
		candidates = append(candidates,
			filepath.Join(workingDir, "bin", "asc"),
			filepath.Join(workingDir, "apps", "studio", "bin", "asc"),
		)
	}

	seen := make(map[string]struct{}, len(candidates))
	deduped := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		deduped = append(deduped, candidate)
	}
	return deduped
}

func fileExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := os.Stat(filepath.Clean(path))
	return err == nil && !info.IsDir()
}
