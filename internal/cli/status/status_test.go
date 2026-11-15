package status

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestBuildDashboardSnapshotSignatureTreatsNilAndEmptySlicesEqually(t *testing.T) {
	first := &dashboardResponse{
		Summary: statusSummary{
			Health:     "green",
			NextAction: "No action needed.",
			Blockers:   nil,
		},
		Submission: &submissionSection{
			InFlight:       false,
			BlockingIssues: nil,
		},
	}
	second := &dashboardResponse{
		Summary: statusSummary{
			Health:     "green",
			NextAction: "No action needed.",
			Blockers:   []string{},
		},
		Submission: &submissionSection{
			InFlight:       false,
			BlockingIssues: []string{},
		},
	}

	firstSig, err := buildDashboardSnapshotSignature(first)
	if err != nil {
		t.Fatalf("buildDashboardSnapshotSignature(first) error: %v", err)
	}
	secondSig, err := buildDashboardSnapshotSignature(second)
	if err != nil {
		t.Fatalf("buildDashboardSnapshotSignature(second) error: %v", err)
	}

	if firstSig != secondSig {
		t.Fatalf("expected semantically identical snapshots to match, got %q != %q", firstSig, secondSig)
	}
}

func TestBuildDashboardSnapshotSignatureChangesWhenVisibleDataChanges(t *testing.T) {
	first := &dashboardResponse{
		Review: &reviewSection{
			State: "WAITING_FOR_REVIEW",
		},
	}
	second := &dashboardResponse{
		Review: &reviewSection{
			State: "IN_REVIEW",
		},
	}

	firstSig, err := buildDashboardSnapshotSignature(first)
	if err != nil {
		t.Fatalf("buildDashboardSnapshotSignature(first) error: %v", err)
	}
	secondSig, err := buildDashboardSnapshotSignature(second)
	if err != nil {
		t.Fatalf("buildDashboardSnapshotSignature(second) error: %v", err)
	}

	if firstSig == secondSig {
		t.Fatalf("expected differing visible review state to change snapshot signature, got %q", firstSig)
	}
}

func TestPrintWatchSnapshot_EmptyFormatUsesSharedDefaultOutput(t *testing.T) {
	shared.ResetDefaultOutputFormat()
	t.Setenv("ASC_DEFAULT_OUTPUT", "table")
	t.Cleanup(shared.ResetDefaultOutputFormat)

	resp := &dashboardResponse{
		Summary: statusSummary{
			Health:     "green",
			NextAction: "No action needed.",
		},
	}

	d, stdoutBuf, stderrBuf := newStatusTestDeps()
	if err := printWatchSnapshot(d, resp, "", false, false); err != nil {
		t.Fatalf("printWatchSnapshot() error = %v", err)
	}
	stdout, stderr := stdoutBuf.String(), stderrBuf.String()

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if strings.Contains(stdout, `"health"`) {
		t.Fatalf("expected table-style output, got JSON %q", stdout)
	}
	if !strings.Contains(stdout, "SUMMARY") {
		t.Fatalf("expected rendered table section, got %q", stdout)
	}
}

// newStatusTestDeps returns deps whose stdout and stderr are buffers and
// whose clock is the system clock.
func newStatusTestDeps() (*shared.Deps, *bytes.Buffer, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer
	return &shared.Deps{IO: &shared.IO{Out: &stdout, ErrOut: &stderr}, Now: time.Now}, &stdout, &stderr
}

func TestParseInclude_DefaultsToAllSections(t *testing.T) {
	includes, err := parseInclude("")
	if err != nil {
		t.Fatalf("parseInclude error: %v", err)
	}

	if !includes.app || !includes.builds || !includes.testflight || !includes.appstore || !includes.submission || !includes.review || !includes.phasedRelease || !includes.links {
		t.Fatalf("expected all sections enabled by default, got %+v", includes)
	}
}

func TestParseInclude_RejectsUnknownSection(t *testing.T) {
	_, err := parseInclude("builds,unknown")
	if err == nil {
		t.Fatal("expected error for unknown include section")
	}
}

func TestParseInclude_AppOnly(t *testing.T) {
	includes, err := parseInclude("app")
	if err != nil {
		t.Fatalf("parseInclude error: %v", err)
	}
	if !includes.app {
		t.Fatal("expected app include enabled")
	}
	if includes.builds || includes.testflight || includes.appstore || includes.submission || includes.review || includes.phasedRelease || includes.links {
		t.Fatalf("expected only app include enabled, got %+v", includes)
	}
}

