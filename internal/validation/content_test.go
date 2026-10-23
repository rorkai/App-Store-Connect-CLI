package validation

import (
	"strings"
	"testing"
)

func TestContentChecksFlagRejectionProneCopy(t *testing.T) {
	tests := []struct {
		name         string
		value        string
		inKeywords   bool
		wantID       string
		wantSeverity Severity
		wantQuoted   string
	}{
		{name: "lorem ipsum sequence", value: "Lorem ipsum dolor sit amet.", wantID: "content.placeholder_text", wantQuoted: "Lorem ipsum dolor sit amet"},
		{name: "todo marker", value: "TODO: write the final description.", wantID: "content.placeholder_text", wantQuoted: "TODO"},
		{name: "tbd marker", value: "Pricing details TBD.", wantID: "content.placeholder_text", wantQuoted: "TBD"},
		{name: "fixme marker", value: "FIXME before submission.", wantID: "content.placeholder_text", wantQuoted: "FIXME"},
		{name: "android platform", value: "Also available on Android devices.", wantID: "content.other_platforms", wantQuoted: "Android"},
		{name: "store name", value: "Rated five stars on Google Play.", wantID: "content.other_platforms", wantQuoted: "Google Play"},
		{name: "lowercase android keyword", value: "notes,android,sync", inKeywords: true, wantID: "content.other_platforms", wantQuoted: "android"},
		{name: "coming soon", value: "New seasonal challenges are coming soon.", wantID: "content.unfinished_features", wantSeverity: SeverityInfo, wantQuoted: "coming soon"},
		{name: "public beta", value: "Join the public beta today.", wantID: "content.unfinished_features", wantSeverity: SeverityInfo, wantQuoted: "public beta"},
		{name: "pre-release", value: "This pre-release adds widgets.", wantID: "content.unfinished_features", wantSeverity: SeverityInfo, wantQuoted: "pre-release"},
		{name: "debug build", value: "This debug build logs every request.", wantID: "content.test_words", wantQuoted: "debug build"},
		{name: "testing purposes", value: "Uploaded for testing purposes.", wantID: "content.test_words", wantQuoted: "for testing purposes"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			loc := VersionLocalization{ID: "loc-1", Locale: "en-US", Description: test.value}
			if test.inKeywords {
				loc = VersionLocalization{ID: "loc-1", Locale: "en-US", Keywords: test.value}
			}
			checks := contentChecks([]VersionLocalization{loc}, nil)
			if len(checks) != 1 {
				t.Fatalf("checks = %+v, want one %s check", checks, test.wantID)
			}
			wantSeverity := test.wantSeverity
			if wantSeverity == "" {
				wantSeverity = SeverityWarning
			}
			check := checks[0]
			if check.ID != test.wantID || check.Severity != wantSeverity {
				t.Fatalf("check = %+v, want %s %s", check, test.wantID, wantSeverity)
			}
			if !strings.Contains(check.Message, `"`+test.wantQuoted+`"`) || check.Remediation == "" {
				t.Fatalf("check = %+v, want quoted match and remediation", check)
			}
		})
	}
}

func TestContentChecksIgnoreEditoriallyAmbiguousMetadata(t *testing.T) {
	tests := []string{
		"Protected by Google reCAPTCHA.",
		"Back up your notes to Google Drive.",
		"Beta Test Timer helps developers coordinate release checks.",
		"Command androids in a robot arena.",
		"Your android companion learns as you play.",
		"Sort entries in alphabetical order.",
		"Log beta blockers and relax with alpha and beta waves.",
		"Track your test results and latest builds.",
		"Home testing only takes 15 minutes.",
		"Some features are not yet available in your region.",
		"Start a free trial and subscribe when you are ready.",
		"Manage your todo list and keep to-do items in one place.",
		"Find your place in line and hold it.",
		"Generate placeholder images and dummy text samples.",
		"Paste your text here to transform it.",
	}

	for _, value := range tests {
		t.Run(value, func(t *testing.T) {
			checks := contentChecks(
				[]VersionLocalization{{ID: "loc-1", Locale: "en-US", Description: value}},
				nil,
			)
			if len(checks) != 0 {
				t.Fatalf("checks for %q = %+v, want none", value, checks)
			}
		})
	}
}

