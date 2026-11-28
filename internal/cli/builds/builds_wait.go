package builds

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const (
	buildsWaitDefaultTimeout      = 15 * time.Minute
	buildsWaitDefaultPollInterval = 30 * time.Second
)

// newBuildsWaitCommand waits for build processing to reach a terminal state.
func newBuildsWaitCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("wait", flag.ExitOnError)

	buildID := shared.BindResourceIDFlag(fs, "build-id", "builds", "Build ID to wait for")
	appID := shared.BindResourceIDFlag(fs, "app", "apps", "App Store Connect app ID, bundle ID, or exact app name (required when --build-id is not provided)")
	latest := fs.Bool("latest", false, "Wait for the latest matching build for --app context")
	shared.BindFlagAlias(fs, "newest", "latest")
	version := fs.String("version", "", "Optional marketing version filter (CFBundleShortVersionString) for --app")
	buildNumber := fs.String("build-number", "", "Select a unique build by build number (CFBundleVersion) for --app context")
	shared.BindBuildFlagAlias(fs)
	since := fs.String("since", "", "Only consider builds uploaded on or after this RFC3339 timestamp")
	platform := fs.String("platform", "", "Platform filter for --app selectors (required with --build-number): IOS, MAC_OS, TV_OS, VISION_OS")
	timeout := fs.Duration("timeout", buildsWaitDefaultTimeout, "Maximum time to wait for build processing")
	pollInterval := fs.Duration("poll-interval", buildsWaitDefaultPollInterval, "Polling interval for build status checks")
	failOnInvalid := fs.Bool("fail-on-invalid", false, "Exit non-zero if build reaches INVALID")
	reportPending := fs.Bool("report-pending", false, "When --timeout expires first, print the pending state and resume command to stdout and exit 7 instead of failing")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "wait",
		ShortUsage: "asc builds wait [flags]",
		ShortHelp:  "Wait for a build to finish processing.",
		LongHelp: `Wait for a build to finish processing.

This command polls build processing state until a terminal condition:
  - VALID   -> exits 0
  - FAILED  -> exits non-zero
  - INVALID -> exits non-zero only with --fail-on-invalid

When --timeout expires first, the command fails (exit 1) with the last known
state and the command that resumes the wait. With --report-pending it instead
prints a pending result (status "pending", phase "discovery" or "processing",
the matching build or build upload, elapsed time, and resumeCommand) to stdout
and exits 7, so callers with short time limits can resume. A build upload that
FAILED is still reported as a failure.

Build selector modes (mutually exclusive):
  - --build-id BUILD_ID
  - --app APP_ID --latest
      [--version VERSION] [--platform PLATFORM] [--since RFC3339]
  - --app APP_ID --build-number NUMBER --platform PLATFORM
      [--version VERSION] [--since RFC3339]

Examples:
  asc builds wait --build-id "BUILD_ID"
  asc builds wait --build-id "BUILD_ID" --timeout 20m --poll-interval 15s
  asc builds wait --app "1500196580" --latest
  asc builds wait --app "1500196580" --latest --since "2026-03-02T18:00:00Z"
  asc builds wait --app "1500196580" --build-number "2" --platform IOS --version "2.4.0"
  asc builds wait --app "123456789" --build-number "42" --platform MAC_OS --fail-on-invalid
  asc builds wait --app "1500196580" --build-number "78" --platform IOS --timeout 50s --report-pending`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			started := d.Now()
			buildValue := strings.TrimSpace(*buildID)
			resolvedAppID := d.ResolveAppID(*appID)
			versionValue := strings.TrimSpace(*version)
			buildNumberValue := strings.TrimSpace(*buildNumber)
			sinceValue := strings.TrimSpace(*since)
			platformValue := strings.TrimSpace(*platform)

			if *pollInterval <= 0 {
				return d.UsageError("--poll-interval must be greater than 0")
			}
			if *timeout <= 0 {
				return d.UsageError("--timeout must be greater than 0")
			}

			if buildValue != "" {
				if strings.TrimSpace(*appID) != "" || *latest || versionValue != "" || buildNumberValue != "" || platformValue != "" || sinceValue != "" {
					return d.UsageError("--build-id is mutually exclusive with app-scoped selectors (--app, --latest, --version, --build-number, --platform, --since)")
				}
			} else {
				if resolvedAppID == "" {
					return d.UsageError("--app is required when --build-id is not provided")
				}
				if *latest && buildNumberValue != "" {
					return d.UsageError("--latest and --build-number are mutually exclusive")
				}
				if !*latest && buildNumberValue == "" {
					return d.UsageError("--latest or --build-number is required when using --app")
				}
				if buildNumberValue != "" && platformValue == "" {
					return d.UsageError(buildNumberRequiresPlatformMessage)
				}
			}

			var normalizedPlatform string
			if platformValue != "" {
				var err error
				normalizedPlatform, err = shared.NormalizeAppStoreVersionPlatform(platformValue)
				if err != nil {
					return d.UsageError(err.Error())
				}
			}

			var sinceTime *time.Time
			if sinceValue != "" {
				parsedSince, err := parseBuildUploadedTimestamp(sinceValue)
				if err != nil {
					return d.UsageError("--since must be an RFC3339 timestamp (e.g., 2026-03-02T18:00:00Z)")
				}
				sinceTime = &parsedSince
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("builds wait: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeoutDuration(ctx, *timeout)
			defer cancel()

			var buildResp *asc.BuildResponse
			failureContext := shared.BuildProcessingFailureContext{
				ShortVersion: versionValue,
				Platform:     normalizedPlatform,
			}
			timedOut := buildsWaitTimeout{
				d:             d,
				fs:            fs,
				started:       started,
				timeout:       *timeout,
				reportPending: *reportPending,
				output:        output,
				selector: appBuildWaitSelector{
					Latest:      *latest,
					Version:     versionValue,
					BuildNumber: buildNumberValue,
					Platform:    normalizedPlatform,
					Since:       sinceTime,
				},
				sinceValue: sinceValue,
			}
			if buildValue != "" {
				buildResp = &asc.BuildResponse{
					Data: asc.Resource[asc.BuildAttributes]{
						ID: buildValue,
					},
				}
			} else {
				lookupAppID, err := shared.ResolveAppIDWithLookup(requestCtx, client, resolvedAppID)
				if err != nil {
					return fmt.Errorf("builds wait: %w", err)
				}
				failureContext.AppID = lookupAppID
				timedOut.selector.AppID = lookupAppID

				buildResp, err = waitForBuildDiscovery(requestCtx, d, client, timedOut.selector, *pollInterval, &timedOut.observed)
				if err != nil {
					if requestCtx.Err() != nil && errors.Is(err, context.DeadlineExceeded) {
						return timedOut.discoveryTimeout()
					}
					return fmt.Errorf("builds wait: %w", err)
				}
			}

			waitBuildID := buildResp.Data.ID
			buildResp, err = waitForBuildProcessingState(requestCtx, d, client, buildResp.Data.ID, *pollInterval, *failOnInvalid, failureContext, &timedOut.observed)
			if err != nil {
				if requestCtx.Err() != nil && errors.Is(err, context.DeadlineExceeded) {
					return timedOut.processingTimeout(waitBuildID)
				}
				return fmt.Errorf("builds wait: %w", err)
			}

			format, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty)
			if err != nil {
				return err
			}

			processingState := strings.ToUpper(strings.TrimSpace(buildResp.Data.Attributes.ProcessingState))
			if processingState == "" {
				processingState = "UNKNOWN"
			}
			result := &asc.BuildWaitResult{
				Data:            buildResp.Data,
				Links:           buildResp.Links,
				BuildID:         strings.TrimSpace(buildResp.Data.ID),
				BuildNumber:     strings.TrimSpace(buildResp.Data.Attributes.Version),
				ProcessingState: processingState,
				Elapsed:         d.Now().Sub(started).Round(time.Second).String(),
			}
			if versionValue != "" {
				result.Version = versionValue
			}

			return d.PrintOutput(result, format, *output.Pretty)
		},
	}
}

