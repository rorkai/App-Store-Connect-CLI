package validate

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type buildsRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn buildsRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestFetchAppBuildCount_UsesPagingTotalWhenPresent(t *testing.T) {
	client := newBuildsTestClient(t, buildsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			return buildsJSONResponse(http.StatusMethodNotAllowed, `{"errors":[{"status":"405"}]}`)
		}
		return buildsJSONResponse(http.StatusOK, `{
			"data":[{"type":"builds","id":"build-1"}],
			"meta":{"paging":{"total":7}}
		}`)
	}))

	count, status, err := fetchAppBuildCount(context.Background(), client, "app-1")
	if err != nil {
		t.Fatalf("fetchAppBuildCount() error = %v", err)
	}
	if count != 7 {
		t.Fatalf("expected paging total 7, got %d", count)
	}
	if !status.Verified || status.SkipReason != "" {
		t.Fatalf("expected verified status without skip reason, got %+v", status)
	}
}

func TestFetchAppBuildCount_FallsBackToResponseLengthWithoutPagingTotal(t *testing.T) {
	client := newBuildsTestClient(t, buildsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			return buildsJSONResponse(http.StatusMethodNotAllowed, `{"errors":[{"status":"405"}]}`)
		}
		return buildsJSONResponse(http.StatusOK, `{
			"data":[
				{"type":"builds","id":"build-1"},
				{"type":"builds","id":"build-2"}
			]
		}`)
	}))

	count, status, err := fetchAppBuildCount(context.Background(), client, "app-1")
	if err != nil {
		t.Fatalf("fetchAppBuildCount() error = %v", err)
	}
	if count != 2 {
		t.Fatalf("expected fallback count 2, got %d", count)
	}
	if !status.Verified || status.SkipReason != "" {
		t.Fatalf("expected verified status without skip reason, got %+v", status)
	}
}

func TestFetchAppBuildCount_SkipsKnownMetadataFailures(t *testing.T) {
	tests := []struct {
		name        string
		roundTrip   buildsRoundTripFunc
		wantSnippet string
	}{
		{
			name: "forbidden",
			roundTrip: func(*http.Request) (*http.Response, error) {
				return buildsJSONResponse(http.StatusForbidden, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Forbidden","detail":"not allowed"}]}`)
			},
			wantSnippet: "cannot read them",
		},
		{
			name: "retryable",
			roundTrip: func(*http.Request) (*http.Response, error) {
				return buildsJSONResponse(http.StatusTooManyRequests, `{"errors":[{"status":"429","code":"RATE_LIMITED","title":"Too Many Requests","detail":"rate limited"}]}`)
			},
			wantSnippet: "temporarily unavailable or rate limited",
		},
		{
			name: "deadline exceeded",
			roundTrip: func(*http.Request) (*http.Response, error) {
				return nil, context.DeadlineExceeded
			},
			wantSnippet: "endpoint timed out",
		},
		{
			name: "transport unreachable",
			roundTrip: func(*http.Request) (*http.Response, error) {
				return nil, &net.DNSError{
					Err:       "dial tcp: i/o timeout",
					Name:      "api.appstoreconnect.apple.com",
					IsTimeout: true,
				}
			},
			wantSnippet: "endpoint could not be reached",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := newBuildsTestClient(t, tc.roundTrip)

			count, status, err := fetchAppBuildCount(context.Background(), client, "app-1")
			if err != nil {
				t.Fatalf("fetchAppBuildCount() error = %v", err)
			}
			if count != 0 {
				t.Fatalf("expected count 0 when check is skipped, got %d", count)
			}
			if status.Verified {
				t.Fatalf("expected skipped build check status, got %+v", status)
			}
			if !strings.Contains(status.SkipReason, tc.wantSnippet) {
				t.Fatalf("expected skip reason to contain %q, got %q", tc.wantSnippet, status.SkipReason)
			}
		})
	}
}

func TestFetchAppBuildCount_PropagatesCanceledContext(t *testing.T) {
	client := newBuildsTestClient(t, buildsRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, context.Canceled
	}))

	count, status, err := fetchAppBuildCount(context.Background(), client, "app-1")
	if err == nil {
		t.Fatal("expected context canceled error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
	if count != 0 {
		t.Fatalf("expected count 0 on cancellation, got %d", count)
	}
	if status.Verified || status.SkipReason != "" {
		t.Fatalf("expected zero-value status on cancellation, got %+v", status)
	}
}

func TestFetchAppBuildCount_WrapsUnexpectedErrors(t *testing.T) {
	client := newBuildsTestClient(t, buildsRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return buildsJSONResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","title":"Internal Error","detail":"boom"}]}`)
	}))

	_, _, err := fetchAppBuildCount(context.Background(), client, "app-1")
	if err == nil {
		t.Fatal("expected error for unexpected build endpoint failures")
	}
	if !strings.Contains(err.Error(), "failed to fetch app builds") {
		t.Fatalf("expected wrapped fetch-app-builds error, got %v", err)
	}
}

func newBuildsTestClient(t *testing.T, transport http.RoundTripper) *asc.Client {
	t.Helper()

	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "AuthKey.p8")
	writeBuildsTestECDSAPEM(t, keyPath)

	client, err := asc.NewClientWithHTTPClient("KEY123", "ISS456", keyPath, &http.Client{Transport: transport})
	if err != nil {
		t.Fatalf("NewClientWithHTTPClient() error = %v", err)
	}
	return client
}

func writeBuildsTestECDSAPEM(t *testing.T, path string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey() error = %v", err)
	}

	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if pemBytes == nil {
		t.Fatal("failed to encode private key PEM")
	}

	if err := os.WriteFile(path, pemBytes, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func buildsJSONResponse(status int, body string) (*http.Response, error) {
	return &http.Response{
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}
