package asc

import (
	"strconv"
	"strings"
)

type ageRatingField struct {
	Name  string
	Value string
}

func ageRatingDeclarationRows(resp *AgeRatingDeclarationResponse) ([]string, [][]string) {
	fields := ageRatingFields(resp)
	headers := []string{"Field", "Value"}
	rows := make([][]string, 0, len(fields))
	for _, field := range fields {
		rows = append(rows, []string{field.Name, field.Value})
	}
	return headers, rows
}

func printAgeRatingDeclarationTable(resp *AgeRatingDeclarationResponse) error {
	h, r := ageRatingDeclarationRows(resp)
	RenderTable(h, r)
	return nil
}

func printAgeRatingDeclarationMarkdown(resp *AgeRatingDeclarationResponse) error {
	h, r := ageRatingDeclarationRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func ageRatingFields(resp *AgeRatingDeclarationResponse) []ageRatingField {
	if resp == nil {
		return nil
	}
	attrs := resp.Data.Attributes
	fields := []ageRatingField{
		{Name: "ID", Value: fallbackValue(resp.Data.ID)},
		{Name: "Type", Value: fallbackValue(string(resp.Data.Type))},
		{Name: "Gambling", Value: formatOptionalBool(attrs.Gambling)},
		{Name: "Gambling Simulated", Value: formatOptionalString(attrs.GamblingSimulated)},
		{Name: "Alcohol/Tobacco/Drug Use", Value: formatOptionalString(attrs.AlcoholTobaccoOrDrugUseOrReferences)},
		{Name: "Contests", Value: formatOptionalString(attrs.Contests)},
		{Name: "Medical/Treatment", Value: formatOptionalString(attrs.MedicalOrTreatmentInformation)},
		{Name: "Profanity/Crude Humor", Value: formatOptionalString(attrs.ProfanityOrCrudeHumor)},
		{Name: "Sexual Content/Nudity", Value: formatOptionalString(attrs.SexualContentOrNudity)},
		{Name: "Sexual Content Graphic/Nudity", Value: formatOptionalString(attrs.SexualContentGraphicAndNudity)},
		{Name: "Horror/Fear", Value: formatOptionalString(attrs.HorrorOrFearThemes)},
		{Name: "Mature/Suggestive Themes", Value: formatOptionalString(attrs.MatureOrSuggestiveThemes)},
		{Name: "Violence Cartoon/Fantasy", Value: formatOptionalString(attrs.ViolenceCartoonOrFantasy)},
		{Name: "Violence Realistic", Value: formatOptionalString(attrs.ViolenceRealistic)},
		{Name: "Violence Realistic Prolonged Graphic/Sadistic", Value: formatOptionalString(attrs.ViolenceRealisticProlongedGraphicOrSadistic)},
		{Name: "Seventeen Plus", Value: formatOptionalBool(attrs.SeventeenPlus)},
		{Name: "Unrestricted Web Access", Value: formatOptionalBool(attrs.UnrestrictedWebAccess)},
		{Name: "Kids Age Band", Value: formatOptionalString(attrs.KidsAgeBand)},
	}
	return fields
}

func formatOptionalBool(value *bool) string {
	if value == nil {
		return "-"
	}
	return strconv.FormatBool(*value)
}

func formatOptionalString(value *string) string {
	if value == nil {
		return "-"
	}
	if strings.TrimSpace(*value) == "" {
		return "-"
	}
	return *value
}

func fallbackValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}
