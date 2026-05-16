package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestBackgroundAssetsSubmitValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing app",
			args:    []string{"background-assets", "submit", "--all", "--confirm"},
			wantErr: "--app is required",
		},
		{
			name:    "missing selection",
			args:    []string{"background-assets", "submit", "--app", "123456789", "--confirm"},
			wantErr: "one of --all, --asset-pack-identifier, --background-asset-id, or --version-id is required",
		},
		{
			name:    "mutually exclusive selection: --all + --asset-pack-identifier",
			args:    []string{"background-assets", "submit", "--app", "123456789", "--all", "--asset-pack-identifier", "stamps.us", "--confirm"},
			wantErr: "mutually exclusive",
		},
		{
			name:    "mutually exclusive selection: --background-asset-id + --version-id",
			args:    []string{"background-assets", "submit", "--app", "123456789", "--background-asset-id", "AID", "--version-id", "VID", "--confirm"},
			wantErr: "mutually exclusive",
		},
		{
			name:    "missing confirm and dry-run",
			args:    []string{"background-assets", "submit", "--app", "123456789", "--all"},
			wantErr: "--confirm is required unless --dry-run is set",
		},
		{
			name:    "no-submit conflicts with dry-run",
			args:    []string{"background-assets", "submit", "--app", "123456789", "--all", "--dry-run", "--no-submit"},
			wantErr: "--no-submit and --dry-run are mutually exclusive",
		},
		{
			name:    "invalid platform",
			args:    []string{"background-assets", "submit", "--app", "123456789", "--all", "--platform", "WEBOS", "--confirm"},
			wantErr: "platform",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := captureOutput(t, func() {
				if code := rootcmd.Run(test.args, "1.2.3"); code != rootcmd.ExitUsage {
					t.Fatalf("expected exit code %d, got %d", rootcmd.ExitUsage, code)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestBackgroundAssetsSubmitDryRunWithExplicitVersionIDsDoesNotCallNetwork(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	var calls int32
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		atomic.AddInt32(&calls, 1)
		t.Fatalf("unexpected network call during dry-run: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	args := []string{
		"background-assets", "submit",
		"--app", "123456789",
		"--version-id", "ver-a, ver-a, ver-b",
		"--dry-run",
		"--output", "json",
	}

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if atomic.LoadInt32(&calls) != 0 {
		t.Fatalf("expected zero network calls, got %d", calls)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var parsed struct {
		AppID         string `json:"appId"`
		DryRun        bool   `json:"dryRun"`
		AttachedItems int    `json:"attachedItems"`
		Items         []struct {
			BackgroundAssetVersionID string `json:"backgroundAssetVersionId"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("expected valid JSON on stdout, got %q (err=%v)", stdout, err)
	}
	if !parsed.DryRun {
		t.Fatalf("expected dryRun=true, got %+v", parsed)
	}
	if parsed.AppID != "123456789" {
		t.Fatalf("expected appId=123456789, got %q", parsed.AppID)
	}
	if len(parsed.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(parsed.Items))
	}
	if parsed.Items[0].BackgroundAssetVersionID != "ver-a" || parsed.Items[1].BackgroundAssetVersionID != "ver-b" {
		t.Fatalf("expected items [ver-a ver-b], got %+v", parsed.Items)
	}
}

func TestBackgroundAssetsSubmitAllHappyPath(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	type counters struct {
		listAssets       int32
		listVersionsByID map[string]*int32
		createSubmission int32
		createItem       int32
		patchSubmit      int32
	}
	c := &counters{listVersionsByID: map[string]*int32{
		"asset-1": new(int32),
		"asset-2": new(int32),
	}}

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		path := req.URL.Path
		switch {
		case req.Method == http.MethodGet && path == "/v1/apps/123456789/backgroundAssets":
			atomic.AddInt32(&c.listAssets, 1)
			return jsonResponse(http.StatusOK, `{"data":[
				{"type":"backgroundAssets","id":"asset-1","attributes":{"assetPackIdentifier":"pack.one"}},
				{"type":"backgroundAssets","id":"asset-2","attributes":{"assetPackIdentifier":"pack.two"}}
			],"links":{"next":""}}`)
		case req.Method == http.MethodGet && strings.HasPrefix(path, "/v1/backgroundAssets/") && strings.HasSuffix(path, "/versions"):
			assetID := strings.TrimSuffix(strings.TrimPrefix(path, "/v1/backgroundAssets/"), "/versions")
			counter, ok := c.listVersionsByID[assetID]
			if !ok {
				t.Fatalf("unexpected versions list for asset %q", assetID)
			}
			atomic.AddInt32(counter, 1)
			return jsonResponse(http.StatusOK, `{"data":[
				{"type":"backgroundAssetVersions","id":"`+assetID+`-v1","attributes":{"state":"COMPLETE","version":"1","platforms":["IOS"],"createdDate":"2026-05-01T00:00:00Z"}}
			],"links":{"next":""}}`)
		case req.Method == http.MethodPost && path == "/v1/reviewSubmissions":
			atomic.AddInt32(&c.createSubmission, 1)
			return jsonResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"sub-xyz","attributes":{"platform":"IOS","state":"READY_FOR_REVIEW"}}}`)
		case req.Method == http.MethodPost && path == "/v1/reviewSubmissionItems":
			atomic.AddInt32(&c.createItem, 1)
			return jsonResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"item-`+req.URL.Path+`"}}`)
		case req.Method == http.MethodPatch && path == "/v1/reviewSubmissions/sub-xyz":
			atomic.AddInt32(&c.patchSubmit, 1)
			return jsonResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"sub-xyz","attributes":{"platform":"IOS","state":"WAITING_FOR_REVIEW","submittedDate":"2026-05-14T18:33:37Z"}}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	args := []string{
		"background-assets", "submit",
		"--app", "123456789",
		"--all",
		"--confirm",
		"--output", "json",
	}

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if got := atomic.LoadInt32(&c.listAssets); got != 1 {
		t.Errorf("expected 1 listAssets call, got %d", got)
	}
	if got := atomic.LoadInt32(c.listVersionsByID["asset-1"]); got != 1 {
		t.Errorf("expected 1 listVersions call for asset-1, got %d", got)
	}
	if got := atomic.LoadInt32(c.listVersionsByID["asset-2"]); got != 1 {
		t.Errorf("expected 1 listVersions call for asset-2, got %d", got)
	}
	if got := atomic.LoadInt32(&c.createSubmission); got != 1 {
		t.Errorf("expected 1 createSubmission call, got %d", got)
	}
	if got := atomic.LoadInt32(&c.createItem); got != 2 {
		t.Errorf("expected 2 createItem calls, got %d", got)
	}
	if got := atomic.LoadInt32(&c.patchSubmit); got != 1 {
		t.Errorf("expected 1 patchSubmit call, got %d", got)
	}

	var parsed struct {
		SubmissionID    string `json:"submissionId"`
		SubmissionState string `json:"submissionState"`
		AttachedItems   int    `json:"attachedItems"`
	}
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("invalid JSON on stdout: %v\n%s", err, stdout)
	}
	if parsed.SubmissionID != "sub-xyz" {
		t.Errorf("expected submissionId sub-xyz, got %q", parsed.SubmissionID)
	}
	if parsed.SubmissionState != "WAITING_FOR_REVIEW" {
		t.Errorf("expected submissionState WAITING_FOR_REVIEW, got %q", parsed.SubmissionState)
	}
	if parsed.AttachedItems != 2 {
		t.Errorf("expected 2 attached items, got %d", parsed.AttachedItems)
	}
}

func TestBackgroundAssetsSubmitReuseExistingSkipsAttached(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	var (
		createSubmission int32
		createItem       int32
		patchSubmit      int32
		listItems        int32
	)

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		path := req.URL.Path
		switch {
		case req.Method == http.MethodGet && path == "/v1/apps/123456789/backgroundAssets":
			return jsonResponse(http.StatusOK, `{"data":[
				{"type":"backgroundAssets","id":"asset-1","attributes":{"assetPackIdentifier":"pack.one"}},
				{"type":"backgroundAssets","id":"asset-2","attributes":{"assetPackIdentifier":"pack.two"}}
			],"links":{"next":""}}`)
		case req.Method == http.MethodGet && strings.HasPrefix(path, "/v1/backgroundAssets/") && strings.HasSuffix(path, "/versions"):
			assetID := strings.TrimSuffix(strings.TrimPrefix(path, "/v1/backgroundAssets/"), "/versions")
			return jsonResponse(http.StatusOK, `{"data":[
				{"type":"backgroundAssetVersions","id":"`+assetID+`-v1","attributes":{"state":"COMPLETE","version":"1","platforms":["IOS"]}}
			],"links":{"next":""}}`)
		case req.Method == http.MethodGet && path == "/v1/reviewSubmissions/sub-existing/items":
			atomic.AddInt32(&listItems, 1)
			if include := req.URL.Query().Get("include"); !strings.Contains(include, "backgroundAssetVersion") {
				t.Errorf("expected include=backgroundAssetVersion on items list, got %q", include)
			}
			return jsonResponse(http.StatusOK, `{"data":[
				{"type":"reviewSubmissionItems","id":"existing-item-1","attributes":{"state":"READY_FOR_REVIEW"},"relationships":{"backgroundAssetVersion":{"data":{"type":"backgroundAssetVersions","id":"asset-1-v1"}}}}
			],"links":{"next":""}}`)
		case req.Method == http.MethodPost && path == "/v1/reviewSubmissions":
			atomic.AddInt32(&createSubmission, 1)
			t.Errorf("CreateReviewSubmission should not be called when reusing")
			return jsonResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"unexpected"}}`)
		case req.Method == http.MethodPost && path == "/v1/reviewSubmissionItems":
			atomic.AddInt32(&createItem, 1)
			return jsonResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"item-new"}}`)
		case req.Method == http.MethodPatch && path == "/v1/reviewSubmissions/sub-existing":
			atomic.AddInt32(&patchSubmit, 1)
			return jsonResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"sub-existing","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-05-14T18:33:37Z"}}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	args := []string{
		"background-assets", "submit",
		"--app", "123456789",
		"--all",
		"--review-submission-id", "sub-existing",
		"--confirm",
		"--output", "json",
	}

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if got := atomic.LoadInt32(&listItems); got != 1 {
		t.Errorf("expected 1 listItems call, got %d", got)
	}
	if got := atomic.LoadInt32(&createSubmission); got != 0 {
		t.Errorf("expected 0 createSubmission calls when reusing, got %d", got)
	}
	if got := atomic.LoadInt32(&createItem); got != 1 {
		t.Errorf("expected 1 createItem call (asset-1 already attached, asset-2 new), got %d", got)
	}
	if got := atomic.LoadInt32(&patchSubmit); got != 1 {
		t.Errorf("expected 1 patchSubmit call, got %d", got)
	}

	var parsed struct {
		AttachedItems          int `json:"attachedItems"`
		SkippedAlreadyAttached []struct {
			BackgroundAssetVersionID string `json:"backgroundAssetVersionId"`
		} `json:"skippedAlreadyAttached"`
	}
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("invalid JSON on stdout: %v\n%s", err, stdout)
	}
	if parsed.AttachedItems != 1 {
		t.Errorf("expected 1 newly attached item, got %d", parsed.AttachedItems)
	}
	if len(parsed.SkippedAlreadyAttached) != 1 || parsed.SkippedAlreadyAttached[0].BackgroundAssetVersionID != "asset-1-v1" {
		t.Errorf("expected 1 skipped item asset-1-v1, got %+v", parsed.SkippedAlreadyAttached)
	}
}

func TestBackgroundAssetsSubmitRollsBackEmptySubmissionOnFirstAttachFailure(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	var (
		createSubmission int32
		patchCancel      int32
	)

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		path := req.URL.Path
		switch {
		case req.Method == http.MethodGet && path == "/v1/apps/123456789/backgroundAssets":
			return jsonResponse(http.StatusOK, `{"data":[
				{"type":"backgroundAssets","id":"asset-1","attributes":{"assetPackIdentifier":"pack.one"}}
			],"links":{"next":""}}`)
		case req.Method == http.MethodGet && strings.HasPrefix(path, "/v1/backgroundAssets/") && strings.HasSuffix(path, "/versions"):
			return jsonResponse(http.StatusOK, `{"data":[
				{"type":"backgroundAssetVersions","id":"asset-1-v1","attributes":{"state":"COMPLETE","version":"1","platforms":["IOS"]}}
			],"links":{"next":""}}`)
		case req.Method == http.MethodPost && path == "/v1/reviewSubmissions":
			atomic.AddInt32(&createSubmission, 1)
			return jsonResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"sub-doomed","attributes":{"platform":"IOS","state":"READY_FOR_REVIEW"}}}`)
		case req.Method == http.MethodPost && path == "/v1/reviewSubmissionItems":
			return jsonResponse(http.StatusConflict, `{"errors":[{"status":"409","code":"ENTITY_ERROR.STATE_NOT_ALLOWED","detail":"already attached to another submission"}]}`)
		case req.Method == http.MethodPatch && path == "/v1/reviewSubmissions/sub-doomed":
			atomic.AddInt32(&patchCancel, 1)
			return jsonResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"sub-doomed","attributes":{"state":"CANCELING"}}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	args := []string{
		"background-assets", "submit",
		"--app", "123456789",
		"--all",
		"--confirm",
		"--output", "json",
	}

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatalf("expected error when first attach fails, got nil")
	}
	if !strings.Contains(runErr.Error(), "rolled back the submission") {
		t.Fatalf("expected error to mention rollback, got %v", runErr)
	}
	if got := atomic.LoadInt32(&createSubmission); got != 1 {
		t.Errorf("expected 1 createSubmission call, got %d", got)
	}
	if got := atomic.LoadInt32(&patchCancel); got != 1 {
		t.Errorf("expected 1 cancel call (rollback), got %d", got)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout on failure, got %q", stdout)
	}
}

