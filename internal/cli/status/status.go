package status

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type includeSet struct {
	app           bool
	builds        bool
	testflight    bool
	appstore      bool
	submission    bool
	review        bool
	phasedRelease bool
	links         bool
}

type dashboardResponse struct {
	App           *statusApp            `json:"app,omitempty"`
	Summary       statusSummary         `json:"summary"`
	Builds        *buildsSection        `json:"builds,omitempty"`
	TestFlight    *testFlightSection    `json:"testflight,omitempty"`
	AppStore      *appStoreSection      `json:"appstore,omitempty"`
	Submission    *submissionSection    `json:"submission,omitempty"`
	Review        *reviewSection        `json:"review,omitempty"`
	PhasedRelease *phasedReleaseSection `json:"phasedRelease,omitempty"`
	Links         *linksSection         `json:"links,omitempty"`
}

type statusApp struct {
	ID       string `json:"id"`
	BundleID string `json:"bundleId"`
	Name     string `json:"name"`
}

type statusSummary struct {
	Health       string                  `json:"health"`
	NextAction   string                  `json:"nextAction"`
	NextCommands []asc.StatusNextCommand `json:"nextCommands"`
	Blockers     []string                `json:"blockers"`
	Platform     string                  `json:"platform,omitempty"`
}

type buildsSection struct {
	Latest *latestBuild `json:"latest,omitempty"`
}

type latestBuild struct {
	ID              string `json:"id"`
	Version         string `json:"version,omitempty"`
	BuildNumber     string `json:"buildNumber"`
	ProcessingState string `json:"processingState,omitempty"`
	// Expired reports Apple's build expiry flag, which stays independent of
	// processingState: an expired build is still VALID but no longer installable.
	Expired            *bool  `json:"expired,omitempty"`
	UploadedDate       string `json:"uploadedDate,omitempty"`
	Platform           string `json:"platform,omitempty"`
	appStoreRestricted bool
}

type testFlightSection struct {
	LatestDistributedBuildID string `json:"latestDistributedBuildId,omitempty"`
	BetaReviewState          string `json:"betaReviewState,omitempty"`
	// InternalBuildState is the TestFlight internal state of the latest uploaded
	// build, which is the build reported in builds.latest. ExternalBuildState
	// describes LatestDistributedBuildID instead, so the two can disagree while a
	// newer build is still processing for internal testers.
	InternalBuildState   string                      `json:"internalBuildState,omitempty"`
	ExternalBuildState   string                      `json:"externalBuildState,omitempty"`
	SubmittedDate        string                      `json:"submittedDate,omitempty"`
	BetaReviewSubmission *betaReviewSubmissionStatus `json:"betaReviewSubmission,omitempty"`
	latestBuild          *betaReviewBuildStatus
}

// betaBuildStates pairs the TestFlight-side states App Store Connect reports for
// a single build.
type betaBuildStates struct {
	internal string
	external string
}

type betaReviewSubmissionStatus struct {
	ID                    string                 `json:"id"`
	State                 string                 `json:"state,omitempty"`
	SubmittedDate         string                 `json:"submittedDate,omitempty"`
	RelationToLatestBuild string                 `json:"relationToLatestBuild"`
	Build                 *betaReviewBuildStatus `json:"build,omitempty"`
}

type betaReviewBuildStatus struct {
	ID          string `json:"id"`
	Version     string `json:"version,omitempty"`
	BuildNumber string `json:"buildNumber,omitempty"`
	Platform    string `json:"platform,omitempty"`
}

type appStoreSection struct {
	VersionID   string `json:"versionId,omitempty"`
	Version     string `json:"version,omitempty"`
	State       string `json:"state,omitempty"`
	Platform    string `json:"platform,omitempty"`
	CreatedDate string `json:"createdDate,omitempty"`
}

type submissionSection struct {
	InFlight       bool     `json:"inFlight"`
	BlockingIssues []string `json:"blockingIssues"`
}

type reviewSection struct {
	LatestSubmissionID string `json:"latestSubmissionId,omitempty"`
	State              string `json:"state,omitempty"`
	SubmittedDate      string `json:"submittedDate,omitempty"`
	Platform           string `json:"platform,omitempty"`
}

type phasedReleaseSection struct {
	Configured         bool   `json:"configured"`
	ID                 string `json:"id,omitempty"`
	State              string `json:"state,omitempty"`
	StartDate          string `json:"startDate,omitempty"`
	CurrentDayNumber   int    `json:"currentDayNumber,omitempty"`
	TotalPauseDuration int    `json:"totalPauseDuration,omitempty"`
}

type linksSection struct {
	AppStoreConnect string `json:"appStoreConnect"`
	TestFlight      string `json:"testFlight"`
	Review          string `json:"review"`
}

type relationshipReference struct {
	Data asc.ResourceData `json:"data"`
}

type sectionTask struct {
	name string
	run  func() error
}

var allowedIncludes = []string{
	"app",
	"builds",
	"testflight",
	"appstore",
	"submission",
	"review",
	"phased-release",
	"links",
}

const maxBetaReviewBuildPrefetches = 5

// StatusCommand returns the root status dashboard command.
func StatusCommand() *ffcli.Command {
	fs := flag.NewFlagSet("status", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID, bundle ID, or exact app name (required, or ASC_APP_ID env)")
	platform := fs.String("platform", "", "Filter release status by platform: IOS, MAC_OS, TV_OS, VISION_OS")
	include := fs.String("include", "", "Comma-separated sections: app,builds,testflight,appstore,submission,review,phased-release,links")
	watch := fs.Bool("watch", false, "Poll and emit snapshots when status changes")
	pollInterval := fs.Duration("poll-interval", 30*time.Second, "Polling interval for --watch")
	maxPolls := fs.Int("max-polls", 0, "Maximum polls for --watch (0 = unlimited)")
	until := fs.String("until", "", "Wait until a condition is reached, then exit (implies --watch): "+strings.Join(untilConditions, ", "))
	timeout := fs.Duration("timeout", 0, "Maximum time to wait for --until (0 = no limit)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "status",
		ShortUsage: "asc status [flags]",
		ShortHelp:  "Show a release pipeline dashboard for an app.",
		LongHelp: `Show a release pipeline dashboard for an app.

This command aggregates release signals into one deterministic payload for CI,
agents, and human review.

Examples:
  asc status --app "123456789"
  asc status --app "com.example.app"
  asc status --app "My App"
  asc status --app "123456789" --platform MAC_OS
  asc status --app "123456789" --include builds,testflight,submission
  asc status --app "123456789" --watch --poll-interval 15s
  asc status --app "123456789" --until review-done --timeout 2h
  asc status --app "123456789" --output table

--until polls like --watch, then prints a final result after the snapshots
({"until","reached","outcome","state","polls"}) and exits:
  review-done       latest App Store version is approved (ACCEPTED, PENDING_*_RELEASE,
                    PROCESSING_FOR_DISTRIBUTION, READY_FOR_DISTRIBUTION) -> exit 0,
                    or REJECTED, METADATA_REJECTED, INVALID_BINARY, DEVELOPER_REJECTED -> exit 1
  ready-for-sale    latest App Store version is READY_FOR_DISTRIBUTION -> exit 0,
                    or rejected as above -> exit 1
  processed         latest build processingState is VALID -> exit 0, FAILED or INVALID -> exit 1
  testflight-ready  latest build internal TestFlight state is READY_FOR_BETA_TESTING or
                    IN_BETA_TESTING -> exit 0, PROCESSING_EXCEPTION,
                    MISSING_EXPORT_COMPLIANCE, or EXPIRED -> exit 1
  change            first snapshot that differs from the first one -> exit 0
Conditions are checked on every snapshot, including the first. When --max-polls,
--timeout, or an interrupt ends the wait first, the result reports "pending"
and the command exits 7.

summary.nextCommands lists runnable asc commands for common states, with a
reason and whether each command mutates App Store Connect. Read-only mode
omits mutating commands.`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}

			includes, err := parseInclude(*include)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if *pollInterval <= 0 {
				return shared.UsageError("--poll-interval must be greater than 0")
			}
			if *maxPolls < 0 {
				return shared.UsageError("--max-polls must be greater than or equal to 0")
			}
			if *until != "" {
				if err := validateUntil(*until, includes); err != nil {
					return shared.UsageError(err.Error())
				}
			}
			if *timeout < 0 {
				return shared.UsageError("--timeout must be greater than or equal to 0")
			}
			if *timeout > 0 && *until == "" {
				return shared.UsageError("--timeout requires --until")
			}
			watching := *watch || *until != ""
			if *maxPolls > 0 && !watching {
				return shared.UsageError("--max-polls requires --watch")
			}
			normalizedPlatform := ""
			if strings.TrimSpace(*platform) != "" {
				normalizedPlatform, err = shared.NormalizeAppStoreVersionPlatform(*platform)
				if err != nil {
					return shared.UsageError(err.Error())
				}
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("status: %w", err)
			}

			lookupCtx, cancel := shared.ContextWithTimeout(ctx)
			resolvedAppID, err = shared.ResolveAppIDWithLookup(lookupCtx, client, resolvedAppID)
			cancel()
			if err != nil {
				return fmt.Errorf("status: %w", err)
			}

			if watching {
				if *timeout > 0 {
					var cancelWait context.CancelFunc
					ctx, cancelWait = context.WithTimeout(ctx, *timeout)
					defer cancelWait()
				}
				return watchDashboard(ctx, client, resolvedAppID, normalizedPlatform, includes, *output.Output, *output.Pretty, *pollInterval, *maxPolls, *until)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := collectDashboard(requestCtx, client, resolvedAppID, normalizedPlatform, includes, false)
			if err != nil {
				return fmt.Errorf("status: %w", err)
			}

			return shared.PrintOutputWithRenderers(
				resp,
				*output.Output,
				*output.Pretty,
				func() error { renderTable(resp); return nil },
				func() error { renderMarkdown(resp); return nil },
			)
		},
	}
}

