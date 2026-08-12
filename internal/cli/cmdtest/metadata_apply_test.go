package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	metadatacli "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/metadata"
)

func TestMetadataApplyValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "metadata apply missing dir",
			args:    []string{"metadata", "apply", "--app", "123456789", "--version", "1.2.3"},
			wantErr: "--dir is required",
		},
		{
			name:    "metadata apply positional args rejected",
			args:    []string{"metadata", "apply", "--app", "123456789", "--version", "1.2.3", "--dir", "./metadata", "extra"},
			wantErr: "metadata apply does not accept positional arguments",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestMetadataApplyReviewDirRequiresConfirmBeforeSideEffects(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	reviewDir := filepath.Join(t.TempDir(), "review")
	stdout, stderr, runErr := runMetadataReviewCommandRaw(t, []string{
		"metadata", "apply",
		"--app", "app-1",
		"--version", "1.2.3",
		"--dir", dir,
		"--review-dir", reviewDir,
		"--output", "json",
	})
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %T: %v", runErr, runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--confirm is required when applying an approved metadata plan") {
		t.Fatalf("expected confirm error, got %q", stderr)
	}
}

func TestMetadataPlanApproveStatusAndApplyReviewDir(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "en-US.json"), []byte(`{"name":"Outslept","subtitle":"Sleep debt recovery"}`), 0o644); err != nil {
		t.Fatalf("write app-info metadata: %v", err)
	}

	reviewDir := filepath.Join(t.TempDir(), "review")
	patches := 0
	appInfoLocalizationReads := 0
	versionLocalizationReads := 0
	withMetadataReviewHTTP(t, func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-1/appInfos":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`), nil
		case "/v1/apps/app-1/appStoreVersions":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`), nil
		case "/v1/appInfos/appinfo-1/appInfoLocalizations":
			appInfoLocalizationReads++
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"loc-en","attributes":{"locale":"en-US","name":"Outslept","subtitle":"Sleep tracker"}}],"links":{"next":""}}`), nil
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			versionLocalizationReads++
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
		case "/v1/appInfoLocalizations/loc-en":
			if req.Method != http.MethodPatch {
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			}
			patches++
			assertMetadataPatchPayload(t, req, "appInfoLocalizations", "loc-en", map[string]string{
				"name":     "Outslept",
				"subtitle": "Sleep debt recovery",
			})
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"appInfoLocalizations","id":"loc-en","attributes":{"locale":"en-US","name":"Outslept","subtitle":"Sleep debt recovery"}}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	planStdout := runMetadataReviewCommand(t, []string{
		"metadata", "plan",
		"--app", "app-1",
		"--version", "1.2.3",
		"--platform", "IOS",
		"--dir", dir,
		"--review-dir", reviewDir,
		"--output", "json",
		"--pretty",
	})
	var planOut struct {
		PlanHash string `json:"planHash"`
		PlanPath string `json:"planPath"`
		Plan     struct {
			Updates []struct {
				Key string `json:"key"`
			} `json:"updates"`
		} `json:"plan"`
	}
	if err := json.Unmarshal([]byte(planStdout), &planOut); err != nil {
		t.Fatalf("parse plan output: %v\n%s", err, planStdout)
	}
	if planOut.PlanHash == "" || planOut.PlanPath != filepath.Join(reviewDir, "plan.json") {
		t.Fatalf("unexpected plan artifact identity: %+v", planOut)
	}
	if len(planOut.Plan.Updates) != 1 || planOut.Plan.Updates[0].Key != "app-info:en-US:subtitle" {
		t.Fatalf("unexpected planned updates: %+v", planOut.Plan.Updates)
	}
	if _, err := os.Stat(filepath.Join(reviewDir, "plan.json")); err != nil {
		t.Fatalf("expected plan artifact: %v", err)
	}
	if patches != 0 {
		t.Fatalf("plan must not mutate, patches=%d", patches)
	}

	approveStdout := runMetadataReviewCommand(t, []string{
		"metadata", "approve",
		"--review-dir", reviewDir,
		"--all",
		"--note", "reviewed by test",
		"--output", "json",
		"--pretty",
	})
	var approvalOut struct {
		PlanHash     string   `json:"planHash"`
		ApprovedKeys []string `json:"approvedKeys"`
	}
	if err := json.Unmarshal([]byte(approveStdout), &approvalOut); err != nil {
		t.Fatalf("parse approval output: %v\n%s", err, approveStdout)
	}
	if approvalOut.PlanHash != planOut.PlanHash || len(approvalOut.ApprovedKeys) != 1 || approvalOut.ApprovedKeys[0] != "app-info:en-US:subtitle" {
		t.Fatalf("unexpected approval output: %+v", approvalOut)
	}
	if _, err := os.Stat(filepath.Join(reviewDir, "approved.json")); err != nil {
		t.Fatalf("expected approval artifact: %v", err)
	}

	statusStdout := runMetadataReviewCommand(t, []string{
		"metadata", "status",
		"--review-dir", reviewDir,
		"--output", "json",
		"--pretty",
	})
	var statusOut struct {
		Ready         bool `json:"ready"`
		ApprovedCount int  `json:"approvedCount"`
		PendingCount  int  `json:"pendingCount"`
	}
	if err := json.Unmarshal([]byte(statusStdout), &statusOut); err != nil {
		t.Fatalf("parse status output: %v\n%s", err, statusStdout)
	}
	if !statusOut.Ready || statusOut.ApprovedCount != 1 || statusOut.PendingCount != 0 {
		t.Fatalf("unexpected status output: %+v", statusOut)
	}

	appInfoLocalizationReads = 0
	versionLocalizationReads = 0
	dryRunStdout := runMetadataReviewCommand(t, []string{
		"metadata", "apply",
		"--app", "app-1",
		"--version", "1.2.3",
		"--platform", "IOS",
		"--dir", dir,
		"--review-dir", reviewDir,
		"--dry-run",
		"--confirm",
		"--output", "json",
	})
	var dryRunOut struct {
		DryRun  bool          `json:"dryRun"`
		Updates []interface{} `json:"updates"`
		Actions []interface{} `json:"actions"`
	}
	if err := json.Unmarshal([]byte(dryRunStdout), &dryRunOut); err != nil {
		t.Fatalf("parse dry-run output: %v\n%s", err, dryRunStdout)
	}
	if !dryRunOut.DryRun || len(dryRunOut.Updates) != 1 || len(dryRunOut.Actions) != 0 || patches != 0 {
		t.Fatalf("review-dir dry-run mutated or returned unexpected output: dryRun=%+v patches=%d", dryRunOut, patches)
	}
	if appInfoLocalizationReads != 1 || versionLocalizationReads != 1 {
		t.Fatalf("expected one verification read per localization endpoint on dry-run, got app-info=%d version=%d", appInfoLocalizationReads, versionLocalizationReads)
	}

	appInfoLocalizationReads = 0
	versionLocalizationReads = 0
	applyStdout := runMetadataReviewCommand(t, []string{
		"metadata", "apply",
		"--app", "app-1",
		"--version", "1.2.3",
		"--platform", "IOS",
		"--dir", dir,
		"--review-dir", reviewDir,
		"--confirm",
		"--output", "json",
	})
	var applyOut struct {
		Applied   bool `json:"applied"`
		Total     int  `json:"total"`
		Succeeded int  `json:"succeeded"`
		Failed    int  `json:"failed"`
	}
	if err := json.Unmarshal([]byte(applyStdout), &applyOut); err != nil {
		t.Fatalf("parse apply output: %v\n%s", err, applyStdout)
	}
	if !applyOut.Applied || applyOut.Total != 1 || applyOut.Succeeded != 1 || applyOut.Failed != 0 || patches != 1 {
		t.Fatalf("unexpected apply output=%+v patches=%d", applyOut, patches)
	}
	if appInfoLocalizationReads != 1 || versionLocalizationReads != 1 {
		t.Fatalf("expected guarded apply to verify and apply from one plan pass, got app-info=%d version=%d", appInfoLocalizationReads, versionLocalizationReads)
	}
}