type appBuildWaitSelector struct {
	Latest            bool
	AppID             string
	Version           string
	BuildNumber       string
	Platform          string
	Since             *time.Time
	BuildAudienceType asc.BuildAudienceType
}

// buildWaitObservation is the latest state a wait has seen, kept so a wait
// that times out can say where the build stands.
type buildWaitObservation struct {
	// upload is the newest build upload matching the selector, read while no
	// build is visible yet. It is nil when none matched or none was read.
	upload *asc.BuildUploadResponse
	// build is the last build state read while waiting for processing.
	build *asc.BuildResponse
}

// waitForBuildDiscovery polls until a build matches selector. When observed is
// non-nil, every poll that finds no build also records the newest matching
// build upload, best effort, so a timeout can report an upload that App Store
// Connect accepted but has not exposed as a build yet.
func waitForBuildDiscovery(
	ctx context.Context,
	d *shared.Deps,
	client *asc.Client,
	selector appBuildWaitSelector,
	pollInterval time.Duration,
	observed *buildWaitObservation,
) (*asc.BuildResponse, error) {
	started := d.Now()
	return asc.PollUntilTolerant(ctx, pollInterval, func(ctx context.Context) (*asc.BuildResponse, bool, error) {
		buildResp, err := resolveBuildForAppWait(ctx, d, client, selector, true)
		if err != nil {
			return nil, false, err
		}
		if buildResp != nil {
			return buildResp, true, nil
		}

		if observed != nil {
			upload, uploadErr := shared.LatestBuildUploadForWait(ctx, client, shared.BuildUploadWaitSelector{
				AppID:       selector.AppID,
				Version:     selector.Version,
				BuildNumber: selector.BuildNumber,
				Platform:    selector.Platform,
				Since:       selector.Since,
			})
			// The upload only describes the wait; a failed read keeps the
			// previous observation instead of failing discovery.
			if uploadErr == nil {
				observed.upload = upload
			}
		}

		fmt.Fprintf(
			d.IO.ErrOut,
			"Waiting for build discovery... (%s elapsed)\n",
			d.Now().Sub(started).Round(time.Second),
		)
		return nil, false, nil
	}, asc.PollOptions{Tolerate: asc.IsTransientWaitError, Stderr: d.IO.ErrOut})
}