func watchDashboard(ctx context.Context, client *asc.Client, appID string, platform string, includes includeSet, output string, pretty bool, pollInterval time.Duration, maxPolls int, until string) error {
	seen := ""
	polls := 0
	var check untilCheck
	finish := func() error {
		if until == "" {
			return nil
		}
		return finishUntil(until, check, polls, output, pretty)
	}

	for poll := 1; maxPolls == 0 || poll <= maxPolls; poll++ {
		requestCtx, cancel := shared.ContextWithTimeout(ctx)
		resp, err := collectDashboard(requestCtx, client, appID, platform, includes, true)
		cancel()
		if err != nil {
			if watchContextDone(ctx) {
				return finish()
			}
			return fmt.Errorf("status: %w", err)
		}
		polls = poll

		current, err := buildDashboardSnapshotSignature(resp)
		if err != nil {
			return fmt.Errorf("status: encode watch snapshot: %w", err)
		}
		changed := poll > 1 && current != seen
		if poll == 1 || changed {
			if err := printWatchSnapshot(resp, output, pretty, poll > 1); err != nil {
				return err
			}
			seen = current
		}

		if check = evaluateUntil(until, resp, changed); check.outcome != "" {
			return finish()
		}

		if maxPolls > 0 && poll >= maxPolls {
			return finish()
		}
		if err := waitForNextPoll(ctx, pollInterval); err != nil {
			if watchContextDone(ctx) {
				return finish()
			}
			return err
		}
	}

	return finish()
}

func watchContextDone(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	return errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded)
}

