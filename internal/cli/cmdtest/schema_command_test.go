package cmdtest

import (
	"encoding/json"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

type schemaEndpoint struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

func TestSchemaSupportsDocumentedTrailingPrettyFlag(t *testing.T) {
	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{"schema", "builds", "--pretty"}, "1.2.3")
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d with stderr %q", code, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.HasPrefix(stdout, "[\n  {") {
		t.Fatalf("expected pretty-printed JSON, got %q", stdout)
	}

	var endpoints []schemaEndpoint
	if err := json.Unmarshal([]byte(stdout), &endpoints); err != nil {
		t.Fatalf("unmarshal schema output: %v\nstdout=%s", err, stdout)
	}
	if len(endpoints) == 0 {
		t.Fatal("expected endpoints matching builds")
	}
}

func TestSchemaSupportsMethodFlagAfterQuery(t *testing.T) {
	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{"schema", "apps", "--method", "POST"}, "1.2.3")
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d with stderr %q", code, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var endpoints []schemaEndpoint
	if err := json.Unmarshal([]byte(stdout), &endpoints); err != nil {
		t.Fatalf("unmarshal schema output: %v\nstdout=%s", err, stdout)
	}
	if len(endpoints) == 0 {
		t.Fatal("expected POST endpoints matching apps")
	}
	for _, endpoint := range endpoints {
		if endpoint.Method != "POST" {
			t.Fatalf("expected only POST endpoints, got %#v", endpoints)
		}
	}
}

func TestSchemaExactQueriesReturnSingleEndpoint(t *testing.T) {
	tests := []struct {
		query string
		want  schemaEndpoint
	}{
		{query: "GET /v1/apps", want: schemaEndpoint{Method: "GET", Path: "/v1/apps"}},
		{query: "apps.list", want: schemaEndpoint{Method: "GET", Path: "/v1/apps"}},
		{query: "builds.appStoreVersion.get", want: schemaEndpoint{Method: "GET", Path: "/v1/builds/{id}/appStoreVersion"}},
		{query: "apps.metrics.betaTesterUsages.list", want: schemaEndpoint{Method: "GET", Path: "/v1/apps/{id}/metrics/betaTesterUsages"}},
		{query: "v2.gameCenterAchievements.get", want: schemaEndpoint{Method: "GET", Path: "/v2/gameCenterAchievements/{id}"}},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			var code int
			stdout, stderr := captureOutput(t, func() {
				code = rootcmd.Run([]string{"schema", tt.query}, "1.2.3")
			})

			if code != 0 {
				t.Fatalf("expected exit code 0, got %d with stderr %q", code, stderr)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if strings.Contains(stdout, "getAction") {
				t.Fatalf("expected index-only cardinality metadata to remain internal, got %q", stdout)
			}

			var endpoints []schemaEndpoint
			if err := json.Unmarshal([]byte(stdout), &endpoints); err != nil {
				t.Fatalf("unmarshal schema output: %v\nstdout=%s", err, stdout)
			}
			if len(endpoints) != 1 || endpoints[0] != tt.want {
				t.Fatalf("expected exact endpoint %#v, got %#v", tt.want, endpoints)
			}
		})
	}
}

func TestSchemaRejectsInvalidArgumentsWithUsageExit(t *testing.T) {
	tests := []struct {
		args    []string
		wantErr string
	}{
		{args: []string{"schema", "--list", "extra"}, wantErr: "--list does not accept a query"},
		{args: []string{"schema", "apps", "--unknown-schema-flag"}, wantErr: "flag provided but not defined: -unknown-schema-flag"},
		{args: []string{"schema", "--list", "--method", "DELTE"}, wantErr: "invalid --method"},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			assertUsageExit(t, tt.args, tt.wantErr)
		})
	}
}

func TestSchemaQueriesWithoutMatchesReturnEmptyJSONArray(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "literal query after flag terminator", args: []string{"schema", "--", "--list"}},
		{name: "no such endpoint", args: []string{"schema", "definitely-no-such-endpoint"}},
		{name: "fuzzy query does not search action suffixes", args: []string{"schema", "update"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var code int
			stdout, stderr := captureOutput(t, func() {
				code = rootcmd.Run(tt.args, "1.2.3")
			})

			if code != 0 {
				t.Fatalf("expected exit code 0, got %d with stderr %q", code, stderr)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if stdout != "[]\n" {
				t.Fatalf("expected empty JSON array, got %q", stdout)
			}
		})
	}
}

func TestSchemaPreservesFuzzyQueries(t *testing.T) {
	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{"schema", "builds"}, "1.2.3")
	})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d with stderr %q", code, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var endpoints []schemaEndpoint
	if err := json.Unmarshal([]byte(stdout), &endpoints); err != nil {
		t.Fatalf("unmarshal schema output: %v\nstdout=%s", err, stdout)
	}
	if len(endpoints) < 2 {
		t.Fatalf("expected multiple fuzzy matches, got %#v", endpoints)
	}
	found := false
	for _, endpoint := range endpoints {
		if endpoint.Method == "GET" && endpoint.Path == "/v1/builds" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected GET /v1/builds among fuzzy results, got %#v", endpoints)
	}
}

func TestRun_SchemaValidMethodReturnsSuccess(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	stdout, stderr := captureOutput(t, func() {
		code := rootcmd.Run([]string{"schema", "--list", "--method", "DELETE"}, "1.0.0")
		if code != rootcmd.ExitSuccess {
			t.Fatalf("expected exit code %d, got %d", rootcmd.ExitSuccess, code)
		}
	})

	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("expected empty stderr, got: %s", stderr)
	}
	if !strings.Contains(stdout, "\"method\":\"DELETE\"") {
		t.Fatalf("expected DELETE endpoints in output, got: %s", stdout)
	}
}
