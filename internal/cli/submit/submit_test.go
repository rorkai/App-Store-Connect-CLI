package submit

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestSubmitCommandShape(t *testing.T) {
	cmd := SubmitCommand()
	if cmd == nil {
		t.Fatal("expected submit command")
	}
	if cmd.Name != "submit" {
		t.Fatalf("unexpected command name: %q", cmd.Name)
	}
	if len(cmd.Subcommands) != 4 {
		t.Fatalf("expected 4 submit subcommands, got %d", len(cmd.Subcommands))
	}
}

func TestSubmitCreateCommand_MissingConfirm(t *testing.T) {
	cmd := SubmitCreateCommand()
	if err := cmd.FlagSet.Parse([]string{"--build", "BUILD_ID", "--version", "1.0.0", "--app", "123"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}
	if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
}

func TestSubmitCreateCommand_MutuallyExclusiveVersionFlags(t *testing.T) {
	cmd := SubmitCreateCommand()
	args := []string{
		"--confirm",
		"--build", "BUILD_ID",
		"--app", "123",
		"--version", "1.0.0",
		"--version-id", "VERSION_ID",
	}
	if err := cmd.FlagSet.Parse(args); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}
	err := cmd.Exec(context.Background(), nil)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp for mutually exclusive flags, got %v", err)
	}
}

func TestSubmitStatusCommandValidation(t *testing.T) {
	t.Run("missing id and version-id", func(t *testing.T) {
		cmd := SubmitStatusCommand()
		if err := cmd.FlagSet.Parse([]string{}); err != nil {
			t.Fatalf("failed to parse flags: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp, got %v", err)
		}
	})

	t.Run("mutually exclusive id and version-id", func(t *testing.T) {
		cmd := SubmitStatusCommand()
		if err := cmd.FlagSet.Parse([]string{"--id", "S", "--version-id", "V"}); err != nil {
			t.Fatalf("failed to parse flags: %v", err)
		}
		err := cmd.Exec(context.Background(), nil)
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp, got %v", err)
		}
	})
}

func TestSubmitStatusCommand_ByIDUsesReviewSubmissionEndpoint(t *testing.T) {
	setupSubmitAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 2)
	http.DefaultTransport = submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.RequestURI())

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/review-submission-123":
			return submitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "reviewSubmissions",
					"id": "review-submission-123",
					"attributes": {
						"state": "IN_REVIEW",
						"submittedDate": "2026-03-16T10:00:00Z"
					},
					"relationships": {
						"appStoreVersionForReview": {
							"data": {
								"type": "appStoreVersions",
								"id": "version-123"
							}
						}
					}
				}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-123":
			return submitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "appStoreVersions",
					"id": "version-123",
					"attributes": {
						"versionString": "1.2.3",
						"platform": "IOS",
						"appStoreState": "WAITING_FOR_REVIEW"
					}
				}
			}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	})

	cmd := SubmitStatusCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--id", "review-submission-123", "--output", "json"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	stdout, err := captureSubmitCommandOutput(t, func() error {
		return cmd.Exec(context.Background(), nil)
	})
	if err != nil {
		t.Fatalf("expected command to succeed, got %v", err)
	}

	var result asc.AppStoreVersionSubmissionStatusResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
	}
	if result.ID != "review-submission-123" {
		t.Fatalf("expected review submission ID, got %q", result.ID)
	}
	if result.VersionID != "version-123" {
		t.Fatalf("expected version ID version-123, got %q", result.VersionID)
	}
	if result.VersionString != "1.2.3" {
		t.Fatalf("expected version string 1.2.3, got %q", result.VersionString)
	}
	if result.Platform != "IOS" {
		t.Fatalf("expected platform IOS, got %q", result.Platform)
	}
	if result.State != "IN_REVIEW" {
		t.Fatalf("expected review submission state IN_REVIEW, got %q", result.State)
	}
	if result.CreatedDate == nil || *result.CreatedDate != "2026-03-16T10:00:00Z" {
		t.Fatalf("expected submittedDate to be surfaced as createdDate, got %+v", result.CreatedDate)
	}

	wantRequests := []string{
		"GET /v1/reviewSubmissions/review-submission-123",
		"GET /v1/appStoreVersions/version-123",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
}

