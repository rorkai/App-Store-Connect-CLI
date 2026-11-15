package web

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebReviewIAPsAttachRequiresIAPID(t *testing.T) {
	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebReviewIAPsAttachCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "123456789",
		"--confirm",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	_, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		err := cmd.Exec(context.Background(), nil)
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp, got %v", err)
		}
	})
	if !strings.Contains(stderr, "--iap-id is required") {
		t.Fatalf("expected --iap-id guidance in stderr, got %q", stderr)
	}
}

func TestWebReviewIAPsAttachRequiresConfirm(t *testing.T) {
	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebReviewIAPsAttachCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "123456789",
		"--iap-id", "9000000001",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	_, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		err := cmd.Exec(context.Background(), nil)
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp, got %v", err)
		}
	})
	if !strings.Contains(stderr, "--confirm is required") {
		t.Fatalf("expected --confirm guidance in stderr, got %q", stderr)
	}
}

func TestWebReviewIAPsAttachRejectsNonNumericAppID(t *testing.T) {
	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebReviewIAPsAttachCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "com.example.app",
		"--iap-id", "9000000001",
		"--confirm",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	_, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		err := cmd.Exec(context.Background(), nil)
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp, got %v", err)
		}
	})
	if !strings.Contains(stderr, "--app must be a numeric App Store Connect app ID") {
		t.Fatalf("expected numeric --app guidance in stderr, got %q", stderr)
	}
}

func TestWebReviewIAPsAttachRejectsMatchedIAPWithoutIrisResourceID(t *testing.T) {

	origResolveSession := resolveSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					switch {
					case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/123456789/inAppPurchases":
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body: io.NopCloser(strings.NewReader(`{
								"data": [{
									"type": "inAppPurchases",
									"id": "   ",
									"attributes": {
										"productId": "com.example.lifetime",
										"referenceName": "Lifetime",
										"state": "READY_TO_SUBMIT"
									}
								}]
							}`)),
							Request: req,
						}, nil
					case req.Method == http.MethodPost:
						t.Fatal("attach POST must not run when the matched IAP has no iris resource id")
						return nil, nil
					default:
						t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
						return nil, nil
					}
				}),
			},
		}, "cache", nil
	}

	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebReviewIAPsAttachCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "123456789",
		"--iap-id", "com.example.lifetime",
		"--confirm",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	_, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		err := cmd.Exec(context.Background(), nil)
		if err == nil {
			t.Fatal("expected missing iris id error, got nil")
		}
		if !strings.Contains(err.Error(), "missing an iris resource id") {
			t.Fatalf("expected missing iris id error, got %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestWebReviewIAPsAttachRefusesIAPOutsideApp(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolveSession
	})

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					if req.Method != http.MethodGet || req.URL.Path != "/iris/v1/apps/123456789/inAppPurchases" {
						t.Fatalf("unexpected request before app-scoping refusal: %s %s", req.Method, req.URL.Path)
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body:       io.NopCloser(strings.NewReader(`{"data":[]}`)),
						Request:    req,
					}, nil
				}),
			},
		}, "cache", nil
	}

	d, _, _ := newWebTestDeps()
	cmd := newWebReviewIAPsAttachCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "123456789",
		"--iap-id", "9000000001",
		"--confirm",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	err := cmd.Exec(context.Background(), nil)
	if err == nil {
		t.Fatal("expected app-scoping error")
	}
	if !strings.Contains(err.Error(), `in-app purchase "9000000001" was not found under app "123456789"`) {
		t.Fatalf("expected app-scoping error, got %v", err)
	}
}

func TestWebReviewIAPsAttachRejectsAmbiguousSelectorBeforeMutation(t *testing.T) {

	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	postCalls := 0
	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/123456789/inAppPurchases":
					body := `{"data":[{"id":"iap-1","type":"inAppPurchases","attributes":{"productId":"com.example.duplicate","referenceName":"First"}},{"id":"iap-2","type":"inAppPurchases","attributes":{"productId":"com.example.duplicate","referenceName":"Second"}}],"links":{"next":""}}`
					return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
				case req.Method == http.MethodPost:
					postCalls++
					t.Fatalf("ambiguous selector must not attach: %s", req.URL.Path)
					return nil, nil
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
					return nil, nil
				}
			})},
		}, "cache", nil
	}

	d, _, _ := newWebTestDeps()
	cmd := newWebReviewIAPsAttachCommand(d)
	if err := cmd.FlagSet.Parse([]string{"--app", "123456789", "--iap-id", "com.example.duplicate", "--confirm"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := cmd.Exec(context.Background(), nil)
	if err == nil {
		t.Fatal("expected ambiguous selector error")
	}
	if !strings.Contains(err.Error(), `2 in-app purchases match "com.example.duplicate" by product ID; pass --iap-id with one of:`) {
		t.Fatalf("expected ambiguity diagnostic, got %v", err)
	}
	for _, want := range []string{"iap-1", "com.example.duplicate", "First", "iap-2", "Second", "Use the Iris resource ID to disambiguate"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected %q in bounded ambiguity diagnostic, got %v", want, err)
		}
	}
	diagnostic, ok := shared.DiagnosticFromError(err)
	if !ok || diagnostic.Code != shared.DiagnosticInvalidInput || diagnostic.Parameter != "--iap-id" {
		t.Fatalf("diagnostic = %+v, found=%t, want invalid_input for --iap-id", diagnostic, ok)
	}
	if postCalls != 0 {
		t.Fatalf("expected no attach request, got %d", postCalls)
	}
}

