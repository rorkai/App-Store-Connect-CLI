package cmd

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/telemetry"
)

// TestStrayPositionalTelemetryNeverCarriesTheOperand guards the privacy
// contract: the rejected token is an unbounded caller-supplied value, so it
// must stay on stderr and never reach the telemetry event.
func TestStrayPositionalTelemetryNeverCarriesTheOperand(t *testing.T) {
	const operand = "6759231657"

	originalEmitTelemetry := emitTelemetry
	t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })

	var (
		emitted      bool
		gotCommand   string
		gotExitCode  int
		gotContext   telemetry.EventContext
		emittedCount int
	)
	emitTelemetry = func(commandName string, _ string, _ time.Duration, exitCode int, eventContext telemetry.EventContext) {
		emitted = true
		emittedCount++
		gotCommand = commandName
		gotExitCode = exitCode
		gotContext = eventContext
	}

	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"apps", "view", operand}, "1.2.3"); code != ExitUsage {
			t.Fatalf("exit code = %d, want %d", code, ExitUsage)
		}
	})
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, `unexpected argument "`+operand+`"`) {
		t.Fatalf("stderr = %q, want the rejected operand named", stderr)
	}
	if !emitted {
		t.Fatal("expected a telemetry event for the usage failure")
	}
	if emittedCount != 1 {
		t.Fatalf("telemetry events = %d, want 1", emittedCount)
	}
	if gotCommand != "asc apps view" {
		t.Fatalf("telemetry command = %q, want %q", gotCommand, "asc apps view")
	}
	if gotExitCode != ExitUsage {
		t.Fatalf("telemetry exit code = %d, want %d", gotExitCode, ExitUsage)
	}
	if gotContext.FailureParameter != "" {
		t.Fatalf("telemetry failure parameter = %q, want empty", gotContext.FailureParameter)
	}

	event, ok := telemetry.BuildEventWithContext(gotCommand, "1.2.3", 0, gotExitCode, gotContext)
	if !ok {
		t.Fatal("expected a built telemetry event")
	}
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	if strings.Contains(string(payload), operand) {
		t.Fatalf("telemetry payload carries the rejected operand: %s", payload)
	}
}

// TestStrayPositionalHintRendersSafelyPerShell covers the copyable correction:
// a POSIX shell gets a quoted argument, and Windows gets no hint at all when
// the operand cannot be rendered safely for cmd.exe and PowerShell.
func TestStrayPositionalHintRendersSafelyPerShell(t *testing.T) {
	tests := []struct {
		name     string
		operand  string
		goos     string
		want     string
		rendered bool
	}{
		{name: "plain id", operand: "123", goos: "darwin", want: "asc apps view --id 123", rendered: true},
		{
			name:     "spaces and a command separator",
			operand:  "my app; whoami",
			goos:     "darwin",
			want:     "asc apps view --id 'my app; whoami'",
			rendered: true,
		},
		{
			name:     "leading comment marker",
			operand:  "#app-1",
			goos:     "darwin",
			want:     "asc apps view --id '#app-1'",
			rendered: true,
		},
		{
			name:     "embedded single quote",
			operand:  "it's",
			goos:     "darwin",
			want:     `asc apps view --id 'it'\''s'`,
			rendered: true,
		},
		{name: "leading equals on posix", operand: "=ls", goos: "darwin", want: "asc apps view --id '=ls'", rendered: true},
		{name: "spaces on linux", operand: "my app", goos: "linux", want: "asc apps view --id 'my app'", rendered: true},
		{name: "newline on posix", operand: "line\nnext", goos: "darwin"},
		{name: "ansi escape on posix", operand: "a\x1b[31mred", goos: "darwin"},
		{name: "bidi override on posix", operand: "a\u202eb", goos: "darwin"},
		{name: "invalid utf-8 on posix", operand: "a\xffb", goos: "darwin"},
		{name: "plain id on windows", operand: "123", goos: "windows", want: "asc apps view --id 123", rendered: true},
		{name: "ampersand on windows", operand: "x&whoami", goos: "windows"},
		{name: "semicolon on windows", operand: "x;whoami", goos: "windows"},
		{name: "empty operand on windows", operand: "", goos: "windows"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, rendered := strayPositionalHint("asc apps view", "id", test.operand, test.goos)
			if rendered != test.rendered {
				t.Fatalf("rendered = %v, want %v (hint %q)", rendered, test.rendered, got)
			}
			if got != test.want {
				t.Fatalf("hint = %q, want %q", got, test.want)
			}
		})
	}
}