func TestSubmitStatusCommand_ByIDIgnoresInaccessibleItemLookup(t *testing.T) {
	setupSubmitAuth(t)

	tests := []struct {
		name       string
		statusCode int
		body       string
	}{
		{
			name:       "forbidden items lookup",
			statusCode: http.StatusForbidden,
			body:       `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Forbidden"}]}`,
		},
		{
			name:       "missing items lookup",
			statusCode: http.StatusNotFound,
			body:       `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			originalTransport := http.DefaultTransport
			t.Cleanup(func() {
				http.DefaultTransport = originalTransport
			})

			requests := make([]string, 0, 3)
			http.DefaultTransport = submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests = append(requests, req.Method+" "+req.URL.RequestURI())

				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/review-submission-123":
					return submitJSONResponse(http.StatusOK, `{
						"data": {
							"type": "reviewSubmissions",
							"id": "review-submission-123",
							"attributes": {
								"state": "IN_REVIEW",
								"submittedDate": "2026-03-16T10:00:00Z"
							}
						}
					}`)
				case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/review-submission-123/items":
					return submitJSONResponse(test.statusCode, test.body)
				default:
					return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
				}
			})

			cmd := SubmitStatusCommand()
			cmd.FlagSet.SetOutput(io.Discard)
			if err := cmd.FlagSet.Parse([]string{"--id", "review-submission-123", "--output", "json"}); err != nil {
				t.Fatalf("failed to parse flags: %v", err)
			}

			stdout, err := captureSubmitCommandOutput(t, func() error {
				return cmd.Exec(context.Background(), nil)
			})
			if err != nil {
				t.Fatalf("expected command to succeed, got %v", err)
			}

			var result asc.AppStoreVersionSubmissionStatusResult
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
			}
			if result.ID != "review-submission-123" {
				t.Fatalf("expected review submission ID, got %q", result.ID)
			}
			if result.State != "IN_REVIEW" {
				t.Fatalf("expected review submission state IN_REVIEW, got %q", result.State)
			}
			if result.VersionID != "" {
				t.Fatalf("expected empty version ID when items lookup is inaccessible, got %q", result.VersionID)
			}
			if result.VersionString != "" {
				t.Fatalf("expected empty version string when items lookup is inaccessible, got %q", result.VersionString)
			}
			if result.Platform != "" {
				t.Fatalf("expected empty platform when items lookup is inaccessible, got %q", result.Platform)
			}
			if result.CreatedDate == nil || *result.CreatedDate != "2026-03-16T10:00:00Z" {
				t.Fatalf("expected submittedDate to remain available, got %+v", result.CreatedDate)
			}

			wantRequests := []string{
				"GET /v1/reviewSubmissions/review-submission-123",
				"GET /v1/reviewSubmissions/review-submission-123/items?fields%5BreviewSubmissionItems%5D=appStoreVersion&limit=200",
			}
			if !reflect.DeepEqual(requests, wantRequests) {
				t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
			}
		})
	}
}

func TestSubmitStatusCommand_ByVersionIDUsesReviewSubmissionsForCurrentSubmission(t *testing.T) {
	setupSubmitAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 2)
	http.DefaultTransport = submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.RequestURI())

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-123":
			if got := req.URL.Query().Get("include"); got != "app" {
				return nil, fmt.Errorf("expected include=app, got %q", got)
			}
			return submitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "appStoreVersions",
					"id": "version-123",
					"attributes": {
						"versionString": "1.2.3",
						"platform": "IOS",
						"appStoreState": "WAITING_FOR_REVIEW"
					},
					"relationships": {
						"app": {
							"data": {
								"type": "apps",
								"id": "app-123"
							}
						}
					}
				}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-123/reviewSubmissions":
			if got := req.URL.Query().Get("include"); got != "appStoreVersionForReview" {
				return nil, fmt.Errorf("expected include=appStoreVersionForReview, got %q", got)
			}
			if got := req.URL.Query().Get("limit"); got != "200" {
				return nil, fmt.Errorf("expected limit=200, got %q", got)
			}
			return submitJSONResponse(http.StatusOK, `{
				"data": [
					{
						"type": "reviewSubmissions",
						"id": "review-submission-other",
						"attributes": {
							"state": "READY_FOR_REVIEW"
						},
						"relationships": {
							"appStoreVersionForReview": {
								"data": {
									"type": "appStoreVersions",
									"id": "version-other"
								}
							}
						}
					},
					{
						"type": "reviewSubmissions",
						"id": "review-submission-123",
						"attributes": {
							"state": "IN_REVIEW",
							"submittedDate": "2026-03-16T11:00:00Z"
						},
						"relationships": {
							"appStoreVersionForReview": {
								"data": {
									"type": "appStoreVersions",
									"id": "version-123"
								}
							}
						}
					}
				]
			}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	})

	cmd := SubmitStatusCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--version-id", "version-123", "--output", "json"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	stdout, err := captureSubmitCommandOutput(t, func() error {
		return cmd.Exec(context.Background(), nil)
	})
	if err != nil {
		t.Fatalf("expected command to succeed, got %v", err)
	}

	var result asc.AppStoreVersionSubmissionStatusResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
	}
	if result.ID != "review-submission-123" {
		t.Fatalf("expected review submission ID review-submission-123, got %q", result.ID)
	}
	if result.VersionID != "version-123" {
		t.Fatalf("expected version ID version-123, got %q", result.VersionID)
	}
	if result.State != "IN_REVIEW" {
		t.Fatalf("expected review submission state IN_REVIEW, got %q", result.State)
	}
	if result.VersionString != "1.2.3" || result.Platform != "IOS" {
		t.Fatalf("unexpected version details: %+v", result)
	}

	wantRequests := []string{
		"GET /v1/appStoreVersions/version-123?include=app",
		"GET /v1/apps/app-123/reviewSubmissions?include=appStoreVersionForReview&limit=200",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
}

