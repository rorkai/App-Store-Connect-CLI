package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/handlertest"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

// capturedDuplicateAppCreate409 carries exactly what was captured live on
// 2026-09-29 when re-creating disposable app 6759231657 (same name, bundle ID
// and SKU, --auto-rename=false): POST /iris/v1/apps answered HTTP 409 with
// three errors[] entries whose codes are, in order, the three below. Only the
// status and the codes were captured; the entries' detail, title and source
// were not printed, so this fixture deliberately carries no other fields.
const capturedDuplicateAppCreate409 = `{"errors":[` +
	`{"code":"ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE"},` +
	`{"code":"ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE"},` +
	`{"code":"ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE.SAME_ACCOUNT"}]}`

// capturedDuplicateAppCreateError is the error text the CLI printed for the
// captured 409 (no request ID or correlation key in the test transport).
const capturedDuplicateAppCreateError = "web apps create failed: web api error (status 409), codes=[" +
	"ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE " +
	"ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE.SAME_ACCOUNT]"

// syntheticDuplicateAppCreate409WithNameDetail uses the captured status and
// codes. SYNTHETIC: every detail and source field below is inferred, not
// captured. errors[0] carries the "app name ... already being used" detail
// that makes webcore.IsDuplicateAppNameError fire, so the test can prove that
// --if-exists skip resolves an existing app before --auto-rename retries.
const syntheticDuplicateAppCreate409WithNameDetail = `{"errors":[` +
	`{"code":"ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE","detail":"The app name you entered is already being used.","source":{"pointer":"/included/appInfoLocalizations/name"}},` +
	`{"code":"ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE","detail":"The SKU you entered has already been used.","source":{"pointer":"/data/attributes/sku"}},` +
	`{"code":"ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE.SAME_ACCOUNT","detail":"The bundle ID you entered is already used by another app in your account.","source":{"pointer":"/data/attributes/bundleId"}}]}`

type ifExistsCreateHarness struct {
	mu          sync.Mutex
	createNames []string
	apiRequests []string
}

func (h *ifExistsCreateHarness) creates() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.createNames...)
}

func (h *ifExistsCreateHarness) reads() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.apiRequests...)
}

// setupIfExistsCreate wires the web session to answer every POST /apps with
// firstBody as HTTP 409 on the first attempt and 201 afterwards, and the
// public API to answer GET /v1/apps with the given responses keyed by filter.
func setupIfExistsCreate(t *testing.T, firstBody string, byBundleID, bySKU string) *ifExistsCreateHarness {
	t.Helper()
	h := &ifExistsCreateHarness{}

	origResolve := resolveAppCreateSessionFn
	t.Cleanup(func() { resolveAppCreateSessionFn = origResolve })
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		var payload struct {
			Included []struct {
				Type       string `json:"type"`
				Attributes struct {
					Name string `json:"name"`
				} `json:"attributes"`
			} `json:"included"`
		}
		_ = json.Unmarshal(body, &payload)
		name := ""
		for _, inc := range payload.Included {
			if inc.Type == "appInfoLocalizations" {
				name = inc.Attributes.Name
			}
		}
		h.mu.Lock()
		h.createNames = append(h.createNames, req.Method+" "+req.URL.Path+" "+name)
		attempt := len(h.createNames)
		h.mu.Unlock()

		status := http.StatusCreated
		respBody := `{"data":{"id":"app-new","type":"apps","attributes":{}}}`
		if attempt == 1 {
			status = http.StatusConflict
			respBody = firstBody
		}
		return &http.Response{
			StatusCode: status,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(respBody)),
			Request:    req,
		}, nil
	})
	resolveAppCreateSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{Client: &http.Client{Transport: transport}}, "cache", nil
	}

	fixture := handlertest.New(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		h.mu.Lock()
		h.apiRequests = append(h.apiRequests, req.Method+" "+req.URL.Path+"?"+req.URL.RawQuery)
		h.mu.Unlock()
		if req.Method != http.MethodGet || req.URL.Path != "/v1/apps" {
			fixture.Respond(w, "unexpected request: %s %s", req.Method, req.URL.Path)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		query := req.URL.Query()
		switch {
		case query.Get("filter[bundleId]") != "":
			_, _ = w.Write([]byte(byBundleID))
		case query.Get("filter[sku]") != "":
			_, _ = w.Write([]byte(bySKU))
		default:
			fixture.Respond(w, "unexpected apps query: %s", req.URL.RawQuery)
		}
	}))
	t.Cleanup(server.Close)
	setAppCreateASCClient(t, server)
	stubBundleIDPreflight(t)
	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")
	return h
}