func buildDashboardSnapshotSignature(resp *dashboardResponse) (string, error) {
	data, err := json.Marshal(normalizeDashboardSnapshot(resp))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func normalizeDashboardSnapshot(resp *dashboardResponse) *dashboardResponse {
	if resp == nil {
		return nil
	}

	normalized := *resp
	normalized.Summary = resp.Summary
	normalized.Summary.Blockers = normalizeStringSlice(resp.Summary.Blockers)
	if resp.Submission != nil {
		submission := *resp.Submission
		submission.BlockingIssues = normalizeStringSlice(resp.Submission.BlockingIssues)
		normalized.Submission = &submission
	}
	return &normalized
}

func normalizeStringSlice(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return append([]string(nil), values...)
}

func printWatchSnapshot(resp *dashboardResponse, output string, pretty bool, separator bool) error {
	format := strings.ToLower(strings.TrimSpace(output))
	if format == "" {
		format = shared.DefaultOutputFormat()
	}
	switch format {
	case "json":
		var (
			data []byte
			err  error
		)
		if pretty {
			data, err = json.MarshalIndent(resp, "", "  ")
		} else {
			data, err = json.Marshal(resp)
		}
		if err != nil {
			return fmt.Errorf("status: encode watch snapshot: %w", err)
		}
		_, err = fmt.Fprintln(os.Stdout, string(data))
		return err
	case "table":
		if separator {
			fmt.Fprintln(os.Stdout)
		}
		renderTable(resp)
		return nil
	case "markdown", "md":
		if separator {
			fmt.Fprintln(os.Stdout, "\n---")
		}
		renderMarkdown(resp)
		return nil
	default:
		return shared.PrintOutputWithRenderers(
			resp,
			output,
			pretty,
			func() error { renderTable(resp); return nil },
			func() error { renderMarkdown(resp); return nil },
		)
	}
}

func waitForNextPoll(ctx context.Context, pollInterval time.Duration) error {
	timer := time.NewTimer(pollInterval)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func parseInclude(value string) (includeSet, error) {
	parts := shared.SplitCSV(strings.ToLower(strings.TrimSpace(value)))
	if len(parts) == 0 {
		return includeSet{
			app:           true,
			builds:        true,
			testflight:    true,
			appstore:      true,
			submission:    true,
			review:        true,
			phasedRelease: true,
			links:         true,
		}, nil
	}

	includes := includeSet{}
	for _, part := range parts {
		switch part {
		case "app":
			includes.app = true
		case "builds":
			includes.builds = true
		case "testflight":
			includes.testflight = true
		case "appstore":
			includes.appstore = true
		case "submission":
			includes.submission = true
		case "review":
			includes.review = true
		case "phased-release":
			includes.phasedRelease = true
		case "links":
			includes.links = true
		default:
			return includeSet{}, fmt.Errorf("--include contains unsupported section %q (allowed: %s)", part, strings.Join(allowedIncludes, ","))
		}
	}

	return includes, nil
}

func collectDashboard(ctx context.Context, client *asc.Client, appID string, platform string, includes includeSet, watchMode bool) (*dashboardResponse, error) {
	resp := &dashboardResponse{}
	if includes.app {
		appResp, err := client.GetApp(ctx, appID)
		if err != nil {
			return nil, err
		}
		resp.App = &statusApp{
			ID:       appResp.Data.ID,
			BundleID: appResp.Data.Attributes.BundleID,
			Name:     appResp.Data.Attributes.Name,
		}
	}

	if includes.links {
		resp.Links = &linksSection{
			AppStoreConnect: fmt.Sprintf("https://appstoreconnect.apple.com/apps/%s", appID),
			TestFlight:      fmt.Sprintf("https://appstoreconnect.apple.com/apps/%s/testflight/ios", appID),
			Review:          fmt.Sprintf("https://appstoreconnect.apple.com/apps/%s/appstore/review", appID),
		}
	}

	var tasks []sectionTask

	if includes.builds || includes.testflight {
		tasks = append(tasks, sectionTask{
			name: "builds/testflight",
			run: func() error {
				return fillBuildsAndTestFlight(ctx, client, appID, platform, includes, resp)
			},
		})
	}
	if includes.appstore || includes.phasedRelease {
		tasks = append(tasks, sectionTask{
			name: "appstore/phased-release",
			run: func() error {
				return fillAppStoreAndPhasedRelease(ctx, client, appID, platform, includes, resp)
			},
		})
	}
	if includes.submission || includes.review {
		tasks = append(tasks, sectionTask{
			name: "submission/review",
			run: func() error {
				return fillSubmissionAndReview(ctx, client, appID, platform, includes, resp, watchMode)
			},
		})
	}

	if err := runTasks(tasks, 3); err != nil {
		return nil, err
	}
	resp.Summary = buildStatusSummary(resp)
	resp.Summary.NextCommands = resolveNextCommands(resp, appID, platform)
	resp.Summary.Platform = platform

	return resp, nil
}

func runTasks(tasks []sectionTask, limit int) error {
	if len(tasks) == 0 {
		return nil
	}

	if limit < 1 {
		limit = 1
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, limit)
	errCh := make(chan error, len(tasks))

	for _, task := range tasks {
		current := task
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := current.run(); err != nil {
				errCh <- fmt.Errorf("%s: %w", current.name, err)
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		return err
	}
	return nil
}

func fillBuildsAndTestFlight(ctx context.Context, client *asc.Client, appID string, platform string, includes includeSet, resp *dashboardResponse) error {
	buildIncludes := []string{"preReleaseVersion"}
	if includes.testflight {
		buildIncludes = append(buildIncludes, "buildBetaDetail")
	}
	buildOpts := []asc.BuildsOption{
		asc.WithBuildsSort("-uploadedDate"),
		asc.WithBuildsLimit(50),
		asc.WithBuildsInclude(buildIncludes),
	}
	if platform != "" {
		buildOpts = append(buildOpts, asc.WithBuildsPreReleaseVersionPlatforms([]string{platform}))
	}
	buildsResp, err := client.GetBuilds(ctx, appID, buildOpts...)
	if err != nil {
		return err
	}

	var latest *asc.Resource[asc.BuildAttributes]
	if len(buildsResp.Data) > 0 {
		latest = &buildsResp.Data[0]
	}
	buildsByID := buildReviewContexts(buildsResp)
	var latestContext *betaReviewBuildStatus
	if latest != nil {
		latestContext = buildsByID[latest.ID]
		if latestContext == nil {
			latestContext = &betaReviewBuildStatus{
				ID:          latest.ID,
				BuildNumber: latest.Attributes.Version,
			}
			buildsByID[latest.ID] = latestContext
		}
		if latestContext.Version == "" || latestContext.Platform == "" {
			preRelease, preErr := client.GetBuildPreReleaseVersion(ctx, latest.ID)
			if preErr != nil {
				if !asc.IsNotFound(preErr) {
					return preErr
				}
			} else {
				latestContext.Version = preRelease.Data.Attributes.Version
				latestContext.Platform = string(preRelease.Data.Attributes.Platform)
			}
		}
	}

	if includes.builds {
		section := &buildsSection{}
		if latest != nil {
			entry := &latestBuild{
				ID:                 latest.ID,
				Version:            latestContext.Version,
				BuildNumber:        latest.Attributes.Version,
				ProcessingState:    latest.Attributes.ProcessingState,
				appStoreRestricted: strings.EqualFold(strings.TrimSpace(string(latest.Attributes.BuildAudienceType)), string(asc.BuildAudienceTypeInternalOnly)),
				Expired:            optionalBuildExpired(latest.Attributes),
				UploadedDate:       latest.Attributes.UploadedDate,
				Platform:           latestContext.Platform,
			}
			section.Latest = entry
		}
		resp.Builds = section
	}

	if !includes.testflight {
		return nil
	}

	section := &testFlightSection{latestBuild: latestContext}
	if len(buildsResp.Data) == 0 {
		resp.TestFlight = section
		return nil
	}

	betaStatesByBuild := buildBetaStatesByBuildID(buildsResp)

	// The latest build carries the internal state operators need: it can still be
	// PROCESSING for internal testers while processingState already reads VALID.
	if latest != nil {
		section.InternalBuildState = strings.ToUpper(strings.TrimSpace(betaStatesByBuild[latest.ID].internal))
	}

	for _, build := range buildsResp.Data {
		state := strings.ToUpper(strings.TrimSpace(betaStatesByBuild[build.ID].external))
		if isDistributedState(state) {
			section.LatestDistributedBuildID = build.ID
			section.ExternalBuildState = state
			break
		}
	}

	reviewSubmissions, err := fetchBetaReviewSubmissionsForStatus(ctx, client, appID, platform, buildsResp, buildsByID)
	if err != nil {
		return err
	}
	reviewBuildsBySubmissionID := make(map[string]*betaReviewBuildStatus, len(reviewSubmissions.Data))
	missingActiveBuilds := make([]asc.Resource[asc.BetaAppReviewSubmissionAttributes], 0)
	attemptedBuildFallbacks := make(map[string]struct{}, maxBetaReviewBuildPrefetches)
	for _, submission := range reviewSubmissions.Data {
		reviewBuild := reviewBuildForSubmission(submission, buildsByID)
		if reviewBuild != nil {
			reviewBuildsBySubmissionID[submission.ID] = reviewBuild
		}
		if isInProgressBetaReviewState(submission.Attributes.BetaReviewState) && betaReviewBuildContextIncomplete(reviewBuild) {
			missingActiveBuilds = append(missingActiveBuilds, submission)
		}
	}
	// include=build is the normal correlation path. Prefetch at most five partial
	// active contexts, then reserve one final fallback for the selected submission.
	// This caps resolution at six contexts and at most twelve related API requests.
	sortBetaReviewSubmissionsLatestFirst(missingActiveBuilds)
	for index, submission := range missingActiveBuilds {
		if index >= maxBetaReviewBuildPrefetches {
			break
		}
		attemptedBuildFallbacks[submission.ID] = struct{}{}
		if reviewBuild := resolveBetaReviewBuildContext(ctx, client, submission, buildsByID); reviewBuild != nil {
			reviewBuildsBySubmissionID[submission.ID] = reviewBuild
		}
	}

	latestReviewSubmission := selectBetaReviewSubmissionForLatestBuild(reviewSubmissions.Data, latestContext, buildsByID, reviewBuildsBySubmissionID)
	if latestReviewSubmission != nil {
		reviewBuild := reviewBuildsBySubmissionID[latestReviewSubmission.ID]
		_, fallbackAttempted := attemptedBuildFallbacks[latestReviewSubmission.ID]
		if betaReviewBuildContextIncomplete(reviewBuild) && !fallbackAttempted {
			if resolvedBuild := resolveBetaReviewBuildContext(ctx, client, *latestReviewSubmission, buildsByID); resolvedBuild != nil {
				reviewBuildsBySubmissionID[latestReviewSubmission.ID] = resolvedBuild
			}
			latestReviewSubmission = selectBetaReviewSubmissionForLatestBuild(reviewSubmissions.Data, latestContext, buildsByID, reviewBuildsBySubmissionID)
			reviewBuild = reviewBuildsBySubmissionID[latestReviewSubmission.ID]
		}

		section.BetaReviewState = latestReviewSubmission.Attributes.BetaReviewState
		section.SubmittedDate = latestReviewSubmission.Attributes.SubmittedDate
		section.BetaReviewSubmission = &betaReviewSubmissionStatus{
			ID:                    latestReviewSubmission.ID,
			State:                 latestReviewSubmission.Attributes.BetaReviewState,
			SubmittedDate:         latestReviewSubmission.Attributes.SubmittedDate,
			RelationToLatestBuild: betaReviewBuildRelation(latestContext, reviewBuild),
			Build:                 reviewBuild,
		}
	}

	resp.TestFlight = section
	return nil
}

func fetchBetaReviewSubmissionsForStatus(
	ctx context.Context,
	client *asc.Client,
	appID string,
	platform string,
	latestBuilds *asc.BuildsResponse,
	buildsByID map[string]*betaReviewBuildStatus,
) (*asc.BetaAppReviewSubmissionsResponse, error) {
	result := &asc.BetaAppReviewSubmissionsResponse{}
	seenSubmissions := make(map[string]struct{})
	appendSubmissions := func(firstPage *asc.BetaAppReviewSubmissionsResponse) error {
		paginated, err := asc.PaginateAll(ctx, firstPage, func(pageCtx context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return client.GetBetaAppReviewSubmissions(pageCtx, asc.NextURL[asc.BetaAppReviewSubmissionsOption](nextURL))
		})
		if err != nil {
			return err
		}
		all, ok := paginated.(*asc.BetaAppReviewSubmissionsResponse)
		if !ok {
			return fmt.Errorf("unexpected beta review submissions response type %T", paginated)
		}
		for _, submission := range all.Data {
			if submission.ID != "" {
				if _, exists := seenSubmissions[submission.ID]; exists {
					continue
				}
				seenSubmissions[submission.ID] = struct{}{}
			}
			result.Data = append(result.Data, submission)
		}
		return nil
	}

	latestBuildIDs := make(map[string]struct{}, len(latestBuilds.Data))
	buildIDs := make([]string, 0, len(latestBuilds.Data))
	for _, build := range latestBuilds.Data {
		latestBuildIDs[build.ID] = struct{}{}
		buildIDs = append(buildIDs, build.ID)
	}
	if len(buildIDs) > 0 {
		firstPage, err := client.GetBetaAppReviewSubmissions(
			ctx,
			asc.WithBetaAppReviewSubmissionsBuildIDs(buildIDs),
			asc.WithBetaAppReviewSubmissionsIncludeBuild(),
			asc.Limit[asc.BetaAppReviewSubmissionsOption](200),
		)
		if err != nil {
			return nil, err
		}
		if err := appendSubmissions(firstPage); err != nil {
			return nil, err
		}
	}

	activeBuildOpts := []asc.BuildsOption{
		asc.WithBuildsBetaReviewStates([]string{"WAITING_FOR_REVIEW", "IN_REVIEW"}),
		asc.WithBuildsLimit(50),
		asc.WithBuildsInclude([]string{"preReleaseVersion"}),
	}
	if platform != "" {
		activeBuildOpts = append(activeBuildOpts, asc.WithBuildsPreReleaseVersionPlatforms([]string{platform}))
	}
	activeBuilds, err := client.GetBuilds(ctx, appID, activeBuildOpts...)
	if err != nil {
		return nil, err
	}

	err = asc.PaginateEach(ctx, activeBuilds, func(pageCtx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetBuilds(pageCtx, appID, asc.WithBuildsNextURL(nextURL))
	}, func(page asc.PaginatedResponse) error {
		buildPage, ok := page.(*asc.BuildsResponse)
		if !ok {
			return fmt.Errorf("unexpected active beta review builds response type %T", page)
		}
		for buildID, buildContext := range buildReviewContexts(buildPage) {
			if existing := buildsByID[buildID]; existing == nil || betaReviewBuildContextIncomplete(existing) {
				buildsByID[buildID] = buildContext
			}
		}

		olderActiveBuildIDs := make([]string, 0, len(buildPage.Data))
		for _, build := range buildPage.Data {
			if _, alreadyQueried := latestBuildIDs[build.ID]; !alreadyQueried {
				olderActiveBuildIDs = append(olderActiveBuildIDs, build.ID)
			}
		}
		if len(olderActiveBuildIDs) == 0 {
			return nil
		}

		firstPage, err := client.GetBetaAppReviewSubmissions(
			ctx,
			asc.WithBetaAppReviewSubmissionsBuildIDs(olderActiveBuildIDs),
			asc.WithBetaAppReviewSubmissionsIncludeBuild(),
			asc.Limit[asc.BetaAppReviewSubmissionsOption](200),
		)
		if err != nil {
			return err
		}
		return appendSubmissions(firstPage)
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func buildBetaStatesByBuildID(builds *asc.BuildsResponse) map[string]betaBuildStates {
	statesByBuild := make(map[string]betaBuildStates, len(builds.Data))
	if len(builds.Included) == 0 {
		return statesByBuild
	}

	var included []asc.Resource[asc.BuildBetaDetailAttributes]
	if err := json.Unmarshal(builds.Included, &included); err != nil {
		return statesByBuild
	}
	statesByDetail := make(map[string]betaBuildStates, len(included))
	for _, resource := range included {
		if resource.Type == asc.ResourceTypeBuildBetaDetails {
			statesByDetail[resource.ID] = betaBuildStatesFromAttributes(resource.Attributes)
		}
	}

	for _, build := range builds.Data {
		if detailID, ok := optionalRelationshipResourceID(build.Relationships, "buildBetaDetail"); ok {
			if states, found := statesByDetail[detailID]; found {
				statesByBuild[build.ID] = states
			}
		}
	}

	return statesByBuild
}

func optionalBuildExpired(attributes asc.BuildAttributes) *bool {
	expired, known := attributes.ExpiredValue()
	if !known {
		return nil
	}
	return &expired
}

func betaBuildStatesFromAttributes(attributes asc.BuildBetaDetailAttributes) betaBuildStates {
	return betaBuildStates{
		internal: strings.TrimSpace(attributes.InternalBuildState),
		external: strings.TrimSpace(attributes.ExternalBuildState),
	}
}

func optionalRelationshipResourceID(relationships json.RawMessage, key string) (string, bool) {
	if len(relationships) == 0 {
		return "", false
	}

	var references map[string]relationshipReference
	if err := json.Unmarshal(relationships, &references); err != nil {
		return "", false
	}

	reference, ok := references[key]
	if !ok {
		return "", false
	}

	id := strings.TrimSpace(reference.Data.ID)
	if id == "" {
		return "", false
	}

	return id, true
}

func buildReviewContexts(builds *asc.BuildsResponse) map[string]*betaReviewBuildStatus {
	contexts := make(map[string]*betaReviewBuildStatus)
	if builds == nil {
		return contexts
	}

	preReleaseVersions := make(map[string]asc.PreReleaseVersionAttributes)
	if len(builds.Included) > 0 {
		var included []asc.Resource[asc.PreReleaseVersionAttributes]
		if err := json.Unmarshal(builds.Included, &included); err == nil {
			for _, resource := range included {
				if resource.Type == asc.ResourceTypePreReleaseVersions {
					preReleaseVersions[resource.ID] = resource.Attributes
				}
			}
		}
	}

	for _, build := range builds.Data {
		context := &betaReviewBuildStatus{
			ID:          build.ID,
			BuildNumber: build.Attributes.Version,
		}
		if preReleaseID, ok := optionalRelationshipResourceID(build.Relationships, "preReleaseVersion"); ok {
			if preRelease, found := preReleaseVersions[preReleaseID]; found {
				context.Version = preRelease.Version
				context.Platform = string(preRelease.Platform)
			}
		}
		contexts[build.ID] = context
	}

	return contexts
}

func reviewBuildForSubmission(submission asc.Resource[asc.BetaAppReviewSubmissionAttributes], buildsByID map[string]*betaReviewBuildStatus) *betaReviewBuildStatus {
	buildID, ok := optionalRelationshipResourceID(submission.Relationships, "build")
	if !ok {
		return nil
	}
	if build := buildsByID[buildID]; build != nil {
		return build
	}
	return &betaReviewBuildStatus{ID: buildID}
}

func resolveBetaReviewBuildContext(
	ctx context.Context,
	client *asc.Client,
	submission asc.Resource[asc.BetaAppReviewSubmissionAttributes],
	buildsByID map[string]*betaReviewBuildStatus,
) *betaReviewBuildStatus {
	reviewBuild := reviewBuildForSubmission(submission, buildsByID)
	if reviewBuild == nil || reviewBuild.BuildNumber == "" {
		relatedBuild, err := client.GetBetaAppReviewSubmissionBuild(ctx, submission.ID)
		if err != nil || relatedBuild == nil || strings.TrimSpace(relatedBuild.Data.ID) == "" {
			return reviewBuild
		}
		reviewBuild = buildsByID[relatedBuild.Data.ID]
		if reviewBuild == nil {
			reviewBuild = &betaReviewBuildStatus{ID: relatedBuild.Data.ID}
			buildsByID[relatedBuild.Data.ID] = reviewBuild
		}
		if reviewBuild.BuildNumber == "" {
			reviewBuild.BuildNumber = relatedBuild.Data.Attributes.Version
		}
	}

	if reviewBuild.Version == "" || reviewBuild.Platform == "" {
		if preRelease, err := client.GetBuildPreReleaseVersion(ctx, reviewBuild.ID); err == nil && preRelease != nil {
			reviewBuild.Version = preRelease.Data.Attributes.Version
			reviewBuild.Platform = string(preRelease.Data.Attributes.Platform)
		}
	}
	return reviewBuild
}

func betaReviewBuildContextIncomplete(build *betaReviewBuildStatus) bool {
	return build == nil || build.BuildNumber == "" || build.Version == "" || build.Platform == ""
}

func betaReviewBuildRelation(latest, review *betaReviewBuildStatus) string {
	if latest == nil || review == nil || strings.TrimSpace(review.ID) == "" {
		return "unknown"
	}
	if latest.ID == review.ID {
		return "sameBuild"
	}
	if sameBetaReviewVersionTrain(latest, review) {
		return "sameVersionTrain"
	}
	if latest.Version != "" && latest.Platform != "" && review.Version != "" && review.Platform != "" {
		return "differentVersionTrain"
	}
	return "unknown"
}

func sameBetaReviewVersionTrain(first, second *betaReviewBuildStatus) bool {
	if first == nil || second == nil {
		return false
	}
	return strings.TrimSpace(first.Version) != "" &&
		strings.EqualFold(first.Version, second.Version) &&
		strings.TrimSpace(first.Platform) != "" &&
		strings.EqualFold(first.Platform, second.Platform)
}

func fillAppStoreAndPhasedRelease(ctx context.Context, client *asc.Client, appID string, platform string, includes includeSet, resp *dashboardResponse) error {
	versionOpts := []asc.AppStoreVersionsOption{asc.Limit[asc.AppStoreVersionsOption](200)}
	if platform != "" {
		versionOpts = append(versionOpts, asc.WithAppStoreVersionsPlatforms([]string{platform}))
	}
	versions, err := shared.FetchAllAppStoreVersions(ctx, client, appID, versionOpts...)
	if err != nil {
		return err
	}

	latestVersion := selectLatestAppStoreVersion(versions)
	if includes.appstore {
		section := &appStoreSection{}
		if latestVersion != nil {
			section.VersionID = latestVersion.ID
			section.Version = latestVersion.Attributes.VersionString
			section.State = shared.ResolveAppStoreVersionState(latestVersion.Attributes)
			section.Platform = string(latestVersion.Attributes.Platform)
			section.CreatedDate = latestVersion.Attributes.CreatedDate
		}
		resp.AppStore = section
	}

	if !includes.phasedRelease {
		return nil
	}

	phased := &phasedReleaseSection{Configured: false}
	if latestVersion != nil {
		phaseResp, phaseErr := client.GetAppStoreVersionPhasedRelease(ctx, latestVersion.ID)
		if phaseErr != nil {
			if !asc.IsNotFound(phaseErr) {
				return phaseErr
			}
		} else if phaseResp.Data != nil {
			phased.Configured = true
			phased.ID = phaseResp.Data.ID
			phased.State = string(phaseResp.Data.Attributes.PhasedReleaseState)
			phased.StartDate = phaseResp.Data.Attributes.StartDate
			phased.CurrentDayNumber = phaseResp.Data.Attributes.CurrentDayNumber
			phased.TotalPauseDuration = phaseResp.Data.Attributes.TotalPauseDuration
		}
	}

	resp.PhasedRelease = phased
	return nil
}

func fillSubmissionAndReview(ctx context.Context, client *asc.Client, appID string, platform string, includes includeSet, resp *dashboardResponse, watchMode bool) error {
	submissions, err := fetchStatusReviewSubmissions(ctx, client, appID, platform, watchMode)
	if err != nil {
		return err
	}
	latest := selectLatestReviewSubmission(submissions)
	latestByPlatform := selectLatestReviewSubmissionsByPlatform(submissions)

	if includes.submission {
		section := &submissionSection{
			InFlight:       false,
			BlockingIssues: []string{},
		}
		for _, submission := range latestByPlatform {
			state := string(submission.Attributes.SubmissionState)
			if isInFlightSubmissionState(state) {
				section.InFlight = true
			}
			if strings.EqualFold(state, string(asc.ReviewSubmissionStateUnresolvedIssues)) {
				section.BlockingIssues = append(section.BlockingIssues, fmt.Sprintf("submission %s has unresolved issues", submission.ID))
			}
		}
		slices.Sort(section.BlockingIssues)
		resp.Submission = section
	}

	if includes.review {
		section := &reviewSection{}
		if latest != nil {
			section.LatestSubmissionID = latest.ID
			section.State = string(latest.Attributes.SubmissionState)
			section.SubmittedDate = latest.Attributes.SubmittedDate
			section.Platform = string(latest.Attributes.Platform)
		}
		resp.Review = section
	}

	return nil
}

func fetchStatusReviewSubmissions(ctx context.Context, client *asc.Client, appID string, platform string, watchMode bool) ([]asc.ReviewSubmissionResource, error) {
	opts := []asc.ReviewSubmissionsOption{asc.Limit[asc.ReviewSubmissionsOption](200)}
	if platform != "" {
		opts = append(opts, asc.WithReviewSubmissionsPlatforms([]string{platform}))
	}
	if !watchMode {
		return shared.FetchAllReviewSubmissions(ctx, client, appID, opts...)
	}

	// Watch mode uses a bounded recent snapshot instead of walking submission history on every poll.
	resp, err := client.GetReviewSubmissions(ctx, appID, opts...)
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.Data == nil {
		return []asc.ReviewSubmissionResource{}, nil
	}
	return resp.Data, nil
}

func selectLatestAppStoreVersion(versions []asc.Resource[asc.AppStoreVersionAttributes]) *asc.Resource[asc.AppStoreVersionAttributes] {
	if len(versions) == 0 {
		return nil
	}

	best := versions[0]
	for _, current := range versions[1:] {
		dateOrder := shared.CompareRFC3339DateStrings(current.Attributes.CreatedDate, best.Attributes.CreatedDate)
		if dateOrder > 0 {
			best = current
			continue
		}
		if dateOrder == 0 && current.ID > best.ID {
			best = current
		}
	}
	return &best
}

func selectLatestReviewSubmission(submissions []asc.ReviewSubmissionResource) *asc.ReviewSubmissionResource {
	if len(submissions) == 0 {
		return nil
	}

	best := submissions[0]
	for _, current := range submissions[1:] {
		if shared.ShouldPreferLatestReviewSubmission(current, best) {
			best = current
		}
	}
	return &best
}

func selectLatestReviewSubmissionsByPlatform(submissions []asc.ReviewSubmissionResource) []asc.ReviewSubmissionResource {
	if len(submissions) == 0 {
		return nil
	}

	latest := make(map[string]asc.ReviewSubmissionResource)
	for _, current := range submissions {
		platformKey := strings.ToUpper(strings.TrimSpace(string(current.Attributes.Platform)))
		if platformKey == "" {
			platformKey = "__UNKNOWN__"
		}
		best, ok := latest[platformKey]
		if !ok || shared.ShouldPreferLatestReviewSubmission(current, best) {
			latest[platformKey] = current
		}
	}

	selected := make([]asc.ReviewSubmissionResource, 0, len(latest))
	for _, submission := range latest {
		selected = append(selected, submission)
	}

	slices.SortFunc(selected, func(a, b asc.ReviewSubmissionResource) int {
		return strings.Compare(a.ID, b.ID)
	})
	return selected
}

func selectLatestBetaReviewSubmission(submissions []asc.Resource[asc.BetaAppReviewSubmissionAttributes]) *asc.Resource[asc.BetaAppReviewSubmissionAttributes] {
	if len(submissions) == 0 {
		return nil
	}

	best := submissions[0]
	for _, current := range submissions[1:] {
		dateOrder := shared.CompareRFC3339DateStrings(current.Attributes.SubmittedDate, best.Attributes.SubmittedDate)
		if dateOrder > 0 {
			best = current
			continue
		}
		if dateOrder == 0 && current.ID > best.ID {
			best = current
		}
	}
	return &best
}

func selectBetaReviewSubmissionForLatestBuild(
	submissions []asc.Resource[asc.BetaAppReviewSubmissionAttributes],
	latestBuild *betaReviewBuildStatus,
	buildsByID map[string]*betaReviewBuildStatus,
	reviewBuildsBySubmissionID map[string]*betaReviewBuildStatus,
) *asc.Resource[asc.BetaAppReviewSubmissionAttributes] {
	if len(submissions) == 0 {
		return nil
	}

	relevantActive := make([]asc.Resource[asc.BetaAppReviewSubmissionAttributes], 0)
	unknownActive := make([]asc.Resource[asc.BetaAppReviewSubmissionAttributes], 0)
	relevantTerminal := make([]asc.Resource[asc.BetaAppReviewSubmissionAttributes], 0)
	for _, submission := range submissions {
		reviewBuild := reviewBuildsBySubmissionID[submission.ID]
		if reviewBuild == nil {
			reviewBuild = reviewBuildForSubmission(submission, buildsByID)
		}
		relation := betaReviewBuildRelation(latestBuild, reviewBuild)
		if relation == "unknown" && isInProgressBetaReviewState(submission.Attributes.BetaReviewState) {
			unknownActive = append(unknownActive, submission)
			continue
		}
		if relation != "sameBuild" && relation != "sameVersionTrain" {
			continue
		}
		if isInProgressBetaReviewState(submission.Attributes.BetaReviewState) {
			relevantActive = append(relevantActive, submission)
		} else {
			relevantTerminal = append(relevantTerminal, submission)
		}
	}

	if selected := selectLatestBetaReviewSubmission(relevantActive); selected != nil {
		return selected
	}
	if selected := selectLatestBetaReviewSubmission(unknownActive); selected != nil {
		return selected
	}
	if selected := selectLatestBetaReviewSubmission(relevantTerminal); selected != nil {
		return selected
	}
	return selectLatestBetaReviewSubmission(submissions)
}

func sortBetaReviewSubmissionsLatestFirst(submissions []asc.Resource[asc.BetaAppReviewSubmissionAttributes]) {
	slices.SortFunc(submissions, func(first, second asc.Resource[asc.BetaAppReviewSubmissionAttributes]) int {
		dateOrder := shared.CompareRFC3339DateStrings(first.Attributes.SubmittedDate, second.Attributes.SubmittedDate)
		if dateOrder != 0 {
			return -dateOrder
		}
		return -strings.Compare(first.ID, second.ID)
	})
}

func isInProgressBetaReviewState(state string) bool {
	switch strings.ToUpper(strings.TrimSpace(state)) {
	case "WAITING_FOR_REVIEW", "IN_REVIEW":
		return true
	default:
		return false
	}
}

func isDistributedState(state string) bool {
	switch strings.ToUpper(strings.TrimSpace(state)) {
	case "IN_BETA_TESTING", "READY_FOR_TESTING":
		return true
	default:
		return false
	}
}

func isInFlightSubmissionState(state string) bool {
	switch strings.ToUpper(strings.TrimSpace(state)) {
	case string(asc.ReviewSubmissionStateReadyForReview),
		string(asc.ReviewSubmissionStateWaitingForReview),
		string(asc.ReviewSubmissionStateInReview),
		string(asc.ReviewSubmissionStateUnresolvedIssues),
		string(asc.ReviewSubmissionStateCanceling):
		return true
	default:
		return false
	}
}

func buildStatusSummary(resp *dashboardResponse) statusSummary {
	blockers := collectBlockers(resp)
	health := resolveHealth(resp, blockers)
	return statusSummary{
		Health:     health,
		NextAction: resolveNextAction(resp, blockers),
		Blockers:   blockers,
	}
}

func collectBlockers(resp *dashboardResponse) []string {
	blockers := make([]string, 0)
	if resp == nil {
		return blockers
	}

	if resp.Submission != nil && len(resp.Submission.BlockingIssues) > 0 {
		blockers = append(blockers, resp.Submission.BlockingIssues...)
	}

	if resp.Review != nil {
		state := strings.ToUpper(strings.TrimSpace(resp.Review.State))
		switch state {
		case "UNRESOLVED_ISSUES":
			blockers = append(blockers, "App Store review has unresolved issues")
		case "DEVELOPER_REJECTED", "REJECTED":
			blockers = append(blockers, "App Store review is rejected")
		}
	}

	if resp.AppStore != nil {
		state := strings.ToUpper(strings.TrimSpace(resp.AppStore.State))
		switch state {
		case "DEVELOPER_REJECTED", "REJECTED", "METADATA_REJECTED", "INVALID_BINARY":
			blockers = append(blockers, fmt.Sprintf("App Store version is in blocking state %s", state))
		}
	}

	if resp.Builds != nil && resp.Builds.Latest == nil {
		blockers = append(blockers, "No builds found for this app")
	}
	if blocker := betaReviewBlocker(resp); blocker != "" {
		blockers = append(blockers, blocker)
	}

	slices.Sort(blockers)
	return slices.Compact(blockers)
}

func resolveHealth(resp *dashboardResponse, blockers []string) string {
	if len(blockers) > 0 {
		return "red"
	}
	if resp == nil {
		return "yellow"
	}

	if resp.Submission != nil && resp.Submission.InFlight {
		return "yellow"
	}
	if resp.Review != nil && isInProgressReviewState(resp.Review.State) {
		return "yellow"
	}
	if resp.AppStore != nil && isInProgressAppStoreState(resp.AppStore.State) {
		return "yellow"
	}
	if resp.TestFlight != nil && resp.TestFlight.BetaReviewSubmission != nil {
		review := resp.TestFlight.BetaReviewSubmission
		if isInProgressBetaReviewState(review.State) && (review.RelationToLatestBuild == "sameBuild" || review.RelationToLatestBuild == "unknown") {
			return "yellow"
		}
		if strings.EqualFold(strings.TrimSpace(review.State), "REJECTED") && (review.RelationToLatestBuild == "sameBuild" || review.RelationToLatestBuild == "sameVersionTrain") {
			return "yellow"
		}
	}

	return "green"
}

func resolveNextAction(resp *dashboardResponse, blockers []string) string {
	if len(blockers) > 0 {
		if action := betaReviewBlockerNextAction(resp); action != "" && len(blockers) == 1 {
			return action
		}
		return fmt.Sprintf("Resolve blocker: %s", blockers[0])
	}
	if resp == nil {
		return "Review release status."
	}

	if resp.Submission != nil && resp.Submission.InFlight {
		return "Wait for App Store review outcome."
	}
	if resp.Review != nil && isInProgressReviewState(resp.Review.State) {
		return "Monitor App Store review progress."
	}
	if resp.TestFlight != nil && resp.TestFlight.BetaReviewSubmission != nil {
		review := resp.TestFlight.BetaReviewSubmission
		if isInProgressBetaReviewState(review.State) {
			switch review.RelationToLatestBuild {
			case "sameBuild":
				return fmt.Sprintf("Wait for Beta App Review of %s to finish.", betaReviewBuildLabel(review.Build))
			case "unknown":
				return fmt.Sprintf("Inspect Beta App Review submission %s to identify its build.", review.ID)
			}
		}
		if strings.EqualFold(strings.TrimSpace(review.State), "REJECTED") && (review.RelationToLatestBuild == "sameBuild" || review.RelationToLatestBuild == "sameVersionTrain") {
			return fmt.Sprintf("Review Beta App Review feedback for %s before the next external testing submission.", betaReviewBuildLabel(review.Build))
		}
	}
	if resp.AppStore != nil {
		state := strings.ToUpper(strings.TrimSpace(resp.AppStore.State))
		switch {
		case state == "PREPARE_FOR_SUBMISSION":
			return "Prepare metadata and submit for review."
		case shared.IsLiveAppStoreVersionState(state):
			return "No action needed."
		}
	}
	if resp.Builds != nil && resp.Builds.Latest == nil {
		return "Upload a build to App Store Connect."
	}
	if resp.TestFlight != nil && resp.TestFlight.ExternalBuildState == "" && resp.TestFlight.BetaReviewState == "" {
		return "Decide whether to submit a build for external TestFlight."
	}

	return "Review release status."
}

func betaReviewBlocker(resp *dashboardResponse) string {
	if resp == nil || resp.TestFlight == nil || resp.TestFlight.BetaReviewSubmission == nil {
		return ""
	}
	review := resp.TestFlight.BetaReviewSubmission
	latest := resp.TestFlight.latestBuild
	if review.RelationToLatestBuild != "sameVersionTrain" || !isInProgressBetaReviewState(review.State) || review.Build == nil || latest == nil || review.Build.ID == latest.ID {
		return ""
	}
	if resp.TestFlight.LatestDistributedBuildID == latest.ID {
		return ""
	}
	return fmt.Sprintf(
		"Beta App Review for %s is %s and blocks external testing for latest %s",
		betaReviewBuildLabel(review.Build),
		strings.ToUpper(strings.TrimSpace(review.State)),
		betaReviewBuildLabel(latest),
	)
}

func betaReviewBlockerNextAction(resp *dashboardResponse) string {
	if betaReviewBlocker(resp) == "" {
		return ""
	}
	review := resp.TestFlight.BetaReviewSubmission
	return fmt.Sprintf(
		"Wait for Beta App Review of %s to finish before submitting %s for external testing.",
		betaReviewBuildLabel(review.Build),
		betaReviewBuildLabel(resp.TestFlight.latestBuild),
	)
}

func betaReviewBuildLabel(build *betaReviewBuildStatus) string {
	if build == nil {
		return "unknown build"
	}
	identifier := strings.TrimSpace(build.BuildNumber)
	if identifier == "" {
		identifier = strings.TrimSpace(build.ID)
	}
	if identifier == "" {
		return "unknown build"
	}
	label := "build " + identifier
	if strings.TrimSpace(build.Version) != "" {
		label += " (version " + strings.TrimSpace(build.Version) + ")"
	}
	return label
}

func isInProgressReviewState(state string) bool {
	switch strings.ToUpper(strings.TrimSpace(state)) {
	case "WAITING_FOR_REVIEW", "IN_REVIEW":
		return true
	default:
		return false
	}
}

func isInProgressAppStoreState(state string) bool {
	switch strings.ToUpper(strings.TrimSpace(state)) {
	case "PREPARE_FOR_SUBMISSION", "WAITING_FOR_REVIEW", "IN_REVIEW", "PENDING_DEVELOPER_RELEASE", "PENDING_APPLE_RELEASE", "PROCESSING_FOR_DISTRIBUTION":
		return true
	default:
		return false
	}
}

func phasedReleaseProgressBar(phased *phasedReleaseSection) string {
	if phased == nil {
		return "n/a"
	}
	if !phased.Configured {
		return "not configured"
	}
	return asc.FormatPhasedReleaseProgressBar(phased.CurrentDayNumber)
}

func renderTable(resp *dashboardResponse) {
	renderDashboard(resp, false)
}

func renderMarkdown(resp *dashboardResponse) {
	renderDashboard(resp, true)
}

var statusNow = time.Now

func renderDashboard(resp *dashboardResponse, markdown bool) {
	summary := resp.Summary
	if summary.Health == "" {
		summary = buildStatusSummary(resp)
	}

	summaryRows := [][]string{
		{"health", fmt.Sprintf("%s %s", healthSymbol(summary.Health), shared.OrNA(summary.Health))},
		{"nextAction", shared.OrNA(summary.NextAction)},
		{"blockerCount", fmt.Sprintf("%d", len(summary.Blockers))},
	}
	if summary.Platform != "" {
		summaryRows = append(summaryRows, []string{"platform", summary.Platform})
	}
	shared.RenderSection("Summary", []string{"field", "value"}, summaryRows, markdown)

	if len(summary.Blockers) > 0 {
		attentionRows := make([][]string, 0, len(summary.Blockers))
		for i, blocker := range summary.Blockers {
			attentionRows = append(attentionRows, []string{fmt.Sprintf("[x] blocker_%d", i+1), blocker})
		}
		shared.RenderSection("Needs Attention", []string{"item", "detail"}, attentionRows, markdown)
	}

	if len(summary.NextCommands) > 0 {
		commandRows := make([][]string, 0, len(summary.NextCommands))
		for _, next := range summary.NextCommands {
			commandRows = append(commandRows, []string{next.Command, fmt.Sprintf("%t", next.Mutates), next.Reason})
		}
		shared.RenderSection("Next Commands", []string{"command", "mutates", "reason"}, commandRows, markdown)
	}

	if resp.App != nil {
		shared.RenderSection("App", []string{"field", "value"}, [][]string{
			{"id", resp.App.ID},
			{"name", resp.App.Name},
			{"bundleId", resp.App.BundleID},
		}, markdown)
	}

	if resp.Builds != nil {
		rows := make([][]string, 0)
		if resp.Builds.Latest == nil {
			rows = append(rows, []string{"latest", "[-] none"})
		} else {
			expired := "[-] unknown"
			if resp.Builds.Latest.Expired != nil && !*resp.Builds.Latest.Expired {
				expired = "[+] false"
			}
			if resp.Builds.Latest.Expired != nil && *resp.Builds.Latest.Expired {
				expired = "[x] true"
			}
			rows = append(
				rows,
				[]string{"latest.id", resp.Builds.Latest.ID},
				[]string{"latest.version", shared.OrNA(resp.Builds.Latest.Version)},
				[]string{"latest.buildNumber", shared.OrNA(resp.Builds.Latest.BuildNumber)},
				[]string{"latest.processingState", prefixedState(resp.Builds.Latest.ProcessingState)},
				[]string{"latest.expired", expired},
				[]string{"latest.uploadedDate", formatDateWithRelative(resp.Builds.Latest.UploadedDate)},
				[]string{"latest.platform", shared.OrNA(resp.Builds.Latest.Platform)},
			)
		}
		shared.RenderSection("Builds", []string{"field", "value"}, rows, markdown)
	}

	if resp.TestFlight != nil {
		rows := [][]string{
			{"internalBuildState", prefixedInternalBuildState(resp.TestFlight.InternalBuildState)},
			{"latestDistributedBuildId", shared.OrNA(resp.TestFlight.LatestDistributedBuildID)},
			{"externalBuildState", prefixedState(resp.TestFlight.ExternalBuildState)},
		}
		if review := resp.TestFlight.BetaReviewSubmission; review != nil {
			rows = append(
				rows,
				[]string{"betaReviewSubmission.id", shared.OrNA(review.ID)},
				[]string{"betaReviewSubmission.state", prefixedState(review.State)},
				[]string{"betaReviewSubmission.submittedDate", formatDateWithRelative(review.SubmittedDate)},
				[]string{"betaReviewSubmission.relationToLatestBuild", shared.OrNA(review.RelationToLatestBuild)},
			)
			if review.Build == nil {
				rows = append(rows, []string{"betaReviewSubmission.build", "[-] unknown"})
			} else {
				rows = append(
					rows,
					[]string{"betaReviewSubmission.build.id", shared.OrNA(review.Build.ID)},
					[]string{"betaReviewSubmission.build.version", shared.OrNA(review.Build.Version)},
					[]string{"betaReviewSubmission.build.buildNumber", shared.OrNA(review.Build.BuildNumber)},
					[]string{"betaReviewSubmission.build.platform", shared.OrNA(review.Build.Platform)},
				)
			}
		} else {
			rows = append(rows, []string{"betaReviewSubmission", "[-] none"})
		}
		rows = append(
			rows,
			[]string{"betaReviewState", prefixedState(resp.TestFlight.BetaReviewState)},
			[]string{"submittedDate", formatDateWithRelative(resp.TestFlight.SubmittedDate)},
		)
		shared.RenderSection("TestFlight", []string{"field", "value"}, rows, markdown)
	}

	if resp.AppStore != nil {
		shared.RenderSection("App Store", []string{"field", "value"}, [][]string{
			{"versionId", shared.OrNA(resp.AppStore.VersionID)},
			{"version", shared.OrNA(resp.AppStore.Version)},
			{"state", prefixedState(resp.AppStore.State)},
			{"platform", shared.OrNA(resp.AppStore.Platform)},
			{"createdDate", formatDateWithRelative(resp.AppStore.CreatedDate)},
		}, markdown)
	}

	if resp.Submission != nil {
		inFlight := "[-] false"
		if resp.Submission.InFlight {
			inFlight = "[~] true"
		}
		shared.RenderSection("Submission", []string{"field", "value"}, [][]string{
			{"inFlight", inFlight},
			{"blockingIssueCount", fmt.Sprintf("%d", len(resp.Submission.BlockingIssues))},
		}, markdown)
	}

	if resp.Review != nil {
		shared.RenderSection("Review", []string{"field", "value"}, [][]string{
			{"latestSubmissionId", shared.OrNA(resp.Review.LatestSubmissionID)},
			{"state", prefixedState(resp.Review.State)},
			{"submittedDate", formatDateWithRelative(resp.Review.SubmittedDate)},
			{"platform", shared.OrNA(resp.Review.Platform)},
		}, markdown)
	}

	if resp.PhasedRelease != nil {
		configured := "[-] false"
		if resp.PhasedRelease.Configured {
			configured = "[+] true"
		}
		shared.RenderSection("Phased Release", []string{"field", "value"}, [][]string{
			{"configured", configured},
			{"id", shared.OrNA(resp.PhasedRelease.ID)},
			{"state", prefixedState(resp.PhasedRelease.State)},
			{"startDate", formatDateWithRelative(resp.PhasedRelease.StartDate)},
			{"currentDayNumber", fmt.Sprintf("%d", resp.PhasedRelease.CurrentDayNumber)},
			{"totalPauseDuration", fmt.Sprintf("%d", resp.PhasedRelease.TotalPauseDuration)},
			{"progress", phasedReleaseProgressBar(resp.PhasedRelease)},
		}, markdown)
	}

	if resp.Links != nil {
		shared.RenderSection("Links", []string{"field", "value"}, [][]string{
			{"appStoreConnect", shared.OrNA(resp.Links.AppStoreConnect)},
			{"testFlight", shared.OrNA(resp.Links.TestFlight)},
			{"review", shared.OrNA(resp.Links.Review)},
		}, markdown)
	}
}

func healthSymbol(health string) string {
	switch strings.ToLower(strings.TrimSpace(health)) {
	case "green":
		return "[+]"
	case "yellow":
		return "[~]"
	case "red":
		return "[x]"
	default:
		return "[-]"
	}
}

func prefixedState(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "[-] n/a"
	}
	return fmt.Sprintf("%s %s", stateSymbol(trimmed), trimmed)
}

func prefixedInternalBuildState(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "[-] n/a"
	}
	return fmt.Sprintf("%s %s", internalBuildStateSymbol(trimmed), trimmed)
}

func internalBuildStateSymbol(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "PROCESSING", "IN_EXPORT_COMPLIANCE_REVIEW":
		return "[~]"
	case "PROCESSING_EXCEPTION", "MISSING_EXPORT_COMPLIANCE", "EXPIRED":
		return "[x]"
	case "READY_FOR_BETA_TESTING", "IN_BETA_TESTING":
		return "[+]"
	default:
		return stateSymbol(value)
	}
}

func stateSymbol(value string) string {
	upper := strings.ToUpper(strings.TrimSpace(value))
	if upper == "" {
		return "[-]"
	}
	if strings.Contains(upper, "REJECTED") ||
		strings.Contains(upper, "INVALID") ||
		strings.Contains(upper, "UNRESOLVED") ||
		strings.Contains(upper, "FAILED") ||
		strings.Contains(upper, "ERROR") {
		return "[x]"
	}
	if strings.Contains(upper, "WAITING") ||
		strings.Contains(upper, "IN_REVIEW") ||
		strings.Contains(upper, "FOR_REVIEW") ||
		strings.Contains(upper, "PROCESSING") ||
		strings.Contains(upper, "PENDING") ||
		strings.Contains(upper, "PREPARE") ||
		strings.Contains(upper, "SUBMITTED") ||
		strings.Contains(upper, "IN_PROGRESS") ||
		strings.Contains(upper, "NOT_READY") {
		return "[~]"
	}
	if strings.Contains(upper, "READY") ||
		strings.Contains(upper, "VALID") ||
		strings.Contains(upper, "ACTIVE") ||
		strings.Contains(upper, "APPROVED") ||
		strings.Contains(upper, "COMPLETE") {
		return "[+]"
	}
	return "[-]"
}

func formatDateWithRelative(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "n/a"
	}

	if parsed, ok := parseRelativeDate(trimmed); ok {
		return fmt.Sprintf("%s (%s)", trimmed, relativeTimeText(parsed, statusNow().UTC()))
	}

	return trimmed
}

func parseRelativeDate(value string) (time.Time, bool) {
	if parsed, ok := shared.ParseRFC3339Date(value); ok {
		return parsed.UTC(), true
	}
	if parsed, err := time.Parse("2006-01-02", value); err == nil {
		return time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.UTC), true
	}
	return time.Time{}, false
}

func relativeTimeText(target, now time.Time) string {
	diff := now.Sub(target)
	if diff < 0 {
		return "in " + humanizeDuration(-diff)
	}
	return humanizeDuration(diff) + " ago"
}

func humanizeDuration(value time.Duration) string {
	if value < time.Minute {
		return "less than 1m"
	}
	if value < time.Hour {
		return fmt.Sprintf("%dm", int(value.Minutes()))
	}
	if value < 24*time.Hour {
		return fmt.Sprintf("%dh", int(value.Hours()))
	}
	return fmt.Sprintf("%dd", int(value.Hours()/24))
}
