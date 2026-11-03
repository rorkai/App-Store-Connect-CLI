package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestSubscriptionsCreateAndSetupRequestAttributes(t *testing.T) {
	createArgs := []string{"subscriptions", "create", "--group-id", "group-1", "--reference-name", "Pro Monthly", "--product-id", "com.example.pro.monthly"}
	setupArgs := []string{"subscriptions", "setup", "--group-id", "group-1", "--reference-name", "Pro Monthly", "--product-id", "com.example.pro.monthly", "--no-verify", "--output", "json"}
	reviewFlags := []string{"--review-note", "  Same paywall structure  ", "--group-level", "3"}

	omitted := map[string]any{"name": "Pro Monthly", "productId": "com.example.pro.monthly"}
	provided := map[string]any{"name": "Pro Monthly", "productId": "com.example.pro.monthly", "reviewNote": "Same paywall structure", "groupLevel": float64(3)}

	tests := []struct {
		name string
		args []string
		want map[string]any
	}{
		{name: "create sends both fields", args: slices.Concat(createArgs, reviewFlags), want: provided},
		{name: "create omits unset fields", args: createArgs, want: omitted},
		{name: "setup sends both fields", args: slices.Concat(setupArgs, reviewFlags), want: provided},
		{name: "setup omits unset fields", args: setupArgs, want: omitted},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })

			var got map[string]any
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionGroups/group-1/subscriptions":
					return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
				case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptions":
					var payload struct {
						Data struct {
							Attributes map[string]any `json:"attributes"`
						} `json:"data"`
					}
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("decode request body: %v", err)
					}
					got = payload.Data.Attributes
					return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"subscriptions","id":"sub-1","attributes":{"name":"Pro Monthly","productId":"com.example.pro.monthly"}}}`), nil
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			})

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})
			if runErr != nil {
				t.Fatalf("run error: %v", runErr)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("attributes = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestSubscriptionsSetupComparesReviewNoteAndGroupLevelWithApple(t *testing.T) {
	const appleValues = `,"reviewNote":"Apple note","groupLevel":2`

	tests := []struct {
		name         string
		flags        []string
		existing     string
		readback     string
		wantErr      string
		wantExit     int
		wantFailStep string
	}{
		{
			name:         "existing subscription has a different group level",
			flags:        []string{"--group-level", "3", "--no-verify"},
			existing:     appleValues,
			wantErr:      "has a different group level",
			wantExit:     rootcmd.ExitConflict,
			wantFailStep: "create_subscription",
		},
		{
			name:         "existing subscription has a different review note",
			flags:        []string{"--review-note", "New note", "--no-verify"},
			existing:     appleValues,
			wantErr:      "has a different review note",
			wantExit:     rootcmd.ExitConflict,
			wantFailStep: "create_subscription",
		},
		{
			name:     "existing subscription matches the requested values",
			flags:    []string{"--review-note", "Apple note", "--group-level", "2", "--no-verify"},
			existing: appleValues,
		},
		{
			name:     "existing values are not compared when the flags are omitted",
			flags:    []string{"--no-verify"},
			existing: appleValues,
		},
		{
			name:         "readback reports a different group level",
			flags:        []string{"--group-level", "3"},
			readback:     appleValues,
			wantErr:      "group level mismatch",
			wantExit:     rootcmd.ExitError,
			wantFailStep: "verify_state",
		},
		{
			name:         "readback reports a different review note",
			flags:        []string{"--review-note", "New note"},
			readback:     appleValues,
			wantErr:      "review note mismatch",
			wantExit:     rootcmd.ExitError,
			wantFailStep: "verify_state",
		},
		{
			name:         "matching readback continues to the state check",
			flags:        []string{"--review-note", "Apple note", "--group-level", "2"},
			readback:     appleValues + `,"state":"MISSING_METADATA"`,
			wantErr:      "MISSING_METADATA",
			wantExit:     rootcmd.ExitError,
			wantFailStep: "verify_state",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })

			subscription := func(extra string) string {
				return `{"type":"subscriptions","id":"sub-1","attributes":{"name":"Pro Monthly","productId":"com.example.pro.monthly"` + extra + `}}`
			}
			created := false
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionGroups/group-1/subscriptions":
					if test.existing == "" {
						return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
					}
					return jsonHTTPResponse(http.StatusOK, `{"data":[`+subscription(test.existing)+`],"links":{"next":""}}`), nil
				case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptions":
					created = true
					return jsonHTTPResponse(http.StatusCreated, `{"data":`+subscription("")+`}`), nil
				case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionGroups/group-1":
					return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Pro"}}}`), nil
				case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/sub-1":
					return jsonHTTPResponse(http.StatusOK, `{"data":`+subscription(test.readback)+`}`), nil
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			})

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			args := []string{"subscriptions", "setup", "--group-id", "group-1", "--reference-name", "Pro Monthly", "--product-id", "com.example.pro.monthly", "--output", "json"}
			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(slices.Concat(args, test.flags)); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if test.existing != "" && created {
				t.Fatal("expected the existing subscription to be reused, not created")
			}
			if test.wantErr == "" {
				if runErr != nil {
					t.Fatalf("run error: %v", runErr)
				}
				return
			}
			if got := rootcmd.ExitCodeFromError(runErr); got != test.wantExit {
				t.Fatalf("exit code = %d, want %d (err=%v)", got, test.wantExit, runErr)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantErr, stderr)
			}
			var result subscriptionsSetupOutput
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatalf("parse setup result: %v\nstdout=%q", err, stdout)
			}
			if result.Status != "error" || result.FailedStep != test.wantFailStep {
				t.Fatalf("unexpected setup result: %+v", result)
			}
		})
	}
}