const (
	ifExistsTestName     = "ASC Test 20260216074703"
	ifExistsTestBundleID = "com.rudrank.asc.throwaway20260216074703"
	ifExistsTestSKU      = "asc-test-20260216074703"
	emptyAppsResponse    = `{"data":[],"links":{}}`
)

func appsListResponse(id, name, bundleID, sku string) string {
	return `{"data":[{"type":"apps","id":"` + id + `","attributes":{"name":"` + name + `","bundleId":"` + bundleID + `","sku":"` + sku + `","primaryLocale":"en-US"}}],"links":{}}`
}

func ifExistsCreateOptions(mode shared.IfExistsMode, autoRename bool) AppsCreateRunOptions {
	return AppsCreateRunOptions{
		Name:       ifExistsTestName,
		BundleID:   ifExistsTestBundleID,
		SKU:        ifExistsTestSKU,
		AppleID:    "user@example.com",
		Output:     "json",
		AutoRename: autoRename,
		IfExists:   mode,
	}
}

func TestRunAppsCreateIfExistsFailKeepsCaptured409(t *testing.T) {
	for _, mode := range []shared.IfExistsMode{"", shared.IfExistsFail} {
		t.Run("mode="+string(mode), func(t *testing.T) {
			h := setupIfExistsCreate(t, capturedDuplicateAppCreate409, emptyAppsResponse, emptyAppsResponse)

			var err error
			stdout, _ := captureOutput(t, func() {
				err = RunAppsCreate(context.Background(), ifExistsCreateOptions(mode, true))
			})
			if err == nil {
				t.Fatal("expected the captured 409 to fail")
			}
			if err.Error() != capturedDuplicateAppCreateError {
				t.Fatalf("error = %q\nwant    %q", err.Error(), capturedDuplicateAppCreateError)
			}
			var apiErr *webcore.APIError
			if !errors.As(err, &apiErr) || apiErr.Status != http.StatusConflict {
				t.Fatalf("expected wrapped web 409, got %T %v", err, err)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if got := h.creates(); len(got) != 1 {
				t.Fatalf("creates = %v, want exactly one POST", got)
			}
			if got := h.reads(); len(got) != 0 {
				t.Fatalf("fail mode must not read back, got %v", got)
			}
		})
	}
}

func TestRunAppsCreateIfExistsSkipCaptured409ResolvesExistingApp(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "captured codes only", body: capturedDuplicateAppCreate409},
		{name: "synthetic name detail would trigger auto-rename", body: syntheticDuplicateAppCreate409WithNameDetail},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := setupIfExistsCreate(t, tc.body,
				appsListResponse("6759231657", ifExistsTestName, ifExistsTestBundleID, ifExistsTestSKU),
				emptyAppsResponse)

			var err error
			stdout, stderr := captureOutput(t, func() {
				err = RunAppsCreate(context.Background(), ifExistsCreateOptions(shared.IfExistsSkip, true))
			})
			if err != nil {
				t.Fatalf("RunAppsCreate error: %v", err)
			}
			if got := h.creates(); len(got) != 1 {
				t.Fatalf("creates = %v, want one POST and no auto-rename retry", got)
			}
			reads := h.reads()
			if len(reads) != 1 || !strings.Contains(reads[0], "filter%5BbundleId%5D="+ifExistsTestBundleID) {
				t.Fatalf("reads = %v, want one bundle ID read-back", reads)
			}

			var receipt map[string]any
			if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
				t.Fatalf("unmarshal receipt: %v\nstdout=%s", err, stdout)
			}
			want := map[string]any{
				"id":            "6759231657",
				"name":          ifExistsTestName,
				"bundleId":      ifExistsTestBundleID,
				"sku":           ifExistsTestSKU,
				"alreadyExists": true,
				"action":        "skipped",
			}
			for key, value := range want {
				if receipt[key] != value {
					t.Fatalf("receipt[%q] = %v, want %v (receipt=%s)", key, receipt[key], value, stdout)
				}
			}
			if len(receipt) != len(want) {
				t.Fatalf("receipt has unexpected fields: %s", stdout)
			}
			wantLine := `web apps create: app 6759231657 already exists with bundle ID "` + ifExistsTestBundleID + `"; left unchanged (--if-exists skip)`
			if !strings.Contains(stderr, wantLine) {
				t.Fatalf("stderr = %q, want %q", stderr, wantLine)
			}
			if strings.Contains(stderr, "Created app successfully") || strings.Contains(stderr, "retrying with") {
				t.Fatalf("stderr claims a create or rename: %q", stderr)
			}
		})
	}
}