func TestSelectLatestAppStoreVersion_DeterministicTieBreak(t *testing.T) {
	versions := []asc.Resource[asc.AppStoreVersionAttributes]{
		{
			ID: "ver-1",
			Attributes: asc.AppStoreVersionAttributes{
				CreatedDate: "2026-02-20T00:00:00Z",
			},
		},
		{
			ID: "ver-2",
			Attributes: asc.AppStoreVersionAttributes{
				CreatedDate: "2026-02-20T00:00:00Z",
			},
		},
	}

	selected := selectLatestAppStoreVersion(versions)
	if selected == nil {
		t.Fatal("expected selected version, got nil")
		return
	}
	if selected.ID != "ver-2" {
		t.Fatalf("expected deterministic tie-break to choose ver-2, got %q", selected.ID)
	}
}

func TestSelectLatestAppStoreVersion_ParsesRFC3339Offsets(t *testing.T) {
	versions := []asc.Resource[asc.AppStoreVersionAttributes]{
		{
			ID: "ver-older",
			Attributes: asc.AppStoreVersionAttributes{
				CreatedDate: "2026-02-20T01:00:00+01:00",
			},
		},
		{
			ID: "ver-newer",
			Attributes: asc.AppStoreVersionAttributes{
				CreatedDate: "2026-02-20T00:30:00Z",
			},
		},
	}

	selected := selectLatestAppStoreVersion(versions)
	if selected == nil {
		t.Fatal("expected selected version, got nil")
		return
	}
	if selected.ID != "ver-newer" {
		t.Fatalf("expected ver-newer to be selected, got %q", selected.ID)
	}
}

func TestSelectLatestReviewSubmission_DeterministicTieBreak(t *testing.T) {
	submissions := []asc.ReviewSubmissionResource{
		{
			ID: "sub-1",
			Attributes: asc.ReviewSubmissionAttributes{
				SubmittedDate: "2026-02-20T00:00:00Z",
			},
		},
		{
			ID: "sub-2",
			Attributes: asc.ReviewSubmissionAttributes{
				SubmittedDate: "2026-02-20T00:00:00Z",
			},
		},
	}

	selected := selectLatestReviewSubmission(submissions)
	if selected == nil {
		t.Fatal("expected selected submission, got nil")
		return
	}
	if selected.ID != "sub-2" {
		t.Fatalf("expected deterministic tie-break to choose sub-2, got %q", selected.ID)
	}
}

func TestSelectLatestReviewSubmission_ParsesRFC3339Offsets(t *testing.T) {
	submissions := []asc.ReviewSubmissionResource{
		{
			ID: "sub-older",
			Attributes: asc.ReviewSubmissionAttributes{
				SubmittedDate: "2026-02-20T01:00:00+01:00",
			},
		},
		{
			ID: "sub-newer",
			Attributes: asc.ReviewSubmissionAttributes{
				SubmittedDate: "2026-02-20T00:30:00Z",
			},
		},
	}

	selected := selectLatestReviewSubmission(submissions)
	if selected == nil {
		t.Fatal("expected selected submission, got nil")
		return
	}
	if selected.ID != "sub-newer" {
		t.Fatalf("expected sub-newer to be selected, got %q", selected.ID)
	}
}

func TestSelectLatestReviewSubmission_PrefersActiveSubmissionWithoutSubmittedDate(t *testing.T) {
	submissions := []asc.ReviewSubmissionResource{
		{
			ID: "sub-complete",
			Attributes: asc.ReviewSubmissionAttributes{
				SubmissionState: asc.ReviewSubmissionStateComplete,
				SubmittedDate:   "2026-03-16T10:00:00Z",
			},
		},
		{
			ID: "sub-ready",
			Attributes: asc.ReviewSubmissionAttributes{
				SubmissionState: asc.ReviewSubmissionStateReadyForReview,
				SubmittedDate:   "",
			},
		},
	}

	selected := selectLatestReviewSubmission(submissions)
	if selected == nil {
		t.Fatal("expected selected submission, got nil")
		return
	}
	if selected.ID != "sub-ready" {
		t.Fatalf("expected active ready-for-review submission to win, got %q", selected.ID)
	}
}

