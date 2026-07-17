package asc

import (
	"strings"
	"testing"
)

func TestPrintTableAgeRatingIncludesSocialMediaFields(t *testing.T) {
	socialMedia := true
	ageRestricted := false
	resp := &AgeRatingDeclarationResponse{
		Data: Resource[AgeRatingDeclarationAttributes]{
			ID:   "age-441",
			Type: ResourceTypeAgeRatingDeclarations,
			Attributes: AgeRatingDeclarationAttributes{
				SocialMedia:              &NullableBool{Value: &socialMedia},
				SocialMediaAgeRestricted: &NullableBool{Value: &ageRestricted},
			},
		},
	}

	output := captureStdout(t, func() error { return PrintTable(resp) })
	for _, want := range []string{"Social Media", "Social Media Age Restricted", "true", "false"} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected table output to contain %q, got %q", want, output)
		}
	}
}