func TestMetadataApplyReviewDirRejectsDriftBeforeMutation(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	metadataPath := filepath.Join(dir, "app-info", "en-US.json")
	if err := os.WriteFile(metadataPath, []byte(`{"name":"Outslept","subtitle":"Sleep debt recovery"}`), 0o644); err != nil {
		t.Fatalf("write app-info metadata: %v", err)
	}

	reviewDir := filepath.Join(t.TempDir(), "review")
	mutations := 0
	withMetadataReviewHTTP(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			mutations++
		}
		switch req.URL.Path {
		case "/v1/apps/app-1/appInfos":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`), nil
		case "/v1/apps/app-1/appStoreVersions":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`), nil
		case "/v1/appInfos/appinfo-1/appInfoLocalizations":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"loc-en","attributes":{"locale":"en-US","name":"Outslept","subtitle":"Sleep tracker"}}],"links":{"next":""}}`), nil
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	_ = runMetadataReviewCommand(t, []string{
		"metadata", "plan",
		"--app", "app-1",
		"--version", "1.2.3",
		"--dir", dir,
		"--review-dir", reviewDir,
		"--output", "json",
	})
	_ = runMetadataReviewCommand(t, []string{
		"metadata", "approve",
		"--review-dir", reviewDir,
		"--all",
		"--output", "json",
	})
	if err := os.WriteFile(metadataPath, []byte(`{"name":"Outslept","subtitle":"Sleep debt and bedtime"}`), 0o644); err != nil {
		t.Fatalf("write drifted app-info metadata: %v", err)
	}

	stdout, stderr, runErr := runMetadataReviewCommandRaw(t, []string{
		"metadata", "apply",
		"--app", "app-1",
		"--version", "1.2.3",
		"--dir", dir,
		"--review-dir", reviewDir,
		"--confirm",
		"--output", "json",
	})
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %T: %v", runErr, runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "approved metadata plan drifted") {
		t.Fatalf("expected drift error, got %q", stderr)
	}
	if mutations != 0 {
		t.Fatalf("expected no mutation before drift rejection, got %d", mutations)
	}
}

func TestMetadataApplyReviewDirAllowsEmptyPlanWithoutApproval(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "en-US.json"), []byte(`{"name":"Outslept","subtitle":"Sleep tracker"}`), 0o644); err != nil {
		t.Fatalf("write app-info metadata: %v", err)
	}

	reviewDir := filepath.Join(t.TempDir(), "review")
	mutations := 0
	withMetadataReviewHTTP(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			mutations++
		}
		switch req.URL.Path {
		case "/v1/apps/app-1/appInfos":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`), nil
		case "/v1/apps/app-1/appStoreVersions":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`), nil
		case "/v1/appInfos/appinfo-1/appInfoLocalizations":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"loc-en","attributes":{"locale":"en-US","name":"Outslept","subtitle":"Sleep tracker"}}],"links":{"next":""}}`), nil
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	_ = runMetadataReviewCommand(t, []string{
		"metadata", "plan",
		"--app", "app-1",
		"--version", "1.2.3",
		"--platform", "IOS",
		"--dir", dir,
		"--review-dir", reviewDir,
		"--output", "json",
	})
	statusStdout := runMetadataReviewCommand(t, []string{
		"metadata", "status",
		"--review-dir", reviewDir,
		"--output", "json",
	})
	var statusOut struct {
		Ready      bool `json:"ready"`
		TotalCount int  `json:"totalCount"`
	}
	if err := json.Unmarshal([]byte(statusStdout), &statusOut); err != nil {
		t.Fatalf("parse status output: %v\n%s", err, statusStdout)
	}
	if !statusOut.Ready || statusOut.TotalCount != 0 {
		t.Fatalf("expected empty plan to be ready, got %+v", statusOut)
	}

	applyStdout := runMetadataReviewCommand(t, []string{
		"metadata", "apply",
		"--app", "app-1",
		"--version", "1.2.3",
		"--platform", "IOS",
		"--dir", dir,
		"--review-dir", reviewDir,
		"--confirm",
		"--output", "json",
	})
	var applyOut struct {
		Applied bool `json:"applied"`
		Total   int  `json:"total"`
	}
	if err := json.Unmarshal([]byte(applyStdout), &applyOut); err != nil {
		t.Fatalf("parse apply output: %v\n%s", err, applyStdout)
	}
	if !applyOut.Applied || applyOut.Total != 0 || mutations != 0 {
		t.Fatalf("expected no-op apply without mutations, output=%+v mutations=%d", applyOut, mutations)
	}
}

func withMetadataReviewHTTP(t *testing.T, fn roundTripFunc) {
	t.Helper()
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = fn
}

