package cmdtest

import (
	"net/http"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestAgeRatingSetAllNoneUsesSafeDefaultsAndPreservesOverrides(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.PATCH("/v1/ageRatingDeclarations/age-1").JSONBody(`{"data":{"type":"ageRatingDeclarations","id":"age-1","attributes":{
		"advertising":false,"gambling":true,"healthOrWellnessTopics":false,"lootBox":false,"messagingAndChat":false,
		"parentalControls":false,"ageAssurance":false,"socialMedia":false,"socialMediaAgeRestricted":false,
		"unrestrictedWebAccess":false,"userGeneratedContent":false,"alcoholTobaccoOrDrugUseOrReferences":"NONE",
		"contests":"NONE","gamblingSimulated":"NONE","gunsOrOtherWeapons":"NONE","medicalOrTreatmentInformation":"NONE",
		"profanityOrCrudeHumor":"NONE","sexualContentGraphicAndNudity":"NONE","sexualContentOrNudity":"NONE",
		"horrorOrFearThemes":"NONE","matureOrSuggestiveThemes":"NONE","violenceCartoonOrFantasy":"NONE",
		"violenceRealistic":"FREQUENT_OR_INTENSE","violenceRealisticProlongedGraphicOrSadistic":"NONE"}}}`),
		asctest.JSON(http.StatusOK, `{
			"data":{
				"type":"ageRatingDeclarations",
				"id":"age-1",
				"attributes":{
					"advertising":false,
					"gambling":true,
					"violenceRealistic":"FREQUENT_OR_INTENSE",
					"contests":"NONE"
				}
			}
		}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg),
		"age-rating", "edit",
		"--id", "age-1",
		"--all-none",
		"--gambling", "true",
		"--violence-realistic", "FREQUENT_OR_INTENSE",
		"--output", "json",
	)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if stdout == "" {
		t.Fatal("expected JSON output on stdout")
	}
}
