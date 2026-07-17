package validate

import (
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestMapAgeRatingDeclarationIncludesSocialMediaFields(t *testing.T) {
	trueValue := true
	falseValue := false

	mapped := mapAgeRatingDeclaration(asc.AgeRatingDeclarationAttributes{
		SocialMedia:              &asc.NullableBool{Value: &trueValue},
		SocialMediaAgeRestricted: &asc.NullableBool{Value: &falseValue},
	})

	if mapped.SocialMedia == nil || !*mapped.SocialMedia {
		t.Fatalf("expected socialMedia=true, got %#v", mapped.SocialMedia)
	}
	if mapped.SocialMediaAgeRestricted == nil || *mapped.SocialMediaAgeRestricted {
		t.Fatalf("expected socialMediaAgeRestricted=false, got %#v", mapped.SocialMediaAgeRestricted)
	}
}
