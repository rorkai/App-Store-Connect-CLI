package appclips

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var appClipActions = map[string]asc.AppClipAction{
	string(asc.AppClipActionOpen): asc.AppClipActionOpen,
	string(asc.AppClipActionView): asc.AppClipActionView,
	string(asc.AppClipActionPlay): asc.AppClipActionPlay,
}

var appClipBusinessCategories = map[string]asc.AppClipAdvancedExperienceBusinessCategory{
	string(asc.AppClipAdvancedExperienceBusinessCategoryAutomotive):           asc.AppClipAdvancedExperienceBusinessCategoryAutomotive,
	string(asc.AppClipAdvancedExperienceBusinessCategoryBeauty):               asc.AppClipAdvancedExperienceBusinessCategoryBeauty,
	string(asc.AppClipAdvancedExperienceBusinessCategoryBikes):                asc.AppClipAdvancedExperienceBusinessCategoryBikes,
	string(asc.AppClipAdvancedExperienceBusinessCategoryBooks):                asc.AppClipAdvancedExperienceBusinessCategoryBooks,
	string(asc.AppClipAdvancedExperienceBusinessCategoryCasino):               asc.AppClipAdvancedExperienceBusinessCategoryCasino,
	string(asc.AppClipAdvancedExperienceBusinessCategoryEducation):            asc.AppClipAdvancedExperienceBusinessCategoryEducation,
	string(asc.AppClipAdvancedExperienceBusinessCategoryEducationJapan):       asc.AppClipAdvancedExperienceBusinessCategoryEducationJapan,
	string(asc.AppClipAdvancedExperienceBusinessCategoryEntertainment):        asc.AppClipAdvancedExperienceBusinessCategoryEntertainment,
	string(asc.AppClipAdvancedExperienceBusinessCategoryEVCharger):            asc.AppClipAdvancedExperienceBusinessCategoryEVCharger,
	string(asc.AppClipAdvancedExperienceBusinessCategoryFinancialUSD):         asc.AppClipAdvancedExperienceBusinessCategoryFinancialUSD,
	string(asc.AppClipAdvancedExperienceBusinessCategoryFinancialCNY):         asc.AppClipAdvancedExperienceBusinessCategoryFinancialCNY,
	string(asc.AppClipAdvancedExperienceBusinessCategoryFinancialGBP):         asc.AppClipAdvancedExperienceBusinessCategoryFinancialGBP,
	string(asc.AppClipAdvancedExperienceBusinessCategoryFinancialJPY):         asc.AppClipAdvancedExperienceBusinessCategoryFinancialJPY,
	string(asc.AppClipAdvancedExperienceBusinessCategoryFinancialEUR):         asc.AppClipAdvancedExperienceBusinessCategoryFinancialEUR,
	string(asc.AppClipAdvancedExperienceBusinessCategoryFitness):              asc.AppClipAdvancedExperienceBusinessCategoryFitness,
	string(asc.AppClipAdvancedExperienceBusinessCategoryFoodAndDrink):         asc.AppClipAdvancedExperienceBusinessCategoryFoodAndDrink,
	string(asc.AppClipAdvancedExperienceBusinessCategoryGas):                  asc.AppClipAdvancedExperienceBusinessCategoryGas,
	string(asc.AppClipAdvancedExperienceBusinessCategoryGrocery):              asc.AppClipAdvancedExperienceBusinessCategoryGrocery,
	string(asc.AppClipAdvancedExperienceBusinessCategoryHealthAndMedical):     asc.AppClipAdvancedExperienceBusinessCategoryHealthAndMedical,
	string(asc.AppClipAdvancedExperienceBusinessCategoryHotelAndTravel):       asc.AppClipAdvancedExperienceBusinessCategoryHotelAndTravel,
	string(asc.AppClipAdvancedExperienceBusinessCategoryMusic):                asc.AppClipAdvancedExperienceBusinessCategoryMusic,
	string(asc.AppClipAdvancedExperienceBusinessCategoryParking):              asc.AppClipAdvancedExperienceBusinessCategoryParking,
	string(asc.AppClipAdvancedExperienceBusinessCategoryPetServices):          asc.AppClipAdvancedExperienceBusinessCategoryPetServices,
	string(asc.AppClipAdvancedExperienceBusinessCategoryProfessionalServices): asc.AppClipAdvancedExperienceBusinessCategoryProfessionalServices,
	string(asc.AppClipAdvancedExperienceBusinessCategoryShopping):             asc.AppClipAdvancedExperienceBusinessCategoryShopping,
	string(asc.AppClipAdvancedExperienceBusinessCategoryTicketing):            asc.AppClipAdvancedExperienceBusinessCategoryTicketing,
	string(asc.AppClipAdvancedExperienceBusinessCategoryTransit):              asc.AppClipAdvancedExperienceBusinessCategoryTransit,
}