func TestBackgroundAssetsSubmitFlagOrderEdgeCases(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("dry-run should not call the network: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	cases := []struct {
		name string
		args []string
	}{
		{
			name: "global flag before subcommands",
			args: []string{
				"--profile", "review-edge",
				"background-assets", "submit",
				"--app", "123456789",
				"--version-id", "v-before",
				"--dry-run",
				"--output", "json",
			},
		},
		{
			name: "all flags before subcommand-style positionals",
			args: []string{
				"background-assets", "submit",
				"--app", "123456789",
				"--version-id", "v1",
				"--platform", "IOS",
				"--dry-run",
				"--output", "json",
			},
		},
		{
			name: "flag values that look like subcommand names",
			args: []string{
				"background-assets", "submit",
				"--app", "123456789",
				"--version-id", "submit",
				"--dry-run",
				"--output", "json",
			},
		},
		{
			name: "mixed-order: dry-run before app, output last",
			args: []string{
				"background-assets", "submit",
				"--dry-run",
				"--version-id", "vx",
				"--app", "123456789",
				"--output", "json",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(c.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("unexpected run error: %v", err)
				}
			})

			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			var out struct {
				DryRun bool `json:"dryRun"`
			}
			if err := json.Unmarshal([]byte(stdout), &out); err != nil {
				t.Fatalf("expected valid dry-run JSON on stdout, got %q (err=%v)", stdout, err)
			}
			if !out.DryRun {
				t.Fatalf("expected dryRun=true in stdout JSON, got %q", stdout)
			}
		})
	}
}
