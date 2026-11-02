package cmdtest

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestVersionsReviewTypeValidationBeforeAuth(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "create",
			args: []string{"versions", "create", "--app", "app-1", "--version", "1.0", "--review-type", "TESTFLIGHT"},
			want: "Error: versions create: --review-type must be one of: APP_STORE, NOTARIZATION",
		},
		{
			name: "update",
			args: []string{"versions", "update", "--version-id", "version-1", "--review-type", "TESTFLIGHT"},
			want: "Error: versions update: --review-type must be one of: APP_STORE, NOTARIZATION",
		},
		{
			name: "update with a blank value",
			args: []string{"versions", "update", "--version-id", "version-1", "--review-type", "   "},
			want: "Error: versions update: --review-type must be one of: APP_STORE, NOTARIZATION",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clearASCAuth(t)
			stdout, stderr := captureOutput(t, func() {
				if code := rootcmd.Run(test.args, "test"); code != rootcmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
				}
			})
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, test.want) {
				t.Fatalf("stderr = %q, want %q", stderr, test.want)
			}
			if strings.Contains(stderr, "missing authentication") {
				t.Fatalf("stderr = %q, validation must run before auth", stderr)
			}
		})
	}
}

// Omitted rows guard the other direction: a request without the flag must not
// carry the attribute, so an unrelated write never changes a version's review.
func TestVersionsReviewTypeRequestBody(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		method     string
		path       string
		wantType   any
		wantInBody bool
	}{
		{
			name:       "create sends the normalized value",
			args:       []string{"versions", "create", "--app", "app-1", "--version", "2.0.0", "--review-type", "notarization", "--output", "json"},
			method:     http.MethodPost,
			path:       "/v1/appStoreVersions",
			wantType:   "NOTARIZATION",
			wantInBody: true,
		},
		{
			name:       "update accepts the flag alone",
			args:       []string{"versions", "update", "--version-id", "version-1", "--review-type", "APP_STORE", "--output", "json"},
			method:     http.MethodPatch,
			path:       "/v1/appStoreVersions/version-1",
			wantType:   "APP_STORE",
			wantInBody: true,
		},
		{
			name:   "create omits it when unset",
			args:   []string{"versions", "create", "--app", "app-1", "--version", "2.0.0", "--output", "json"},
			method: http.MethodPost,
			path:   "/v1/appStoreVersions",
		},
		{
			name:   "update omits it when unset",
			args:   []string{"versions", "update", "--version-id", "version-1", "--copyright", "2026 Example", "--output", "json"},
			method: http.MethodPatch,
			path:   "/v1/appStoreVersions/version-1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var attributes map[string]any
			_, _, seen, runErr := runIfExistsCommand(t, test.args, func(req ifExistsRequest) (*http.Response, error) {
				if req.Method != test.method || req.Path != test.path {
					t.Fatalf("request = %s %s, want %s %s", req.Method, req.Path, test.method, test.path)
				}
				var body struct {
					Data struct {
						Attributes map[string]any `json:"attributes"`
					} `json:"data"`
				}
				if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
					t.Fatalf("unmarshal request body: %v; body=%s", err, req.Body)
				}
				attributes = body.Data.Attributes
				return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"2.0.0","platform":"IOS"}}}`)
			})
			if runErr != nil {
				t.Fatalf("run error: %v", runErr)
			}
			if len(seen) != 1 {
				t.Fatalf("requests = %+v, want one write", seen)
			}
			got, present := attributes["reviewType"]
			if present != test.wantInBody || got != test.wantType {
				t.Fatalf("reviewType = %#v (present %t), want %#v (present %t); attributes=%v", got, present, test.wantType, test.wantInBody, attributes)
			}
		})
	}
}

// When --review-type is the only updatable flag, dropping it would report a skip
// for a version whose review type never changed.
func TestVersionsCreateIfExistsUpdateAppliesReviewType(t *testing.T) {
	stdout, _, seen, runErr := runIfExistsCommand(t, []string{
		"versions", "create", "--app", "app-1", "--version", "2.0.0",
		"--review-type", "NOTARIZATION", "--if-exists", "update", "--output", "json",
	}, func(req ifExistsRequest) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.Path == "/v1/appStoreVersions":
			return jsonResponse(http.StatusConflict, versionsDuplicate409)
		case req.Method == http.MethodGet && req.Path == "/v1/apps/app-1/appStoreVersions":
			return jsonResponse(http.StatusOK, existingVersionsList)
		case req.Method == http.MethodPatch && req.Path == "/v1/appStoreVersions/version-existing":
			if !strings.Contains(req.Body, `"reviewType":"NOTARIZATION"`) {
				t.Fatalf("PATCH body = %s, want reviewType", req.Body)
			}
			return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-existing","attributes":{"versionString":"2.0.0","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION"}}}`)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.Path)
			return nil, nil
		}
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if len(seen) != 3 {
		t.Fatalf("requests = %+v, want POST, GET, PATCH", seen)
	}
	if !strings.Contains(stdout, `"action":"updated"`) {
		t.Fatalf("stdout = %q, want action=updated", stdout)
	}
}