func TestSubmitStatusCommand_ByVersionIDFallsBackToLegacyRelationshipAndVersionState(t *testing.T) {
	setupSubmitAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 3)
	http.DefaultTransport = submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.RequestURI())

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-123":
			if got := req.URL.Query().Get("include"); got != "app" {
				return nil, fmt.Errorf("expected include=app, got %q", got)
			}
			return submitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "appStoreVersions",
					"id": "version-123",
					"attributes": {
						"versionString": "1.2.3",
						"platform": "IOS",
						"appStoreState": "WAITING_FOR_REVIEW"
					},
					"relationships": {
						"app": {
							"data": {
								"type": "apps",
								"id": "app-123"
							}
						}
					}
				}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-123/reviewSubmissions":
			return submitJSONResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-123/appStoreVersionSubmission":
			return submitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "appStoreVersionSubmissions",
					"id": "legacy-submission-123",
					"attributes": {
						"createdDate": "2026-03-16T09:00:00Z"
					},
					"relationships": {
						"appStoreVersion": {
							"data": {
								"type": "appStoreVersions",
								"id": "version-123"
							}
						}
					}
				}
			}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	})

	cmd := SubmitStatusCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--version-id", "version-123", "--output", "json"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	stdout, err := captureSubmitCommandOutput(t, func() error {
		return cmd.Exec(context.Background(), nil)
	})
	if err != nil {
		t.Fatalf("expected command to succeed, got %v", err)
	}

	var result asc.AppStoreVersionSubmissionStatusResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
	}
	if result.ID != "legacy-submission-123" {
		t.Fatalf("expected legacy submission ID fallback, got %q", result.ID)
	}
	if result.State != "WAITING_FOR_REVIEW" {
		t.Fatalf("expected version state fallback WAITING_FOR_REVIEW, got %q", result.State)
	}
	if result.CreatedDate == nil || *result.CreatedDate != "2026-03-16T09:00:00Z" {
		t.Fatalf("expected legacy created date, got %+v", result.CreatedDate)
	}

	wantRequests := []string{
		"GET /v1/appStoreVersions/version-123?include=app",
		"GET /v1/apps/app-123/reviewSubmissions?include=appStoreVersionForReview&limit=200",
		"GET /v1/appStoreVersions/version-123/appStoreVersionSubmission",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
}

func TestSubmitStatusCommand_ByVersionIDFallsBackWhenReviewSubmissionListingIsForbidden(t *testing.T) {
	setupSubmitAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 3)
	http.DefaultTransport = submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.RequestURI())

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-123":
			if got := req.URL.Query().Get("include"); got != "app" {
				return nil, fmt.Errorf("expected include=app, got %q", got)
			}
			return submitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "appStoreVersions",
					"id": "version-123",
					"attributes": {
						"versionString": "1.2.3",
						"platform": "IOS",
						"appStoreState": "WAITING_FOR_REVIEW"
					},
					"relationships": {
						"app": {
							"data": {
								"type": "apps",
								"id": "app-123"
							}
						}
					}
				}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-123/reviewSubmissions":
			return submitJSONResponse(http.StatusForbidden, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Forbidden"}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-123/appStoreVersionSubmission":
			return submitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "appStoreVersionSubmissions",
					"id": "legacy-submission-123",
					"attributes": {
						"createdDate": "2026-03-16T09:00:00Z"
					},
					"relationships": {
						"appStoreVersion": {
							"data": {
								"type": "appStoreVersions",
								"id": "version-123"
							}
						}
					}
				}
			}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	})

	cmd := SubmitStatusCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--version-id", "version-123", "--output", "json"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	stdout, err := captureSubmitCommandOutput(t, func() error {
		return cmd.Exec(context.Background(), nil)
	})
	if err != nil {
		t.Fatalf("expected command to succeed, got %v", err)
	}

	var result asc.AppStoreVersionSubmissionStatusResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
	}
	if result.ID != "legacy-submission-123" {
		t.Fatalf("expected legacy submission ID fallback, got %q", result.ID)
	}
	if result.State != "WAITING_FOR_REVIEW" {
		t.Fatalf("expected version state fallback WAITING_FOR_REVIEW, got %q", result.State)
	}
	if result.CreatedDate == nil || *result.CreatedDate != "2026-03-16T09:00:00Z" {
		t.Fatalf("expected legacy created date, got %+v", result.CreatedDate)
	}

	wantRequests := []string{
		"GET /v1/appStoreVersions/version-123?include=app",
		"GET /v1/apps/app-123/reviewSubmissions?include=appStoreVersionForReview&limit=200",
		"GET /v1/appStoreVersions/version-123/appStoreVersionSubmission",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
}

func TestSubmitStatusCommand_ByIDNotFoundSuggestsVersionIDFallback(t *testing.T) {
	setupSubmitAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/missing-submission" {
			return submitJSONResponse(http.StatusNotFound, `{
				"errors": [{
					"status": "404",
					"code": "NOT_FOUND",
					"title": "Not Found"
				}]
			}`)
		}
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
	})

	cmd := SubmitStatusCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--id", "missing-submission"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	_, err := captureSubmitCommandOutput(t, func() error {
		return cmd.Exec(context.Background(), nil)
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `retry with --version-id to inspect the App Store version state`) {
		t.Fatalf("expected fallback hint in error, got %v", err)
	}
}

