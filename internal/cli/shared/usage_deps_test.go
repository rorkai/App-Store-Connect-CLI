package shared

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"
)

func TestDepsUsageFuncsMatchProcessVariants(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "")
	os.Unsetenv("NO_COLOR")

	fs := flag.NewFlagSet("items", flag.ContinueOnError)
	fs.String("app", "", "App Store Connect app ID")
	fs.String("output", "json", "Output format: json (default), table, markdown")
	cmd := &ffcli.Command{
		Name:       "items",
		ShortUsage: "asc items <subcommand> [flags]",
		ShortHelp:  "Manage items.",
		LongHelp:   "Manage items.\n\nExamples:\n  asc items list --app APP_ID",
		FlagSet:    fs,
		Subcommands: []*ffcli.Command{
			{Name: "list", ShortHelp: "List items."},
			{Name: "legacy", ShortHelp: "DEPRECATED: use list."},
		},
	}

	tests := []struct {
		name    string
		process func(*ffcli.Command) string
		deps    func(*Deps) func(*ffcli.Command) string
	}{
		{
			name:    "DefaultUsageFunc",
			process: DefaultUsageFunc,
			deps:    func(d *Deps) func(*ffcli.Command) string { return d.DefaultUsageFunc },
		},
		{
			name:    "VisibleUsageFunc",
			process: func(c *ffcli.Command) string { return DefaultDeps().VisibleUsageFunc(c) },
			deps:    func(d *Deps) func(*ffcli.Command) string { return d.VisibleUsageFunc },
		},
		{
			name:    "Bold",
			process: func(*ffcli.Command) string { return DefaultDeps().Bold("GETTING STARTED") },
			deps: func(d *Deps) func(*ffcli.Command) string {
				return func(*ffcli.Command) string { return d.Bold("GETTING STARTED") }
			},
		},
	}

	// processStderrTTY makes only the process stderr a terminal, when tty is
	// true.
	processStderrTTY := func(t *testing.T, tty bool) {
		stderrFD := int(os.Stderr.Fd())
		setTerminalDetection(t, func(fd int) bool { return tty && fd == stderrFD })
	}

	for _, test := range tests {
		for _, tty := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stderr_tty=%t", test.name, tty), func(t *testing.T) {
				processStderrTTY(t, tty)
				want := test.process(cmd)
				if strings.Contains(want, bold) != tty {
					t.Fatalf("process usage = %q, want bold headings: %t", want, tty)
				}

				// The injected stderr decides; the process stderr says the
				// opposite.
				processStderrTTY(t, !tty)
				var stdout, stderr bytes.Buffer
				d := &Deps{IO: &IO{Out: &stdout, ErrOut: &stderr, StdoutTTY: !tty, StderrTTY: tty}}
				if got := test.deps(d)(cmd); got != want {
					t.Fatalf("deps usage = %q, want %q", got, want)
				}
				if stdout.Len() != 0 || stderr.Len() != 0 {
					t.Fatalf("deps stdout = %q, stderr = %q, want both empty", stdout.String(), stderr.String())
				}

				// DefaultDeps writes to the process stderr, so it decides from
				// the process stderr when the help is rendered, as the package
				// functions do, not when it was built.
				d = DefaultDeps()
				processStderrTTY(t, tty)
				if got := test.deps(d)(cmd); got != want {
					t.Fatalf("DefaultDeps usage = %q, want %q", got, want)
				}
			})
		}
	}
}