func runMetadataReviewCommandRaw(t *testing.T, args []string) (stdout, stderr string, runErr error) {
	t.Helper()
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr = captureOutput(t, func() {
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	return stdout, stderr, runErr
}

func runMetadataReviewCommand(t *testing.T, args []string) string {
	t.Helper()
	stdout, stderr, runErr := runMetadataReviewCommandRaw(t, args)
	if runErr != nil {
		t.Fatalf("run %v: %T %v\nstdout=%s\nstderr=%s", args, runErr, runErr, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("unexpected stderr for %v: %q", args, stderr)
	}
	return stdout
}

func TestMetadataApplyDryRunSuggestsApplyCommandForAmbiguousAppInfos(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "en-US.json"), []byte(`{"name":"App Name"}`), 0o644); err != nil {
		t.Fatalf("write app-info file: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-1/appStoreVersions":
			body := `{
				"data":[
					{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION"}}
				],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/apps/app-1/appInfos":
			body := `{
				"data":[
					{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}},
					{"type":"appInfos","id":"appinfo-2","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}
				]
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected path: %s", req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "apply",
			"--app", "app-1",
			"--version", "1.2.3",
			"--platform", "IOS",
			"--dir", dir,
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, `Error: multiple app infos found for app "app-1"`) {
		t.Fatalf("expected ambiguous app-info error, got %q", stderr)
	}
	if !strings.Contains(stderr, `asc apps info list --app "app-1"`) {
		t.Fatalf("expected remediation to mention apps info list, got %q", stderr)
	}
	if !strings.Contains(stderr, `asc metadata apply --app "app-1" --version "1.2.3" --platform IOS --dir "`+dir+`" --app-info "appinfo-1" --dry-run`) {
		t.Fatalf("expected remediation example to use metadata apply, got %q", stderr)
	}
	if !strings.Contains(stderr, "appinfo-2") {
		t.Fatalf("expected all candidate app info ids in remediation, got %q", stderr)
	}
}

func TestMetadataApplyPreflightsAllCreatesBeforeMutation(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "en-US.json"), []byte(`{"name":"Updated name"}`), 0o644); err != nil {
		t.Fatalf("write en-US file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "fr-FR.json"), []byte(`{"subtitle":"Missing create name"}`), 0o644); err != nil {
		t.Fatalf("write fr-FR file: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	mutations := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			mutations++
		}
		switch req.URL.Path {
		case "/v1/apps/app-1/appInfos":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`), nil
		case "/v1/apps/app-1/appStoreVersions":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`), nil
		case "/v1/appInfos/appinfo-1/appInfoLocalizations":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"loc-en","attributes":{"locale":"en-US","name":"Old name"}}],"links":{"next":""}}`), nil
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "apply",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", dir,
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %v", runErr)
	}
	if mutations != 0 || stdout != "" {
		t.Fatalf("expected no mutation/output, mutations=%d stdout=%q", mutations, stdout)
	}
	if !strings.Contains(stderr, `app-info localization "fr-FR" requires name`) {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
}

func TestMetadataApplyFailsOnPartialMutation(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	workDir := t.TempDir()
	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(previousDir) })

	dir := filepath.Join(workDir, "metadata")
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "version", "1.2.3"), 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "en-US.json"), []byte(`{"name":"App Name","subtitle":"Local subtitle"}`), 0o644); err != nil {
		t.Fatalf("write app-info file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "version", "1.2.3", "fr-FR.json"), []byte(`{"description":"Local French"}`), 0o644); err != nil {
		t.Fatalf("write French version file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "version", "1.2.3", "ja.json"), []byte(`{"description":"Local Japanese"}`), 0o644); err != nil {
		t.Fatalf("write Japanese version file: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	patchCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-1/appInfos":
			body := `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/apps/app-1/appStoreVersions":
			body := `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appInfos/appinfo-1/appInfoLocalizations":
			body := `{"data":[{"type":"appInfoLocalizations","id":"loc-app-en","attributes":{"locale":"en-US","name":"App Name","subtitle":"Remote subtitle"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			body := `{"data":[
				{"type":"appStoreVersionLocalizations","id":"loc-ver-fr","attributes":{"locale":"fr-FR","description":"Remote French"}},
				{"type":"appStoreVersionLocalizations","id":"loc-ver-ja","attributes":{"locale":"ja","description":"Remote Japanese"}}
			],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appInfoLocalizations/loc-app-en":
			if req.Method == http.MethodPatch {
				patchCount++
				assertMetadataPatchPayload(t, req, "appInfoLocalizations", "loc-app-en", map[string]string{
					"name":     "App Name",
					"subtitle": "Local subtitle",
				})
				body := `{"data":{"type":"appInfoLocalizations","id":"loc-app-en","attributes":{"locale":"en-US","name":"App Name","subtitle":"Local subtitle"}}}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}
		case "/v1/appStoreVersionLocalizations/loc-ver-fr":
			if req.Method == http.MethodPatch {
				patchCount++
				assertMetadataPatchPayload(t, req, "appStoreVersionLocalizations", "loc-ver-fr", map[string]string{"description": "Local French"})
				body := `{"errors":[{"status":"500","code":"INTERNAL_ERROR","title":"Internal Error","detail":"boom"}]}`
				return &http.Response{
					StatusCode: http.StatusInternalServerError,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}
		case "/v1/appStoreVersionLocalizations/loc-ver-ja":
			if req.Method == http.MethodPatch {
				patchCount++
				assertMetadataPatchPayload(t, req, "appStoreVersionLocalizations", "loc-ver-ja", map[string]string{"description": "Local Japanese"})
				body := `{"data":{"type":"appStoreVersionLocalizations","id":"loc-ver-ja","attributes":{"locale":"ja","description":"Local Japanese"}}}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}
		}
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		return nil, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "apply",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", dir,
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected reported partial failure, got %T: %v", runErr, runErr)
	}
	if got := rootcmd.ExitCodeFromError(runErr); got != rootcmd.ExitHTTPInternalServer {
		t.Fatalf("expected partial server failure exit %d, got %d", rootcmd.ExitHTTPInternalServer, got)
	}
	if patchCount != 3 {
		t.Fatalf("expected all three patch attempts despite the middle failure, got %d", patchCount)
	}
	if !strings.Contains(runErr.Error(), "metadata apply: 1 localization(s) failed") {
		t.Fatalf("expected batch failure summary, got %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr on failure, got %q", stderr)
	}

	var result struct {
		Applied             bool   `json:"applied"`
		Total               int    `json:"total"`
		Succeeded           int    `json:"succeeded"`
		Failed              int    `json:"failed"`
		FailureArtifactPath string `json:"failureArtifactPath"`
		Actions             []struct {
			Scope         string            `json:"scope"`
			Locale        string            `json:"locale"`
			Status        string            `json:"status"`
			DesiredFields map[string]string `json:"desiredFields"`
			Error         string            `json:"error"`
		} `json:"actions"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse output: %v\nstdout=%q", err, stdout)
	}
	if result.Applied || result.Total != 3 || result.Succeeded != 2 || result.Failed != 1 {
		t.Fatalf("unexpected partial summary: %+v", result)
	}
	if len(result.Actions) != 3 || result.Actions[0].Status != "succeeded" || result.Actions[1].Status != "failed" || result.Actions[2].Status != "succeeded" {
		t.Fatalf("unexpected action sequence: %+v", result.Actions)
	}
	if result.Actions[1].Scope != "version" || result.Actions[1].Locale != "fr-FR" || result.Actions[1].DesiredFields["description"] != "Local French" {
		t.Fatalf("unexpected failed action: %+v", result.Actions[1])
	}
	if !strings.Contains(result.Actions[1].Error, "boom") {
		t.Fatalf("expected row error in structured output, got %+v", result.Actions[1])
	}

	artifactData, err := os.ReadFile(result.FailureArtifactPath)
	if err != nil {
		t.Fatalf("read failure artifact: %v", err)
	}
	var artifact struct {
		SchemaVersion int    `json:"schemaVersion"`
		Command       string `json:"command"`
		AppID         string `json:"appId"`
		AppInfoID     string `json:"appInfoId"`
		Version       string `json:"version"`
		VersionID     string `json:"versionId"`
		Dir           string `json:"dir"`
		Failures      []struct {
			Scope          string            `json:"scope"`
			Locale         string            `json:"locale"`
			Action         string            `json:"action"`
			LocalizationID string            `json:"localizationId"`
			DesiredFields  map[string]string `json:"desiredFields"`
		} `json:"failures"`
	}
	if err := json.Unmarshal(artifactData, &artifact); err != nil {
		t.Fatalf("parse failure artifact: %v", err)
	}
	if artifact.SchemaVersion != 1 || artifact.Command != "apply" || artifact.AppID != "app-1" || artifact.AppInfoID != "appinfo-1" || artifact.Version != "1.2.3" || artifact.VersionID != "version-1" || artifact.Dir != dir {
		t.Fatalf("unexpected failure artifact identity: %+v", artifact)
	}
	if len(artifact.Failures) != 1 || artifact.Failures[0].Scope != "version" || artifact.Failures[0].Locale != "fr-FR" || artifact.Failures[0].Action != "update" || artifact.Failures[0].LocalizationID != "loc-ver-fr" || artifact.Failures[0].DesiredFields["description"] != "Local French" {
		t.Fatalf("unexpected failure artifact: %+v", artifact)
	}
}

func TestMetadataApplyIgnoresRemoteOnlyEmptyLocalizationAcrossResume(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")
	t.Chdir(t.TempDir())

	metadataDir := filepath.Join(t.TempDir(), "metadata")
	versionDir := filepath.Join(metadataDir, "version", "1.2.3")
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatalf("mkdir version metadata: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "bn-BD.json"), []byte(`{"description":"Desired Bangla"}`), 0o600); err != nil {
		t.Fatalf("write version metadata: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	remoteDescription := "Old Bangla"
	patches := 0
	deletes := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-1/appInfos":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`), nil
		case "/v1/apps/app-1/appStoreVersions":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{}}`), nil
		case "/v1/appInfos/appinfo-1/appInfoLocalizations":
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{}}`), nil
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			body := fmt.Sprintf(`{"data":[{"type":"appStoreVersionLocalizations","id":"loc-bn","attributes":{"locale":"bn-BD","description":%q}},{"type":"appStoreVersionLocalizations","id":"loc-empty","attributes":{"locale":"en-US","description":"","keywords":"","marketingUrl":"","promotionalText":"","supportUrl":"","whatsNew":""}}],"links":{}}`, remoteDescription)
			return jsonHTTPResponse(http.StatusOK, body), nil
		case "/v1/appStoreVersionLocalizations/loc-bn":
			if req.Method != http.MethodPatch {
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			}
			patches++
			assertMetadataPatchPayload(t, req, "appStoreVersionLocalizations", "loc-bn", map[string]string{"description": "Desired Bangla"})
			if patches == 1 {
				return jsonHTTPResponse(http.StatusUnprocessableEntity, `{"errors":[{"status":"422","code":"ENTITY_ERROR","detail":"temporary Apple rejection"}]}`), nil
			}
			remoteDescription = "Desired Bangla"
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-bn","attributes":{"locale":"bn-BD","description":"Desired Bangla"}}}`), nil
		case "/v1/appStoreVersionLocalizations/loc-empty":
			deletes++
			t.Fatalf("empty remote-only localization must not be deleted: %s %s", req.Method, req.URL.Path)
			return nil, nil
		default:
			t.Fatalf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
			return nil, nil
		}
	})

	type result struct {
		Applied   bool              `json:"applied"`
		DryRun    bool              `json:"dryRun"`
		Updates   []json.RawMessage `json:"updates"`
		Deletes   []json.RawMessage `json:"deletes"`
		Total     int               `json:"total"`
		Succeeded int               `json:"succeeded"`
		Failed    int               `json:"failed"`
	}
	runApply := func(t *testing.T, dryRun bool) (result, error) {
		t.Helper()
		args := []string{"metadata", "apply", "--app", "app-1", "--version", "1.2.3", "--dir", metadataDir, "--output", "json"}
		if dryRun {
			args = append(args, "--dry-run")
		}
		root := RootCommand("1.2.3")
		root.FlagSet.SetOutput(io.Discard)
		var runErr error
		stdout, stderr := captureOutput(t, func() {
			if err := root.Parse(args); err != nil {
				t.Fatalf("parse: %v", err)
			}
			runErr = root.Run(context.Background())
		})
		if stderr != "" {
			t.Fatalf("unexpected stderr: %q", stderr)
		}
		var parsed result
		if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
			t.Fatalf("decode output: %v\n%s", err, stdout)
		}
		return parsed, runErr
	}

	initialPlan, err := runApply(t, true)
	if err != nil || !initialPlan.DryRun || len(initialPlan.Updates) != 1 || len(initialPlan.Deletes) != 0 {
		t.Fatalf("unexpected initial plan: result=%+v err=%v", initialPlan, err)
	}
	partial, err := runApply(t, false)
	if _, ok := errors.AsType[ReportedError](err); !ok || partial.Total != 1 || partial.Succeeded != 0 || partial.Failed != 1 {
		t.Fatalf("expected one reported mutation failure: result=%+v err=%T %v", partial, err, err)
	}
	resumePlan, err := runApply(t, true)
	if err != nil || len(resumePlan.Updates) != 1 || len(resumePlan.Deletes) != 0 {
		t.Fatalf("unexpected resume plan: result=%+v err=%v", resumePlan, err)
	}
	resumed, err := runApply(t, false)
	if err != nil || resumed.Total != 1 || resumed.Succeeded != 1 || resumed.Failed != 0 || !resumed.Applied {
		t.Fatalf("unexpected resume result: result=%+v err=%v", resumed, err)
	}
	noOp, err := runApply(t, false)
	if err != nil || noOp.Total != 0 || noOp.Failed != 0 || !noOp.Applied || len(noOp.Deletes) != 0 {
		t.Fatalf("unexpected no-op result: result=%+v err=%v", noOp, err)
	}
	if patches != 2 || deletes != 0 {
		t.Fatalf("unexpected mutations: patches=%d deletes=%d", patches, deletes)
	}
}