func TestSubmitCancelCommandValidation(t *testing.T) {
	t.Run("missing confirm", func(t *testing.T) {
		cmd := SubmitCancelCommand()
		if err := cmd.FlagSet.Parse([]string{"--id", "S"}); err != nil {
			t.Fatalf("failed to parse flags: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp, got %v", err)
		}
	})

	t.Run("mutually exclusive id and version-id", func(t *testing.T) {
		cmd := SubmitCancelCommand()
		if err := cmd.FlagSet.Parse([]string{"--confirm", "--id", "S", "--version-id", "V"}); err != nil {
			t.Fatalf("failed to parse flags: %v", err)
		}
		err := cmd.Exec(context.Background(), nil)
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp, got %v", err)
		}
	})
}

func TestCommandWrapper(t *testing.T) {
	if got := SubmitCommand(); got == nil {
		t.Fatal("expected Command wrapper to return submit command")
	}
}

type submitRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn submitRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func setupSubmitAuth(t *testing.T) {
	t.Helper()

	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "AuthKey.p8")
	writeSubmitECDSAPEM(t, keyPath)

	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_KEY_ID", "TEST_KEY")
	t.Setenv("ASC_ISSUER_ID", "TEST_ISSUER")
	t.Setenv("ASC_PRIVATE_KEY_PATH", keyPath)
}

func writeSubmitECDSAPEM(t *testing.T, path string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key error: %v", err)
	}
	data := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if data == nil {
		t.Fatal("failed to encode PEM")
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write key file error: %v", err)
	}
}

func submitJSONResponse(status int, body string) (*http.Response, error) {
	return &http.Response{
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

func TestSubmitCancelCommand_ByIDUsesReviewSubmissionEndpoint(t *testing.T) {
	setupSubmitAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 1)
	http.DefaultTransport = submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.Path)

		if req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/review-submission-123" {
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"review-submission-123"}}`)
		}

		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
	})

	cmd := SubmitCancelCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--id", "review-submission-123", "--confirm"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), nil); err != nil {
		t.Fatalf("expected command to succeed, got %v", err)
	}

	wantRequests := []string{"PATCH /v1/reviewSubmissions/review-submission-123"}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
}

func TestSubmitCancelCommand_ByVersionIDAttemptsReviewCancelThenFallsBackToLegacyDelete(t *testing.T) {
	setupSubmitAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 3)
	http.DefaultTransport = submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.Path)

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-123/appStoreVersionSubmission":
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersionSubmissions","id":"legacy-submission-123"}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/legacy-submission-123":
			return submitJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case req.Method == http.MethodDelete && req.URL.Path == "/v1/appStoreVersionSubmissions/legacy-submission-123":
			return submitJSONResponse(http.StatusNoContent, "")
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	cmd := SubmitCancelCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--version-id", "version-123", "--confirm"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), nil); err != nil {
		t.Fatalf("expected command to succeed, got %v", err)
	}

	wantRequests := []string{
		"GET /v1/appStoreVersions/version-123/appStoreVersionSubmission",
		"PATCH /v1/reviewSubmissions/legacy-submission-123",
		"DELETE /v1/appStoreVersionSubmissions/legacy-submission-123",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
}

func TestSubmitCancelCommand_ByIDNotFoundReportsReviewSubmissionError(t *testing.T) {
	setupSubmitAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/missing-review-id" {
			return submitJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		}
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
	})

	cmd := SubmitCancelCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--id", "missing-review-id", "--confirm"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `no review submission found for ID "missing-review-id"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSubmitCancelCommand_ByVersionIDNotFoundReportsLegacySubmissionError(t *testing.T) {
	setupSubmitAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/missing-version/appStoreVersionSubmission" {
			return submitJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		}
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
	})

	cmd := SubmitCancelCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--version-id", "missing-version", "--confirm"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `no legacy submission found for version "missing-version"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIsAppUpdate_IncludesReleasedAndRemovedStatesFilters(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			return nil, fmt.Errorf("unexpected method: %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-123/appStoreVersions" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}

		query := req.URL.Query()
		if got := query.Get("filter[platform]"); got != "IOS" {
			return nil, fmt.Errorf("unexpected filter[platform]: got %q want %q", got, "IOS")
		}
		if got := query.Get("filter[appStoreState]"); got != "READY_FOR_SALE,DEVELOPER_REMOVED_FROM_SALE,REMOVED_FROM_SALE" {
			return nil, fmt.Errorf("unexpected filter[appStoreState]: %q", got)
		}
		if got := query.Get("limit"); got != "1" {
			return nil, fmt.Errorf("unexpected limit: got %q want %q", got, "1")
		}

		return submitJSONResponse(http.StatusOK, `{
			"data": [
				{
					"type": "appStoreVersions",
					"id": "version-1",
					"attributes": {}
				}
			]
		}`)
	}))

	isUpdate, err := isAppUpdate(context.Background(), client, "app-123", "IOS")
	if err != nil {
		t.Fatalf("isAppUpdate() error = %v", err)
	}
	if !isUpdate {
		t.Fatal("isAppUpdate() = false, want true when released/removed versions exist")
	}
}

