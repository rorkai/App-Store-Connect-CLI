package cmdtest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

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

	// userInput rows hold a decision only the developer can make, so their
	// commands must carry placeholders and needsUserInput instead of a choice.
	tests := []struct {
		checkID   string
		field     string
		want      []string
		userInput bool
		// note is what the remediation text says a placeholder may be.
		note string
	}{
		{checkID: "build.required.missing", want: []string{`asc versions attach-build --app "app-1" --version-id "ver-1" --latest`}},
		{checkID: "screenshots.required.any", userInput: true, want: []string{`asc screenshots upload --app "app-1" --version-id "ver-1" --locale "en-US" --path "SCREENSHOTS_DIR" --device-type "DEVICE_TYPE"`}},
		{checkID: "categories.primary_missing", field: "primaryCategory", userInput: true, want: []string{`asc categories set --app "app-1" --app-info "info-1" --primary "CATEGORY_ID"`}},
		{checkID: "content_rights.missing", field: "contentRightsDeclaration", userInput: true, want: []string{`asc apps update --id "app-1" --content-rights "USES_THIRD_PARTY_CONTENT_OR_DOES_NOT_USE_THIRD_PARTY_CONTENT"`}},
		{checkID: "review_details.missing", userInput: true, want: []string{`asc review details-create --version-id "ver-1" --contact-first-name "FIRST_NAME" --contact-last-name "LAST_NAME" --contact-email "EMAIL" --contact-phone "PHONE"`}},
		{checkID: "metadata.required.description", field: "description", userInput: true, want: []string{`asc localizations update --id "ver-loc-1" --description "DESCRIPTION"`}},
		{checkID: "metadata.required.keywords", field: "keywords", userInput: true, want: []string{`asc localizations update --id "ver-loc-1" --keywords "KEYWORDS"`}},
		{checkID: "metadata.required.support_url", field: "supportUrl", userInput: true, want: []string{`asc localizations update --id "ver-loc-1" --support-url "SUPPORT_URL"`}},
		{checkID: "legal.required.copyright", field: "copyright", userInput: true, want: []string{`asc versions update --version-id "ver-1" --copyright "YEAR RIGHTS_HOLDER"`}},
		{checkID: "age_rating.missing_field", field: "sharedLocation", userInput: true, want: []string{`asc age-rating edit --app "app-1" --app-info-id "info-1" --shared-location "TRUE_OR_FALSE"`}},
		{checkID: "age_rating.missing_field", field: "datingAndRelationships", userInput: true, want: []string{`asc age-rating edit --app "app-1" --app-info-id "info-1" --dating-and-relationships "TRUE_OR_FALSE"`}},
		{checkID: "age_rating.missing_field", field: "advertising", userInput: true, want: []string{`asc age-rating edit --app "app-1" --app-info-id "info-1" --advertising "TRUE_OR_FALSE"`}},
		{checkID: "age_rating.missing_field", field: "alcoholTobaccoOrDrugUseOrReferences", userInput: true, want: []string{`asc age-rating edit --app "app-1" --app-info-id "info-1" --alcohol-tobacco-drug-use "LEVEL"`}, note: "LEVEL is NONE, INFREQUENT_OR_MILD, FREQUENT_OR_INTENSE, INFREQUENT, or FREQUENT"},
		{checkID: "availability.missing", field: "appAvailabilityV2", userInput: true, want: []string{`asc pricing availability create --app "app-1" --territory "TERRITORIES" --available true --available-in-new-territories "TRUE_OR_FALSE"`}},
		{checkID: "pricing.schedule.missing", field: "appPriceSchedule", userInput: true, want: []string{
			`asc pricing schedule create --app "app-1" --price-point "PRICE_POINT_ID" --base-territory "BASE_TERRITORY"`,
			`asc pricing schedule create --app "app-1" --free --base-territory "BASE_TERRITORY"`,
		}},
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
			if found.Resolution == nil || found.Resolution.Fixability != validation.FixabilityAPIFixable || !slices.Equal(found.Resolution.Commands, test.want) || found.Resolution.NeedsUserInput != test.userInput {
				t.Fatalf("resolution = %+v, want api-fixable commands %q with needsUserInput=%v", found.Resolution, test.want, test.userInput)
			}
			if !strings.Contains(found.Remediation, strings.Join(strings.Fields(test.want[0])[:3], " ")) {
				t.Fatalf("remediation = %q, want it to name %s", found.Remediation, test.want[0])
			}
			if test.userInput != strings.Contains(found.Remediation, "placeholder") {
				t.Fatalf("remediation = %q, want the placeholder note only when the developer must answer", found.Remediation)
			}
			if !strings.Contains(found.Remediation, test.note) {
				t.Fatalf("remediation = %q, want it to say %q", found.Remediation, test.note)
			}
		})
	}

	for _, step := range report.Remediation.Steps {
		if step.Blocking && step.Resolution == nil && step.CheckID != "version.state.editable" {
			t.Errorf("blocking remediation step %s (field %q) has no resolution", step.CheckID, step.Field)
		}
	}
}

