package validation

import "time"

// Severity represents the validation severity level.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

// Fixability identifies the channel that can resolve a validation finding.
type Fixability string

const (
	FixabilityAPIFixable Fixability = "api-fixable"
	FixabilityWebFixable Fixability = "web-fixable"
	FixabilityManual     Fixability = "manual"
)

// Resolution describes how to resolve an actionable validation finding.
type Resolution struct {
	Fixability         Fixability `json:"fixability"`
	Commands           []string   `json:"commands,omitempty"`
	AppStoreConnectURL string     `json:"appStoreConnectUrl,omitempty"`
}

// DeepStatus is the terminal state of one deep validation check.
type DeepStatus string

const (
	DeepStatusPassed        DeepStatus = "passed"
	DeepStatusBlocked       DeepStatus = "blocked"
	DeepStatusUnverified    DeepStatus = "unverified"
	DeepStatusNotApplicable DeepStatus = "notApplicable"
)

// DeepSource identifies where a deep validation result came from.
type DeepSource string

const (
	DeepSourcePublicAPI  DeepSource = "publicApi"
	DeepSourceWebSession DeepSource = "webSession"
	DeepSourceManual     DeepSource = "manual"
)

// DeepSessionStatus records whether a cached Apple web session was available.
type DeepSessionStatus string

const (
	DeepSessionCached           DeepSessionStatus = "cached"
	DeepSessionExpired          DeepSessionStatus = "expired"
	DeepSessionUnavailable      DeepSessionStatus = "unavailable"
	DeepSessionValidationFailed DeepSessionStatus = "validationFailed"
)

const (
	DeepCheckPrivacyPublishState       = "privacy.publish_state"
	DeepCheckIAPAttachment             = "iap.first_type_app_version_attachment"
	DeepCheckSubscriptionAttachment    = "subscriptions.first_type_app_version_attachment"
	DeepCheckAgreementsActive          = "agreements.active"
	DeepCheckAvailabilityConfigured    = "availability.configured"
	DeepCheckReviewInformationComplete = "review_information.required_fields"
)

// DeepCheck is one deterministic result collected by deep validation.
type DeepCheck struct {
	ID         string      `json:"id"`
	Status     DeepStatus  `json:"status"`
	Source     DeepSource  `json:"source"`
	Message    string      `json:"message"`
	Resolution *Resolution `json:"resolution,omitempty"`
}

// DeepSummary aggregates deep checks by terminal status.
type DeepSummary struct {
	Passed        int `json:"passed"`
	Blocked       int `json:"blocked"`
	Unverified    int `json:"unverified"`
	NotApplicable int `json:"notApplicable"`
}

// DeepReport contains the additive results collected by --deep.
type DeepReport struct {
	SessionStatus DeepSessionStatus `json:"sessionStatus"`
	Summary       DeepSummary       `json:"summary"`
	Checks        []DeepCheck       `json:"checks"`
}

// CheckResult represents a single validation check result.
type CheckResult struct {
	ID           string      `json:"id"`
	Severity     Severity    `json:"severity"`
	Message      string      `json:"message"`
	Remediation  string      `json:"remediation,omitempty"`
	Locale       string      `json:"locale,omitempty"`
	Field        string      `json:"field,omitempty"`
	ResourceType string      `json:"resourceType,omitempty"`
	ResourceID   string      `json:"resourceId,omitempty"`
	Resolution   *Resolution `json:"resolution,omitempty"`
}

// Summary aggregates check counts by severity.
type Summary struct {
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
	Infos    int `json:"infos"`
	Blocking int `json:"blocking"`
}

// Remediation summarizes actionable validation steps in priority order.
type Remediation struct {
	TotalActionable int               `json:"totalActionable"`
	Steps           []RemediationStep `json:"steps"`
}

// Report is the top-level validation output.
type Report struct {
	AppID                 string        `json:"appId"`
	VersionID             string        `json:"versionId"`
	VersionString         string        `json:"versionString,omitempty"`
	VersionState          string        `json:"-"`
	Platform              string        `json:"platform,omitempty"`
	Summary               Summary       `json:"summary"`
	Remediation           Remediation   `json:"remediation"`
	Checks                []CheckResult `json:"checks"`
	Strict                bool          `json:"strict,omitempty"`
	Deep                  *DeepReport   `json:"deep,omitempty"`
	HasActiveMonetization bool          `json:"-"`
	MonetizationKnown     bool          `json:"-"`
	HasPaidAppPrice       bool          `json:"-"`
	AppPricingKnown       bool          `json:"-"`
}