func TestIsAppUpdate_EmptyPlatformSkipsPlatformFilter(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			return nil, fmt.Errorf("unexpected method: %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-123/appStoreVersions" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}

		query := req.URL.Query()
		if got := query.Get("filter[platform]"); got != "" {
			return nil, fmt.Errorf("did not expect filter[platform], got %q", got)
		}
		if got := query.Get("filter[appStoreState]"); got != "READY_FOR_SALE,DEVELOPER_REMOVED_FROM_SALE,REMOVED_FROM_SALE" {
			return nil, fmt.Errorf("unexpected filter[appStoreState]: %q", got)
		}
		if got := query.Get("limit"); got != "1" {
			return nil, fmt.Errorf("unexpected limit: got %q want %q", got, "1")
		}

		return submitJSONResponse(http.StatusOK, `{"data":[]}`)
	}))

	isUpdate, err := isAppUpdate(context.Background(), client, "app-123", "   ")
	if err != nil {
		t.Fatalf("isAppUpdate() error = %v", err)
	}
	if isUpdate {
		t.Fatal("isAppUpdate() = true, want false when no versions are returned")
	}
}

func TestIsAppUpdate_PropagatesClientErrors(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			return nil, fmt.Errorf("unexpected method: %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-123/appStoreVersions" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
		return submitJSONResponse(http.StatusInternalServerError, `{
			"errors": [{
				"status": "500",
				"code": "INTERNAL_ERROR",
				"title": "Internal Server Error"
			}]
		}`)
	}))

	_, err := isAppUpdate(context.Background(), client, "app-123", "IOS")
	if err == nil {
		t.Fatal("isAppUpdate() error = nil, want non-nil")
	}
}

func TestFindReviewSubmissionForVersion_FallsBackToSubmissionItems(t *testing.T) {
	requests := make([]string, 0, 2)
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.RequestURI())

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-123/reviewSubmissions":
			return submitJSONResponse(http.StatusOK, `{
				"data": [
					{
						"type": "reviewSubmissions",
						"id": "review-submission-123",
						"attributes": {
							"state": "READY_FOR_REVIEW"
						}
					}
				]
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/review-submission-123/items":
			if got := req.URL.Query().Get("fields[reviewSubmissionItems]"); got != "appStoreVersion" {
				return nil, fmt.Errorf("expected review submission items fields query, got %q", got)
			}
			if got := req.URL.Query().Get("limit"); got != "200" {
				return nil, fmt.Errorf("expected limit=200, got %q", got)
			}
			return submitJSONResponse(http.StatusOK, `{
				"data": [
					{
						"type": "reviewSubmissionItems",
						"id": "item-1",
						"relationships": {
							"appStoreVersion": {
								"data": {
									"type": "appStoreVersions",
									"id": "version-123"
								}
							}
						}
					}
				]
			}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))

	submission, err := findReviewSubmissionForVersion(context.Background(), client, "app-123", "version-123")
	if err != nil {
		t.Fatalf("findReviewSubmissionForVersion() error: %v", err)
	}
	if submission == nil {
		t.Fatal("expected review submission match, got nil")
	}
	if submission.ID != "review-submission-123" {
		t.Fatalf("expected review submission ID review-submission-123, got %q", submission.ID)
	}

	wantRequests := []string{
		"GET /v1/apps/app-123/reviewSubmissions?include=appStoreVersionForReview&limit=200",
		"GET /v1/reviewSubmissions/review-submission-123/items?fields%5BreviewSubmissionItems%5D=appStoreVersion&limit=200",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
}

func TestFindReviewSubmissionForVersion_ContinuesAfterPerSubmissionLookupErrors(t *testing.T) {
	requests := make([]string, 0, 3)
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.RequestURI())

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-123/reviewSubmissions":
			return submitJSONResponse(http.StatusOK, `{
				"data": [
					{
						"type": "reviewSubmissions",
						"id": "broken-submission",
						"attributes": {
							"state": "COMPLETE"
						}
					},
					{
						"type": "reviewSubmissions",
						"id": "current-submission",
						"attributes": {
							"state": "WAITING_FOR_REVIEW"
						},
						"relationships": {
							"appStoreVersionForReview": {
								"data": {
									"type": "appStoreVersions",
									"id": "version-123"
								}
							}
						}
					}
				]
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/broken-submission/items":
			return submitJSONResponse(http.StatusForbidden, `{
				"errors": [{
					"status": "403",
					"code": "FORBIDDEN",
					"title": "Forbidden"
				}]
			}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))

	submission, err := findReviewSubmissionForVersion(context.Background(), client, "app-123", "version-123")
	if err != nil {
		t.Fatalf("findReviewSubmissionForVersion() error: %v", err)
	}
	if submission == nil {
		t.Fatal("expected review submission match, got nil")
	}
	if submission.ID != "current-submission" {
		t.Fatalf("expected current-submission, got %q", submission.ID)
	}
}

