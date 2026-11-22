package validation

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// SummarizeDeepChecks counts deep checks by terminal status.
func SummarizeDeepChecks(checks []DeepCheck) DeepSummary {
	summary := DeepSummary{}
	for _, check := range checks {
		switch check.Status {
		case DeepStatusPassed:
			summary.Passed++
		case DeepStatusBlocked:
			summary.Blocked++
		case DeepStatusUnverified:
			summary.Unverified++
		case DeepStatusNotApplicable:
			summary.NotApplicable++
		}
	}
	return summary
}

// ApplyDeepValidation merges deep evidence into a public readiness report and
// rebuilds all derived counts and remediation steps.
func ApplyDeepValidation(report Report, deep DeepReport, findings []CheckResult) Report {
	privacyVerified := false
	for _, check := range deep.Checks {
		if check.ID == DeepCheckPrivacyPublishState && (check.Status == DeepStatusPassed || check.Status == DeepStatusBlocked) {
			privacyVerified = true
			break
		}
	}
	checks := make([]CheckResult, 0, len(report.Checks)+len(findings))
	for _, check := range report.Checks {
		if check.ID == privacyPublishStateUnverifiedID && privacyVerified {
			continue
		}
		checks = append(checks, check)
	}
	checks = append(checks, findings...)

	for index := range checks {
		if checks[index].Resolution != nil {
			continue
		}
		if strings.TrimSpace(checks[index].Remediation) == "" && checks[index].Severity == SeverityInfo {
			continue
		}
		checks[index].Resolution = resolutionForPublicFinding(report, checks[index])
	}

	deep.Summary = SummarizeDeepChecks(deep.Checks)
	report.Checks = checks
	report.Summary = summarize(checks, report.Strict)
	report.Remediation = BuildRemediation(checks, report.Strict)
	report.Deep = &deep
	return report
}

// attachFixCommands gives every finding that a known asc command resolves the
// api-fixable resolution --deep attaches, and names the command at the end of
// the remediation text, so the default report says how to fix each blocker
// in JSON, in table output and in jq filters that read only remediation.
func attachFixCommands(appID, versionID string, checks []CheckResult) {
	report := Report{AppID: appID, VersionID: versionID}
	for index := range checks {
		check := &checks[index]
		if check.Resolution != nil || strings.TrimSpace(check.Remediation) == "" {
			continue
		}
		command := publicAPIResolutionCommand(report, *check)
		if command == "" {
			continue
		}
		check.Resolution = &Resolution{
			Fixability:         FixabilityAPIFixable,
			Commands:           []string{command},
			AppStoreConnectURL: defaultManualResolution(appID, check.ID).AppStoreConnectURL,
		}
		if !strings.Contains(check.Remediation, commandName(command)) {
			check.Remediation = strings.TrimSuffix(strings.TrimSpace(check.Remediation), ".") + ": " + command
		}
	}
}

// commandName returns the command path of an asc invocation, for example
// "asc apps update", so remediation text that already names it is kept.
func commandName(command string) string {
	fields := strings.Fields(command)
	if len(fields) > 3 {
		fields = fields[:3]
	}
	return strings.Join(fields, " ")
}

func resolutionForPublicFinding(report Report, check CheckResult) *Resolution {
	appURL := defaultManualResolution(report.AppID, check.ID).AppStoreConnectURL
	command := publicAPIResolutionCommand(report, check)
	if command == "" {
		return &Resolution{Fixability: FixabilityManual, AppStoreConnectURL: appURL}
	}
	return &Resolution{
		Fixability:         FixabilityAPIFixable,
		Commands:           []string{command},
		AppStoreConnectURL: appURL,
	}
}

