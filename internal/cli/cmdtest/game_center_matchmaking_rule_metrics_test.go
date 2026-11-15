package cmdtest

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestGameCenterMatchmakingRuleMetricsRejectUnsupportedResultDimension(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "number results filter result",
			args:    []string{"game-center", "matchmaking", "metrics", "rule-number-results", "--rule-id", "rule-1", "--granularity", "P1D", "--filter-result", "MATCHED"},
			wantErr: "game-center matchmaking metrics rule-number-results: --filter-result is not supported by this endpoint (supported filters: --filter-queue)",
		},
		{
			name:    "rule errors filter result",
			args:    []string{"game-center", "matchmaking", "metrics", "rule-errors", "--rule-id", "rule-1", "--granularity", "P1D", "--filter-result", "MATCHED"},
			wantErr: "game-center matchmaking metrics rule-errors: --filter-result is not supported by this endpoint (supported filters: --filter-queue)",
		},
		{
			name:    "number results group by result",
			args:    []string{"game-center", "matchmaking", "metrics", "rule-number-results", "--rule-id", "rule-1", "--granularity", "P1D", "--group-by", "result"},
			wantErr: `game-center matchmaking metrics rule-number-results: unsupported --group-by value "result" (supported values: gameCenterMatchmakingQueue)`,
		},
		{
			name:    "rule errors group by result",
			args:    []string{"game-center", "matchmaking", "metrics", "rule-errors", "--rule-id", "rule-1", "--granularity", "P1D", "--group-by", "gameCenterMatchmakingQueue,result"},
			wantErr: `game-center matchmaking metrics rule-errors: unsupported --group-by value "result" (supported values: gameCenterMatchmakingQueue)`,
		},
		{
			name:    "boolean results unknown group by",
			args:    []string{"game-center", "matchmaking", "metrics", "rule-boolean-results", "--rule-id", "rule-1", "--granularity", "P1D", "--group-by", "gameCenterDetail"},
			wantErr: `game-center matchmaking metrics rule-boolean-results: unsupported --group-by value "gameCenterDetail" (supported values: result, gameCenterMatchmakingQueue)`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientFactoryCalled := false
			factory := func() (*asc.Client, error) {
				clientFactoryCalled = true
				return nil, errors.New("client factory must not run during validation")
			}

			assertUsageExitWithDeps(t, factory, test.args, test.wantErr)
			if clientFactoryCalled {
				t.Fatal("client factory ran before rule metrics validation")
			}
		})
	}
}

func TestGameCenterMatchmakingRuleMetricsBuildDocumentedQueries(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantPath  string
		wantQuery url.Values
	}{
		{
			name: "boolean results supports result dimension",
			args: []string{
				"game-center", "matchmaking", "metrics", "rule-boolean-results",
				"--rule-id", "rule-1",
				"--granularity", "P1D",
				"--group-by", "result,gameCenterMatchmakingQueue",
				"--filter-result", "true",
				"--filter-queue", "queue-1",
				"--output", "json",
			},
			wantPath: "/v1/gameCenterMatchmakingRules/rule-1/metrics/matchmakingBooleanRuleResults",
			wantQuery: url.Values{
				"granularity":                        {"P1D"},
				"groupBy":                            {"result,gameCenterMatchmakingQueue"},
				"filter[result]":                     {"true"},
				"filter[gameCenterMatchmakingQueue]": {"queue-1"},
			},
		},
		{
			name: "number results omits result dimension",
			args: []string{
				"game-center", "matchmaking", "metrics", "rule-number-results",
				"--rule-id", "rule-1",
				"--granularity", "P1D",
				"--group-by", "gameCenterMatchmakingQueue",
				"--filter-queue", "queue-1",
				"--output", "json",
			},
			wantPath: "/v1/gameCenterMatchmakingRules/rule-1/metrics/matchmakingNumberRuleResults",
			wantQuery: url.Values{
				"granularity":                        {"P1D"},
				"groupBy":                            {"gameCenterMatchmakingQueue"},
				"filter[gameCenterMatchmakingQueue]": {"queue-1"},
			},
		},
		{
			name: "rule errors omits result dimension",
			args: []string{
				"game-center", "matchmaking", "metrics", "rule-errors",
				"--rule-id", "rule-1",
				"--granularity", "P1D",
				"--group-by", "gameCenterMatchmakingQueue",
				"--output", "json",
			},
			wantPath: "/v1/gameCenterMatchmakingRules/rule-1/metrics/matchmakingRuleErrors",
			wantQuery: url.Values{
				"granularity": {"P1D"},
				"groupBy":     {"gameCenterMatchmakingQueue"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method != http.MethodGet || req.URL.Path != test.wantPath {
					t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
					http.Error(w, "unexpected request", http.StatusBadRequest)
					return
				}
				if got := req.URL.Query(); got.Encode() != test.wantQuery.Encode() {
					t.Errorf("query = %q, want %q", got.Encode(), test.wantQuery.Encode())
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"data":[{"dataPoints":[{"start":"2026-01-01","values":{"count":7}}]}],"links":{"self":"metrics-self"}}`)
			}))
			t.Cleanup(server.Close)

			stdout, stderr, err := runRootWithDeps(t, stubServerClient(t, server), test.args...)
			if err != nil {
				t.Fatalf("run error: %v", err)
			}
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			if !strings.Contains(stdout, `"count":7`) {
				t.Fatalf("stdout = %q, want metrics response", stdout)
			}
		})
	}
}
