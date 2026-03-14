package cmdtest

import (
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"
)

// TestExperimentalCommandsHaveStabilityLabel ensures every command surface
// that is marked experimental carries a consistent "[experimental]" prefix in
// its ShortHelp so that the label is visible in grouped root help, subcommand
// listings, and generated docs.
func TestExperimentalCommandsHaveStabilityLabel(t *testing.T) {
	root := RootCommand("1.2.3")

	webCmd := findSubcommand(root, "web")
	if webCmd == nil {
		t.Fatal("command [web] not found")
	}
	assertExperimentalCommandTree(t, webCmd, []string{"web"})

	cases := []struct {
		path []string // subcommand path from root
	}{
		{[]string{"screenshots", "run"}},
		{[]string{"screenshots", "capture"}},
		{[]string{"screenshots", "frame"}},
		{[]string{"screenshots", "list-frame-devices"}},
		{[]string{"screenshots", "review-generate"}},
		{[]string{"screenshots", "review-open"}},
		{[]string{"screenshots", "review-approve"}},
	}

	for _, tc := range cases {
		cmd := findSubcommand(root, tc.path...)
		assertExperimentalCommand(t, cmd, tc.path)
	}
}

func assertExperimentalCommandTree(t *testing.T, cmd *ffcli.Command, path []string) {
	t.Helper()

	assertExperimentalCommand(t, cmd, path)

	for _, sub := range cmd.Subcommands {
		assertExperimentalCommandTree(t, sub, append(path, sub.Name))
	}
}

func assertExperimentalCommand(t *testing.T, cmd *ffcli.Command, path []string) {
	t.Helper()

	if cmd == nil {
		t.Errorf("command %v not found", path)
		return
	}
	if !strings.HasPrefix(cmd.ShortHelp, "[experimental]") {
		t.Errorf("command %v: expected ShortHelp to start with [experimental], got %q", path, cmd.ShortHelp)
	}
}
