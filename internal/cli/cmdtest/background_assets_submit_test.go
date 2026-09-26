package cmdtest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func backgroundReviewSubmissionDetailJSON(resourceType, resourceID, state, platform, appID string) string {
	return fmt.Sprintf(`{"data":{"type":%q,"id":%q,"attributes":{"state":%q,"platform":%q},"relationships":{"app":{"data":{"type":"apps","id":%q}}}}}`, resourceType, resourceID, state, platform, appID)
}

func TestBackgroundAssetsSubmitPreflightRejectsAdversarialResponses(t *testing.T) {
	const (
		appID       = "123456789"
		platform    = "IOS"
		validType   = "reviewSubmissions"
		validID     = "sub-existing"
		validState  = "READY_FOR_REVIEW"
		validItemID = "item-1"
	)
	validDetail := backgroundReviewSubmissionDetailJSON(validType, validID, validState, platform, appID)
	validCreate := backgroundReviewSubmissionDetailJSON(validType, "sub-created", validState, platform, appID)
	validItems := `{"data":[],"links":{"self":"https://api.appstoreconnect.apple.com/v1/reviewSubmissions/sub-existing/items","next":""}}`

	tests := []struct {
		name               string
		reuseID            string
		createBody         string
		detailBody         string
		itemsBody          string
		cancelStatus       int
		cancelBody         string
		wantErr            string
		wantDetailRequests int32
		wantCancelRequests int32
	}{
		{
			name:               "missing item collection self link",
			reuseID:            validID,
			detailBody:         validDetail,
			itemsBody:          `{"data":[{"type":"reviewSubmissionItems","id":"item-1"}],"links":{"next":""}}`,
			wantErr:            "review submission items response links self is required",
			wantDetailRequests: 1,
		},
		{
			name:               "mixed item collection errors",
			reuseID:            validID,
			detailBody:         validDetail,
			itemsBody:          `{"data":[],"errors":[{"status":"500","detail":"partial response"}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/reviewSubmissions/sub-existing/items"}}`,
			wantErr:            "review submission items response must not contain top-level errors",
			wantDetailRequests: 1,
		},
		{
			name:               "wrong item resource type",
			reuseID:            validID,
			detailBody:         validDetail,
			itemsBody:          fmt.Sprintf(`{"data":[{"type":"appStoreVersions","id":%q}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/reviewSubmissions/sub-existing/items","next":""}}`, validItemID),
			wantErr:            "review submission items response data[0] type must be \"reviewSubmissionItems\"",
			wantDetailRequests: 1,
		},
		{
			name:               "empty item resource ID",
			reuseID:            validID,
			detailBody:         validDetail,
			itemsBody:          `{"data":[{"type":"reviewSubmissionItems","id":""}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/reviewSubmissions/sub-existing/items","next":""}}`,
			wantErr:            "review submission items response data[0] id must not be empty",
			wantDetailRequests: 1,
		},
		{
			name:               "background asset relationship missing linkage",
			reuseID:            validID,
			detailBody:         validDetail,
			itemsBody:          `{"data":[{"type":"reviewSubmissionItems","id":"item-1","relationships":{"backgroundAssetVersion":{"links":{"related":"/v1/reviewSubmissionItems/item-1/backgroundAssetVersion"}}}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/reviewSubmissions/sub-existing/items","next":""}}`,
			wantErr:            "background asset version relationship type \"\", not \"backgroundAssetVersions\"",
			wantDetailRequests: 1,
		},
		{
			name:               "existing submission wrong state",
			reuseID:            validID,
			detailBody:         backgroundReviewSubmissionDetailJSON(validType, validID, "WAITING_FOR_REVIEW", platform, appID),
			itemsBody:          validItems,
			wantErr:            "is in state \"WAITING_FOR_REVIEW\", not \"READY_FOR_REVIEW\"",
			wantDetailRequests: 1,
		},
		{
			name:               "existing submission wrong platform",
			reuseID:            validID,
			detailBody:         backgroundReviewSubmissionDetailJSON(validType, validID, validState, "MAC_OS", appID),
			itemsBody:          validItems,
			wantErr:            "is for platform \"MAC_OS\", not \"IOS\"",
			wantDetailRequests: 1,
		},
		{
			name:               "existing submission wrong app",
			reuseID:            validID,
			detailBody:         backgroundReviewSubmissionDetailJSON(validType, validID, validState, platform, "987654321"),
			itemsBody:          validItems,
			wantErr:            "belongs to app \"987654321\", not \"123456789\"",
			wantDetailRequests: 1,
		},
		{
			name:               "existing submission wrong ID",
			reuseID:            validID,
			detailBody:         backgroundReviewSubmissionDetailJSON(validType, "sub-other", validState, platform, appID),
			itemsBody:          validItems,
			wantErr:            "returned review submission sub-other instead of sub-existing",
			wantDetailRequests: 1,
		},
		{
			name:               "created receipt mixed errors",
			createBody:         `{"data":{"type":"reviewSubmissions","id":"sub-created"},"errors":[{"status":"500","detail":"partial response"}]}`,
			itemsBody:          validItems,
			wantErr:            "review submission response must not contain top-level errors",
			wantCancelRequests: 1,
		},
		{
			name:               "created receipt mixed errors and rollback fails",
			createBody:         `{"data":{"type":"reviewSubmissions","id":"sub-created"},"errors":[]}`,
			itemsBody:          validItems,
			cancelStatus:       http.StatusInternalServerError,
			cancelBody:         `{"errors":[{"status":"500","detail":"cancel unavailable"}]}`,
			wantErr:            `submission "sub-created" is leaked`,
			wantCancelRequests: 1,
		},
		{
			name:               "created receipt wrong resource type",
			createBody:         backgroundReviewSubmissionDetailJSON("appStoreVersions", "sub-created", validState, platform, appID),
			itemsBody:          validItems,
			wantErr:            "created review submission returned resource type \"appStoreVersions\"",
			wantCancelRequests: 1,
		},
		{
			name:       "created receipt empty ID",
			createBody: backgroundReviewSubmissionDetailJSON(validType, "", validState, platform, appID),
			itemsBody:  validItems,
			wantErr:    "created review submission returned an empty ID",
		},
		{
			name:               "created receipt wrong state",
			createBody:         backgroundReviewSubmissionDetailJSON(validType, "sub-created", "WAITING_FOR_REVIEW", platform, appID),
			itemsBody:          validItems,
			wantErr:            "created review submission sub-created is in state \"WAITING_FOR_REVIEW\"",
			wantCancelRequests: 1,
		},
		{
			name:               "created receipt wrong platform",
			createBody:         backgroundReviewSubmissionDetailJSON(validType, "sub-created", validState, "MAC_OS", appID),
			itemsBody:          validItems,
			wantErr:            "created review submission sub-created is for platform \"MAC_OS\"",
			wantCancelRequests: 1,
		},
		{
			name:               "created receipt wrong app",
			createBody:         backgroundReviewSubmissionDetailJSON(validType, "sub-created", validState, platform, "987654321"),
			itemsBody:          validItems,
			wantErr:            "created review submission sub-created belongs to app \"987654321\"",
			wantCancelRequests: 1,
		},
		{
			name:               "created detail wrong state",
			createBody:         validCreate,
			detailBody:         backgroundReviewSubmissionDetailJSON(validType, "sub-created", "WAITING_FOR_REVIEW", platform, appID),
			itemsBody:          validItems,
			wantErr:            "is in state \"WAITING_FOR_REVIEW\", not \"READY_FOR_REVIEW\"",
			wantDetailRequests: 1,
			wantCancelRequests: 1,
		},
		{
			name:               "created detail wrong platform",
			createBody:         validCreate,
			detailBody:         backgroundReviewSubmissionDetailJSON(validType, "sub-created", validState, "MAC_OS", appID),
			itemsBody:          validItems,
			wantErr:            "is for platform \"MAC_OS\", not \"IOS\"",
			wantDetailRequests: 1,
			wantCancelRequests: 1,
		},
		{
			name:               "created detail wrong app",
			createBody:         validCreate,
			detailBody:         backgroundReviewSubmissionDetailJSON(validType, "sub-created", validState, platform, "987654321"),
			itemsBody:          validItems,
			wantErr:            "belongs to app \"987654321\", not \"123456789\"",
			wantDetailRequests: 1,
			wantCancelRequests: 1,
		},
		{
			name:               "created detail wrong ID",
			createBody:         validCreate,
			detailBody:         backgroundReviewSubmissionDetailJSON(validType, "sub-other", validState, platform, appID),
			itemsBody:          validItems,
			wantErr:            "returned review submission sub-other instead of sub-created",
			wantDetailRequests: 1,
			wantCancelRequests: 1,
		},
		{
			name:               "created detail mixed errors",
			createBody:         validCreate,
			detailBody:         `{"data":{"type":"reviewSubmissions","id":"sub-created"},"errors":[{"status":"500","detail":"partial response"}]}`,
			itemsBody:          validItems,
			wantErr:            "review submission response must not contain top-level errors",
			wantDetailRequests: 1,
			wantCancelRequests: 1,
		},
		{
			name:               "created item collection missing self link",
			createBody:         validCreate,
			detailBody:         backgroundReviewSubmissionDetailJSON(validType, "sub-created", validState, platform, appID),
			itemsBody:          `{"data":[],"links":{}}`,
			wantErr:            "review submission items response links self is required",
			wantDetailRequests: 1,
			wantCancelRequests: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })

			var detailRequests, cancelRequests, itemPosts int32
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				path := req.URL.Path
				switch {
				case req.Method == http.MethodPost && path == "/v1/reviewSubmissions":
					if test.createBody == "" {
						t.Fatalf("unexpected review submission create")
					}
					return jsonResponse(http.StatusCreated, test.createBody)
				case req.Method == http.MethodGet && strings.HasPrefix(path, "/v1/reviewSubmissions/") && !strings.HasSuffix(path, "/items"):
					atomic.AddInt32(&detailRequests, 1)
					body := test.detailBody
					if body == "" {
						body = backgroundReviewSubmissionDetailJSON(validType, "sub-created", validState, platform, appID)
					}
					return jsonResponse(http.StatusOK, body)
				case req.Method == http.MethodGet && strings.HasSuffix(path, "/items"):
					body := test.itemsBody
					if body == "" {
						body = validItems
					}
					return jsonResponse(http.StatusOK, body)
				case req.Method == http.MethodPost && path == "/v1/reviewSubmissionItems":
					atomic.AddInt32(&itemPosts, 1)
					return jsonResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"item-created"}}`)
				case req.Method == http.MethodPatch:
					atomic.AddInt32(&cancelRequests, 1)
					status := test.cancelStatus
					if status == 0 {
						status = http.StatusOK
					}
					body := test.cancelBody
					if body == "" {
						body = `{"data":{"type":"reviewSubmissions","id":"sub-created","attributes":{"state":"CANCELING"}}}`
					}
					return jsonResponse(status, body)
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			})

			args := []string{"background-assets", "submit", "--app", appID, "--version-id", "version-1", "--confirm", "--output", "json"}
			if test.reuseID != "" {
				args = append(args, "--review-submission-id", test.reuseID)
			} else if test.createBody == "" {
				t.Fatalf("test case must specify reuseID or createBody")
			}

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			var runErr error
			_, _ = captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})
			if runErr == nil {
				t.Fatalf("expected preflight error containing %q", test.wantErr)
			}
			if !strings.Contains(runErr.Error(), test.wantErr) {
				t.Fatalf("error = %q, want substring %q", runErr, test.wantErr)
			}
			if got := atomic.LoadInt32(&itemPosts); got != 0 {
				t.Fatalf("expected zero review-submission item POSTs, got %d", got)
			}
			if got := atomic.LoadInt32(&detailRequests); got != test.wantDetailRequests {
				t.Fatalf("detail requests = %d, want %d", got, test.wantDetailRequests)
			}
			if got := atomic.LoadInt32(&cancelRequests); got != test.wantCancelRequests {
				t.Fatalf("cancel requests = %d, want %d", got, test.wantCancelRequests)
			}
		})
	}
}

func TestBackgroundAssetsSubmitStopsBeforeFetchingItemPageBeyondLimit(t *testing.T) {
	const (
		appID        = "123456789"
		submissionID = "sub-existing"
	)
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	var itemGets, itemPosts int32
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/"+submissionID:
			return jsonResponse(http.StatusOK, backgroundReviewSubmissionDetailJSON("reviewSubmissions", submissionID, "READY_FOR_REVIEW", "IOS", appID))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/"+submissionID+"/items":
			page := atomic.AddInt32(&itemGets, 1)
			next := fmt.Sprintf("https://api.appstoreconnect.apple.com/v1/reviewSubmissions/%s/items?page=%d", submissionID, page+1)
			body := fmt.Sprintf(`{"data":[],"links":{"self":%q,"next":%q}}`, req.URL.String(), next)
			return jsonResponse(http.StatusOK, body)
		case req.Method == http.MethodPost:
			atomic.AddInt32(&itemPosts, 1)
			return jsonResponse(http.StatusInternalServerError, `{"errors":[{"detail":"unexpected mutation"}]}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	args := []string{
		"background-assets", "submit",
		"--app", appID,
		"--version-id", "version-1",
		"--review-submission-id", submissionID,
		"--confirm",
		"--output", "json",
	}
	if err := root.Parse(args); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	_, _ = captureOutput(t, func() {
		err := root.Run(context.Background())
		if err == nil || !strings.Contains(err.Error(), "page 1001 exceeds the maximum of 1000 pages") {
			t.Fatalf("error = %v, want pagination limit failure", err)
		}
	})

	if got := atomic.LoadInt32(&itemGets); got != 1000 {
		t.Fatalf("item GET requests = %d, want 1000", got)
	}
	if got := atomic.LoadInt32(&itemPosts); got != 0 {
		t.Fatalf("mutation POST requests = %d, want 0", got)
	}
}

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
		case req.Method == http.MethodGet && path == "/v1/reviewSubmissions/sub-xyz":
			return jsonResponse(http.StatusOK, backgroundReviewSubmissionDetailJSON("reviewSubmissions", "sub-xyz", "READY_FOR_REVIEW", "IOS", "123456789"))
		case req.Method == http.MethodGet && path == "/v1/reviewSubmissions/sub-xyz/items":
			return jsonResponse(http.StatusOK, `{"data":[],"links":{"self":"https://api.appstoreconnect.apple.com/v1/reviewSubmissions/sub-xyz/items"}}`)
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
		case req.Method == http.MethodGet && path == "/v1/reviewSubmissions/sub-existing":
			return jsonResponse(http.StatusOK, backgroundReviewSubmissionDetailJSON("reviewSubmissions", "sub-existing", "READY_FOR_REVIEW", "IOS", "123456789"))
		case req.Method == http.MethodGet && path == "/v1/reviewSubmissions/sub-existing/items":
			atomic.AddInt32(&listItems, 1)
			if include := req.URL.Query().Get("include"); !strings.Contains(include, "backgroundAssetVersion") {
				t.Errorf("expected include=backgroundAssetVersion on items list, got %q", include)
			}
			return jsonResponse(http.StatusOK, `{"data":[
				{"type":"reviewSubmissionItems","id":"existing-item-1","attributes":{"state":"READY_FOR_REVIEW"},"relationships":{"backgroundAssetVersion":{"data":{"type":"backgroundAssetVersions","id":"asset-1-v1"}}}}
			],"links":{"self":"https://api.appstoreconnect.apple.com/v1/reviewSubmissions/sub-existing/items","next":""}}`)
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
		case req.Method == http.MethodGet && path == "/v1/reviewSubmissions/sub-doomed":
			return jsonResponse(http.StatusOK, backgroundReviewSubmissionDetailJSON("reviewSubmissions", "sub-doomed", "READY_FOR_REVIEW", "IOS", "123456789"))
		case req.Method == http.MethodGet && path == "/v1/reviewSubmissions/sub-doomed/items":
			return jsonResponse(http.StatusOK, `{"data":[],"links":{"self":"https://api.appstoreconnect.apple.com/v1/reviewSubmissions/sub-doomed/items"}}`)
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