func TestContentChecksIgnoreLocalizedAndProductUsesOfPlaceholderPhrases(t *testing.T) {
	tests := []struct {
		name        string
		versionLocs []VersionLocalization
		appInfoLocs []AppInfoLocalization
	}{
		{
			name:        "Spanish todo phrase",
			versionLocs: []VersionLocalization{{ID: "loc-1", Locale: "es-ES", Description: "TODO EN UN SOLO LUGAR"}},
		},
		{
			name:        "TODO product name",
			appInfoLocs: []AppInfoLocalization{{ID: "info-1", Locale: "en-US", Name: "TODO Planner"}},
		},
		{
			name:        "Lorem Ipsum product name",
			appInfoLocs: []AppInfoLocalization{{ID: "info-1", Locale: "en-US", Name: "Lorem Ipsum Generator"}},
		},
		{
			name:        "Lorem Ipsum text creation",
			versionLocs: []VersionLocalization{{ID: "loc-1", Locale: "en-US", Description: "Create Lorem ipsum text"}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			checks := contentChecks(test.versionLocs, test.appInfoLocs)
			if len(checks) != 0 {
				t.Fatalf("checks = %+v, want no placeholder warning for legitimate copy", checks)
			}
		})
	}
}

func TestContentChecksKeepExplicitLocalizedMarkerWarnings(t *testing.T) {
	checks := contentChecks(
		[]VersionLocalization{{ID: "loc-1", Locale: "es-ES", Description: "TODO: completar la descripción"}},
		nil,
	)
	if len(checks) != 1 || checks[0].ID != "content.placeholder_text" {
		t.Fatalf("checks = %+v, want explicit placeholder marker warning", checks)
	}
}

func TestContentChecksUseUnicodeAwarePlaceholderBoundaries(t *testing.T) {
	for _, value := range []string{
		"MÉTODO helps you organize research.",
		"ME\u0301TODO helps you organize research.",
		"TODOアプリ keeps tasks in sync.",
		"éLorem ipsum is a product name.",
		"e\u0301Lorem ipsum is a product name.",
	} {
		t.Run(value, func(t *testing.T) {
			checks := contentChecks(
				[]VersionLocalization{{ID: "loc-1", Locale: "en-US", Description: value}},
				nil,
			)
			if len(checks) != 0 {
				t.Fatalf("checks for %q = %+v, want none", value, checks)
			}
		})
	}

	checks := contentChecks(
		[]VersionLocalization{{ID: "loc-1", Locale: "ja", Description: "（TODO：finalize the copy）"}},
		nil,
	)
	if len(checks) != 1 || checks[0].ID != "content.placeholder_text" {
		t.Fatalf("checks = %+v, want punctuation-delimited placeholder warning", checks)
	}
}

func TestContentChecksPreservePlaceholderSourceOrderAcrossPatternGroups(t *testing.T) {
	checks := contentChecks(
		[]VersionLocalization{{ID: "loc-1", Locale: "en-US", Description: "TODO: before lorem ipsum dolor sit amet and FIXME"}},
		nil,
	)
	if len(checks) != 1 {
		t.Fatalf("checks = %+v, want one placeholder warning", checks)
	}
	message := checks[0].Message
	todo := strings.Index(message, `"TODO"`)
	lorem := strings.Index(message, `"lorem ipsum dolor sit amet"`)
	fixme := strings.Index(message, `"FIXME"`)
	if todo < 0 || lorem < 0 || fixme < 0 || todo >= lorem || lorem >= fixme {
		t.Fatalf("placeholder order = %q, want TODO, lorem ipsum, FIXME", message)
	}
}

func TestContentChecksMatchPlaceholderAcrossWhitespaceSeparators(t *testing.T) {
	separators := []string{"  ", "\n", "\v", "\u0085", "\u2028", "\u2029"}
	for _, separator := range separators {
		checks := contentChecks(
			[]VersionLocalization{{ID: "loc-1", Locale: "en-US", Description: "lorem" + separator + "ipsum dolor sit amet"}},
			nil,
		)
		if len(checks) != 1 || checks[0].ID != "content.placeholder_text" {
			t.Fatalf("separator %q checks = %+v, want placeholder warning", separator, checks)
		}
		if !strings.Contains(checks[0].Message, "lorem ipsum dolor sit amet") {
			t.Fatalf("separator %q message = %q, want normalized phrase", separator, checks[0].Message)
		}
	}
}

