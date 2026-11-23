package validation

import (
	"fmt"
	"net/url"
	"strings"
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
		fix := publicAPIResolutionCommand(report, *check)
		if len(fix.commands) == 0 {
			continue
		}
		check.Resolution = &Resolution{
			Fixability:         FixabilityAPIFixable,
			Commands:           fix.commands,
			NeedsUserInput:     fix.needsUserInput,
			AppStoreConnectURL: defaultManualResolution(appID, check.ID).AppStoreConnectURL,
		}
		if !strings.Contains(check.Remediation, commandName(fix.commands[0])) {
			remediation := strings.TrimSuffix(strings.TrimSpace(check.Remediation), ".") + ": " + strings.Join(fix.commands, " or ")
			if fix.needsUserInput {
				remediation += " (replace each UPPERCASE placeholder with the developer's answer"
				if fix.note != "" {
					remediation += "; " + fix.note
				}
				remediation += ")"
			}
			check.Remediation = remediation
		}
	}
}

// fixCommand is the asc invocation that resolves one finding. Commands hold
// only values asc knows, such as resource IDs; anything the developer must
// decide (a price, territories, an age rating answer, listing text) is an
// UPPERCASE placeholder, and needsUserInput says so.
type fixCommand struct {
	commands       []string
	needsUserInput bool
	// note lists the values a placeholder takes when its name alone does not.
	note string
}

func knownFix(command string) fixCommand {
	return fixCommand{commands: []string{command}}
}

func userFix(commands ...string) fixCommand {
	return fixCommand{commands: commands, needsUserInput: true}
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
	fix := publicAPIResolutionCommand(report, check)
	if len(fix.commands) == 0 {
		return &Resolution{Fixability: FixabilityManual, AppStoreConnectURL: appURL}
	}
	return &Resolution{
		Fixability:         FixabilityAPIFixable,
		Commands:           fix.commands,
		NeedsUserInput:     fix.needsUserInput,
		AppStoreConnectURL: appURL,
	}
}

func publicAPIResolutionCommand(report Report, check CheckResult) fixCommand {
	resourceID := strings.TrimSpace(check.ResourceID)
	field := strings.TrimSpace(check.Field)
	switch check.ID {
	case "legal.required.copyright":
		if versionID := strings.TrimSpace(report.VersionID); versionID != "" {
			return userFix(fmt.Sprintf("asc versions update --version-id %q --copyright %q", versionID, "YEAR RIGHTS_HOLDER"))
		}
	case "content_rights.missing", "content_rights.invalid":
		if appID := strings.TrimSpace(report.AppID); appID != "" {
			return userFix(fmt.Sprintf("asc apps update --id %q --content-rights %q", appID, "USES_THIRD_PARTY_CONTENT_OR_DOES_NOT_USE_THIRD_PARTY_CONTENT"))
		}
	case "review_details.missing":
		if versionID := strings.TrimSpace(report.VersionID); versionID != "" {
			return userFix(fmt.Sprintf("asc review details-create --version-id %q --contact-first-name %q --contact-last-name %q --contact-email %q --contact-phone %q", versionID, "FIRST_NAME", "LAST_NAME", "EMAIL", "PHONE"))
		}
	case "categories.primary_missing":
		if appID := strings.TrimSpace(report.AppID); appID != "" {
			command := fmt.Sprintf("asc categories set --app %q", appID)
			if resourceID != "" {
				command += fmt.Sprintf(" --app-info %q", resourceID)
			}
			return userFix(command + fmt.Sprintf(" --primary %q", "CATEGORY_ID"))
		}
	case "build.required.missing", "build.invalid.expired":
		appID, versionID := strings.TrimSpace(report.AppID), strings.TrimSpace(report.VersionID)
		if appID != "" && versionID != "" {
			return knownFix(fmt.Sprintf("asc versions attach-build --app %q --version-id %q --latest", appID, versionID))
		}
	case "age_rating.missing_field":
		appID := strings.TrimSpace(report.AppID)
		if flag, values := ageRatingResolutionField(field); appID != "" && flag != "" {
			fix := userFix(fmt.Sprintf("asc age-rating edit --app %q --%s %q", appID, flag, values))
			if values == ageRatingLevelPlaceholder {
				// INFREQUENT_OR_MILD is one value, so the levels cannot be
				// joined with _OR_ the way TRUE_OR_FALSE is.
				fix.note = ageRatingLevelPlaceholder + " is NONE, INFREQUENT_OR_MILD, FREQUENT_OR_INTENSE, INFREQUENT, or FREQUENT"
			}
			return fix
		}
	case "availability.missing":
		if appID := strings.TrimSpace(report.AppID); appID != "" {
			return userFix(fmt.Sprintf("asc pricing availability create --app %q --territory %q --available true --available-in-new-territories %q", appID, "TERRITORIES", "TRUE_OR_FALSE"))
		}
	case "availability.territories.none":
		if appID := strings.TrimSpace(report.AppID); appID != "" {
			return userFix(fmt.Sprintf("asc pricing availability edit --app %q --territory %q --available true", appID, "TERRITORIES"))
		}
	case "pricing.schedule.missing":
		if appID := strings.TrimSpace(report.AppID); appID != "" {
			return userFix(
				fmt.Sprintf("asc pricing schedule create --app %q --price-point %q --base-territory %q", appID, "PRICE_POINT_ID", "BASE_TERRITORY"),
				fmt.Sprintf("asc pricing schedule create --app %q --free --base-territory %q", appID, "BASE_TERRITORY"),
			)
		}
	case "screenshots.required.any":
		appID, versionID := strings.TrimSpace(report.AppID), strings.TrimSpace(report.VersionID)
		if appID != "" && versionID != "" {
			// The screenshot files and their display type are the developer's:
			// the device type depends on the app's platform and devices.
			command := fmt.Sprintf("asc screenshots upload --app %q --version-id %q", appID, versionID)
			if locale := strings.TrimSpace(check.Locale); locale != "" {
				command += fmt.Sprintf(" --locale %q", locale)
			}
			return userFix(command + fmt.Sprintf(" --path %q --device-type %q", "SCREENSHOTS_DIR", "DEVICE_TYPE"))
		}
	case "screenshots.required.set_nonempty":
		// Deleting the screenshots of a set leaves the set behind. Either
		// upload to it again or delete the set itself; which one is the
		// developer's call.
		appID, versionID := strings.TrimSpace(report.AppID), strings.TrimSpace(report.VersionID)
		locale, deviceType := strings.TrimSpace(check.Locale), strings.TrimPrefix(check.displayType, "APP_")
		if resourceID == "" || appID == "" || versionID == "" || locale == "" || deviceType == "" {
			return fixCommand{}
		}
		deleteSet := fmt.Sprintf("asc localizations screenshot-sets delete --id %q --confirm", resourceID)
		upload := fmt.Sprintf("asc screenshots upload --app %q --version-id %q --locale %q --path %q --device-type %q", appID, versionID, locale, "SCREENSHOTS_DIR", deviceType)
		return userFix(upload, deleteSet)
	}

	if resourceID == "" {
		return fixCommand{}
	}
	flag, example := localizationResolutionField(field)
	switch check.ResourceType {
	case "appStoreVersionLocalization":
		if flag != "" {
			return userFix(fmt.Sprintf("asc localizations update --id %q --%s %q", resourceID, flag, example))
		}
	case "appInfoLocalization":
		if flag != "" {
			return userFix(fmt.Sprintf("asc localizations update --type app-info --id %q --%s %q", resourceID, flag, example))
		}
	case "appStoreReviewDetail":
		if flag, example := reviewDetailsResolutionField(field); flag != "" {
			return userFix(fmt.Sprintf("asc review details-update --id %q --%s %q", resourceID, flag, example))
		}
	}
	return fixCommand{}
}

