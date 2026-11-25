package cmdtest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
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
		"datingAndRelationships":false,"sharedLocation":false,"parentalControls":false,"ageAssurance":false,"socialMedia":false,"socialMediaAgeRestricted":false,
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

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), []string{
		"age-rating", "edit",
		"--id", "age-1",
		"--all-none",
		"--gambling", "true",
		"--violence-realistic", "FREQUENT_OR_INTENSE",
		"--output", "json",
	}...)
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

func TestAgeRatingEditLocationAndRelationships(t *testing.T) {
	for _, test := range []struct {
		flag      string
		attribute string
		value     bool
	}{
		{flag: "shared-location", attribute: "sharedLocation", value: true},
		{flag: "dating-and-relationships", attribute: "datingAndRelationships", value: false},
	} {
		t.Run(test.flag, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_APP_ID", "")
			body := fmt.Sprintf(`{"data":{"type":"ageRatingDeclarations","id":"age-1","attributes":{"%s":%t}}}`, test.attribute, test.value)
			reg := asctest.NewRegistry(t)
			reg.Register(asctest.PATCH("/v1/ageRatingDeclarations/age-1").JSONBody(body), asctest.JSON(http.StatusOK, body))
			stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg),
				"age-rating", "edit", "--id", "age-1", "--"+test.flag, strconv.FormatBool(test.value), "--output", "json")
			if err != nil || stderr != "" {
				t.Fatalf("err=%v stderr=%q", err, stderr)
			}
			var response struct {
				Data struct {
					Attributes map[string]bool `json:"attributes"`
				} `json:"data"`
			}
			if err := json.Unmarshal([]byte(stdout), &response); err != nil {
				t.Fatalf("parse output: %v", err)
			}
			value, exists := response.Data.Attributes[test.attribute]
			if !exists || value != test.value {
				t.Fatalf("attributes=%v, want %s=%t", response.Data.Attributes, test.attribute, test.value)
			}
		})
	}
}