func TestRunAppsCreateIfExistsSkipAcceptsAutoRenamedNameOnlyWithAutoRename(t *testing.T) {
	// A prior run with --auto-rename may have created the app under the
	// suffixed name the CLI generates; a retry must recognize it.
	renamed := formatAppNameWithSuffix(ifExistsTestName, bundleIDNameSuffix(ifExistsTestBundleID))
	existing := appsListResponse("6759231657", renamed, ifExistsTestBundleID, ifExistsTestSKU)

	t.Run("auto-rename on", func(t *testing.T) {
		h := setupIfExistsCreate(t, capturedDuplicateAppCreate409, existing, emptyAppsResponse)
		var err error
		stdout, _ := captureOutput(t, func() {
			err = RunAppsCreate(context.Background(), ifExistsCreateOptions(shared.IfExistsSkip, true))
		})
		if err != nil {
			t.Fatalf("RunAppsCreate error: %v", err)
		}
		if !strings.Contains(stdout, `"name":"`+renamed+`"`) {
			t.Fatalf("stdout = %s, want existing renamed app", stdout)
		}
		if got := h.creates(); len(got) != 1 {
			t.Fatalf("creates = %v, want one POST", got)
		}
	})

	t.Run("auto-rename off", func(t *testing.T) {
		h := setupIfExistsCreate(t, capturedDuplicateAppCreate409, existing, emptyAppsResponse)
		var err error
		stdout, _ := captureOutput(t, func() {
			err = RunAppsCreate(context.Background(), ifExistsCreateOptions(shared.IfExistsSkip, false))
		})
		if err == nil {
			t.Fatal("expected a name mismatch to fail")
		}
		if !strings.Contains(err.Error(), capturedDuplicateAppCreateError) || !strings.Contains(err.Error(), `name "`+renamed+`"`) {
			t.Fatalf("error = %v", err)
		}
		if stdout != "" {
			t.Fatalf("stdout = %q, want empty", stdout)
		}
		if got := h.creates(); len(got) != 1 {
			t.Fatalf("creates = %v, want one POST", got)
		}
	})
}

func TestRunAppsCreateIfExistsSkipMismatchNeverAutoRenames(t *testing.T) {
	for _, tc := range []struct {
		name       string
		byBundleID string
		bySKU      string
		wantReads  int
		wantDetail string
	}{
		{
			name:       "bundle ID held by app with different SKU",
			byBundleID: appsListResponse("app-other", ifExistsTestName, ifExistsTestBundleID, "OTHER-SKU"),
			bySKU:      emptyAppsResponse,
			wantReads:  1,
			wantDetail: `SKU "OTHER-SKU"`,
		},
		{
			name:       "bundle ID held by app with different name",
			byBundleID: appsListResponse("app-other", "Someone Else", ifExistsTestBundleID, ifExistsTestSKU),
			bySKU:      emptyAppsResponse,
			wantReads:  1,
			wantDetail: `name "Someone Else"`,
		},
		{
			name:       "SKU held by app with different bundle ID",
			byBundleID: emptyAppsResponse,
			bySKU:      appsListResponse("app-sku", "Other", "com.example.other", ifExistsTestSKU),
			wantReads:  2,
			wantDetail: `SKU "` + ifExistsTestSKU + `" is already used by app app-sku`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := setupIfExistsCreate(t, syntheticDuplicateAppCreate409WithNameDetail, tc.byBundleID, tc.bySKU)

			var err error
			stdout, stderr := captureOutput(t, func() {
				err = RunAppsCreate(context.Background(), ifExistsCreateOptions(shared.IfExistsSkip, true))
			})
			if err == nil {
				t.Fatal("expected mismatch to fail")
			}
			if !strings.Contains(err.Error(), "web api error (status 409)") || !strings.Contains(err.Error(), tc.wantDetail) {
				t.Fatalf("error = %v, want 409 and %q", err, tc.wantDetail)
			}
			var apiErr *webcore.APIError
			if !errors.As(err, &apiErr) || apiErr.Status != http.StatusConflict {
				t.Fatalf("expected wrapped web 409, got %T", err)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if got := h.creates(); len(got) != 1 {
				t.Fatalf("creates = %v, want one POST and no auto-rename retry", got)
			}
			if strings.Contains(stderr, "retrying with") {
				t.Fatalf("stderr shows auto-rename: %q", stderr)
			}
			if got := h.reads(); len(got) != tc.wantReads {
				t.Fatalf("reads = %v, want %d", got, tc.wantReads)
			}
		})
	}
}

