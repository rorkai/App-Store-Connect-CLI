package shared

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// External beta states are defined by the App Store Connect OpenAPI schema.
// Keep the values here local to the state evaluator so callers do not need to
// duplicate provider strings or infer readiness from a human-facing message.
const (
	externalBetaStateProcessing              = "PROCESSING"
	externalBetaStateProcessingException     = "PROCESSING_EXCEPTION"
	externalBetaStateMissingExportCompliance = "MISSING_EXPORT_COMPLIANCE"
	externalBetaStateReadyForBetaTesting     = "READY_FOR_BETA_TESTING"
	externalBetaStateReadyForTesting         = "READY_FOR_TESTING"
	externalBetaStateNotReadyForTesting      = "NOT_READY_FOR_TESTING"
	externalBetaStateInBetaTesting           = "IN_BETA_TESTING"
	externalBetaStateExpired                 = "EXPIRED"
	externalBetaStateReadyForBetaSubmission  = "READY_FOR_BETA_SUBMISSION"
	externalBetaStateInExportComplianceRev   = "IN_EXPORT_COMPLIANCE_REVIEW"
	externalBetaStateWaitingForBetaReview    = "WAITING_FOR_BETA_REVIEW"
	externalBetaStateInBetaReview            = "IN_BETA_REVIEW"
	externalBetaStateBetaRejected            = "BETA_REJECTED"
	externalBetaStateBetaApproved            = "BETA_APPROVED"
	externalBetaStateNotApplicable           = "NOT_APPLICABLE"
)

type buildBetaGroupPreflightClient interface {
	GetBuild(ctx context.Context, buildID string, opts ...asc.BuildOption) (*asc.BuildResponse, error)
	GetBuildBuildBetaDetail(ctx context.Context, buildID string) (*asc.BuildBetaDetailResponse, error)
}

// BuildBetaGroupPreflightOptions controls diagnostics for the assignment.
type BuildBetaGroupPreflightOptions struct {
	OperationName string
}

type buildBetaGroupPrecondition struct {
	Summary string
	Fix     string
}

// PreflightBuildBetaGroupAssignment checks the provider state required for a
// relationship assignment. A normal assignment fails closed when the current
// state cannot prove that the relationship is acceptable; this keeps an
// unavailable or incomplete read from turning into an avoidable POST. The
// pure state checks are shared with the diagnostic wording used by callers.
func PreflightBuildBetaGroupAssignment(
	ctx context.Context,
	client buildBetaGroupPreflightClient,
	buildID string,
	plan BuildBetaGroupAssignmentPlan,
	opts BuildBetaGroupPreflightOptions,
) error {
	return preflightBuildBetaGroupAssignmentTo(ctx, os.Stderr, client, buildID, plan, opts)
}

// PreflightBuildBetaGroupAssignment is PreflightBuildBetaGroupAssignment with
// the "Error:" lines written to d's stderr.
func (d *Deps) PreflightBuildBetaGroupAssignment(
	ctx context.Context,
	client buildBetaGroupPreflightClient,
	buildID string,
	plan BuildBetaGroupAssignmentPlan,
	opts BuildBetaGroupPreflightOptions,
) error {
	return preflightBuildBetaGroupAssignmentTo(ctx, d.IO.ErrOut, client, buildID, plan, opts)
}

func preflightBuildBetaGroupAssignmentTo(
	ctx context.Context,
	stderr io.Writer,
	client buildBetaGroupPreflightClient,
	buildID string,
	plan BuildBetaGroupAssignmentPlan,
	opts BuildBetaGroupPreflightOptions,
) error {
	buildID = strings.TrimSpace(buildID)
	if buildID == "" || len(plan.GroupsToAdd) == 0 {
		return nil
	}

	build, err := client.GetBuild(ctx, buildID)
	if err != nil {
		return reportBuildBetaGroupPreflightUnknown(stderr, opts.OperationName, buildID, "build state", err)
	}
	if build == nil {
		return reportBuildBetaGroupPreflightUnknown(stderr, opts.OperationName, buildID, "build state", nil)
	}

	if precondition, ok := evaluateBuildBetaGroupBuildState(buildID, build.Data.Attributes, plan.IncludesExternalGroup()); ok {
		return reportBuildBetaGroupPrecondition(stderr, opts.OperationName, precondition)
	}
	if !plan.IncludesExternalGroup() {
		return nil
	}

	detail, err := client.GetBuildBuildBetaDetail(ctx, buildID)
	if err != nil {
		return reportBuildBetaGroupPreflightUnknown(stderr, opts.OperationName, buildID, "external beta state", err)
	}
	if detail == nil {
		return reportBuildBetaGroupPreflightUnknown(stderr, opts.OperationName, buildID, "external beta state", nil)
	}

	if precondition, ok := evaluateBuildBetaGroupExternalState(buildID, build.Data.Attributes, detail.Data.Attributes.ExternalBuildState); ok {
		return reportBuildBetaGroupPrecondition(stderr, opts.OperationName, precondition)
	}
	return nil
}

