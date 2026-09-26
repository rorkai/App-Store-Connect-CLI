package completion

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestCompletionNodesIncludeNestedCommandsAndVisibleFlags(t *testing.T) {
	rootFlags := flag.NewFlagSet("asc", flag.ContinueOnError)
	rootFlags.String("profile", "", "Authentication profile")
	rootFlags.Bool("version", false, "Print version")

	appsFlags := flag.NewFlagSet("apps", flag.ContinueOnError)
	appsFlags.String("app", "", "App ID")
	appsFlags.Bool("paginate", false, "Fetch every page")
	appsFlags.String("legacy-app", "", "Deprecated alias")
	shared.HideFlagFromHelp(appsFlags.Lookup("legacy-app"))

	viewFlags := flag.NewFlagSet("view", flag.ContinueOnError)
	viewFlags.String("id", "", "Resource ID")
	nodes := completionNodes([]*ffcli.Command{
		{
			Name:    "apps",
			FlagSet: appsFlags,
			Subcommands: []*ffcli.Command{
				{Name: "view", FlagSet: viewFlags},
				{Name: "old-view", ShortHelp: "DEPRECATED: use view"},
				{Name: "removed-view", ShortHelp: "REMOVED: use view"},
				{Name: "compat-view", ShortHelp: "Compatibility alias: use view"},
				{Name: "compat-views", ShortHelp: "Compatibility aliases for view"},
			},
		},
		{Name: "completion"},
		{Name: "old-apps", ShortHelp: "DEPRECATED: use apps"},
		nil,
		{Name: "   "},
	}, rootFlags)

	root := findCompletionNode(t, nodes, "")
	if got, want := root.subcommands, []string{"apps", "completion"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("root subcommands = %v, want %v", got, want)
	}
	if got, want := root.flags, []string{"--profile", "--version"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("root flags = %v, want %v", got, want)
	}
	if got, want := root.valueFlags, []string{"--profile"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("root value flags = %v, want %v", got, want)
	}
	apps := findCompletionNode(t, nodes, "apps")
	if got, want := apps.subcommands, []string{"view"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("apps subcommands = %v, want %v", got, want)
	}
	if got, want := apps.flags, []string{"--app", "--paginate", "--profile"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("apps flags = %v, want %v", got, want)
	}
	if got, want := apps.valueFlags, []string{"--app", "--profile"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("apps value flags = %v, want %v", got, want)
	}
	view := findCompletionNode(t, nodes, "apps view")
	if got, want := view.flags, []string{"--id", "--profile"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("view flags = %v, want %v", got, want)
	}

	serialized := completionNodeText(nodes)
	for _, hidden := range []string{"legacy-app", "old-view", "removed-view", "compat-view", "compat-views", "old-apps"} {
		if strings.Contains(serialized, hidden) {
			t.Fatalf("hidden lifecycle entry %q leaked into completion nodes: %s", hidden, serialized)
		}
	}
}

func TestCompletionCommandValidationAndOutput(t *testing.T) {
	resolveCalls := 0
	resolve := func() []*ffcli.Command {
		resolveCalls++
		return []*ffcli.Command{
			{Name: "apps"},
			{Name: "builds"},
		}
	}
	rootFlags := flag.NewFlagSet("asc", flag.ContinueOnError)
	rootFlags.String("profile", "", "Authentication profile")
	resolveRootFlags := func() *flag.FlagSet { return rootFlags }
	cmd := CompletionCommand(resolve, resolveRootFlags)
	if resolveCalls != 0 {
		t.Fatalf("command tree resolved during command construction")
	}

	// Missing shell should fail with ErrHelp.
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}
	if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp for missing shell, got %v", err)
	}
	if resolveCalls != 0 {
		t.Fatalf("command tree resolved before shell validation")
	}

	// Unsupported shell should fail with ErrHelp.
	cmd = CompletionCommand(resolve, resolveRootFlags)
	if err := cmd.FlagSet.Parse([]string{"--shell", "tcsh"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}
	if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp for unsupported shell, got %v", err)
	}
	if resolveCalls != 0 {
		t.Fatalf("command tree resolved for unsupported shell")
	}

	// Supported shell should print script and succeed.
	cmd = CompletionCommand(resolve, resolveRootFlags)
	if err := cmd.FlagSet.Parse([]string{"--shell", "bash"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}
	stdout := captureStdout(t, func() error {
		return cmd.Exec(context.Background(), nil)
	})
	if !strings.Contains(stdout, "complete -F _asc_completions asc") {
		t.Fatalf("expected bash completion script output, got %q", stdout)
	}
	if !strings.Contains(stdout, "--profile") {
		t.Fatalf("expected root flag completion data, got %q", stdout)
	}
	if strings.Contains(stdout, "apps builds completion") {
		t.Fatalf("completion command was synthesized despite being absent from the resolver: %q", stdout)
	}
	if resolveCalls != 1 {
		t.Fatalf("command tree resolve calls = %d, want 1", resolveCalls)
	}
}