func localizationResolutionField(field string) (string, string) {
	switch field {
	case "description":
		return "description", "DESCRIPTION"
	case "keywords":
		return "keywords", "KEYWORDS"
	case "supportUrl":
		return "support-url", "SUPPORT_URL"
	case "whatsNew":
		return "whats-new", "WHATS_NEW"
	case "marketingUrl":
		return "marketing-url", "MARKETING_URL"
	case "name":
		return "name", "APP_NAME"
	case "subtitle":
		return "subtitle", "SUBTITLE"
	case "privacyPolicyUrl":
		return "privacy-policy-url", "PRIVACY_POLICY_URL"
	case "privacyChoicesUrl":
		return "privacy-choices-url", "PRIVACY_CHOICES_URL"
	default:
		return "", ""
	}
}

// ageRatingLevelPlaceholder stands for one of the content-descriptor levels.
const ageRatingLevelPlaceholder = "LEVEL"

// ageRatingResolutionField maps an age rating declaration attribute to its
// asc age-rating edit flag and the placeholder for the values that flag
// accepts.
func ageRatingResolutionField(field string) (string, string) {
	const levels = ageRatingLevelPlaceholder
	switch field {
	case "advertising", "gambling":
		return field, "TRUE_OR_FALSE"
	case "contests":
		return field, levels
	case "healthOrWellnessTopics":
		return "health-or-wellness-topics", "TRUE_OR_FALSE"
	case "lootBox":
		return "loot-box", "TRUE_OR_FALSE"
	case "messagingAndChat":
		return "messaging-and-chat", "TRUE_OR_FALSE"
	case "parentalControls":
		return "parental-controls", "TRUE_OR_FALSE"
	case "ageAssurance":
		return "age-assurance", "TRUE_OR_FALSE"
	case "socialMedia":
		return "social-media", "TRUE_OR_FALSE"
	case "socialMediaAgeRestricted":
		return "social-media-age-restricted", "TRUE_OR_FALSE"
	case "unrestrictedWebAccess":
		return "unrestricted-web-access", "TRUE_OR_FALSE"
	case "userGeneratedContent":
		return "user-generated-content", "TRUE_OR_FALSE"
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