func TestSelectLatestBetaReviewSubmission_ParsesRFC3339Offsets(t *testing.T) {
	submissions := []asc.Resource[asc.BetaAppReviewSubmissionAttributes]{
		{
			ID: "beta-sub-older",
			Attributes: asc.BetaAppReviewSubmissionAttributes{
				SubmittedDate: "2026-02-20T01:00:00+01:00",
			},
		},
		{
			ID: "beta-sub-newer",
			Attributes: asc.BetaAppReviewSubmissionAttributes{
				SubmittedDate: "2026-02-20T00:30:00Z",
			},
		},
	}

	selected := selectLatestBetaReviewSubmission(submissions)
	if selected == nil {
		t.Fatal("expected selected submission, got nil")
		return
	}
	if selected.ID != "beta-sub-newer" {
		t.Fatalf("expected beta-sub-newer to be selected, got %q", selected.ID)
	}
}

func TestSelectBetaReviewSubmissionForLatestBuild_PrefersActiveSameTrainSubmission(t *testing.T) {
	latest := &betaReviewBuildStatus{ID: "build-326", Version: "1.2.3", BuildNumber: "326", Platform: "IOS"}
	buildsByID := map[string]*betaReviewBuildStatus{
		"build-326": latest,
		"build-325": {ID: "build-325", Version: "1.2.3", BuildNumber: "325", Platform: "IOS"},
		"build-mac": {ID: "build-mac", Version: "1.2.3", BuildNumber: "900", Platform: "MAC_OS"},
	}
	submissions := []asc.Resource[asc.BetaAppReviewSubmissionAttributes]{
		betaReviewSubmissionFixture("approved-latest", "build-326", "APPROVED", "2026-08-10T04:00:00Z"),
		betaReviewSubmissionFixture("waiting-older", "build-325", "WAITING_FOR_REVIEW", "2026-08-09T04:00:00Z"),
		betaReviewSubmissionFixture("waiting-other-platform", "build-mac", "WAITING_FOR_REVIEW", "2026-08-10T05:00:00Z"),
	}

	selected := selectBetaReviewSubmissionForLatestBuild(submissions, latest, buildsByID, nil)
	if selected == nil || selected.ID != "waiting-older" {
		t.Fatalf("expected active same-train submission waiting-older, got %+v", selected)
	}
}

func TestSelectBetaReviewSubmissionForLatestBuild_PrefersRelevantTrainOverNewerOtherPlatform(t *testing.T) {
	latest := &betaReviewBuildStatus{ID: "build-ios", Version: "2.0", BuildNumber: "20", Platform: "IOS"}
	buildsByID := map[string]*betaReviewBuildStatus{
		"build-ios": latest,
		"build-mac": {ID: "build-mac", Version: "2.0", BuildNumber: "50", Platform: "MAC_OS"},
	}
	submissions := []asc.Resource[asc.BetaAppReviewSubmissionAttributes]{
		betaReviewSubmissionFixture("approved-ios", "build-ios", "APPROVED", "2026-08-09T04:00:00Z"),
		betaReviewSubmissionFixture("waiting-mac", "build-mac", "WAITING_FOR_REVIEW", "2026-08-10T05:00:00Z"),
	}

	selected := selectBetaReviewSubmissionForLatestBuild(submissions, latest, buildsByID, nil)
	if selected == nil || selected.ID != "approved-ios" {
		t.Fatalf("expected relevant iOS submission approved-ios, got %+v", selected)
	}
}