func TestFindReviewSubmissionForVersion_PrefersCurrentSubmissionOverHistoricalMatch(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-123/reviewSubmissions":
			return submitJSONResponse(http.StatusOK, `{
				"data": [
					{
						"type": "reviewSubmissions",
						"id": "historical-submission",
						"attributes": {
							"state": "COMPLETE",
							"submittedDate": "2026-03-15T11:00:00Z"
						},
						"relationships": {
							"appStoreVersionForReview": {
								"data": {
									"type": "appStoreVersions",
									"id": "version-123"
								}
							}
						}
					},
					{
						"type": "reviewSubmissions",
						"id": "current-submission",
						"attributes": {
							"state": "IN_REVIEW",
							"submittedDate": "2026-03-16T11:00:00Z"
						},
						"relationships": {
							"appStoreVersionForReview": {
								"data": {
									"type": "appStoreVersions",
									"id": "version-123"
								}
							}
						}
					}
				]
			}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))

	submission, err := findReviewSubmissionForVersion(context.Background(), client, "app-123", "version-123")
	if err != nil {
		t.Fatalf("findReviewSubmissionForVersion() error: %v", err)
	}
	if submission == nil {
		t.Fatal("expected review submission match, got nil")
	}
	if submission.ID != "current-submission" {
		t.Fatalf("expected current-submission, got %q", submission.ID)
	}
}

func TestFindReviewSubmissionForVersion_PropagatesUnexpectedLookupErrors(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-123/reviewSubmissions":
			return submitJSONResponse(http.StatusOK, `{
				"data": [
					{
						"type": "reviewSubmissions",
						"id": "broken-submission",
						"attributes": {
							"state": "WAITING_FOR_REVIEW"
						}
					}
				]
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/broken-submission/items":
			return submitJSONResponse(http.StatusInternalServerError, `{
				"errors": [{
					"status": "500",
					"code": "INTERNAL_ERROR",
					"title": "Server Error"
				}]
			}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))

	submission, err := findReviewSubmissionForVersion(context.Background(), client, "app-123", "version-123")
	if err == nil {
		t.Fatal("expected lookup error, got nil")
	}
	if submission != nil {
		t.Fatalf("expected nil submission on unexpected lookup error, got %#v", submission)
	}
	if !strings.Contains(err.Error(), "Server Error") {
		t.Fatalf("expected server error to propagate, got %v", err)
	}
}

func TestExtractExistingSubmissionID(t *testing.T) {
	t.Run("returns submission ID from associated error", func(t *testing.T) {
		err := &asc.APIError{
			Code:   "ENTITY_ERROR",
			Title:  "The request entity is not valid.",
			Detail: "An attribute value is not valid.",
			AssociatedErrors: map[string][]asc.APIAssociatedError{
				"/v1/reviewSubmissionItems": {
					{
						Code:   "ENTITY_ERROR.RELATIONSHIP.INVALID",
						Detail: "appStoreVersions with id 883340862 was already added to another reviewSubmission with id fb5dad8e-bd5f-4d96-bc2f-561cf74a7e7a",
					},
				},
			},
		}
		got := extractExistingSubmissionID(err)
		want := "fb5dad8e-bd5f-4d96-bc2f-561cf74a7e7a"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("returns empty for non-APIError", func(t *testing.T) {
		err := fmt.Errorf("some random error")
		if got := extractExistingSubmissionID(err); got != "" {
			t.Fatalf("expected empty, got %q", got)
		}
	})

	t.Run("returns empty for APIError without matching detail", func(t *testing.T) {
		err := &asc.APIError{
			Code:   "ENTITY_ERROR",
			Title:  "Something else went wrong.",
			Detail: "Unrelated problem.",
			AssociatedErrors: map[string][]asc.APIAssociatedError{
				"/v1/reviewSubmissionItems": {
					{Code: "OTHER_ERROR", Detail: "something unrelated"},
				},
			},
		}
		if got := extractExistingSubmissionID(err); got != "" {
			t.Fatalf("expected empty, got %q", got)
		}
	})

	t.Run("returns empty for APIError with no associated errors", func(t *testing.T) {
		err := &asc.APIError{
			Code:  "ENTITY_ERROR",
			Title: "Something went wrong.",
		}
		if got := extractExistingSubmissionID(err); got != "" {
			t.Fatalf("expected empty, got %q", got)
		}
	})

	t.Run("works with wrapped APIError", func(t *testing.T) {
		apiErr := &asc.APIError{
			Code: "ENTITY_ERROR",
			AssociatedErrors: map[string][]asc.APIAssociatedError{
				"/v1/reviewSubmissionItems": {
					{
						Code:   "ENTITY_ERROR.RELATIONSHIP.INVALID",
						Detail: "appStoreVersions with id 999 was already added to another reviewSubmission with id aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
					},
				},
			},
		}
		wrapped := fmt.Errorf("add item failed: %w", apiErr)
		got := extractExistingSubmissionID(wrapped)
		want := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("handles uppercase UUID", func(t *testing.T) {
		err := &asc.APIError{
			Code: "ENTITY_ERROR",
			AssociatedErrors: map[string][]asc.APIAssociatedError{
				"/v1/reviewSubmissionItems": {
					{
						Code:   "ENTITY_ERROR.RELATIONSHIP.INVALID",
						Detail: "appStoreVersions with id 123 was Already Added to another reviewSubmission with id FB5DAD8E-BD5F-4D96-BC2F-561CF74A7E7A",
					},
				},
			},
		}
		got := extractExistingSubmissionID(err)
		want := "FB5DAD8E-BD5F-4D96-BC2F-561CF74A7E7A"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("handles non-UUID identifier", func(t *testing.T) {
		err := &asc.APIError{
			Code: "ENTITY_ERROR",
			AssociatedErrors: map[string][]asc.APIAssociatedError{
				"/v1/reviewSubmissionItems": {
					{
						Code:   "ENTITY_ERROR.RELATIONSHIP.INVALID",
						Detail: "appStoreVersions with id 123 was already added to another reviewSubmission with id some-opaque-id-12345",
					},
				},
			},
		}
		got := extractExistingSubmissionID(err)
		want := "some-opaque-id-12345"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})
}

func TestAddVersionToSubmissionOrRecover_ExhaustsRetriesForRecentlyCanceledSubmission(t *testing.T) {
	const staleSubmissionID = "stale-1"

	attempts := 0
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/v1/reviewSubmissionItems" {
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		attempts++
		return submitJSONResponse(http.StatusConflict, submitAlreadyAddedConflictBody(staleSubmissionID))
	}))

	originalDelays := submitCreateRecentlyCanceledRetryDelays
	submitCreateRecentlyCanceledRetryDelays = []time.Duration{time.Millisecond, time.Millisecond}
	t.Cleanup(func() {
		submitCreateRecentlyCanceledRetryDelays = originalDelays
	})

	resolvedID, err := addVersionToSubmissionOrRecover(
		context.Background(),
		client,
		"new-sub-1",
		"version-1",
		map[string]struct{}{staleSubmissionID: {}},
	)
	if err == nil {
		t.Fatal("expected retry exhaustion error")
	}
	if resolvedID != "" {
		t.Fatalf("expected empty resolved submission ID on failure, got %q", resolvedID)
	}
	if !strings.Contains(err.Error(), "still attached to recently canceled review submission stale-1 after 2 retries") {
		t.Fatalf("expected retry exhaustion message, got: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 add-item attempts (initial + 2 retries), got %d", attempts)
	}
}

func TestAddVersionToSubmissionOrRecover_ReturnsContextErrorWhileWaitingForDetach(t *testing.T) {
	const staleSubmissionID = "stale-1"

	attempts := 0
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/v1/reviewSubmissionItems" {
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		attempts++
		return submitJSONResponse(http.StatusConflict, submitAlreadyAddedConflictBody(staleSubmissionID))
	}))

	originalDelays := submitCreateRecentlyCanceledRetryDelays
	submitCreateRecentlyCanceledRetryDelays = []time.Duration{100 * time.Millisecond}
	t.Cleanup(func() {
		submitCreateRecentlyCanceledRetryDelays = originalDelays
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	resolvedID, err := addVersionToSubmissionOrRecover(
		ctx,
		client,
		"new-sub-1",
		"version-1",
		map[string]struct{}{staleSubmissionID: {}},
	)
	if err == nil {
		t.Fatal("expected context cancellation while waiting to retry")
	}
	if resolvedID != "" {
		t.Fatalf("expected empty resolved submission ID on failure, got %q", resolvedID)
	}
	if !strings.Contains(err.Error(), "waiting for recently canceled review submission stale-1 to clear") {
		t.Fatalf("expected wait/cancellation error message, got: %v", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected wrapped context deadline exceeded error, got: %v", err)
	}
	if attempts != 1 {
		t.Fatalf("expected one add-item attempt before context cancellation, got %d", attempts)
	}
}

func TestCleanupEmptyReviewSubmissionWarnsOnUnexpectedCancelError(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPatch || req.URL.Path != "/v1/reviewSubmissions/empty-sub-1" {
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		return submitJSONResponse(http.StatusInternalServerError, `{
			"errors": [{
				"status": "500",
				"code": "INTERNAL_ERROR",
				"title": "Internal Server Error"
			}]
		}`)
	}))

	stderr := captureSubmitStderr(t, func() {
		cleanupEmptyReviewSubmission(context.Background(), client, "empty-sub-1")
	})
	if !strings.Contains(stderr, "Warning: failed to cancel empty submission empty-sub-1:") {
		t.Fatalf("expected cleanup warning, got %q", stderr)
	}
}

func TestCleanupEmptyReviewSubmissionIgnoresExpectedNonCancellableState(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPatch || req.URL.Path != "/v1/reviewSubmissions/empty-sub-1" {
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		return submitJSONResponse(http.StatusConflict, `{
			"errors": [{
				"status": "409",
				"code": "CONFLICT",
				"title": "Resource state is invalid.",
				"detail": "Resource is not in cancellable state"
			}]
		}`)
	}))

	stderr := captureSubmitStderr(t, func() {
		cleanupEmptyReviewSubmission(context.Background(), client, "empty-sub-1")
	})
	if stderr != "" {
		t.Fatalf("expected no cleanup warning for expected non-cancellable state, got %q", stderr)
	}
}

func TestCleanupEmptyReviewSubmissionWarnsOnGenericConflict(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPatch || req.URL.Path != "/v1/reviewSubmissions/empty-sub-1" {
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		return submitJSONResponse(http.StatusConflict, `{
			"errors": [{
				"status": "409",
				"code": "CONFLICT",
				"title": "Conflict",
				"detail": "Another operation is already in progress"
			}]
		}`)
	}))

	stderr := captureSubmitStderr(t, func() {
		cleanupEmptyReviewSubmission(context.Background(), client, "empty-sub-1")
	})
	if !strings.Contains(stderr, "Warning: failed to cancel empty submission empty-sub-1:") {
		t.Fatalf("expected cleanup warning for generic conflict, got %q", stderr)
	}
}

func TestCancelStaleReviewSubmissionsWarnsOnGenericConflict(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "reviewSubmissions",
					"id": "stale-sub-1",
					"attributes": {
						"state": "READY_FOR_REVIEW",
						"platform": "IOS"
					}
				}]
			}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/stale-sub-1":
			return submitJSONResponse(http.StatusConflict, `{
				"errors": [{
					"status": "409",
					"code": "CONFLICT",
					"title": "Conflict",
					"detail": "Another operation is already in progress"
				}]
			}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	}))

	stderr := captureSubmitStderr(t, func() {
		got := cancelStaleReviewSubmissions(context.Background(), client, "app-1", "IOS")
		if got != nil {
			t.Fatalf("expected no canceled submissions, got %#v", got)
		}
	})
	if !strings.Contains(stderr, "Warning: failed to cancel stale submission stale-sub-1:") {
		t.Fatalf("expected stale submission warning for generic conflict, got %q", stderr)
	}
	if strings.Contains(stderr, "Skipped stale submission stale-sub-1") {
		t.Fatalf("did not expect stale submission skip message, got %q", stderr)
	}
}