func TestRunAppsCreateIfExistsSkipNothingFoundKeepsHistoricalPath(t *testing.T) {
	t.Run("captured codes only fail unchanged", func(t *testing.T) {
		h := setupIfExistsCreate(t, capturedDuplicateAppCreate409, emptyAppsResponse, emptyAppsResponse)
		var err error
		_, _ = captureOutput(t, func() {
			err = RunAppsCreate(context.Background(), ifExistsCreateOptions(shared.IfExistsSkip, true))
		})
		if err == nil || err.Error() != capturedDuplicateAppCreateError {
			t.Fatalf("error = %v, want %q", err, capturedDuplicateAppCreateError)
		}
		if got := h.creates(); len(got) != 1 {
			t.Fatalf("creates = %v", got)
		}
		if got := h.reads(); len(got) != 2 {
			t.Fatalf("reads = %v, want bundle ID and SKU read-backs", got)
		}
	})

	t.Run("name-only conflict still auto-renames", func(t *testing.T) {
		h := setupIfExistsCreate(t, syntheticDuplicateAppCreate409WithNameDetail, emptyAppsResponse, emptyAppsResponse)
		var err error
		_, stderr := captureOutput(t, func() {
			err = RunAppsCreate(context.Background(), ifExistsCreateOptions(shared.IfExistsSkip, true))
		})
		if err != nil {
			t.Fatalf("RunAppsCreate error: %v", err)
		}
		if got := h.creates(); len(got) != 2 {
			t.Fatalf("creates = %v, want original plus one auto-rename retry", got)
		}
		if !strings.Contains(stderr, "Created app successfully (id=app-new)") {
			t.Fatalf("stderr = %q", stderr)
		}
	})
}

func TestRunAppsCreateIfExistsSkipIgnoresUnlistedConflicts(t *testing.T) {
	for _, body := range []string{
		`{"errors":[{"code":"ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE.DIFFERENT_ACCOUNT"}]}`,
		`{"errors":[{"code":"STATE_ERROR"}]}`,
		`not json`,
	} {
		t.Run(body, func(t *testing.T) {
			h := setupIfExistsCreate(t, body,
				appsListResponse("6759231657", ifExistsTestName, ifExistsTestBundleID, ifExistsTestSKU),
				emptyAppsResponse)
			var err error
			_, _ = captureOutput(t, func() {
				err = RunAppsCreate(context.Background(), ifExistsCreateOptions(shared.IfExistsSkip, false))
			})
			if err == nil {
				t.Fatal("expected unlisted 409 to fail")
			}
			if got := h.reads(); len(got) != 0 {
				t.Fatalf("unlisted 409 must not read back, got %v", got)
			}
		})
	}
}

func TestRunAppsCreateIfExistsSkipReadBackErrorIsAppended(t *testing.T) {
	h := setupIfExistsCreate(t, capturedDuplicateAppCreate409, `not json`, emptyAppsResponse)
	var err error
	_, _ = captureOutput(t, func() {
		err = RunAppsCreate(context.Background(), ifExistsCreateOptions(shared.IfExistsSkip, true))
	})
	if err == nil || !strings.HasPrefix(err.Error(), capturedDuplicateAppCreateError) || !strings.Contains(err.Error(), "read-back after conflict failed") {
		t.Fatalf("error = %v", err)
	}
	if got := h.creates(); len(got) != 1 {
		t.Fatalf("creates = %v, want no retry after a failed read-back", got)
	}
}

func TestRunAppsCreateIfExistsSkipRejectsAccessBeforeHTTP(t *testing.T) {
	origResolve := resolveAppCreateSessionFn
	origClientFactory := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		t.Fatal("did not expect ASC client lookup")
		return nil, nil
	})
	t.Cleanup(func() {
		resolveAppCreateSessionFn = origResolve
		origClientFactory()
	})
	resolveAppCreateSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		t.Fatal("did not expect web session lookup")
		return nil, "", nil
	}

	opts := ifExistsCreateOptions(shared.IfExistsSkip, true)
	opts.Access = "full"
	err := RunAppsCreate(context.Background(), opts)
	if err == nil || !strings.Contains(err.Error(), "--if-exists skip cannot be combined with --access") {
		t.Fatalf("error = %v", err)
	}
}

func TestRunAppsCreateIfExistsSkipRequiresOfficialAuthBeforeHTTP(t *testing.T) {
	origResolve := resolveAppCreateSessionFn
	origClientFactory := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		return nil, shared.ErrMissingAuth
	})
	t.Cleanup(func() {
		resolveAppCreateSessionFn = origResolve
		origClientFactory()
	})
	resolveAppCreateSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		t.Fatal("did not expect web session lookup")
		return nil, "", nil
	}

	var err error
	_, _ = captureOutput(t, func() {
		err = RunAppsCreate(context.Background(), ifExistsCreateOptions(shared.IfExistsSkip, true))
	})
	if err == nil || !strings.Contains(err.Error(), "--if-exists skip requires official App Store Connect API authentication") {
		t.Fatalf("error = %v", err)
	}
}