func TestSelectBetaReviewSubmissionForLatestBuild_PreservesUnknownActiveCorrelation(t *testing.T) {
	latest := &betaReviewBuildStatus{ID: "build-326", Version: "1.2.3", BuildNumber: "326", Platform: "IOS"}
	buildsByID := map[string]*betaReviewBuildStatus{"build-326": latest}
	submissions := []asc.Resource[asc.BetaAppReviewSubmissionAttributes]{
		betaReviewSubmissionFixture("approved-326", "build-326", "APPROVED", "2026-08-10T05:00:00Z"),
		{
			ID: "waiting-unknown",
			Attributes: asc.BetaAppReviewSubmissionAttributes{
				BetaReviewState: "WAITING_FOR_REVIEW",
				SubmittedDate:   "2026-08-09T03:00:00Z",
			},
		},
	}
	reviewBuildsBySubmissionID := map[string]*betaReviewBuildStatus{
		// A related-build lookup succeeded but pre-release context did not.
		"waiting-unknown": {ID: "build-325", BuildNumber: "325"},
	}

	selected := selectBetaReviewSubmissionForLatestBuild(submissions, latest, buildsByID, reviewBuildsBySubmissionID)
	if selected == nil || selected.ID != "waiting-unknown" {
		t.Fatalf("expected unresolved active review to remain visible, got %+v", selected)
	}
	relation := betaReviewBuildRelation(latest, reviewBuildsBySubmissionID[selected.ID])
	if relation != "unknown" {
		t.Fatalf("expected unknown relation without pre-release context, got %q", relation)
	}
	summary := buildStatusSummary(&dashboardResponse{TestFlight: &testFlightSection{
		BetaReviewState: selected.Attributes.BetaReviewState,
		latestBuild:     latest,
		BetaReviewSubmission: &betaReviewSubmissionStatus{
			ID: selected.ID, State: selected.Attributes.BetaReviewState, RelationToLatestBuild: relation,
			Build: reviewBuildsBySubmissionID[selected.ID],
		},
	}})
	if summary.Health != "yellow" || len(summary.Blockers) != 0 {
		t.Fatalf("expected transiently incomplete correlation to be yellow, not green/red: %+v", summary)
	}
}

func betaReviewSubmissionFixture(id, buildID, state, submittedDate string) asc.Resource[asc.BetaAppReviewSubmissionAttributes] {
	relationships, err := json.Marshal(map[string]any{
		"build": map[string]any{
			"data": map[string]string{"type": "builds", "id": buildID},
		},
	})
	if err != nil {
		panic(err)
	}
	return asc.Resource[asc.BetaAppReviewSubmissionAttributes]{
		ID:            id,
		Relationships: relationships,
		Attributes: asc.BetaAppReviewSubmissionAttributes{
			BetaReviewState: state,
			SubmittedDate:   submittedDate,
		},
	}
}

func TestBuildStatusSummary_RedWhenBlockingIssuesExist(t *testing.T) {
	resp := &dashboardResponse{
		Submission: &submissionSection{
			InFlight:       true,
			BlockingIssues: []string{"submission abc has unresolved issues"},
		},
	}

	summary := buildStatusSummary(resp)
	if summary.Health != "red" {
		t.Fatalf("expected health=red, got %q", summary.Health)
	}
	if summary.NextAction == "" {
		t.Fatal("expected next action")
	}
	if len(summary.Blockers) == 0 {
		t.Fatal("expected blockers")
	}
}

func TestBuildStatusSummary_YellowWhenReviewInFlight(t *testing.T) {
	resp := &dashboardResponse{
		Review: &reviewSection{
			State: "WAITING_FOR_REVIEW",
		},
	}

	summary := buildStatusSummary(resp)
	if summary.Health != "yellow" {
		t.Fatalf("expected health=yellow, got %q", summary.Health)
	}
}

func TestBuildStatusSummary_GreenWhenReadyForSale(t *testing.T) {
	resp := &dashboardResponse{
		AppStore: &appStoreSection{
			State: "READY_FOR_SALE",
		},
		Builds: &buildsSection{
			Latest: &latestBuild{ID: "build-1"},
		},
	}

	summary := buildStatusSummary(resp)
	if summary.Health != "green" {
		t.Fatalf("expected health=green, got %q", summary.Health)
	}
	if summary.NextAction != "No action needed." {
		t.Fatalf("expected no action needed, got %q", summary.NextAction)
	}
}

func TestBuildStatusSummary_GreenWhenReadyForDistribution(t *testing.T) {
	// appVersionState reports a live version as READY_FOR_DISTRIBUTION and
	// is preferred over the legacy appStoreState READY_FOR_SALE.
	resp := &dashboardResponse{
		AppStore: &appStoreSection{
			State: "READY_FOR_DISTRIBUTION",
		},
		Builds: &buildsSection{
			Latest: &latestBuild{ID: "build-1"},
		},
	}

	summary := buildStatusSummary(resp)
	if summary.Health != "green" {
		t.Fatalf("expected health=green, got %q", summary.Health)
	}
	if summary.NextAction != "No action needed." {
		t.Fatalf("expected no action needed, got %q", summary.NextAction)
	}
}

