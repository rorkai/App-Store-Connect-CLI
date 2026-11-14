package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestAgeRatingEditKorea45(t *testing.T) {
	for _, rating := range []string{"ALL", "TWELVE_PLUS", ""} {
		t.Run(rating, func(t *testing.T) {
			setupAuth(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method != http.MethodPatch || req.URL.Path != "/v1/ageRatingDeclarations/age-45" {
					t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
				}
				var payload struct {
					Data struct {
						Attributes map[string]any `json:"attributes"`
					} `json:"data"`
				}
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				if (rating != "" && payload.Data.Attributes["koreaAgeRatingOverride"] != rating) || (rating == "" && payload.Data.Attributes["koreaAgeRatingOverride"] != nil) || payload.Data.Attributes["gracRatingClassificationNumber"] != "CC-2026-123" {
					t.Errorf("unexpected attributes: %#v", payload.Data.Attributes)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"data":{"type":"ageRatingDeclarations","id":"age-45","attributes":{"koreaAgeRatingOverride":"`+rating+`","gracRatingClassificationNumber":"CC-2026-123"}}}`)
			}))
			defer server.Close()
			serverURL, _ := url.Parse(server.URL)
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				cloned := req.Clone(req.Context())
				cloned.URL.Scheme = serverURL.Scheme
				cloned.URL.Host = serverURL.Host
				return server.Client().Transport.RoundTrip(cloned)
			}))
			root := RootCommand("test")
			var runErr error
			args := []string{"age-rating", "edit", "--id", "age-45", "--grac-rating-classification-number", "CC-2026-123", "--output", "json"}
			if rating != "" {
				args = append(args, "--korea-age-rating-override", rating)
			}
			stdout, stderr := captureOutput(t, func() { runErr = root.ParseAndRun(context.Background(), args) })
			if runErr != nil {
				t.Fatalf("run: %v; stderr=%s", runErr, stderr)
			}
			var response struct {
				Data struct {
					Attributes map[string]any `json:"attributes"`
				} `json:"data"`
			}
			if err := json.Unmarshal([]byte(stdout), &response); err != nil {
				t.Fatal(err)
			}
			if response.Data.Attributes["gracRatingClassificationNumber"] != "CC-2026-123" {
				t.Fatalf("missing GRAC number: %s", stdout)
			}
		})
	}
}

func TestAgeRatingEditInvalidKorea45(t *testing.T) {
	assertUsageExit(t, []string{"age-rating", "edit", "--id", "age-45", "--korea-age-rating-override", "TEN_PLUS"}, "--korea-age-rating-override must be one of:")
	assertUsageExit(t, []string{"age-rating", "edit", "--id", "age-45", "--grac-rating-classification-number", " "}, "--grac-rating-classification-number must not be empty")
}

func TestAgeRatingEditClearGRAC(t *testing.T) {
	setupAuth(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPatch || req.URL.Path != "/v1/ageRatingDeclarations/age-45" {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		var payload struct {
			Data struct {
				Attributes map[string]json.RawMessage `json:"attributes"`
			} `json:"data"`
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if len(payload.Data.Attributes) != 1 || string(payload.Data.Attributes["gracRatingClassificationNumber"]) != "null" {
			t.Errorf("expected only an explicit GRAC null, got %#v", payload.Data.Attributes)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":{"type":"ageRatingDeclarations","id":"age-45","attributes":{}}}`)
	}))
	defer server.Close()
	serverURL, _ := url.Parse(server.URL)
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		cloned := req.Clone(req.Context())
		cloned.URL.Scheme = serverURL.Scheme
		cloned.URL.Host = serverURL.Host
		return server.Client().Transport.RoundTrip(cloned)
	}))
	root := RootCommand("test")
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		runErr = root.ParseAndRun(context.Background(), []string{"age-rating", "edit", "--id", "age-45", "--clear-grac-rating-classification-number", "--output", "json"})
	})
	if runErr != nil || stderr != "" || !json.Valid([]byte(stdout)) {
		t.Fatalf("run: %v; stdout=%s; stderr=%s", runErr, stdout, stderr)
	}
}

func TestAgeRatingEditClearGRACUsage(t *testing.T) {
	assertUsageExit(t, []string{"age-rating", "edit", "--id", "age-45", "--clear-grac-rating-classification-number", "--grac-rating-classification-number", "CC-2026-123"}, "--grac-rating-classification-number cannot be combined with --clear-grac-rating-classification-number")
	assertUsageExit(t, []string{"age-rating", "edit", "--id", "age-45", "--clear-grac-rating-classification-number=false"}, "at least one update flag is required")
}