// Input collects the validation inputs.
type Input struct {
	AppID                       string
	AppInfoID                   string
	VersionID                   string
	VersionString               string
	VersionState                string
	Platform                    string
	PrimaryLocale               string
	VersionLocalizations        []VersionLocalization
	AppInfoLocalizations        []AppInfoLocalization
	ReviewDetails               *ReviewDetails
	PrimaryCategoryID           string
	ContentRightsDeclaration    *string
	Build                       *Build
	PriceScheduleID             string
	PricingFetchSkipReason      string
	BaseTerritory               string
	BasePriceMissing            bool
	AvailabilityID              string
	AvailableTerritories        int
	AppAvailableTerritories     []string
	PricingTerritories          []string
	PricingTerritoryCount       int
	AvailabilityFetchSkipReason string
	PricingCoverageSkipReason   string
	ScreenshotSets              []ScreenshotSet
	// SupportsIPad reports whether the app binary declares iPad support in
	// UIDeviceFamily. It is nil when no binary was inspected.
	SupportsIPad                *bool
	Subscriptions               []Subscription
	SubscriptionFetchSkipReason string
	IAPs                        []IAP
	IAPFetchSkipReason          string
	AgeRatingDeclaration        *AgeRatingDeclaration
	ReleaseType                 string
	EarliestReleaseDate         string
	Copyright                   string
	HasPaidAppPrice             bool
	AppPricingKnown             bool
	// CustomProductPageLocalizations lists the search keywords of the app's
	// current custom product page localizations.
	CustomProductPageLocalizations      []CustomProductPageLocalization
	CustomProductPageKeywordsSkipReason string
	// BuildSDKName is the binary's DTSDKName, empty when no binary was
	// inspected.
	BuildSDKName string
	// Now is the time date-dependent checks read; zero means the current time.
	Now time.Time
}

// CustomProductPageLocalization is one custom product page localization and
// its search keywords.
type CustomProductPageLocalization struct {
	PageName       string
	LocalizationID string
	Locale         string
	SearchKeywords []string
}

// VersionLocalization represents version-level metadata.
type VersionLocalization struct {
	ID              string
	Locale          string
	Description     string
	Keywords        string
	WhatsNew        string
	PromotionalText string
	SupportURL      string
	MarketingURL    string
}

// AppInfoLocalization represents app info metadata.
type AppInfoLocalization struct {
	ID                string
	Locale            string
	Name              string
	Subtitle          string
	PrivacyPolicyURL  string
	PrivacyChoicesURL string
}

// ScreenshotSet represents a screenshot set and its assets.
type ScreenshotSet struct {
	ID             string
	DisplayType    string
	Locale         string
	LocalizationID string
	Screenshots    []Screenshot
}

// Screenshot represents a screenshot asset.
type Screenshot struct {
	ID       string
	FileName string
	Width    int
	Height   int
}

// ReviewDetails represents App Store review details for a version.
type ReviewDetails struct {
	ID                  string
	ContactFirstName    string
	ContactLastName     string
	ContactEmail        string
	ContactPhone        string
	DemoAccountName     string
	DemoAccountPassword string
	DemoAccountRequired bool
	Notes               string
}

// Build represents an attached build for a version.
type Build struct {
	ID                            string
	Version                       string
	ProcessingState               string
	Expired                       bool
	UsesNonExemptEncryption       *bool
	AppEncryptionDeclarationID    string
	AppEncryptionDeclarationState string
	MinOSVersion                  string
	Bundles                       []BuildBundle
	// BundlesUnreadReason explains why Bundles could not be read. It is empty
	// when the read succeeded or was not attempted.
	BundlesUnreadReason string
}

// BuildBundle represents one bundle inside a build.
type BuildBundle struct {
	Type     string
	SDKBuild string
}

// AgeRatingDeclaration represents age rating attributes for validation.
type AgeRatingDeclaration struct {
	Advertising              *bool
	Gambling                 *bool
	HealthOrWellnessTopics   *bool
	LootBox                  *bool
	MessagingAndChat         *bool
	ParentalControls         *bool
	AgeAssurance             *bool
	SocialMedia              *bool
	SocialMediaAgeRestricted *bool
	UnrestrictedWebAccess    *bool
	UserGeneratedContent     *bool

	AlcoholTobaccoOrDrugUseOrReferences         *string
	Contests                                    *string
	GamblingSimulated                           *string
	GunsOrOtherWeapons                          *string
	MedicalOrTreatmentInformation               *string
	ProfanityOrCrudeHumor                       *string
	SexualContentGraphicAndNudity               *string
	SexualContentOrNudity                       *string
	HorrorOrFearThemes                          *string
	MatureOrSuggestiveThemes                    *string
	ViolenceCartoonOrFantasy                    *string
	ViolenceRealistic                           *string
	ViolenceRealisticProlongedGraphicOrSadistic *string

	KidsAgeBand               *string
	AgeRatingOverride         *string
	AgeRatingOverrideV2       *string
	KoreaAgeRatingOverride    *string
	DeveloperAgeRatingInfoURL *string
}