func TestBuildStatusSummary_BetaReviewCorrelation(t *testing.T) {
	latest := &betaReviewBuildStatus{ID: "build-326", Version: "1.2.3", BuildNumber: "326", Platform: "IOS"}
	tests := []struct {
		name                     string
		review                   *betaReviewSubmissionStatus
		latestDistributedBuildID string
		wantHealth               string
		wantBlock                bool
		wantAction               string
	}{
		{
			name: "same build waiting is in progress, not blocked",
			review: &betaReviewSubmissionStatus{
				ID: "review-326", State: "WAITING_FOR_REVIEW", RelationToLatestBuild: "sameBuild",
				Build: latest,
			},
			wantHealth: "yellow",
			wantAction: "build 326",
		},
		{
			name: "older same train waiting blocks latest",
			review: &betaReviewSubmissionStatus{
				ID: "review-325", State: "WAITING_FOR_REVIEW", RelationToLatestBuild: "sameVersionTrain",
				Build: &betaReviewBuildStatus{ID: "build-325", Version: "1.2.3", BuildNumber: "325", Platform: "IOS"},
			},
			wantHealth: "red",
			wantBlock:  true,
			wantAction: "build 325",
		},
		{
			name: "older same train waiting does not block an already distributed latest build",
			review: &betaReviewSubmissionStatus{
				ID: "review-325", State: "WAITING_FOR_REVIEW", RelationToLatestBuild: "sameVersionTrain",
				Build: &betaReviewBuildStatus{ID: "build-325", Version: "1.2.3", BuildNumber: "325", Platform: "IOS"},
			},
			latestDistributedBuildID: "build-326",
			wantHealth:               "green",
		},
		{
			name: "older same train in review blocks latest",
			review: &betaReviewSubmissionStatus{
				ID: "review-325", State: "IN_REVIEW", RelationToLatestBuild: "sameVersionTrain",
				Build: &betaReviewBuildStatus{ID: "build-325", Version: "1.2.3", BuildNumber: "325", Platform: "IOS"},
			},
			wantHealth: "red",
			wantBlock:  true,
			wantAction: "build 325",
		},
		{
			name: "older same train approved is terminal history",
			review: &betaReviewSubmissionStatus{
				ID: "review-325", State: "APPROVED", RelationToLatestBuild: "sameVersionTrain",
				Build: &betaReviewBuildStatus{ID: "build-325", Version: "1.2.3", BuildNumber: "325", Platform: "IOS"},
			},
			wantHealth: "green",
		},
		{
			name: "older same train rejected is attention, not a blocker",
			review: &betaReviewSubmissionStatus{
				ID: "review-325", State: "REJECTED", RelationToLatestBuild: "sameVersionTrain",
				Build: &betaReviewBuildStatus{ID: "build-325", Version: "1.2.3", BuildNumber: "325", Platform: "IOS"},
			},
			wantHealth: "yellow",
			wantAction: "feedback",
		},
		{
			name: "other platform waiting is unrelated",
			review: &betaReviewSubmissionStatus{
				ID: "review-mac", State: "WAITING_FOR_REVIEW", RelationToLatestBuild: "differentVersionTrain",
				Build: &betaReviewBuildStatus{ID: "build-mac", Version: "1.2.3", BuildNumber: "900", Platform: "MAC_OS"},
			},
			wantHealth: "green",
		},
		{
			name: "missing relationship is unknown, not guessed",
			review: &betaReviewSubmissionStatus{
				ID: "review-unknown", State: "WAITING_FOR_REVIEW", RelationToLatestBuild: "unknown",
			},
			wantHealth: "yellow",
			wantAction: "identify its build",
		},
		{
			name:       "no review submission",
			wantHealth: "green",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resp := &dashboardResponse{
				TestFlight: &testFlightSection{
					BetaReviewSubmission:     test.review,
					LatestDistributedBuildID: test.latestDistributedBuildID,
					latestBuild:              latest,
				},
			}
			if test.review != nil {
				resp.TestFlight.BetaReviewState = test.review.State
			}

			summary := buildStatusSummary(resp)
			if summary.Health != test.wantHealth {
				t.Fatalf("expected health=%s, got %+v", test.wantHealth, summary)
			}
			if got := len(summary.Blockers) > 0; got != test.wantBlock {
				t.Fatalf("expected blocker=%t, got %v", test.wantBlock, summary.Blockers)
			}
			if test.wantAction != "" && !strings.Contains(summary.NextAction, test.wantAction) {
				t.Fatalf("expected next action containing %q, got %q", test.wantAction, summary.NextAction)
			}
		})
	}
}