func TestReviewIAPAmbiguousSelectionBoundsProviderText(t *testing.T) {
	recoveryID := "iap-recovery-id-" + strings.Repeat("9", shared.AmbiguousDiagnosticTextLimit+32)
	providerText := strings.Repeat("界", shared.AmbiguousDiagnosticTextLimit)
	providerInvalidUTF8 := string([]byte{'r', 'e', 'f', 0xff, 'e', 'r', 'e', 'n', 'c', 'e'})
	err := reviewIAPAmbiguousSelectionError(&webcore.ReviewIAPAmbiguousError{
		Selector: "com.example.duplicate",
		Field:    "product ID",
		Matches: []webcore.ReviewIAP{
			{ID: recoveryID, ProductID: providerText + "\nproduct-tail", ReferenceName: providerInvalidUTF8},
			{ID: "iap-2", ProductID: "com.example.duplicate", ReferenceName: "Second"},
		},
	})
	if err == nil {
		t.Fatal("expected an ambiguity error")
	}
	message := err.Error()
	if !utf8.ValidString(message) {
		t.Fatalf("ambiguity diagnostic must remain valid UTF-8: %q", message)
	}
	if strings.Contains(message, recoveryID) {
		t.Fatalf("displayed recovery ID should be bounded: %q", message)
	}
	if !strings.Contains(message, recoveryID[:len("iap-recovery-id-")]) {
		t.Fatalf("bounded recovery ID should retain useful context: %q", message)
	}
	if strings.Contains(message, "product-tail") || strings.Contains(message, "\x1b") || strings.Contains(message, "\nref") {
		t.Fatalf("provider text must be bounded and terminal-safe: %q", message)
	}
	if !strings.Contains(message, "ref�erence") {
		t.Fatalf("invalid provider UTF-8 should be replaced while retaining context: %q", message)
	}
	var structured *shared.AmbiguousSelectionError
	if !errors.As(err, &structured) || len(structured.Candidates) != 2 || structured.Candidates[0].ID != recoveryID {
		t.Fatalf("structured ambiguity must retain exact recovery candidates: %#v", structured)
	}
	if structured.Candidates[0].Label != providerText+"\nproduct-tail" || structured.Candidates[0].Extra != providerInvalidUTF8 {
		t.Fatalf("structured ambiguity must retain provider fields for machine consumers: %#v", structured.Candidates[0])
	}
	if structured.Description != `"com.example.duplicate" by product ID` || structured.Flag != "--iap-id" {
		t.Fatalf("structured ambiguity lost selector semantics: %#v", structured)
	}
}