func TestPrintSubmissionErrorHintsUsesExistingRunnableCommands(t *testing.T) {
	stderr := captureSubmitStderr(t, func() {
		printSubmissionErrorHints(errors.New("ageRatingDeclaration contentRightsDeclaration usesNonExemptEncryption appDataUsage primaryCategory"), "app-1")
	})

	for _, want := range []string{
		"Hint: Review current age rating: asc age-rating view --app app-1",
		"Hint: Review age-rating update flags: asc age-rating set --help",
		"Hint: If your app does not use third-party content: asc apps update --id app-1 --content-rights DOES_NOT_USE_THIRD_PARTY_CONTENT",
		"Hint: If your app uses third-party content: asc apps update --id app-1 --content-rights USES_THIRD_PARTY_CONTENT",
		"Hint: Set Uses Non-Exempt Encryption for the attached build in App Store Connect, then retry submission.",
		"Hint: Complete App Privacy at: https://appstoreconnect.apple.com/apps/app-1/appPrivacy",
		"Hint: List available categories: asc categories list",
		"Hint: Review category update flags: asc app-setup categories set --help",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected hint %q in stderr, got %q", want, stderr)
		}
	}

	for _, unwanted := range []string{
		"--all-none",
		"content-rights set",
		"--uses-third-party-content",
		"builds update",
		"--primary SPORTS",
		"...",
		"|",
	} {
		if strings.Contains(stderr, unwanted) {
			t.Fatalf("did not expect %q in stderr, got %q", unwanted, stderr)
		}
	}
}

