package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewSubmitValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing app",
			args:    []string{"review", "submit", "--version", "1.2.3", "--build", "build-1", "--confirm"},
			wantErr: "--app is required",
		},
		{
			name:    "missing build",
			args:    []string{"review", "submit", "--app", "app-1", "--version", "1.2.3", "--confirm"},
			wantErr: "--build is required",
		},
		{
			name:    "missing version selector",
			args:    []string{"review", "submit", "--app", "app-1", "--build", "build-1", "--confirm"},
			wantErr: "--version or --version-id is required",
		},
		{
			name:    "conflicting version selectors",
			args:    []string{"review", "submit", "--app", "app-1", "--version", "1.2.3", "--version-id", "version-1", "--build", "build-1", "--confirm"},
			wantErr: "--version and --version-id are mutually exclusive",
		},
		{
			name:    "missing confirm",
			args:    []string{"review", "submit", "--app", "app-1", "--version", "1.2.3", "--build", "build-1"},
			wantErr: "--confirm is required unless --dry-run is set",
		},
		{
			name:    "invalid platform",
			args:    []string{"review", "submit", "--app", "app-1", "--version", "1.2.3", "--build", "build-1", "--platform", "watchos", "--confirm"},
			wantErr: "--platform must be one of",
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

func TestReviewSubmitLocalizationPreflightUsesReviewSubmitGuidance(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1":
			if req.URL.Query().Get("include") != "app" {
				t.Fatalf("expected include=app, got %q", req.URL.Query().Get("include"))
			}
			return jsonResponse(http.StatusOK, `{
				"data":{
					"type":"appStoreVersions",
					"id":"version-1",
					"attributes":{"platform":"IOS","versionString":"1.2.3"},
					"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}
				},
				"included":[{"type":"apps","id":"app-1","attributes":{"bundleId":"app-1","name":"App One"}}]
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			if !strings.Contains(req.URL.RawQuery, "filter%5BappStoreState%5D=") {
				t.Fatalf("expected appStoreState filter, got %q", req.URL.RawQuery)
			}
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionSubmission":
			return jsonResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		default:
			t.Fatalf("unexpected request during localization preflight: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"review", "submit",
			"--app", "app-1",
			"--version-id", "version-1",
			"--build", "build-1",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr == nil {
		t.Fatal("expected localization preflight error, got nil")
	}
	if !strings.Contains(runErr.Error(), "review submit: submit preflight failed") {
		t.Fatalf("expected review submit error prefix, got %v", runErr)
	}
	if strings.Contains(runErr.Error(), "submit create:") {
		t.Fatalf("did not expect removed submit create prefix, got %v", runErr)
	}
	if !strings.Contains(stderr, "before retrying `asc review submit`") {
		t.Fatalf("expected retry guidance for review submit, got %q", stderr)
	}
	if strings.Contains(stderr, "submit create") {
		t.Fatalf("did not expect removed submit create guidance, got %q", stderr)
	}
}

func TestReviewSubmitSubscriptionPreflightUsesReviewSubmitGuidance(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1":
			if req.URL.Query().Get("include") != "app" {
				t.Fatalf("expected include=app, got %q", req.URL.Query().Get("include"))
			}
			return jsonResponse(http.StatusOK, `{
				"data":{
					"type":"appStoreVersions",
					"id":"version-1",
					"attributes":{"platform":"IOS","versionString":"1.2.3"},
					"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}
				},
				"included":[{"type":"apps","id":"app-1","attributes":{"bundleId":"app-1","name":"App One"}}]
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			if !strings.Contains(req.URL.RawQuery, "filter%5BappStoreState%5D=") {
				t.Fatalf("expected appStoreState filter, got %q", req.URL.RawQuery)
			}
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/subscriptionGroups":
			return jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Premium"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionGroups/group-1/subscriptions":
			return jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptions","id":"sub-1","attributes":{"name":"Monthly","state":"READY_TO_SUBMIT"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/build":
			return jsonResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionSubmission":
			return jsonResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		default:
			t.Fatalf("unexpected request during subscription preflight: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"review", "submit",
			"--app", "app-1",
			"--version-id", "version-1",
			"--build", "build-1",
			"--dry-run",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if !strings.Contains(stdout, `"wouldSubmit":true`) {
		t.Fatalf("expected successful dry-run output, got %q", stdout)
	}
	if !strings.Contains(stderr, "before retrying `asc review submit`") {
		t.Fatalf("expected retry guidance for review submit, got %q", stderr)
	}
	if strings.Contains(stderr, "submit create") {
		t.Fatalf("did not expect removed submit create guidance, got %q", stderr)
	}
}

func TestReviewSubmitDryRunPreviewsWithoutMutations(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	requests := newRequestLog(20)
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests.Add(req.Method + " " + req.URL.Path)

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1":
			if req.URL.Query().Get("include") != "app" {
				t.Fatalf("expected include=app, got %q", req.URL.Query().Get("include"))
			}
			return jsonResponse(http.StatusOK, `{
				"data":{
					"type":"appStoreVersions",
					"id":"version-1",
					"attributes":{"platform":"IOS","versionString":"1.2.3"},
					"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}
				},
				"included":[{"type":"apps","id":"app-1","attributes":{"bundleId":"app-1","name":"App One"}}]
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			if !strings.Contains(req.URL.RawQuery, "filter%5BappStoreState%5D=") {
				t.Fatalf("expected appStoreState filter, got %q", req.URL.RawQuery)
			}
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/subscriptionGroups":
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/build":
			return jsonResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionSubmission":
			return jsonResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		default:
			t.Fatalf("unexpected request during dry-run: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"review", "submit",
			"--app", "app-1",
			"--version-id", "version-1",
			"--build", "build-1",
			"--dry-run",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}

	var payload struct {
		AppID            string `json:"appId"`
		VersionID        string `json:"versionId"`
		BuildID          string `json:"buildId"`
		Platform         string `json:"platform"`
		DryRun           bool   `json:"dryRun"`
		WouldSubmit      bool   `json:"wouldSubmit"`
		AlreadySubmitted bool   `json:"alreadySubmitted"`
		BuildAttachment  struct {
			VersionID   string `json:"versionId"`
			BuildID     string `json:"buildId"`
			WouldAttach bool   `json:"wouldAttach"`
		} `json:"buildAttachment"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr for clean dry-run, got %q", stderr)
	}
	if payload.AppID != "app-1" {
		t.Fatalf("expected appId app-1, got %q", payload.AppID)
	}
	if payload.VersionID != "version-1" {
		t.Fatalf("expected versionId version-1, got %q", payload.VersionID)
	}
	if payload.BuildID != "build-1" {
		t.Fatalf("expected buildId build-1, got %q", payload.BuildID)
	}
	if payload.Platform != "IOS" {
		t.Fatalf("expected platform IOS, got %q", payload.Platform)
	}
	if !payload.DryRun {
		t.Fatal("expected dryRun=true")
	}
	if !payload.WouldSubmit {
		t.Fatal("expected wouldSubmit=true")
	}
	if payload.AlreadySubmitted {
		t.Fatal("did not expect alreadySubmitted=true")
	}
	if !payload.BuildAttachment.WouldAttach {
		t.Fatal("expected buildAttachment.wouldAttach=true")
	}

	recordedRequests := strings.Join(requests.Snapshot(), "\n")
	if strings.Contains(recordedRequests, "POST /v1/reviewSubmissions") || strings.Contains(recordedRequests, "PATCH /v1/reviewSubmissions/") {
		t.Fatalf("expected dry-run to avoid mutations, got requests: %s", recordedRequests)
	}
}

func TestReviewSubmitUsesModernReviewSubmissionFlow(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	requests := newRequestLog(20)
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests.Add(req.Method + " " + req.URL.Path)

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			query := req.URL.Query()
			switch {
			case query.Get("filter[versionString]") == "1.2.3":
				if query.Get("filter[platform]") != "IOS" {
					t.Fatalf("expected filter[platform]=IOS, got %q", query.Get("filter[platform]"))
				}
				return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}]}`)
			case query.Get("filter[platform]") == "IOS" && strings.Contains(query.Get("filter[appStoreState]"), "READY_FOR_SALE"):
				return jsonResponse(http.StatusOK, `{"data":[]}`)
			default:
				t.Fatalf("unexpected app store versions query: %s", req.URL.RawQuery)
				return nil, nil
			}
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/subscriptionGroups":
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/build":
			return jsonResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionSubmission":
			return jsonResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			return jsonResponse(http.StatusNoContent, "")
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			if req.URL.Query().Get("filter[state]") != "READY_FOR_REVIEW" {
				t.Fatalf("expected filter[state]=READY_FOR_REVIEW, got %q", req.URL.Query().Get("filter[state]"))
			}
			if req.URL.Query().Get("filter[platform]") != "IOS" {
				t.Fatalf("expected filter[platform]=IOS, got %q", req.URL.Query().Get("filter[platform]"))
			}
			return jsonResponse(http.StatusOK, `{"data":[],"links":{}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			return jsonResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"review-sub-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			return jsonResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"item-1"}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/review-sub-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"review-sub-1","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-04-08T00:00:00Z"}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appStoreVersionSubmissions":
			t.Fatalf("review submit should not use the legacy appStoreVersionSubmissions endpoint")
			return nil, nil
		default:
			t.Fatalf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"review", "submit",
			"--app", "app-1",
			"--version", "1.2.3",
			"--build", "build-1",
			"--confirm",
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

	var payload struct {
		AppID           string `json:"appId"`
		Version         string `json:"version"`
		VersionID       string `json:"versionId"`
		BuildID         string `json:"buildId"`
		Platform        string `json:"platform"`
		SubmissionID    string `json:"submissionId"`
		BuildAttachment struct {
			Attached bool `json:"attached"`
		} `json:"buildAttachment"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
	}

	if payload.AppID != "app-1" {
		t.Fatalf("expected appId app-1, got %q", payload.AppID)
	}
	if payload.Version != "1.2.3" {
		t.Fatalf("expected version 1.2.3, got %q", payload.Version)
	}
	if payload.VersionID != "version-1" {
		t.Fatalf("expected versionId version-1, got %q", payload.VersionID)
	}
	if payload.BuildID != "build-1" {
		t.Fatalf("expected buildId build-1, got %q", payload.BuildID)
	}
	if payload.Platform != "IOS" {
		t.Fatalf("expected platform IOS, got %q", payload.Platform)
	}
	if payload.SubmissionID != "review-sub-1" {
		t.Fatalf("expected submissionId review-sub-1, got %q", payload.SubmissionID)
	}
	if !payload.BuildAttachment.Attached {
		t.Fatal("expected build attachment to report attached=true")
	}

	recordedRequests := strings.Join(requests.Snapshot(), "\n")
	if strings.Contains(recordedRequests, "POST /v1/appStoreVersionSubmissions") {
		t.Fatalf("did not expect legacy endpoint usage, got requests: %s", recordedRequests)
	}
	if !strings.Contains(recordedRequests, "POST /v1/reviewSubmissions") {
		t.Fatalf("expected modern review submission create request, got requests: %s", recordedRequests)
	}
}

func TestReviewSubmitAlreadySubmittedSkipsPreflightAndBuildAttachment(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	requests := newRequestLog(20)
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests.Add(req.Method + " " + req.URL.Path)

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1":
			if req.URL.Query().Get("include") != "app" {
				t.Fatalf("expected include=app, got %q", req.URL.Query().Get("include"))
			}
			return jsonResponse(http.StatusOK, `{
				"data":{
					"type":"appStoreVersions",
					"id":"version-1",
					"attributes":{"platform":"IOS","versionString":"1.2.3"},
					"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}
				},
				"included":[{"type":"apps","id":"app-1","attributes":{"bundleId":"app-1","name":"App One"}}]
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionSubmission":
			return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersionSubmissions","id":"legacy-sub-1"}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			t.Fatalf("did not expect localization preflight when version is already submitted")
			return nil, nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			t.Fatalf("did not expect publish-state preflight when version is already submitted")
			return nil, nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/subscriptionGroups":
			t.Fatalf("did not expect subscription preflight when version is already submitted")
			return nil, nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/build":
			t.Fatalf("did not expect build lookup when version is already submitted")
			return nil, nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			t.Fatalf("did not expect build attachment mutation when version is already submitted")
			return nil, nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			t.Fatalf("did not expect new review submission creation when version is already submitted")
			return nil, nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			t.Fatalf("did not expect review submission item creation when version is already submitted")
			return nil, nil
		case req.Method == http.MethodPatch && strings.HasPrefix(req.URL.Path, "/v1/reviewSubmissions/"):
			t.Fatalf("did not expect review submission submission when version is already submitted")
			return nil, nil
		default:
			t.Fatalf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"review", "submit",
			"--app", "app-1",
			"--version-id", "version-1",
			"--build", "build-2",
			"--confirm",
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

	var payload struct {
		AppID            string    `json:"appId"`
		VersionID        string    `json:"versionId"`
		BuildID          string    `json:"buildId"`
		SubmissionID     string    `json:"submissionId"`
		AlreadySubmitted bool      `json:"alreadySubmitted"`
		BuildAttachment  *struct{} `json:"buildAttachment"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
	}

	if payload.AppID != "app-1" {
		t.Fatalf("expected appId app-1, got %q", payload.AppID)
	}
	if payload.VersionID != "version-1" {
		t.Fatalf("expected versionId version-1, got %q", payload.VersionID)
	}
	if payload.BuildID != "build-2" {
		t.Fatalf("expected buildId build-2, got %q", payload.BuildID)
	}
	if payload.SubmissionID != "legacy-sub-1" {
		t.Fatalf("expected submissionId legacy-sub-1, got %q", payload.SubmissionID)
	}
	if !payload.AlreadySubmitted {
		t.Fatal("expected alreadySubmitted=true")
	}
	if payload.BuildAttachment != nil {
		t.Fatalf("expected buildAttachment to be omitted when already submitted, got %s", stdout)
	}

	recordedRequests := strings.Join(requests.Snapshot(), "\n")
	if !strings.Contains(recordedRequests, "GET /v1/appStoreVersions/version-1/appStoreVersionSubmission") {
		t.Fatalf("expected existing submission lookup, got requests: %s", recordedRequests)
	}
	if strings.Contains(recordedRequests, "/relationships/build") {
		t.Fatalf("did not expect build mutation requests, got requests: %s", recordedRequests)
	}
}
