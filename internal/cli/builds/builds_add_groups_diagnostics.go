package builds

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const buildBetaGroupDiagnosticBudget = 2 * time.Second

func assignmentIncludesExternalGroup(groups []resolvedBuildBetaGroup) bool {
	for _, group := range groups {
		if !group.IsInternalGroup {
			return true
		}
	}
	return false
}

func reportBuildBetaGroupAssignmentFailure(
	ctx context.Context,
	client *asc.Client,
	buildID string,
	groups []resolvedBuildBetaGroup,
	originalErr error,
) error {
	if originalErr == nil {
		return nil
	}

	var apiErr *asc.APIError
	if !errors.As(originalErr, &apiErr) || apiErr == nil || apiErr.StatusCode != http.StatusUnprocessableEntity {
		return fmt.Errorf("builds add-groups: failed to add groups: %w", originalErr)
	}

	buildID = strings.TrimSpace(buildID)
	message := fmt.Sprintf(
		"builds add-groups: App Store Connect rejected adding beta groups to build %s (HTTP 422): %s",
		buildID,
		appleUnprocessableDetail(apiErr),
	)
	lines := readBuildBetaGroupStateDiagnostics(ctx, client, buildID, assignmentIncludesExternalGroup(groups))
	for _, group := range groups {
		if group.IsInternalGroup && group.HasAccessToAllBuilds {
			lines = append(lines, fmt.Sprintf("Internal group %q (%s) has access to all builds, so App Store Connect adds builds to it automatically; leave it out of --group or pass --skip-internal", group.NameForDisplay(), group.ID))
		}
	}

	fmt.Fprintf(os.Stderr, "Error: %s\n", shared.SanitizeTerminal(message))
	for _, line := range lines {
		fmt.Fprintln(os.Stderr, shared.SanitizeTerminal(line))
	}
	if len(lines) == 0 {
		fmt.Fprintf(os.Stderr, "Re-check the current build state before retrying: asc builds info --build-id %q\n", buildID)
	}

	return shared.WithDiagnostic(
		shared.NewStderrReportedError(shared.NewValidationError(
			shared.NewErrorWithCause(errors.New(message), originalErr),
		)),
		shared.DiagnosticStateNotReady,
		"",
	)
}

func reportBuildBetaGroupAssignmentDryRun(
	ctx context.Context,
	client *asc.Client,
	buildID string,
	plan shared.BuildBetaGroupAssignmentPlan,
	output shared.OutputFlags,
) error {
	for _, group := range plan.SkippedInternalGroups {
		fmt.Fprintf(
			os.Stderr,
			"Skipped internal group %q (%s) because --skip-internal was set\n",
			group.NameForDisplay(),
			shared.SanitizeTerminal(group.ID),
		)
	}

	groupIDs := plan.GroupIDsToAdd()
	if len(groupIDs) == 0 {
		fmt.Fprintf(os.Stderr, "No groups to add for build %s after applying filters\n", buildID)
	} else {
		lines := readBuildBetaGroupStateDiagnostics(ctx, client, buildID, plan.IncludesExternalGroup())
		for _, line := range lines {
			fmt.Fprintln(os.Stderr, shared.SanitizeTerminal(line))
		}
		if len(lines) == 0 {
			fmt.Fprintf(os.Stderr, "Dry-run readiness is unknown; re-check the current build state with asc builds info --build-id %q\n", buildID)
		}
		fmt.Fprintf(os.Stderr, "Dry run: no beta groups were added to build %s; readiness is advisory and does not predict a future assignment\n", buildID)
	}

	action := "would-add"
	if len(groupIDs) == 0 {
		action = "no-op"
	}
	return shared.PrintOutput(&asc.BuildBetaGroupsUpdateResult{
		BuildID:  buildID,
		GroupIDs: groupIDs,
		Action:   action,
		DryRun:   true,
	}, *output.Output, *output.Pretty)
}

func readBuildBetaGroupStateDiagnostics(
	ctx context.Context,
	client *asc.Client,
	buildID string,
	includesExternal bool,
) []string {
	diagnosticCtx, cancel := context.WithTimeout(ctx, buildBetaGroupDiagnosticBudget)
	defer cancel()

	lines := make([]string, 0, 4)
	if build, err := client.GetBuild(diagnosticCtx, buildID); err == nil && build != nil {
		if summary := formatCurrentBuildState(build.Data.Attributes); summary != "" {
			lines = append(lines, summary)
		}
		lines = append(lines, buildStateGuidance(buildID, build.Data.Attributes)...)
	}

	if includesExternal {
		if detail, err := client.GetBuildBuildBetaDetail(diagnosticCtx, buildID); err == nil && detail != nil {
			state := strings.ToUpper(strings.TrimSpace(detail.Data.Attributes.ExternalBuildState))
			if state != "" {
				lines = append(lines, "Current external beta state: externalBuildState="+state)
				lines = append(lines, externalBetaStateGuidance(buildID, state)...)
			}
		}
	}

	return lines
}