func TestCompletionScriptsContainNestedPathsAndFlags(t *testing.T) {
	nodes := []completionNode{
		{path: "", subcommands: []string{"apps"}, flags: []string{"--profile", "--version"}, valueFlags: []string{"--profile"}},
		{path: "apps", subcommands: []string{"view"}, flags: []string{"--app", "--paginate"}, valueFlags: []string{"--app"}},
		{path: "apps view", flags: []string{"--id"}, valueFlags: []string{"--id"}},
	}

	tests := []struct {
		name   string
		script string
		marker string
	}{
		{name: "bash", script: bashScript(nodes), marker: "complete -F _asc_completions asc"},
		{name: "zsh", script: zshScript(nodes), marker: "compdef _asc asc"},
		{name: "fish", script: fishScript(nodes), marker: "complete -c asc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, want := range []string{tt.marker, "apps view", "--app", "--paginate", "--id"} {
				if !strings.Contains(tt.script, want) {
					t.Fatalf("script missing %q:\n%s", want, tt.script)
				}
			}
		})
	}
}

func TestFishCompletionGuardsUnknownMetadataPaths(t *testing.T) {
	script := fishScript([]completionNode{{path: "", subcommands: []string{"apps"}}})
	if got := strings.Count(script, `if test -z "$index"`); got != 2 {
		t.Fatalf("fish completion index guards = %d, want 2:\n%s", got, script)
	}
}

func TestCompletionScriptsEscapeGeneratedMetadata(t *testing.T) {
	nodes := []completionNode{
		{path: "", subcommands: []string{"what's-new"}},
		{path: "what's-new", flags: []string{"--owner's-id"}, valueFlags: []string{"--owner's-id"}},
	}

	for name, script := range map[string]string{
		"bash": bashScript(nodes),
		"zsh":  zshScript(nodes),
	} {
		if !strings.Contains(script, `'what'"'"'s-new'`) || !strings.Contains(script, `'--owner'"'"'s-id'`) {
			t.Fatalf("%s script did not shell-escape metadata:\n%s", name, script)
		}
	}
	fish := fishScript(nodes)
	if !strings.Contains(fish, `'what\'s-new'`) || !strings.Contains(fish, `'--owner\'s-id'`) {
		t.Fatalf("fish script did not shell-escape metadata:\n%s", fish)
	}
}

func TestBashCompletionTracksNestedPathsAndSkipsFlagValues(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash is not installed")
	}

	nodes := []completionNode{
		{path: "", subcommands: []string{"apps"}, flags: []string{"--profile", "--version"}, valueFlags: []string{"--profile"}},
		{path: "apps", subcommands: []string{"view"}, flags: []string{"--app"}, valueFlags: []string{"--app"}},
		{path: "apps view", flags: []string{"--id"}, valueFlags: []string{"--id"}},
	}
	scriptPath := filepath.Join(t.TempDir(), "asc-completion.bash")
	if err := os.WriteFile(scriptPath, []byte(bashScript(nodes)), 0o600); err != nil {
		t.Fatalf("write completion script: %v", err)
	}

	command := `source "$1"
printf '%s\n' ROOT
_asc_completion_candidates
printf '%s\n' NESTED
_asc_completion_candidates apps view
printf '%s\n' FLAG_VALUE_MATCHES_SUBCOMMAND
_asc_completion_candidates apps --app view
printf '%s\n' ROOT_FLAG_VALUE_MATCHES_SUBCOMMAND
_asc_completion_candidates --profile apps
`
	out, err := exec.Command(bash, "--noprofile", "--norc", "-c", command, "bash", scriptPath).CombinedOutput()
	if err != nil {
		t.Fatalf("exercise bash completion: %v\n%s", err, out)
	}
	got := string(out)
	for _, want := range []string{
		"ROOT\napps\n",
		"NESTED\n--id\n",
		"FLAG_VALUE_MATCHES_SUBCOMMAND\nview\n--app\n",
		"ROOT_FLAG_VALUE_MATCHES_SUBCOMMAND\napps\n--profile\n--version\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("bash resolver output missing %q:\n%s", want, got)
		}
	}
}