var appClipLanguages = map[string]asc.AppClipAdvancedExperienceLanguage{
	string(asc.AppClipAdvancedExperienceLanguageAR): asc.AppClipAdvancedExperienceLanguageAR,
	string(asc.AppClipAdvancedExperienceLanguageCA): asc.AppClipAdvancedExperienceLanguageCA,
	string(asc.AppClipAdvancedExperienceLanguageCS): asc.AppClipAdvancedExperienceLanguageCS,
	string(asc.AppClipAdvancedExperienceLanguageDA): asc.AppClipAdvancedExperienceLanguageDA,
	string(asc.AppClipAdvancedExperienceLanguageDE): asc.AppClipAdvancedExperienceLanguageDE,
	string(asc.AppClipAdvancedExperienceLanguageEL): asc.AppClipAdvancedExperienceLanguageEL,
	string(asc.AppClipAdvancedExperienceLanguageEN): asc.AppClipAdvancedExperienceLanguageEN,
	string(asc.AppClipAdvancedExperienceLanguageES): asc.AppClipAdvancedExperienceLanguageES,
	string(asc.AppClipAdvancedExperienceLanguageFI): asc.AppClipAdvancedExperienceLanguageFI,
	string(asc.AppClipAdvancedExperienceLanguageFR): asc.AppClipAdvancedExperienceLanguageFR,
	string(asc.AppClipAdvancedExperienceLanguageHE): asc.AppClipAdvancedExperienceLanguageHE,
	string(asc.AppClipAdvancedExperienceLanguageHI): asc.AppClipAdvancedExperienceLanguageHI,
	string(asc.AppClipAdvancedExperienceLanguageHR): asc.AppClipAdvancedExperienceLanguageHR,
	string(asc.AppClipAdvancedExperienceLanguageHU): asc.AppClipAdvancedExperienceLanguageHU,
	string(asc.AppClipAdvancedExperienceLanguageID): asc.AppClipAdvancedExperienceLanguageID,
	string(asc.AppClipAdvancedExperienceLanguageIT): asc.AppClipAdvancedExperienceLanguageIT,
	string(asc.AppClipAdvancedExperienceLanguageJA): asc.AppClipAdvancedExperienceLanguageJA,
	string(asc.AppClipAdvancedExperienceLanguageKO): asc.AppClipAdvancedExperienceLanguageKO,
	string(asc.AppClipAdvancedExperienceLanguageMS): asc.AppClipAdvancedExperienceLanguageMS,
	string(asc.AppClipAdvancedExperienceLanguageNL): asc.AppClipAdvancedExperienceLanguageNL,
	string(asc.AppClipAdvancedExperienceLanguageNO): asc.AppClipAdvancedExperienceLanguageNO,
	string(asc.AppClipAdvancedExperienceLanguagePL): asc.AppClipAdvancedExperienceLanguagePL,
	string(asc.AppClipAdvancedExperienceLanguagePT): asc.AppClipAdvancedExperienceLanguagePT,
	string(asc.AppClipAdvancedExperienceLanguageRO): asc.AppClipAdvancedExperienceLanguageRO,
	string(asc.AppClipAdvancedExperienceLanguageRU): asc.AppClipAdvancedExperienceLanguageRU,
	string(asc.AppClipAdvancedExperienceLanguageSK): asc.AppClipAdvancedExperienceLanguageSK,
	string(asc.AppClipAdvancedExperienceLanguageSV): asc.AppClipAdvancedExperienceLanguageSV,
	string(asc.AppClipAdvancedExperienceLanguageTH): asc.AppClipAdvancedExperienceLanguageTH,
	string(asc.AppClipAdvancedExperienceLanguageTR): asc.AppClipAdvancedExperienceLanguageTR,
	string(asc.AppClipAdvancedExperienceLanguageUK): asc.AppClipAdvancedExperienceLanguageUK,
	string(asc.AppClipAdvancedExperienceLanguageVI): asc.AppClipAdvancedExperienceLanguageVI,
	string(asc.AppClipAdvancedExperienceLanguageZH): asc.AppClipAdvancedExperienceLanguageZH,
}