func newSubmitTestClient(t *testing.T, transport http.RoundTripper) *asc.Client {
	t.Helper()

	keyPath := filepath.Join(t.TempDir(), "AuthKey.p8")
	writeSubmitECDSAPEM(t, keyPath)

	client, err := asc.NewClientWithHTTPClient("TEST_KEY", "TEST_ISSUER", keyPath, &http.Client{
		Transport: transport,
	})
	if err != nil {
		t.Fatalf("NewClientWithHTTPClient() error: %v", err)
	}
	return client
}

func captureSubmitStderr(t *testing.T, fn func()) string {
	t.Helper()

	oldStderr := os.Stderr
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe error: %v", err)
	}

	os.Stderr = stderrW
	defer func() {
		os.Stderr = oldStderr
	}()

	fn()

	if err := stderrW.Close(); err != nil {
		t.Fatalf("close stderr writer: %v", err)
	}
	data, err := io.ReadAll(stderrR)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	if err := stderrR.Close(); err != nil {
		t.Fatalf("close stderr reader: %v", err)
	}
	return string(data)
}

func submitAlreadyAddedConflictBody(existingSubmissionID string) string {
	return fmt.Sprintf(`{
		"errors": [{
			"status": "409",
			"code": "ENTITY_ERROR",
			"title": "The request entity is not valid.",
			"detail": "An attribute value is not valid.",
			"meta": {
				"associatedErrors": {
					"/v1/reviewSubmissionItems": [{
						"code": "ENTITY_ERROR.RELATIONSHIP.INVALID",
						"detail": "appStoreVersions with id version-1 was already added to another reviewSubmission with id %s"
					}]
				}
			}
		}]
	}`, existingSubmissionID)
}

func captureSubmitCommandOutput(t *testing.T, fn func() error) (string, error) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr

	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stderr pipe: %v", err)
	}

	os.Stdout = wOut
	os.Stderr = wErr

	outC := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, rOut)
		_ = rOut.Close()
		outC <- buf.String()
	}()

	go func() {
		_, _ = io.Copy(io.Discard, rErr)
		_ = rErr.Close()
	}()

	runErr := fn()

	_ = wOut.Close()
	_ = wErr.Close()

	stdout := <-outC

	os.Stdout = oldStdout
	os.Stderr = oldStderr

	return stdout, runErr
}
