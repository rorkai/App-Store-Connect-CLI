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
	restore := reviewcli.SetReviewItemsClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	t.Cleanup(restore)
}