var appClipDefaultExperienceIncludeAliases = map[string]string{
	"app-clip":                         "appClip",
	"release-with-app-store-version":   "releaseWithAppStoreVersion",
	"default-experience-localizations": "appClipDefaultExperienceLocalizations",
	"review-detail":                    "appClipAppStoreReviewDetail",
}

func normalizeAppClipAction(value string) (asc.AppClipAction, error) {
	trimmed := strings.ToUpper(strings.TrimSpace(value))
	if trimmed == "" {
		return "", fmt.Errorf("action is required")
	}
	action, ok := appClipActions[trimmed]
	if !ok {
		return "", fmt.Errorf("invalid action %q", value)
	}
	return action, nil
}

func normalizeAppClipActionList(value string) ([]string, error) {
	items := shared.SplitCSVUpper(value)
	if len(items) == 0 {
		return nil, nil
	}
	for _, item := range items {
		if _, ok := appClipActions[item]; !ok {
			return nil, fmt.Errorf("invalid action %q", item)
		}
	}
	return items, nil
}

func normalizeAppClipBusinessCategory(value string) (asc.AppClipAdvancedExperienceBusinessCategory, error) {
	trimmed := strings.ToUpper(strings.TrimSpace(value))
	if trimmed == "" {
		return "", fmt.Errorf("category is required")
	}
	category, ok := appClipBusinessCategories[trimmed]
	if !ok {
		return "", fmt.Errorf("invalid category %q", value)
	}
	return category, nil
}

func normalizeAppClipLanguage(value string) (asc.AppClipAdvancedExperienceLanguage, error) {
	trimmed := strings.ToUpper(strings.TrimSpace(value))
	if trimmed == "" {
		return "", fmt.Errorf("default language is required")
	}
	lang, ok := appClipLanguages[trimmed]
	if !ok {
		return "", fmt.Errorf("invalid default language %q", value)
	}
	return lang, nil
}

func normalizeAppClipDefaultExperienceInclude(value string) ([]string, error) {
	include := shared.SplitCSV(value)
	if len(include) == 0 {
		return nil, nil
	}
	allowed := map[string]struct{}{}
	for _, option := range appClipDefaultExperienceIncludeList() {
		allowed[option] = struct{}{}
	}

	normalized := make([]string, 0, len(include))
	for _, option := range include {
		trimmed := strings.TrimSpace(option)
		if trimmed == "" {
			continue
		}
		if mapped, ok := appClipDefaultExperienceIncludeAliases[trimmed]; ok {
			normalized = append(normalized, mapped)
			continue
		}
		if _, ok := allowed[trimmed]; !ok {
			return nil, fmt.Errorf("--include must be one of: %s", strings.Join(appClipDefaultExperienceIncludeList(), ", "))
		}
		normalized = append(normalized, trimmed)
	}

	return normalized, nil
}

func appClipDefaultExperienceIncludeList() []string {
	return []string{
		"appClip",
		"releaseWithAppStoreVersion",
		"appClipDefaultExperienceLocalizations",
		"appClipAppStoreReviewDetail",
	}
}

