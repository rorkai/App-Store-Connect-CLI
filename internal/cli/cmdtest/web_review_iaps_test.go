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
	"strings"
	"testing"
	"time"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebReviewIAPsAttachRootSuccess(t *testing.T) {
	setupCachedWebReviewIAPSession(t, "user@example.com")

	const (
		irisResourceID = "11111111-2222-3333-4444-555555555555"
		productID      = "com.example.removeads"
	)

	requests := newRequestLog(3)
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests.Add(req.Method + " " + req.URL.Host + req.URL.Path)
		switch {
		case req.Method == http.MethodGet &&
			req.URL.Host == "appstoreconnect.apple.com" &&
			req.URL.Path == "/olympus/v1/session":
			return webReviewIAPJSONResponse(http.StatusOK, `{
				"provider": {
					"providerId": 123456,
					"publicProviderId": "team-1",
					"name": "Team"
				},
				"user": {
					"emailAddress": "user@example.com"
				}
			}`, req), nil
		case req.Method == http.MethodGet &&
			req.URL.Host == "appstoreconnect.apple.com" &&
			req.URL.Path == "/iris/v1/apps/123456789/inAppPurchases":
			if got := req.URL.Query().Get("limit"); got != "300" {
				t.Fatalf("expected web IAP lookup limit=300, got %q", got)
			}
			return webReviewIAPJSONResponse(http.StatusOK, `{
					"data": [{
						"type": "inAppPurchases",
						"id": "`+irisResourceID+`",
						"attributes": {
							"productId": "`+productID+`",
							"referenceName": "Remove Ads",
							"state": "READY_TO_SUBMIT"
						}
					}]
				}`, req), nil
		case req.Method == http.MethodPost &&
			req.URL.Host == "appstoreconnect.apple.com" &&
			req.URL.Path == "/iris/v1/inAppPurchaseSubmissions":
			bodyBytes, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			var post struct {
				Data struct {
					Attributes struct {
						SubmitWithNextAppStoreVersion bool `json:"submitWithNextAppStoreVersion"`
					} `json:"attributes"`
					Relationships struct {
						InAppPurchaseV2 struct {
							Data struct {
								ID string `json:"id"`
							} `json:"data"`
						} `json:"inAppPurchaseV2"`
					} `json:"relationships"`
				} `json:"data"`
			}
			if err := json.Unmarshal(bodyBytes, &post); err != nil {
				t.Fatalf("decode request body: %v; body=%s", err, bodyBytes)
			}
			if got := post.Data.Relationships.InAppPurchaseV2.Data.ID; got != irisResourceID {
				t.Fatalf("expected request body relationship id %q, got %q", irisResourceID, got)
			}
			if got := post.Data.Relationships.InAppPurchaseV2.Data.ID; got == productID {
				t.Fatalf("request body must not use productId as relationship id; body=%s", bodyBytes)
			}
			if !post.Data.Attributes.SubmitWithNextAppStoreVersion {
				t.Fatalf("expected request body to submit with next app store version; body=%s", bodyBytes)
			}
			return webReviewIAPJSONResponse(http.StatusCreated, `{
					"data": {
					"type": "inAppPurchaseSubmissions",
					"id": "submission-1",
					"attributes": {
						"submitWithNextAppStoreVersion": true
						},
						"relationships": {
							"inAppPurchaseV2": {
								"data": {"type": "inAppPurchases", "id": "`+irisResourceID+`"}
							}
						}
					}
			}`, req), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"web", "review", "iaps", "attach",
			"--app", "123456789",
			"--iap-id", productID,
			"--confirm",
			"--apple-id", "user@example.com",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr != nil {
		t.Fatalf("run error: %v\nstderr=%s", runErr, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		AppID      string `json:"appId"`
		IAPID      string `json:"iapId"`
		Operation  string `json:"operation"`
		Changed    bool   `json:"changed"`
		Submission struct {
			ID              string `json:"id"`
			InAppPurchaseID string `json:"inAppPurchaseId"`
		} `json:"submission"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to parse stdout JSON: %v\nstdout=%s", err, stdout)
	}
	if payload.AppID != "123456789" || payload.IAPID != productID || payload.Operation != "attach" || !payload.Changed {
		t.Fatalf("unexpected payload: %#v", payload)
	}
	if payload.Submission.ID != "submission-1" || payload.Submission.InAppPurchaseID != irisResourceID {
		t.Fatalf("unexpected submission: %#v", payload.Submission)
	}

	wantRequests := []string{
		"GET appstoreconnect.apple.com/olympus/v1/session",
		"GET appstoreconnect.apple.com/iris/v1/apps/123456789/inAppPurchases",
		"POST appstoreconnect.apple.com/iris/v1/inAppPurchaseSubmissions",
	}
	if got := requests.Snapshot(); strings.Join(got, "\n") != strings.Join(wantRequests, "\n") {
		t.Fatalf("unexpected requests:\nwant=%v\ngot=%v", wantRequests, got)
	}
}

func TestWebReviewIAPsAttachValidationErrors(t *testing.T) {
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
			name:       "invalid confirm value",
			args:       []string{"web", "review", "iaps", "attach", "--app", "123456789", "--iap-id", "9000000001", "--confirm=maybe"},
			wantStderr: `invalid boolean value "maybe" for -confirm`,
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

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet &&
			req.URL.Host == "appstoreconnect.apple.com" &&
			req.URL.Path == "/olympus/v1/session":
			return webReviewIAPJSONResponse(http.StatusOK, `{
				"provider": {
					"providerId": 123456,
					"publicProviderId": "team-1",
					"name": "Team"
				},
				"user": {
					"emailAddress": "`+email+`"
				}
			}`, req), nil
		case req.Method == http.MethodGet &&
			req.URL.Host == "appstoreconnect.apple.com" &&
			req.URL.Path == "/iris/v1/apps/123456789/inAppPurchases":
			return webReviewIAPJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "inAppPurchases",
					"id": "9000000001",
					"attributes": {
						"name": "Remove Ads",
						"productId": "com.example.removeads",
						"inAppPurchaseType": "NON_CONSUMABLE",
						"state": "READY_TO_SUBMIT"
					}
				}]
			}`, req), nil
		case req.Method == http.MethodPost &&
			req.URL.Host == "appstoreconnect.apple.com" &&
			req.URL.Path == "/iris/v1/inAppPurchaseSubmissions":
			return webReviewIAPJSONResponse(http.StatusCreated, `{
				"data": {
					"type": "inAppPurchaseSubmissions",
					"id": "submission-1",
					"attributes": {
						"submitWithNextAppStoreVersion": true
					},
					"relationships": {
						"inAppPurchaseV2": {
							"data": {"type": "inAppPurchases", "id": "9000000001"}
						}
					}
				}
			}`, req), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
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