func TestWebReviewIAPsAttachRejectsMissingOrWrongTypeBeforeMutation(t *testing.T) {

	cases := []struct {
		name    string
		body    string
		v2Body  string
		v2Error bool
		want    string
	}{
		{
			name: "missing",
			body: `{"data":[],"links":{"next":""}}`,
			want: `was not found under app`,
		},
		{
			name: "wrong type",
			body: `{"data":[{"id":"sub-1","type":"subscriptions","attributes":{"productId":"com.example.wrong"}}],"links":{"next":""}}`,
			want: `unexpected resource type`,
		},
		{name: "v2 empty", v2Body: `{"data":[]}`, want: "no V2 in-app purchase"},
		{name: "v2 product mismatch", v2Body: `{"data":[{"type":"inAppPurchases","id":"9000000001","attributes":{"productId":"com.example.other"}}]}`, want: "no V2 in-app purchase"},
		{name: "v2 ambiguous", v2Body: `{"data":[{"type":"inAppPurchases","id":"9000000001","attributes":{"productId":"com.example.selected"}},{"type":"inAppPurchases","id":"9000000002","attributes":{"productId":"com.example.selected"}}]}`, want: "multiple V2 in-app purchases"},
		{name: "v2 wrong type", v2Body: `{"data":[{"type":"subscriptions","id":"9000000001","attributes":{"productId":"com.example.selected"}}]}`, want: "unexpected resource type"},
		{name: "v2 missing id", v2Body: `{"data":[{"type":"inAppPurchases","attributes":{"productId":"com.example.selected"}}]}`, want: "missing resource id"},
		{name: "v2 read error", v2Error: true, want: "V2 catalog unavailable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			origResolveSession := resolveSessionFn
			t.Cleanup(func() { resolveSessionFn = origResolveSession })

			postCalls := 0
			resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
				return &webcore.AuthSession{
					Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
						if req.Method == http.MethodPost {
							postCalls++
							t.Fatalf("%s selector must not attach: %s", tc.name, req.URL.Path)
						}
						body := tc.body
						if req.Method != http.MethodGet {
							t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
						}
						switch req.URL.Path {
						case "/iris/v1/apps/123456789/inAppPurchases":
							if tc.v2Body != "" || tc.v2Error {
								body = `{"data":[{"type":"inAppPurchases","id":"legacy-id","attributes":{"productId":"com.example.selected"}}]}`
							}
						case "/iris/v1/apps/123456789/inAppPurchasesV2":
							if tc.v2Error {
								return nil, errors.New("V2 catalog unavailable")
							}
							body = tc.v2Body
						default:
							t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
						}
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(body)),
							Request:    req,
						}, nil
					})},
				}, "cache", nil
			}

			selector := tc.name
			if tc.v2Body != "" || tc.v2Error {
				selector = "legacy-id"
			}
			d, _, _ := newWebTestDeps()
			cmd := newWebReviewIAPsAttachCommand(d)
			if err := cmd.FlagSet.Parse([]string{"--app", "123456789", "--iap-id", selector, "--confirm"}); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			err := cmd.Exec(context.Background(), nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q error, got %v", tc.want, err)
			}
			if postCalls != 0 {
				t.Fatalf("expected no attach request, got %d", postCalls)
			}
		})
	}
}

func TestWebReviewIAPsGroupCommandReturnsHelpWhenNoSubcommand(t *testing.T) {
	d, _, _ := newWebTestDeps()
	cmd := newWebReviewIAPsCommand(d)
	if cmd.UsageFunc == nil {
		t.Fatal("WebReviewIAPsCommand should set UsageFunc for consistent rendering")
	}
	err := cmd.Exec(context.Background(), nil)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp from group Exec with no subcommand, got %v", err)
	}
}

