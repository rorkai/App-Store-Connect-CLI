package main

import (
	"github.com/rudrankriyam/App-Store-Connect-CLI/apps/studio/internal/studio/approvals"
	"github.com/rudrankriyam/App-Store-Connect-CLI/apps/studio/internal/studio/ascbin"
	"github.com/rudrankriyam/App-Store-Connect-CLI/apps/studio/internal/studio/environment"
	"github.com/rudrankriyam/App-Store-Connect-CLI/apps/studio/internal/studio/settings"
	"github.com/rudrankriyam/App-Store-Connect-CLI/apps/studio/internal/studio/threads"
)

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

type FeedbackScreenshot struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type FeedbackItem struct {
	ID             string               `json:"id"`
	Comment        string               `json:"comment"`
	Email          string               `json:"email"`
	DeviceModel    string               `json:"deviceModel"`
	DeviceFamily   string               `json:"deviceFamily"`
	OSVersion      string               `json:"osVersion"`
	AppPlatform    string               `json:"appPlatform"`
	CreatedDate    string               `json:"createdDate"`
	Locale         string               `json:"locale"`
	TimeZone       string               `json:"timeZone"`
	ConnectionType string               `json:"connectionType"`
	Battery        int                  `json:"batteryPercentage"`
	Screenshots    []FeedbackScreenshot `json:"screenshots"`
}

type FeedbackResponse struct {
	Feedback []FeedbackItem `json:"feedback"`
	Total    int            `json:"total"`
	Error    string         `json:"error,omitempty"`
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

type ScreenshotSet struct {
	DisplayType string          `json:"displayType"`
	Screenshots []AppScreenshot `json:"screenshots"`
}

type ScreenshotsResponse struct {
	Sets  []ScreenshotSet `json:"sets"`
	Error string          `json:"error,omitempty"`
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

type rawASCApp struct {
	Type       string `json:"type"`
	ID         string `json:"id"`
	Attributes struct {
		Name     string `json:"name"`
		BundleID string `json:"bundleId"`
		SKU      string `json:"sku"`
	} `json:"attributes"`
}
