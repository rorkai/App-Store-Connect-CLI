package shared

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// VerbatimText is implemented by errors whose message holds literal text, such
// as Apple's own message and commands to run, that a command-tree rewrite must
// leave as is.
type VerbatimText interface {
	error
	VerbatimText()
}

// betaReviewReadinessClient reads the TestFlight information App Store
// Connect checks before it accepts a build for external testing.
type betaReviewReadinessClient interface {
	GetBuildApp(ctx context.Context, buildID string) (*asc.AppResponse, error)
	GetBetaAppLocalizations(ctx context.Context, opts ...asc.BetaAppLocalizationsOption) (*asc.BetaAppLocalizationsResponse, error)
	GetBetaAppReviewDetails(ctx context.Context, opts ...asc.BetaAppReviewDetailsOption) (*asc.BetaAppReviewDetailsResponse, error)
	GetBuildBuildBetaDetail(ctx context.Context, buildID string) (*asc.BuildBetaDetailResponse, error)
}

// ExplainBetaReviewSubmissionError adds to submitErr, Apple's 422 for a beta
// app review submission, every empty field external testing needs and the
// command that sets it. Apple names at most one field per attempt, and for the
// feedback email and review contact it says only "Missing requied data.".
// Live (2026-11-22), a build was accepted once the beta app description,
// feedback email and the four review contact fields were set. A build that
// still needs export compliance is listed too.
//
// Apple also answers some apps whose information is complete with
// "There was an error processing your request.: Please try again later." and
// no field. Then it says that nothing asc can set is missing and who can
// clear it, because callers otherwise keep editing fields and retrying.
//
// submitErr is returned unchanged when it is not a 422, when a read fails, or
// when nothing is missing and Apple named its own reason, so Apple's error is
// never replaced.
func ExplainBetaReviewSubmissionError(ctx context.Context, client betaReviewReadinessClient, buildID string, submitErr error) error {
	var apiErr *asc.APIError
	if !errors.As(submitErr, &apiErr) || apiErr.HTTPStatusCode() != http.StatusUnprocessableEntity {
		return submitErr
	}
	explanation := describeMissingBetaReviewInfo(ctx, client, buildID, apiErr)
	if explanation == "" {
		return submitErr
	}
	return betaReviewSubmissionExplainedError{err: submitErr, explanation: explanation}
}

// betaReviewSubmissionExplainedError is Apple's submission error followed by
// what external testing still needs. Its text is kept verbatim by command-tree
// rewrites, so the commands it names stay runnable.
type betaReviewSubmissionExplainedError struct {
	err         error
	explanation string
}

func (e betaReviewSubmissionExplainedError) Error() string {
	return e.err.Error() + "\n" + e.explanation
}

func (e betaReviewSubmissionExplainedError) Unwrap() error { return e.err }

// VerbatimText marks the error text as literal: Apple's message and commands.
func (betaReviewSubmissionExplainedError) VerbatimText() {}

func describeMissingBetaReviewInfo(ctx context.Context, client betaReviewReadinessClient, buildID string, apiErr *asc.APIError) string {
	appResp, err := client.GetBuildApp(ctx, buildID)
	if err != nil || appResp == nil || strings.TrimSpace(appResp.Data.ID) == "" {
		return ""
	}
	appID := strings.TrimSpace(appResp.Data.ID)
	localizations, err := client.GetBetaAppLocalizations(ctx, asc.WithBetaAppLocalizationAppIDs([]string{appID}))
	if err != nil || localizations == nil {
		return ""
	}
	details, err := client.GetBetaAppReviewDetails(ctx, asc.WithBetaAppReviewDetailsAppID(appID))
	if err != nil || details == nil {
		return ""
	}

	// The external beta state is extra evidence: when it cannot be read, the
	// TestFlight information above is still reported.
	externalState := ""
	if detail, err := client.GetBuildBuildBetaDetail(ctx, buildID); err == nil && detail != nil {
		externalState = strings.ToUpper(strings.TrimSpace(detail.Data.Attributes.ExternalBuildState))
	}

	var missing, fixes []string
	missing, fixes = appendMissingBetaLocalizationInfo(missing, fixes, appID, localizations.Data)
	if len(details.Data) > 0 {
		missing, fixes = appendMissingBetaReviewDetailInfo(missing, fixes, details.Data[0])
	}
	if externalState == externalBetaStateMissingExportCompliance {
		missing = append(missing, "export compliance for build "+buildID)
		fixes = append(fixes,
			fmt.Sprintf("asc builds update --build-id %q --uses-non-exempt-encryption=false", buildID),
			fmt.Sprintf("(or, if the app uses non-exempt encryption: asc encryption declarations assign-builds --id \"DECLARATION_ID\" --build-id %q)", buildID),
		)
	}
	if len(missing) == 0 {
		if externalState == externalBetaStateReadyForBetaSubmission && len(details.Data) > 0 && betaReviewErrorNamesNoReason(apiErr) {
			return describeUnexplainedBetaReviewRejection(buildID, apiErr)
		}
		return ""
	}

	var b strings.Builder
	b.WriteString("App Store Connect needs this TestFlight information before external testing:\n")
	for _, item := range missing {
		fmt.Fprintf(&b, "  - %s\n", item)
	}
	b.WriteString("Set it with:\n")
	for _, fix := range fixes {
		fmt.Fprintf(&b, "  %s\n", fix)
	}
	fmt.Fprintf(&b, "Then run again: asc testflight review submit --build-id %q --confirm", buildID)
	return b.String()
}

