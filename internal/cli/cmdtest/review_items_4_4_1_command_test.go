package cmdtest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	reviewcli "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/reviews"
)

func TestReviewListsOwnerlessNextPaginateAllPages(t *testing.T) {
	tests := []struct {
		name         string
		args         func(string) []string
		path         string
		resourceType string
		firstID      string
		secondID     string
		setClient    func(*testing.T, *httptest.Server)
	}{
		{
			name: "review items",
			args: func(next string) []string {
				return []string{"review", "items-list", "--next", next, "--paginate", "--output", "json", "--pretty=false"}
			},
			path: "/v1/reviewSubmissions/sub-1/items", resourceType: "reviewSubmissionItems", firstID: "item-1", secondID: "item-2", setClient: setReviewItemsTestServerClient,
		},
		{
			name: "review submissions with ambient app id",
			args: func(next string) []string {
				return []string{"review", "submissions-list", "--next", next, "--paginate", "--output", "json", "--pretty=false"}
			},
			path: "/v1/reviewSubmissions", resourceType: "reviewSubmissions", firstID: "submission-1", secondID: "submission-2", setClient: setReviewSubmissionsTestServerClient,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupSubmitCreateAuth(t)
			t.Setenv("ASC_APP_ID", "ambient-app-must-not-conflict")

			requestCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				requestCount++
				if req.Method != http.MethodGet || req.URL.Path != test.path {
					t.Fatalf("request = %s %s, want GET %s", req.Method, req.URL.Path, test.path)
				}
				w.Header().Set("Content-Type", "application/json")
				switch requestCount {
				case 1:
					if got := req.URL.RawQuery; got != "cursor=start" {
						t.Fatalf("first query = %q, want opaque cursor=start", got)
					}
					next := "https://api.appstoreconnect.apple.com" + test.path + "?cursor=next"
					_, _ = fmt.Fprintf(w, `{"data":[{"type":%q,"id":%q}],"links":{"next":%q}}`, test.resourceType, test.firstID, next)
				case 2:
					if got := req.URL.RawQuery; got != "cursor=next" {
						t.Fatalf("second query = %q, want opaque cursor=next", got)
					}
					_, _ = fmt.Fprintf(w, `{"data":[{"type":%q,"id":%q}],"links":{"next":""}}`, test.resourceType, test.secondID)
				default:
					t.Fatalf("unexpected extra request: %s", req.URL)
				}
			}))
			t.Cleanup(server.Close)
			test.setClient(t, server)

			next := "https://api.appstoreconnect.apple.com" + test.path + "?cursor=start"
			stdout, stderr := captureOutput(t, func() {
				code := cmd.Run(test.args(next), "1.2.3")
				if code != cmd.ExitSuccess {
					t.Fatalf("exit code = %d, want %d", code, cmd.ExitSuccess)
				}
			})
			if got := stripDeprecatedCommandWarnings(stderr); strings.TrimSpace(got) != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			if requestCount != 2 || !strings.Contains(stdout, test.firstID) || !strings.Contains(stdout, test.secondID) {
				t.Fatalf("requests = %d, stdout = %q, want two aggregated pages", requestCount, stdout)
			}
		})
	}
}

func TestRunReviewItemsAddSupports441SubscriptionVersionTypes(t *testing.T) {
	tests := []struct {
		name             string
		itemType         string
		relationshipName string
		resourceType     string
	}{
		{name: "subscription version", itemType: "subscriptionVersions", relationshipName: "subscriptionVersion", resourceType: "subscriptionVersions"},
		{name: "subscription group version", itemType: "subscriptionGroupVersions", relationshipName: "subscriptionGroupVersion", resourceType: "subscriptionGroupVersions"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupSubmitCreateAuth(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method != http.MethodPost || req.URL.Path != "/v1/reviewSubmissionItems" {
					t.Fatalf("request = %s %s, want POST /v1/reviewSubmissionItems", req.Method, req.URL.Path)
				}
				assertJSONDocument(t, req.Body, fmt.Sprintf(`{
					"data":{
						"type":"reviewSubmissionItems",
						"relationships":{
							"reviewSubmission":{"data":{"type":"reviewSubmissions","id":"submission-1"}},
							%q:{"data":{"type":%q,"id":"version-1"}}
						}
					}
				}`, test.relationshipName, test.resourceType))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = fmt.Fprintf(w, `{"data":{"type":"reviewSubmissionItems","id":"item-1","relationships":{%q:{"data":{"type":%q,"id":"version-1"}}}}}`, test.relationshipName, test.resourceType)
			}))
			defer server.Close()
			setReviewItemsTestServerClient(t, server)

			stdout, stderr := captureOutput(t, func() {
				code := cmd.Run([]string{
					"review", "items-add",
					"--submission", "submission-1",
					"--item-type", test.itemType,
					"--item-id", "version-1",
					"--output", "json",
				}, "1.2.3")
				if code != cmd.ExitSuccess {
					t.Fatalf("exit code = %d, want %d", code, cmd.ExitSuccess)
				}
			})
			if got := stripDeprecatedCommandWarnings(stderr); strings.TrimSpace(got) != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			var response asc.ReviewSubmissionItemResponse
			if err := json.Unmarshal([]byte(stdout), &response); err != nil {
				t.Fatalf("stdout = %q, want structured JSON: %v", stdout, err)
			}
			if response.Data.ID != "item-1" {
				t.Fatalf("item ID = %q, want item-1", response.Data.ID)
			}
		})
	}
}

func TestRunReviewItemsAddRejectsPositionalArgsBeforeAuth(t *testing.T) {
	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"review", "items-add",
			"--submission", "submission-1",
			"--item-type", "subscriptionVersions",
			"--item-id", "version-1",
			"unexpected",
		}, "1.2.3")
		if code != cmd.ExitUsage {
			t.Fatalf("exit code = %d, want %d", code, cmd.ExitUsage)
		}
	})
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "unexpected positional arguments") {
		t.Fatalf("stderr = %q, want positional argument error", stderr)
	}
}

func setReviewItemsTestServerClient(t *testing.T, server *httptest.Server) {
	t.Helper()
	client := newReviewTestServerClient(t, server)
	restore := reviewcli.SetReviewItemsClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	t.Cleanup(restore)
}

func setReviewSubmissionsTestServerClient(t *testing.T, server *httptest.Server) {
	t.Helper()
	client := newReviewTestServerClient(t, server)
	restore := reviewcli.SetReviewSubmissionsClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	t.Cleanup(restore)
}

func newReviewTestServerClient(t *testing.T, server *httptest.Server) *asc.Client {
	t.Helper()
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		cloned := req.Clone(req.Context())
		cloned.URL.Scheme = serverURL.Scheme
		cloned.URL.Host = serverURL.Host
		return server.Client().Transport.RoundTrip(cloned)
	})
	client, err := asc.NewClientWithHTTPClient(
		"TEST_KEY",
		"TEST_ISSUER",
		os.Getenv("ASC_PRIVATE_KEY_PATH"),
		&http.Client{Transport: transport},
	)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	return client
}
