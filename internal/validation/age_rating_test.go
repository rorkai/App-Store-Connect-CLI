package validation

import "testing"

func TestAgeRatingChecks_Incomplete(t *testing.T) {
	checks := ageRatingChecks(&AgeRatingDeclaration{})
	if !hasCheckID(checks, "age_rating.missing_field") {
		t.Fatalf("expected missing field check")
	}
}

func TestAgeRatingChecks_Complete(t *testing.T) {
	trueValue := true
	falseValue := false
	level := "NONE"
	decl := AgeRatingDeclaration{
		Advertising:                         &falseValue,
		Gambling:                            &falseValue,
		HealthOrWellnessTopics:              &falseValue,
		LootBox:                             &falseValue,
		MessagingAndChat:                    &trueValue,
		ParentalControls:                    &trueValue,
		AgeAssurance:                        &falseValue,
		SocialMedia:                         &trueValue,
		SocialMediaAgeRestricted:            &falseValue,
		UnrestrictedWebAccess:               &falseValue,
		UserGeneratedContent:                &trueValue,
		AlcoholTobaccoOrDrugUseOrReferences: &level,
		Contests:                            &level,
		GamblingSimulated:                   &level,
		GunsOrOtherWeapons:                  &level,
		MedicalOrTreatmentInformation:       &level,
		ProfanityOrCrudeHumor:               &level,
		SexualContentGraphicAndNudity:       &level,
		SexualContentOrNudity:               &level,
		HorrorOrFearThemes:                  &level,
		MatureOrSuggestiveThemes:            &level,
		ViolenceCartoonOrFantasy:            &level,
		ViolenceRealistic:                   &level,
		ViolenceRealisticProlongedGraphicOrSadistic: &level,
	}

	checks := ageRatingChecks(&decl)
	if len(checks) != 0 {
		t.Fatalf("expected no checks, got %d", len(checks))
	}
}

func TestAgeRatingChecks_RequiresSocialMediaFields(t *testing.T) {
	falseValue := false
	declaration := &AgeRatingDeclaration{
		SocialMedia: &falseValue,
	}

	checks := ageRatingChecks(declaration)
	if !hasCheckField(checks, "socialMediaAgeRestricted") {
		t.Fatalf("expected missing socialMediaAgeRestricted check, got %#v", checks)
	}
	if hasCheckField(checks, "socialMedia") {
		t.Fatalf("did not expect missing socialMedia check, got %#v", checks)
	}
}

func hasCheckField(checks []CheckResult, field string) bool {
	for _, check := range checks {
		if check.Field == field {
			return true
		}
	}
	return false
}
