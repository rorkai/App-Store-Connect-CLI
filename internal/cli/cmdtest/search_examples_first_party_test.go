package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

// TestSearchExamplesAreFirstPartyForFixedCommands pins the commands whose help
// used to open the Examples block with another command's invocation. `asc
// search` indexes every line under "Examples:" as that command's own example,
// so an agent taking examples[0] would run the wrong command. Prerequisite
// invocations belong in the prose above the block.
//
// Not covered here: `asc doctor` shares its command object with `asc auth
// doctor`, so its examples document the canonical path on purpose. `asc
// screenshots download` is excluded while PR #2067 rewrites that file.
func TestSearchExamplesAreFirstPartyForFixedCommands(t *testing.T) {
	tests := []struct {
		command string
		query   string
	}{
		{command: "asc apps", query: "list and manage apps in app store connect"},
		{command: "asc release", query: "release stage workflow"},
		{command: "asc subscriptions", query: "subscriptions groups manage"},
		{command: "asc subscriptions review", query: "subscriptions review screenshots"},
		{command: "asc screenshots list", query: "screenshots list localization"},
		{command: "asc screenshots upload", query: "screenshots upload localization"},
		{command: "asc video-previews list", query: "video previews list"},
		{command: "asc video-previews upload", query: "video previews upload"},
		{command: "asc video-previews download", query: "video previews download"},
	}

	for _, test := range tests {
		t.Run(test.command, func(t *testing.T) {
			result, ok := searchResultForCommand(t, test.query, test.command)
			if !ok {
				t.Fatalf("search %q did not return %q", test.query, test.command)
			}
			if len(result.Examples) == 0 {
				t.Fatalf("expected indexed examples for %q", test.command)
			}
			for i, example := range result.Examples {
				if example != test.command && !strings.HasPrefix(example, test.command+" ") {
					t.Fatalf("examples[%d] for %q is %q; every example must invoke the command itself", i, test.command, example)
				}
			}
		})
	}
}

func searchResultForCommand(t *testing.T, query, command string) (searchResult, bool) {
	t.Helper()

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"search", "--output", "json", "--limit", "25", query}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var response searchResponse
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("failed to unmarshal search JSON: %v\nstdout=%s", err, stdout)
	}
	for _, result := range response.Results {
		if result.Command == command {
			return result, true
		}
	}
	return searchResult{}, false
}
