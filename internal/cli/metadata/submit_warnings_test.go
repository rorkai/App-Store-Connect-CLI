package metadata

import (
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestEffectiveVersionCreateLocalization_PrefersCreateLocalization(t *testing.T) {
	patch := versionLocalPatch{
		localization: VersionLocalization{
			Keywords: "one,two",
		},
		createLocalization: VersionLocalization{
			Description: "Description",
			Keywords:    "one,two",
			SupportURL:  "https://example.com/support",
		},
	}

	got := effectiveVersionCreateLocalization(patch)
	if got.Description != "Description" {
		t.Fatalf("expected create localization description, got %+v", got)
	}
	if got.SupportURL != "https://example.com/support" {
		t.Fatalf("expected create localization support URL, got %+v", got)
	}
}

func TestEffectiveVersionCreateLocalization_FallsBackToPatchLocalization(t *testing.T) {
	patch := versionLocalPatch{
		localization: VersionLocalization{
			Description: "Description",
			Keywords:    "one,two",
		},
	}

	got := effectiveVersionCreateLocalization(patch)
	if got.Description != "Description" || got.Keywords != "one,two" {
		t.Fatalf("expected fallback localization values, got %+v", got)
	}
}

func TestVersionCreateWarningForPatch_SkipsExistingRemoteLocale(t *testing.T) {
	patch := versionLocalPatch{
		localization: VersionLocalization{Keywords: "one,two"},
	}

	if _, ok := versionCreateWarningForPatch("fr-FR", patch, true, shared.SubmitReadinessCreateModePlanned, shared.SubmitReadinessOptions{}); ok {
		t.Fatal("expected no warning when remote locale already exists")
	}
}

func TestVersionCreateWarningForPatch_UsesEffectiveCreatePayload(t *testing.T) {
	patch := versionLocalPatch{
		localization: VersionLocalization{
			Keywords: "one,two",
		},
		createLocalization: VersionLocalization{
			Description: "Description",
			Keywords:    "one,two",
			SupportURL:  "https://example.com/support",
		},
	}

	if _, ok := versionCreateWarningForPatch("fr-FR", patch, false, shared.SubmitReadinessCreateModeApplied, shared.SubmitReadinessOptions{}); ok {
		t.Fatal("expected no warning when effective create payload is complete")
	}
}

func TestVersionCreateWarningsForPatches_ReturnsSortedWarnings(t *testing.T) {
	warnings := versionCreateWarningsForPatches(
		map[string]versionLocalPatch{
			"ja": {
				localization: VersionLocalization{Keywords: "nihongo"},
			},
			"en-US": {
				localization: VersionLocalization{
					Description: "Description",
					Keywords:    "english,keywords",
					SupportURL:  "https://example.com/support",
				},
			},
			"fr-FR": {
				localization: VersionLocalization{Keywords: "bonjour"},
			},
		},
		map[string]VersionLocalization{
			"en-US": {Keywords: "english,keywords"},
		},
		shared.SubmitReadinessCreateModePlanned,
		shared.SubmitReadinessOptions{},
	)

	if len(warnings) != 2 {
		t.Fatalf("expected 2 warnings, got %d: %+v", len(warnings), warnings)
	}
	if warnings[0].Locale != "fr-FR" || warnings[1].Locale != "ja" {
		t.Fatalf("expected sorted warnings for fr-FR and ja, got %+v", warnings)
	}
	for _, warning := range warnings {
		if len(warning.MissingFields) != 2 || warning.MissingFields[0] != "description" || warning.MissingFields[1] != "supportUrl" {
			t.Fatalf("expected description/supportUrl missing fields, got %+v", warning)
		}
	}
}

func TestVersionCreateWarningsForPatches_RequiresWhatsNewForUpdates(t *testing.T) {
	warnings := versionCreateWarningsForPatches(
		map[string]versionLocalPatch{
			"en-US": {
				localization: VersionLocalization{
					Description: "Description",
					Keywords:    "english,keywords",
					SupportURL:  "https://example.com/support",
				},
			},
		},
		map[string]VersionLocalization{},
		shared.SubmitReadinessCreateModePlanned,
		shared.SubmitReadinessOptions{RequireWhatsNew: true},
	)

	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %+v", len(warnings), warnings)
	}
	if warnings[0].Locale != "en-US" {
		t.Fatalf("expected warning for en-US, got %+v", warnings[0])
	}
	if len(warnings[0].MissingFields) != 1 || warnings[0].MissingFields[0] != "whatsNew" {
		t.Fatalf("expected missing fields [whatsNew], got %+v", warnings[0].MissingFields)
	}
}

func TestSuccessfulVersionCreateWarningsFiltersFailedCreates(t *testing.T) {
	warnings := []shared.SubmitReadinessCreateWarning{
		{Locale: "en-US", Mode: shared.SubmitReadinessCreateModeApplied, MissingFields: []string{"keywords"}},
		{Locale: "fr-FR", Mode: shared.SubmitReadinessCreateModeApplied, MissingFields: []string{"supportUrl"}},
		{Locale: "ja", Mode: shared.SubmitReadinessCreateModeApplied, MissingFields: []string{"description"}},
		{Locale: "de-DE", Mode: shared.SubmitReadinessCreateModeApplied, MissingFields: []string{"description"}},
	}
	actions := []ApplyAction{
		{Scope: versionDirName, Locale: "en-US", Action: "create", Status: "succeeded", LocalizationID: "en-id"},
		{Scope: versionDirName, Locale: "fr-FR", Action: "create", Status: "failed", Error: "rejected"},
		{Scope: versionDirName, Locale: "ja", Action: "reconcile", Status: "succeeded", LocalizationID: "ja-id"},
		{Scope: versionDirName, Locale: "de-DE", Action: "reconcile", Status: "succeeded", LocalizationID: "de-id", AlreadyExists: true},
	}

	filtered := successfulVersionCreateWarnings(warnings, actions, nil)
	if len(filtered) != 2 || filtered[0].Locale != "en-US" || filtered[1].Locale != "ja" {
		t.Fatalf("unexpected applied warnings: %+v", filtered)
	}
}