func assertMetadataPatchPayload(t *testing.T, req *http.Request, resourceType, id string, fields map[string]string) {
	t.Helper()
	var payload struct {
		Data struct {
			Type       string            `json:"type"`
			ID         string            `json:"id"`
			Attributes map[string]string `json:"attributes"`
		} `json:"data"`
	}
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		t.Fatalf("decode PATCH payload: %v", err)
	}
	if payload.Data.Type != resourceType || payload.Data.ID != id {
		t.Fatalf("unexpected PATCH identity: %+v", payload.Data)
	}
	if len(payload.Data.Attributes) != len(fields) {
		t.Fatalf("expected only specified fields %v, got %v", fields, payload.Data.Attributes)
	}
	for field, value := range fields {
		if payload.Data.Attributes[field] != value {
			t.Fatalf("unexpected PATCH field %s: got %q want %q", field, payload.Data.Attributes[field], value)
		}
	}
}

func TestMetadataApplyReportsFailureArtifactWriteErrorAfterSummary(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_MAX_RETRIES", "0")

	workDir := t.TempDir()
	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(previousDir) })
	if err := os.WriteFile(filepath.Join(workDir, ".asc"), []byte("blocks report directory"), 0o644); err != nil {
		t.Fatalf("write blocking .asc file: %v", err)
	}

	dir := filepath.Join(workDir, "metadata")
	if err := os.MkdirAll(filepath.Join(dir, "version", "1.2.3"), 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "version", "1.2.3", "en-US.json"), []byte(`{"description":"Local description"}`), 0o644); err != nil {
		t.Fatalf("write version file: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-1/appInfos":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`), nil
		case "/v1/apps/app-1/appStoreVersions":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`), nil
		case "/v1/appInfos/appinfo-1/appInfoLocalizations":
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Remote description"}}],"links":{"next":""}}`), nil
		case "/v1/appStoreVersionLocalizations/loc-en":
			return jsonHTTPResponse(http.StatusUnprocessableEntity, `{"errors":[{"status":"422","code":"ENTITY_ERROR.ATTRIBUTE.INVALID","detail":"invalid description"}]}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "apply",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", dir,
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected reported error, got %T: %v", runErr, runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var result struct {
		Failed               int    `json:"failed"`
		FailureArtifactPath  string `json:"failureArtifactPath"`
		FailureArtifactError string `json:"failureArtifactError"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse output: %v\nstdout=%q", err, stdout)
	}
	if result.Failed != 1 || result.FailureArtifactPath != "" || !strings.Contains(result.FailureArtifactError, "not a directory") {
		t.Fatalf("unexpected artifact failure summary: %+v", result)
	}
	if !strings.Contains(runErr.Error(), "write failure artifact") {
		t.Fatalf("expected artifact failure in reported error, got %v", runErr)
	}
}

