package appclips

import (
	"context"
	"fmt"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
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
	items := splitCSVUpper(value)
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
	include := splitCSV(value)
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
		return trimmed, nil
	}

	bundle := strings.TrimSpace(bundleID)
	if bundle == "" {
		return "", fmt.Errorf("--app-clip-id or --bundle-id is required")
	}
	if strings.TrimSpace(appID) == "" {
		return "", fmt.Errorf("--app is required with --bundle-id")
	}

	resp, err := client.GetAppClips(ctx, appID, asc.WithAppClipsBundleIDs([]string{bundle}), asc.WithAppClipsLimit(200))
	if err != nil {
		return "", fmt.Errorf("failed to resolve app clip ID: %w", err)
	}
	if len(resp.Data) == 0 {
		return "", fmt.Errorf("no App Clip found for bundle ID %q", bundle)
	}
	if len(resp.Data) > 1 {
		return "", fmt.Errorf("multiple App Clips found for bundle ID %q", bundle)
	}

	return resp.Data[0].ID, nil
}
