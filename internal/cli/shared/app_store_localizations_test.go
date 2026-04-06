package shared

import (
	"slices"
	"strings"
	"testing"
)

func TestAppStoreLocalizationCatalogIncludesCurrentHelpAndLiveLocales(t *testing.T) {
	locales := AppStoreLocalizationCatalog()

	if len(locales) != 50 {
		t.Fatalf("expected 50 supported locales, got %d", len(locales))
	}

	for _, want := range []string{"en-US", "bn-BD", "sl-SI", "ur-PK"} {
		if !slices.ContainsFunc(locales, func(locale AppStoreLocalizationLocale) bool { return locale.Code == want }) {
			t.Fatalf("expected supported locales to include %q, got %v", want, locales)
		}
	}
}

func TestSupportedMetadataLocalesDeriveFromSharedCatalog(t *testing.T) {
	locales := SupportedMetadataLocales()

	if len(locales) != 39 {
		t.Fatalf("expected 39 metadata locales, got %d", len(locales))
	}
	if !slices.Contains(locales, "en-US") {
		t.Fatalf("expected metadata locales to include en-US, got %v", locales)
	}
	for _, unexpected := range []string{"bn-BD", "sl-SI", "ur-PK"} {
		if slices.Contains(locales, unexpected) {
			t.Fatalf("expected metadata locales to exclude %q, got %v", unexpected, locales)
		}
	}
}

func TestCanonicalizeAppStoreLocalizationLocale(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "en_us", want: "en-US"},
		{input: "EN-gb", want: "en-GB"},
		{input: "sl-si", want: "sl-SI"},
		{input: "bn-BD", want: "bn-BD"},
		{input: "en-IN", want: "en-IN"},
		{input: "zh-Hant-HK", want: "zh-Hant-HK"},
		{input: "eo", want: "eo"},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got, err := CanonicalizeAppStoreLocalizationLocale(test.input)
			if err != nil {
				t.Fatalf("CanonicalizeAppStoreLocalizationLocale() error: %v", err)
			}
			if got != test.want {
				t.Fatalf("expected %q, got %q", test.want, got)
			}
		})
	}
}

func TestCanonicalizeAppStoreLocalizationLocaleRejectsRootShorthand(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{
			input: "nl",
			want:  []string{`unsupported locale "nl"`, "nl-NL"},
		},
		{
			input: "en",
			want:  []string{`unsupported locale "en"`, "use one of", "en-AU", "en-US"},
		},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			_, err := CanonicalizeAppStoreLocalizationLocale(test.input)
			if err == nil {
				t.Fatal("expected unsupported locale error")
			}

			for _, want := range test.want {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("expected error to contain %q, got %v", want, err)
				}
			}
		})
	}
}