func TestMetadataApplyFailedCreateDoesNotPrintAppliedWarning(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_MAX_RETRIES", "0")

	workDir := t.TempDir()
	t.Chdir(workDir)
	dir := filepath.Join(workDir, "metadata")
	if err := os.MkdirAll(filepath.Join(dir, "version", "1.2.3"), 0o755); err != nil {
		t.Fatalf("mkdir version: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "version", "1.2.3", "en-US.json"), []byte(`{"description":"Incomplete create","whatsNew":"Notes"}`), 0o644); err != nil {
		t.Fatalf("write version: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	posts := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-1/appInfos":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`), nil
		case "/v1/apps/app-1/appStoreVersions":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`), nil
		case "/v1/appInfos/appinfo-1/appInfoLocalizations", "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
		case "/v1/appStoreVersionLocalizations":
			posts++
			return jsonHTTPResponse(http.StatusUnprocessableEntity, `{"errors":[{"status":"422","code":"ENTITY_ERROR.ATTRIBUTE.INVALID","detail":"create rejected"}]}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"metadata", "apply", "--app", "app-1", "--version", "1.2.3", "--dir", dir, "--output", "json"}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected reported partial failure, got %T: %v", runErr, runErr)
	}
	if posts != 1 || stderr != "" || strings.Contains(stderr, "Warning:") {
		t.Fatalf("failed create must not print applied warning: posts=%d stderr=%q", posts, stderr)
	}
	var result struct {
		Failed              int    `json:"failed"`
		FailureArtifactPath string `json:"failureArtifactPath"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse output: %v", err)
	}
	if result.Failed != 1 || result.FailureArtifactPath == "" {
		t.Fatalf("unexpected failed create summary: %+v", result)
	}
	payload, err := os.ReadFile(result.FailureArtifactPath)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	if !strings.Contains(string(payload), `"action":"create"`) || !strings.Contains(string(payload), `"locale":"en-US"`) || !strings.Contains(string(payload), `"description":"Incomplete create"`) {
		t.Fatalf("failed create artifact is not replayable: %s", payload)
	}
}

func TestMetadataApplyReconcilesRequestTimeoutWithFreshReadback(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_TIMEOUT", "20ms")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "version", "1.2.3"), 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "version", "1.2.3", "en-US.json"), []byte(`{"description":"Local description"}`), 0o644); err != nil {
		t.Fatalf("write version file: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	versionReads := 0
	patchCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-1/appInfos":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`), nil
		case "/v1/apps/app-1/appStoreVersions":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`), nil
		case "/v1/appInfos/appinfo-1/appInfoLocalizations":
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			versionReads++
			description := "Remote description"
			if versionReads > 1 {
				description = "Local description"
			}
			body := `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-ver-en","attributes":{"locale":"en-US","description":"` + description + `"}}],"links":{"next":""}}`
			return jsonHTTPResponse(http.StatusOK, body), nil
		case "/v1/appStoreVersionLocalizations/loc-ver-en":
			if req.Method != http.MethodPatch {
				t.Fatalf("expected PATCH, got %s", req.Method)
			}
			patchCount++
			<-req.Context().Done()
			time.Sleep(5 * time.Millisecond)
			return nil, req.Context().Err()
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "apply",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", dir,
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	var result struct {
		Applied bool `json:"applied"`
		Actions []struct {
			Scope  string `json:"scope"`
			Locale string `json:"locale"`
			Action string `json:"action"`
		} `json:"actions"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse output: %v", err)
	}
	if !result.Applied || patchCount != 1 || versionReads != 2 {
		t.Fatalf("unexpected recovery: result=%+v patches=%d reads=%d", result, patchCount, versionReads)
	}
	if len(result.Actions) != 1 || result.Actions[0].Scope != "version" || result.Actions[0].Locale != "en-US" || result.Actions[0].Action != "reconcile" {
		t.Fatalf("unexpected actions: %+v", result.Actions)
	}
}

func TestMetadataApplyChecksStateAgainBeforeMutationReplay(t *testing.T) {
	tests := []struct {
		name            string
		matchSecondRead bool
		wantPatches     int
		wantAction      string
	}{
		{name: "eventual readback match avoids replay", matchSecondRead: true, wantPatches: 1, wantAction: "reconcile"},
		{name: "negative second readback permits replay", wantPatches: 2, wantAction: "update"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			t.Setenv("ASC_APP_ID", "")
			t.Setenv("ASC_MAX_RETRIES", "1")
			t.Setenv("ASC_BASE_DELAY", "1ms")
			t.Setenv("ASC_MAX_DELAY", "1ms")

			dir := t.TempDir()
			if err := os.MkdirAll(filepath.Join(dir, "version", "1.2.3"), 0o755); err != nil {
				t.Fatalf("mkdir version dir: %v", err)
			}
			if err := os.WriteFile(filepath.Join(dir, "version", "1.2.3", "en-US.json"), []byte(`{"description":"Local description"}`), 0o644); err != nil {
				t.Fatalf("write version file: %v", err)
			}

			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })
			versionReads := 0
			patchCount := 0
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch req.URL.Path {
				case "/v1/apps/app-1/appInfos":
					return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`), nil
				case "/v1/apps/app-1/appStoreVersions":
					return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`), nil
				case "/v1/appInfos/appinfo-1/appInfoLocalizations":
					return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
				case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
					versionReads++
					description := "Remote description"
					if test.matchSecondRead && versionReads == 3 {
						description = "Local description"
					}
					body := `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-ver-en","attributes":{"locale":"en-US","description":"` + description + `"}}],"links":{"next":""}}`
					return jsonHTTPResponse(http.StatusOK, body), nil
				case "/v1/appStoreVersionLocalizations/loc-ver-en":
					if req.Method != http.MethodPatch {
						t.Fatalf("expected PATCH, got %s", req.Method)
					}
					patchCount++
					if patchCount == 1 {
						return jsonHTTPResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","detail":"ambiguous"}]}`), nil
					}
					return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-ver-en","attributes":{"description":"Local description"}}}`), nil
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
					return nil, nil
				}
			})

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{
					"metadata", "apply",
					"--app", "app-1",
					"--version", "1.2.3",
					"--dir", dir,
					"--output", "json",
				}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			var result struct {
				Actions []struct {
					Action string `json:"action"`
				} `json:"actions"`
			}
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatalf("parse output: %v", err)
			}
			if patchCount != test.wantPatches || versionReads != 3 {
				t.Fatalf("unexpected recovery calls: patches=%d reads=%d", patchCount, versionReads)
			}
			if len(result.Actions) != 1 || result.Actions[0].Action != test.wantAction {
				t.Fatalf("unexpected action result: %+v", result.Actions)
			}
		})
	}
}