func resolveAppClipID(ctx context.Context, client *asc.Client, appID string, appClipID string, bundleID string) (string, error) {
	if trimmed := strings.TrimSpace(appClipID); trimmed != "" {
		if strings.TrimSpace(appID) == "" {
			return trimmed, nil
		}

		if err := verifyAppClipBelongsToApp(ctx, client, appID, trimmed); err != nil {
			return "", err
		}
		return trimmed, nil
	}

	bundle := strings.TrimSpace(bundleID)
	if bundle == "" {
		return "", fmt.Errorf("--app-clip-id or --bundle-id is required")
	}
	if strings.TrimSpace(appID) == "" {
		return "", fmt.Errorf("--app is required with --bundle-id")
	}

	ctx = asc.WithNoRedirectReads(ctx)
	firstPage, err := client.GetAppClips(ctx, appID, asc.WithAppClipsBundleIDs([]string{bundle}), asc.Limit[asc.AppClipsOption](200))
	if err != nil {
		return "", fmt.Errorf("failed to resolve app clip ID: %w", err)
	}

	resp, err := asc.PaginateAll(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		continuation, err := validateAppClipsContinuationURL(nextURL, appID)
		if err != nil {
			return nil, err
		}
		return client.GetAppClips(ctx, appID, asc.NextURL[asc.AppClipsOption](continuation))
	})
	if err != nil {
		return "", fmt.Errorf("failed to resolve app clip ID: %w", err)
	}

	appClips, ok := resp.(*asc.AppClipsResponse)
	if !ok || appClips == nil {
		return "", fmt.Errorf("empty App Clip response for bundle ID %q", bundle)
	}
	if len(appClips.Data) == 0 {
		return "", fmt.Errorf("no App Clip found for bundle ID %q", bundle)
	}

	matches := make([]asc.Resource[asc.AppClipAttributes], 0, len(appClips.Data))
	for _, clip := range appClips.Data {
		if strings.EqualFold(strings.TrimSpace(clip.Attributes.BundleID), bundle) {
			matches = append(matches, clip)
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no App Clip found for exact bundle ID %q", bundle)
	}
	if len(matches) > 1 {
		candidates := make([]shared.AmbiguousCandidate, 0, len(matches))
		for _, clip := range matches {
			candidates = append(candidates, shared.AmbiguousCandidate{ID: strings.TrimSpace(clip.ID), Label: strings.TrimSpace(clip.Attributes.BundleID)})
		}
		ambiguous := shared.AmbiguousError("App Clip", "--app-clip-id", bundle, candidates)
		return "", ambiguous
	}

	return strings.TrimSpace(matches[0].ID), nil
}

func verifyAppClipBelongsToApp(ctx context.Context, client *asc.Client, appID string, appClipID string) error {
	ctx = asc.WithNoRedirectReads(ctx)
	firstPage, err := client.GetAppClips(ctx, appID, asc.Limit[asc.AppClipsOption](200))
	if err != nil {
		return fmt.Errorf("failed to verify App Clip %q belongs to app %q: %w", appClipID, appID, err)
	}

	resp, err := asc.PaginateAll(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		continuation, err := validateAppClipsContinuationURL(nextURL, appID)
		if err != nil {
			return nil, err
		}
		return client.GetAppClips(ctx, appID, asc.NextURL[asc.AppClipsOption](continuation))
	})
	if err != nil {
		return fmt.Errorf("failed to verify App Clip %q belongs to app %q: %w", appClipID, appID, err)
	}

	appClips, ok := resp.(*asc.AppClipsResponse)
	if !ok || appClips == nil {
		return fmt.Errorf("failed to verify App Clip %q belongs to app %q: invalid App Clip response", appClipID, appID)
	}
	for _, clip := range appClips.Data {
		if strings.TrimSpace(clip.ID) == appClipID {
			return nil
		}
	}

	return fmt.Errorf("app clip %q does not belong to selected app %q", appClipID, appID)
}

func validateAppClipsContinuationURL(nextURL, appID string) (string, error) {
	raw := strings.TrimSpace(nextURL)
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid provider continuation URL: %w", err)
	}
	if parsed.User != nil || parsed.Fragment != "" {
		return "", fmt.Errorf("provider continuation URL must stay on selected app's App Clips collection")
	}
	if parsed.Scheme != "" || parsed.Host != "" {
		base, err := url.Parse(asc.BaseURL)
		if err != nil || !strings.EqualFold(parsed.Scheme, base.Scheme) || !strings.EqualFold(parsed.Host, base.Host) {
			return "", fmt.Errorf("provider continuation URL must stay on selected app's App Clips collection")
		}
		parsed.Scheme = strings.ToLower(parsed.Scheme)
		parsed.Host = strings.ToLower(parsed.Host)
	}
	resolvedAppID, err := asc.ValidateResourcePathSegment(appID)
	if err != nil {
		return "", fmt.Errorf("invalid selected app ID: %w", err)
	}
	expectedPath := "/v1/apps/" + resolvedAppID + "/appClips"
	if parsed.Path != expectedPath {
		return "", fmt.Errorf("provider continuation URL must point to selected app's App Clips collection")
	}
	if raw == "" {
		return "", fmt.Errorf("provider continuation URL must point to selected app's App Clips collection")
	}
	return parsed.String(), nil
}