func evaluateBuildBetaGroupBuildState(buildID string, attributes asc.BuildAttributes, includesExternal bool) (buildBetaGroupPrecondition, bool) {
	state := strings.ToUpper(strings.TrimSpace(attributes.ProcessingState))
	switch state {
	case asc.BuildProcessingStateProcessing:
		return buildBetaGroupPrecondition{
			Summary: fmt.Sprintf("build %s is still processing (processingState %s) and cannot be assigned to beta groups yet", buildID, state),
			Fix:     fmt.Sprintf("Wait for processing to finish, then retry: asc builds wait --build-id %q", buildID),
		}, true
	case asc.BuildProcessingStateFailed:
		return buildBetaGroupPrecondition{
			Summary: fmt.Sprintf("build %s failed processing (processingState %s) and can never be assigned to beta groups", buildID, state),
			Fix:     replacementBuildGuidance(),
		}, true
	case asc.BuildProcessingStateInvalid:
		return buildBetaGroupPrecondition{
			Summary: fmt.Sprintf("build %s is invalid (processingState %s) and cannot be assigned to beta groups", buildID, state),
			Fix:     replacementBuildGuidance(),
		}, true
	case asc.BuildProcessingStateValid:
		// Continue with the expiry and external-only checks below.
	case "":
		return unknownBuildStatePrecondition(buildID, "processingState is empty")
	default:
		return unknownBuildStatePrecondition(buildID, fmt.Sprintf("processingState %q is not a recognized App Store Connect state", state))
	}

	expired, known := attributes.ExpiredValue()
	if !known {
		return unknownBuildStatePrecondition(buildID, "expired was not reported by App Store Connect")
	}
	if expired {
		summary := fmt.Sprintf("build %s has expired and cannot be assigned to beta groups", buildID)
		if expiration := strings.TrimSpace(attributes.ExpirationDate); expiration != "" {
			summary = fmt.Sprintf("build %s has expired (expirationDate %s) and cannot be assigned to beta groups", buildID, expiration)
		}
		return buildBetaGroupPrecondition{
			Summary: summary,
			Fix:     `Pick a build that has not expired: asc builds list --app "APP_ID" --limit 5`,
		}, true
	}

	if !includesExternal {
		return buildBetaGroupPrecondition{}, false
	}

	audience := strings.ToUpper(strings.TrimSpace(string(attributes.BuildAudienceType)))
	switch audience {
	case string(asc.BuildAudienceTypeAppStoreEligible):
		return buildBetaGroupPrecondition{}, false
	case string(asc.BuildAudienceTypeInternalOnly):
		return buildBetaGroupPrecondition{
			Summary: fmt.Sprintf("build %s is marked buildAudienceType %s and cannot be assigned to external beta groups", buildID, audience),
			Fix:     `Use an internal beta group, or upload a build eligible for App Store distribution: asc builds upload --app "APP_ID" --ipa "PATH_TO_IPA"`,
		}, true
	case "":
		return unknownBuildStatePrecondition(buildID, "buildAudienceType was not reported for an external assignment")
	default:
		return unknownBuildStatePrecondition(buildID, fmt.Sprintf("buildAudienceType %q is not a recognized App Store Connect value", audience))
	}
}

