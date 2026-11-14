package agerating

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestAgeRatingCommandShape(t *testing.T) {
	d := &shared.Deps{IO: &shared.IO{Out: io.Discard, ErrOut: io.Discard}}
	cmd := NewAgeRatingCommand(d)
	if cmd == nil {
		t.Fatal("expected age-rating command")
		return
	}
	if cmd.Name != "age-rating" {
		t.Fatalf("unexpected command name: %q", cmd.Name)
	}
	if len(cmd.Subcommands) != 3 {
		t.Fatalf("expected 3 subcommands, got %d", len(cmd.Subcommands))
	}
	if got := NewAgeRatingCommand(d); got == nil {
		t.Fatal("expected Command wrapper to return command")
	}
	usage := cmd.UsageFunc(cmd)
	for _, visible := range []string{
		"\n  view   View an age rating declaration.",
		"\n  edit   Update an age rating declaration.",
		"\n  audit  Audit social-media age rating responses across apps.",
	} {
		if !strings.Contains(usage, visible) {
			t.Fatalf("expected age-rating help to include canonical verb %q, got %q", strings.TrimSpace(visible), usage)
		}
	}
	for _, hidden := range []string{"\n  get\t", "\n  set\t"} {
		if strings.Contains(usage, hidden) {
			t.Fatalf("expected age-rating help to hide legacy verb %q, got %q", strings.TrimSpace(hidden), usage)
		}
	}
}

func TestAgeRatingValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name       string
		newCommand func(*shared.Deps) *ffcli.Command
		args       []string
		wantStderr string
	}{
		{name: "view missing app or ids", newCommand: newAgeRatingViewCommand, wantStderr: "Error: --app is required (or set ASC_APP_ID)\n"},
		{name: "view conflicting app-info and version", newCommand: newAgeRatingViewCommand, args: []string{"--app-info-id", "A", "--version-id", "V"}, wantStderr: "Error: only one of --app-info-id or --version-id is allowed\n"},
		{name: "edit missing id and app", newCommand: newAgeRatingEditCommand, wantStderr: "Error: --id or --app is required (or set ASC_APP_ID)\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			cmd := test.newCommand(&shared.Deps{
				Client: func() (*asc.Client, error) {
					return nil, errors.New("unexpected client request")
				},
				IO: &shared.IO{Out: &stdout, ErrOut: &stderr},
			})
			if err := cmd.FlagSet.Parse(test.args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected ErrHelp, got %v", err)
			}
			if got := stderr.String(); got != test.wantStderr {
				t.Fatalf("stderr = %q, want %q", got, test.wantStderr)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

func TestAgeRatingHelpers(t *testing.T) {
	// Bool fields parse correctly
	attrs, err := buildAgeRatingAttributes(map[string]string{
		"advertising":                 "false",
		"gambling":                    "true",
		"social-media":                "true",
		"social-media-age-restricted": "false",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if attrs.Advertising == nil || *attrs.Advertising != false {
		t.Fatal("expected advertising=false")
	}
	if attrs.Gambling == nil || *attrs.Gambling != true {
		t.Fatal("expected gambling=true")
	}
	if attrs.SocialMedia == nil || attrs.SocialMedia.Value == nil || !*attrs.SocialMedia.Value {
		t.Fatal("expected social-media=true")
	}
	if attrs.SocialMediaAgeRestricted == nil || attrs.SocialMediaAgeRestricted.Value == nil || *attrs.SocialMediaAgeRestricted.Value {
		t.Fatal("expected social-media-age-restricted=false")
	}

	// Invalid bool value returns error
	if _, err := buildAgeRatingAttributes(map[string]string{
		"gambling": "notabool",
	}); err == nil {
		t.Fatal("expected error for invalid bool value")
	}

	// Enum parsing
	if got, err := parseOptionalEnumFlag("--kids-age-band", "five_and_under", kidsAgeBandValues); err != nil || got == nil || *got != "FIVE_AND_UNDER" {
		t.Fatalf("expected normalized enum value, got %v err=%v", got, err)
	}
	if got, err := parseOptionalEnumFlag("--violence-realistic", "infrequent", ageRatingLevelValues); err != nil || got == nil || *got != "INFREQUENT" {
		t.Fatalf("expected legacy enum value to be accepted, got %v err=%v", got, err)
	}
	if _, err := parseOptionalEnumFlag("--kids-age-band", "bad", kidsAgeBandValues); err == nil {
		t.Fatal("expected enum validation error")
	}

	// Enum fields parse correctly via buildAgeRatingAttributes
	attrs2, err := buildAgeRatingAttributes(map[string]string{
		"guns-or-other-weapons": "FREQUENT_OR_INTENSE",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if attrs2.GunsOrOtherWeapons == nil || *attrs2.GunsOrOtherWeapons != "FREQUENT_OR_INTENSE" {
		t.Fatal("expected guns-or-other-weapons=FREQUENT_OR_INTENSE")
	}

	// Override and URL fields parse correctly
	attrs3, err := buildAgeRatingAttributes(map[string]string{
		"age-rating-override-v2":        "EIGHTEEN_PLUS",
		"korea-age-rating-override":     "NINETEEN_PLUS",
		"developer-age-rating-info-url": "https://example.com/age-rating",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if attrs3.AgeRatingOverrideV2 == nil || *attrs3.AgeRatingOverrideV2 != "EIGHTEEN_PLUS" {
		t.Fatal("expected age-rating-override-v2=EIGHTEEN_PLUS")
	}
	if attrs3.KoreaAgeRatingOverride == nil || *attrs3.KoreaAgeRatingOverride != "NINETEEN_PLUS" {
		t.Fatal("expected korea-age-rating-override=NINETEEN_PLUS")
	}
	if attrs3.DeveloperAgeRatingInfoURL == nil || *attrs3.DeveloperAgeRatingInfoURL != "https://example.com/age-rating" {
		t.Fatal("expected developer-age-rating-info-url to be parsed")
	}
	if _, err := buildAgeRatingAttributes(map[string]string{
		"developer-age-rating-info-url": "not-a-url",
	}); err == nil {
		t.Fatal("expected invalid URL error")
	}

	// hasAgeRatingUpdates
	if hasAgeRatingUpdates(asc.AgeRatingDeclarationAttributes{}) {
		t.Fatal("expected no updates for zero-value attrs")
	}
	value := "NONE"
	if !hasAgeRatingUpdates(asc.AgeRatingDeclarationAttributes{GamblingSimulated: &value}) {
		t.Fatal("expected updates when one pointer attribute is set")
	}
	boolVal := false
	if !hasAgeRatingUpdates(asc.AgeRatingDeclarationAttributes{Advertising: &boolVal}) {
		t.Fatal("expected updates when one bool attribute is set")
	}
	override := "NINE_PLUS"
	if !hasAgeRatingUpdates(asc.AgeRatingDeclarationAttributes{AgeRatingOverrideV2: &override}) {
		t.Fatal("expected updates when override attribute is set")
	}
}

func TestValidateAgeRatingDependenciesRejectsTransitiveUGCContradiction(t *testing.T) {
	trueValue := true
	falseValue := false

	var stderr bytes.Buffer
	err := validateAgeRatingDependencies(&shared.Deps{IO: &shared.IO{ErrOut: &stderr}}, asc.AgeRatingDeclarationAttributes{
		AgeAssurance:             &trueValue,
		SocialMediaAgeRestricted: &asc.NullableBool{Value: &trueValue},
		UserGeneratedContent:     &falseValue,
	})
	if err == nil || !strings.Contains(err.Error(), "--social-media-age-restricted true cannot be combined with --user-generated-content false") {
		t.Fatalf("expected transitive user-generated-content dependency error, got %v", err)
	}
	if got, want := stderr.String(), "Error: --social-media-age-restricted true cannot be combined with --user-generated-content false\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestValidateAgeRatingDependenciesAllowsPartialUpdates(t *testing.T) {
	trueValue := true
	falseValue := false

	tests := []struct {
		name  string
		attrs asc.AgeRatingDeclarationAttributes
	}{
		{
			name:  "social media may rely on stored user generated content",
			attrs: asc.AgeRatingDeclarationAttributes{SocialMedia: &asc.NullableBool{Value: &trueValue}},
		},
		{
			name: "age restriction may rely on stored prerequisites",
			attrs: asc.AgeRatingDeclarationAttributes{
				SocialMediaAgeRestricted: &asc.NullableBool{Value: &trueValue},
			},
		},
		{
			name: "all prerequisites explicitly enabled",
			attrs: asc.AgeRatingDeclarationAttributes{
				AgeAssurance:             &trueValue,
				SocialMedia:              &asc.NullableBool{Value: &trueValue},
				SocialMediaAgeRestricted: &asc.NullableBool{Value: &trueValue},
				UserGeneratedContent:     &trueValue,
			},
		},
		{
			name: "unrelated false fields",
			attrs: asc.AgeRatingDeclarationAttributes{
				AgeAssurance:         &falseValue,
				SocialMedia:          &asc.NullableBool{Value: &falseValue},
				UserGeneratedContent: &falseValue,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stderr bytes.Buffer
			if err := validateAgeRatingDependencies(&shared.Deps{IO: &shared.IO{ErrOut: &stderr}}, test.attrs); err != nil {
				t.Fatalf("unexpected dependency error: %v", err)
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestApplyAllNoneDefaultsSetsAllContentDescriptors(t *testing.T) {
	values := map[string]string{}
	applyAllNoneDefaults(values)

	attrs, err := buildAgeRatingAttributes(values)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	boolChecks := map[string]*bool{
		"advertising":                 attrs.Advertising,
		"gambling":                    attrs.Gambling,
		"health-or-wellness-topics":   attrs.HealthOrWellnessTopics,
		"loot-box":                    attrs.LootBox,
		"messaging-and-chat":          attrs.MessagingAndChat,
		"parental-controls":           attrs.ParentalControls,
		"age-assurance":               attrs.AgeAssurance,
		"social-media":                nullableBoolValue(attrs.SocialMedia),
		"social-media-age-restricted": nullableBoolValue(attrs.SocialMediaAgeRestricted),
		"unrestricted-web-access":     attrs.UnrestrictedWebAccess,
		"user-generated-content":      attrs.UserGeneratedContent,
	}
	for name, value := range boolChecks {
		if value == nil || *value {
			t.Fatalf("expected %s=false, got %#v", name, value)
		}
	}

	enumChecks := map[string]*string{
		"alcohol-tobacco-drug-use":      attrs.AlcoholTobaccoOrDrugUseOrReferences,
		"contests":                      attrs.Contests,
		"gambling-simulated":            attrs.GamblingSimulated,
		"guns-or-other-weapons":         attrs.GunsOrOtherWeapons,
		"medical-treatment":             attrs.MedicalOrTreatmentInformation,
		"profanity-humor":               attrs.ProfanityOrCrudeHumor,
		"sexual-content-nudity":         attrs.SexualContentOrNudity,
		"sexual-content-graphic-nudity": attrs.SexualContentGraphicAndNudity,
		"horror-fear":                   attrs.HorrorOrFearThemes,
		"mature-suggestive":             attrs.MatureOrSuggestiveThemes,
		"violence-cartoon":              attrs.ViolenceCartoonOrFantasy,
		"violence-realistic":            attrs.ViolenceRealistic,
		"violence-realistic-graphic":    attrs.ViolenceRealisticProlongedGraphicOrSadistic,
	}
	for name, value := range enumChecks {
		if value == nil || *value != "NONE" {
			t.Fatalf("expected %s=NONE, got %#v", name, value)
		}
	}

	if attrs.KidsAgeBand != nil {
		t.Fatalf("expected kids age band to remain unset, got %#v", attrs.KidsAgeBand)
	}
	if attrs.AgeRatingOverride != nil {
		t.Fatalf("expected age rating override to remain unset, got %#v", attrs.AgeRatingOverride)
	}
	if attrs.AgeRatingOverrideV2 != nil {
		t.Fatalf("expected age rating override v2 to remain unset, got %#v", attrs.AgeRatingOverrideV2)
	}
	if attrs.KoreaAgeRatingOverride != nil {
		t.Fatalf("expected korea age rating override to remain unset, got %#v", attrs.KoreaAgeRatingOverride)
	}
	if attrs.DeveloperAgeRatingInfoURL != nil {
		t.Fatalf("expected developer age rating info url to remain unset, got %#v", attrs.DeveloperAgeRatingInfoURL)
	}
}