func TestBuildStatusSummary_AppStoreBlockerPrecedesBetaReviewAction(t *testing.T) {
	latest := &betaReviewBuildStatus{ID: "build-326", Version: "1.2.3", BuildNumber: "326", Platform: "IOS"}
	resp := &dashboardResponse{
		Review: &reviewSection{State: "REJECTED"},
		TestFlight: &testFlightSection{
			latestBuild: latest,
			BetaReviewSubmission: &betaReviewSubmissionStatus{
				ID: "review-325", State: "WAITING_FOR_REVIEW", RelationToLatestBuild: "sameVersionTrain",
				Build: &betaReviewBuildStatus{ID: "build-325", Version: "1.2.3", BuildNumber: "325", Platform: "IOS"},
			},
		},
	}

	summary := buildStatusSummary(resp)
	if len(summary.Blockers) != 2 {
		t.Fatalf("expected both App Store and beta review blockers, got %v", summary.Blockers)
	}
	if summary.NextAction != "Resolve blocker: App Store review is rejected" {
		t.Fatalf("expected App Store blocker precedence, got %q", summary.NextAction)
	}
}

func TestRenderDashboardLabelsBetaReviewBuildExplicitly(t *testing.T) {
	resp := &dashboardResponse{
		Summary: statusSummary{Health: "yellow", NextAction: "Wait.", Blockers: []string{}},
		TestFlight: &testFlightSection{
			BetaReviewState: "WAITING_FOR_REVIEW",
			SubmittedDate:   "2026-08-09T04:00:00Z",
			BetaReviewSubmission: &betaReviewSubmissionStatus{
				ID: "review-325", State: "WAITING_FOR_REVIEW", SubmittedDate: "2026-08-09T04:00:00Z", RelationToLatestBuild: "sameVersionTrain",
				Build: &betaReviewBuildStatus{ID: "build-325", Version: "1.2.3", BuildNumber: "325", Platform: "IOS"},
			},
		},
	}

	d, stdoutBuf, stderrBuf := newStatusTestDeps()
	renderTable(d, resp)
	stdout, stderr := stdoutBuf.String(), stderrBuf.String()
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	for _, label := range []string{
		"betaReviewSubmission.id",
		"betaReviewSubmission.relationToLatestBuild",
		"betaReviewSubmission.build.id",
		"betaReviewSubmission.build.buildNumber",
	} {
		if !strings.Contains(stdout, label) {
			t.Fatalf("expected explicit label %q in output:\n%s", label, stdout)
		}
	}
	for _, label := range []string{"betaReviewState", "submittedDate"} {
		if !strings.Contains(stdout, label) {
			t.Fatalf("expected legacy compatibility label %q in output:\n%s", label, stdout)
		}
	}
	if strings.Index(stdout, "betaReviewSubmission.build.id") > strings.Index(stdout, "betaReviewState") {
		t.Fatalf("expected explicit review build identity before legacy state alias:\n%s", stdout)
	}
}

func TestPhasedReleaseProgressBar(t *testing.T) {
	bar := phasedReleaseProgressBar(&phasedReleaseSection{
		Configured:       true,
		CurrentDayNumber: 3,
	})
	if bar == "" {
		t.Fatal("expected progress bar")
	}
	if bar != "[####------] 3/7" {
		t.Fatalf("expected deterministic bar, got %q", bar)
	}
}

