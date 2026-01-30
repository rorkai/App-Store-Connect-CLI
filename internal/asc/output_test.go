package asc

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

func captureStdout(t *testing.T, fn func() error) string {
	t.Helper()

	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe error: %v", err)
	}
	os.Stdout = w

	err = fn()

	if closeErr := w.Close(); closeErr != nil {
		t.Fatalf("close error: %v", closeErr)
	}
	os.Stdout = orig

	var buf bytes.Buffer
	if _, readErr := io.Copy(&buf, r); readErr != nil {
		t.Fatalf("read error: %v", readErr)
	}
	if err != nil {
		t.Fatalf("function error: %v", err)
	}

	return buf.String()
}

func TestPrintTable_Feedback(t *testing.T) {
	resp := &FeedbackResponse{
		Data: []Resource[FeedbackAttributes]{
			{
				ID: "1",
				Attributes: FeedbackAttributes{
					CreatedDate: "2026-01-20T00:00:00Z",
					Email:       "tester@example.com",
					Comment:     "Looks good",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Created") || !strings.Contains(output, "Email") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "tester@example.com") {
		t.Fatalf("expected email in output, got: %s", output)
	}
}

func TestPrintTable_FeedbackWithScreenshots(t *testing.T) {
	resp := &FeedbackResponse{
		Data: []Resource[FeedbackAttributes]{
			{
				ID: "1",
				Attributes: FeedbackAttributes{
					CreatedDate: "2026-01-20T00:00:00Z",
					Email:       "tester@example.com",
					Comment:     "Looks good",
					Screenshots: []FeedbackScreenshotImage{
						{URL: "https://example.com/shot.png", Width: 320, Height: 640},
					},
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Screenshots") {
		t.Fatalf("expected screenshots column, got: %s", output)
	}
	if !strings.Contains(output, "https://example.com/shot.png") {
		t.Fatalf("expected screenshot URL in output, got: %s", output)
	}
}

func TestPrintTable_Feedback_StripsControlChars(t *testing.T) {
	resp := &FeedbackResponse{
		Data: []Resource[FeedbackAttributes]{
			{
				ID: "1",
				Attributes: FeedbackAttributes{
					CreatedDate: "2026-01-20T00:00:00Z",
					Email:       "test\x1b[2J@example.com",
					Comment:     "ok\x1b[31mRED\x1b[0m",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if strings.Contains(output, "\x1b") {
		t.Fatalf("expected control characters to be stripped, got: %q", output)
	}
}

func TestPrintMarkdown_FeedbackWithScreenshots(t *testing.T) {
	resp := &FeedbackResponse{
		Data: []Resource[FeedbackAttributes]{
			{
				ID: "1",
				Attributes: FeedbackAttributes{
					CreatedDate: "2026-01-20T00:00:00Z",
					Email:       "tester@example.com",
					Comment:     "Looks good",
					Screenshots: []FeedbackScreenshotImage{
						{URL: "https://example.com/shot.png"},
					},
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| Screenshots |") {
		t.Fatalf("expected screenshots column, got: %s", output)
	}
	if !strings.Contains(output, "https://example.com/shot.png") {
		t.Fatalf("expected screenshot URL in output, got: %s", output)
	}
}

func TestPrintMarkdown_Reviews(t *testing.T) {
	resp := &ReviewsResponse{
		Data: []Resource[ReviewAttributes]{
			{
				ID: "1",
				Attributes: ReviewAttributes{
					CreatedDate: "2026-01-20T00:00:00Z",
					Rating:      5,
					Title:       "Great app",
					Territory:   "US",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| Created | Rating |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "Great app") {
		t.Fatalf("expected title in output, got: %s", output)
	}
}

func TestPrintMarkdown_Reviews_StripsControlChars(t *testing.T) {
	resp := &ReviewsResponse{
		Data: []Resource[ReviewAttributes]{
			{
				ID: "1",
				Attributes: ReviewAttributes{
					CreatedDate: "2026-01-20T00:00:00Z",
					Rating:      5,
					Title:       "Nice\x1b[31mTitle\x1b[0m",
					Territory:   "US",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if strings.Contains(output, "\x1b") {
		t.Fatalf("expected control characters to be stripped, got: %q", output)
	}
}

func TestPrintTable_OfferCodes(t *testing.T) {
	resp := &SubscriptionOfferCodeOneTimeUseCodesResponse{
		Data: []Resource[SubscriptionOfferCodeOneTimeUseCodeAttributes]{
			{
				ID: "code-1",
				Attributes: SubscriptionOfferCodeOneTimeUseCodeAttributes{
					NumberOfCodes:  5,
					CreatedDate:    "2026-01-20T00:00:00Z",
					ExpirationDate: "2026-01-31",
					Active:         true,
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Expires") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "code-1") {
		t.Fatalf("expected offer code id in output, got: %s", output)
	}
}

func TestPrintMarkdown_OfferCodes(t *testing.T) {
	resp := &SubscriptionOfferCodeOneTimeUseCodesResponse{
		Data: []Resource[SubscriptionOfferCodeOneTimeUseCodeAttributes]{
			{
				ID: "code-1",
				Attributes: SubscriptionOfferCodeOneTimeUseCodeAttributes{
					NumberOfCodes:  5,
					CreatedDate:    "2026-01-20T00:00:00Z",
					ExpirationDate: "2026-01-31",
					Active:         true,
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| ID |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "code-1") {
		t.Fatalf("expected offer code id in output, got: %s", output)
	}
}

func TestPrintTable_WinBackOffers(t *testing.T) {
	minimum := 3
	maximum := 12
	waitMonths := 6
	endDate := "2026-03-01"
	promotionIntent := WinBackOfferPromotionUseAutoGeneratedAssets
	resp := &WinBackOffersResponse{
		Data: []Resource[WinBackOfferAttributes]{
			{
				ID: "offer-1",
				Attributes: WinBackOfferAttributes{
					ReferenceName: "Spring Offer",
					OfferID:       "OFFER-1",
					Duration:      SubscriptionOfferDurationOneMonth,
					OfferMode:     SubscriptionOfferModePayAsYouGo,
					PeriodCount:   1,
					CustomerEligibilityPaidSubscriptionDurationInMonths: 6,
					CustomerEligibilityTimeSinceLastSubscribedInMonths:  &IntegerRange{Minimum: &minimum, Maximum: &maximum},
					CustomerEligibilityWaitBetweenOffersInMonths:        &waitMonths,
					StartDate:       "2026-02-01",
					EndDate:         &endDate,
					Priority:        WinBackOfferPriorityHigh,
					PromotionIntent: &promotionIntent,
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Reference Name") || !strings.Contains(output, "Priority") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "offer-1") {
		t.Fatalf("expected offer id in output, got: %s", output)
	}
}

func TestPrintMarkdown_WinBackOffers(t *testing.T) {
	minimum := 1
	maximum := 6
	resp := &WinBackOffersResponse{
		Data: []Resource[WinBackOfferAttributes]{
			{
				ID: "offer-2",
				Attributes: WinBackOfferAttributes{
					ReferenceName: "Summer Offer",
					OfferID:       "OFFER-2",
					Duration:      SubscriptionOfferDurationTwoWeeks,
					OfferMode:     SubscriptionOfferModeFreeTrial,
					PeriodCount:   2,
					CustomerEligibilityPaidSubscriptionDurationInMonths: 3,
					CustomerEligibilityTimeSinceLastSubscribedInMonths:  &IntegerRange{Minimum: &minimum, Maximum: &maximum},
					StartDate: "2026-04-01",
					Priority:  WinBackOfferPriorityNormal,
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| ID |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "offer-2") {
		t.Fatalf("expected offer id in output, got: %s", output)
	}
}

func TestPrintTable_WinBackOfferPrices(t *testing.T) {
	relationships, err := json.Marshal(WinBackOfferPriceRelationships{
		Territory: Relationship{
			Data: ResourceData{Type: ResourceTypeTerritories, ID: "USA"},
		},
		SubscriptionPricePoint: Relationship{
			Data: ResourceData{Type: ResourceTypeSubscriptionPricePoints, ID: "PRICE_POINT_1"},
		},
	})
	if err != nil {
		t.Fatalf("marshal relationships error: %v", err)
	}

	resp := &WinBackOfferPricesResponse{
		Data: []Resource[WinBackOfferPriceAttributes]{
			{
				ID:            "price-1",
				Relationships: relationships,
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Territory") || !strings.Contains(output, "Price Point") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "USA") || !strings.Contains(output, "PRICE_POINT_1") {
		t.Fatalf("expected relationship ids in output, got: %s", output)
	}
}

func TestPrintMarkdown_WinBackOfferPrices(t *testing.T) {
	relationships, err := json.Marshal(WinBackOfferPriceRelationships{
		Territory: Relationship{
			Data: ResourceData{Type: ResourceTypeTerritories, ID: "GBR"},
		},
		SubscriptionPricePoint: Relationship{
			Data: ResourceData{Type: ResourceTypeSubscriptionPricePoints, ID: "PRICE_POINT_2"},
		},
	})
	if err != nil {
		t.Fatalf("marshal relationships error: %v", err)
	}

	resp := &WinBackOfferPricesResponse{
		Data: []Resource[WinBackOfferPriceAttributes]{
			{
				ID:            "price-2",
				Relationships: relationships,
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| ID |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "GBR") || !strings.Contains(output, "PRICE_POINT_2") {
		t.Fatalf("expected relationship ids in output, got: %s", output)
	}
}

func TestPrintTable_Apps(t *testing.T) {
	resp := &AppsResponse{
		Data: []Resource[AppAttributes]{
			{
				ID: "123",
				Attributes: AppAttributes{
					Name:     "Demo App",
					BundleID: "com.example.demo",
					SKU:      "SKU-1",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Bundle ID") {
		t.Fatalf("expected apps header in output, got: %s", output)
	}
	if !strings.Contains(output, "com.example.demo") {
		t.Fatalf("expected bundle ID in output, got: %s", output)
	}
}

func TestPrintMarkdown_Apps(t *testing.T) {
	resp := &AppsResponse{
		Data: []Resource[AppAttributes]{
			{
				ID: "123",
				Attributes: AppAttributes{
					Name:     "Demo App",
					BundleID: "com.example.demo",
					SKU:      "SKU-1",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| ID | Name | Bundle ID | SKU |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "Demo App") {
		t.Fatalf("expected app name in output, got: %s", output)
	}
}

func TestPrintTable_Actors(t *testing.T) {
	resp := &ActorsResponse{
		Data: []Resource[ActorAttributes]{
			{
				ID: "actor-1",
				Attributes: ActorAttributes{
					ActorType:     "USER",
					UserFirstName: "Jane",
					UserLastName:  "Doe",
					UserEmail:     "jane@example.com",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "API Key ID") {
		t.Fatalf("expected actors header in output, got: %s", output)
	}
	if !strings.Contains(output, "jane@example.com") {
		t.Fatalf("expected actor email in output, got: %s", output)
	}
}

func TestPrintMarkdown_Actors(t *testing.T) {
	resp := &ActorsResponse{
		Data: []Resource[ActorAttributes]{
			{
				ID: "actor-1",
				Attributes: ActorAttributes{
					ActorType:     "API_KEY",
					UserFirstName: "",
					UserLastName:  "",
					APIKeyID:      "APIKEY123",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| ID | Type | Name | Email | API Key ID |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "APIKEY123") {
		t.Fatalf("expected api key id in output, got: %s", output)
	}
}

func TestPrintTable_AppStoreVersionLocalizations(t *testing.T) {
	resp := &AppStoreVersionLocalizationsResponse{
		Data: []Resource[AppStoreVersionLocalizationAttributes]{
			{
				ID: "loc-1",
				Attributes: AppStoreVersionLocalizationAttributes{
					Locale:   "en-US",
					WhatsNew: "Bug fixes",
					Keywords: "keyword1, keyword2",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Locale") {
		t.Fatalf("expected locale header, got: %s", output)
	}
	if !strings.Contains(output, "en-US") {
		t.Fatalf("expected locale in output, got: %s", output)
	}
}

func TestPrintTable_AppStoreVersionLocalization(t *testing.T) {
	resp := &AppStoreVersionLocalizationResponse{
		Data: Resource[AppStoreVersionLocalizationAttributes]{
			ID: "loc-1",
			Attributes: AppStoreVersionLocalizationAttributes{
				Locale:   "en-US",
				WhatsNew: "Bug fixes",
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "en-US") {
		t.Fatalf("expected locale in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppStoreVersionLocalizations(t *testing.T) {
	resp := &AppStoreVersionLocalizationsResponse{
		Data: []Resource[AppStoreVersionLocalizationAttributes]{
			{
				ID: "loc-1",
				Attributes: AppStoreVersionLocalizationAttributes{
					Locale:   "en-US",
					WhatsNew: "Bug fixes",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| Locale | Whats New |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "en-US") {
		t.Fatalf("expected locale in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppStoreVersionLocalization(t *testing.T) {
	resp := &AppStoreVersionLocalizationResponse{
		Data: Resource[AppStoreVersionLocalizationAttributes]{
			ID: "loc-1",
			Attributes: AppStoreVersionLocalizationAttributes{
				Locale:   "en-US",
				WhatsNew: "Bug fixes",
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| Locale | Whats New |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "en-US") {
		t.Fatalf("expected locale in output, got: %s", output)
	}
}

func TestPrintTable_AppStoreVersionLocalizationDeleteResult(t *testing.T) {
	result := &AppStoreVersionLocalizationDeleteResult{
		ID:      "loc-1",
		Deleted: true,
	}

	output := captureStdout(t, func() error {
		return PrintTable(result)
	})

	if !strings.Contains(output, "Deleted") {
		t.Fatalf("expected deleted header, got: %s", output)
	}
	if !strings.Contains(output, "loc-1") {
		t.Fatalf("expected id in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppStoreVersionLocalizationDeleteResult(t *testing.T) {
	result := &AppStoreVersionLocalizationDeleteResult{
		ID:      "loc-1",
		Deleted: true,
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(result)
	})

	if !strings.Contains(output, "| ID | Deleted |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "loc-1") {
		t.Fatalf("expected id in output, got: %s", output)
	}
}

func TestPrintTable_BetaBuildLocalizations(t *testing.T) {
	resp := &BetaBuildLocalizationsResponse{
		Data: []Resource[BetaBuildLocalizationAttributes]{
			{
				ID: "loc-1",
				Attributes: BetaBuildLocalizationAttributes{
					Locale:   "en-US",
					WhatsNew: "Test the new feature",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "What to Test") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "en-US") {
		t.Fatalf("expected locale in output, got: %s", output)
	}
}

func TestPrintTable_BetaBuildLocalization(t *testing.T) {
	resp := &BetaBuildLocalizationResponse{
		Data: Resource[BetaBuildLocalizationAttributes]{
			ID: "loc-1",
			Attributes: BetaBuildLocalizationAttributes{
				Locale:   "en-US",
				WhatsNew: "Test the new feature",
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "en-US") {
		t.Fatalf("expected locale in output, got: %s", output)
	}
}

func TestPrintMarkdown_BetaBuildLocalizations(t *testing.T) {
	resp := &BetaBuildLocalizationsResponse{
		Data: []Resource[BetaBuildLocalizationAttributes]{
			{
				ID: "loc-1",
				Attributes: BetaBuildLocalizationAttributes{
					Locale:   "en-US",
					WhatsNew: "Test the new feature",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| Locale | What to Test |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "en-US") {
		t.Fatalf("expected locale in output, got: %s", output)
	}
}

func TestPrintMarkdown_BetaBuildLocalization(t *testing.T) {
	resp := &BetaBuildLocalizationResponse{
		Data: Resource[BetaBuildLocalizationAttributes]{
			ID: "loc-1",
			Attributes: BetaBuildLocalizationAttributes{
				Locale:   "en-US",
				WhatsNew: "Test the new feature",
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| Locale | What to Test |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "en-US") {
		t.Fatalf("expected locale in output, got: %s", output)
	}
}

func TestPrintTable_BetaBuildLocalizationDeleteResult(t *testing.T) {
	result := &BetaBuildLocalizationDeleteResult{
		ID:      "loc-1",
		Deleted: true,
	}

	output := captureStdout(t, func() error {
		return PrintTable(result)
	})

	if !strings.Contains(output, "Deleted") {
		t.Fatalf("expected deleted header, got: %s", output)
	}
	if !strings.Contains(output, "loc-1") {
		t.Fatalf("expected id in output, got: %s", output)
	}
}

func TestPrintMarkdown_BetaBuildLocalizationDeleteResult(t *testing.T) {
	result := &BetaBuildLocalizationDeleteResult{
		ID:      "loc-1",
		Deleted: true,
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(result)
	})

	if !strings.Contains(output, "| ID | Deleted |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "loc-1") {
		t.Fatalf("expected id in output, got: %s", output)
	}
}

func TestPrintTable_AgeRatingDeclaration(t *testing.T) {
	boolPtr := func(value bool) *bool { return &value }
	stringPtr := func(value string) *string { return &value }

	resp := &AgeRatingDeclarationResponse{
		Data: Resource[AgeRatingDeclarationAttributes]{
			Type: ResourceTypeAgeRatingDeclarations,
			ID:   "age-1",
			Attributes: AgeRatingDeclarationAttributes{
				Gambling:          boolPtr(false),
				KidsAgeBand:       stringPtr("FIVE_AND_UNDER"),
				ViolenceRealistic: stringPtr("NONE"),
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Gambling") {
		t.Fatalf("expected gambling header, got: %s", output)
	}
	if !strings.Contains(output, "false") {
		t.Fatalf("expected gambling value, got: %s", output)
	}
	if !strings.Contains(output, "FIVE_AND_UNDER") {
		t.Fatalf("expected kids age band, got: %s", output)
	}
}

func TestPrintMarkdown_AgeRatingDeclaration(t *testing.T) {
	boolPtr := func(value bool) *bool { return &value }
	stringPtr := func(value string) *string { return &value }

	resp := &AgeRatingDeclarationResponse{
		Data: Resource[AgeRatingDeclarationAttributes]{
			Type: ResourceTypeAgeRatingDeclarations,
			ID:   "age-1",
			Attributes: AgeRatingDeclarationAttributes{
				Gambling:    boolPtr(true),
				KidsAgeBand: stringPtr("SIX_TO_EIGHT"),
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| Field | Value |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "true") {
		t.Fatalf("expected gambling value, got: %s", output)
	}
	if !strings.Contains(output, "SIX_TO_EIGHT") {
		t.Fatalf("expected kids age band, got: %s", output)
	}
}

func TestPrintTable_AppInfoLocalizations(t *testing.T) {
	resp := &AppInfoLocalizationsResponse{
		Data: []Resource[AppInfoLocalizationAttributes]{
			{
				ID: "loc-1",
				Attributes: AppInfoLocalizationAttributes{
					Locale:           "en-US",
					Name:             "Demo App",
					PrivacyPolicyURL: "https://example.com",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Privacy Policy URL") {
		t.Fatalf("expected privacy policy header, got: %s", output)
	}
	if !strings.Contains(output, "Demo App") {
		t.Fatalf("expected name in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppInfoLocalizations(t *testing.T) {
	resp := &AppInfoLocalizationsResponse{
		Data: []Resource[AppInfoLocalizationAttributes]{
			{
				ID: "loc-1",
				Attributes: AppInfoLocalizationAttributes{
					Locale: "en-US",
					Name:   "Demo App",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| Locale | Name | Subtitle |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "Demo App") {
		t.Fatalf("expected name in output, got: %s", output)
	}
}

func TestPrintTable_LocalizationDownloadResult(t *testing.T) {
	result := &LocalizationDownloadResult{
		Files: []LocalizationFileResult{
			{Locale: "en-US", Path: "localizations/en-US.strings"},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(result)
	})

	if !strings.Contains(output, "Path") {
		t.Fatalf("expected path header, got: %s", output)
	}
	if !strings.Contains(output, "en-US") {
		t.Fatalf("expected locale in output, got: %s", output)
	}
}

func TestPrintMarkdown_LocalizationUploadResult(t *testing.T) {
	result := &LocalizationUploadResult{
		Results: []LocalizationUploadLocaleResult{
			{Locale: "en-US", Action: "create", LocalizationID: "loc-1"},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(result)
	})

	if !strings.Contains(output, "| Locale | Action |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "loc-1") {
		t.Fatalf("expected localization id in output, got: %s", output)
	}
}

func TestPrintTable_AppTags(t *testing.T) {
	resp := &AppTagsResponse{
		Data: []Resource[AppTagAttributes]{
			{
				ID: "tag-1",
				Attributes: AppTagAttributes{
					Name:              "Strategy",
					VisibleInAppStore: true,
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Visible In App Store") {
		t.Fatalf("expected visibility header, got: %s", output)
	}
	if !strings.Contains(output, "Strategy") {
		t.Fatalf("expected tag name in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppTags(t *testing.T) {
	resp := &AppTagsResponse{
		Data: []Resource[AppTagAttributes]{
			{
				ID: "tag-1",
				Attributes: AppTagAttributes{
					Name:              "Strategy",
					VisibleInAppStore: false,
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| ID | Name | Visible In App Store |") {
		t.Fatalf("expected app tags header, got: %s", output)
	}
	if !strings.Contains(output, "Strategy") {
		t.Fatalf("expected tag name in output, got: %s", output)
	}
}

func TestPrintTable_PromotedPurchases(t *testing.T) {
	visible := true
	enabled := false
	resp := &PromotedPurchasesResponse{
		Data: []Resource[PromotedPurchaseAttributes]{
			{
				ID: "promo-1",
				Attributes: PromotedPurchaseAttributes{
					VisibleForAllUsers: &visible,
					Enabled:            &enabled,
					State:              "APPROVED",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Visible For All Users") {
		t.Fatalf("expected visibility header, got: %s", output)
	}
	if !strings.Contains(output, "promo-1") {
		t.Fatalf("expected promoted purchase ID in output, got: %s", output)
	}
}

func TestPrintMarkdown_PromotedPurchases(t *testing.T) {
	visible := false
	enabled := true
	resp := &PromotedPurchasesResponse{
		Data: []Resource[PromotedPurchaseAttributes]{
			{
				ID: "promo-2",
				Attributes: PromotedPurchaseAttributes{
					VisibleForAllUsers: &visible,
					Enabled:            &enabled,
					State:              "IN_REVIEW",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| ID | Visible For All Users | Enabled | State |") {
		t.Fatalf("expected promoted purchases header, got: %s", output)
	}
	if !strings.Contains(output, "promo-2") {
		t.Fatalf("expected promoted purchase ID in output, got: %s", output)
	}
}

func TestPrintTable_Nominations(t *testing.T) {
	resp := &NominationsResponse{
		Data: []Resource[NominationAttributes]{
			{
				ID: "nom-1",
				Attributes: NominationAttributes{
					Name:             "Spring Launch",
					Type:             NominationTypeAppLaunch,
					State:            NominationStateDraft,
					PublishStartDate: "2026-02-01T08:00:00Z",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Publish Start") {
		t.Fatalf("expected publish start header, got: %s", output)
	}
	if !strings.Contains(output, "Spring Launch") {
		t.Fatalf("expected nomination name in output, got: %s", output)
	}
}

func TestPrintMarkdown_Nominations(t *testing.T) {
	resp := &NominationsResponse{
		Data: []Resource[NominationAttributes]{
			{
				ID: "nom-1",
				Attributes: NominationAttributes{
					Name:             "Spring Launch",
					Type:             NominationTypeNewContent,
					State:            NominationStateSubmitted,
					PublishStartDate: "2026-02-01T08:00:00Z",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| ID | Name | Type | State |") {
		t.Fatalf("expected nominations header, got: %s", output)
	}
	if !strings.Contains(output, "Spring Launch") {
		t.Fatalf("expected nomination name in output, got: %s", output)
	}
}

func TestPrintTable_NominationDeleteResult(t *testing.T) {
	result := &NominationDeleteResult{
		ID:      "nom-1",
		Deleted: true,
	}

	output := captureStdout(t, func() error {
		return PrintTable(result)
	})

	if !strings.Contains(output, "Deleted") {
		t.Fatalf("expected deleted header, got: %s", output)
	}
	if !strings.Contains(output, "nom-1") {
		t.Fatalf("expected nomination id in output, got: %s", output)
	}
}

func TestPrintMarkdown_NominationDeleteResult(t *testing.T) {
	result := &NominationDeleteResult{
		ID:      "nom-1",
		Deleted: true,
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(result)
	})

	if !strings.Contains(output, "| ID | Deleted |") {
		t.Fatalf("expected delete markdown header, got: %s", output)
	}
	if !strings.Contains(output, "nom-1") {
		t.Fatalf("expected nomination id in output, got: %s", output)
	}
}

func TestPrintTable_Linkages(t *testing.T) {
	resp := &LinkagesResponse{
		Data: []ResourceData{
			{Type: ResourceTypeTerritories, ID: "USA"},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Type") || !strings.Contains(output, "ID") {
		t.Fatalf("expected linkages headers, got: %s", output)
	}
	if !strings.Contains(output, "territories") || !strings.Contains(output, "USA") {
		t.Fatalf("expected linkage values, got: %s", output)
	}
}

func TestPrintMarkdown_Linkages(t *testing.T) {
	resp := &LinkagesResponse{
		Data: []ResourceData{
			{Type: ResourceTypeTerritories, ID: "USA"},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| Type | ID |") {
		t.Fatalf("expected linkages header, got: %s", output)
	}
	if !strings.Contains(output, "territories") || !strings.Contains(output, "USA") {
		t.Fatalf("expected linkage values, got: %s", output)
	}
}

func TestPrintTable_ReviewSubmissions(t *testing.T) {
	resp := &ReviewSubmissionsResponse{
		Data: []ReviewSubmissionResource{
			{
				ID: "submission-1",
				Attributes: ReviewSubmissionAttributes{
					SubmissionState: ReviewSubmissionStateReadyForReview,
					Platform:        PlatformIOS,
					SubmittedDate:   "2026-01-20T00:00:00Z",
				},
				Relationships: &ReviewSubmissionRelationships{
					App: &Relationship{Data: ResourceData{Type: ResourceTypeApps, ID: "app-1"}},
					Items: &RelationshipList{Data: []ResourceData{
						{Type: ResourceTypeReviewSubmissionItems, ID: "item-1"},
					}},
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "State") {
		t.Fatalf("expected review submission headers, got: %s", output)
	}
	if !strings.Contains(output, "submission-1") || !strings.Contains(output, "READY_FOR_REVIEW") {
		t.Fatalf("expected review submission values, got: %s", output)
	}
}

func TestPrintMarkdown_ReviewSubmissions(t *testing.T) {
	resp := &ReviewSubmissionsResponse{
		Data: []ReviewSubmissionResource{
			{
				ID: "submission-1",
				Attributes: ReviewSubmissionAttributes{
					SubmissionState: ReviewSubmissionStateReadyForReview,
					Platform:        PlatformIOS,
					SubmittedDate:   "2026-01-20T00:00:00Z",
				},
				Relationships: &ReviewSubmissionRelationships{
					App: &Relationship{Data: ResourceData{Type: ResourceTypeApps, ID: "app-1"}},
					Items: &RelationshipList{Data: []ResourceData{
						{Type: ResourceTypeReviewSubmissionItems, ID: "item-1"},
					}},
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| ID | State |") {
		t.Fatalf("expected review submission markdown header, got: %s", output)
	}
	if !strings.Contains(output, "submission-1") || !strings.Contains(output, "READY_FOR_REVIEW") {
		t.Fatalf("expected review submission values, got: %s", output)
	}
}

func TestPrintTable_ReviewSubmissionItems(t *testing.T) {
	resp := &ReviewSubmissionItemsResponse{
		Data: []ReviewSubmissionItemResource{
			{
				ID: "item-1",
				Attributes: ReviewSubmissionItemAttributes{
					State: "READY_FOR_REVIEW",
				},
				Relationships: &ReviewSubmissionItemRelationships{
					ReviewSubmission: &Relationship{Data: ResourceData{Type: ResourceTypeReviewSubmissions, ID: "submission-1"}},
					AppStoreVersion:  &Relationship{Data: ResourceData{Type: ResourceTypeAppStoreVersions, ID: "version-1"}},
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Item Type") {
		t.Fatalf("expected review item headers, got: %s", output)
	}
	if !strings.Contains(output, "item-1") || !strings.Contains(output, "appStoreVersions") {
		t.Fatalf("expected review item values, got: %s", output)
	}
}

func TestPrintMarkdown_ReviewSubmissionItems(t *testing.T) {
	resp := &ReviewSubmissionItemsResponse{
		Data: []ReviewSubmissionItemResource{
			{
				ID: "item-1",
				Attributes: ReviewSubmissionItemAttributes{
					State: "READY_FOR_REVIEW",
				},
				Relationships: &ReviewSubmissionItemRelationships{
					ReviewSubmission: &Relationship{Data: ResourceData{Type: ResourceTypeReviewSubmissions, ID: "submission-1"}},
					AppStoreVersion:  &Relationship{Data: ResourceData{Type: ResourceTypeAppStoreVersions, ID: "version-1"}},
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| ID | State |") {
		t.Fatalf("expected review item markdown header, got: %s", output)
	}
	if !strings.Contains(output, "item-1") || !strings.Contains(output, "appStoreVersions") {
		t.Fatalf("expected review item values, got: %s", output)
	}
}

func TestPrintTable_BetaGroups(t *testing.T) {
	resp := &BetaGroupsResponse{
		Data: []Resource[BetaGroupAttributes]{
			{
				ID: "group-1",
				Attributes: BetaGroupAttributes{
					Name:              "Beta",
					IsInternalGroup:   true,
					PublicLinkEnabled: false,
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Public Link") {
		t.Fatalf("expected public link header, got: %s", output)
	}
	if !strings.Contains(output, "Beta") {
		t.Fatalf("expected group name in output, got: %s", output)
	}
}

func TestPrintMarkdown_BetaGroups(t *testing.T) {
	resp := &BetaGroupsResponse{
		Data: []Resource[BetaGroupAttributes]{
			{
				ID: "group-1",
				Attributes: BetaGroupAttributes{
					Name:            "Beta",
					IsInternalGroup: false,
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| ID | Name | Internal |") {
		t.Fatalf("expected beta groups header, got: %s", output)
	}
	if !strings.Contains(output, "Beta") {
		t.Fatalf("expected group name in output, got: %s", output)
	}
}

func TestPrintTable_BetaTesters(t *testing.T) {
	resp := &BetaTestersResponse{
		Data: []Resource[BetaTesterAttributes]{
			{
				ID: "tester-1",
				Attributes: BetaTesterAttributes{
					Email:      "tester@example.com",
					FirstName:  "Test",
					LastName:   "User",
					State:      BetaTesterStateInvited,
					InviteType: BetaInviteTypeEmail,
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Invite") {
		t.Fatalf("expected invite header, got: %s", output)
	}
	if !strings.Contains(output, "tester@example.com") {
		t.Fatalf("expected tester email in output, got: %s", output)
	}
}

func TestPrintMarkdown_BetaTesters(t *testing.T) {
	resp := &BetaTestersResponse{
		Data: []Resource[BetaTesterAttributes]{
			{
				ID: "tester-1",
				Attributes: BetaTesterAttributes{
					Email:     "tester@example.com",
					FirstName: "Test",
					LastName:  "User",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| ID | Email | Name | State | Invite |") {
		t.Fatalf("expected beta testers header, got: %s", output)
	}
	if !strings.Contains(output, "tester@example.com") {
		t.Fatalf("expected tester email in output, got: %s", output)
	}
}

func TestPrintTable_Builds(t *testing.T) {
	resp := &BuildsResponse{
		Data: []Resource[BuildAttributes]{
			{
				ID: "1",
				Attributes: BuildAttributes{
					Version:         "1.2.3",
					UploadedDate:    "2026-01-20T00:00:00Z",
					ProcessingState: "PROCESSING",
					Expired:         false,
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Processing") {
		t.Fatalf("expected builds header in output, got: %s", output)
	}
	if !strings.Contains(output, "1.2.3") {
		t.Fatalf("expected build version in output, got: %s", output)
	}
}

func TestPrintMarkdown_Builds(t *testing.T) {
	resp := &BuildsResponse{
		Data: []Resource[BuildAttributes]{
			{
				ID: "1",
				Attributes: BuildAttributes{
					Version:         "1.2.3",
					UploadedDate:    "2026-01-20T00:00:00Z",
					ProcessingState: "PROCESSING",
					Expired:         false,
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| Version | Uploaded | Processing | Expired |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "1.2.3") {
		t.Fatalf("expected build version in output, got: %s", output)
	}
}

func TestPrintTable_BuildBundles(t *testing.T) {
	bundleID := "com.example.app"
	bundleType := BuildBundleTypeApp
	sdkBuild := "16A100"
	platformBuild := "22A200"
	fileName := "App.app"

	resp := &BuildBundlesResponse{
		Data: []Resource[BuildBundleAttributes]{
			{
				ID: "bundle-1",
				Attributes: BuildBundleAttributes{
					BundleID:      &bundleID,
					BundleType:    &bundleType,
					SDKBuild:      &sdkBuild,
					PlatformBuild: &platformBuild,
					FileName:      &fileName,
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Bundle ID") {
		t.Fatalf("expected bundle ID header, got: %s", output)
	}
	if !strings.Contains(output, "com.example.app") {
		t.Fatalf("expected bundle ID in output, got: %s", output)
	}
}

func TestPrintMarkdown_BuildBundles(t *testing.T) {
	bundleID := "com.example.app"
	bundleType := BuildBundleTypeAppClip
	sdkBuild := "16A100"
	platformBuild := "22A200"
	fileName := "AppClip.app"

	resp := &BuildBundlesResponse{
		Data: []Resource[BuildBundleAttributes]{
			{
				ID: "bundle-2",
				Attributes: BuildBundleAttributes{
					BundleID:      &bundleID,
					BundleType:    &bundleType,
					SDKBuild:      &sdkBuild,
					PlatformBuild: &platformBuild,
					FileName:      &fileName,
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| ID | Bundle ID | Type |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "APP_CLIP") {
		t.Fatalf("expected bundle type in output, got: %s", output)
	}
}

func TestPrintTable_BuildBundleFileSizes(t *testing.T) {
	deviceModel := "iPhone16,1"
	osVersion := "18.0"
	downloadBytes := int64(2048)
	installBytes := int64(4096)

	resp := &BuildBundleFileSizesResponse{
		Data: []Resource[BuildBundleFileSizeAttributes]{
			{
				ID: "size-1",
				Attributes: BuildBundleFileSizeAttributes{
					DeviceModel:   &deviceModel,
					OSVersion:     &osVersion,
					DownloadBytes: &downloadBytes,
					InstallBytes:  &installBytes,
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Download Bytes") {
		t.Fatalf("expected download bytes header, got: %s", output)
	}
	if !strings.Contains(output, "iPhone16,1") {
		t.Fatalf("expected device model in output, got: %s", output)
	}
}

func TestPrintMarkdown_BuildBundleFileSizes(t *testing.T) {
	deviceModel := "iPhone16,1"
	osVersion := "18.0"
	downloadBytes := int64(2048)
	installBytes := int64(4096)

	resp := &BuildBundleFileSizesResponse{
		Data: []Resource[BuildBundleFileSizeAttributes]{
			{
				ID: "size-1",
				Attributes: BuildBundleFileSizeAttributes{
					DeviceModel:   &deviceModel,
					OSVersion:     &osVersion,
					DownloadBytes: &downloadBytes,
					InstallBytes:  &installBytes,
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| ID | Device Model | OS Version |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "4096") {
		t.Fatalf("expected install bytes in output, got: %s", output)
	}
}

func TestPrintTable_BetaAppClipInvocations(t *testing.T) {
	urlValue := "https://example.com/clip"

	resp := &BetaAppClipInvocationsResponse{
		Data: []Resource[BetaAppClipInvocationAttributes]{
			{
				ID: "inv-1",
				Attributes: BetaAppClipInvocationAttributes{
					URL: &urlValue,
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "URL") {
		t.Fatalf("expected URL header, got: %s", output)
	}
	if !strings.Contains(output, "https://example.com/clip") {
		t.Fatalf("expected URL in output, got: %s", output)
	}
}

func TestPrintMarkdown_BetaAppClipInvocations(t *testing.T) {
	urlValue := "https://example.com/clip"

	resp := &BetaAppClipInvocationsResponse{
		Data: []Resource[BetaAppClipInvocationAttributes]{
			{
				ID: "inv-1",
				Attributes: BetaAppClipInvocationAttributes{
					URL: &urlValue,
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| ID | URL |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "inv-1") {
		t.Fatalf("expected invocation ID in output, got: %s", output)
	}
}

func TestPrintTable_AppClipDomainStatusResult(t *testing.T) {
	lastUpdated := "2026-01-20T00:00:00Z"
	domain := "example.com"
	valid := true
	errCode := "BAD_HTTP_RESPONSE"

	result := &AppClipDomainStatusResult{
		BuildBundleID:   "bundle-1",
		Available:       true,
		StatusID:        "status-1",
		LastUpdatedDate: &lastUpdated,
		Domains: []AppClipDomainStatusDomain{
			{
				Domain:          &domain,
				IsValid:         &valid,
				LastUpdatedDate: &lastUpdated,
				ErrorCode:       &errCode,
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(result)
	})

	if !strings.Contains(output, "Build Bundle ID") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "example.com") {
		t.Fatalf("expected domain in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppClipDomainStatusResult(t *testing.T) {
	lastUpdated := "2026-01-20T00:00:00Z"
	domain := "example.com"
	valid := true

	result := &AppClipDomainStatusResult{
		BuildBundleID:   "bundle-1",
		Available:       true,
		StatusID:        "status-1",
		LastUpdatedDate: &lastUpdated,
		Domains: []AppClipDomainStatusDomain{
			{
				Domain:          &domain,
				IsValid:         &valid,
				LastUpdatedDate: &lastUpdated,
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(result)
	})

	if !strings.Contains(output, "| Build Bundle ID | Available | Status ID |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "example.com") {
		t.Fatalf("expected domain in output, got: %s", output)
	}
}

func TestPrintTable_BuildExpireAllResult(t *testing.T) {
	result := &BuildExpireAllResult{
		DryRun: true,
		Builds: []BuildExpireAllItem{
			{
				ID:           "BUILD_1",
				Version:      "1.2.3",
				UploadedDate: "2026-01-20T00:00:00Z",
				AgeDays:      10,
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(result)
	})

	if !strings.Contains(output, "Age Days") || !strings.Contains(output, "Status") {
		t.Fatalf("expected expire-all header in output, got: %s", output)
	}
	if !strings.Contains(output, "would-expire") {
		t.Fatalf("expected dry-run status in output, got: %s", output)
	}
	if !strings.Contains(output, "BUILD_1") {
		t.Fatalf("expected build id in output, got: %s", output)
	}
}

func TestPrintMarkdown_BuildExpireAllResult(t *testing.T) {
	result := &BuildExpireAllResult{
		DryRun: true,
		Builds: []BuildExpireAllItem{
			{
				ID:           "BUILD_1",
				Version:      "1.2.3",
				UploadedDate: "2026-01-20T00:00:00Z",
				AgeDays:      10,
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(result)
	})

	if !strings.Contains(output, "| ID | Version | Uploaded | Age Days | Status |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "would-expire") {
		t.Fatalf("expected dry-run status in output, got: %s", output)
	}
	if !strings.Contains(output, "BUILD_1") {
		t.Fatalf("expected build id in output, got: %s", output)
	}
}

func TestPrintTable_AppStoreVersions(t *testing.T) {
	resp := &AppStoreVersionsResponse{
		Data: []Resource[AppStoreVersionAttributes]{
			{
				ID: "VERSION_123",
				Attributes: AppStoreVersionAttributes{
					VersionString:   "1.0.0",
					Platform:        Platform("IOS"),
					AppVersionState: "READY_FOR_REVIEW",
					CreatedDate:     "2026-01-20T00:00:00Z",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Version") || !strings.Contains(output, "Platform") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "VERSION_123") {
		t.Fatalf("expected version ID in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppStoreVersions(t *testing.T) {
	resp := &AppStoreVersionsResponse{
		Data: []Resource[AppStoreVersionAttributes]{
			{
				ID: "VERSION_123",
				Attributes: AppStoreVersionAttributes{
					VersionString: "1.0.0",
					Platform:      Platform("IOS"),
					AppStoreState: "READY_FOR_REVIEW",
					CreatedDate:   "2026-01-20T00:00:00Z",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| Version | Platform |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "READY_FOR_REVIEW") {
		t.Fatalf("expected state in output, got: %s", output)
	}
}

func TestPrintTable_BuildInfo(t *testing.T) {
	resp := &BuildResponse{
		Data: Resource[BuildAttributes]{
			ID: "1",
			Attributes: BuildAttributes{
				Version:         "2.0.0",
				UploadedDate:    "2026-01-20T00:00:00Z",
				ProcessingState: "VALID",
				Expired:         true,
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Processing") {
		t.Fatalf("expected build info header in output, got: %s", output)
	}
	if !strings.Contains(output, "2.0.0") {
		t.Fatalf("expected build version in output, got: %s", output)
	}
}

func TestPrintMarkdown_BuildInfo(t *testing.T) {
	resp := &BuildResponse{
		Data: Resource[BuildAttributes]{
			ID: "1",
			Attributes: BuildAttributes{
				Version:         "2.0.0",
				UploadedDate:    "2026-01-20T00:00:00Z",
				ProcessingState: "VALID",
				Expired:         true,
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| Version | Uploaded | Processing | Expired |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "2.0.0") {
		t.Fatalf("expected build version in output, got: %s", output)
	}
}

func TestPrintPrettyJSON(t *testing.T) {
	resp := &ReviewsResponse{
		Data: []Resource[ReviewAttributes]{
			{
				ID: "1",
				Attributes: ReviewAttributes{
					CreatedDate: "2026-01-20T00:00:00Z",
					Rating:      5,
					Title:       "Great app",
					Body:        "Nice work",
					Territory:   "US",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintPrettyJSON(resp)
	})

	if !strings.Contains(output, "\n  \"data\"") {
		t.Fatalf("expected pretty JSON indentation, got: %s", output)
	}
}

func TestPrintTable_BuildUploadResult(t *testing.T) {
	resp := &BuildUploadResult{
		UploadID: "UPLOAD_123",
		FileID:   "FILE_123",
		FileName: "app.ipa",
		FileSize: 1024,
		Operations: []UploadOperation{
			{
				Method: "PUT",
				URL:    "https://example.com/upload",
				Length: 1024,
				Offset: 0,
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Upload ID") {
		t.Fatalf("expected upload header, got: %s", output)
	}
	if !strings.Contains(output, "UPLOAD_123") {
		t.Fatalf("expected upload ID in output, got: %s", output)
	}
	if !strings.Contains(output, "PUT") {
		t.Fatalf("expected operation method in output, got: %s", output)
	}
}

func TestPrintMarkdown_BuildUploadResult(t *testing.T) {
	resp := &BuildUploadResult{
		UploadID: "UPLOAD_123",
		FileID:   "FILE_123",
		FileName: "app.ipa",
		FileSize: 1024,
		Operations: []UploadOperation{
			{
				Method: "PUT",
				URL:    "https://example.com/upload",
				Length: 1024,
				Offset: 0,
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| Upload ID | File ID |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "UPLOAD_123") {
		t.Fatalf("expected upload ID in output, got: %s", output)
	}
	if !strings.Contains(output, "https://example.com/upload") {
		t.Fatalf("expected upload URL in output, got: %s", output)
	}
}

func TestPrintTable_AppScreenshotListResult(t *testing.T) {
	result := &AppScreenshotListResult{
		VersionLocalizationID: "LOC_123",
		Sets: []AppScreenshotSetWithScreenshots{
			{
				Set: Resource[AppScreenshotSetAttributes]{
					ID: "SET_123",
					Attributes: AppScreenshotSetAttributes{
						ScreenshotDisplayType: "APP_IPHONE_65",
					},
				},
				Screenshots: []Resource[AppScreenshotAttributes]{
					{
						ID: "SHOT_123",
						Attributes: AppScreenshotAttributes{
							FileName: "shot.png",
							FileSize: 1024,
							AssetDeliveryState: &AssetDeliveryState{
								State: "COMPLETE",
							},
						},
					},
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(result)
	})

	if !strings.Contains(output, "Set ID") {
		t.Fatalf("expected header, got: %s", output)
	}
	if !strings.Contains(output, "SET_123") {
		t.Fatalf("expected set ID in output, got: %s", output)
	}
	if !strings.Contains(output, "SHOT_123") {
		t.Fatalf("expected screenshot ID in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppPreviewUploadResult(t *testing.T) {
	result := &AppPreviewUploadResult{
		VersionLocalizationID: "LOC_123",
		SetID:                 "SET_123",
		PreviewType:           "IPHONE_65",
		Results: []AssetUploadResultItem{
			{FileName: "preview.mov", AssetID: "PREVIEW_123", State: "COMPLETE"},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(result)
	})

	if !strings.Contains(output, "| Localization ID | Set ID | Preview Type |") {
		t.Fatalf("expected preview header, got: %s", output)
	}
	if !strings.Contains(output, "PREVIEW_123") {
		t.Fatalf("expected preview ID in output, got: %s", output)
	}
}

func TestPrintTable_AssetDeleteResult(t *testing.T) {
	result := &AssetDeleteResult{ID: "ASSET_123", Deleted: true}

	output := captureStdout(t, func() error {
		return PrintTable(result)
	})

	if !strings.Contains(output, "ASSET_123") {
		t.Fatalf("expected asset ID in output, got: %s", output)
	}
	if !strings.Contains(output, "Deleted") {
		t.Fatalf("expected Deleted header, got: %s", output)
	}
}

func TestPrintTable_BuildUploadResult_WithUploadState(t *testing.T) {
	uploaded := true
	checksumVerified := true
	resp := &BuildUploadResult{
		UploadID:         "UPLOAD_123",
		FileID:           "FILE_123",
		FileName:         "app.ipa",
		FileSize:         1024,
		Uploaded:         &uploaded,
		ChecksumVerified: &checksumVerified,
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Uploaded") {
		t.Fatalf("expected uploaded column, got: %s", output)
	}
	if !strings.Contains(output, "Checksum Verified") {
		t.Fatalf("expected checksum verified column, got: %s", output)
	}
	if !strings.Contains(output, "true") {
		t.Fatalf("expected true values in output, got: %s", output)
	}
}

func TestPrintTable_SubmissionResult(t *testing.T) {
	createdDate := "2026-01-20T00:00:00Z"
	resp := &AppStoreVersionSubmissionResult{
		SubmissionID: "SUBMIT_123",
		CreatedDate:  &createdDate,
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Submission ID") {
		t.Fatalf("expected submission header, got: %s", output)
	}
	if !strings.Contains(output, "SUBMIT_123") {
		t.Fatalf("expected submission ID in output, got: %s", output)
	}
}

func TestPrintMarkdown_SubmissionResult(t *testing.T) {
	createdDate := "2026-01-20T00:00:00Z"
	resp := &AppStoreVersionSubmissionResult{
		SubmissionID: "SUBMIT_123",
		CreatedDate:  &createdDate,
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| Submission ID | Created Date |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "SUBMIT_123") {
		t.Fatalf("expected submission ID in output, got: %s", output)
	}
}

func TestPrintTable_SubmissionCreateResult(t *testing.T) {
	createdDate := "2026-01-20T00:00:00Z"
	resp := &AppStoreVersionSubmissionCreateResult{
		SubmissionID: "SUBMIT_123",
		VersionID:    "VERSION_123",
		BuildID:      "BUILD_123",
		CreatedDate:  &createdDate,
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Submission ID") {
		t.Fatalf("expected submission header, got: %s", output)
	}
	if !strings.Contains(output, "VERSION_123") || !strings.Contains(output, "BUILD_123") {
		t.Fatalf("expected IDs in output, got: %s", output)
	}
}

func TestPrintMarkdown_SubmissionCreateResult(t *testing.T) {
	createdDate := "2026-01-20T00:00:00Z"
	resp := &AppStoreVersionSubmissionCreateResult{
		SubmissionID: "SUBMIT_123",
		VersionID:    "VERSION_123",
		BuildID:      "BUILD_123",
		CreatedDate:  &createdDate,
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| Submission ID | Version ID | Build ID | Created Date |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "VERSION_123") {
		t.Fatalf("expected version ID in output, got: %s", output)
	}
}

func TestPrintTable_SubmissionStatusResult(t *testing.T) {
	createdDate := "2026-01-20T00:00:00Z"
	resp := &AppStoreVersionSubmissionStatusResult{
		ID:            "SUBMIT_123",
		VersionID:     "VERSION_123",
		VersionString: "1.0.0",
		Platform:      "IOS",
		State:         "WAITING_FOR_REVIEW",
		CreatedDate:   &createdDate,
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Submission ID") {
		t.Fatalf("expected submission header, got: %s", output)
	}
	if !strings.Contains(output, "WAITING_FOR_REVIEW") {
		t.Fatalf("expected state in output, got: %s", output)
	}
}

func TestPrintMarkdown_SubmissionStatusResult(t *testing.T) {
	createdDate := "2026-01-20T00:00:00Z"
	resp := &AppStoreVersionSubmissionStatusResult{
		ID:            "SUBMIT_123",
		VersionID:     "VERSION_123",
		VersionString: "1.0.0",
		Platform:      "IOS",
		State:         "WAITING_FOR_REVIEW",
		CreatedDate:   &createdDate,
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| Submission ID | Version ID | Version | Platform | State | Created Date |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "WAITING_FOR_REVIEW") {
		t.Fatalf("expected state in output, got: %s", output)
	}
}

func TestPrintTable_SubmissionCancelResult(t *testing.T) {
	resp := &AppStoreVersionSubmissionCancelResult{
		ID:        "SUBMIT_123",
		Cancelled: true,
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Cancelled") {
		t.Fatalf("expected cancelled header, got: %s", output)
	}
	if !strings.Contains(output, "SUBMIT_123") {
		t.Fatalf("expected submission ID in output, got: %s", output)
	}
}

func TestPrintMarkdown_SubmissionCancelResult(t *testing.T) {
	resp := &AppStoreVersionSubmissionCancelResult{
		ID:        "SUBMIT_123",
		Cancelled: true,
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| Submission ID | Cancelled |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "SUBMIT_123") {
		t.Fatalf("expected submission ID in output, got: %s", output)
	}
}

func TestPrintTable_AppStoreVersionDetailResult(t *testing.T) {
	resp := &AppStoreVersionDetailResult{
		ID:            "VERSION_123",
		VersionString: "1.0.0",
		Platform:      "IOS",
		State:         "READY_FOR_REVIEW",
		BuildID:       "BUILD_123",
		BuildVersion:  "1.0.0",
		SubmissionID:  "SUBMIT_123",
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Version ID") {
		t.Fatalf("expected version header, got: %s", output)
	}
	if !strings.Contains(output, "BUILD_123") {
		t.Fatalf("expected build ID in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppStoreVersionDetailResult(t *testing.T) {
	resp := &AppStoreVersionDetailResult{
		ID:            "VERSION_123",
		VersionString: "1.0.0",
		Platform:      "IOS",
		State:         "READY_FOR_REVIEW",
		BuildID:       "BUILD_123",
		BuildVersion:  "1.0.0",
		SubmissionID:  "SUBMIT_123",
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| Version ID | Version | Platform | State | Build ID |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "SUBMIT_123") {
		t.Fatalf("expected submission ID in output, got: %s", output)
	}
}

func TestPrintTable_AppStoreVersionPhasedReleaseResponse(t *testing.T) {
	resp := &AppStoreVersionPhasedReleaseResponse{
		Data: Resource[AppStoreVersionPhasedReleaseAttributes]{
			Type: "appStoreVersionPhasedReleases",
			ID:   "PHASED_123",
			Attributes: AppStoreVersionPhasedReleaseAttributes{
				PhasedReleaseState: PhasedReleaseStateActive,
				StartDate:          "2026-01-20",
				CurrentDayNumber:   3,
				TotalPauseDuration: 0,
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Phased Release ID") {
		t.Fatalf("expected phased release header, got: %s", output)
	}
	if !strings.Contains(output, "PHASED_123") {
		t.Fatalf("expected phased release ID in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppStoreVersionPhasedReleaseResponse(t *testing.T) {
	resp := &AppStoreVersionPhasedReleaseResponse{
		Data: Resource[AppStoreVersionPhasedReleaseAttributes]{
			Type: "appStoreVersionPhasedReleases",
			ID:   "PHASED_123",
			Attributes: AppStoreVersionPhasedReleaseAttributes{
				PhasedReleaseState: PhasedReleaseStatePaused,
				StartDate:          "2026-01-21",
				CurrentDayNumber:   2,
				TotalPauseDuration: 1,
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| Phased Release ID | State | Start Date | Current Day | Total Pause Duration |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "PHASED_123") {
		t.Fatalf("expected phased release ID in output, got: %s", output)
	}
}

func TestPrintTable_AppStoreVersionPhasedReleaseDeleteResult(t *testing.T) {
	resp := &AppStoreVersionPhasedReleaseDeleteResult{
		ID:      "PHASED_123",
		Deleted: true,
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Deleted") {
		t.Fatalf("expected deleted header, got: %s", output)
	}
	if !strings.Contains(output, "PHASED_123") {
		t.Fatalf("expected phased release ID in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppStoreVersionPhasedReleaseDeleteResult(t *testing.T) {
	resp := &AppStoreVersionPhasedReleaseDeleteResult{
		ID:      "PHASED_123",
		Deleted: true,
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| Phased Release ID | Deleted |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "PHASED_123") {
		t.Fatalf("expected phased release ID in output, got: %s", output)
	}
}

func TestPrintTable_AppStoreVersionAttachBuildResult(t *testing.T) {
	resp := &AppStoreVersionAttachBuildResult{
		VersionID: "VERSION_123",
		BuildID:   "BUILD_123",
		Attached:  true,
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Attached") {
		t.Fatalf("expected attached header, got: %s", output)
	}
	if !strings.Contains(output, "BUILD_123") {
		t.Fatalf("expected build ID in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppStoreVersionAttachBuildResult(t *testing.T) {
	resp := &AppStoreVersionAttachBuildResult{
		VersionID: "VERSION_123",
		BuildID:   "BUILD_123",
		Attached:  true,
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| Version ID | Build ID | Attached |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "VERSION_123") {
		t.Fatalf("expected version ID in output, got: %s", output)
	}
}

func TestPrintTable_AppStoreVersionReleaseRequestResult(t *testing.T) {
	resp := &AppStoreVersionReleaseRequestResult{
		ReleaseRequestID: "RELEASE_123",
		VersionID:        "VERSION_123",
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Release Request ID") {
		t.Fatalf("expected release request header, got: %s", output)
	}
	if !strings.Contains(output, "RELEASE_123") {
		t.Fatalf("expected release request ID in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppStoreVersionReleaseRequestResult(t *testing.T) {
	resp := &AppStoreVersionReleaseRequestResult{
		ReleaseRequestID: "RELEASE_123",
		VersionID:        "VERSION_123",
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| Release Request ID | Version ID |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "VERSION_123") {
		t.Fatalf("expected version ID in output, got: %s", output)
	}
}

func TestPrintTable_BuildBetaGroupsUpdateResult(t *testing.T) {
	resp := &BuildBetaGroupsUpdateResult{
		BuildID:  "BUILD_123",
		GroupIDs: []string{"GROUP_1", "GROUP_2"},
		Action:   "added",
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Group IDs") {
		t.Fatalf("expected group IDs header, got: %s", output)
	}
	if !strings.Contains(output, "GROUP_1, GROUP_2") {
		t.Fatalf("expected group IDs in output, got: %s", output)
	}
}

func TestPrintMarkdown_BuildBetaGroupsUpdateResult(t *testing.T) {
	resp := &BuildBetaGroupsUpdateResult{
		BuildID:  "BUILD_123",
		GroupIDs: []string{"GROUP_1", "GROUP_2"},
		Action:   "removed",
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| Build ID | Group IDs | Action |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "GROUP_1, GROUP_2") {
		t.Fatalf("expected group IDs in output, got: %s", output)
	}
}

func TestPrintTable_BuildIndividualTestersUpdateResult(t *testing.T) {
	resp := &BuildIndividualTestersUpdateResult{
		BuildID:   "BUILD_456",
		TesterIDs: []string{"TESTER_1", "TESTER_2"},
		Action:    "added",
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Tester IDs") {
		t.Fatalf("expected tester IDs header, got: %s", output)
	}
	if !strings.Contains(output, "TESTER_1, TESTER_2") {
		t.Fatalf("expected tester IDs in output, got: %s", output)
	}
}

func TestPrintMarkdown_BuildIndividualTestersUpdateResult(t *testing.T) {
	resp := &BuildIndividualTestersUpdateResult{
		BuildID:   "BUILD_456",
		TesterIDs: []string{"TESTER_1", "TESTER_2"},
		Action:    "removed",
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| Build ID | Tester IDs | Action |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "TESTER_1, TESTER_2") {
		t.Fatalf("expected tester IDs in output, got: %s", output)
	}
}

func TestPrintTable_BuildUploadDeleteResult(t *testing.T) {
	result := &BuildUploadDeleteResult{
		ID:      "upload-1",
		Deleted: true,
	}

	output := captureStdout(t, func() error {
		return PrintTable(result)
	})

	if !strings.Contains(output, "Deleted") {
		t.Fatalf("expected deleted header, got: %s", output)
	}
	if !strings.Contains(output, "upload-1") {
		t.Fatalf("expected upload ID in output, got: %s", output)
	}
}

func TestPrintMarkdown_BuildUploadDeleteResult(t *testing.T) {
	result := &BuildUploadDeleteResult{
		ID:      "upload-2",
		Deleted: true,
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(result)
	})

	if !strings.Contains(output, "| ID | Deleted |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "upload-2") {
		t.Fatalf("expected upload ID in output, got: %s", output)
	}
}

func TestPrintTable_PromotedPurchaseDeleteResult(t *testing.T) {
	result := &PromotedPurchaseDeleteResult{
		ID:      "promo-1",
		Deleted: true,
	}

	output := captureStdout(t, func() error {
		return PrintTable(result)
	})

	if !strings.Contains(output, "Deleted") {
		t.Fatalf("expected deleted header, got: %s", output)
	}
	if !strings.Contains(output, "promo-1") {
		t.Fatalf("expected promoted purchase ID in output, got: %s", output)
	}
}

func TestPrintMarkdown_PromotedPurchaseDeleteResult(t *testing.T) {
	result := &PromotedPurchaseDeleteResult{
		ID:      "promo-2",
		Deleted: true,
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(result)
	})

	if !strings.Contains(output, "| ID | Deleted |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "promo-2") {
		t.Fatalf("expected promoted purchase ID in output, got: %s", output)
	}
}

func TestPrintTable_AppPromotedPurchasesLinkResult(t *testing.T) {
	result := &AppPromotedPurchasesLinkResult{
		AppID:               "app-1",
		PromotedPurchaseIDs: []string{"promo-1", "promo-2"},
		Action:              "linked",
	}

	output := captureStdout(t, func() error {
		return PrintTable(result)
	})

	if !strings.Contains(output, "Promoted Purchase IDs") {
		t.Fatalf("expected promoted purchase IDs header, got: %s", output)
	}
	if !strings.Contains(output, "promo-1, promo-2") {
		t.Fatalf("expected promoted purchase IDs in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppPromotedPurchasesLinkResult(t *testing.T) {
	result := &AppPromotedPurchasesLinkResult{
		AppID:               "app-1",
		PromotedPurchaseIDs: []string{"promo-1", "promo-2"},
		Action:              "linked",
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(result)
	})

	if !strings.Contains(output, "| App ID | Promoted Purchase IDs | Action |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "promo-1, promo-2") {
		t.Fatalf("expected promoted purchase IDs in output, got: %s", output)
	}
}

func TestPrintTable_SalesReportResult(t *testing.T) {
	result := &SalesReportResult{
		VendorNumber:  "12345678",
		ReportType:    "SALES",
		ReportSubType: "SUMMARY",
		Frequency:     "DAILY",
		ReportDate:    "2024-01-20",
		Version:       "1_0",
		FilePath:      "sales_report_2024-01-20_SALES.tsv.gz",
		FileSize:      1234,
		Decompressed:  false,
	}

	output := captureStdout(t, func() error {
		return PrintTable(result)
	})

	if !strings.Contains(output, "Vendor") {
		t.Fatalf("expected vendor header in output, got: %s", output)
	}
	if !strings.Contains(output, "sales_report_2024-01-20_SALES.tsv.gz") {
		t.Fatalf("expected file path in output, got: %s", output)
	}
}

func TestPrintTable_FinanceReportResult(t *testing.T) {
	result := &FinanceReportResult{
		VendorNumber: "12345678",
		ReportType:   "FINANCIAL",
		RegionCode:   "US",
		ReportDate:   "2025-12",
		FilePath:     "finance_report_2025-12_FINANCIAL_US.tsv.gz",
		Bytes:        2048,
	}

	output := captureStdout(t, func() error {
		return PrintTable(result)
	})

	if !strings.Contains(output, "Region") {
		t.Fatalf("expected region header in output, got: %s", output)
	}
	if !strings.Contains(output, "finance_report_2025-12_FINANCIAL_US.tsv.gz") {
		t.Fatalf("expected file path in output, got: %s", output)
	}
}

func TestPrintMarkdown_FinanceReportResult(t *testing.T) {
	result := &FinanceReportResult{
		VendorNumber: "12345678",
		ReportType:   "FINANCE_DETAIL",
		RegionCode:   "Z1",
		ReportDate:   "2025-12",
		FilePath:     "finance_report_2025-12_FINANCE_DETAIL_Z1.tsv.gz",
		Bytes:        2048,
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(result)
	})

	if !strings.Contains(output, "| Vendor | Type | Region |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "finance_report_2025-12_FINANCE_DETAIL_Z1.tsv.gz") {
		t.Fatalf("expected file path in output, got: %s", output)
	}
}

func TestPrintTable_FinanceRegionsResult(t *testing.T) {
	result := &FinanceRegionsResult{
		Regions: []FinanceRegion{
			{
				ReportRegion:       "Americas",
				ReportCurrency:     "USD",
				RegionCode:         "US",
				CountriesOrRegions: "United States",
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(result)
	})

	if !strings.Contains(output, "Region") {
		t.Fatalf("expected region header in output, got: %s", output)
	}
	if !strings.Contains(output, "United States") {
		t.Fatalf("expected country in output, got: %s", output)
	}
}

func TestPrintMarkdown_FinanceRegionsResult(t *testing.T) {
	result := &FinanceRegionsResult{
		Regions: []FinanceRegion{
			{
				ReportRegion:       "Americas",
				ReportCurrency:     "USD",
				RegionCode:         "US",
				CountriesOrRegions: "United States",
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(result)
	})

	if !strings.Contains(output, "| Region | Currency | Code | Countries or Regions |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "| Americas | USD | US | United States |") {
		t.Fatalf("expected region row, got: %s", output)
	}
}

func TestPrintMarkdown_AnalyticsReportRequestResult(t *testing.T) {
	result := &AnalyticsReportRequestResult{
		RequestID:   "req-1",
		AppID:       "app-1",
		AccessType:  "ONGOING",
		State:       "PROCESSING",
		CreatedDate: "2024-01-20T12:00:00Z",
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(result)
	})

	if !strings.Contains(output, "| Request ID |") {
		t.Fatalf("expected request header in output, got: %s", output)
	}
	if !strings.Contains(output, "req-1") {
		t.Fatalf("expected request ID in output, got: %s", output)
	}
}

func TestPrintTable_AnalyticsReportRequests(t *testing.T) {
	resp := &AnalyticsReportRequestsResponse{
		Data: []AnalyticsReportRequestResource{
			{
				ID: "req-1",
				Attributes: AnalyticsReportRequestAttributes{
					AccessType:  AnalyticsAccessTypeOngoing,
					State:       AnalyticsReportRequestStateProcessing,
					CreatedDate: "2024-01-20T12:00:00Z",
				},
				Relationships: &AnalyticsReportRequestRelationships{
					App: &Relationship{Data: ResourceData{Type: ResourceTypeApps, ID: "app-1"}},
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Access Type") {
		t.Fatalf("expected access type header in output, got: %s", output)
	}
	if !strings.Contains(output, "req-1") {
		t.Fatalf("expected request ID in output, got: %s", output)
	}
}

func TestPrintTable_SandboxTesters(t *testing.T) {
	resp := &SandboxTestersResponse{
		Data: []Resource[SandboxTesterAttributes]{
			{
				ID: "tester-1",
				Attributes: SandboxTesterAttributes{
					AccountName: "tester@example.com",
					FirstName:   "Test",
					LastName:    "User",
					Territory:   "USA",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Email") || !strings.Contains(output, "Territory") {
		t.Fatalf("expected sandbox tester headers, got: %s", output)
	}
	if !strings.Contains(output, "tester@example.com") {
		t.Fatalf("expected tester email in output, got: %s", output)
	}
}

func TestPrintTable_BetaTesterGroupsUpdateResult(t *testing.T) {
	result := &BetaTesterGroupsUpdateResult{
		TesterID: "tester-1",
		GroupIDs: []string{"group-1", "group-2"},
		Action:   "added",
	}

	output := captureStdout(t, func() error {
		return PrintTable(result)
	})

	if !strings.Contains(output, "Tester ID") || !strings.Contains(output, "Group IDs") {
		t.Fatalf("expected table headers, got: %s", output)
	}
	if !strings.Contains(output, "group-1,group-2") {
		t.Fatalf("expected group IDs in output, got: %s", output)
	}
}

func TestPrintMarkdown_BetaTesterGroupsUpdateResult(t *testing.T) {
	result := &BetaTesterGroupsUpdateResult{
		TesterID: "tester-1",
		GroupIDs: []string{"group-1", "group-2"},
		Action:   "removed",
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(result)
	})

	if !strings.Contains(output, "| Tester ID | Group IDs | Action |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "group-1,group-2") {
		t.Fatalf("expected group IDs in output, got: %s", output)
	}
}

func TestPrintTable_SandboxTesterClearHistoryResult(t *testing.T) {
	result := &SandboxTesterClearHistoryResult{
		RequestID: "request-1",
		TesterID:  "tester-1",
		Cleared:   true,
	}

	output := captureStdout(t, func() error {
		return PrintTable(result)
	})

	if !strings.Contains(output, "Request ID") {
		t.Fatalf("expected request header, got: %s", output)
	}
	if !strings.Contains(output, "tester-1") {
		t.Fatalf("expected tester ID in output, got: %s", output)
	}
}

func TestPrintMarkdown_SandboxTesterClearHistoryResult(t *testing.T) {
	result := &SandboxTesterClearHistoryResult{
		RequestID: "request-1",
		TesterID:  "tester-1",
		Cleared:   true,
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(result)
	})

	if !strings.Contains(output, "| Request ID | Tester ID | Cleared |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "request-1") {
		t.Fatalf("expected request ID in output, got: %s", output)
	}
}

func TestPrintTable_Devices(t *testing.T) {
	resp := &DevicesResponse{
		Data: []Resource[DeviceAttributes]{
			{
				ID: "device-1",
				Attributes: DeviceAttributes{
					Name:     "My iPhone",
					UDID:     "UDID-1",
					Platform: DevicePlatformIOS,
					Status:   DeviceStatusEnabled,
					Model:    "iPhone15,3",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "UDID") {
		t.Fatalf("expected UDID header in output, got: %s", output)
	}
	if !strings.Contains(output, "My iPhone") {
		t.Fatalf("expected device name in output, got: %s", output)
	}
}

func TestPrintMarkdown_Devices(t *testing.T) {
	resp := &DevicesResponse{
		Data: []Resource[DeviceAttributes]{
			{
				ID: "device-1",
				Attributes: DeviceAttributes{
					Name:     "My iPhone",
					UDID:     "UDID-1",
					Platform: DevicePlatformIOS,
					Status:   DeviceStatusEnabled,
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| ID | Name | UDID | Platform | Status |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "UDID-1") {
		t.Fatalf("expected UDID in output, got: %s", output)
	}
}

func TestPrintTable_AccessibilityDeclarations(t *testing.T) {
	resp := &AccessibilityDeclarationsResponse{
		Data: []Resource[AccessibilityDeclarationAttributes]{
			{
				ID: "decl-1",
				Attributes: AccessibilityDeclarationAttributes{
					DeviceFamily: DeviceFamilyIPhone,
					State:        AccessibilityDeclarationStateDraft,
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Device Family") {
		t.Fatalf("expected device family header in output, got: %s", output)
	}
	if !strings.Contains(output, "IPHONE") {
		t.Fatalf("expected device family in output, got: %s", output)
	}
}

func TestPrintMarkdown_AccessibilityDeclaration(t *testing.T) {
	supportsVoiceover := true
	resp := &AccessibilityDeclarationResponse{
		Data: Resource[AccessibilityDeclarationAttributes]{
			ID:   "decl-1",
			Type: ResourceTypeAccessibilityDeclarations,
			Attributes: AccessibilityDeclarationAttributes{
				DeviceFamily:      DeviceFamilyIPhone,
				SupportsVoiceover: &supportsVoiceover,
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| Field | Value |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "Supports Voiceover") {
		t.Fatalf("expected voiceover field in output, got: %s", output)
	}
}

func TestPrintTable_AppStoreReviewDetail(t *testing.T) {
	resp := &AppStoreReviewDetailResponse{
		Data: Resource[AppStoreReviewDetailAttributes]{
			ID: "detail-1",
			Attributes: AppStoreReviewDetailAttributes{
				ContactFirstName:    "Dev",
				ContactLastName:     "Example",
				ContactEmail:        "dev@example.com",
				ContactPhone:        "123-456-7890",
				DemoAccountName:     "demo",
				DemoAccountRequired: true,
				Notes:               "Review notes",
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Contact") {
		t.Fatalf("expected Contact header in output, got: %s", output)
	}
	if !strings.Contains(output, "dev@example.com") {
		t.Fatalf("expected contact email in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppStoreReviewDetail(t *testing.T) {
	resp := &AppStoreReviewDetailResponse{
		Data: Resource[AppStoreReviewDetailAttributes]{
			ID: "detail-1",
			Attributes: AppStoreReviewDetailAttributes{
				ContactFirstName:    "Dev",
				ContactLastName:     "Example",
				ContactEmail:        "dev@example.com",
				ContactPhone:        "123-456-7890",
				DemoAccountName:     "demo",
				DemoAccountRequired: true,
				Notes:               "Review notes",
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| ID | Contact | Email |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "Dev Example") {
		t.Fatalf("expected contact name in output, got: %s", output)
	}
}

func TestPrintTable_AppStoreReviewAttachments(t *testing.T) {
	state := "UPLOADED"
	resp := &AppStoreReviewAttachmentsResponse{
		Data: []Resource[AppStoreReviewAttachmentAttributes]{
			{
				ID: "attach-1",
				Attributes: AppStoreReviewAttachmentAttributes{
					FileName:           "review.pdf",
					FileSize:           1024,
					SourceFileChecksum: "abcd1234",
					AssetDeliveryState: &AppMediaAssetState{State: &state},
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "File Name") {
		t.Fatalf("expected file name header in output, got: %s", output)
	}
	if !strings.Contains(output, "review.pdf") {
		t.Fatalf("expected file name in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppStoreReviewAttachment(t *testing.T) {
	state := "UPLOADED"
	resp := &AppStoreReviewAttachmentResponse{
		Data: Resource[AppStoreReviewAttachmentAttributes]{
			ID:   "attach-1",
			Type: ResourceTypeAppStoreReviewAttachments,
			Attributes: AppStoreReviewAttachmentAttributes{
				FileName:           "review.pdf",
				FileSize:           2048,
				SourceFileChecksum: "abcd1234",
				AssetDeliveryState: &AppMediaAssetState{State: &state},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| Field | Value |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "File Name") {
		t.Fatalf("expected file name field in output, got: %s", output)
	}
}

func TestPrintTable_AppStoreReviewAttachmentDeleteResult(t *testing.T) {
	result := &AppStoreReviewAttachmentDeleteResult{
		ID:      "attach-1",
		Deleted: true,
	}

	output := captureStdout(t, func() error {
		return PrintTable(result)
	})

	if !strings.Contains(output, "Deleted") {
		t.Fatalf("expected Deleted header in output, got: %s", output)
	}
	if !strings.Contains(output, "attach-1") {
		t.Fatalf("expected id in output, got: %s", output)
	}
}

func TestPrintTable_EndAppAvailabilityPreOrder(t *testing.T) {
	resp := &EndAppAvailabilityPreOrderResponse{
		Data: Resource[EndAppAvailabilityPreOrderAttributes]{
			Type: ResourceTypeEndAppAvailabilityPreOrders,
			ID:   "end-1",
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "ID") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "end-1") {
		t.Fatalf("expected id in output, got: %s", output)
	}
}

func TestPrintMarkdown_EndAppAvailabilityPreOrder(t *testing.T) {
	resp := &EndAppAvailabilityPreOrderResponse{
		Data: Resource[EndAppAvailabilityPreOrderAttributes]{
			Type: ResourceTypeEndAppAvailabilityPreOrders,
			ID:   "end-1",
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| ID |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "end-1") {
		t.Fatalf("expected id in output, got: %s", output)
	}
}

func TestPrintTable_RoutingAppCoverage(t *testing.T) {
	state := "COMPLETE"
	resp := &RoutingAppCoverageResponse{
		Data: Resource[RoutingAppCoverageAttributes]{
			ID:   "cover-1",
			Type: ResourceTypeRoutingAppCoverages,
			Attributes: RoutingAppCoverageAttributes{
				FileName:           "coverage.geojson",
				FileSize:           2048,
				SourceFileChecksum: "abcd1234",
				AssetDeliveryState: &AppMediaAssetState{State: &state},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "File Name") {
		t.Fatalf("expected file name header in output, got: %s", output)
	}
	if !strings.Contains(output, "coverage.geojson") {
		t.Fatalf("expected file name in output, got: %s", output)
	}
}

func TestPrintMarkdown_RoutingAppCoverage(t *testing.T) {
	state := "COMPLETE"
	resp := &RoutingAppCoverageResponse{
		Data: Resource[RoutingAppCoverageAttributes]{
			ID:   "cover-1",
			Type: ResourceTypeRoutingAppCoverages,
			Attributes: RoutingAppCoverageAttributes{
				FileName:           "coverage.geojson",
				FileSize:           2048,
				SourceFileChecksum: "abcd1234",
				AssetDeliveryState: &AppMediaAssetState{State: &state},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| Field | Value |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "File Name") {
		t.Fatalf("expected file name field in output, got: %s", output)
	}
}

func TestPrintTable_RoutingAppCoverageDeleteResult(t *testing.T) {
	result := &RoutingAppCoverageDeleteResult{
		ID:      "cover-1",
		Deleted: true,
	}

	output := captureStdout(t, func() error {
		return PrintTable(result)
	})

	if !strings.Contains(output, "Deleted") {
		t.Fatalf("expected Deleted header in output, got: %s", output)
	}
	if !strings.Contains(output, "cover-1") {
		t.Fatalf("expected id in output, got: %s", output)
	}
}

func TestPrintTable_AppEncryptionDeclarations(t *testing.T) {
	exempt := true
	proprietary := false
	thirdParty := true
	french := true
	resp := &AppEncryptionDeclarationsResponse{
		Data: []Resource[AppEncryptionDeclarationAttributes]{
			{
				ID:   "decl-1",
				Type: ResourceTypeAppEncryptionDeclarations,
				Attributes: AppEncryptionDeclarationAttributes{
					AppEncryptionDeclarationState:   AppEncryptionDeclarationStateApproved,
					Exempt:                          &exempt,
					ContainsProprietaryCryptography: &proprietary,
					ContainsThirdPartyCryptography:  &thirdParty,
					AvailableOnFrenchStore:          &french,
					CreatedDate:                     "2026-01-28T00:00:00Z",
					CodeValue:                       "EI12345",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Proprietary Crypto") {
		t.Fatalf("expected proprietary crypto header in output, got: %s", output)
	}
	if !strings.Contains(output, "APPROVED") {
		t.Fatalf("expected state in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppEncryptionDeclaration(t *testing.T) {
	exempt := false
	resp := &AppEncryptionDeclarationResponse{
		Data: Resource[AppEncryptionDeclarationAttributes]{
			ID:   "decl-1",
			Type: ResourceTypeAppEncryptionDeclarations,
			Attributes: AppEncryptionDeclarationAttributes{
				AppDescription:                 "Uses TLS",
				Exempt:                         &exempt,
				AppEncryptionDeclarationState:  AppEncryptionDeclarationStateCreated,
				ContainsThirdPartyCryptography: &exempt,
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| Field | Value |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "App Description") {
		t.Fatalf("expected app description field in output, got: %s", output)
	}
}

func TestPrintTable_AppEncryptionDeclarationDocument(t *testing.T) {
	state := "COMPLETE"
	resp := &AppEncryptionDeclarationDocumentResponse{
		Data: Resource[AppEncryptionDeclarationDocumentAttributes]{
			ID:   "doc-1",
			Type: ResourceTypeAppEncryptionDeclarationDocuments,
			Attributes: AppEncryptionDeclarationDocumentAttributes{
				FileName:           "export.pdf",
				FileSize:           2048,
				SourceFileChecksum: "abcd1234",
				AssetDeliveryState: &AppMediaAssetState{State: &state},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "File Name") {
		t.Fatalf("expected file name header in output, got: %s", output)
	}
	if !strings.Contains(output, "export.pdf") {
		t.Fatalf("expected file name in output, got: %s", output)
	}
}

func TestPrintTable_AppEncryptionDeclarationBuildsUpdateResult(t *testing.T) {
	result := &AppEncryptionDeclarationBuildsUpdateResult{
		DeclarationID: "decl-1",
		BuildIDs:      []string{"build-1", "build-2"},
		Action:        "assigned",
	}

	output := captureStdout(t, func() error {
		return PrintTable(result)
	})

	if !strings.Contains(output, "Declaration ID") {
		t.Fatalf("expected declaration id header in output, got: %s", output)
	}
	if !strings.Contains(output, "decl-1") {
		t.Fatalf("expected declaration id in output, got: %s", output)
	}
}

func TestPrintTable_PerfPowerMetrics(t *testing.T) {
	resp := &PerfPowerMetricsResponse{
		Data: json.RawMessage(`{"version":"1","insights":{"trendingUp":[{"metric":"cpu"}],"regressions":[{}]},"productData":[{},{}]}`),
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Trending Up") {
		t.Fatalf("expected trending up header in output, got: %s", output)
	}
	if !strings.Contains(output, "2") {
		t.Fatalf("expected product count in output, got: %s", output)
	}
}

func TestPrintMarkdown_PerfPowerMetrics(t *testing.T) {
	resp := &PerfPowerMetricsResponse{
		Data: json.RawMessage(`{"version":"1","insights":{"trendingUp":[],"regressions":[{}]},"productData":[{}]}`),
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| Version | Products |") {
		t.Fatalf("expected markdown header in output, got: %s", output)
	}
	if !strings.Contains(output, "| 1 | 1 |") {
		t.Fatalf("expected summary values in output, got: %s", output)
	}
}

func TestPrintTable_DiagnosticSignatures(t *testing.T) {
	resp := &DiagnosticSignaturesResponse{
		Data: []Resource[DiagnosticSignatureAttributes]{
			{
				ID:   "diag-1",
				Type: ResourceTypeDiagnosticSignatures,
				Attributes: DiagnosticSignatureAttributes{
					DiagnosticType: DiagnosticSignatureTypeHangs,
					Signature:      "main",
					Weight:         0.75,
					Insight: &DiagnosticInsight{
						Direction: DiagnosticInsightDirectionUp,
					},
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Signature") {
		t.Fatalf("expected signature header in output, got: %s", output)
	}
	if !strings.Contains(output, "HANGS") {
		t.Fatalf("expected diagnostic type in output, got: %s", output)
	}
}

func TestPrintMarkdown_DiagnosticLogs(t *testing.T) {
	resp := &DiagnosticLogsResponse{
		Data: json.RawMessage(`{"version":"1","productData":[{"diagnosticLogs":[{},{}],"diagnosticInsights":[{}]},{"diagnosticLogs":[{}]}]}`),
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| Version | Products | Logs | Insights |") {
		t.Fatalf("expected markdown header in output, got: %s", output)
	}
	if !strings.Contains(output, "| 1 | 2 | 3 | 1 |") {
		t.Fatalf("expected summary values in output, got: %s", output)
	}
}

func TestPrintTable_PerformanceDownloadResult(t *testing.T) {
	result := &PerformanceDownloadResult{
		DownloadType: "metrics",
		AppID:        "app-1",
		FilePath:     "metrics.json",
		FileSize:     1024,
		Decompressed: false,
	}

	output := captureStdout(t, func() error {
		return PrintTable(result)
	})

	if !strings.Contains(output, "Compressed File") {
		t.Fatalf("expected compressed file header in output, got: %s", output)
	}
	if !strings.Contains(output, "metrics.json") {
		t.Fatalf("expected file path in output, got: %s", output)
	}
}

func TestPrintTable_MarketplaceSearchDetail(t *testing.T) {
	resp := &MarketplaceSearchDetailResponse{
		Data: Resource[MarketplaceSearchDetailAttributes]{
			ID: "detail-1",
			Attributes: MarketplaceSearchDetailAttributes{
				CatalogURL: "https://example.com/catalog",
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Catalog URL") {
		t.Fatalf("expected catalog url header, got: %s", output)
	}
	if !strings.Contains(output, "https://example.com/catalog") {
		t.Fatalf("expected catalog url in output, got: %s", output)
	}
}

func TestPrintMarkdown_MarketplaceSearchDetail(t *testing.T) {
	resp := &MarketplaceSearchDetailResponse{
		Data: Resource[MarketplaceSearchDetailAttributes]{
			ID: "detail-1",
			Attributes: MarketplaceSearchDetailAttributes{
				CatalogURL: "https://example.com/catalog",
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| ID | Catalog URL |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "detail-1") {
		t.Fatalf("expected search detail id in output, got: %s", output)
	}
}

func TestPrintTable_MarketplaceWebhooks(t *testing.T) {
	resp := &MarketplaceWebhooksResponse{
		Data: []Resource[MarketplaceWebhookAttributes]{
			{
				ID: "webhook-1",
				Attributes: MarketplaceWebhookAttributes{
					EndpointURL: "https://example.com/webhook",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Endpoint URL") {
		t.Fatalf("expected endpoint url header, got: %s", output)
	}
	if !strings.Contains(output, "https://example.com/webhook") {
		t.Fatalf("expected endpoint url in output, got: %s", output)
	}
}

func TestPrintMarkdown_MarketplaceWebhooks(t *testing.T) {
	resp := &MarketplaceWebhooksResponse{
		Data: []Resource[MarketplaceWebhookAttributes]{
			{
				ID: "webhook-1",
				Attributes: MarketplaceWebhookAttributes{
					EndpointURL: "https://example.com/webhook",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| ID | Endpoint URL |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "webhook-1") {
		t.Fatalf("expected webhook id in output, got: %s", output)
	}
}

func TestPrintTable_AndroidToIosAppMappingDetails(t *testing.T) {
	resp := &AndroidToIosAppMappingDetailsResponse{
		Data: []Resource[AndroidToIosAppMappingDetailAttributes]{
			{
				ID:   "map-1",
				Type: ResourceTypeAndroidToIosAppMappingDetails,
				Attributes: AndroidToIosAppMappingDetailAttributes{
					PackageName: "com.example.android",
					AppSigningKeyPublicCertificateSha256Fingerprints: []string{"sha1", "sha2"},
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Package Name") {
		t.Fatalf("expected package name header, got: %s", output)
	}
	if !strings.Contains(output, "com.example.android") {
		t.Fatalf("expected package name value, got: %s", output)
	}
}

func TestPrintMarkdown_AndroidToIosAppMappingDetail(t *testing.T) {
	resp := &AndroidToIosAppMappingDetailResponse{
		Data: Resource[AndroidToIosAppMappingDetailAttributes]{
			ID:   "map-1",
			Type: ResourceTypeAndroidToIosAppMappingDetails,
			Attributes: AndroidToIosAppMappingDetailAttributes{
				PackageName: "com.example.android",
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| Package Name |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "com.example.android") {
		t.Fatalf("expected package name value, got: %s", output)
	}
}

func TestPrintTable_MarketplaceSearchDetailDeleteResult(t *testing.T) {
	result := &MarketplaceSearchDetailDeleteResult{
		ID:      "detail-1",
		Deleted: true,
	}

	output := captureStdout(t, func() error {
		return PrintTable(result)
	})

	if !strings.Contains(output, "Deleted") {
		t.Fatalf("expected deleted header, got: %s", output)
	}
	if !strings.Contains(output, "detail-1") {
		t.Fatalf("expected search detail id in output, got: %s", output)
	}
}

func TestPrintTable_AndroidToIosAppMappingDeleteResult(t *testing.T) {
	result := &AndroidToIosAppMappingDeleteResult{
		ID:      "map-1",
		Deleted: true,
	}

	output := captureStdout(t, func() error {
		return PrintTable(result)
	})

	if !strings.Contains(output, "Deleted") {
		t.Fatalf("expected deleted header, got: %s", output)
	}
	if !strings.Contains(output, "map-1") {
		t.Fatalf("expected mapping id, got: %s", output)
	}
}

func TestPrintMarkdown_MarketplaceWebhookDeleteResult(t *testing.T) {
	result := &MarketplaceWebhookDeleteResult{
		ID:      "webhook-1",
		Deleted: true,
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(result)
	})

	if !strings.Contains(output, "| ID | Deleted |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "webhook-1") {
		t.Fatalf("expected webhook id in output, got: %s", output)
	}
}

func TestPrintTable_AppEvents(t *testing.T) {
	resp := &AppEventsResponse{
		Data: []Resource[AppEventAttributes]{
			{
				ID: "event-1",
				Attributes: AppEventAttributes{
					ReferenceName: "Summer Challenge",
					Badge:         "CHALLENGE",
					EventState:    "DRAFT",
					Priority:      "HIGH",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Reference Name") {
		t.Fatalf("expected app events header, got: %s", output)
	}
	if !strings.Contains(output, "event-1") {
		t.Fatalf("expected event id in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppEvents(t *testing.T) {
	resp := &AppEventsResponse{
		Data: []Resource[AppEventAttributes]{
			{
				ID: "event-2",
				Attributes: AppEventAttributes{
					ReferenceName: "Launch Party",
					Badge:         "PREMIERE",
					EventState:    "READY_FOR_REVIEW",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| ID | Reference Name |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "Launch Party") {
		t.Fatalf("expected event name in output, got: %s", output)
	}
}

func TestPrintTable_AppEvents_Empty(t *testing.T) {
	resp := &AppEventsResponse{Data: []Resource[AppEventAttributes]{}}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Reference Name") {
		t.Fatalf("expected app events header, got: %s", output)
	}
}

func TestPrintMarkdown_AppEvents_Empty(t *testing.T) {
	resp := &AppEventsResponse{Data: []Resource[AppEventAttributes]{}}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| ID | Reference Name |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
}

func TestPrintTable_AppEventLocalizations(t *testing.T) {
	resp := &AppEventLocalizationsResponse{
		Data: []Resource[AppEventLocalizationAttributes]{
			{
				ID: "loc-1",
				Attributes: AppEventLocalizationAttributes{
					Locale:           "en-US",
					Name:             "Summer Challenge",
					ShortDescription: "Compete this week",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Short Description") {
		t.Fatalf("expected localization header, got: %s", output)
	}
	if !strings.Contains(output, "en-US") {
		t.Fatalf("expected locale in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppEventLocalizations(t *testing.T) {
	resp := &AppEventLocalizationsResponse{
		Data: []Resource[AppEventLocalizationAttributes]{
			{
				ID: "loc-2",
				Attributes: AppEventLocalizationAttributes{
					Locale:           "fr-FR",
					Name:             "Evenement",
					ShortDescription: "Court",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| ID | Locale |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "fr-FR") {
		t.Fatalf("expected locale in output, got: %s", output)
	}
}

func TestPrintTable_AppEventLocalizations_Empty(t *testing.T) {
	resp := &AppEventLocalizationsResponse{Data: []Resource[AppEventLocalizationAttributes]{}}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Short Description") {
		t.Fatalf("expected localization header, got: %s", output)
	}
}

func TestPrintMarkdown_AppEventLocalizations_Empty(t *testing.T) {
	resp := &AppEventLocalizationsResponse{Data: []Resource[AppEventLocalizationAttributes]{}}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| ID | Locale |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
}

func TestPrintMarkdown_AppEventScreenshots(t *testing.T) {
	state := "COMPLETE"
	resp := &AppEventScreenshotsResponse{
		Data: []Resource[AppEventScreenshotAttributes]{
			{
				ID: "shot-1",
				Attributes: AppEventScreenshotAttributes{
					FileName:           "event.png",
					FileSize:           1024,
					AppEventAssetType:  "EVENT_CARD",
					AssetDeliveryState: &AppMediaAssetState{State: &state},
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| ID | File Name |") {
		t.Fatalf("expected screenshots markdown header, got: %s", output)
	}
	if !strings.Contains(output, "event.png") {
		t.Fatalf("expected file name in output, got: %s", output)
	}
}

func TestPrintTable_AppEventScreenshots(t *testing.T) {
	state := "COMPLETE"
	resp := &AppEventScreenshotsResponse{
		Data: []Resource[AppEventScreenshotAttributes]{
			{
				ID: "shot-2",
				Attributes: AppEventScreenshotAttributes{
					FileName:           "event2.png",
					FileSize:           2048,
					AppEventAssetType:  "EVENT_DETAILS_PAGE",
					AssetDeliveryState: &AppMediaAssetState{State: &state},
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "File Name") {
		t.Fatalf("expected screenshots header, got: %s", output)
	}
	if !strings.Contains(output, "event2.png") {
		t.Fatalf("expected file name in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppEventScreenshots_Empty(t *testing.T) {
	resp := &AppEventScreenshotsResponse{Data: []Resource[AppEventScreenshotAttributes]{}}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| ID | File Name |") {
		t.Fatalf("expected screenshots markdown header, got: %s", output)
	}
}

func TestPrintTable_AppEventScreenshots_Empty(t *testing.T) {
	resp := &AppEventScreenshotsResponse{Data: []Resource[AppEventScreenshotAttributes]{}}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "File Name") {
		t.Fatalf("expected screenshots header, got: %s", output)
	}
}

func TestPrintTable_AppEventVideoClips(t *testing.T) {
	state := "COMPLETE"
	resp := &AppEventVideoClipsResponse{
		Data: []Resource[AppEventVideoClipAttributes]{
			{
				ID: "clip-1",
				Attributes: AppEventVideoClipAttributes{
					FileName:          "clip.mov",
					FileSize:          4096,
					AppEventAssetType: "EVENT_CARD",
					VideoDeliveryState: &AppMediaVideoState{
						State: &state,
					},
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "File Name") {
		t.Fatalf("expected video clips header, got: %s", output)
	}
	if !strings.Contains(output, "clip.mov") {
		t.Fatalf("expected file name in output, got: %s", output)
	}
}

func TestPrintTable_AppEventVideoClips_Empty(t *testing.T) {
	resp := &AppEventVideoClipsResponse{Data: []Resource[AppEventVideoClipAttributes]{}}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "File Name") {
		t.Fatalf("expected video clips header, got: %s", output)
	}
}

func TestPrintMarkdown_AppEventVideoClips(t *testing.T) {
	state := "COMPLETE"
	resp := &AppEventVideoClipsResponse{
		Data: []Resource[AppEventVideoClipAttributes]{
			{
				ID: "clip-2",
				Attributes: AppEventVideoClipAttributes{
					FileName:          "clip2.mov",
					FileSize:          1024,
					AppEventAssetType: "EVENT_DETAILS_PAGE",
					VideoDeliveryState: &AppMediaVideoState{
						State: &state,
					},
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| ID | File Name |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "clip2.mov") {
		t.Fatalf("expected file name in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppEventVideoClips_Empty(t *testing.T) {
	resp := &AppEventVideoClipsResponse{Data: []Resource[AppEventVideoClipAttributes]{}}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "| ID | File Name |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
}

func TestPrintTable_AppEventSubmissionResult(t *testing.T) {
	submittedDate := "2026-02-01T00:00:00Z"
	result := &AppEventSubmissionResult{
		SubmissionID:  "submit-1",
		ItemID:        "item-1",
		EventID:       "event-1",
		AppID:         "app-1",
		Platform:      "IOS",
		SubmittedDate: &submittedDate,
	}

	output := captureStdout(t, func() error {
		return PrintTable(result)
	})

	if !strings.Contains(output, "Submission ID") {
		t.Fatalf("expected submission header, got: %s", output)
	}
	if !strings.Contains(output, "event-1") {
		t.Fatalf("expected event id in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppEventSubmissionResult(t *testing.T) {
	submittedDate := "2026-02-01T00:00:00Z"
	result := &AppEventSubmissionResult{
		SubmissionID:  "submit-2",
		ItemID:        "item-2",
		EventID:       "event-2",
		AppID:         "app-2",
		Platform:      "IOS",
		SubmittedDate: &submittedDate,
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(result)
	})

	if !strings.Contains(output, "| Submission ID |") {
		t.Fatalf("expected submission markdown header, got: %s", output)
	}
	if !strings.Contains(output, "event-2") {
		t.Fatalf("expected event id in output, got: %s", output)
	}
}

func TestPrintTable_AppEventDeleteResult(t *testing.T) {
	result := &AppEventDeleteResult{ID: "event-3", Deleted: true}

	output := captureStdout(t, func() error {
		return PrintTable(result)
	})

	if !strings.Contains(output, "Deleted") {
		t.Fatalf("expected deleted header, got: %s", output)
	}
	if !strings.Contains(output, "event-3") {
		t.Fatalf("expected event id in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppEventDeleteResult(t *testing.T) {
	result := &AppEventDeleteResult{ID: "event-3", Deleted: true}

	output := captureStdout(t, func() error {
		return PrintMarkdown(result)
	})

	if !strings.Contains(output, "| ID | Deleted |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "event-3") {
		t.Fatalf("expected event id in output, got: %s", output)
	}
}

func TestPrintTable_AppEventLocalizationDeleteResult(t *testing.T) {
	result := &AppEventLocalizationDeleteResult{ID: "loc-3", Deleted: true}

	output := captureStdout(t, func() error {
		return PrintTable(result)
	})

	if !strings.Contains(output, "Deleted") {
		t.Fatalf("expected deleted header, got: %s", output)
	}
	if !strings.Contains(output, "loc-3") {
		t.Fatalf("expected localization id in output, got: %s", output)
	}
}

func TestPrintMarkdown_AppEventLocalizationDeleteResult(t *testing.T) {
	result := &AppEventLocalizationDeleteResult{ID: "loc-3", Deleted: true}

	output := captureStdout(t, func() error {
		return PrintMarkdown(result)
	})

	if !strings.Contains(output, "| ID | Deleted |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "loc-3") {
		t.Fatalf("expected localization id in output, got: %s", output)
	}
}