// betaReviewErrorNamesNoReason reports whether every entry of Apple's 422 is
// the bare ENTITY_UNPROCESSABLE or MISSING_REQUIRED_DATA code, which name no
// cause, as opposed to a specific one such as another build in review.
func betaReviewErrorNamesNoReason(apiErr *asc.APIError) bool {
	if len(apiErr.AllCodes) == 0 {
		return false
	}
	for _, code := range apiErr.AllCodes {
		switch strings.ToUpper(strings.TrimSpace(code)) {
		case "ENTITY_UNPROCESSABLE", "ENTITY_UNPROCESSABLE.MISSING_REQUIRED_DATA":
		default:
			return false
		}
	}
	return true
}

func describeUnexplainedBetaReviewRejection(buildID string, apiErr *asc.APIError) string {
	var b strings.Builder
	fmt.Fprintf(&b, "App Store Connect gave no reason (%s). Nothing asc can set is missing: ", SanitizeTerminal(apiErr.AllCodes[0]))
	fmt.Fprintf(&b, "the beta app description, feedback email and beta app review contact are set, and build %s is %s.\n", SanitizeTerminal(buildID), externalBetaStateReadyForBetaSubmission)
	b.WriteString("Editing TestFlight information or retrying the same request does not change this answer. Someone with App Store Connect access needs to:\n")
	b.WriteString("  - accept any pending agreement (Account Holder): https://appstoreconnect.apple.com/agreements\n")
	b.WriteString("  - or submit the build in App Store Connect > TestFlight, and contact Apple Developer Support (https://developer.apple.com/contact/) if it fails there too")
	return b.String()
}

func appendMissingBetaLocalizationInfo(missing, fixes []string, appID string, localizations []asc.Resource[asc.BetaAppLocalizationAttributes]) ([]string, []string) {
	if len(localizations) == 0 {
		missing = append(missing, "beta app description", "feedback email")
		fixes = append(fixes, fmt.Sprintf("asc testflight app-localizations create --app %q --locale \"en-US\" --description \"DESCRIPTION\" --feedback-email \"EMAIL\"", appID))
		return missing, fixes
	}
	hasDescription, hasFeedbackEmail := false, false
	for _, localization := range localizations {
		hasDescription = hasDescription || strings.TrimSpace(localization.Attributes.Description) != ""
		hasFeedbackEmail = hasFeedbackEmail || strings.TrimSpace(localization.Attributes.FeedbackEmail) != ""
	}
	var flags []string
	if !hasDescription {
		missing = append(missing, "beta app description")
		flags = append(flags, `--description "DESCRIPTION"`)
	}
	if !hasFeedbackEmail {
		missing = append(missing, "feedback email")
		flags = append(flags, `--feedback-email "EMAIL"`)
	}
	if len(flags) > 0 {
		fixes = append(fixes, fmt.Sprintf("asc testflight app-localizations update --id %q %s", localizations[0].ID, strings.Join(flags, " ")))
	}
	return missing, fixes
}

func appendMissingBetaReviewDetailInfo(missing, fixes []string, detail asc.Resource[asc.BetaAppReviewDetailAttributes]) ([]string, []string) {
	attrs := detail.Attributes
	var flags []string
	var contact []string
	for _, field := range []struct{ label, value, flag string }{
		{"first name", attrs.ContactFirstName, `--contact-first-name "FIRST_NAME"`},
		{"last name", attrs.ContactLastName, `--contact-last-name "LAST_NAME"`},
		{"phone", attrs.ContactPhone, `--contact-phone "PHONE"`},
		{"email", attrs.ContactEmail, `--contact-email "EMAIL"`},
	} {
		if strings.TrimSpace(field.value) == "" {
			contact = append(contact, field.label)
			flags = append(flags, field.flag)
		}
	}
	if len(contact) > 0 {
		missing = append(missing, "beta app review contact "+strings.Join(contact, ", "))
	}
	if attrs.DemoAccountRequired {
		var demo []string
		if strings.TrimSpace(attrs.DemoAccountName) == "" {
			demo = append(demo, "name")
			flags = append(flags, `--demo-account-name "NAME"`)
		}
		if strings.TrimSpace(attrs.DemoAccountPassword) == "" {
			demo = append(demo, "password")
			flags = append(flags, `--demo-account-password "PASSWORD"`)
		}
		if len(demo) > 0 {
			missing = append(missing, "demo account "+strings.Join(demo, ", ")+" (the demo account is marked required)")
		}
	}
	if len(flags) > 0 {
		fixes = append(fixes, fmt.Sprintf("asc testflight review edit --id %q %s", detail.ID, strings.Join(flags, " ")))
	}
	return missing, fixes
}