func evaluateBuildBetaGroupExternalState(buildID string, attributes asc.BuildAttributes, rawState string) (buildBetaGroupPrecondition, bool) {
	state := strings.ToUpper(strings.TrimSpace(rawState))
	switch state {
	case externalBetaStateProcessing:
		return buildBetaGroupPrecondition{
			Summary: fmt.Sprintf("build %s is still processing for external testing (externalBuildState %s), so external beta groups cannot be assigned yet", buildID, state),
			Fix:     fmt.Sprintf("Wait for processing to finish, then retry: asc builds wait --build-id %q", buildID),
		}, true
	case externalBetaStateProcessingException:
		return buildBetaGroupPrecondition{
			Summary: fmt.Sprintf("build %s hit a processing exception for external testing (externalBuildState %s), so external beta groups cannot be assigned", buildID, state),
			Fix:     replacementBuildGuidance(),
		}, true
	case externalBetaStateMissingExportCompliance:
		return buildBetaGroupPrecondition{
			Summary: fmt.Sprintf("build %s is missing an export-compliance declaration (externalBuildState %s), so external beta groups cannot be assigned", buildID, state),
			Fix: fmt.Sprintf(
				"Declare encryption use: asc builds update --build-id %q --uses-non-exempt-encryption=false\nOr assign an existing declaration: asc encryption declarations assign-builds --id \"DECLARATION_ID\" --build-id %q",
				buildID,
				buildID,
			),
		}, true
	case externalBetaStateInExportComplianceRev:
		return buildBetaGroupPrecondition{
			Summary: fmt.Sprintf("build %s is in export-compliance review (externalBuildState %s), so external beta groups cannot be assigned yet", buildID, state),
			Fix:     fmt.Sprintf("Re-check the state and retry once review finishes: asc builds info --build-id %q", buildID),
		}, true
	case externalBetaStateExpired:
		return buildBetaGroupPrecondition{
			Summary: fmt.Sprintf("build %s has expired for external testing (externalBuildState %s), so external beta groups cannot be assigned", buildID, state),
			Fix:     `Pick a build that has not expired: asc builds list --app "APP_ID" --limit 5`,
		}, true
	case externalBetaStateReadyForBetaSubmission:
		// Apple accepts the documented two-step workflow: stage the external
		// group first, then submit the build for review separately.
	case externalBetaStateWaitingForBetaReview, externalBetaStateInBetaReview:
		return buildBetaGroupPrecondition{
			Summary: fmt.Sprintf("build %s is awaiting beta app review (externalBuildState %s), so external beta groups cannot be assigned until review completes", buildID, state),
			Fix:     fmt.Sprintf("Wait for beta app review to complete, then re-check: asc builds info --build-id %q", buildID),
		}, true
	case externalBetaStateBetaRejected:
		return buildBetaGroupPrecondition{
			Summary: fmt.Sprintf("build %s did not pass beta app review (externalBuildState %s), so external beta groups cannot be assigned", buildID, state),
			Fix:     fmt.Sprintf("Address the rejection in App Store Connect, then resubmit: asc testflight review submit --build-id %q --confirm", buildID),
		}, true
	case externalBetaStateNotApplicable:
		return buildBetaGroupPrecondition{
			Summary: fmt.Sprintf("build %s is not eligible for external testing (externalBuildState %s), so external beta groups cannot be assigned", buildID, state),
			Fix:     `Use an internal beta group, or upload a build eligible for App Store distribution: asc builds upload --app "APP_ID" --ipa "PATH_TO_IPA"`,
		}, true
	case externalBetaStateNotReadyForTesting:
		return buildBetaGroupPrecondition{
			Summary: fmt.Sprintf("build %s is not ready for external testing (externalBuildState %s), so external beta groups cannot be assigned yet", buildID, state),
			Fix:     fmt.Sprintf("Re-check the current build state, then retry: asc builds info --build-id %q", buildID),
		}, true
	case externalBetaStateReadyForBetaTesting, externalBetaStateReadyForTesting, externalBetaStateInBetaTesting, externalBetaStateBetaApproved:
		// Continue to the export-compliance evidence check below. READY_FOR_TESTING
		// is retained for compatibility with responses observed before Apple
		// renamed the state in the current OpenAPI schema.
	case "":
		return unknownExternalStatePrecondition(buildID, "externalBuildState was not reported by App Store Connect")
	default:
		return unknownExternalStatePrecondition(buildID, fmt.Sprintf("externalBuildState %q is not a recognized App Store Connect state", state))
	}

	if attributes.UsesNonExemptEncryption == nil {
		return buildBetaGroupPrecondition{
			Summary: fmt.Sprintf("build %s has no export-compliance declaration (usesNonExemptEncryption was not reported), so external beta groups cannot be assigned", buildID),
			Fix: fmt.Sprintf(
				"Declare encryption use: asc builds update --build-id %q --uses-non-exempt-encryption=false\nOr assign an existing declaration: asc encryption declarations assign-builds --id \"DECLARATION_ID\" --build-id %q",
				buildID,
				buildID,
			),
		}, true
	}

	return buildBetaGroupPrecondition{}, false
}

