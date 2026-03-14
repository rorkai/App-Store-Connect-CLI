package status

import (
	"context"
	"encoding/json"
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
	Health     string   `json:"health"`
	NextAction string   `json:"nextAction"`
	Blockers   []string `json:"blockers"`
}

type buildsSection struct {
	Latest *latestBuild `json:"latest,omitempty"`
}

type latestBuild struct {
	ID              string `json:"id"`
	Version         string `json:"version,omitempty"`
	BuildNumber     string `json:"buildNumber"`
	ProcessingState string `json:"processingState,omitempty"`
	UploadedDate    string `json:"uploadedDate,omitempty"`
	Platform        string `json:"platform,omitempty"`
}

type testFlightSection struct {
	LatestDistributedBuildID string `json:"latestDistributedBuildId,omitempty"`
	BetaReviewState          string `json:"betaReviewState,omitempty"`
	ExternalBuildState       string `json:"externalBuildState,omitempty"`
	SubmittedDate            string `json:"submittedDate,omitempty"`
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

// StatusCommand returns the root status dashboard command.
func StatusCommand() *ffcli.Command {
	fs := flag.NewFlagSet("status", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID, bundle ID, or exact app name (required, or ASC_APP_ID env)")
	include := fs.String("include", "", "Comma-separated sections: app,builds,testflight,appstore,submission,review,phased-release,links")
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
  asc status --app "123456789" --include builds,testflight,submission
  asc status --app "123456789" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				fmt.Fprintln(os.Stderr, "Error: status does not accept positional arguments")
				return flag.ErrHelp
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			includes, err := parseInclude(*include)
			if err != nil {
				return shared.UsageError(err.Error())
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("status: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resolvedAppID, err = shared.ResolveAppIDWithLookup(requestCtx, client, resolvedAppID)
			if err != nil {
				return fmt.Errorf("status: %w", err)
			}

			resp, err := collectDashboard(requestCtx, client, resolvedAppID, includes)
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

func collectDashboard(ctx context.Context, client *asc.Client, appID string, includes includeSet) (*dashboardResponse, error) {
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
				return fillBuildsAndTestFlight(ctx, client, appID, includes, resp)
			},
		})
	}
	if includes.appstore || includes.phasedRelease {
		tasks = append(tasks, sectionTask{
			name: "appstore/phased-release",
			run: func() error {
				return fillAppStoreAndPhasedRelease(ctx, client, appID, includes, resp)
			},
		})
	}
	if includes.submission || includes.review {
		tasks = append(tasks, sectionTask{
			name: "submission/review",
			run: func() error {
				return fillSubmissionAndReview(ctx, client, appID, includes, resp)
			},
		})
	}

	if err := runTasks(tasks, 3); err != nil {
		return nil, err
	}
	resp.Summary = buildStatusSummary(resp)

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

func fillBuildsAndTestFlight(ctx context.Context, client *asc.Client, appID string, includes includeSet, resp *dashboardResponse) error {
	buildsResp, err := client.GetBuilds(ctx, appID, asc.WithBuildsSort("-uploadedDate"), asc.WithBuildsLimit(50))
	if err != nil {
		return err
	}

	var latest *asc.Resource[asc.BuildAttributes]
	if len(buildsResp.Data) > 0 {
		latest = &buildsResp.Data[0]
	}

	if includes.builds {
		section := &buildsSection{}
		if latest != nil {
			entry := &latestBuild{
				ID:              latest.ID,
				BuildNumber:     latest.Attributes.Version,
				ProcessingState: latest.Attributes.ProcessingState,
				UploadedDate:    latest.Attributes.UploadedDate,
			}

			preRelease, preErr := client.GetBuildPreReleaseVersion(ctx, latest.ID)
			if preErr != nil {
				if !asc.IsNotFound(preErr) {
					return preErr
				}
			} else {
				entry.Version = preRelease.Data.Attributes.Version
				entry.Platform = string(preRelease.Data.Attributes.Platform)
			}
			section.Latest = entry
		}
		resp.Builds = section
	}

	if !includes.testflight {
		return nil
	}

	section := &testFlightSection{}
	if len(buildsResp.Data) == 0 {
		resp.TestFlight = section
		return nil
	}

	buildIDs := make([]string, 0, len(buildsResp.Data))
	for _, build := range buildsResp.Data {
		buildIDs = append(buildIDs, build.ID)
	}

	betaDetails, err := client.GetBuildBetaDetails(ctx,
		asc.WithBuildBetaDetailsBuildIDs(buildIDs),
		asc.WithBuildBetaDetailsLimit(200),
	)
	if err != nil {
		return err
	}
	externalStateByBuild := buildExternalStatesByBuildID(buildIDs, betaDetails)

	for _, build := range buildsResp.Data {
		state := strings.ToUpper(strings.TrimSpace(externalStateByBuild[build.ID]))
		if isDistributedState(state) {
			section.LatestDistributedBuildID = build.ID
			section.ExternalBuildState = state
			break
		}
	}

	reviewSubmissions, err := client.GetBetaAppReviewSubmissions(ctx,
		asc.WithBetaAppReviewSubmissionsBuildIDs(buildIDs),
		asc.WithBetaAppReviewSubmissionsLimit(200),
	)
	if err != nil {
		return err
	}
	latestReviewSubmission := selectLatestBetaReviewSubmission(reviewSubmissions.Data)
	if latestReviewSubmission != nil {
		section.BetaReviewState = latestReviewSubmission.Attributes.BetaReviewState
		section.SubmittedDate = latestReviewSubmission.Attributes.SubmittedDate
	}

	resp.TestFlight = section
	return nil
}

func buildExternalStatesByBuildID(buildIDs []string, betaDetails *asc.BuildBetaDetailsResponse) map[string]string {
	// BuildBetaDetails can omit relationships.build in some real API responses.
	// Use relationship mapping when available, otherwise fall back to positional mapping.
	externalStateByBuild := make(map[string]string, len(buildIDs))
	if betaDetails != nil {
		usedRelationshipMapping := false
		for _, detail := range betaDetails.Data {
			buildID, ok := optionalRelationshipResourceID(detail.Relationships, "build")
			if !ok {
				continue
			}
			usedRelationshipMapping = true
			externalStateByBuild[buildID] = strings.TrimSpace(detail.Attributes.ExternalBuildState)
		}

		// Without relationships, mapping by position is ambiguous for multiple
		// builds because the API does not guarantee response order for filters.
		// Keep a single-item fallback where positional mapping is unambiguous.
		if !usedRelationshipMapping && len(buildIDs) == 1 && len(betaDetails.Data) == 1 {
			externalStateByBuild[buildIDs[0]] = strings.TrimSpace(betaDetails.Data[0].Attributes.ExternalBuildState)
		}
	}

	return externalStateByBuild
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

func fillAppStoreAndPhasedRelease(ctx context.Context, client *asc.Client, appID string, includes includeSet, resp *dashboardResponse) error {
	versions, err := client.GetAppStoreVersions(ctx, appID, asc.WithAppStoreVersionsLimit(200))
	if err != nil {
		return err
	}

	latestVersion := selectLatestAppStoreVersion(versions.Data)
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
		} else {
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

func fillSubmissionAndReview(ctx context.Context, client *asc.Client, appID string, includes includeSet, resp *dashboardResponse) error {
	submissions, err := client.GetReviewSubmissions(ctx, appID, asc.WithReviewSubmissionsLimit(200))
	if err != nil {
		return err
	}

	if includes.submission {
		section := &submissionSection{
			InFlight:       false,
			BlockingIssues: []string{},
		}
		for _, submission := range submissions.Data {
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
		latest := selectLatestReviewSubmission(submissions.Data)
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

func selectLatestAppStoreVersion(versions []asc.Resource[asc.AppStoreVersionAttributes]) *asc.Resource[asc.AppStoreVersionAttributes] {
	if len(versions) == 0 {
		return nil
	}

	best := versions[0]
	for _, current := range versions[1:] {
		dateOrder := compareRFC3339DateStrings(current.Attributes.CreatedDate, best.Attributes.CreatedDate)
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
		dateOrder := compareRFC3339DateStrings(current.Attributes.SubmittedDate, best.Attributes.SubmittedDate)
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

func selectLatestBetaReviewSubmission(submissions []asc.Resource[asc.BetaAppReviewSubmissionAttributes]) *asc.Resource[asc.BetaAppReviewSubmissionAttributes] {
	if len(submissions) == 0 {
		return nil
	}

	best := submissions[0]
	for _, current := range submissions[1:] {
		dateOrder := compareRFC3339DateStrings(current.Attributes.SubmittedDate, best.Attributes.SubmittedDate)
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

func compareRFC3339DateStrings(current, best string) int {
	currentTime, currentValid := parseRFC3339Date(current)
	bestTime, bestValid := parseRFC3339Date(best)

	switch {
	case currentValid && bestValid:
		if currentTime.After(bestTime) {
			return 1
		}
		if currentTime.Before(bestTime) {
			return -1
		}
		return 0
	case currentValid:
		return 1
	case bestValid:
		return -1
	default:
		current = strings.TrimSpace(current)
		best = strings.TrimSpace(best)
		if current > best {
			return 1
		}
		if current < best {
			return -1
		}
		return 0
	}
}

func parseRFC3339Date(value string) (time.Time, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, false
	}
	if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return parsed, true
	}
	if parsed, err := time.Parse(time.RFC3339Nano, trimmed); err == nil {
		return parsed, true
	}
	return time.Time{}, false
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

	return "green"
}

func resolveNextAction(resp *dashboardResponse, blockers []string) string {
	if len(blockers) > 0 {
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
	if resp.AppStore != nil {
		state := strings.ToUpper(strings.TrimSpace(resp.AppStore.State))
		switch state {
		case "PREPARE_FOR_SUBMISSION":
			return "Prepare metadata and submit for review."
		case "READY_FOR_SALE":
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

	shared.RenderSection("Summary", []string{"field", "value"}, [][]string{
		{"health", fmt.Sprintf("%s %s", healthSymbol(summary.Health), shared.OrNA(summary.Health))},
		{"nextAction", shared.OrNA(summary.NextAction)},
		{"blockerCount", fmt.Sprintf("%d", len(summary.Blockers))},
	}, markdown)

	if len(summary.Blockers) > 0 {
		attentionRows := make([][]string, 0, len(summary.Blockers))
		for i, blocker := range summary.Blockers {
			attentionRows = append(attentionRows, []string{fmt.Sprintf("[x] blocker_%d", i+1), blocker})
		}
		shared.RenderSection("Needs Attention", []string{"item", "detail"}, attentionRows, markdown)
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
			rows = append(rows,
				[]string{"latest.id", resp.Builds.Latest.ID},
				[]string{"latest.version", shared.OrNA(resp.Builds.Latest.Version)},
				[]string{"latest.buildNumber", shared.OrNA(resp.Builds.Latest.BuildNumber)},
				[]string{"latest.processingState", prefixedState(resp.Builds.Latest.ProcessingState)},
				[]string{"latest.uploadedDate", formatDateWithRelative(resp.Builds.Latest.UploadedDate)},
				[]string{"latest.platform", shared.OrNA(resp.Builds.Latest.Platform)},
			)
		}
		shared.RenderSection("Builds", []string{"field", "value"}, rows, markdown)
	}

	if resp.TestFlight != nil {
		shared.RenderSection("TestFlight", []string{"field", "value"}, [][]string{
			{"latestDistributedBuildId", shared.OrNA(resp.TestFlight.LatestDistributedBuildID)},
			{"betaReviewState", prefixedState(resp.TestFlight.BetaReviewState)},
			{"externalBuildState", prefixedState(resp.TestFlight.ExternalBuildState)},
			{"submittedDate", formatDateWithRelative(resp.TestFlight.SubmittedDate)},
		}, markdown)
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
	if parsed, ok := parseRFC3339Date(value); ok {
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
