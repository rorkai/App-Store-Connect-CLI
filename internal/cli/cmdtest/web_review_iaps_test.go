package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebReviewIAPsAttachRootSuccess(t *testing.T) {
	setupCachedWebReviewIAPSession(t, "user@example.com")
	requests := newRequestLog(10)
	attached := false
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests.Add(req.Method + " " + req.URL.Path)
		body := ""
		status := http.StatusOK
		switch req.Method + " " + req.URL.Path {
		case "GET /olympus/v1/session":
			body = `{"provider":{"providerId":123456,"publicProviderId":"team-1","name":"Team"},"user":{"emailAddress":"user@example.com"}}`
		case "GET /iris/v1/apps/123456789/inAppPurchases":
			body = `{"data":[{"type":"inAppPurchases","id":"legacy-iap","attributes":{"productId":"com.example.removeads","referenceName":"Remove Ads","state":"READY_TO_SUBMIT"}}]}`
		case "GET /iris/v1/apps/123456789/inAppPurchasesV2":
			if req.URL.Query().Get("page[cursor]") == "next" {
				body = `{"data":[{"type":"inAppPurchases","id":"9000000001","attributes":{"productId":"com.example.removeads"}}]}`
			} else {
				body = `{"data":[],"links":{"next":"/iris/v1/apps/123456789/inAppPurchasesV2?page%5Bcursor%5D=next"}}`
			}
		case "GET /iris/v2/inAppPurchases/9000000001/versions":
			if req.URL.Query().Get("page[cursor]") == "next" {
				body = `{"data":[{"type":"inAppPurchaseVersions","id":"iap-version","attributes":{"state":"PREPARE_FOR_SUBMISSION","version":2}}]}`
			} else {
				if req.URL.Query().Get("limit") != "200" {
					t.Fatalf("unexpected versions query: %s", req.URL.RawQuery)
				}
				body = `{"data":[{"type":"inAppPurchaseVersions","id":"old-version","attributes":{"state":"APPROVED","version":1}}],"links":{"next":"https://appstoreconnect.apple.com/iris/v2/inAppPurchases/9000000001/versions?page%5Bcursor%5D=next"}}`
			}
		case "GET /iris/v1/apps/123456789/reviewSubmissions":
			body = `{"data":[{"type":"reviewSubmissions","id":"draft-1","attributes":{"state":"READY_FOR_REVIEW"},"relationships":{"appStoreVersionForReview":{"data":null}}}]}`
		case "GET /iris/v1/reviewSubmissions/draft-1/items":
			body = `{"data":[]}`
			if attached {
				body = `{"data":[{"type":"reviewSubmissionItems","id":"item-1","attributes":{"state":"READY_FOR_REVIEW"},"relationships":{"inAppPurchaseVersion":{"data":{"type":"inAppPurchaseVersions","id":"iap-version"}}}}]}`
			}
		case "POST /iris/v1/reviewSubmissionItems":
			var got map[string]any
			if err := json.NewDecoder(req.Body).Decode(&got); err != nil {
				t.Fatal(err)
			}
			var want map[string]any
			if err := json.Unmarshal([]byte(`{"data":{"type":"reviewSubmissionItems","relationships":{"reviewSubmission":{"data":{"type":"reviewSubmissions","id":"draft-1"}},"inAppPurchaseVersion":{"data":{"type":"inAppPurchaseVersions","id":"iap-version"}}}}}`), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("attach payload = %#v, want %#v", got, want)
			}
			attached = true
			status = http.StatusCreated
			body = `{"data":{"type":"reviewSubmissionItems","id":"item-1","attributes":{"state":"READY_FOR_REVIEW"},"relationships":{"inAppPurchaseVersion":{"data":{"type":"inAppPurchaseVersions","id":"iap-version"}}}}}`
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
		}
		return webReviewIAPJSONResponse(status, body, req), nil
	}))
	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{"web", "review", "iaps", "attach", "--app", "123456789", "--iap-id", "com.example.removeads", "--confirm", "--apple-id", "user@example.com", "--output", "json"}, "1.2.3")
	})
	if code != rootcmd.ExitSuccess || stderr != "" {
		t.Fatalf("exit=%d stderr=%q stdout=%q", code, stderr, stdout)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	var want map[string]any
	if err := json.Unmarshal([]byte(`{"appId":"123456789","iapId":"com.example.removeads","operation":"attach","changed":true,"submission":{"id":"","reviewSubmissionItemId":"item-1","inAppPurchaseId":"9000000001","reviewSubmissionId":"draft-1","inAppPurchaseVersionId":"iap-version","submitWithNextAppStoreVersion":false}}`), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("receipt = %#v, want %#v", got, want)
	}
	wantRequests := []string{
		"GET /olympus/v1/session",
		"GET /iris/v1/apps/123456789/inAppPurchases",
		"GET /iris/v1/apps/123456789/inAppPurchasesV2",
		"GET /iris/v1/apps/123456789/inAppPurchasesV2",
		"GET /iris/v2/inAppPurchases/9000000001/versions",
		"GET /iris/v2/inAppPurchases/9000000001/versions",
		"GET /iris/v1/apps/123456789/reviewSubmissions",
		"GET /iris/v1/reviewSubmissions/draft-1/items",
		"POST /iris/v1/reviewSubmissionItems",
		"GET /iris/v1/reviewSubmissions/draft-1/items",
	}
	if got := requests.Snapshot(); !reflect.DeepEqual(got, wantRequests) {
		t.Fatalf("requests = %v, want %v", got, wantRequests)
	}
}

func TestWebReviewIAPsAttachValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", t.TempDir()+"/config.json")
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name:       "missing app",
			args:       []string{"web", "review", "iaps", "attach", "--iap-id", "9000000001", "--confirm"},
			wantStderr: "--app is required",
		},
		{
			name:       "missing iap id",
			args:       []string{"web", "review", "iaps", "attach", "--app", "123456789", "--confirm"},
			wantStderr: "--iap-id is required",
		},
		{
			name:       "missing confirm",
			args:       []string{"web", "review", "iaps", "attach", "--app", "123456789", "--iap-id", "9000000001"},
			wantStderr: "--confirm is required",
		},
		{
			name:       "non numeric app",
			args:       []string{"web", "review", "iaps", "attach", "--app", "com.example.app", "--iap-id", "9000000001", "--confirm"},
			wantStderr: "--app must be a numeric App Store Connect app ID",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantStderr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantStderr, stderr)
			}
		})
	}
}

func TestWebReviewIAPsAttachArgumentParsingEdges(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "root flag before subcommand",
			args: []string{
				"--profile", "ci",
				"web", "review", "iaps", "attach",
				"--app", "123456789",
				"--iap-id", "9000000001",
				"--apple-id", "attach",
				"--submission", "draft-1",
				"--confirm",
			},
		},
		{
			name: "mixed flag order",
			args: []string{
				"web", "review", "iaps", "attach",
				"--iap-id", "9000000001",
				"--apple-id", "attach",
				"--app", "123456789",
				"--confirm",
			},
		},
		{
			name: "flag value matching subcommand",
			args: []string{
				"web", "review", "iaps", "attach",
				"--app", "123456789",
				"--iap-id", "9000000001",
				"--apple-id", "attach",
				"--submission", "draft-1",
				"--confirm",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupCachedWebReviewIAPSession(t, "attach")
			installSuccessfulWebReviewIAPAttachTransport(t, "attach")

			var code int
			stdout, stderr := captureOutput(t, func() {
				code = rootcmd.Run(test.args, "1.2.3")
			})
			if code != rootcmd.ExitSuccess {
				t.Fatalf("exit code = %d, want %d\nstderr=%s", code, rootcmd.ExitSuccess, stderr)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			var payload struct {
				Operation string `json:"operation"`
			}
			if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
				t.Fatalf("expected valid JSON output, parse error: %v\nstdout=%q", err, stdout)
			}
			if payload.Operation != "attach" {
				t.Fatalf("expected operation=attach, got %#v", payload)
			}
		})
	}
}

