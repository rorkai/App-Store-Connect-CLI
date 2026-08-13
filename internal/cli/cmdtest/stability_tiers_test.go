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
		{[]string{"screenshots", "plan"}},
		{[]string{"screenshots", "apply"}},
		{[]string{"xcode", "build"}},
		{[]string{"apps", "rename"}},
	}

	for _, tc := range cases {
		cmd := findSubcommand(root, tc.path...)
		assertExperimentalCommand(t, cmd, tc.path)
	}
}

func TestWebCommandsDoNotHaveExperimentalStabilityLabel(t *testing.T) {
	root := RootCommand("1.2.3")

	webCmd := findSubcommand(root, "web")
	if webCmd == nil {
		t.Fatal("command [web] not found")
	}
	assertCommandTreeDoesNotMentionExperimental(t, webCmd, []string{"web"})
}

func TestWebCommandsDoNotHaveEndpointWarningLabels(t *testing.T) {
	root := RootCommand("1.2.3")

	webCmd := findSubcommand(root, "web")
	if webCmd == nil {
		t.Fatal("command [web] not found")
	}
	assertCommandTreeDoesNotMentionEndpointWarnings(t, webCmd, []string{"web"})
}

func assertCommandTreeDoesNotMentionExperimental(t *testing.T, cmd *ffcli.Command, path []string) {
	t.Helper()

	assertCommandDoesNotMentionExperimental(t, cmd, path)

	for _, sub := range cmd.Subcommands {
		assertCommandTreeDoesNotMentionExperimental(t, sub, append(path, sub.Name))
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

func assertCommandDoesNotMentionExperimental(t *testing.T, cmd *ffcli.Command, path []string) {
	t.Helper()

	if cmd == nil {
		t.Errorf("command %v not found", path)
		return
	}
	if strings.Contains(strings.ToLower(cmd.ShortHelp), "experimental") {
		t.Errorf("command %v: expected ShortHelp not to mention experimental, got %q", path, cmd.ShortHelp)
	}
	if strings.Contains(strings.ToLower(cmd.LongHelp), "experimental") {
		t.Errorf("command %v: expected LongHelp not to mention experimental, got %q", path, cmd.LongHelp)
	}
}

func assertCommandTreeDoesNotMentionEndpointWarnings(t *testing.T, cmd *ffcli.Command, path []string) {
	t.Helper()

	assertCommandDoesNotMentionEndpointWarnings(t, cmd, path)

	for _, sub := range cmd.Subcommands {
		assertCommandTreeDoesNotMentionEndpointWarnings(t, sub, append(path, sub.Name))
	}
}

func assertCommandDoesNotMentionEndpointWarnings(t *testing.T, cmd *ffcli.Command, path []string) {
	t.Helper()

	if cmd == nil {
		t.Errorf("command %v not found", path)
		return
	}
	help := strings.ToLower(cmd.ShortHelp + "\n" + cmd.LongHelp)
	for _, token := range []string{
		"unofficial",
		"discouraged",
		"private endpoint",
		"private web",
		"not sanctioned",
		"at your own risk",
		"account restrictions",
		"production-critical",
		"break without notice",
	} {
		if strings.Contains(help, token) {
			t.Errorf("command %v: expected help not to mention %q, got %q", path, token, cmd.LongHelp)
		}
	}
}
