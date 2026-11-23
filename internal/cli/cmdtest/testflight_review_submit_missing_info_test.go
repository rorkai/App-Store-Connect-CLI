package cmdtest

import (
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

// Apple's live 422 for a build whose app has a beta app description but no
// feedback email or review contact (disposable app, 2026-11-22).
const betaReviewMissingDataBody = `{"errors":[{"id":"1ccd2628-6151-4418-88c8-6e125020b132","status":"422","code":"ENTITY_UNPROCESSABLE.MISSING_REQUIRED_DATA","title":"Missing required information to submit for external testing","detail":"Missing requied data."}]}`

// Apple's 422 for a build whose TestFlight information is complete, which
// names no reason.
const betaReviewNoReasonBody = `{"errors":[{"id":"6a1f0c3e-2b7d-4e59-9c1a-0f4b8d2e7a61","status":"422","code":"ENTITY_UNPROCESSABLE","title":"There was an error processing your request.","detail":"Please try again later."}]}`

// A 422 from the submission names one missing field at a time, or none
// ("Missing requied data.", "Please try again later."). The command reads the
// app's TestFlight information and the build's external beta state and lists
// every empty field external testing needs, with the command that sets it.
// When nothing is missing and Apple names no reason, it says so, so the
// caller does not keep editing fields or retrying. It adds nothing when Apple
// names its own reason.
func TestTestFlightReviewSubmitExplainsMissingExternalTestingInfo(t *testing.T) {
	const (
		filledLocalization = `{"data":[{"type":"betaAppLocalizations","id":"loc-1","attributes":{"description":"A beta","feedbackEmail":"beta@example.com","locale":"en-US"}}],"links":{}}`
		filledDetail       = `{"data":[{"type":"betaAppReviewDetails","id":"app-1","attributes":{"contactEmail":"dev@example.com","contactFirstName":"Ada","contactLastName":"Lovelace","contactPhone":"+1 408 555 0100","demoAccountRequired":false}}],"links":{}}`
	)
	tests := []struct {
		name          string
		submitBody    string
		localizations string
		detail        string
		externalState string
		wantAdded     []string
	}{
		{
			name:          "feedback email and review contact missing",
			submitBody:    betaReviewMissingDataBody,
			localizations: `{"data":[{"type":"betaAppLocalizations","id":"loc-1","attributes":{"description":"A beta","feedbackEmail":null,"locale":"en-US"}}],"links":{}}`,
			detail:        `{"data":[{"type":"betaAppReviewDetails","id":"app-1","attributes":{"contactEmail":null,"contactFirstName":null,"contactLastName":null,"contactPhone":null,"demoAccountRequired":null}}],"links":{}}`,
			externalState: "READY_FOR_BETA_SUBMISSION",
			wantAdded: []string{
				"App Store Connect needs this TestFlight information before external testing:",
				"  - feedback email",
				"  - beta app review contact first name, last name, phone, email",
				`  asc testflight app-localizations update --id "loc-1" --feedback-email "EMAIL"`,
				`  asc testflight review edit --id "app-1" --contact-first-name "FIRST_NAME" --contact-last-name "LAST_NAME" --contact-phone "PHONE" --contact-email "EMAIL"`,
				`Then run again: asc testflight review submit --build-id "build-1" --confirm`,
			},
		},
		{
			name:          "no beta app localization and a required demo account",
			submitBody:    `{"errors":[{"status":"422","code":"ENTITY_UNPROCESSABLE","title":"betaAppLocalizations not found for this app.","detail":"betaAppLocalization must exist for an app"}]}`,
			localizations: `{"data":[],"links":{}}`,
			detail:        `{"data":[{"type":"betaAppReviewDetails","id":"app-1","attributes":{"contactEmail":"dev@example.com","contactFirstName":"Ada","contactLastName":"Lovelace","contactPhone":"+1 408 555 0100","demoAccountRequired":true}}],"links":{}}`,
			externalState: "READY_FOR_BETA_SUBMISSION",
			wantAdded: []string{
				"  - beta app description",
				"  - feedback email",
				"  - demo account name, password (the demo account is marked required)",
				`  asc testflight app-localizations create --app "app-1" --locale "en-US" --description "DESCRIPTION" --feedback-email "EMAIL"`,
				`  asc testflight review edit --id "app-1" --demo-account-name "NAME" --demo-account-password "PASSWORD"`,
			},
		},
		{
			name:          "build missing export compliance",
			submitBody:    betaReviewMissingDataBody,
			localizations: filledLocalization,
			detail:        filledDetail,
			externalState: "MISSING_EXPORT_COMPLIANCE",
			wantAdded: []string{
				"  - export compliance for build build-1",
				`  asc builds update --build-id "build-1" --uses-non-exempt-encryption=false`,
				`Then run again: asc testflight review submit --build-id "build-1" --confirm`,
			},
		},
		{
			name:          "nothing missing and Apple names no reason",
			submitBody:    betaReviewNoReasonBody,
			localizations: filledLocalization,
			detail:        filledDetail,
			externalState: "READY_FOR_BETA_SUBMISSION",
			wantAdded: []string{
				"\nApp Store Connect gave no reason (ENTITY_UNPROCESSABLE). Nothing asc can set is missing:",
				"beta app description, feedback email and beta app review contact are set, and build build-1 is READY_FOR_BETA_SUBMISSION.",
				"Editing TestFlight information or retrying the same request does not change this answer.",
				"https://appstoreconnect.apple.com/agreements",
			},
		},
		{
			name:          "nothing missing and Apple names its reason",
			submitBody:    `{"errors":[{"status":"422","code":"ENTITY_UNPROCESSABLE.ANOTHER_BUILD_IN_REVIEW","title":"Another build is in review.","detail":"Another build of this app is already in beta app review."}]}`,
			localizations: filledLocalization,
			detail:        filledDetail,
			externalState: "READY_FOR_BETA_SUBMISSION",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.POST("/v1/betaAppReviewSubmissions"), asctest.JSON(http.StatusUnprocessableEntity, test.submitBody))
			reg.Register(asctest.GET("/v1/builds/build-1/app"), asctest.JSON(http.StatusOK, `{"data":{"type":"apps","id":"app-1","attributes":{"name":"App"}}}`))
			reg.Register(asctest.GET("/v1/betaAppLocalizations", "filter[app]=app-1"), asctest.JSON(http.StatusOK, test.localizations))
			reg.Register(asctest.GET("/v1/betaAppReviewDetails", "filter[app]=app-1"), asctest.JSON(http.StatusOK, test.detail))
			reg.Register(asctest.GET("/v1/builds/build-1/buildBetaDetail"), asctest.JSON(http.StatusOK, `{"data":{"type":"buildBetaDetails","id":"build-1","attributes":{"externalBuildState":"`+test.externalState+`"}}}`))

			stdout, stderr, runErr := runCommand(t, []string{"testflight", "review", "submit", "--build-id", "build-1", "--confirm"})
			if runErr == nil {
				t.Fatal("expected the submission error")
			}
			var apiErr *asc.APIError
			if !errors.As(runErr, &apiErr) || apiErr.HTTPStatusCode() != http.StatusUnprocessableEntity {
				t.Fatalf("expected Apple's 422 to stay in the error chain, got %v", runErr)
			}
			message := runErr.Error()
			if !strings.HasPrefix(message, "testflight review submit: failed to submit: ") {
				t.Fatalf("expected Apple's error first, got %q", message)
			}
			for _, want := range test.wantAdded {
				if !strings.Contains(message, want) {
					t.Errorf("error is missing %q:\n%s", want, message)
				}
			}
			if len(test.wantAdded) == 0 && strings.Contains(message, "\n") {
				t.Errorf("expected Apple's error alone, got:\n%s", message)
			}
			if stdout != "" || stderr != "" {
				t.Fatalf("expected no output, got stdout=%q stderr=%q", stdout, stderr)
			}
		})
	}
}