func TestValidateAgeRatingFixUsesSelectedAppInfo(t *testing.T) {
	for _, tt := range []struct {
		versionState, nextInfoState, wantInfoID string
	}{
		{"READY_FOR_SALE", "PREPARE_FOR_SUBMISSION", "info-live"},
		{"PREPARE_FOR_SUBMISSION", "WAITING_FOR_REVIEW", "info-1"},
	} {
		t.Run(tt.versionState, func(t *testing.T) {
			fixture := validValidateFixture()
			fixture.version = strings.ReplaceAll(fixture.version, "PREPARE_FOR_SUBMISSION", tt.versionState)
			fixture.appInfos = fmt.Sprintf(`{"data":[
				{"type":"appInfos","id":"info-1","attributes":{"state":%q},"relationships":{"ageRatingDeclaration":{"data":{"type":"ageRatingDeclarations","id":"age-next"}}}},
				{"type":"appInfos","id":"info-live","attributes":{"state":"READY_FOR_DISTRIBUTION"},"relationships":{"ageRatingDeclaration":{"data":{"type":"ageRatingDeclarations","id":"age-live"}}}}
			],"included":[
				{"type":"ageRatingDeclarations","id":"age-live","attributes":{}},
				{"type":"ageRatingDeclarations","id":"age-next","attributes":{}}
			]}`, tt.nextInfoState)
			report, err := runValidateReport(t, fixture)
			if err == nil || !strings.Contains(err.Error(), "blocking issue(s)") {
				t.Fatalf("validate: %v", err)
			}
			want := fmt.Sprintf(`asc age-rating edit --app "app-1" --app-info-id %q --shared-location "TRUE_OR_FALSE"`, tt.wantInfoID)
			for _, check := range report.Checks {
				if check.ID == "age_rating.missing_field" && check.Field == "sharedLocation" {
					if check.Resolution == nil || !slices.Equal(check.Resolution.Commands, []string{want}) {
						t.Fatalf("resolution = %+v, want %s", check.Resolution, want)
					}
					return
				}
			}
			t.Fatal("missing sharedLocation finding")
		})
	}
}

// TestValidateEmptyScreenshotSetOffersUploadOrDelete covers a set left empty
// after its screenshots were deleted: uploading to it and deleting the set
// both clear the blocker, so the report must name both commands.
func TestValidateEmptyScreenshotSetOffersUploadOrDelete(t *testing.T) {
	fixture := validValidateFixture()
	fixture.screenshotsBySet = map[string]string{"set-1": `{"data":[]}`}
	client := newValidateTestClient(t, fixture)
	stdout, _, runErr := runRootWithDeps(t, func() (*asc.Client, error) { return client, nil }, "validate", "--app", "app-1", "--version-id", "ver-1", "--output", "json")
	if runErr == nil || !strings.Contains(runErr.Error(), "blocking issue(s)") {
		t.Fatalf("run error = %v, want the blocking-issues error", runErr)
	}
	var report validation.Report
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("parse report: %v\n%s", err, stdout)
	}

	want := []string{
		`asc screenshots upload --app "app-1" --version-id "ver-1" --locale "en-US" --path "SCREENSHOTS_DIR" --device-type "IPHONE_65"`,
		`asc localizations screenshot-sets delete --id "set-1" --confirm`,
	}
	for _, check := range report.Checks {
		if check.ID != "screenshots.required.set_nonempty" {
			continue
		}
		if check.Resolution == nil || check.Resolution.Fixability != validation.FixabilityAPIFixable || !slices.Equal(check.Resolution.Commands, want) || !check.Resolution.NeedsUserInput {
			t.Fatalf("resolution = %+v, want api-fixable commands %q with needsUserInput", check.Resolution, want)
		}
		for _, command := range want {
			if !strings.Contains(check.Remediation, command) {
				t.Fatalf("remediation = %q, want it to name %s", check.Remediation, command)
			}
		}
		return
	}
	t.Fatalf("no screenshots.required.set_nonempty check in report: %+v", report.Checks)
}
