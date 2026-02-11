package validation

import "strings"

var ageRatingLevelValues = map[string]struct{}{
	"NONE":                {},
	"INFREQUENT_OR_MILD":  {},
	"FREQUENT_OR_INTENSE": {},
	"INFREQUENT":          {},
	"FREQUENT":            {},
}

var ageRatingOverrideValues = map[string]struct{}{
	"NONE":           {},
	"NINE_PLUS":      {},
	"THIRTEEN_PLUS":  {},
	"SIXTEEN_PLUS":   {},
	"SEVENTEEN_PLUS": {},
	"UNRATED":        {},
}

var ageRatingOverrideV2Values = map[string]struct{}{
	"NONE":          {},
	"NINE_PLUS":     {},
	"THIRTEEN_PLUS": {},
	"SIXTEEN_PLUS":  {},
	"EIGHTEEN_PLUS": {},
	"UNRATED":       {},
}

var koreaAgeRatingOverrideValues = map[string]struct{}{
	"NONE":          {},
	"FIFTEEN_PLUS":  {},
	"NINETEEN_PLUS": {},
}

var kidsAgeBandValues = map[string]struct{}{
	"FIVE_AND_UNDER": {},
	"SIX_TO_EIGHT":   {},
	"NINE_TO_ELEVEN": {},
}

func ageRatingChecks(declaration *AgeRatingDeclaration) []CheckResult {
	var checks []CheckResult
	if declaration == nil {
		return []CheckResult{
			{
				ID:          "age_rating.missing_field",
				Severity:    SeverityError,
				Message:     "age rating declaration is missing",
				Remediation: "Complete the age rating declaration in App Store Connect",
			},
		}
	}

	requiredBools := []struct {
		field string
		value *bool
	}{
		{field: "advertising", value: declaration.Advertising},
		{field: "gambling", value: declaration.Gambling},
		{field: "healthOrWellnessTopics", value: declaration.HealthOrWellnessTopics},
		{field: "lootBox", value: declaration.LootBox},
		{field: "messagingAndChat", value: declaration.MessagingAndChat},
		{field: "parentalControls", value: declaration.ParentalControls},
		{field: "ageAssurance", value: declaration.AgeAssurance},
		{field: "unrestrictedWebAccess", value: declaration.UnrestrictedWebAccess},
		{field: "userGeneratedContent", value: declaration.UserGeneratedContent},
	}

	for _, item := range requiredBools {
		if item.value == nil {
			checks = append(checks, missingAgeRatingField(item.field))
		}
	}

	requiredEnums := []struct {
		field string
		value *string
	}{
		{field: "alcoholTobaccoOrDrugUseOrReferences", value: declaration.AlcoholTobaccoOrDrugUseOrReferences},
		{field: "contests", value: declaration.Contests},
		{field: "gamblingSimulated", value: declaration.GamblingSimulated},
		{field: "gunsOrOtherWeapons", value: declaration.GunsOrOtherWeapons},
		{field: "medicalOrTreatmentInformation", value: declaration.MedicalOrTreatmentInformation},
		{field: "profanityOrCrudeHumor", value: declaration.ProfanityOrCrudeHumor},
		{field: "sexualContentGraphicAndNudity", value: declaration.SexualContentGraphicAndNudity},
		{field: "sexualContentOrNudity", value: declaration.SexualContentOrNudity},
		{field: "horrorOrFearThemes", value: declaration.HorrorOrFearThemes},
		{field: "matureOrSuggestiveThemes", value: declaration.MatureOrSuggestiveThemes},
		{field: "violenceCartoonOrFantasy", value: declaration.ViolenceCartoonOrFantasy},
		{field: "violenceRealistic", value: declaration.ViolenceRealistic},
		{field: "violenceRealisticProlongedGraphicOrSadistic", value: declaration.ViolenceRealisticProlongedGraphicOrSadistic},
	}

	for _, item := range requiredEnums {
		if item.value == nil || strings.TrimSpace(*item.value) == "" {
			checks = append(checks, missingAgeRatingField(item.field))
			continue
		}
		if !validAgeRatingLevel(*item.value) {
			checks = append(checks, invalidAgeRatingField(item.field, *item.value))
		}
	}

	checks = append(checks, validateOptionalEnum("ageRatingOverride", declaration.AgeRatingOverride, ageRatingOverrideValues)...)
	checks = append(checks, validateOptionalEnum("ageRatingOverrideV2", declaration.AgeRatingOverrideV2, ageRatingOverrideV2Values)...)
	checks = append(checks, validateOptionalEnum("koreaAgeRatingOverride", declaration.KoreaAgeRatingOverride, koreaAgeRatingOverrideValues)...)
	checks = append(checks, validateOptionalEnum("kidsAgeBand", declaration.KidsAgeBand, kidsAgeBandValues)...)

	return checks
}

func missingAgeRatingField(field string) CheckResult {
	return CheckResult{
		ID:          "age_rating.missing_field",
		Severity:    SeverityError,
		Field:       field,
		Message:     "age rating field is missing",
		Remediation: "Complete the age rating declaration in App Store Connect",
	}
}

func invalidAgeRatingField(field string, value string) CheckResult {
	return CheckResult{
		ID:          "age_rating.invalid_value",
		Severity:    SeverityError,
		Field:       field,
		Message:     "age rating field has an invalid value: " + strings.TrimSpace(value),
		Remediation: "Use a supported value for the age rating field",
	}
}

func validateOptionalEnum(field string, value *string, allowed map[string]struct{}) []CheckResult {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	if _, ok := allowed[strings.TrimSpace(*value)]; ok {
		return nil
	}
	return []CheckResult{
		invalidAgeRatingField(field, *value),
	}
}

func validAgeRatingLevel(value string) bool {
	_, ok := ageRatingLevelValues[strings.TrimSpace(value)]
	return ok
}
