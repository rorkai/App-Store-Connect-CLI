package cmdtest

import (
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

// buildIDOnlyCommands lists the commands whose hidden `--build` alias was
// removed in 5.0.0 and stays removed: they also take `--build-number`.
var buildIDOnlyCommands = [][]string{
	{"builds", "info"},
	{"builds", "wait"},
	{"builds", "dsyms"},
	{"builds", "expire"},
	{"builds", "update"},
	{"builds", "add-groups"},
	{"builds", "remove-groups"},
	{"builds", "individual-testers", "list"},
	{"builds", "individual-testers", "add"},
	{"builds", "individual-testers", "remove"},
	{"builds", "metrics", "beta-usages"},
	{"builds", "icons", "list"},
	{"builds", "links", "view"},
	{"builds", "app", "view"},
	{"builds", "pre-release-version", "view"},
	{"builds", "beta-app-review-submission", "view"},
	{"builds", "build-beta-detail", "view"},
	{"builds", "app-encryption-declaration", "view"},
	{"builds", "test-notes", "list"},
	{"builds", "test-notes", "view"},
	{"builds", "test-notes", "create"},
	{"builds", "test-notes", "update"},
	{"builds", "test-notes", "delete"},
	{"publish", "testflight"},
}

func TestBuildIDOnlyCommandsRejectLegacyBuildAliasAsUnknownFlag(t *testing.T) {
	for _, path := range buildIDOnlyCommands {
		t.Run(strings.Join(path, " "), func(t *testing.T) {
			args := append(append([]string{}, path...), "--build", "BUILD_123")
			assertRemovedFlagGuidance(t, args, "--build")
		})
		if strings.Join(path, " ") == "builds wait" {
			continue // --newest is a hidden alias of --latest there
		}
		t.Run(strings.Join(path, " ")+" newest", func(t *testing.T) {
			var code int
			_, stderr := captureOutput(t, func() {
				code = cmd.Run(append(append([]string{}, path...), "--newest"), "1.2.3")
			})
			unregistered := strings.Contains(stderr, "unknown flag `--newest`") || strings.Contains(stderr, "`--newest` was removed in 5.0.0")
			if code != cmd.ExitUsage || !unregistered {
				t.Fatalf("exit=%d stderr=%q, want --newest rejected as unregistered", code, stderr)
			}
		})
	}
}