func resolveBuildForAppWait(
	ctx context.Context,
	d *shared.Deps,
	client *asc.Client,
	selector appBuildWaitSelector,
	allowEmpty bool,
) (*asc.BuildResponse, error) {
	if selector.Latest {
		buildResp, err := d.ResolveLatestBuild(ctx, client, shared.LatestBuildSelectionOptions{
			AppID:                 selector.AppID,
			Version:               selector.Version,
			Platform:              selector.Platform,
			ProcessingStateValues: buildsWaitProcessingStates(),
			BuildAudienceType:     selector.BuildAudienceType,
		}, allowEmpty)
		if err != nil {
			return nil, err
		}
		return applyWaitSinceConstraint(buildResp, selector.Since)
	}

	buildResp, err := resolveBuildByNumberSelection(ctx, d, client, buildNumberSelectionOptions{
		AppID:                 selector.AppID,
		Version:               selector.Version,
		BuildNumber:           selector.BuildNumber,
		Platform:              selector.Platform,
		Since:                 selector.Since,
		ProcessingStateValues: buildsWaitProcessingStates(),
	}, allowEmpty)
	if err != nil {
		return nil, err
	}

	return buildResp, nil
}

func applyWaitSinceConstraint(buildResp *asc.BuildResponse, since *time.Time) (*asc.BuildResponse, error) {
	if buildResp == nil || since == nil {
		return buildResp, nil
	}

	uploadedAt, err := parseBuildUploadedTimestamp(buildResp.Data.Attributes.UploadedDate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse uploadedDate for build %s: %w", buildResp.Data.ID, err)
	}
	if uploadedAt.Before(since.UTC()) {
		return nil, nil
	}

	return buildResp, nil
}

