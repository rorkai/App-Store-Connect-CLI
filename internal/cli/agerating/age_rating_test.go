package agerating

import (
	"context"
	"errors"
	"flag"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestAgeRatingCommandShape(t *testing.T) {
	cmd := AgeRatingCommand()
	if cmd == nil {
		t.Fatal("expected age-rating command")
	}
	if cmd.Name != "age-rating" {
		t.Fatalf("unexpected command name: %q", cmd.Name)
	}
	if len(cmd.Subcommands) != 3 {
		t.Fatalf("expected 3 subcommands, got %d", len(cmd.Subcommands))
	}
	if got := AgeRatingCommand(); got == nil {
		t.Fatal("expected Command wrapper to return command")
	}
	usage := cmd.UsageFunc(cmd)
	if strings.Contains(usage, "\n  get\t") {
		t.Fatalf("expected age-rating help to hide get alias, got %q", usage)
	}
}

func TestAgeRatingValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	t.Run("view missing app or ids", func(t *testing.T) {
		cmd := AgeRatingViewCommand()
		if err := cmd.FlagSet.Parse([]string{}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	t.Run("view conflicting app-info and version", func(t *testing.T) {
		cmd := AgeRatingViewCommand()
		if err := cmd.FlagSet.Parse([]string{"--app-info-id", "A", "--version-id", "V"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := cmd.Exec(context.Background(), nil)
		if err == nil || errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected non-ErrHelp error, got %v", err)
		}
	})

	t.Run("set missing id and app", func(t *testing.T) {
		cmd := AgeRatingSetCommand()
		if err := cmd.FlagSet.Parse([]string{}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})
}

func TestCompatAgeRatingGetAliasShape(t *testing.T) {
	cmd := compatAgeRatingGetAliasCommand()
	if cmd == nil {
		t.Fatal("expected get compatibility alias command")
	}
	if cmd.Name != "get" {
		t.Fatalf("unexpected alias command name: %q", cmd.Name)
	}
	if cmd.ShortHelp != "Compatibility alias for `asc age-rating view`." {
		t.Fatalf("unexpected alias short help: %q", cmd.ShortHelp)
	}
}

func TestAgeRatingHelpers(t *testing.T) {
	// Bool fields parse correctly
	attrs, err := buildAgeRatingAttributes(map[string]string{
		"advertising": "false",
		"gambling":    "true",
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
