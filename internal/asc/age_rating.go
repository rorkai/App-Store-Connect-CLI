package asc

import (
	"context"
	"encoding/json"
	"fmt"
)

// AgeRatingDeclarationAttributes describes the age rating declaration attributes.
type AgeRatingDeclarationAttributes struct {
	// Boolean content descriptors
	Advertising            *bool `json:"advertising,omitempty"`
	Gambling               *bool `json:"gambling,omitempty"`
	HealthOrWellnessTopics *bool `json:"healthOrWellnessTopics,omitempty"`
	LootBox                *bool `json:"lootBox,omitempty"`
	MessagingAndChat       *bool `json:"messagingAndChat,omitempty"`
	ParentalControls       *bool `json:"parentalControls,omitempty"`
	AgeAssurance           *bool `json:"ageAssurance,omitempty"`
	UnrestrictedWebAccess  *bool `json:"unrestrictedWebAccess,omitempty"`
	UserGeneratedContent   *bool `json:"userGeneratedContent,omitempty"`

	// Enum content descriptors (NONE, INFREQUENT_OR_MILD, FREQUENT_OR_INTENSE)
	AlcoholTobaccoOrDrugUseOrReferences         *string `json:"alcoholTobaccoOrDrugUseOrReferences,omitempty"`
	Contests                                    *string `json:"contests,omitempty"`
	GamblingSimulated                           *string `json:"gamblingSimulated,omitempty"`
	GunsOrOtherWeapons                          *string `json:"gunsOrOtherWeapons,omitempty"`
	MedicalOrTreatmentInformation               *string `json:"medicalOrTreatmentInformation,omitempty"`
	ProfanityOrCrudeHumor                       *string `json:"profanityOrCrudeHumor,omitempty"`
	SexualContentGraphicAndNudity               *string `json:"sexualContentGraphicAndNudity,omitempty"`
	SexualContentOrNudity                       *string `json:"sexualContentOrNudity,omitempty"`
	HorrorOrFearThemes                          *string `json:"horrorOrFearThemes,omitempty"`
	MatureOrSuggestiveThemes                    *string `json:"matureOrSuggestiveThemes,omitempty"`
	ViolenceCartoonOrFantasy                    *string `json:"violenceCartoonOrFantasy,omitempty"`
	ViolenceRealistic                           *string `json:"violenceRealistic,omitempty"`
	ViolenceRealisticProlongedGraphicOrSadistic *string `json:"violenceRealisticProlongedGraphicOrSadistic,omitempty"`

	// Age rating overrides and metadata
	KidsAgeBand               *string `json:"kidsAgeBand,omitempty"`
	AgeRatingOverride         *string `json:"ageRatingOverride,omitempty"`
	AgeRatingOverrideV2       *string `json:"ageRatingOverrideV2,omitempty"`
	KoreaAgeRatingOverride    *string `json:"koreaAgeRatingOverride,omitempty"`
	DeveloperAgeRatingInfoURL *string `json:"developerAgeRatingInfoUrl,omitempty"`

	// Deprecated fields (kept for backward compatibility with older API responses)
	SeventeenPlus *bool `json:"seventeenPlus,omitempty"`
}

// AppStoreAgeRating represents an App Store age rating value.
type AppStoreAgeRating string

