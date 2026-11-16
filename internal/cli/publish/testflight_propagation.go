package publish

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var postUploadBuildPropagationBackoffs = []time.Duration{
	0,
	2 * time.Second,
	4 * time.Second,
	8 * time.Second,
	16 * time.Second,
	30 * time.Second,
}

var addUploadedBuildBetaGroupsFn = addUploadedBuildBetaGroups

type postUploadBuildDistributionClient interface {
	GetBuild(ctx context.Context, buildID string, opts ...asc.BuildOption) (*asc.BuildResponse, error)
	AddBetaGroupsToBuildWithNotify(ctx context.Context, buildID string, groupIDs []string, notify bool) (asc.BuildBetaGroupsNotificationAction, error)
}

type postUploadBuildPropagationRetryPolicy struct {
	Backoffs         []time.Duration
	Wait             func(context.Context, time.Duration) error
	DiagnosticWriter io.Writer
}

type postUploadBuildProcessingFailure struct {
	build *asc.BuildResponse
	state string
}

func (e *postUploadBuildProcessingFailure) Error() string {
	return fmt.Sprintf("build processing failed: %s", e.state)
}
func (e *postUploadBuildProcessingFailure) ValidationFailure() bool { return true }
func (e *postUploadBuildProcessingFailure) Diagnostic() shared.Diagnostic {
	return shared.Diagnostic{Code: shared.DiagnosticStateNotReady}
}

func addUploadedBuildBetaGroups(
	ctx context.Context,
	d *shared.Deps,
	client postUploadBuildDistributionClient,
	buildID string,
	groups []shared.ResolvedBetaGroup,
	opts shared.AddBuildBetaGroupsOptions,
) (*shared.AddBuildBetaGroupsResult, error) {
	return addUploadedBuildBetaGroupsWithPolicy(ctx, client, buildID, groups, opts, postUploadBuildPropagationRetryPolicy{
		Backoffs: postUploadBuildPropagationBackoffs,
		Wait: func(ctx context.Context, delay time.Duration) error {
			return waitForPostUploadBuildPropagation(ctx, d.After, delay)
		},
		DiagnosticWriter: d.IO.ErrOut,
	})
}

func addUploadedBuildBetaGroupsWithPolicy(
	ctx context.Context,
	client postUploadBuildDistributionClient,
	buildID string,
	groups []shared.ResolvedBetaGroup,
	opts shared.AddBuildBetaGroupsOptions,
	policy postUploadBuildPropagationRetryPolicy,
) (*shared.AddBuildBetaGroupsResult, error) {
	for retryIndex := 0; ; retryIndex++ {
		result, err := shared.AddBuildBetaGroups(ctx, client, buildID, groups, opts)
		if err == nil {
			return result, nil
		}
		if !isPostUploadBuildPropagationError(err, buildID) {
			return nil, err
		}

		confirmedBuild, confirmErr := client.GetBuild(ctx, buildID)
		if confirmErr != nil {
			return nil, fmt.Errorf("build %q could not be confirmed after beta-group relationship returned build-not-found: %w", buildID, confirmErr)
		}
		if confirmedBuild == nil || strings.TrimSpace(confirmedBuild.Data.ID) != strings.TrimSpace(buildID) {
			return nil, fmt.Errorf("build %q could not be confirmed after beta-group relationship returned build-not-found: response did not contain the requested build", buildID)
		}
		confirmedState := strings.ToUpper(strings.TrimSpace(confirmedBuild.Data.Attributes.ProcessingState))
		if confirmedState == asc.BuildProcessingStateFailed || confirmedState == asc.BuildProcessingStateInvalid {
			return nil, &postUploadBuildProcessingFailure{build: confirmedBuild, state: confirmedState}
		}
		if retryIndex >= len(policy.Backoffs) {
			return nil, fmt.Errorf("beta-group relationship still reported uploaded build %q missing after %d attempts: %w", buildID, retryIndex+1, err)
		}
		delay := policy.Backoffs[retryIndex]
		writePostUploadBuildPropagationDiagnostic(policy.DiagnosticWriter, buildID, retryIndex+2, len(policy.Backoffs)+1, delay)
		if waitErr := policy.Wait(ctx, delay); waitErr != nil {
			return nil, fmt.Errorf("wait to retry beta-group relationship for uploaded build %q: %w", buildID, waitErr)
		}
	}
}

func writePostUploadBuildPropagationDiagnostic(writer io.Writer, buildID string, attempt, totalAttempts int, delay time.Duration) {
	if writer == nil {
		return
	}
	waitDescription := "immediately"
	if delay > 0 {
		waitDescription = "in " + delay.String()
	}
	fmt.Fprintf(
		writer,
		"Uploaded TestFlight build %s is still propagating; retrying beta-group assignment (attempt %d/%d) %s.\n",
		shared.SanitizeTerminal(buildID),
		attempt,
		totalAttempts,
		waitDescription,
	)
}

func isPostUploadBuildPropagationError(err error, buildID string) bool {
	var partialErr *asc.BuildBetaGroupsPartialError
	if errors.As(err, &partialErr) {
		return false
	}

	var apiErr *asc.APIError
	if !errors.As(err, &apiErr) || apiErr == nil || apiErr.StatusCode != http.StatusNotFound {
		return false
	}

	detail := strings.ToLower(strings.TrimSpace(apiErr.Title + " " + apiErr.Detail))
	trimmedBuildID := strings.ToLower(strings.TrimSpace(buildID))
	return trimmedBuildID != "" &&
		strings.Contains(detail, "resource of type") &&
		strings.Contains(detail, "builds") &&
		containsExactBuildIDToken(detail, trimmedBuildID)
}

func containsExactBuildIDToken(detail, buildID string) bool {
	pattern := `(?i)(^|[^a-z0-9_-])` + regexp.QuoteMeta(buildID) + `($|[^a-z0-9_-])`
	return regexp.MustCompile(pattern).MatchString(detail)
}

func waitForPostUploadBuildPropagation(ctx context.Context, after func(time.Duration) <-chan time.Time, delay time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if delay <= 0 {
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-after(delay):
		return nil
	}
}
