package metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestMetadataApproveWritesToRequestedReviewDir(t *testing.T) {
	reviewDir := filepath.Join(t.TempDir(), "relative-review")
	if err := os.MkdirAll(reviewDir, 0o755); err != nil {
		t.Fatalf("mkdir review dir: %v", err)
	}
	plan := metadataReviewTestPlan(t, reviewDir)
	plan.PlanPath = filepath.Join("relative-review", metadataPlanFileName)
	plan.ApprovalPath = filepath.Join("relative-review", metadataApprovalFileName)
	if err := writeMetadataReviewJSON(filepath.Join(reviewDir, metadataPlanFileName), plan); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	otherCWD := t.TempDir()
	if err := os.Mkdir(filepath.Join(otherCWD, "relative-review"), 0o755); err != nil {
		t.Fatalf("mkdir stale relative review dir: %v", err)
	}
	t.Chdir(otherCWD)

	approval, err := ExecuteMetadataApprove(MetadataApproveOptions{
		ReviewDir: reviewDir,
		All:       true,
	})
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if approval.ApprovalPath != filepath.Join(reviewDir, metadataApprovalFileName) {
		t.Fatalf("expected approval path under requested review dir, got %q", approval.ApprovalPath)
	}
	if _, err := os.Stat(filepath.Join(reviewDir, metadataApprovalFileName)); err != nil {
		t.Fatalf("expected approval in requested review dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(otherCWD, "relative-review", metadataApprovalFileName)); !os.IsNotExist(err) {
		t.Fatalf("expected no approval in stale cwd-relative dir, err=%v", err)
	}
}

func TestReadMetadataPlanArtifactRejectsMismatchedHash(t *testing.T) {
	reviewDir := t.TempDir()
	plan := metadataReviewTestPlan(t, reviewDir)
	plan.Plan.Updates[0].To = "Edited after review"
	planPath := filepath.Join(reviewDir, metadataPlanFileName)
	if err := writeMetadataReviewJSON(planPath, plan); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	_, err := readMetadataPlanArtifact(planPath)
	if err == nil {
		t.Fatal("expected mismatched hash error")
	}
	if !strings.Contains(err.Error(), "planHash does not match") {
		t.Fatalf("expected planHash mismatch error, got %v", err)
	}
}

func TestMetadataApproveMergesExistingApprovalForSamePlan(t *testing.T) {
	reviewDir := t.TempDir()
	plan := metadataReviewTestPlan(t, reviewDir)
	plan.Plan.Updates = append(plan.Plan.Updates, PlanItem{
		Key:    "app-info:en-US:name",
		Scope:  appInfoDirName,
		Locale: "en-US",
		Field:  "name",
		Reason: "field differs",
		From:   "Outslept",
		To:     "Outslept: Sleep Scores",
	})
	updateMetadataReviewTestPlanHash(t, &plan)
	if err := writeMetadataReviewJSON(filepath.Join(reviewDir, metadataPlanFileName), plan); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	first, err := ExecuteMetadataApprove(MetadataApproveOptions{
		ReviewDir: reviewDir,
		Key:       "app-info:en-US:subtitle",
	})
	if err != nil {
		t.Fatalf("approve first key: %v", err)
	}
	if len(first.ApprovedKeys) != 1 || first.ApprovedKeys[0] != "app-info:en-US:subtitle" {
		t.Fatalf("unexpected first approval: %+v", first.ApprovedKeys)
	}

	second, err := ExecuteMetadataApprove(MetadataApproveOptions{
		ReviewDir: reviewDir,
		Key:       "app-info:en-US:name",
	})
	if err != nil {
		t.Fatalf("approve second key: %v", err)
	}
	want := []string{"app-info:en-US:name", "app-info:en-US:subtitle"}
	if strings.Join(second.ApprovedKeys, ",") != strings.Join(want, ",") {
		t.Fatalf("expected merged approvals %v, got %v", want, second.ApprovedKeys)
	}
	status, err := ExecuteMetadataReviewStatus(reviewDir)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !status.Ready || status.PendingCount != 0 || status.ApprovedCount != 2 {
		t.Fatalf("expected merged approvals to be ready, got %+v", status)
	}
}

func TestVerifyApprovedMetadataPlanReportsDriftBeforeMissingApproval(t *testing.T) {
	reviewDir := t.TempDir()
	plan := metadataReviewTestPlan(t, reviewDir)
	if err := writeMetadataReviewJSON(filepath.Join(reviewDir, metadataPlanFileName), plan); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	currentPlan := plan.Plan
	currentPlan.Updates[0].To = "Drifted after planning"
	err := VerifyApprovedMetadataPlan(metadataReviewTestPushOptions(plan.Plan), currentPlan, reviewDir)
	if err == nil {
		t.Fatal("expected drift error")
	}
	if !strings.Contains(err.Error(), "approved metadata plan drifted") {
		t.Fatalf("expected drift error, got %v", err)
	}
}

func TestVerifyApprovedMetadataPlanAllowsEmptyPlanWithStaleApproval(t *testing.T) {
	reviewDir := t.TempDir()
	plan := metadataReviewTestPlan(t, reviewDir)
	plan.Plan.Updates = nil
	updateMetadataReviewTestPlanHash(t, &plan)
	if err := writeMetadataReviewJSON(filepath.Join(reviewDir, metadataPlanFileName), plan); err != nil {
		t.Fatalf("write plan: %v", err)
	}
	staleApproval := MetadataApprovalArtifact{
		SchemaVersion: metadataReviewSchemaV1,
		ApprovedAt:    time.Now().UTC().Format(time.RFC3339),
		PlanHash:      "stale-plan-hash",
		ReviewDir:     reviewDir,
		PlanPath:      filepath.Join(reviewDir, metadataPlanFileName),
		ApprovalPath:  filepath.Join(reviewDir, metadataApprovalFileName),
		Mode:          metadataApprovalAllOption,
	}
	if err := writeMetadataReviewJSON(filepath.Join(reviewDir, metadataApprovalFileName), staleApproval); err != nil {
		t.Fatalf("write stale approval: %v", err)
	}

	if err := VerifyApprovedMetadataPlan(metadataReviewTestPushOptions(plan.Plan), plan.Plan, reviewDir); err != nil {
		t.Fatalf("expected empty plan with stale approval to be ready: %v", err)
	}
	status, err := ExecuteMetadataReviewStatus(reviewDir)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !status.Ready || status.TotalCount != 0 || status.PendingCount != 0 {
		t.Fatalf("expected empty plan status to be ready despite stale approval, got %+v", status)
	}
}

func metadataReviewTestPlan(t *testing.T, reviewDir string) MetadataPlanArtifact {
	t.Helper()
	result := PushPlanResult{
		AppID:     "app-1",
		AppInfoID: "appinfo-1",
		Version:   "1.2.3",
		VersionID: "version-1",
		Dir:       "./metadata",
		DryRun:    true,
		Includes:  []string{includeLocalizations},
		Updates: []PlanItem{{
			Key:    "app-info:en-US:subtitle",
			Scope:  appInfoDirName,
			Locale: "en-US",
			Field:  "subtitle",
			Reason: "field differs",
			From:   "Sleep tracker",
			To:     "Sleep scores",
		}},
	}
	options := metadataPlanOptionsFromPush(PushExecutionOptions{
		AppID:        result.AppID,
		Version:      result.Version,
		Dir:          result.Dir,
		Include:      includeLocalizations,
		DryRun:       true,
		AllowDeletes: false,
	}, result)
	planHash, err := hashMetadataPlan(options, result)
	if err != nil {
		t.Fatalf("hash plan: %v", err)
	}
	return MetadataPlanArtifact{
		SchemaVersion: metadataReviewSchemaV1,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Command:       "metadata plan",
		ReviewDir:     reviewDir,
		PlanPath:      filepath.Join(reviewDir, metadataPlanFileName),
		ApprovalPath:  filepath.Join(reviewDir, metadataApprovalFileName),
		PlanHash:      planHash,
		Options:       options,
		Plan:          result,
	}
}

func updateMetadataReviewTestPlanHash(t *testing.T, plan *MetadataPlanArtifact) {
	t.Helper()
	options := metadataPlanOptionsFromPush(metadataReviewTestPushOptions(plan.Plan), plan.Plan)
	planHash, err := hashMetadataPlan(options, plan.Plan)
	if err != nil {
		t.Fatalf("hash plan: %v", err)
	}
	plan.Options = options
	plan.PlanHash = planHash
}

func metadataReviewTestPushOptions(plan PushPlanResult) PushExecutionOptions {
	return PushExecutionOptions{
		AppID:        plan.AppID,
		AppInfoID:    plan.AppInfoID,
		Version:      plan.Version,
		Dir:          plan.Dir,
		Include:      strings.Join(plan.Includes, ","),
		DryRun:       true,
		AllowDeletes: false,
	}
}

func TestVerifyApprovedMetadataPlanRejectsChangedIfExistsMode(t *testing.T) {
	reviewDir := t.TempDir()
	plan := metadataReviewTestPlan(t, reviewDir)
	if err := writeMetadataReviewJSON(filepath.Join(reviewDir, metadataPlanFileName), plan); err != nil {
		t.Fatalf("write plan: %v", err)
	}
	if plan.Options.IfExists != "" {
		t.Fatalf("plan options ifExists = %q, want the default fail to be omitted", plan.Options.IfExists)
	}

	opts := metadataReviewTestPushOptions(plan.Plan)
	opts.IfExists = string(shared.IfExistsUpdate)
	err := VerifyApprovedMetadataPlan(opts, plan.Plan, reviewDir)
	if err == nil {
		t.Fatal("expected applying an approved plan with a different --if-exists mode to be rejected")
	}
	if !strings.Contains(err.Error(), "approved metadata plan drifted") {
		t.Fatalf("expected drift error, got %v", err)
	}
}

func TestMetadataPlanOptionsIfExistsFailMatchesUnsetHash(t *testing.T) {
	result := PushPlanResult{AppID: "app-1", Version: "1.2.3", Dir: "./metadata", DryRun: true}
	unset := metadataPlanOptionsFromPush(PushExecutionOptions{AppID: "app-1", Version: "1.2.3", Dir: "./metadata"}, result)
	explicitFail := metadataPlanOptionsFromPush(PushExecutionOptions{AppID: "app-1", Version: "1.2.3", Dir: "./metadata", IfExists: string(shared.IfExistsFail)}, result)

	unsetHash, err := hashMetadataPlan(unset, result)
	if err != nil {
		t.Fatalf("hash unset: %v", err)
	}
	failHash, err := hashMetadataPlan(explicitFail, result)
	if err != nil {
		t.Fatalf("hash explicit fail: %v", err)
	}
	if unsetHash != failHash {
		t.Fatalf("explicit --if-exists fail must hash like the unset default: %s != %s", unsetHash, failHash)
	}

	skip := metadataPlanOptionsFromPush(PushExecutionOptions{AppID: "app-1", Version: "1.2.3", Dir: "./metadata", IfExists: string(shared.IfExistsSkip)}, result)
	skipHash, err := hashMetadataPlan(skip, result)
	if err != nil {
		t.Fatalf("hash skip: %v", err)
	}
	if skipHash == failHash {
		t.Fatal("--if-exists skip must change the plan hash")
	}
}
