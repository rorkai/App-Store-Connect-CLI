package submit

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
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
	if len(cmd.Subcommands) != 3 {
		t.Fatalf("expected 3 submit subcommands, got %d", len(cmd.Subcommands))
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