func replacementBuildGuidance() string {
	return "Upload a replacement iOS build: asc builds upload --app \"APP_ID\" --ipa \"PATH_TO_IPA\"\nOr upload a replacement macOS build: asc builds upload --app \"APP_ID\" --pkg \"PATH_TO_PKG\" --version \"VERSION\" --build-number \"BUILD_NUMBER\""
}

func unknownBuildStatePrecondition(buildID, reason string) (buildBetaGroupPrecondition, bool) {
	return buildBetaGroupPrecondition{
		Summary: fmt.Sprintf("builds add-groups: cannot prove build %s is ready for beta-group assignment: %s", buildID, reason),
		Fix:     fmt.Sprintf("Re-check the current build state, then retry: asc builds info --build-id %q", buildID),
	}, true
}

func unknownExternalStatePrecondition(buildID, reason string) (buildBetaGroupPrecondition, bool) {
	return buildBetaGroupPrecondition{
		Summary: fmt.Sprintf("builds add-groups: cannot prove build %s is ready for external beta-group assignment: %s", buildID, reason),
		Fix:     fmt.Sprintf("Re-check the current build state, then retry: asc builds info --build-id %q", buildID),
	}, true
}

func reportBuildBetaGroupPrecondition(stderr io.Writer, operationName string, precondition buildBetaGroupPrecondition) error {
	message := precondition.Summary
	if operation := strings.TrimSpace(operationName); operation != "" && !strings.HasPrefix(message, operation+":") {
		message = fmt.Sprintf("%s: %s", operation, message)
	}

	fmt.Fprintf(stderr, "Error: %s\n", SanitizeTerminal(message))
	if fix := strings.TrimSpace(precondition.Fix); fix != "" {
		fmt.Fprintln(stderr, SanitizeTerminal(fix))
	}

	return WithDiagnostic(
		NewStderrReportedError(NewValidationError(errors.New(message))),
		DiagnosticStateNotReady,
		"",
	)
}

func reportBuildBetaGroupPreflightUnknown(stderr io.Writer, operationName, buildID, subject string, cause error) error {
	reason := "the response was empty"
	if cause != nil {
		reason = SanitizeTerminal(cause.Error())
	}
	precondition := buildBetaGroupPrecondition{
		Summary: fmt.Sprintf("cannot verify %s for build %s before adding beta groups: %s", subject, buildID, reason),
		Fix:     fmt.Sprintf("Re-check the current build state, then retry: asc builds info --build-id %q", buildID),
	}
	message := precondition.Summary
	if operation := strings.TrimSpace(operationName); operation != "" {
		message = fmt.Sprintf("%s: %s", operation, message)
	}
	fmt.Fprintf(stderr, "Error: %s\n", SanitizeTerminal(message))
	fmt.Fprintln(stderr, SanitizeTerminal(precondition.Fix))

	// A provider/read failure is not evidence that the build is unsuitable.
	// Keep the rendered diagnostic concise while preserving the original cause
	// so HTTP-derived exit codes and telemetry still describe that failure.
	if cause != nil {
		return WithDiagnostic(
			NewStderrReportedError(NewErrorWithCause(errors.New(message), cause)),
			DiagnosticRequestFailed,
			"",
		)
	}
	return WithDiagnostic(NewStderrReportedError(NewValidationError(errors.New(message))), DiagnosticStateNotReady, "")
}