func TestWebReviewIAPsAttachDraftSelectionAndMembership(t *testing.T) {
	const drafts = `[{"type":"reviewSubmissions","id":"draft-1","attributes":{"state":"READY_FOR_REVIEW"}}]`
	const twoDrafts = `[{"type":"reviewSubmissions","id":"draft-1","attributes":{"state":"READY_FOR_REVIEW"}},{"type":"reviewSubmissions","id":"draft-2","attributes":{"state":"READY_FOR_REVIEW"}}]`
	const versions = `[{"type":"inAppPurchaseVersions","id":"version-1","attributes":{"state":"PREPARE_FOR_SUBMISSION","version":1}}]`
	const item = `{"type":"reviewSubmissionItems","id":"item-1","attributes":{"state":"READY_FOR_REVIEW"},"relationships":{"inAppPurchaseVersion":{"data":{"type":"inAppPurchaseVersions","id":"version-1"}}}}`
	const alreadyExists = `{"errors":[{"code":"ENTITY_ERROR.ATTRIBUTE.INVALID.ALREADY_EXISTS","title":"Already exists"}]}`
	tests := []struct {
		name, drafts, versions, items, afterItems, submission, postError, wantErr string
		wantPosts                                                                 int
		wantChanged                                                               bool
	}{
		{name: "explicit draft among several", drafts: twoDrafts, submission: "draft-1", wantPosts: 1, wantChanged: true},
		{name: "ambiguous drafts", drafts: twoDrafts, wantErr: "multiple READY_FOR_REVIEW drafts; select one with --submission"},
		{name: "wrong app draft", submission: "other-app-draft", wantErr: "--submission must identify a READY_FOR_REVIEW draft under --app"},
		{name: "noneditable draft", drafts: strings.ReplaceAll(drafts, "READY_FOR_REVIEW", "IN_REVIEW"), submission: "draft-1", wantErr: "--submission must identify a READY_FOR_REVIEW draft under --app"},
		{name: "no draft", drafts: "[]", wantErr: "no READY_FOR_REVIEW draft"},
		{name: "no editable version", versions: strings.ReplaceAll(versions, "PREPARE_FOR_SUBMISSION", "APPROVED"), wantErr: "no editable version"},
		{name: "ambiguous versions", versions: strings.TrimSuffix(versions, "]") + `,` + strings.TrimPrefix(strings.ReplaceAll(versions, "version-1", "version-2"), "["), wantErr: "multiple editable versions"},
		{name: "invalid version identity", versions: strings.ReplaceAll(versions, "inAppPurchaseVersions", "inAppPurchases"), wantErr: "invalid resource identity"},
		{name: "already in selected draft", versions: strings.ReplaceAll(versions, "PREPARE_FOR_SUBMISSION", "READY_FOR_REVIEW"), items: "[" + item + "]"},
		{name: "removed item is not attached", items: "[" + strings.ReplaceAll(item, "READY_FOR_REVIEW", "REMOVED") + "]", wantPosts: 1, wantChanged: true},
		{name: "unrelated item is not attached", items: "[" + strings.ReplaceAll(item, "inAppPurchaseVersion", "subscriptionVersion") + "]", wantPosts: 1, wantChanged: true},
		{name: "matching item missing state", items: "[" + strings.ReplaceAll(item, `"state":"READY_FOR_REVIEW"`, `"state":""`) + "]", wantErr: "incomplete identity or state"},
		{name: "duplicate conflict verified", postError: alreadyExists, wantPosts: 1},
		{name: "duplicate conflict not verified", postError: alreadyExists, afterItems: "[]", wantPosts: 1, wantErr: "status 409"},
		{name: "unknown conflict remains error", postError: `{"errors":[{"code":"STATE_ERROR.FIRST_NON_CONSUMABLE_MUST_BE_SUBMITTED_ON_VERSION","title":"First IAP requires app version"}]}`, wantPosts: 1, wantErr: "FIRST_NON_CONSUMABLE_MUST_BE_SUBMITTED_ON_VERSION"},
		{name: "created item missing on readback", afterItems: "[]", wantPosts: 1, wantErr: "was created but is absent"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			original := resolveSessionFn
			t.Cleanup(func() { resolveSessionFn = original })
			posts := 0
			resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
				return &webcore.AuthSession{Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					status, body := http.StatusOK, ""
					switch req.Method + " " + req.URL.Path {
					case "GET /iris/v1/apps/123456789/inAppPurchases":
						body = `{"data":[{"type":"inAppPurchases","id":"legacy-1","attributes":{"productId":"com.example.pro","state":"WAITING_FOR_REVIEW"}}]}`
					case "GET /iris/v1/apps/123456789/inAppPurchasesV2":
						body = `{"data":[{"type":"inAppPurchases","id":"9000000001","attributes":{"productId":"com.example.pro"}}]}`
					case "GET /iris/v2/inAppPurchases/9000000001/versions":
						data := tc.versions
						if data == "" {
							data = versions
						}
						body = `{"data":` + data + `}`
					case "GET /iris/v1/apps/123456789/reviewSubmissions":
						data := tc.drafts
						if data == "" {
							data = drafts
						}
						body = `{"data":` + data + `}`
					case "GET /iris/v1/reviewSubmissions/draft-1/items":
						data := tc.items
						if data == "" {
							data = "[]"
						}
						if posts > 0 {
							data = tc.afterItems
							if data == "" {
								data = "[" + item + "]"
							}
						}
						body = `{"data":` + data + `}`
					case "POST /iris/v1/reviewSubmissionItems":
						posts++
						status = http.StatusCreated
						body = `{"data":` + item + `}`
						if tc.postError != "" {
							status = http.StatusConflict
							body = tc.postError
						}
					default:
						t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
					}
					return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
				})}}, "cache", nil
			}
			d, depsStdout, depsStderr := newWebTestDeps()
			cmd := newWebReviewIAPsAttachCommand(d)
			args := []string{"--app", "123456789", "--iap-id", "com.example.pro", "--confirm", "--output", "json"}
			if tc.submission != "" {
				args = append(args, "--submission", tc.submission)
			}
			if err := cmd.FlagSet.Parse(args); err != nil {
				t.Fatal(err)
			}
			var runErr error
			stdout, _ := runWithWebTestDeps(depsStdout, depsStderr, func() { runErr = cmd.Exec(context.Background(), nil) })
			if posts != tc.wantPosts {
				t.Fatalf("POST count=%d, want %d", posts, tc.wantPosts)
			}
			if tc.wantErr != "" {
				if runErr == nil || !strings.Contains(runErr.Error(), tc.wantErr) {
					t.Fatalf("error=%v, want %q", runErr, tc.wantErr)
				}
				if stdout != "" {
					t.Fatalf("failure emitted receipt: %s", stdout)
				}
				return
			}
			if runErr != nil {
				t.Fatal(runErr)
			}
			var result reviewIAPMutationOutput
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatal(err)
			}
			if result.Changed != tc.wantChanged || result.Submission.ID != "" || result.Submission.ReviewSubmissionItemID != "item-1" || result.Submission.ReviewSubmissionID != "draft-1" || result.Submission.InAppPurchaseVersionID != "version-1" || result.Submission.SubmitWithNextAppStoreVersion {
				t.Fatalf("unexpected receipt: %#v", result)
			}
		})
	}
}