func TestRenderDashboardShowsLatestBuildExpiryAndInternalState(t *testing.T) {
	boolPointer := func(value bool) *bool { return &value }
	tests := []struct {
		name    string
		expired *bool
		want    string
	}{
		{name: "expired build is called out", expired: boolPointer(true), want: "[x] true"},
		{name: "live build is called out", expired: boolPointer(false), want: "[+] false"},
		{name: "unknown expiry is not reported as false", expired: nil, want: "[-] unknown"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resp := &dashboardResponse{
				Summary: statusSummary{Health: "green", NextAction: "Review release status.", Blockers: []string{}},
				Builds: &buildsSection{
					Latest: &latestBuild{
						ID:              "build-2",
						Version:         "1.2.3",
						BuildNumber:     "45",
						ProcessingState: "VALID",
						Expired:         test.expired,
					},
				},
				TestFlight: &testFlightSection{
					InternalBuildState:       "PROCESSING",
					LatestDistributedBuildID: "build-1",
					ExternalBuildState:       "IN_BETA_TESTING",
				},
			}

			d, stdoutBuf, stderrBuf := newStatusTestDeps()
			renderTable(d, resp)
			stdout, stderr := stdoutBuf.String(), stderrBuf.String()
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if !strings.Contains(stdout, "latest.expired") || !strings.Contains(stdout, test.want) {
				t.Fatalf("expected latest.expired row rendered as %q:\n%s", test.want, stdout)
			}
			if !strings.Contains(stdout, "internalBuildState") || !strings.Contains(stdout, "[~] PROCESSING") {
				t.Fatalf("expected internalBuildState row:\n%s", stdout)
			}
		})
	}
}

func TestRenderDashboardMarksBlockingInternalBuildStatesAsFailures(t *testing.T) {
	for _, state := range []string{"PROCESSING_EXCEPTION", "MISSING_EXPORT_COMPLIANCE", "EXPIRED"} {
		t.Run(state, func(t *testing.T) {
			resp := &dashboardResponse{
				Summary:    statusSummary{Health: "yellow", NextAction: "Review release status.", Blockers: []string{}},
				TestFlight: &testFlightSection{InternalBuildState: state},
			}

			d, stdoutBuf, stderrBuf := newStatusTestDeps()
			renderTable(d, resp)
			stdout, stderr := stdoutBuf.String(), stderrBuf.String()
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if !strings.Contains(stdout, "[x] "+state) {
				t.Fatalf("expected internalBuildState %s to render as a failure:\n%s", state, stdout)
			}
		})
	}
}

func TestLatestBuildJSONOmitsUnknownExpiry(t *testing.T) {
	encoded, err := json.Marshal(latestBuild{ID: "build-1", BuildNumber: "42"})
	if err != nil {
		t.Fatalf("marshal latest build: %v", err)
	}
	if strings.Contains(string(encoded), `"expired"`) {
		t.Fatalf("unknown expiry must be omitted, got %s", encoded)
	}
}

func TestStateSymbolClassification(t *testing.T) {
	tests := []struct {
		value string
		want  string
	}{
		{value: "READY_FOR_SALE", want: "[+]"},
		{value: "IN_REVIEW", want: "[~]"},
		{value: "READY_FOR_REVIEW", want: "[~]"},
		{value: "UNRESOLVED_ISSUES", want: "[x]"},
		{value: "", want: "[-]"},
	}
	for _, test := range tests {
		if got := stateSymbol(test.value); got != test.want {
			t.Fatalf("stateSymbol(%q) = %q, want %q", test.value, got, test.want)
		}
	}
}

func TestInternalBuildStateSymbolClassification(t *testing.T) {
	tests := []struct {
		value string
		want  string
	}{
		{value: "PROCESSING", want: "[~]"},
		{value: "PROCESSING_EXCEPTION", want: "[x]"},
		{value: "MISSING_EXPORT_COMPLIANCE", want: "[x]"},
		{value: "READY_FOR_BETA_TESTING", want: "[+]"},
		{value: "IN_BETA_TESTING", want: "[+]"},
		{value: "EXPIRED", want: "[x]"},
		{value: "IN_EXPORT_COMPLIANCE_REVIEW", want: "[~]"},
	}
	for _, test := range tests {
		if got := internalBuildStateSymbol(test.value); got != test.want {
			t.Fatalf("internalBuildStateSymbol(%q) = %q, want %q", test.value, got, test.want)
		}
	}
}

func TestFormatDateWithRelative(t *testing.T) {
	d := &shared.Deps{Now: func() time.Time {
		return time.Date(2026, 2, 20, 12, 0, 0, 0, time.UTC)
	}}

	got := formatDateWithRelative(d, "2026-02-19T12:00:00Z")
	if got != "2026-02-19T12:00:00Z (1d ago)" {
		t.Fatalf("unexpected relative time output %q", got)
	}
}
