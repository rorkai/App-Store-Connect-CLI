package cmdtest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

// freshAppValidateFixture serves what Apple returned on 2026-11-22 for a new
// app whose 1.0 version has nothing filled in yet, with Apple's IDs renamed to
// the fixture's.
func freshAppValidateFixture() validateFixture {
	fixture := validValidateFixture()
	fixture.app = `{"data":{"type":"apps","id":"app-1","attributes":{"primaryLocale":"en-US","contentRightsDeclaration":null}}}`
	fixture.versions = `{"data":[{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"IOS","versionString":"1.0","copyright":null}}]}`
	fixture.version = `{"data":{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"IOS","versionString":"1.0","appStoreState":"PREPARE_FOR_SUBMISSION","appVersionState":"PREPARE_FOR_SUBMISSION","copyright":null,"reviewType":"APP_STORE","releaseType":"AFTER_APPROVAL","earliestReleaseDate":null,"usesIdfa":null,"downloadable":true},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}},"build":{"data":null},"appStoreReviewDetail":{"data":null}}}}`
	fixture.versionLocs = `{"data":[{"type":"appStoreVersionLocalizations","id":"ver-loc-1","attributes":{"description":null,"locale":"en-US","keywords":null,"marketingUrl":null,"promotionalText":null,"supportUrl":null,"whatsNew":null}}]}`
	fixture.reviewDetails = ""
	fixture.build = ""
	fixture.primaryCategory = `{"data":null,"links":{"self":"https://api.appstoreconnect.apple.com/v1/appInfos/info-1/relationships/primaryCategory"}}`
	fixture.availabilityV2Status = http.StatusNotFound
	fixture.availabilityV2 = `{"errors":[{"id":"2d4b30b9-bba7-49b3-8a2a-a9e174127675","status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'appAvailabilities' with id 'app-1'"}]}`
	fixture.priceSchedule = ""
	fixture.ageRating = `{"data":{"type":"ageRatingDeclarations","id":"info-1","attributes":{"advertising":null,"alcoholTobaccoOrDrugUseOrReferences":null,"contests":null,"datingAndRelationships":null,"gambling":null,"gamblingSimulated":null,"gunsOrOtherWeapons":null,"healthOrWellnessTopics":null,"kidsAgeBand":null,"lootBox":null,"medicalOrTreatmentInformation":null,"messagingAndChat":null,"parentalControls":null,"profanityOrCrudeHumor":null,"ageAssurance":null,"sexualContentGraphicAndNudity":null,"sexualContentOrNudity":null,"sharedLocation":null,"socialMedia":null,"socialMediaAgeRestricted":null,"horrorOrFearThemes":null,"matureOrSuggestiveThemes":null,"unrestrictedWebAccess":null,"userGeneratedContent":null,"violenceCartoonOrFantasy":null,"violenceRealisticProlongedGraphicOrSadistic":null,"violenceRealistic":null,"ageRatingOverride":"NONE","ageRatingOverrideV2":"NONE","koreaAgeRatingOverride":"NONE","gracRatingClassificationNumber":null,"developerAgeRatingInfoUrl":null}}}`
	fixture.screenshotSets = map[string]string{"ver-loc-1": `{"data":[]}`}
	fixture.screenshotsBySet = nil
	fixture.subscriptionGroups = ""
	return fixture
}