func publicAPIResolutionCommand(report Report, check CheckResult) string {
	resourceID := strings.TrimSpace(check.ResourceID)
	field := strings.TrimSpace(check.Field)
	switch check.ID {
	case "legal.required.copyright":
		if versionID := strings.TrimSpace(report.VersionID); versionID != "" {
			return fmt.Sprintf("asc versions update --version-id %q --copyright %q", versionID, fmt.Sprintf("%d Your Company", time.Now().UTC().Year()))
		}
	case "content_rights.missing", "content_rights.invalid":
		if appID := strings.TrimSpace(report.AppID); appID != "" {
			return fmt.Sprintf("asc apps update --id %q --content-rights %q", appID, "DECLARATION")
		}
	case "review_details.missing":
		if versionID := strings.TrimSpace(report.VersionID); versionID != "" {
			return fmt.Sprintf("asc review details-create --version-id %q --contact-first-name %q --contact-last-name %q --contact-email %q --contact-phone %q", versionID, "FIRST_NAME", "LAST_NAME", "EMAIL", "PHONE")
		}
	case "categories.primary_missing":
		if appID := strings.TrimSpace(report.AppID); appID != "" {
			command := fmt.Sprintf("asc categories set --app %q", appID)
			if resourceID != "" {
				command += fmt.Sprintf(" --app-info %q", resourceID)
			}
			return command + fmt.Sprintf(" --primary %q", "CATEGORY_ID")
		}
	case "build.required.missing", "build.invalid.expired":
		appID, versionID := strings.TrimSpace(report.AppID), strings.TrimSpace(report.VersionID)
		if appID != "" && versionID != "" {
			return fmt.Sprintf("asc versions attach-build --app %q --version-id %q --latest", appID, versionID)
		}
	case "age_rating.missing_field":
		appID := strings.TrimSpace(report.AppID)
		if flag, values := ageRatingResolutionField(field); appID != "" && flag != "" {
			return fmt.Sprintf("asc age-rating edit --app %q --%s %q", appID, flag, values)
		}
	case "availability.missing":
		if appID := strings.TrimSpace(report.AppID); appID != "" {
			return fmt.Sprintf("asc pricing availability create --app %q --all-territories --available true --available-in-new-territories true", appID)
		}
	case "availability.territories.none":
		if appID := strings.TrimSpace(report.AppID); appID != "" {
			return fmt.Sprintf("asc pricing availability edit --app %q --all-territories --available true", appID)
		}
	case "pricing.schedule.missing":
		if appID := strings.TrimSpace(report.AppID); appID != "" {
			return fmt.Sprintf("asc pricing schedule create --app %q --free --base-territory %q", appID, "USA")
		}
	case "screenshots.required.any":
		appID, versionID := strings.TrimSpace(report.AppID), strings.TrimSpace(report.VersionID)
		if appID != "" && versionID != "" {
			command := fmt.Sprintf("asc screenshots upload --app %q --version-id %q", appID, versionID)
			path := "./screenshots"
			if locale := strings.TrimSpace(check.Locale); locale != "" {
				command += fmt.Sprintf(" --locale %q", locale)
				path += "/" + locale
			}
			return command + fmt.Sprintf(" --path %q --device-type %q", path, "IPHONE_65")
		}
	}

	if resourceID == "" {
		return ""
	}
	flag, example := localizationResolutionField(field)
	switch check.ResourceType {
	case "appStoreVersionLocalization":
		if flag != "" {
			return fmt.Sprintf("asc localizations update --id %q --%s %q", resourceID, flag, example)
		}
	case "appInfoLocalization":
		if flag != "" {
			return fmt.Sprintf("asc localizations update --type app-info --id %q --%s %q", resourceID, flag, example)
		}
	case "appStoreReviewDetail":
		if flag, example := reviewDetailsResolutionField(field); flag != "" {
			return fmt.Sprintf("asc review details-update --id %q --%s %q", resourceID, flag, example)
		}
	}
	return ""
}

func localizationResolutionField(field string) (string, string) {
	switch field {
	case "description":
		return "description", "DESCRIPTION"
	case "keywords":
		return "keywords", "KEYWORDS"
	case "supportUrl":
		return "support-url", "https://example.com/support"
	case "whatsNew":
		return "whats-new", "WHAT'S NEW"
	case "marketingUrl":
		return "marketing-url", "https://example.com"
	case "name":
		return "name", "APP NAME"
	case "subtitle":
		return "subtitle", "SUBTITLE"
	case "privacyPolicyUrl":
		return "privacy-policy-url", "https://example.com/privacy"
	case "privacyChoicesUrl":
		return "privacy-choices-url", "https://example.com/privacy-choices"
	default:
		return "", ""
	}
}