func formatCurrentBuildState(attributes asc.BuildAttributes) string {
	parts := make([]string, 0, 4)
	if state := strings.ToUpper(strings.TrimSpace(attributes.ProcessingState)); state != "" {
		parts = append(parts, "processingState="+state)
	}
	if expired, known := attributes.ExpiredValue(); known {
		parts = append(parts, fmt.Sprintf("expired=%t", expired))
	}
	if attributes.UsesNonExemptEncryption == nil {
		parts = append(parts, "usesNonExemptEncryption=not reported")
	} else {
		parts = append(parts, fmt.Sprintf("usesNonExemptEncryption=%t", *attributes.UsesNonExemptEncryption))
	}
	if audience := strings.TrimSpace(string(attributes.BuildAudienceType)); audience != "" {
		parts = append(parts, "buildAudienceType="+audience)
	}
	if len(parts) == 0 {
		return ""
	}
	return "Current build state: " + strings.Join(parts, ", ")
}

func buildStateGuidance(buildID string, attributes asc.BuildAttributes) []string {
	lines := make([]string, 0, 2)
	switch strings.ToUpper(strings.TrimSpace(attributes.ProcessingState)) {
	case "PROCESSING":
		lines = append(lines, fmt.Sprintf("If processing is blocking assignment, wait for it to finish, then retry: asc builds wait --build-id %q", buildID))
	case "FAILED", "INVALID":
		lines = append(
			lines,
			"For a failed or invalid build, upload a replacement iOS build: asc builds upload --app \"APP_ID\" --ipa \"PATH_TO_IPA\"",
			"Or upload a replacement macOS build: asc builds upload --app \"APP_ID\" --pkg \"PATH_TO_PKG\" --version \"VERSION\" --build-number \"BUILD_NUMBER\"",
		)
	}
	if expired, known := attributes.ExpiredValue(); known && expired {
		lines = append(lines, "Pick a build that has not expired: asc builds list --app \"APP_ID\" --limit 5")
	}
	return lines
}

func externalBetaStateGuidance(buildID, state string) []string {
	switch state {
	case "MISSING_EXPORT_COMPLIANCE":
		return []string{
			fmt.Sprintf("If the app does not use non-exempt encryption, declare that: asc builds update --build-id %q --uses-non-exempt-encryption=false", buildID),
			fmt.Sprintf("Otherwise, assign the applicable encryption declaration: asc encryption declarations assign-builds --id \"DECLARATION_ID\" --build-id %q", buildID),
		}
	case "PROCESSING":
		return []string{fmt.Sprintf("Wait for processing to finish, then retry: asc builds wait --build-id %q", buildID)}
	case "PROCESSING_EXCEPTION":
		return []string{
			"Upload a replacement iOS build: asc builds upload --app \"APP_ID\" --ipa \"PATH_TO_IPA\"",
			"Or upload a replacement macOS build: asc builds upload --app \"APP_ID\" --pkg \"PATH_TO_PKG\" --version \"VERSION\" --build-number \"BUILD_NUMBER\"",
		}
	case "IN_EXPORT_COMPLIANCE_REVIEW":
		return []string{fmt.Sprintf("Re-check the state and retry after review finishes: asc builds info --build-id %q", buildID)}
	case "EXPIRED":
		return []string{"Pick a build that has not expired: asc builds list --app \"APP_ID\" --limit 5"}
	case "BETA_REJECTED":
		return []string{fmt.Sprintf("Address the beta review rejection, then resubmit: asc testflight review submit --build-id %q --confirm", buildID)}
	default:
		return nil
	}
}

func appleUnprocessableDetail(apiErr *asc.APIError) string {
	rendered := strings.TrimSpace(apiErr.Error())
	if rendered == "" {
		return "App Store Connect returned no error detail"
	}

	code := strings.TrimSpace(apiErr.Code)
	if code == "" {
		return rendered
	}

	sections := strings.SplitN(rendered, "\n\n", 2)
	if sections[0] != code {
		sections[0] = fmt.Sprintf("%s (%s)", sections[0], code)
	}
	return strings.Join(sections, "\n\n")
}