func TestMetadataApplyReconcilesAmbiguousCreateWithoutDuplicate(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_MAX_RETRIES", "0")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "en-US.json"), []byte(`{"name":"Created name"}`), 0o644); err != nil {
		t.Fatalf("write app-info: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	appInfoReads := 0
	posts := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-1/appInfos":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`), nil
		case "/v1/apps/app-1/appStoreVersions":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`), nil
		case "/v1/appInfos/appinfo-1/appInfoLocalizations":
			appInfoReads++
			if appInfoReads == 1 {
				return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"created-en","attributes":{"locale":"en-US","name":"Created name"}}],"links":{"next":""}}`), nil
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
		case "/v1/appInfoLocalizations":
			posts++
			return jsonHTTPResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","detail":"ambiguous create"}]}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"metadata", "apply", "--app", "app-1", "--version", "1.2.3", "--dir", dir, "--output", "json"}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run: %v", err)
		}
	})
	if stderr != "" || posts != 1 || appInfoReads != 2 {
		t.Fatalf("unexpected create recovery: posts=%d reads=%d stderr=%q", posts, appInfoReads, stderr)
	}
	if !strings.Contains(stdout, `"action":"reconcile"`) || !strings.Contains(stdout, `"localizationId":"created-en"`) {
		t.Fatalf("expected reconciled create identity, got %s", stdout)
	}
}