func parseBuildUploadedTimestamp(raw string) (time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}, fmt.Errorf("timestamp is required")
	}
	timestamp, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, err
	}
	return timestamp.UTC(), nil
}

func buildsWaitProcessingStates() []string {
	return []string{
		asc.BuildProcessingStateProcessing,
		asc.BuildProcessingStateFailed,
		asc.BuildProcessingStateInvalid,
		asc.BuildProcessingStateValid,
	}
}

// waitForBuildProcessingState polls until the build reaches a terminal
// processing state. When observed is non-nil, it records every build state it
// reads so a timeout can report the last known processing state.
func waitForBuildProcessingState(
	ctx context.Context,
	d *shared.Deps,
	client *asc.Client,
	buildID string,
	pollInterval time.Duration,
	failOnInvalid bool,
	failure shared.BuildProcessingFailureContext,
	observed *buildWaitObservation,
) (*asc.BuildResponse, error) {
	started := d.Now()

	buildResp, err := asc.PollUntilTolerant(ctx, pollInterval, func(ctx context.Context) (*asc.BuildResponse, bool, error) {
		buildResp, err := client.GetBuild(ctx, buildID)
		if err != nil {
			return nil, false, err
		}
		if observed != nil {
			observed.build = buildResp
		}

		state := strings.ToUpper(strings.TrimSpace(buildResp.Data.Attributes.ProcessingState))
		if state == "" {
			state = "UNKNOWN"
		}
		fmt.Fprintf(
			d.IO.ErrOut,
			"Waiting for build %s... (%s, %s elapsed)\n",
			buildID,
			state,
			d.Now().Sub(started).Round(time.Second),
		)

		switch state {
		case asc.BuildProcessingStateValid:
			return buildResp, true, nil
		case asc.BuildProcessingStateFailed:
			return nil, false, &terminalBuildProcessingState{build: buildResp, state: state}
		case asc.BuildProcessingStateInvalid:
			if failOnInvalid {
				return nil, false, &terminalBuildProcessingState{build: buildResp, state: state}
			}
			return buildResp, true, nil
		}
		return nil, false, nil
	}, asc.PollOptions{Tolerate: asc.IsTransientWaitError, Stderr: d.IO.ErrOut})

	// Processing details are fetched after polling stops: the poller reports an
	// expired context in place of any callback error, so a slow details lookup
	// inside the callback could turn a FAILED build into a timeout.
	var terminal *terminalBuildProcessingState
	if errors.As(err, &terminal) {
		return nil, buildProcessingFailureError(ctx, client, buildID, terminal.build, terminal.state, failure)
	}
	return buildResp, err
}

// terminalBuildProcessingState stops polling at a failing processing state.
type terminalBuildProcessingState struct {
	build *asc.BuildResponse
	state string
}

func (e *terminalBuildProcessingState) Error() string {
	return fmt.Sprintf("build processing failed with state %s", e.state)
}

func buildProcessingFailureError(
	ctx context.Context,
	client *asc.Client,
	buildID string,
	buildResp *asc.BuildResponse,
	state string,
	failure shared.BuildProcessingFailureContext,
) error {
	baseErr := fmt.Errorf("build processing failed with state %s", state)
	failure.BuildID = buildID
	if buildResp != nil {
		failure.BundleVersion = buildResp.Data.Attributes.Version
	}
	return shared.WithDiagnostic(
		shared.NewValidationError(shared.EnrichBuildProcessingFailure(ctx, client, failure, baseErr)),
		shared.DiagnosticStateNotReady,
		"",
	)
}
