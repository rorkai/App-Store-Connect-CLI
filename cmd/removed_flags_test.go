package cmd

import (
	"flag"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/telemetry"
)

var replacementFlagPattern = regexp.MustCompile("`--([a-z0-9-]+)`")

// TestRemovedFlagRulesMatchRegisteredCommands keeps removedFlagRules honest
// against the live command tree: every command a rule names exists, none of
// them still registers the removed alias, and every replacement flag a rule
// points at is registered on every command the rule covers.
func TestRemovedFlagRulesMatchRegisteredCommands(t *testing.T) {
	root := RootCommand("5.3.2")
	commandsByPath := map[string]*ffcli.Command{}
	var walk func(command *ffcli.Command, path []string)
	walk = func(command *ffcli.Command, path []string) {
		for _, subcommand := range command.Subcommands {
			subpath := append(append([]string{}, path...), subcommand.Name)
			commandsByPath[strings.Join(subpath, " ")] = subcommand
			walk(subcommand, subpath)
		}
	}
	walk(root, nil)

	for _, rule := range removedFlagRules {
		if (rule.replacement == "") == (rule.note == "") {
			t.Errorf("rule --%s must set exactly one of replacement or note: %+v", rule.flag, rule)
		}
		if len(rule.commands) == 0 {
			t.Errorf("rule --%s names no commands", rule.flag)
		}
		matched := []string{}
		for path, command := range commandsByPath {
			if !rule.matchesCommand(path, command.FlagSet) {
				continue
			}
			if len(command.Subcommands) > 0 {
				continue
			}
			matched = append(matched, path)
			if command.FlagSet == nil {
				t.Errorf("rule --%s: %q has no flag set", rule.flag, path)
				continue
			}
			if command.FlagSet.Lookup(rule.flag) != nil {
				t.Errorf("rule --%s: alias is still registered on %q", rule.flag, path)
			}
			for _, match := range replacementFlagPattern.FindAllStringSubmatch(rule.replacement, -1) {
				if command.FlagSet.Lookup(match[1]) == nil {
					t.Errorf("rule --%s: replacement --%s is not registered on %q", rule.flag, match[1], path)
				}
			}
		}
		for _, command := range rule.commands {
			if _, ok := commandsByPath[command]; !ok {
				t.Errorf("rule --%s names unregistered command %q", rule.flag, command)
			}
		}
		if len(matched) == 0 {
			t.Errorf("rule --%s matches no leaf command", rule.flag)
		}
	}
}

func TestLookupRemovedFlagIsExactPerCommand(t *testing.T) {
	root := RootCommand("5.3.2")
	tests := []struct {
		command string
		flag    string
		want    string
		found   bool
	}{
		{command: "web auth login", flag: "two-factor-code", want: "use `--two-factor-code-command` or `ASC_WEB_2FA_CODE_COMMAND`", found: true},
		// `web auth logout` never bound the 2FA flags, so the historical rule
		// must not claim the alias there.
		{command: "web auth logout", flag: "two-factor-code"},
		// Commands added after the 4.x surface must not inherit a historical
		// hint merely because they use the same replacement flag.
		{command: "web apps history", flag: "two-factor-code"},
		// The similarly named live command never accepted the `--price-id`
		// alias from an unattached internal constructor.
		{command: "subscriptions offers offer-codes create", flag: "price-id"},
		{command: "pre-orders enable", flag: "available-in-new-territories", want: "pre-orders are enabled by patching territory availabilities", found: true},
		// Canonical spellings on other commands never match.
		{command: "apps view", flag: "id"},
		{command: "versions view", flag: "version-id"},
		{command: "builds list", flag: "build"},
		{command: "localizations", flag: "version-id"},
		{command: "builds test-notes", flag: "build"},
		{command: "apps info relationships", flag: "id"},
		{command: "webhooks list", flag: "two-factor-code"},
	}
	for _, test := range tests {
		t.Run(test.command+" --"+test.flag, func(t *testing.T) {
			rule, found := lookupRemovedFlag(test.command, test.flag, commandFlagSet(t, root, test.command))
			if found != test.found {
				t.Fatalf("found = %v, want %v (rule=%+v)", found, test.found, rule)
			}
			if found && rule.guidance() != test.want {
				t.Fatalf("guidance = %q, want %q", rule.guidance(), test.want)
			}
		})
	}
}

// TestRun_RemovedFlagHintKeepsUnknownFlagTelemetry pins that the removal hint
// changes only the stderr text: the event is still classified as
// unknown_flag at the parse stage with the raw token as failure_parameter.
func TestRun_RemovedFlagHintKeepsUnknownFlagTelemetry(t *testing.T) {
	resetReportFlags(t)
	originalEmitTelemetry := emitTelemetry
	t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })

	var gotCommand string
	var gotExitCode int
	var gotContext telemetry.EventContext
	emitTelemetry = func(commandName string, _ string, _ time.Duration, exitCode int, eventContext telemetry.EventContext) {
		gotCommand = commandName
		gotExitCode = exitCode
		gotContext = eventContext
	}

	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"signing", "sync", "push", "--password=PRIVATE_VALUE"}, "5.3.2"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	want := "Error: `--password` was removed in 5.0.0; use `--password-file` or `ASC_SIGNING_SYNC_PASSWORD` (see migrate-to-5-0)\nFor help:\n  asc signing sync push --help\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
	if gotCommand != "asc signing sync push" || gotExitCode != ExitUsage {
		t.Fatalf("telemetry command/exit = %q/%d, want asc signing sync push/%d", gotCommand, gotExitCode, ExitUsage)
	}
	if gotContext.InvocationShape != telemetry.InvocationShapeLeaf ||
		gotContext.ErrorKind != telemetry.ErrorKindUnknownFlag ||
		gotContext.FailureStage != telemetry.FailureStageParse ||
		gotContext.OutcomeKind != telemetry.OutcomeUsageError {
		t.Fatalf("unexpected telemetry context: %+v", gotContext)
	}
	if gotContext.FailureParameter != "--password=PRIVATE_VALUE" {
		t.Fatalf("FailureParameter = %q, want the raw token", gotContext.FailureParameter)
	}
}

// commandFlagSet returns the flag set of the command at the space-joined path,
// or nil when no such command is registered.
func commandFlagSet(t *testing.T, root *ffcli.Command, commandPath string) *flag.FlagSet {
	t.Helper()
	command := root
	for _, name := range strings.Fields(commandPath) {
		var next *ffcli.Command
		for _, subcommand := range command.Subcommands {
			if subcommand.Name == name {
				next = subcommand
				break
			}
		}
		if next == nil {
			return nil
		}
		command = next
	}
	return command.FlagSet
}