// ageRatingResolutionField maps an age rating declaration attribute to its
// asc age-rating edit flag and the values that flag accepts.
func ageRatingResolutionField(field string) (string, string) {
	const levels = "NONE|INFREQUENT_OR_MILD|FREQUENT_OR_INTENSE"
	switch field {
	case "advertising", "gambling":
		return field, "true|false"
	case "contests":
		return field, levels
	case "healthOrWellnessTopics":
		return "health-or-wellness-topics", "true|false"
	case "lootBox":
		return "loot-box", "true|false"
	case "messagingAndChat":
		return "messaging-and-chat", "true|false"
	case "parentalControls":
		return "parental-controls", "true|false"
	case "ageAssurance":
		return "age-assurance", "true|false"
	case "socialMedia":
		return "social-media", "true|false"
	case "socialMediaAgeRestricted":
		return "social-media-age-restricted", "true|false"
	case "unrestrictedWebAccess":
		return "unrestricted-web-access", "true|false"
	case "userGeneratedContent":
		return "user-generated-content", "true|false"
	case "alcoholTobaccoOrDrugUseOrReferences":
		return "alcohol-tobacco-drug-use", levels
	case "gamblingSimulated":
		return "gambling-simulated", levels
	case "gunsOrOtherWeapons":
		return "guns-or-other-weapons", levels
	case "medicalOrTreatmentInformation":
		return "medical-treatment", levels
	case "profanityOrCrudeHumor":
		return "profanity-humor", levels
	case "sexualContentGraphicAndNudity":
		return "sexual-content-graphic-nudity", levels
	case "sexualContentOrNudity":
		return "sexual-content-nudity", levels
	case "horrorOrFearThemes":
		return "horror-fear", levels
	case "matureOrSuggestiveThemes":
		return "mature-suggestive", levels
	case "violenceCartoonOrFantasy":
		return "violence-cartoon", levels
	case "violenceRealistic":
		return "violence-realistic", levels
	case "violenceRealisticProlongedGraphicOrSadistic":
		return "violence-realistic-graphic", levels
	default:
		return "", ""
	}
}

func reviewDetailsResolutionField(field string) (string, string) {
	switch field {
	case "contactFirstName":
		return "contact-first-name", "FIRST_NAME"
	case "contactLastName":
		return "contact-last-name", "LAST_NAME"
	case "contactEmail":
		return "contact-email", "EMAIL"
	case "contactPhone":
		return "contact-phone", "PHONE"
	case "demoAccountName":
		return "demo-account-name", "DEMO_ACCOUNT_NAME"
	case "demoAccountPassword":
		return "demo-account-password", "DEMO_ACCOUNT_PASSWORD"
	default:
		return "", ""
	}
}

func defaultManualResolution(appID, checkID string) *Resolution {
	appURL := appStoreConnectAppURL(appID, "")
	switch {
	case strings.HasPrefix(checkID, "availability."), strings.HasPrefix(checkID, "pricing."):
		appURL = appStoreConnectAppURL(appID, "appstore/pricing")
	case strings.HasPrefix(checkID, "review_details."):
		appURL = appStoreConnectAppURL(appID, "appstore/review")
	case strings.HasPrefix(checkID, "privacy."):
		appURL = appStoreConnectAppURL(appID, "appPrivacy")
	}
	return &Resolution{
		Fixability:         FixabilityManual,
		AppStoreConnectURL: appURL,
	}
}

func appStoreConnectAppURL(appID, suffix string) string {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return ""
	}
	base := "https://appstoreconnect.apple.com/apps/" + url.PathEscape(appID)
	if strings.TrimSpace(suffix) == "" {
		return base
	}
	return base + "/" + strings.TrimPrefix(strings.TrimSpace(suffix), "/")
}
