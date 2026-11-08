package builds

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// VersionBuildSelector chooses the build to attach to an App Store version:
// an explicit --build-id, or --build-number or --latest within the version's
// train, optionally waiting for processing with --wait.
type VersionBuildSelector struct {
	fs           *flag.FlagSet
	buildID      *string
	buildNumber  *string
	latest       *bool
	wait         *bool
	timeout      *time.Duration
	pollInterval *time.Duration
}

// VersionBuildScope is the train a selector searches: builds of one app with
// the version's marketing version string and platform.
type VersionBuildScope struct {
	AppID    string
	Version  string
	Platform string
}

func BindVersionBuildSelector(fs *flag.FlagSet) *VersionBuildSelector {
	return &VersionBuildSelector{
		fs:           fs,
		buildID:      shared.BindResourceIDFlag(fs, "build-id", "builds", "Build ID to attach"),
		buildNumber:  fs.String("build-number", "", "Build number (CFBundleVersion) to attach from the version's app, version string, and platform"),
		latest:       fs.Bool("latest", false, "Attach the most recently uploaded App Store eligible build for the version's app, version string, and platform"),
		wait:         fs.Bool("wait", false, "Wait for the selected build to finish processing (VALID) before attaching"),
		timeout:      fs.Duration("timeout", buildsWaitDefaultTimeout, "Maximum time to wait with --wait"),
		pollInterval: fs.Duration("poll-interval", buildsWaitDefaultPollInterval, "Polling interval for --wait"),
	}
}

func (s *VersionBuildSelector) Validate() error {
	selected := 0
	for _, set := range []bool{s.buildIDValue() != "", s.buildNumberValue() != "", *s.latest} {
		if set {
			selected++
		}
	}
	if selected == 0 {
		fmt.Fprintln(os.Stderr, "Error: --build-id, --build-number, or --latest is required")
		return shared.MissingRequiredUsageError("--build-id")
	}
	if selected > 1 {
		return shared.WithDiagnostic(
			shared.UsageError("--build-id, --build-number, and --latest are mutually exclusive"),
			shared.DiagnosticConflictingInput,
			"",
		)
	}

	if !*s.wait {
		waitFlagSet := false
		s.fs.Visit(func(f *flag.Flag) {
			if f.Name == "timeout" || f.Name == "poll-interval" {
				waitFlagSet = true
			}
		})
		if waitFlagSet {
			return shared.UsageError("--timeout and --poll-interval require --wait")
		}
		return nil
	}
	if *s.timeout <= 0 {
		return shared.UsageError("--timeout must be greater than 0")
	}
	if *s.pollInterval <= 0 {
		return shared.UsageError("--poll-interval must be greater than 0")
	}
	return nil
}

// NeedsScope reports whether the build is chosen from the version's train.
func (s *VersionBuildSelector) NeedsScope() bool {
	return s.buildIDValue() == ""
}

// Resolve returns the selected build ID. --build-id without --wait makes no
// request.
func (s *VersionBuildSelector) Resolve(ctx context.Context, client *asc.Client, scope VersionBuildScope) (string, error) {
	buildID := s.buildIDValue()
	if buildID != "" && !*s.wait {
		return buildID, nil
	}

	selector := appBuildWaitSelector{
		Latest:            *s.latest,
		AppID:             scope.AppID,
		Version:           scope.Version,
		BuildNumber:       s.buildNumberValue(),
		Platform:          scope.Platform,
		BuildAudienceType: asc.BuildAudienceTypeAppStoreEligible,
	}

	if !*s.wait {
		lookupCtx, cancel := shared.ContextWithTimeout(ctx)
		defer cancel()
		buildResp, err := resolveBuildForAppWait(lookupCtx, client, selector, false)
		if err != nil {
			return "", err
		}
		reportSelectedVersionBuild(buildResp, selector)
		if err := requireValidBuild(buildResp); err != nil {
			return "", err
		}
		return strings.TrimSpace(buildResp.Data.ID), nil
	}

	waitCtx, cancel := shared.ContextWithTimeoutDuration(ctx, *s.timeout)
	defer cancel()

	if buildID == "" {
		buildResp, err := waitForBuildDiscovery(waitCtx, client, selector, *s.pollInterval, nil)
		if err != nil {
			return "", s.waitError(waitCtx, err, "a matching build to appear")
		}
		if err := shared.RequireAppStoreBuildAudience(buildResp); err != nil {
			return "", err
		}
		reportSelectedVersionBuild(buildResp, selector)
		buildID = strings.TrimSpace(buildResp.Data.ID)
	}

	failure := shared.BuildProcessingFailureContext{
		AppID:        selector.AppID,
		ShortVersion: selector.Version,
		Platform:     selector.Platform,
	}
	buildResp, err := waitForBuildProcessingState(waitCtx, client, buildID, *s.pollInterval, true, failure, nil)
	if err != nil {
		return "", s.waitError(waitCtx, err, fmt.Sprintf("build %s to finish processing", buildID))
	}
	if err := shared.RequireAppStoreBuildAudience(buildResp); err != nil {
		return "", err
	}
	return buildID, nil
}

func (s *VersionBuildSelector) waitError(waitCtx context.Context, err error, target string) error {
	if waitCtx.Err() != nil && errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("timed out after %s waiting for %s; rerun with a longer --timeout: %w", *s.timeout, target, err)
	}
	return err
}

func (s *VersionBuildSelector) buildIDValue() string {
	return strings.TrimSpace(*s.buildID)
}

func (s *VersionBuildSelector) buildNumberValue() string {
	return strings.TrimSpace(*s.buildNumber)
}

func reportSelectedVersionBuild(buildResp *asc.BuildResponse, selector appBuildWaitSelector) {
	fmt.Fprintf(
		os.Stderr,
		"Selected build %s (%s) for version %s on %s\n",
		strings.TrimSpace(buildResp.Data.Attributes.Version),
		strings.TrimSpace(buildResp.Data.ID),
		selector.Version,
		selector.Platform,
	)
}

func requireValidBuild(buildResp *asc.BuildResponse) error {
	if err := shared.RequireAppStoreBuildAudience(buildResp); err != nil {
		return err
	}
	state := strings.ToUpper(strings.TrimSpace(buildResp.Data.Attributes.ProcessingState))
	buildNumber := strings.TrimSpace(buildResp.Data.Attributes.Version)
	buildID := strings.TrimSpace(buildResp.Data.ID)
	switch state {
	case asc.BuildProcessingStateValid:
		return nil
	case asc.BuildProcessingStateProcessing:
		return fmt.Errorf("build %s (%s) is still processing; add --wait to attach it once it is VALID", buildNumber, buildID)
	default:
		return fmt.Errorf("build %s (%s) has processing state %s; only VALID builds can be attached", buildNumber, buildID, state)
	}
}