func TestWebReviewIAPsAttachInvalidValueExitCodes(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name:       "invalid app selector",
			args:       []string{"web", "review", "iaps", "attach", "--app", "com.example.app", "--iap-id", "9000000001", "--confirm"},
			wantStderr: "--app must be a numeric App Store Connect app ID",
		},
		{
			name:       "blank explicit submission",
			args:       []string{"web", "review", "iaps", "attach", "--app", "123456789", "--iap-id", "iap-1", "--submission", "   ", "--confirm"},
			wantStderr: "--submission must not be blank",
		},
		{
			name:       "invalid confirm value",
			args:       []string{"web", "review", "iaps", "attach", "--app", "123456789", "--iap-id", "9000000001", "--confirm=maybe"},
			wantStderr: `invalid boolean value "maybe" for --confirm`,
		},
		{
			name:       "blank iap selector",
			args:       []string{"web", "review", "iaps", "attach", "--app", "123456789", "--iap-id", "   ", "--confirm"},
			wantStderr: "--iap-id is required",
		},
		{
			name:       "subcommand flag before attach rejected",
			args:       []string{"web", "review", "iaps", "--app", "123456789", "attach", "--iap-id", "9000000001", "--confirm"},
			wantStderr: "Error: unknown flag `--app` for `asc web review iaps`",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExit(t, test.args, test.wantStderr)
		})
	}
}

func setupCachedWebReviewIAPSession(t *testing.T, email string) {
	t.Helper()
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", t.TempDir()+"/config.json")

	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_STRICT_AUTH", "")
	t.Setenv("ASC_WEB_SESSION_CACHE", "1")
	t.Setenv("ASC_WEB_SESSION_CACHE_BACKEND", "file")
	t.Setenv("ASC_WEB_SESSION_CACHE_DIR", t.TempDir())

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create cookie jar: %v", err)
	}
	appStoreURL, err := url.Parse("https://appstoreconnect.apple.com/")
	if err != nil {
		t.Fatalf("parse app store URL: %v", err)
	}
	jar.SetCookies(appStoreURL, []*http.Cookie{
		{
			Name:    "myacinfo",
			Value:   "test-token",
			Path:    "/",
			Domain:  "appstoreconnect.apple.com",
			Expires: time.Now().Add(time.Hour),
		},
	})
	if err := webcore.PersistSession(&webcore.AuthSession{
		Client:    &http.Client{Jar: jar},
		UserEmail: email,
	}); err != nil {
		t.Fatalf("persist cached web session: %v", err)
	}
}

func installSuccessfulWebReviewIAPAttachTransport(t *testing.T, email string) {
	t.Helper()
	attached := false
	const item = `{"type":"reviewSubmissionItems","id":"item-1","attributes":{"state":"READY_FOR_REVIEW"},"relationships":{"inAppPurchaseVersion":{"data":{"type":"inAppPurchaseVersions","id":"version-1"}}}}`
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, status := "", http.StatusOK
		switch req.Method + " " + req.URL.Path {
		case "GET /olympus/v1/session":
			body = `{"provider":{"providerId":123456,"publicProviderId":"team-1","name":"Team"},"user":{"emailAddress":"` + email + `"}}`
		case "GET /iris/v1/apps/123456789/inAppPurchases":
			body = `{"data":[{"type":"inAppPurchases","id":"9000000001","attributes":{"productId":"com.example.removeads","referenceName":"Remove Ads","state":"READY_TO_SUBMIT"}}]}`
		case "GET /iris/v1/apps/123456789/inAppPurchasesV2":
			body = `{"data":[{"type":"inAppPurchases","id":"9000000001","attributes":{"productId":"com.example.removeads"}}]}`
		case "GET /iris/v2/inAppPurchases/9000000001/versions":
			body = `{"data":[{"type":"inAppPurchaseVersions","id":"version-1","attributes":{"state":"PREPARE_FOR_SUBMISSION","version":1}}]}`
		case "GET /iris/v1/apps/123456789/reviewSubmissions":
			body = `{"data":[{"type":"reviewSubmissions","id":"draft-1","attributes":{"state":"READY_FOR_REVIEW"}}]}`
		case "GET /iris/v1/reviewSubmissions/draft-1/items":
			body = `{"data":[]}`
			if attached {
				body = `{"data":[` + item + `]}`
			}
		case "POST /iris/v1/reviewSubmissionItems":
			attached, status, body = true, http.StatusCreated, `{"data":`+item+`}`
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
		}
		return webReviewIAPJSONResponse(status, body, req), nil
	}))
}

func webReviewIAPJSONResponse(status int, body string, req *http.Request) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}
