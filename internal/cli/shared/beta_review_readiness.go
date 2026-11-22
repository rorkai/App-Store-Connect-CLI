package shared

import (
	"context"
	"errors"
	"fmt"
	"net/http"
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
}

// ExplainBetaReviewSubmissionError adds to submitErr, Apple's 422 for a beta
// app review submission, every empty field external testing needs and the
// command that sets it. Apple names at most one field per attempt, and for the
// feedback email and review contact it says only "Missing requied data.".
// Live (2026-11-22), a build was accepted once the beta app description,
// feedback email and the four review contact fields were set.
//
// submitErr is returned unchanged when it is not a 422, when a read fails, or
// when none of those fields is empty, so Apple's error is never replaced.
func ExplainBetaReviewSubmissionError(ctx context.Context, client betaReviewReadinessClient, buildID string, submitErr error) error {
	var apiErr *asc.APIError
	if !errors.As(submitErr, &apiErr) || apiErr.HTTPStatusCode() != http.StatusUnprocessableEntity {
		return submitErr
	}
	explanation := describeMissingBetaReviewInfo(ctx, client, buildID)
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

func describeMissingBetaReviewInfo(ctx context.Context, client betaReviewReadinessClient, buildID string) string {
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

	var missing, fixes []string
	missing, fixes = appendMissingBetaLocalizationInfo(missing, fixes, appID, localizations.Data)
	if len(details.Data) > 0 {
		missing, fixes = appendMissingBetaReviewDetailInfo(missing, fixes, details.Data[0])
	}
	if len(missing) == 0 {
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
