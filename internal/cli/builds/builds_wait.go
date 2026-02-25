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

	buildID := fs.String("build", "", "Build ID to wait for")
	appID := fs.String("app", "", "App Store Connect app ID, bundle ID, or exact app name (required with --build-number)")
	buildNumber := fs.String("build-number", "", "Build number (CFBundleVersion) to resolve and wait for (requires --app)")
	platform := fs.String("platform", "IOS", "Platform filter for --app/--build-number: IOS, MAC_OS, TV_OS, VISION_OS")
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
  - --build BUILD_ID
  - --app APP_ID --build-number NUMBER [--platform IOS]

Examples:
  asc builds wait --build "BUILD_ID"
  asc builds wait --build "BUILD_ID" --timeout 20m --poll-interval 15s
  asc builds wait --app "123456789" --build-number "42"
  asc builds wait --app "123456789" --build-number "42" --platform MAC_OS --fail-on-invalid`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			buildValue := strings.TrimSpace(*buildID)
			buildNumberValue := strings.TrimSpace(*buildNumber)
			appInputProvided := strings.TrimSpace(*appID) != ""
			buildNumberProvided := buildNumberValue != ""

			if *pollInterval <= 0 {
				return shared.UsageError("--poll-interval must be greater than 0")
			}
			if *timeout <= 0 {
				return shared.UsageError("--timeout must be greater than 0")
			}

			if buildValue != "" {
				if appInputProvided || buildNumberProvided {
					return shared.UsageError("--build is mutually exclusive with --app/--build-number")
				}
			} else {
				resolvedAppID := shared.ResolveAppID(*appID)
				if resolvedAppID == "" || buildNumberValue == "" {
					return shared.UsageError("--build is required, or provide --app and --build-number")
				}
			}

			normalizedPlatform, err := shared.NormalizeAppStoreVersionPlatform(*platform)
			if err != nil {
				return shared.UsageError(err.Error())
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds wait: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeoutDuration(ctx, *timeout)
			defer cancel()

			buildResp, err := resolveBuildForWait(requestCtx, client, buildValue, shared.ResolveAppID(*appID), buildNumberValue, normalizedPlatform)
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					return fmt.Errorf("builds wait: timed out resolving build selector after %s", (*timeout).Round(time.Second))
				}
				return fmt.Errorf("builds wait: %w", err)
			}

			waitBuildID := buildResp.Data.ID
			buildResp, err = waitForBuildProcessingState(requestCtx, client, buildResp.Data.ID, *pollInterval, *failOnInvalid)
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					return fmt.Errorf("builds wait: timed out waiting for build %s after %s", waitBuildID, (*timeout).Round(time.Second))
				}
				return fmt.Errorf("builds wait: %w", err)
			}

			return shared.PrintOutput(buildResp, *output.Output, *output.Pretty)
		},
	}
}

func resolveBuildForWait(
	ctx context.Context,
	client *asc.Client,
	buildID string,
	resolvedAppID string,
	buildNumber string,
	platform string,
) (*asc.BuildResponse, error) {
	if buildID != "" {
		return &asc.BuildResponse{
			Data: asc.Resource[asc.BuildAttributes]{
				ID: buildID,
			},
		}, nil
	}

	resolvedAppID = strings.TrimSpace(resolvedAppID)
	buildNumber = strings.TrimSpace(buildNumber)
	if resolvedAppID == "" || buildNumber == "" {
		return nil, fmt.Errorf("app ID and build number are required when build ID is not provided")
	}

	lookupAppID, err := shared.ResolveAppIDWithLookup(ctx, client, resolvedAppID)
	if err != nil {
		return nil, err
	}

	opts := []asc.BuildsOption{
		asc.WithBuildsBuildNumber(buildNumber),
		asc.WithBuildsSort("-uploadedDate"),
		asc.WithBuildsLimit(1),
		asc.WithBuildsProcessingStates([]string{
			asc.BuildProcessingStateProcessing,
			asc.BuildProcessingStateFailed,
			asc.BuildProcessingStateInvalid,
			asc.BuildProcessingStateValid,
		}),
	}
	if strings.TrimSpace(platform) != "" {
		opts = append(opts, asc.WithBuildsPreReleaseVersionPlatforms([]string{platform}))
	}

	buildsResp, err := client.GetBuilds(ctx, lookupAppID, opts...)
	if err != nil {
		return nil, err
	}
	if len(buildsResp.Data) == 0 {
		return nil, fmt.Errorf("no build found for app %q with build number %q", lookupAppID, buildNumber)
	}

	return &asc.BuildResponse{Data: buildsResp.Data[0], Links: buildsResp.Links}, nil
}

func waitForBuildProcessingState(
	ctx context.Context,
	client *asc.Client,
	buildID string,
	pollInterval time.Duration,
	failOnInvalid bool,
) (*asc.BuildResponse, error) {
	started := time.Now()

	for {
		buildResp, err := client.GetBuild(ctx, buildID)
		if err != nil {
			return nil, err
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
			return buildResp, nil
		case asc.BuildProcessingStateFailed:
			return nil, fmt.Errorf("build processing failed with state %s", state)
		case asc.BuildProcessingStateInvalid:
			if failOnInvalid {
				return nil, fmt.Errorf("build processing failed with state %s", state)
			}
			return buildResp, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}