func TestZshCompletionPassesPriorWordsAsSeparateArguments(t *testing.T) {
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh is not installed")
	}

	nodes := []completionNode{
		{path: "", subcommands: []string{"apps"}, flags: []string{"--profile", "--version"}, valueFlags: []string{"--profile"}},
		{path: "apps", subcommands: []string{"view"}, flags: []string{"--app"}, valueFlags: []string{"--app"}},
		{path: "apps view", flags: []string{"--id"}, valueFlags: []string{"--id"}},
	}
	scriptPath := filepath.Join(t.TempDir(), "_asc")
	if err := os.WriteFile(scriptPath, []byte(zshScript(nodes)), 0o600); err != nil {
		t.Fatalf("write completion script: %v", err)
	}

	command := `compdef() { :; }
compadd() { print -rl -- "$@"; }
source "$1"
printf '%s\n' NESTED
words=(asc apps view '')
CURRENT=4
_asc
printf '%s\n' FLAG_VALUE_MATCHES_SUBCOMMAND
words=(asc apps --app view '')
CURRENT=5
_asc
printf '%s\n' ROOT_FLAG_VALUE_MATCHES_SUBCOMMAND
words=(asc --profile apps '')
CURRENT=4
_asc
`
	out, err := exec.Command(zsh, "-f", "-c", command, "zsh", scriptPath).CombinedOutput()
	if err != nil {
		t.Fatalf("exercise zsh completion: %v\n%s", err, out)
	}
	got := string(out)
	for _, want := range []string{
		"NESTED\n--\n--id\n",
		"FLAG_VALUE_MATCHES_SUBCOMMAND\n--\nview\n--app\n",
		"ROOT_FLAG_VALUE_MATCHES_SUBCOMMAND\n--\napps\n--profile\n--version\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("zsh wrapper output missing %q:\n%s", want, got)
		}
	}
}

func findCompletionNode(t *testing.T, nodes []completionNode, path string) completionNode {
	t.Helper()
	for _, node := range nodes {
		if node.path == path {
			return node
		}
	}
	t.Fatalf("completion node %q not found in %v", path, nodes)
	return completionNode{}
}

func completionNodeText(nodes []completionNode) string {
	var b strings.Builder
	for _, node := range nodes {
		b.WriteString(node.path)
		b.WriteString(strings.Join(node.subcommands, " "))
		b.WriteString(strings.Join(node.flags, " "))
		b.WriteString(strings.Join(node.valueFlags, " "))
	}
	return b.String()
}

func captureStdout(t *testing.T, fn func() error) string {
	t.Helper()

	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe error: %v", err)
	}
	os.Stdout = w

	var runErr error
	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()

	runErr = fn()
	_ = w.Close()
	<-done
	os.Stdout = orig
	_ = r.Close()

	if runErr != nil {
		t.Fatalf("unexpected command error: %v", runErr)
	}
	return buf.String()
}

// TestCompletionNodesOfferRootProfileOnEveryCommand pins the completion side of
// the root-owned `--profile` selector: it is accepted after the command name,
// so every command offers it, and a command that binds its own `profile` flag
// still lists exactly one.
func TestCompletionNodesOfferRootProfileOnEveryCommand(t *testing.T) {
	rootFlags := flag.NewFlagSet("asc", flag.ContinueOnError)
	rootFlags.String("profile", "", "Authentication profile")

	runFlags := flag.NewFlagSet("run", flag.ContinueOnError)
	runFlags.String("profile", "", "Path to the provisioning profile")

	nodes := completionNodes([]*ffcli.Command{
		{
			Name:    "signing",
			FlagSet: flag.NewFlagSet("signing", flag.ContinueOnError),
			Subcommands: []*ffcli.Command{
				{Name: "run", FlagSet: runFlags},
			},
		},
	}, rootFlags)

	signing := findCompletionNode(t, nodes, "signing")
	if got, want := signing.flags, []string{"--profile"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("signing flags = %v, want %v", got, want)
	}
	run := findCompletionNode(t, nodes, "signing run")
	if got, want := run.flags, []string{"--profile"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("signing run flags = %v, want %v", got, want)
	}
	if got, want := run.valueFlags, []string{"--profile"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("signing run value flags = %v, want %v", got, want)
	}
}
