package cmdtest

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestSandboxViewByEmailDeduplicatesRepeatedResourceIDsAcrossPages(t *testing.T) {
	setupAuth(t)
	const nextURL = "https://api.appstoreconnect.apple.com/v2/sandboxTesters?cursor=next"
	requests := 0
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v2/sandboxTesters" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		requests++
		switch requests {
		case 1:
			return jsonHTTPResponse(http.StatusOK, `{"data":[
				{"type":"sandboxTesters","id":"tester-1","attributes":{"email":"same@example.com","firstName":"Jane"}}
			],"links":{"next":"`+nextURL+`"}}`), nil
		case 2:
			return jsonHTTPResponse(http.StatusOK, `{"data":[
				{"type":"sandboxTesters","id":"tester-1","attributes":{"email":"same@example.com","firstName":"Jane"}}
			],"links":{}}`), nil
		default:
			t.Fatalf("unexpected request after pagination: %s", req.URL.String())
			return nil, nil
		}
	}))

	stdout, stderr := captureOutput(t, func() {
		if code := cmd.Run([]string{"sandbox", "view", "--email", "same@example.com", "--output", "json"}, "1.2.3"); code != cmd.ExitSuccess {
			t.Fatalf("exit code = %d, want %d", code, cmd.ExitSuccess)
		}
	})
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
	if !strings.Contains(stdout, `"id":"tester-1"`) {
		t.Fatalf("expected selected tester in stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestSandboxViewByEmailKeepsDistinctIDsAmbiguousAcrossPages(t *testing.T) {
	setupAuth(t)
	const nextURL = "https://api.appstoreconnect.apple.com/v2/sandboxTesters?cursor=next"
	requests := 0
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v2/sandboxTesters" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		requests++
		switch requests {
		case 1:
			return jsonHTTPResponse(http.StatusOK, `{"data":[
				{"type":"sandboxTesters","id":" tester-2 ","attributes":{"email":"ambiguous@example.com"}}
			],"links":{"next":"`+nextURL+`"}}`), nil
		case 2:
			return jsonHTTPResponse(http.StatusOK, `{"data":[
				{"type":"sandboxTesters","id":" tester-1 ","attributes":{"email":"ambiguous@example.com"}}
			],"links":{}}`), nil
		default:
			t.Fatalf("unexpected request after pagination: %s", req.URL.String())
			return nil, nil
		}
	}))

	stdout, stderr := captureOutput(t, func() {
		if code := cmd.Run([]string{"sandbox", "view", "--email", "ambiguous@example.com"}, "1.2.3"); code != cmd.ExitError {
			t.Fatalf("exit code = %d, want %d", code, cmd.ExitError)
		}
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	for _, want := range []string{"tester-1", "tester-2"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr missing candidate %q: %s", want, stderr)
		}
	}
	if first, second := strings.Index(stderr, "tester-1"), strings.Index(stderr, "tester-2"); first < 0 || second < 0 || first > second {
		t.Fatalf("stderr candidates are not sorted by normalized ID: %s", stderr)
	}
}

func TestSandboxViewByEmailRejectsBlankResourceID(t *testing.T) {
	setupAuth(t)
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v2/sandboxTesters" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		return jsonHTTPResponse(http.StatusOK, `{"data":[
			{"type":"sandboxTesters","id":"   ","attributes":{"email":"malformed@example.com"}}
		],"links":{}}`), nil
	}))

	stdout, stderr := captureOutput(t, func() {
		if code := cmd.Run([]string{"sandbox", "view", "--email", "malformed@example.com"}, "1.2.3"); code != cmd.ExitError {
			t.Fatalf("exit code = %d, want %d", code, cmd.ExitError)
		}
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "empty ID") {
		t.Fatalf("stderr missing empty ID diagnostic: %s", stderr)
	}
}

func TestSandboxByEmailNormalizesID(t *testing.T) {
	tests := []struct {
		name          string
		email         string
		args          []string
		mutation      func(t *testing.T, req *http.Request) *http.Response
		wantStdout    string
		notWantStdout string
	}{
		{
			name:          "view",
			email:         "view@example.com",
			args:          []string{"sandbox", "view", "--email", "view@example.com", "--output", "json"},
			wantStdout:    `"id":"tester-42"`,
			notWantStdout: `"id":" tester-42 "`,
		},
		{
			name:  "update",
			email: "update@example.com",
			args:  []string{"sandbox", "update", "--email", "update@example.com", "--territory", "USA", "--output", "json"},
			mutation: func(t *testing.T, req *http.Request) *http.Response {
				if req.Method != http.MethodPatch || req.URL.Path != "/v2/sandboxTesters/tester-42" {
					t.Fatalf("expected normalized update request, got %s %s", req.Method, req.URL.Path)
				}
				var body struct {
					Data struct {
						ID string `json:"id"`
					} `json:"data"`
				}
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					t.Fatalf("decode update body: %v", err)
				}
				if body.Data.ID != "tester-42" {
					t.Fatalf("expected normalized update body ID, got %q", body.Data.ID)
				}
				return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"sandboxTesters","id":"tester-42","attributes":{"email":"update@example.com","territory":"USA"}}}`)
			},
			wantStdout: `"id":"tester-42"`,
		},
		{
			name:  "clear-history",
			email: "clear@example.com",
			args:  []string{"sandbox", "clear-history", "--email", "clear@example.com", "--confirm", "--output", "json"},
			mutation: func(t *testing.T, req *http.Request) *http.Response {
				if req.Method != http.MethodPost || req.URL.Path != "/v2/sandboxTestersClearPurchaseHistoryRequest" {
					t.Fatalf("unexpected clear-history request: %s %s", req.Method, req.URL.Path)
				}
				var body struct {
					Data struct {
						Relationships struct {
							SandboxTesters struct {
								Data []struct {
									ID string `json:"id"`
								} `json:"data"`
							} `json:"sandboxTesters"`
						} `json:"relationships"`
					} `json:"data"`
				}
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					t.Fatalf("decode clear-history body: %v", err)
				}
				if len(body.Data.Relationships.SandboxTesters.Data) != 1 || body.Data.Relationships.SandboxTesters.Data[0].ID != "tester-42" {
					t.Fatalf("expected normalized clear-history ID, got %+v", body.Data.Relationships.SandboxTesters.Data)
				}
				return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"sandboxTestersClearPurchaseHistoryRequest","id":"request-1"}}`)
			},
			wantStdout: `"testerId":"tester-42"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method == http.MethodGet {
					if req.URL.Path != "/v2/sandboxTesters" {
						t.Fatalf("unexpected list request: %s", req.URL.String())
					}
					return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"sandboxTesters","id":" tester-42 ","attributes":{"email":"`+test.email+`"}}],"links":{}}`), nil
				}
				if test.mutation == nil {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
				}
				return test.mutation(t, req), nil
			}))

			stdout, stderr := captureOutput(t, func() {
				if code := cmd.Run(test.args, "1.2.3"); code != cmd.ExitSuccess {
					t.Fatalf("exit code = %d, want %d", code, cmd.ExitSuccess)
				}
			})
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if !strings.Contains(stdout, test.wantStdout) {
				t.Fatalf("expected stdout to contain %q, got %q", test.wantStdout, stdout)
			}
			if test.notWantStdout != "" && strings.Contains(stdout, test.notWantStdout) {
				t.Fatalf("expected stdout not to contain %q, got %q", test.notWantStdout, stdout)
			}
		})
	}
}