func TestStrayPositionalUnsafeOperandOmitsCopyableHint(t *testing.T) {
	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"apps", "view", "line\nnext"}, "1.2.3"); code != ExitUsage {
			t.Fatalf("exit code = %d, want %d", code, ExitUsage)
		}
	})
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "unexpected argument") {
		t.Fatalf("stderr = %q, want the sanitized error", stderr)
	}
	if strings.Contains(stderr, "Did you mean:") {
		t.Fatalf("stderr = %q, want no copyable hint for a non-exact operand", stderr)
	}
}

// TestLeafOperandContractMatchesDeclaredUsage walks the whole command tree so a
// new command cannot quietly disagree with the stray-operand exclusion list.
// The reviewed list in commandAcceptsOperandsPath and each command's own
// ShortUsage are independent sources; drift in either one fails here.
func TestLeafOperandContractMatchesDeclaredUsage(t *testing.T) {
	root := RootCommand("1.2.3")

	var (
		declared  []string
		allowed   []string
		mismatchD []string
		mismatchA []string
	)

	var walk func(command *ffcli.Command, path []string)
	walk = func(command *ffcli.Command, path []string) {
		if command == nil {
			return
		}
		current := append(append([]string(nil), path...), command.Name)
		if len(command.Subcommands) > 0 {
			for _, subcommand := range command.Subcommands {
				walk(subcommand, current)
			}
			return
		}

		commandName := strings.Join(current, " ")
		declaresOperands := shared.UsageDeclaresOperands(command.ShortUsage)
		listed := commandAcceptsOperandsPath(commandName)
		if declaresOperands {
			declared = append(declared, commandName)
		}
		if listed {
			allowed = append(allowed, commandName)
		}
		if declaresOperands && !listed {
			mismatchD = append(mismatchD, commandName)
		}
		if listed && !declaresOperands {
			mismatchA = append(mismatchA, commandName)
		}
	}
	walk(root, nil)

	if len(declared) == 0 {
		t.Fatal("no leaf command declares positional operands; the walk found nothing")
	}
	if len(mismatchD) > 0 {
		t.Fatalf(
			"leaf commands declare positional operands in USAGE but are missing from the exclusion list: %s",
			strings.Join(mismatchD, ", "),
		)
	}
	if len(mismatchA) > 0 {
		t.Fatalf(
			"leaf commands are on the exclusion list but no longer declare positional operands in USAGE: %s",
			strings.Join(mismatchA, ", "),
		)
	}

	wantAllowed := []string{
		"asc api",
		"asc docs show",
		"asc schema",
		"asc search",
		"asc signing run",
		"asc workflow run",
	}
	if strings.Join(sortedCopy(allowed), "\n") != strings.Join(wantAllowed, "\n") {
		t.Fatalf("leaf commands accepting operands = %v, want %v", sortedCopy(allowed), wantAllowed)
	}
}

// TestGroupCommandsAreNeverCheckedForStrayOperands documents why `asc snitch`
// keeps working: a command group owns its own unknown-child diagnostics, and
// snitch reads its report text from the operands its group Exec receives.
func TestGroupCommandsAreNeverCheckedForStrayOperands(t *testing.T) {
	root := RootCommand("1.2.3")
	snitch := findDirectSubcommand(root, "snitch")
	if snitch == nil {
		t.Fatal("snitch command not found")
	}
	if len(snitch.Subcommands) == 0 {
		t.Fatal("snitch is no longer a command group; add it to the stray-operand exclusion list")
	}
	if strayPositionalOperands(invocationAnalysis{command: snitch}, "asc snitch") != nil {
		t.Fatal("group commands must never report stray operands")
	}
}

func sortedCopy(values []string) []string {
	copied := append([]string(nil), values...)
	for i := 1; i < len(copied); i++ {
		for j := i; j > 0 && copied[j] < copied[j-1]; j-- {
			copied[j], copied[j-1] = copied[j-1], copied[j]
		}
	}
	return copied
}
