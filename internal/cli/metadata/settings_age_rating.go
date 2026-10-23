package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var ageRatingContentLevels = []string{"NONE", "INFREQUENT_OR_MILD", "FREQUENT_OR_INTENSE", "INFREQUENT", "FREQUENT"}

var ageRatingFields = []metadataSettingField{
	{name: "advertising", kind: metadataSettingBool},
	{name: "ageAssurance", kind: metadataSettingBool},
	{name: "gambling", kind: metadataSettingBool},
	{name: "healthOrWellnessTopics", kind: metadataSettingBool},
	{name: "lootBox", kind: metadataSettingBool},
	{name: "messagingAndChat", kind: metadataSettingBool},
	{name: "parentalControls", kind: metadataSettingBool},
	{name: "socialMedia", kind: metadataSettingBool},
	{name: "socialMediaAgeRestricted", kind: metadataSettingBool},
	{name: "unrestrictedWebAccess", kind: metadataSettingBool},
	{name: "userGeneratedContent", kind: metadataSettingBool},
	{name: "alcoholTobaccoOrDrugUseOrReferences", values: ageRatingContentLevels},
	{name: "contests", values: ageRatingContentLevels},
	{name: "gamblingSimulated", values: ageRatingContentLevels},
	{name: "gunsOrOtherWeapons", values: ageRatingContentLevels},
	{name: "horrorOrFearThemes", values: ageRatingContentLevels},
	{name: "matureOrSuggestiveThemes", values: ageRatingContentLevels},
	{name: "medicalOrTreatmentInformation", values: ageRatingContentLevels},
	{name: "profanityOrCrudeHumor", values: ageRatingContentLevels},
	{name: "sexualContentGraphicAndNudity", values: ageRatingContentLevels},
	{name: "sexualContentOrNudity", values: ageRatingContentLevels},
	{name: "violenceCartoonOrFantasy", values: ageRatingContentLevels},
	{name: "violenceRealistic", values: ageRatingContentLevels},
	{name: "violenceRealisticProlongedGraphicOrSadistic", values: ageRatingContentLevels},
	{name: "kidsAgeBand", values: []string{"FIVE_AND_UNDER", "SIX_TO_EIGHT", "NINE_TO_ELEVEN"}},
	{name: "ageRatingOverride", values: []string{"NONE", "NINE_PLUS", "THIRTEEN_PLUS", "SIXTEEN_PLUS", "SEVENTEEN_PLUS", "UNRATED"}},
	{name: "ageRatingOverrideV2", values: []string{"NONE", "NINE_PLUS", "THIRTEEN_PLUS", "SIXTEEN_PLUS", "EIGHTEEN_PLUS", "UNRATED"}},
	{name: "koreaAgeRatingOverride", values: []string{"NONE", "ALL", "TWELVE_PLUS", "FIFTEEN_PLUS", "NINETEEN_PLUS"}},
	{name: "gracRatingClassificationNumber"},
	{name: "developerAgeRatingInfoUrl"},
}

var ageRatingSettingScope = metadataSettingScope{
	name:         "age-rating",
	fileName:     "age-rating.json",
	fields:       ageRatingFields,
	needsAppInfo: true,
	fetch:        fetchAgeRatingSetting,
	update:       updateAgeRatingSetting,
}

func fetchAgeRatingDeclaration(ctx context.Context, client *asc.Client, appInfoID string) (*asc.AgeRatingDeclarationResponse, error) {
	return shared.RetryReadWithFreshTimeout(ctx, func(requestCtx context.Context) (*asc.AgeRatingDeclarationResponse, error) {
		return client.GetAgeRatingDeclarationForAppInfo(requestCtx, appInfoID)
	})
}

func fetchAgeRatingSetting(ctx context.Context, client *asc.Client, target metadataSettingTarget) (map[string]string, error) {
	resp, err := fetchAgeRatingDeclaration(ctx, client, target.appInfoID)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(resp.Data.Attributes)
	if err != nil {
		return nil, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	values := map[string]string{}
	for _, field := range ageRatingFields {
		rawValue, ok := raw[field.name]
		if !ok || isJSONNull(rawValue) {
			continue
		}
		if field.kind == metadataSettingBool {
			var value bool
			if err := json.Unmarshal(rawValue, &value); err != nil {
				return nil, fmt.Errorf("decode %s: %w", field.name, err)
			}
			values[field.name] = strconv.FormatBool(value)
			continue
		}
		var value string
		if err := json.Unmarshal(rawValue, &value); err != nil {
			return nil, fmt.Errorf("decode %s: %w", field.name, err)
		}
		if value != "" {
			values[field.name] = value
		}
	}
	return values, nil
}

func updateAgeRatingSetting(ctx context.Context, client *asc.Client, target metadataSettingTarget, changed map[string]string) error {
	resp, err := fetchAgeRatingDeclaration(ctx, client, target.appInfoID)
	if err != nil {
		return err
	}
	data, err := encodeMetadataSettingFile(metadataSettingScope{fields: ageRatingFields}, changed)
	if err != nil {
		return err
	}
	var attributes asc.AgeRatingDeclarationAttributes
	if err := json.Unmarshal(data, &attributes); err != nil {
		return err
	}
	_, err = client.UpdateAgeRatingDeclaration(ctx, resp.Data.ID, attributes)
	return err
}
