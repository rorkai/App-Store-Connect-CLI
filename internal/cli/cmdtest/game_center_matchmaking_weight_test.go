package cmdtest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestGameCenterMatchmakingRulesRejectInvalidWeightBeforeClient(t *testing.T) {
	tests := []struct {
		weight  string
		message string
	}{
		{weight: "NaN", message: "--weight must be a finite number"},
		{weight: "+Inf", message: "--weight must be a finite number"},
		{weight: "-Inf", message: "--weight must be a finite number"},
		{weight: "invalid", message: "--weight must be a number"},
	}

	for _, command := range []string{"create", "update"} {
		for _, test := range tests {
			t.Run(command+" "+test.weight, func(t *testing.T) {
				clearASCAuth(t)
				clientFactoryCalls := 0
				factory := func() (*asc.Client, error) {
					clientFactoryCalls++
					return nil, fmt.Errorf("client should not be created")
				}

				stdout, stderr, code := runWithDeps(t, factory, gameCenterMatchmakingRuleArgs(command, test.weight)...)
				if code != rootcmd.ExitUsage {
					t.Errorf("exit code = %d, want %d", code, rootcmd.ExitUsage)
				}

				if stdout != "" {
					t.Errorf("stdout = %q, want empty", stdout)
				}
				errorLine, _, _ := strings.Cut(stderr, "\n")
				if got, want := errorLine+"\n", "Error: "+test.message+"\n"; got != want {
					t.Errorf("stderr error line = %q, want %q; full stderr = %q", got, want, stderr)
				}
				if clientFactoryCalls != 0 {
					t.Errorf("client factory calls = %d, want 0", clientFactoryCalls)
				}
			})
		}
	}
}