func TestContentChecksCoverEveryLocalizedTextField(t *testing.T) {
	version := VersionLocalization{
		ID:              "version-loc",
		Locale:          "en-US",
		Description:     "TODO:",
		Keywords:        "TBD",
		WhatsNew:        "FIXME",
		PromotionalText: "Lorem ipsum dolor sit amet",
	}
	appInfo := AppInfoLocalization{
		ID:       "info-loc",
		Locale:   "en-US",
		Name:     "TODO:",
		Subtitle: "TBD",
	}
	checks := contentChecks([]VersionLocalization{version}, []AppInfoLocalization{appInfo})

	want := map[string]string{
		"description":     "version-loc",
		"keywords":        "version-loc",
		"whatsNew":        "version-loc",
		"promotionalText": "version-loc",
		"name":            "info-loc",
		"subtitle":        "info-loc",
	}
	if len(checks) != len(want) {
		t.Fatalf("checks = %+v, want one per localized text field", checks)
	}
	for _, check := range checks {
		if check.ResourceID != want[check.Field] || check.Locale != "en-US" {
			t.Fatalf("check = %+v, want matching resource and locale", check)
		}
	}
}

func TestContentChecksCollapseRepeatedMatches(t *testing.T) {
	checks := contentChecks(
		[]VersionLocalization{{ID: "loc-1", Locale: "en-US", Description: "TODO: then TODO — and lorem ipsum dolor sit amet"}},
		nil,
	)
	if len(checks) != 1 {
		t.Fatalf("checks = %+v, want one warning", checks)
	}
	if strings.Count(checks[0].Message, `"TODO"`) != 1 || !strings.Contains(checks[0].Message, `"lorem ipsum dolor sit amet"`) {
		t.Fatalf("message = %q, want distinct matches once", checks[0].Message)
	}
}

func TestContentChecksSkipEmptyFields(t *testing.T) {
	checks := contentChecks(
		[]VersionLocalization{{ID: "loc-1", Locale: "en-US"}},
		[]AppInfoLocalization{{ID: "info-1", Locale: "en-US"}},
	)
	if len(checks) != 0 {
		t.Fatalf("checks = %+v, want none", checks)
	}
}

func TestValidateIncludesPlaceholderWarningAndStrictModeControlsBlocking(t *testing.T) {
	input := Input{
		AppID:         "app-1",
		VersionID:     "version-1",
		VersionString: "2.0",
		VersionState:  "PREPARE_FOR_SUBMISSION",
		PrimaryLocale: "en-US",
		Copyright:     "2026 Example",
		VersionLocalizations: []VersionLocalization{{
			ID:          "loc-1",
			Locale:      "en-US",
			Description: "TODO: write the final description.",
			Keywords:    "habits",
			WhatsNew:    "Bug fixes",
			SupportURL:  "https://example.com",
		}},
		AppInfoLocalizations: []AppInfoLocalization{{
			ID:               "info-1",
			Locale:           "en-US",
			Name:             "Habit Timer",
			Subtitle:         "Track habits",
			PrivacyPolicyURL: "https://example.com/privacy",
		}},
		PrimaryCategoryID: "category-1",
	}

	report := Validate(input, false)
	if !hasCheckID(report.Checks, "content.placeholder_text") {
		t.Fatalf("checks = %+v, want placeholder warning", report.Checks)
	}
	if report.Summary.Blocking != report.Summary.Errors {
		t.Fatalf("summary = %+v, want warning non-blocking by default", report.Summary)
	}
	foundStep := false
	for _, step := range report.Remediation.Steps {
		if step.CheckID == "content.placeholder_text" {
			foundStep = true
			if step.Blocking {
				t.Fatalf("step = %+v, want non-blocking remediation by default", step)
			}
		}
	}
	if !foundStep {
		t.Fatal("placeholder warning is missing from remediation plan")
	}

	strictReport := Validate(input, true)
	if strictReport.Summary.Blocking != strictReport.Summary.Errors+strictReport.Summary.Warnings {
		t.Fatalf("strict summary = %+v, want warnings to block", strictReport.Summary)
	}
}