// TestValidateBlockersCarryTheirFixCommand runs the default validate (no
// --deep) against a fresh app and checks that every blocker names the exact
// asc command that resolves it, both as resolution.commands and in the
// remediation text that table output and jq filters read.
func TestValidateBlockersCarryTheirFixCommand(t *testing.T) {
	client := newValidateTestClient(t, freshAppValidateFixture())
	stdout, stderr, runErr := runRootWithDeps(t, func() (*asc.Client, error) { return client, nil }, "validate", "--app", "app-1", "--version-id", "ver-1", "--output", "json")
	if runErr == nil || !strings.Contains(runErr.Error(), "blocking issue(s)") {
		t.Fatalf("run error = %v, want the blocking-issues error", runErr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	var report validation.Report
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("parse report: %v\n%s", err, stdout)
	}

	tests := []struct {
		checkID string
		field   string
		want    string
	}{
		{checkID: "build.required.missing", want: `asc versions attach-build --app "app-1" --version-id "ver-1" --latest`},
		{checkID: "categories.primary_missing", field: "primaryCategory", want: `asc categories set --app "app-1" --app-info "info-1" --primary "CATEGORY_ID"`},
		{checkID: "content_rights.missing", field: "contentRightsDeclaration", want: `asc apps update --id "app-1" --content-rights "DECLARATION"`},
		{checkID: "review_details.missing", want: `asc review details-create --version-id "ver-1" --contact-first-name "FIRST_NAME" --contact-last-name "LAST_NAME" --contact-email "EMAIL" --contact-phone "PHONE"`},
		{checkID: "metadata.required.description", field: "description", want: `asc localizations update --id "ver-loc-1" --description "DESCRIPTION"`},
		{checkID: "metadata.required.keywords", field: "keywords", want: `asc localizations update --id "ver-loc-1" --keywords "KEYWORDS"`},
		{checkID: "metadata.required.support_url", field: "supportUrl", want: `asc localizations update --id "ver-loc-1" --support-url "https://example.com/support"`},
		{checkID: "legal.required.copyright", field: "copyright", want: fmt.Sprintf(`asc versions update --version-id "ver-1" --copyright "%d Your Company"`, time.Now().UTC().Year())},
		{checkID: "age_rating.missing_field", field: "advertising", want: `asc age-rating edit --app "app-1" --advertising "true|false"`},
		{checkID: "age_rating.missing_field", field: "alcoholTobaccoOrDrugUseOrReferences", want: `asc age-rating edit --app "app-1" --alcohol-tobacco-drug-use "NONE|INFREQUENT_OR_MILD|FREQUENT_OR_INTENSE"`},
		{checkID: "availability.missing", field: "appAvailabilityV2", want: `asc pricing availability create --app "app-1" --all-territories --available true --available-in-new-territories true`},
		{checkID: "pricing.schedule.missing", field: "appPriceSchedule", want: `asc pricing schedule create --app "app-1" --free --base-territory "USA"`},
		{checkID: "screenshots.required.any", want: `asc screenshots upload --app "app-1" --version-id "ver-1" --locale "en-US" --path "./screenshots/en-US" --device-type "IPHONE_65"`},
	}
	for _, test := range tests {
		t.Run(test.checkID+"/"+test.field, func(t *testing.T) {
			var found *validation.CheckResult
			for i := range report.Checks {
				check := report.Checks[i]
				if check.ID == test.checkID && check.Severity == validation.SeverityError && (test.field == "" || check.Field == test.field) {
					found = &report.Checks[i]
					break
				}
			}
			if found == nil {
				t.Fatalf("no blocking %s (field %q) in report: %+v", test.checkID, test.field, report.Checks)
			}
			if found.Resolution == nil || found.Resolution.Fixability != validation.FixabilityAPIFixable || len(found.Resolution.Commands) != 1 || found.Resolution.Commands[0] != test.want {
				t.Fatalf("resolution = %+v, want api-fixable with command %s", found.Resolution, test.want)
			}
			if !strings.Contains(found.Remediation, test.want) && !strings.Contains(found.Remediation, strings.Join(strings.Fields(test.want)[:3], " ")) {
				t.Fatalf("remediation = %q, want it to name %s", found.Remediation, test.want)
			}
		})
	}

	for _, step := range report.Remediation.Steps {
		if step.Blocking && step.Resolution == nil && step.CheckID != "version.state.editable" {
			t.Errorf("blocking remediation step %s (field %q) has no resolution", step.CheckID, step.Field)
		}
	}
}