func TestGameCenterMatchmakingRulesSendFiniteWeight(t *testing.T) {
	tests := []struct {
		name       string
		command    string
		weight     string
		method     string
		path       string
		statusCode int
		wantBody   string
		extraArgs  []string
	}{
		{
			name:       "create negative weight",
			command:    "create",
			weight:     "-0.5",
			method:     http.MethodPost,
			path:       "/v1/gameCenterMatchmakingRules",
			statusCode: http.StatusCreated,
			wantBody:   `{"data":{"type":"gameCenterMatchmakingRules","attributes":{"referenceName":"Rule","description":"Match","type":"MATCH","expression":"true","weight":-0.5},"relationships":{"ruleSet":{"data":{"type":"gameCenterMatchmakingRuleSets","id":"RULE_SET_ID"}}}}}`,
		},
		{
			name:       "create omitted weight",
			command:    "create",
			method:     http.MethodPost,
			path:       "/v1/gameCenterMatchmakingRules",
			statusCode: http.StatusCreated,
			wantBody:   `{"data":{"type":"gameCenterMatchmakingRules","attributes":{"referenceName":"Rule","description":"Match","type":"MATCH","expression":"true"},"relationships":{"ruleSet":{"data":{"type":"gameCenterMatchmakingRuleSets","id":"RULE_SET_ID"}}}}}`,
		},
		{
			name:       "create positive weight",
			command:    "create",
			weight:     "0.5",
			method:     http.MethodPost,
			path:       "/v1/gameCenterMatchmakingRules",
			statusCode: http.StatusCreated,
			wantBody:   `{"data":{"type":"gameCenterMatchmakingRules","attributes":{"referenceName":"Rule","description":"Match","type":"MATCH","expression":"true","weight":0.5},"relationships":{"ruleSet":{"data":{"type":"gameCenterMatchmakingRuleSets","id":"RULE_SET_ID"}}}}}`,
		},
		{
			name:       "update zero weight",
			command:    "update",
			weight:     "0",
			method:     http.MethodPatch,
			path:       "/v1/gameCenterMatchmakingRules/RULE_ID",
			statusCode: http.StatusOK,
			wantBody:   `{"data":{"type":"gameCenterMatchmakingRules","id":"RULE_ID","attributes":{"weight":0}}}`,
		},
		{
			name:       "update exponent weight",
			command:    "update",
			weight:     "1e2",
			method:     http.MethodPatch,
			path:       "/v1/gameCenterMatchmakingRules/RULE_ID",
			statusCode: http.StatusOK,
			wantBody:   `{"data":{"type":"gameCenterMatchmakingRules","id":"RULE_ID","attributes":{"weight":100}}}`,
		},
		{
			name:       "update omitted weight",
			command:    "update",
			method:     http.MethodPatch,
			path:       "/v1/gameCenterMatchmakingRules/RULE_ID",
			statusCode: http.StatusOK,
			wantBody:   `{"data":{"type":"gameCenterMatchmakingRules","id":"RULE_ID","attributes":{"description":"New match"}}}`,
			extraArgs:  []string{"--description", "New match"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
			requestCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				requestCount++
				if req.Method != test.method || req.URL.Path != test.path {
					t.Errorf("request = %s %s, want %s %s", req.Method, req.URL.Path, test.method, test.path)
					http.Error(w, "unexpected request", http.StatusBadRequest)
					return
				}
				if authorization := req.Header.Get("Authorization"); !strings.HasPrefix(authorization, "Bearer ") || authorization == "Bearer " {
					t.Errorf("Authorization = %q, want non-empty bearer token", authorization)
				}
				body, err := io.ReadAll(req.Body)
				if err != nil {
					t.Errorf("read request body: %v", err)
				} else if got := strings.TrimSpace(string(body)); got != test.wantBody {
					t.Errorf("request body = %s, want %s", got, test.wantBody)
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(test.statusCode)
				_, _ = io.WriteString(w, `{"data":{"type":"gameCenterMatchmakingRules","id":"RULE_ID","attributes":{"referenceName":"Rule","description":"Match","type":"MATCH","expression":"true"}},"links":{}}`)
			}))
			t.Cleanup(server.Close)

			client := newGameCenterMatchmakingTestClient(t, server)
			clientFactoryCalls := 0
			factory := func() (*asc.Client, error) {
				clientFactoryCalls++
				return client, nil
			}

			args := append(gameCenterMatchmakingRuleArgs(test.command, test.weight), test.extraArgs...)
			args = append(args, "--output", "json")
			stdout, stderr, code := runWithDeps(t, factory, args...)
			if code != rootcmd.ExitSuccess {
				t.Errorf("exit code = %d, want %d", code, rootcmd.ExitSuccess)
			}

			if stderr != "" {
				t.Errorf("stderr = %q, want empty", stderr)
			}
			var response asc.GameCenterMatchmakingRuleResponse
			if err := json.Unmarshal([]byte(stdout), &response); err != nil {
				t.Fatalf("unmarshal stdout: %v; stdout=%q", err, stdout)
			}
			if response.Data.ID != "RULE_ID" || response.Data.Type != asc.ResourceTypeGameCenterMatchmakingRules {
				t.Errorf("response data = %+v, want gameCenterMatchmakingRules RULE_ID", response.Data)
			}
			if clientFactoryCalls != 1 {
				t.Errorf("client factory calls = %d, want 1", clientFactoryCalls)
			}
			if requestCount != 1 {
				t.Errorf("request count = %d, want 1", requestCount)
			}
		})
	}
}

func gameCenterMatchmakingRuleArgs(command, weight string) []string {
	args := []string{"game-center", "matchmaking", "rules", command}
	if command == "create" {
		args = append(
			args,
			"--rule-set-id", "RULE_SET_ID",
			"--reference-name", "Rule",
			"--description", "Match",
			"--type", "MATCH",
			"--expression", "true",
		)
	} else {
		args = append(args, "--id", "RULE_ID")
	}
	if weight != "" {
		args = append(args, "--weight", weight)
	}
	return args
}

func newGameCenterMatchmakingTestClient(t *testing.T, server *httptest.Server) *asc.Client {
	t.Helper()
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Scheme != "https" || req.URL.Host != "api.appstoreconnect.apple.com" {
			t.Errorf("request URL = %s, want official App Store Connect host", req.URL.String())
		}
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