func TestMetadataApplyReconcilesAmbiguousDeleteWithoutReplay(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_MAX_RETRIES", "0")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "en-US.json"), []byte(`{"name":"Existing name"}`), 0o644); err != nil {
		t.Fatalf("write app-info: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	versionReads := 0
	deletes := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-1/appInfos":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`), nil
		case "/v1/apps/app-1/appStoreVersions":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`), nil
		case "/v1/appInfos/appinfo-1/appInfoLocalizations":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"loc-en","attributes":{"locale":"en-US","name":"Existing name"}}],"links":{"next":""}}`), nil
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			versionReads++
			if versionReads == 1 {
				return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"delete-fr","attributes":{"locale":"fr-FR","description":"Old"}}],"links":{"next":""}}`), nil
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
		case "/v1/appStoreVersionLocalizations/delete-fr":
			deletes++
			return jsonHTTPResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","detail":"ambiguous delete"}]}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "apply", "--app", "app-1", "--version", "1.2.3", "--dir", dir,
			"--allow-deletes", "--confirm", "--output", "json",
		}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run: %v", err)
		}
	})
	if stderr != "" || deletes != 1 || versionReads != 2 {
		t.Fatalf("unexpected delete recovery: deletes=%d reads=%d stderr=%q", deletes, versionReads, stderr)
	}
	if !strings.Contains(stdout, `"action":"reconcile"`) || !strings.Contains(stdout, `"localizationId":"delete-fr"`) {
		t.Fatalf("expected reconciled delete identity, got %s", stdout)
	}
}

func TestMetadataApplyRetriesInitialReadWithFreshDeadline(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_TIMEOUT", "20ms")
	t.Setenv("ASC_MAX_RETRIES", "1")
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "1ms")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "version", "1.2.3"), 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "version", "1.2.3", "en-US.json"), []byte(`{"description":"Local description","keywords":"one,two","supportUrl":"https://example.com/support","whatsNew":"New"}`), 0o644); err != nil {
		t.Fatalf("write version file: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	versionResolverReads := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-1/appStoreVersions":
			versionResolverReads++
			if versionResolverReads == 1 {
				<-req.Context().Done()
				return nil, req.Context().Err()
			}
			if err := req.Context().Err(); err != nil {
				t.Fatalf("retry received expired context: %v", err)
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`), nil
		case "/v1/apps/app-1/appInfos":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`), nil
		case "/v1/appInfos/appinfo-1/appInfoLocalizations":
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
		case "/v1/appStoreVersions/version-1":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "apply",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", dir,
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" || versionResolverReads < 2 {
		t.Fatalf("unexpected retry result: reads=%d stderr=%q", versionResolverReads, stderr)
	}
}

func TestMetadataApplyUsesFreshDeadlineForEachSnapshotPage(t *testing.T) {
	setupAuth(t)
	resetCmdtestState()
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_TIMEOUT", "100ms")
	t.Setenv("ASC_MAX_RETRIES", "0")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "en-US.json"), []byte(`{"name":"Local name"}`), 0o644); err != nil {
		t.Fatalf("write app-info: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	appInfoReads := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-1/appStoreVersions":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`), nil
		case "/v1/apps/app-1/appInfos":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`), nil
		case "/v1/appInfos/appinfo-1/appInfoLocalizations":
			appInfoReads++
			if appInfoReads == 1 {
				time.Sleep(60 * time.Millisecond)
				return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":"/v1/appInfos/appinfo-1/appInfoLocalizations?cursor=next"}}`), nil
			}
			deadline, ok := req.Context().Deadline()
			if !ok || time.Until(deadline) < 70*time.Millisecond {
				t.Fatalf("expected fresh second-page deadline, remaining=%s", time.Until(deadline))
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	result, _, err := metadatacli.ExecutePushWithWarnings(context.Background(), metadatacli.PushExecutionOptions{
		CommandName: "apply",
		AppID:       "app-1",
		Version:     "1.2.3",
		Dir:         dir,
		DryRun:      true,
	})
	if err != nil {
		t.Fatalf("ExecutePushWithWarnings() error: %v", err)
	}
	if appInfoReads != 2 || !result.DryRun {
		t.Fatalf("unexpected paginated snapshot: reads=%d result=%+v", appInfoReads, result)
	}
}

