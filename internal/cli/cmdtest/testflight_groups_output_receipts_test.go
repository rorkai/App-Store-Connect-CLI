package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestTestFlightGroupsDestructiveCommandsPrintReceipts(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		path       string
		body       string
		wantStdout []string
		wantStderr string
	}{
		{
			name:       "delete json",
			args:       []string{"testflight", "groups", "delete", "--id", "group-1", "--confirm", "--output", "json"},
			path:       "/v1/betaGroups/group-1",
			wantStdout: []string{`{"id":"group-1","deleted":true}`},
			wantStderr: "Successfully deleted group group-1",
		},
		{
			name:       "delete table",
			args:       []string{"testflight", "groups", "delete", "--id", "group-1", "--confirm", "--output", "table"},
			path:       "/v1/betaGroups/group-1",
			wantStdout: []string{"ID", "Deleted", "group-1", "true"},
			wantStderr: "Successfully deleted group group-1",
		},
		{
			name:       "remove-testers json",
			args:       []string{"testflight", "groups", "remove-testers", "--group", "group-1", "--tester", "tester-1,tester-2", "--confirm", "--output", "json"},
			path:       "/v1/betaGroups/group-1/relationships/betaTesters",
			body:       `{"data":[{"type":"betaTesters","id":"tester-1"},{"type":"betaTesters","id":"tester-2"}]}`,
			wantStdout: []string{`{"groupId":"group-1","testerIds":["tester-1","tester-2"],"action":"removed"}`},
			wantStderr: "Successfully removed 2 tester(s) from group group-1",
		},
		{
			name:       "remove-testers table",
			args:       []string{"testflight", "groups", "remove-testers", "--group", "group-1", "--tester", "tester-1,tester-2", "--confirm", "--output", "table"},
			path:       "/v1/betaGroups/group-1/relationships/betaTesters",
			body:       `{"data":[{"type":"betaTesters","id":"tester-1"},{"type":"betaTesters","id":"tester-2"}]}`,
			wantStdout: []string{"Group ID", "Tester IDs", "Action", "group-1", "tester-1,tester-2", "removed"},
			wantStderr: "Successfully removed 2 tester(s) from group group-1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			matcher := asctest.DELETE(test.path)
			if test.body != "" {
				matcher = matcher.JSONBody(test.body)
			}
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(matcher, asctest.JSON(http.StatusNoContent, ""))

			stdout, stderr, err := runCommand(t, test.args)
			if err != nil {
				t.Fatalf("run error: %v", err)
			}

			for _, want := range test.wantStdout {
				if !strings.Contains(stdout, want) {
					t.Fatalf("expected stdout to contain %q, got %q", want, stdout)
				}
			}
			if !strings.Contains(stderr, test.wantStderr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantStderr, stderr)
			}
		})
	}
}