const (
	AppStoreAgeRatingL             AppStoreAgeRating = "L"
	AppStoreAgeRatingAll           AppStoreAgeRating = "ALL"
	AppStoreAgeRatingOnePlus       AppStoreAgeRating = "ONE_PLUS"
	AppStoreAgeRatingTwoPlus       AppStoreAgeRating = "TWO_PLUS"
	AppStoreAgeRatingThreePlus     AppStoreAgeRating = "THREE_PLUS"
	AppStoreAgeRatingFourPlus      AppStoreAgeRating = "FOUR_PLUS"
	AppStoreAgeRatingFivePlus      AppStoreAgeRating = "FIVE_PLUS"
	AppStoreAgeRatingSixPlus       AppStoreAgeRating = "SIX_PLUS"
	AppStoreAgeRatingSevenPlus     AppStoreAgeRating = "SEVEN_PLUS"
	AppStoreAgeRatingEightPlus     AppStoreAgeRating = "EIGHT_PLUS"
	AppStoreAgeRatingNinePlus      AppStoreAgeRating = "NINE_PLUS"
	AppStoreAgeRatingTenPlus       AppStoreAgeRating = "TEN_PLUS"
	AppStoreAgeRatingElevenPlus    AppStoreAgeRating = "ELEVEN_PLUS"
	AppStoreAgeRatingTwelvePlus    AppStoreAgeRating = "TWELVE_PLUS"
	AppStoreAgeRatingThirteenPlus  AppStoreAgeRating = "THIRTEEN_PLUS"
	AppStoreAgeRatingFourteenPlus  AppStoreAgeRating = "FOURTEEN_PLUS"
	AppStoreAgeRatingFifteenPlus   AppStoreAgeRating = "FIFTEEN_PLUS"
	AppStoreAgeRatingSixteenPlus   AppStoreAgeRating = "SIXTEEN_PLUS"
	AppStoreAgeRatingSeventeenPlus AppStoreAgeRating = "SEVENTEEN_PLUS"
	AppStoreAgeRatingEighteenPlus  AppStoreAgeRating = "EIGHTEEN_PLUS"
	AppStoreAgeRatingNineteenPlus  AppStoreAgeRating = "NINETEEN_PLUS"
	AppStoreAgeRatingTwentyPlus    AppStoreAgeRating = "TWENTY_PLUS"
	AppStoreAgeRatingTwentyOnePlus AppStoreAgeRating = "TWENTY_ONE_PLUS"
	AppStoreAgeRatingUnrated       AppStoreAgeRating = "UNRATED"
)

// AgeRatingDeclarationResponse is the response from age rating declaration endpoints.
type AgeRatingDeclarationResponse = SingleResponse[AgeRatingDeclarationAttributes]

// AgeRatingDeclarationUpdateData is the data portion of an update request.
type AgeRatingDeclarationUpdateData struct {
	Type       ResourceType                   `json:"type"`
	ID         string                         `json:"id"`
	Attributes AgeRatingDeclarationAttributes `json:"attributes"`
}

// AgeRatingDeclarationUpdateRequest is a request to update an age rating declaration.
type AgeRatingDeclarationUpdateRequest struct {
	Data AgeRatingDeclarationUpdateData `json:"data"`
}

// GetAgeRatingDeclarationForAppInfo retrieves the age rating declaration for an app info.
func (c *Client) GetAgeRatingDeclarationForAppInfo(ctx context.Context, appInfoID string) (*AgeRatingDeclarationResponse, error) {
	path := fmt.Sprintf("/v1/appInfos/%s/ageRatingDeclaration", appInfoID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AgeRatingDeclarationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAgeRatingDeclarationForAppStoreVersion retrieves the age rating declaration for a version.
//
// Apple removed the version-scoped endpoint in API 4.3, so this now resolves the
// backing app info and retrieves the declaration from the supported app-info path.
func (c *Client) GetAgeRatingDeclarationForAppStoreVersion(ctx context.Context, versionID string) (*AgeRatingDeclarationResponse, error) {
	appInfoID, err := c.ResolveAppInfoIDForAppStoreVersion(ctx, versionID)
	if err != nil {
		return nil, err
	}
	return c.GetAgeRatingDeclarationForAppInfo(ctx, appInfoID)
}

// UpdateAgeRatingDeclaration updates an age rating declaration by ID.
func (c *Client) UpdateAgeRatingDeclaration(ctx context.Context, declarationID string, attributes AgeRatingDeclarationAttributes) (*AgeRatingDeclarationResponse, error) {
	request := AgeRatingDeclarationUpdateRequest{
		Data: AgeRatingDeclarationUpdateData{
			Type:       ResourceTypeAgeRatingDeclarations,
			ID:         declarationID,
			Attributes: attributes,
		},
	}

	body, err := BuildRequestBody(request)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "PATCH", fmt.Sprintf("/v1/ageRatingDeclarations/%s", declarationID), body)
	if err != nil {
		return nil, err
	}

	var response AgeRatingDeclarationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}
