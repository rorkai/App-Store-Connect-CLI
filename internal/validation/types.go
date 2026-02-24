package validation

// Severity represents the validation severity level.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

// CheckResult represents a single validation check result.
type CheckResult struct {
	ID           string   `json:"id"`
	Severity     Severity `json:"severity"`
	Message      string   `json:"message"`
	Remediation  string   `json:"remediation,omitempty"`
	Locale       string   `json:"locale,omitempty"`
	Field        string   `json:"field,omitempty"`
	ResourceType string   `json:"resourceType,omitempty"`
	ResourceID   string   `json:"resourceId,omitempty"`
}

// Summary aggregates check counts by severity.
type Summary struct {
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
	Infos    int `json:"infos"`
	Blocking int `json:"blocking"`
}

// Report is the top-level validation output.
type Report struct {
	AppID         string        `json:"appId"`
	VersionID     string        `json:"versionId"`
	VersionString string        `json:"versionString,omitempty"`
	Platform      string        `json:"platform,omitempty"`
	Summary       Summary       `json:"summary"`
	Checks        []CheckResult `json:"checks"`
	Strict        bool          `json:"strict,omitempty"`
}

// Input collects the validation inputs.
type Input struct {
	AppID                string
	AppInfoID            string
	VersionID            string
	VersionString        string
	VersionState         string
	Platform             string
	PrimaryLocale        string
	VersionLocalizations []VersionLocalization
	AppInfoLocalizations []AppInfoLocalization
	ReviewDetails        *ReviewDetails
	PrimaryCategoryID    string
	Build                *Build
	PriceScheduleID      string
	AvailabilityID       string
	AvailableTerritories int
	ScreenshotSets       []ScreenshotSet
	AgeRatingDeclaration *AgeRatingDeclaration
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
	ID               string
	Locale           string
	Name             string
	Subtitle         string
	PrivacyPolicyURL string
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
	ID              string
	Version         string
	ProcessingState string
	Expired         bool
}

// AgeRatingDeclaration represents age rating attributes for validation.
type AgeRatingDeclaration struct {
	Advertising            *bool
	Gambling               *bool
	HealthOrWellnessTopics *bool
	LootBox                *bool
	MessagingAndChat       *bool
	ParentalControls       *bool
	AgeAssurance           *bool
	UnrestrictedWebAccess  *bool
	UserGeneratedContent   *bool

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