func TestMetadataApplyCancellationArtifactsRemainingActionsAcrossScopes(t *testing.T) {
	setupAuth(t)
	resetCmdtestState()
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	workDir := t.TempDir()
	t.Chdir(workDir)
	dir := filepath.Join(workDir, "metadata")
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "version", "1.2.3"), 0o755); err != nil {
		t.Fatalf("mkdir version: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "en-US.json"), []byte(`{"name":"English new"}`), 0o644); err != nil {
		t.Fatalf("write en-US: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "fr-FR.json"), []byte(`{"name":"French new"}`), 0o644); err != nil {
		t.Fatalf("write fr-FR: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "version", "1.2.3", "ja.json"), []byte(`{"description":"Japanese new"}`), 0o644); err != nil {
		t.Fatalf("write ja: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	mutations := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-1/appStoreVersions":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`), nil
		case "/v1/apps/app-1/appInfos":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`), nil
		case "/v1/appInfos/appinfo-1/appInfoLocalizations":
			return jsonHTTPResponse(http.StatusOK, `{"data":[
				{"type":"appInfoLocalizations","id":"loc-en","attributes":{"locale":"en-US","name":"English old"}},
				{"type":"appInfoLocalizations","id":"loc-fr","attributes":{"locale":"fr-FR","name":"French old"}}
			],"links":{"next":""}}`), nil
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-ja","attributes":{"locale":"ja","description":"Japanese old"}}],"links":{"next":""}}`), nil
		case "/v1/appInfoLocalizations/loc-en":
			mutations++
			body := &cancelOnEOFReadCloser{
				reader: strings.NewReader(`{"data":{"type":"appInfoLocalizations","id":"loc-en","attributes":{"name":"English new"}}}`),
				cancel: cancel,
			}
			return &http.Response{StatusCode: http.StatusOK, Body: body, Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		case "/v1/appInfoLocalizations/loc-fr", "/v1/appStoreVersionLocalizations/loc-ja":
			mutations++
			t.Fatalf("unexpected mutation after cancellation: %s", req.URL.Path)
			return nil, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	result, warnings, err := metadatacli.ExecutePushWithWarnings(ctx, metadatacli.PushExecutionOptions{
		CommandName: "apply",
		AppID:       "app-1",
		Version:     "1.2.3",
		Dir:         dir,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation error, got %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no create warnings, got %+v", warnings)
	}
	if result.Applied || result.Total != 3 || result.Succeeded != 1 || result.Failed != 2 || mutations != 1 {
		t.Fatalf("unexpected cancellation summary: result=%+v mutations=%d", result, mutations)
	}
	if len(result.Actions) != 3 || result.Actions[0].Status != "succeeded" || result.Actions[1].Status != "failed" || result.Actions[2].Status != "failed" {
		t.Fatalf("unexpected cancellation actions: %+v", result.Actions)
	}
	if result.Actions[1].Locale != "fr-FR" || result.Actions[1].LocalizationID != "loc-fr" || result.Actions[1].DesiredFields["name"] != "French new" {
		t.Fatalf("missing remaining app-info identity: %+v", result.Actions[1])
	}
	if result.Actions[2].Scope != "version" || result.Actions[2].Locale != "ja" || result.Actions[2].LocalizationID != "loc-ja" || result.Actions[2].DesiredFields["description"] != "Japanese new" {
		t.Fatalf("missing remaining version identity: %+v", result.Actions[2])
	}
	if result.FailureArtifactPath == "" {
		t.Fatalf("expected cancellation retry artifact: %+v", result)
	}
	artifact, readErr := os.ReadFile(result.FailureArtifactPath)
	if readErr != nil {
		t.Fatalf("read cancellation artifact: %v", readErr)
	}
	for _, expected := range []string{`"localizationId":"loc-fr"`, `"name":"French new"`, `"localizationId":"loc-ja"`, `"description":"Japanese new"`} {
		if !strings.Contains(string(artifact), expected) {
			t.Fatalf("cancellation artifact missing %q: %s", expected, artifact)
		}
	}
}

type cancelOnEOFReadCloser struct {
	reader io.Reader
	cancel context.CancelFunc
	once   sync.Once
}

func (r *cancelOnEOFReadCloser) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if err == io.EOF {
		r.once.Do(r.cancel)
	}
	return n, err
}

func (r *cancelOnEOFReadCloser) Close() error {
	return nil
}

func TestMetadataApplyPartialBatchPreservesTypedExitCode(t *testing.T) {
	run := runFailedMetadataApply(t, "json", true, false)
	if run.code != rootcmd.ExitHTTPUnprocessable {
		t.Fatalf("expected partial exit %d, got %d; stderr=%q", rootcmd.ExitHTTPUnprocessable, run.code, run.stderr)
	}
	if run.stderr != "" {
		t.Fatalf("expected reported error to avoid duplicate stderr, got %q", run.stderr)
	}
	var result struct {
		Failed              int    `json:"failed"`
		FailureArtifactPath string `json:"failureArtifactPath"`
	}
	if err := json.Unmarshal([]byte(run.stdout), &result); err != nil {
		t.Fatalf("parse structured output: %v\nstdout=%q", err, run.stdout)
	}
	if result.Failed != 1 || result.FailureArtifactPath == "" {
		t.Fatalf("expected structured partial summary, got %+v", result)
	}
}

func TestMetadataApplyPartialTableIncludesRecoveryDetails(t *testing.T) {
	run := runFailedMetadataApply(t, "table", false, false)
	if _, ok := errors.AsType[ReportedError](run.err); !ok {
		t.Fatalf("expected reported error, got %T: %v", run.err, run.err)
	}
	for _, expected := range []string{"Total: 1", "Succeeded: 0", "Failed: 1", "Failure Artifact: .asc/reports/metadata-apply/"} {
		if !strings.Contains(run.stdout, expected) {
			t.Fatalf("table output missing %q: %s", expected, run.stdout)
		}
	}
}

func TestMetadataApplyPartialMarkdownIncludesArtifactWriteError(t *testing.T) {
	run := runFailedMetadataApply(t, "markdown", false, true)
	if _, ok := errors.AsType[ReportedError](run.err); !ok {
		t.Fatalf("expected reported error, got %T: %v", run.err, run.err)
	}
	if !strings.Contains(run.stdout, "**Failure Artifact Error:**") || !strings.Contains(run.stdout, "not a directory") {
		t.Fatalf("markdown output missing artifact error: %s", run.stdout)
	}
}

func TestMetadataApplyInitialConflictPreservesTypedExit(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_MAX_RETRIES", "0")
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "version", "1.2.3"), 0o755); err != nil {
		t.Fatalf("mkdir version: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "version", "1.2.3", "en-US.json"), []byte(`{"description":"Desired"}`), 0o644); err != nil {
		t.Fatalf("write version: %v", err)
	}
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-1/appInfos":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`), nil
		case "/v1/apps/app-1/appStoreVersions":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`), nil
		case "/v1/appInfos/appinfo-1/appInfoLocalizations":
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return jsonHTTPResponse(http.StatusConflict, `{"errors":[{"status":"409","code":"CONFLICT","detail":"snapshot conflict"}]}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{"metadata", "apply", "--app", "app-1", "--version", "1.2.3", "--dir", dir, "--output", "json"}, "1.2.3")
	})
	if code != rootcmd.ExitConflict {
		t.Fatalf("expected conflict exit %d, got %d; stderr=%q", rootcmd.ExitConflict, code, stderr)
	}
	if stdout != "" || strings.Contains(stderr, "0 localization(s) failed") || strings.Contains(stderr, "Failure Artifact") {
		t.Fatalf("unexpected pre-batch summary: stdout=%q stderr=%q", stdout, stderr)
	}
}

type metadataApplyRun struct {
	code   int
	err    error
	stdout string
	stderr string
}

func runFailedMetadataApply(t *testing.T, output string, useEntrypoint bool, blockArtifact bool) metadataApplyRun {
	t.Helper()
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_MAX_RETRIES", "0")
	workDir := t.TempDir()
	t.Chdir(workDir)
	if blockArtifact {
		if err := os.WriteFile(filepath.Join(workDir, ".asc"), []byte("block reports"), 0o644); err != nil {
			t.Fatalf("write blocking .asc: %v", err)
		}
	}
	dir := filepath.Join(workDir, "metadata")
	if err := os.MkdirAll(filepath.Join(dir, "version", "1.2.3"), 0o755); err != nil {
		t.Fatalf("mkdir version: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "version", "1.2.3", "en-US.json"), []byte(`{"description":"Desired"}`), 0o644); err != nil {
		t.Fatalf("write version: %v", err)
	}
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-1/appInfos":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`), nil
		case "/v1/apps/app-1/appStoreVersions":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`), nil
		case "/v1/appInfos/appinfo-1/appInfoLocalizations":
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Old"}}],"links":{"next":""}}`), nil
		case "/v1/appStoreVersionLocalizations/loc-en":
			return jsonHTTPResponse(http.StatusUnprocessableEntity, `{"errors":[{"status":"422","code":"ENTITY_ERROR.ATTRIBUTE.INVALID","detail":"rejected"}]}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	args := []string{"metadata", "apply", "--app", "app-1", "--version", "1.2.3", "--dir", dir, "--output", output}
	var run metadataApplyRun
	run.stdout, run.stderr = captureOutput(t, func() {
		if useEntrypoint {
			run.code = rootcmd.Run(args, "1.2.3")
			return
		}
		root := RootCommand("1.2.3")
		root.FlagSet.SetOutput(io.Discard)
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse: %v", err)
		}
		run.err = root.Run(context.Background())
	})
	return run
}
