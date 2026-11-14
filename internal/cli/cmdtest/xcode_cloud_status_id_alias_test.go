package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestXcodeCloudStatusRunIDReturnsJSONWithoutWarning(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/ciBuildRuns/run-1"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"ciBuildRuns","id":"run-1","attributes":{"number":42,"executionProgress":"COMPLETE","completionStatus":"SUCCEEDED"}}}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode-cloud", "status", "--run-id", "run-1", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	var result struct {
		BuildRunID string `json:"buildRunId"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode output %q: %v", stdout, err)
	}
	if result.BuildRunID != "run-1" {
		t.Fatalf("buildRunId = %q, want run-1", result.BuildRunID)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if requestCount := len(reg.Requests()); requestCount != 1 {
		t.Fatalf("request count = %d, want 1", requestCount)
	}
}

func TestXcodeCloudStatusRejectsRemovedIDAliasAsUnknownFlagBeforeNetwork(t *testing.T) {
	asctest.NewRegistry(t).Install()

	root := RootCommand("1.2.3")
	if root.FlagSet.Lookup("id") != nil {
		t.Fatal("root command unexpectedly defines --id")
	}
	status := findSubcommand(root, "xcode-cloud", "status")
	if status == nil {
		t.Fatal("expected xcode-cloud status command")
		return
	}
	if status.FlagSet.Lookup("id") != nil {
		t.Fatal("removed --id alias is still registered on xcode-cloud status")
	}
	if status.FlagSet.Lookup("run-id") == nil {
		t.Fatal("expected --run-id on xcode-cloud status")
	}

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{"xcode-cloud", "status", "--id", "run-1"}, "1.2.3")
	})
	if code != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, rootcmd.ExitUsage, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "`--id` was removed in 5.0.0") {
		t.Fatalf("stderr = %q, want removed-flag diagnostic", stderr)
	}
	if !strings.Contains(stderr, "--run-id") {
		t.Fatalf("stderr = %q, want --run-id suggestion", stderr)
	}
	if strings.Contains(stderr, "deprecated") {
		t.Fatalf("stderr still carries deprecation wording: %q", stderr)
	}
}

func TestXcodeCloudScmRepositoriesRelationshipsIsUnknownCommand(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
	}{
		{name: "group", args: []string{"xcode-cloud", "scm", "repositories", "relationships"}},
		{name: "git-references", args: []string{"xcode-cloud", "scm", "repositories", "relationships", "git-references", "--repo-id", "REPO_ID"}},
		{name: "pull-requests", args: []string{"xcode-cloud", "scm", "repositories", "relationships", "pull-requests", "--repo-id", "REPO_ID"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var code int
			stdout, stderr := captureOutput(t, func() {
				code = rootcmd.Run(tt.args, "1.2.3")
			})
			if code != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d; stderr=%q", code, rootcmd.ExitUsage, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, "relationships") {
				t.Fatalf("stderr = %q, want unknown-command diagnostic naming relationships", stderr)
			}
			if strings.Contains(stderr, "deprecated") || strings.Contains(stderr, "Warning") {
				t.Fatalf("stderr still treats relationships as a deprecated alias: %q", stderr)
			}
		})
	}
}