func TestBackgroundAssetsSubmitRenewsAttachBudgetAndRollsBackFailure(t *testing.T) {
	const (
		appID        = "123456789"
		submissionID = "sub-created"
		timeout      = 700 * time.Millisecond
	)
	setupAuth(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_TIMEOUT", timeout.String())
	t.Setenv("ASC_TIMEOUT_SECONDS", "")
	t.Setenv("ASC_MAX_RETRIES", "0")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			_, _ = io.WriteString(w, backgroundReviewSubmissionDetailJSON("reviewSubmissions", submissionID, "READY_FOR_REVIEW", "IOS", appID))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/"+submissionID:
			// These individually bounded reads consume most of the old shared
			// command deadline before the attach begins.
			time.Sleep(250 * time.Millisecond)
			_, _ = io.WriteString(w, backgroundReviewSubmissionDetailJSON("reviewSubmissions", submissionID, "READY_FOR_REVIEW", "IOS", appID))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/"+submissionID+"/items":
			time.Sleep(350 * time.Millisecond)
			_, _ = io.WriteString(w, `{"data":[],"links":{"self":"https://api.appstoreconnect.apple.com/v1/reviewSubmissions/sub-created/items","next":""}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			time.Sleep(250 * time.Millisecond)
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"errors":[{"status":"500","detail":"attach failed"}]}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/"+submissionID:
			var body struct {
				Data struct {
					Attributes struct {
						Canceled *bool `json:"canceled"`
					} `json:"attributes"`
				} `json:"data"`
			}
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Errorf("decode rollback request: %v", err)
			} else if body.Data.Attributes.Canceled == nil || !*body.Data.Attributes.Canceled {
				t.Errorf("rollback canceled attribute = %v, want true", body.Data.Attributes.Canceled)
			}
			_, _ = io.WriteString(w, `{"data":{"type":"reviewSubmissions","id":"sub-created","attributes":{"state":"CANCELING"}}}`)
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	serverTransport := server.Client().Transport
	attachBudget := make(chan time.Duration, 1)
	rollbackBudget := make(chan time.Duration, 1)
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if deadline, ok := req.Context().Deadline(); ok {
			remaining := time.Until(deadline)
			switch {
			case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
				attachBudget <- remaining
			case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/"+submissionID:
				rollbackBudget <- remaining
			}
		} else if req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems" {
			attachBudget <- 0
		} else if req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/"+submissionID {
			rollbackBudget <- 0
		}

		cloned := req.Clone(req.Context())
		cloned.URL.Scheme = serverURL.Scheme
		cloned.URL.Host = serverURL.Host
		return serverTransport.RoundTrip(cloned)
	})
	client, err := asc.NewClientWithHTTPClient(
		os.Getenv("ASC_KEY_ID"),
		os.Getenv("ASC_ISSUER_ID"),
		os.Getenv("ASC_PRIVATE_KEY_PATH"),
		&http.Client{Transport: transport},
	)
	if err != nil {
		t.Fatalf("create background-assets submit client: %v", err)
	}
	restoreClient := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil })
	t.Cleanup(restoreClient)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	args := []string{
		"background-assets", "submit",
		"--app", appID,
		"--version-id", "version-1",
		"--confirm",
		"--output", "json",
	}
	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if runErr == nil || !strings.Contains(runErr.Error(), "attach version") || !strings.Contains(runErr.Error(), "rolled back the submission") {
		t.Fatalf("run error = %v, want failed attach followed by rollback", runErr)
	}
	if got := <-attachBudget; got < 500*time.Millisecond {
		t.Fatalf("attach request had only %s remaining, want a fresh per-request budget near %s", got, timeout)
	}
	if got := <-rollbackBudget; got < 500*time.Millisecond {
		t.Fatalf("rollback request had only %s remaining, want an independent fresh budget", got)
	}
}
