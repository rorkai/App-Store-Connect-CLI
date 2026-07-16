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

func ageRatingFields(resp *AgeRatingDeclarationResponse) []ageRatingField {
	if resp == nil {
		return nil
	}
	attrs := resp.Data.Attributes
	fields := []ageRatingField{
		{Name: "ID", Value: fallbackValue(resp.Data.ID)},
		{Name: "Type", Value: fallbackValue(string(resp.Data.Type))},
		// Boolean content descriptors
		{Name: "Advertising", Value: formatOptionalBool(attrs.Advertising)},
		{Name: "Gambling", Value: formatOptionalBool(attrs.Gambling)},
		{Name: "Health/Wellness Topics", Value: formatOptionalBool(attrs.HealthOrWellnessTopics)},
		{Name: "Loot Box", Value: formatOptionalBool(attrs.LootBox)},
		{Name: "Messaging and Chat", Value: formatOptionalBool(attrs.MessagingAndChat)},
		{Name: "Parental Controls", Value: formatOptionalBool(attrs.ParentalControls)},
		{Name: "Age Assurance", Value: formatOptionalBool(attrs.AgeAssurance)},
		{Name: "Social Media", Value: formatOptionalBool(attrs.SocialMedia)},
		{Name: "Social Media Age Restricted", Value: formatOptionalBool(attrs.SocialMediaAgeRestricted)},
		{Name: "Unrestricted Web Access", Value: formatOptionalBool(attrs.UnrestrictedWebAccess)},
		{Name: "User-Generated Content", Value: formatOptionalBool(attrs.UserGeneratedContent)},
		// Enum content descriptors
		{Name: "Alcohol/Tobacco/Drug Use", Value: formatOptionalString(attrs.AlcoholTobaccoOrDrugUseOrReferences)},
		{Name: "Contests", Value: formatOptionalString(attrs.Contests)},
		{Name: "Gambling Simulated", Value: formatOptionalString(attrs.GamblingSimulated)},
		{Name: "Guns/Other Weapons", Value: formatOptionalString(attrs.GunsOrOtherWeapons)},
		{Name: "Medical/Treatment", Value: formatOptionalString(attrs.MedicalOrTreatmentInformation)},
		{Name: "Profanity/Crude Humor", Value: formatOptionalString(attrs.ProfanityOrCrudeHumor)},
		{Name: "Sexual Content/Nudity", Value: formatOptionalString(attrs.SexualContentOrNudity)},
		{Name: "Sexual Content Graphic/Nudity", Value: formatOptionalString(attrs.SexualContentGraphicAndNudity)},
		{Name: "Horror/Fear", Value: formatOptionalString(attrs.HorrorOrFearThemes)},
		{Name: "Mature/Suggestive Themes", Value: formatOptionalString(attrs.MatureOrSuggestiveThemes)},
		{Name: "Violence Cartoon/Fantasy", Value: formatOptionalString(attrs.ViolenceCartoonOrFantasy)},
		{Name: "Violence Realistic", Value: formatOptionalString(attrs.ViolenceRealistic)},
		{Name: "Violence Realistic Prolonged Graphic/Sadistic", Value: formatOptionalString(attrs.ViolenceRealisticProlongedGraphicOrSadistic)},
		// Overrides and metadata
		{Name: "Kids Age Band", Value: formatOptionalString(attrs.KidsAgeBand)},
		{Name: "Age Rating Override", Value: formatOptionalString(attrs.AgeRatingOverride)},
		{Name: "Age Rating Override V2", Value: formatOptionalString(attrs.AgeRatingOverrideV2)},
		{Name: "Korea Age Rating Override", Value: formatOptionalString(attrs.KoreaAgeRatingOverride)},
		{Name: "Developer Age Rating Info URL", Value: formatOptionalString(attrs.DeveloperAgeRatingInfoURL)},
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
