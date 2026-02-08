package asc

import (
	"strings"
	"testing"
)

func TestPrintTable_AppClips(t *testing.T) {
	resp := &AppClipsResponse{
		Data: []Resource[AppClipAttributes]{
			{ID: "clip-1", Attributes: AppClipAttributes{BundleID: "com.example.clip"}},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Bundle ID") || !strings.Contains(output, "com.example.clip") {
		t.Fatalf("expected bundle id in output, got %q", output)
	}
}

func TestPrintMarkdown_AppClips(t *testing.T) {
	resp := &AppClipsResponse{
		Data: []Resource[AppClipAttributes]{
			{ID: "clip-1", Attributes: AppClipAttributes{BundleID: "com.example.clip"}},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "Bundle ID") || !strings.Contains(output, "com.example.clip") {
		t.Fatalf("expected markdown bundle id, got %q", output)
	}
}

func TestPrintTable_AppClips_Empty(t *testing.T) {
	resp := &AppClipsResponse{Data: []Resource[AppClipAttributes]{}}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Bundle ID") {
		t.Fatalf("expected table header, got %q", output)
	}
}

func TestPrintMarkdown_AppClips_Empty(t *testing.T) {
	resp := &AppClipsResponse{Data: []Resource[AppClipAttributes]{}}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "Bundle ID") {
		t.Fatalf("expected markdown header, got %q", output)
	}
}

func TestPrintTable_AppClipDefaultExperiences(t *testing.T) {
	resp := &AppClipDefaultExperiencesResponse{
		Data: []Resource[AppClipDefaultExperienceAttributes]{
			{ID: "exp-1", Attributes: AppClipDefaultExperienceAttributes{Action: AppClipActionOpen}},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Action") || !strings.Contains(output, "OPEN") {
		t.Fatalf("expected action in output, got %q", output)
	}
}

func TestPrintTable_AppClipDefaultExperiences_Empty(t *testing.T) {
	resp := &AppClipDefaultExperiencesResponse{Data: []Resource[AppClipDefaultExperienceAttributes]{}}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Action") {
		t.Fatalf("expected table header, got %q", output)
	}
}

func TestPrintMarkdown_AppClipDefaultExperienceLocalizations(t *testing.T) {
	resp := &AppClipDefaultExperienceLocalizationsResponse{
		Data: []Resource[AppClipDefaultExperienceLocalizationAttributes]{
			{ID: "loc-1", Attributes: AppClipDefaultExperienceLocalizationAttributes{Locale: "en-US", Subtitle: "Try it"}},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "Locale") || !strings.Contains(output, "en-US") {
		t.Fatalf("expected locale in output, got %q", output)
	}
}

func TestPrintMarkdown_AppClipDefaultExperienceLocalizations_Empty(t *testing.T) {
	resp := &AppClipDefaultExperienceLocalizationsResponse{Data: []Resource[AppClipDefaultExperienceLocalizationAttributes]{}}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "Locale") {
		t.Fatalf("expected markdown header, got %q", output)
	}
}

func TestPrintTable_AppClipAdvancedExperiences(t *testing.T) {
	resp := &AppClipAdvancedExperiencesResponse{
		Data: []Resource[AppClipAdvancedExperienceAttributes]{
			{
				ID: "adv-1",
				Attributes: AppClipAdvancedExperienceAttributes{
					Action:           AppClipActionPlay,
					Status:           "ACTIVE",
					BusinessCategory: AppClipAdvancedExperienceBusinessCategoryFoodAndDrink,
					DefaultLanguage:  AppClipAdvancedExperienceLanguageEN,
					IsPoweredBy:      true,
					Link:             "https://example.com",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "FOOD_AND_DRINK") || !strings.Contains(output, "https://example.com") {
		t.Fatalf("expected advanced experience fields in output, got %q", output)
	}
}

func TestPrintTable_AppClipAdvancedExperiences_Empty(t *testing.T) {
	resp := &AppClipAdvancedExperiencesResponse{Data: []Resource[AppClipAdvancedExperienceAttributes]{}}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Business Category") {
		t.Fatalf("expected table header, got %q", output)
	}
}

func TestPrintMarkdown_BetaAppClipInvocationLocalizations(t *testing.T) {
	resp := &BetaAppClipInvocationLocalizationsResponse{
		Data: []Resource[BetaAppClipInvocationLocalizationAttributes]{
			{ID: "loc-1", Attributes: BetaAppClipInvocationLocalizationAttributes{Locale: "en-US", Title: "Try it"}},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "Title") || !strings.Contains(output, "Try it") {
		t.Fatalf("expected title in output, got %q", output)
	}
}

func TestPrintMarkdown_BetaAppClipInvocationLocalizations_Empty(t *testing.T) {
	resp := &BetaAppClipInvocationLocalizationsResponse{Data: []Resource[BetaAppClipInvocationLocalizationAttributes]{}}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "Title") {
		t.Fatalf("expected markdown header, got %q", output)
	}
}
