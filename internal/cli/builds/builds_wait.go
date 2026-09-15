package builds

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
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

// BuildsWaitCommand waits for build processing to reach a terminal state.
func BuildsWaitCommand() *ffcli.Command {
	fs := flag.NewFlagSet("wait", flag.ExitOnError)

	buildID := shared.BindResourceIDFlag(fs, "build-id", "builds", "Build ID to wait for")
	appID := shared.BindResourceIDFlag(fs, "app", "apps", "App Store Connect app ID, bundle ID, or exact app name (required when --build-id is not provided)")
	latest := fs.Bool("latest", false, "Wait for the latest matching build for --app context")
	version := fs.String("version", "", "Optional marketing version filter (CFBundleShortVersionString) for --app")
	buildNumber := fs.String("build-number", "", "Select a unique build by build number (CFBundleVersion) for --app context")
	since := fs.String("since", "", "Only consider builds uploaded on or after this RFC3339 timestamp")
	platform := fs.String("platform", "", "Platform filter for --app selectors (required with --build-number): IOS, MAC_OS, TV_OS, VISION_OS")
	timeout := fs.Duration("timeout", buildsWaitDefaultTimeout, "Maximum time to wait for build processing")
	pollInterval := fs.Duration("poll-interval", buildsWaitDefaultPollInterval, "Polling interval for build status checks")
	failOnInvalid := fs.Bool("fail-on-invalid", false, "Exit non-zero if build reaches INVALID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "wait",
		ShortUsage: "asc builds wait [flags]",
		ShortHelp:  "Wait for a build to finish processing.",
		LongHelp: `Wait for a build to finish processing.

This command polls build processing state until a terminal condition:
  - VALID   -> exits 0
  - FAILED  -> exits non-zero
  - INVALID -> exits non-zero only with --fail-on-invalid

Build selector modes (mutually exclusive):
  - --build-id BUILD_ID
  - --app APP_ID --latest
      [--version VERSION] [--platform PLATFORM] [--since RFC3339]
  - --app APP_ID --build-number NUMBER
      [--version VERSION] [--platform PLATFORM] [--since RFC3339]

Examples:
  asc builds wait --build-id "BUILD_ID"
  asc builds wait --build-id "BUILD_ID" --timeout 20m --poll-interval 15s
  asc builds wait --app "1500196580" --latest
  asc builds wait --app "1500196580" --latest --since "2026-03-02T18:00:00Z"
  asc builds wait --app "1500196580" --build-number "2" --platform IOS --version "2.4.0"
  asc builds wait --app "123456789" --build-number "42" --platform MAC_OS --fail-on-invalid`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			started := time.Now()
			buildValue := strings.TrimSpace(*buildID)
			resolvedAppID := shared.ResolveAppID(*appID)
			versionValue := strings.TrimSpace(*version)
			buildNumberValue := strings.TrimSpace(*buildNumber)
			sinceValue := strings.TrimSpace(*since)
			platformValue := strings.TrimSpace(*platform)

			if *pollInterval <= 0 {
				return shared.UsageError("--poll-interval must be greater than 0")
			}
			if *timeout <= 0 {
				return shared.UsageError("--timeout must be greater than 0")
			}

			if buildValue != "" {
				if strings.TrimSpace(*appID) != "" || *latest || versionValue != "" || buildNumberValue != "" || platformValue != "" || sinceValue != "" {
					return shared.UsageError("--build-id is mutually exclusive with app-scoped selectors (--app, --latest, --version, --build-number, --platform, --since)")
				}
			} else {
				if resolvedAppID == "" {
					return shared.UsageError("--app is required when --build-id is not provided")
				}
				if *latest && buildNumberValue != "" {
					return shared.UsageError("--latest and --build-number are mutually exclusive")
				}
				if !*latest && buildNumberValue == "" {
					return shared.UsageError("--latest or --build-number is required when using --app")
				}
				if buildNumberValue != "" && platformValue == "" {
					return shared.UsageError(buildNumberRequiresPlatformMessage)
				}
			}

			var normalizedPlatform string
			if platformValue != "" {
				var err error
				normalizedPlatform, err = shared.NormalizeAppStoreVersionPlatform(platformValue)
				if err != nil {
					return shared.UsageError(err.Error())
				}
			}

			var sinceTime *time.Time
			if sinceValue != "" {
				parsedSince, err := parseBuildUploadedTimestamp(sinceValue)
				if err != nil {
					return shared.UsageError("--since must be an RFC3339 timestamp (e.g., 2026-03-02T18:00:00Z)")
				}
				sinceTime = &parsedSince
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds wait: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeoutDuration(ctx, *timeout)
			defer cancel()

			var buildResp *asc.BuildResponse
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

				selector := appBuildWaitSelector{
					Latest:      *latest,
					AppID:       lookupAppID,
					Version:     versionValue,
					BuildNumber: buildNumberValue,
					Platform:    normalizedPlatform,
					Since:       sinceTime,
				}

				buildResp, err = waitForBuildDiscovery(requestCtx, client, selector, *pollInterval)
				if err != nil {
					if requestCtx.Err() != nil && errors.Is(err, context.DeadlineExceeded) {
						return fmt.Errorf("builds wait: timed out resolving build selector after %s", (*timeout).Round(time.Second))
					}
					return fmt.Errorf("builds wait: %w", err)
				}
			}

			waitBuildID := buildResp.Data.ID
			buildResp, err = waitForBuildProcessingState(requestCtx, client, buildResp.Data.ID, *pollInterval, *failOnInvalid)
			if err != nil {
				if requestCtx.Err() != nil && errors.Is(err, context.DeadlineExceeded) {
					return fmt.Errorf("builds wait: timed out waiting for build %s after %s", waitBuildID, (*timeout).Round(time.Second))
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
				Elapsed:         time.Since(started).Round(time.Second).String(),
			}
			if versionValue != "" {
				result.Version = versionValue
			}

			return shared.PrintOutput(result, format, *output.Pretty)
		},
	}
}

