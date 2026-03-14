package shared

import (
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestMissingSubmitRequiredLocalizationFields_BaseFields(t *testing.T) {
	attrs := asc.AppStoreVersionLocalizationAttributes{
		Locale:      "en-US",
		Description: "A great app",
		Keywords:    "quran,islam",
		SupportURL:  "https://example.com",
	}
	missing := MissingSubmitRequiredLocalizationFields(attrs)
	if len(missing) != 0 {
		t.Fatalf("expected no missing fields, got %v", missing)
	}
}

func TestMissingSubmitRequiredLocalizationFields_AllEmpty(t *testing.T) {
	attrs := asc.AppStoreVersionLocalizationAttributes{Locale: "en-US"}
	missing := MissingSubmitRequiredLocalizationFields(attrs)
	want := []string{"description", "keywords", "supportUrl"}
	if len(missing) != len(want) {
		t.Fatalf("expected %v, got %v", want, missing)
	}
	for i, field := range want {
		if missing[i] != field {
			t.Fatalf("expected field %q at index %d, got %q", field, i, missing[i])
		}
	}
}

func TestMissingSubmitRequiredLocalizationFields_DoesNotCheckWhatsNew(t *testing.T) {
	attrs := asc.AppStoreVersionLocalizationAttributes{
		Locale:      "en-US",
		Description: "A great app",
		Keywords:    "quran,islam",
		SupportURL:  "https://example.com",
		WhatsNew:    "", // empty but should not be flagged without RequireWhatsNew
	}
	missing := MissingSubmitRequiredLocalizationFields(attrs)
	if len(missing) != 0 {
		t.Fatalf("expected no missing fields without RequireWhatsNew, got %v", missing)
	}
}

func TestMissingSubmitRequiredLocalizationFieldsWithOptions_WhatsNewRequired(t *testing.T) {
	attrs := asc.AppStoreVersionLocalizationAttributes{
		Locale:      "en-US",
		Description: "A great app",
		Keywords:    "quran,islam",
		SupportURL:  "https://example.com",
		WhatsNew:    "",
	}
	opts := SubmitReadinessOptions{RequireWhatsNew: true}
	missing := MissingSubmitRequiredLocalizationFieldsWithOptions(attrs, opts)
	if len(missing) != 1 || missing[0] != "whatsNew" {
		t.Fatalf("expected [whatsNew], got %v", missing)
	}
}

func TestMissingSubmitRequiredLocalizationFieldsWithOptions_WhatsNewPresent(t *testing.T) {
	attrs := asc.AppStoreVersionLocalizationAttributes{
		Locale:      "en-US",
		Description: "A great app",
		Keywords:    "quran,islam",
		SupportURL:  "https://example.com",
		WhatsNew:    "Bug fixes and improvements",
	}
	opts := SubmitReadinessOptions{RequireWhatsNew: true}
	missing := MissingSubmitRequiredLocalizationFieldsWithOptions(attrs, opts)
	if len(missing) != 0 {
		t.Fatalf("expected no missing fields, got %v", missing)
	}
}

func TestSubmitReadinessIssuesByLocaleWithOptions_WhatsNewMixedLocales(t *testing.T) {
	localizations := []asc.Resource[asc.AppStoreVersionLocalizationAttributes]{
		{
			ID: "loc-1",
			Attributes: asc.AppStoreVersionLocalizationAttributes{
				Locale:      "en-US",
				Description: "English description",
				Keywords:    "app,test",
				SupportURL:  "https://example.com",
				WhatsNew:    "Bug fixes",
			},
		},
		{
			ID: "loc-2",
			Attributes: asc.AppStoreVersionLocalizationAttributes{
				Locale:      "ar-SA",
				Description: "Arabic description",
				Keywords:    "تطبيق",
				SupportURL:  "https://example.com",
				WhatsNew:    "", // missing
			},
		},
		{
			ID: "loc-3",
			Attributes: asc.AppStoreVersionLocalizationAttributes{
				Locale:      "fr-FR",
				Description: "French description",
				Keywords:    "application",
				SupportURL:  "https://example.com",
				WhatsNew:    "  ", // whitespace-only
			},
		},
	}

	opts := SubmitReadinessOptions{RequireWhatsNew: true}
	issues := SubmitReadinessIssuesByLocaleWithOptions(localizations, opts)

	if len(issues) != 2 {
		t.Fatalf("expected 2 issues (ar-SA, fr-FR), got %d: %v", len(issues), issues)
	}

	// Issues should be sorted by locale
	if issues[0].Locale != "ar-SA" {
		t.Fatalf("expected first issue locale ar-SA, got %q", issues[0].Locale)
	}
	if issues[1].Locale != "fr-FR" {
		t.Fatalf("expected second issue locale fr-FR, got %q", issues[1].Locale)
	}

	for _, issue := range issues {
		if len(issue.MissingFields) != 1 || issue.MissingFields[0] != "whatsNew" {
			t.Fatalf("expected [whatsNew] for %s, got %v", issue.Locale, issue.MissingFields)
		}
	}
}

func TestSubmitReadinessIssuesByLocale_BackwardCompatible(t *testing.T) {
	localizations := []asc.Resource[asc.AppStoreVersionLocalizationAttributes]{
		{
			ID: "loc-1",
			Attributes: asc.AppStoreVersionLocalizationAttributes{
				Locale:      "en-US",
				Description: "desc",
				Keywords:    "kw",
				SupportURL:  "https://example.com",
				WhatsNew:    "", // empty but should not be flagged by default
			},
		},
	}

	issues := SubmitReadinessIssuesByLocale(localizations)
	if len(issues) != 0 {
		t.Fatalf("expected no issues from backward-compatible call, got %v", issues)
	}
}