// CheckBuildAppOwnership verifies the selected app before acting on an existing build.
func CheckBuildAppOwnership(ctx context.Context, client betaReviewReadinessClient, appID, buildID string) error {
	app, err := client.GetBuildApp(ctx, buildID)
	if err != nil {
		return fmt.Errorf("read build app: %w", err)
	}
	if app == nil || app.Data.Type != "apps" || app.Data.ID != appID {
		return fmt.Errorf("build %s does not belong to app %s", buildID, appID)
	}
	return nil
}

// CheckBuildBetaReviewReadiness checks external testing requirements before mutations.
func CheckBuildBetaReviewReadiness(ctx context.Context, client buildBetaReviewSubmissionClient, appID, buildID string) error {
	submission, err := client.GetBuildBetaAppReviewSubmission(ctx, buildID)
	if err == nil {
		if submission == nil || submission.Data.Type != "betaAppReviewSubmissions" || strings.TrimSpace(submission.Data.ID) == "" {
			return fmt.Errorf("beta review submission response is missing its ID")
		}
		return nil
	}
	if !asc.IsNotFound(err) {
		return fmt.Errorf("read beta review submission: %w", err)
	}
	first, err := client.GetBetaAppLocalizations(ctx, asc.WithBetaAppLocalizationAppIDs([]string{appID}))
	if err != nil {
		return fmt.Errorf("read beta app localizations: %w", err)
	}
	if err := validateBetaReviewLocalizationPage(first); err != nil {
		return err
	}
	all, err := asc.PaginateAll(ctx, first, func(ctx context.Context, next string) (asc.PaginatedResponse, error) {
		boundNext, err := BindCollectionNextURL(ctx, client, next, "/v1/betaAppLocalizations", url.Values{"filter[app]": {appID}})
		if err != nil {
			return nil, err
		}
		page, err := client.GetBetaAppLocalizations(ctx, asc.NextURL[asc.BetaAppLocalizationsOption](boundNext))
		if err == nil {
			err = validateBetaReviewLocalizationPage(page)
		}
		return page, err
	})
	if err != nil {
		return err
	}
	localizations, ok := all.(*asc.BetaAppLocalizationsResponse)
	if !ok || localizations == nil {
		return fmt.Errorf("invalid beta app localizations response")
	}
	details, err := client.GetBetaAppReviewDetails(ctx, asc.WithBetaAppReviewDetailsAppID(appID))
	if err != nil {
		return fmt.Errorf("read beta review details: %w", err)
	}
	beta, err := client.GetBuildBuildBetaDetail(ctx, buildID)
	if err != nil {
		return fmt.Errorf("read beta build details: %w", err)
	}
	if beta == nil || beta.Data.Type != "buildBetaDetails" || strings.TrimSpace(beta.Data.ID) == "" {
		return fmt.Errorf("invalid build beta details response")
	}
	if details != nil {
		for _, detail := range details.Data {
			if detail.Type != "betaAppReviewDetails" || strings.TrimSpace(detail.ID) == "" {
				return fmt.Errorf("invalid beta app review details resource")
			}
		}
	}
	missing, fixes := appendMissingBetaLocalizationInfo(nil, nil, appID, localizations.Data)
	if details == nil || len(details.Data) == 0 {
		missing = append(missing, "beta app review details")
	} else {
		missing, fixes = appendMissingBetaReviewDetailInfo(missing, fixes, details.Data[0])
	}
	if beta != nil && strings.EqualFold(strings.TrimSpace(beta.Data.Attributes.ExternalBuildState), externalBetaStateMissingExportCompliance) {
		missing = append(missing, "export compliance")
	}
	if len(missing) > 0 {
		message := fmt.Sprintf("external testing requires %s before groups can be assigned or testers notified", strings.Join(missing, ", "))
		if len(fixes) > 0 {
			message += ". Set the missing values with " + strings.Join(fixes, ". Then run ")
		}
		return WithDiagnostic(NewValidationError(errors.New(message)), DiagnosticStateNotReady, "")
	}
	return nil
}

func validateBetaReviewLocalizationPage(page *asc.BetaAppLocalizationsResponse) error {
	if page == nil || page.Data == nil {
		return fmt.Errorf("missing beta app localizations response data")
	}
	for _, localization := range page.Data {
		if localization.Type != "betaAppLocalizations" || strings.TrimSpace(localization.ID) == "" {
			return fmt.Errorf("invalid beta app localization resource")
		}
	}
	return nil
}