type appBuildWaitSelector struct {
	Latest      bool
	AppID       string
	Version     string
	BuildNumber string
	Platform    string
	Since       *time.Time
}

func waitForBuildDiscovery(
	ctx context.Context,
	client *asc.Client,
	selector appBuildWaitSelector,
	pollInterval time.Duration,
) (*asc.BuildResponse, error) {
	started := time.Now()
	return asc.PollUntilTolerant(ctx, pollInterval, func(ctx context.Context) (*asc.BuildResponse, bool, error) {
		buildResp, err := resolveBuildForAppWait(ctx, client, selector)
		if err != nil {
			return nil, false, err
		}
		if buildResp != nil {
			return buildResp, true, nil
		}

		fmt.Fprintf(
			os.Stderr,
			"Waiting for build discovery... (%s elapsed)\n",
			time.Since(started).Round(time.Second),
		)
		return nil, false, nil
	}, asc.PollOptions{Tolerate: asc.IsTransientWaitError})
}

func resolveBuildForAppWait(
	ctx context.Context,
	client *asc.Client,
	selector appBuildWaitSelector,
) (*asc.BuildResponse, error) {
	if selector.Latest {
		buildResp, err := shared.ResolveLatestBuild(ctx, client, shared.LatestBuildSelectionOptions{
			AppID:                 selector.AppID,
			Version:               selector.Version,
			Platform:              selector.Platform,
			ProcessingStateValues: buildsWaitProcessingStates(),
		}, true)
		if err != nil {
			return nil, err
		}
		return applyWaitSinceConstraint(buildResp, selector.Since)
	}

	buildResp, err := resolveBuildByNumberSelection(ctx, client, buildNumberSelectionOptions{
		AppID:                 selector.AppID,
		Version:               selector.Version,
		BuildNumber:           selector.BuildNumber,
		Platform:              selector.Platform,
		Since:                 selector.Since,
		ProcessingStateValues: buildsWaitProcessingStates(),
	}, true)
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

func waitForBuildProcessingState(
	ctx context.Context,
	client *asc.Client,
	buildID string,
	pollInterval time.Duration,
	failOnInvalid bool,
) (*asc.BuildResponse, error) {
	started := time.Now()

	return asc.PollUntilTolerant(ctx, pollInterval, func(ctx context.Context) (*asc.BuildResponse, bool, error) {
		buildResp, err := client.GetBuild(ctx, buildID)
		if err != nil {
			return nil, false, err
		}

		state := strings.ToUpper(strings.TrimSpace(buildResp.Data.Attributes.ProcessingState))
		if state == "" {
			state = "UNKNOWN"
		}
		fmt.Fprintf(
			os.Stderr,
			"Waiting for build %s... (%s, %s elapsed)\n",
			buildID,
			state,
			time.Since(started).Round(time.Second),
		)

		switch state {
		case asc.BuildProcessingStateValid:
			return buildResp, true, nil
		case asc.BuildProcessingStateFailed:
			return nil, false, fmt.Errorf("build processing failed with state %s", state)
		case asc.BuildProcessingStateInvalid:
			if failOnInvalid {
				return nil, false, fmt.Errorf("build processing failed with state %s", state)
			}
			return buildResp, true, nil
		}
		return nil, false, nil
	}, asc.PollOptions{Tolerate: asc.IsTransientWaitError})
}
