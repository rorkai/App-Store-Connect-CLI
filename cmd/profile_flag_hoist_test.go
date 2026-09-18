package cmd

import (
	"flag"
	"slices"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"
)

func newHoistTestRoot() *ffcli.Command {
	rootFlags := flag.NewFlagSet("asc", flag.ContinueOnError)
	rootFlags.String("profile", "", "Use named authentication profile")
	rootFlags.Bool("debug", false, "Enable debug logging")

	listFlags := flag.NewFlagSet("list", flag.ContinueOnError)
	listFlags.String("output", "", "Output format")
	listFlags.Bool("paginate", false, "Fetch every page")

	viewFlags := flag.NewFlagSet("view", flag.ContinueOnError)
	viewFlags.String("id", "", "Resource identifier")

	runFlags := flag.NewFlagSet("run", flag.ContinueOnError)
	runFlags.String("profile", "", "Path to the provisioning profile")

	searchFlags := flag.NewFlagSet("search", flag.ContinueOnError)
	searchFlags.String("output", "", "Output format")
	searchFlags.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:    "asc",
		FlagSet: rootFlags,
		Subcommands: []*ffcli.Command{
			{
				Name:    "apps",
				FlagSet: flag.NewFlagSet("apps", flag.ContinueOnError),
				Subcommands: []*ffcli.Command{
					{Name: "list", FlagSet: listFlags},
					{Name: "view", FlagSet: viewFlags},
				},
			},
			{
				Name:    "signing",
				FlagSet: flag.NewFlagSet("signing", flag.ContinueOnError),
				Subcommands: []*ffcli.Command{
					{Name: "run", FlagSet: runFlags},
				},
			},
			{Name: "search", FlagSet: searchFlags},
		},
	}
}

func TestHoistRootProfileFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "leaf command separate value",
			args: []string{"apps", "list", "--profile", "staging"},
			want: []string{"--profile=staging", "apps", "list"},
		},
		{
			name: "leaf command inline value",
			args: []string{"apps", "list", "--profile=staging"},
			want: []string{"--profile=staging", "apps", "list"},
		},
		{
			name: "between command flags",
			args: []string{"apps", "list", "--profile", "staging", "--output", "json"},
			want: []string{"--profile=staging", "apps", "list", "--output", "json"},
		},
		{
			name: "group command",
			args: []string{"apps", "--profile", "staging", "list"},
			want: []string{"--profile=staging", "apps", "list"},
		},
		{
			name: "already at root",
			args: []string{"--profile", "staging", "apps", "list"},
			want: []string{"--profile", "staging", "apps", "list"},
		},
		{
			name: "preserves left to right precedence",
			args: []string{"--profile", "prod", "apps", "list", "--profile", "staging"},
			want: []string{"--profile", "prod", "--profile=staging", "apps", "list"},
		},
		{
			name: "keeps other root flags before the command",
			args: []string{"--debug", "apps", "list", "--profile", "staging"},
			want: []string{"--debug", "--profile=staging", "apps", "list"},
		},
		{
			name: "command owned profile flag is untouched",
			args: []string{"signing", "run", "--profile", "app.mobileprovision", "--", "child"},
			want: []string{"signing", "run", "--profile", "app.mobileprovision", "--", "child"},
		},
		{
			name: "value naming a subcommand is not consumed",
			args: []string{"apps", "--profile", "list"},
			want: []string{"apps", "--profile", "list"},
		},
		{
			name: "missing value is left in place",
			args: []string{"apps", "list", "--profile"},
			want: []string{"apps", "list", "--profile"},
		},
		{
			name: "earlier selector still hoists when a later one has no value",
			args: []string{"apps", "list", "--profile=staging", "--profile"},
			want: []string{"--profile=staging", "apps", "list", "--profile"},
		},
		{
			name: "stops at the terminator",
			args: []string{"search", "--", "--profile", "staging"},
			want: []string{"search", "--", "--profile", "staging"},
		},
		{
			name: "stops at a positional argument",
			args: []string{"search", "upload a build", "--profile", "staging"},
			want: []string{"search", "upload a build", "--profile", "staging"},
		},
		{
			name: "stops at an empty positional argument",
			args: []string{"search", "", "--profile", "staging"},
			want: []string{"search", "", "--profile", "staging"},
		},
		{
			name: "stops at an unknown flag",
			args: []string{"apps", "list", "--bogus", "--profile", "staging"},
			want: []string{"apps", "list", "--bogus", "--profile", "staging"},
		},
		{
			name: "stops at a malformed flag spelling",
			args: []string{"apps", "list", "---profile", "staging"},
			want: []string{"apps", "list", "---profile", "staging"},
		},
		{
			name: "boolean command flag keeps its following token",
			args: []string{"apps", "list", "--paginate", "--profile", "staging"},
			want: []string{"--profile=staging", "apps", "list", "--paginate"},
		},
		{
			name: "spaced boolean value does not end the walk",
			args: []string{"apps", "list", "--paginate", "false", "--profile", "staging"},
			want: []string{"--profile=staging", "apps", "list", "--paginate", "false"},
		},
		{
			name: "positional payload command keeps its spaced boolean value",
			args: []string{"search", "--pretty", "false", "--profile", "staging"},
			want: []string{"search", "--pretty", "false", "--profile", "staging"},
		},
		{
			name: "no profile flag leaves args untouched",
			args: []string{"apps", "list", "--output", "json"},
			want: []string{"apps", "list", "--output", "json"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := hoistRootProfileFlag(newHoistTestRoot(), slices.Clone(test.args))
			if !slices.Equal(got, test.want) {
				t.Fatalf("hoistRootProfileFlag(%q) = %q, want %q", test.args, got, test.want)
			}
		})
	}
}

func TestHoistRootProfileFlagWithoutRootBinding(t *testing.T) {
	root := newHoistTestRoot()
	root.FlagSet = flag.NewFlagSet("asc", flag.ContinueOnError)

	args := []string{"apps", "list", "--profile", "staging"}
	got := hoistRootProfileFlag(root, slices.Clone(args))
	if !slices.Equal(got, args) {
		t.Fatalf("hoistRootProfileFlag = %q, want %q", got, args)
	}
}

// TestCommandOwnedProfileFlagInventory pins the commands that define their own
// `profile` flag. Those keep it, so a new command-local `--profile` must be a
// deliberate decision rather than a silent change to credential selection.
func TestCommandOwnedProfileFlagInventory(t *testing.T) {
	want := map[string]struct{}{"asc signing run": {}}

	got := map[string]struct{}{}
	var walk func(command *ffcli.Command, path []string)
	walk = func(command *ffcli.Command, path []string) {
		if command == nil {
			return
		}
		commandPath := append(append([]string{}, path...), command.Name)
		if len(commandPath) > 1 && command.FlagSet != nil {
			if command.FlagSet.Lookup(rootProfileFlagName) != nil {
				got[strings.Join(commandPath, " ")] = struct{}{}
			}
		}
		for _, subcommand := range command.Subcommands {
			walk(subcommand, commandPath)
		}
	}
	walk(RootCommand("test"), nil)

	for command := range want {
		if _, ok := got[command]; !ok {
			t.Errorf("%s no longer defines its own --profile flag", command)
		}
	}
	for command := range got {
		if _, ok := want[command]; !ok {
			t.Errorf("%s defines a command-local --profile flag; root --profile is the credential selector", command)
		}
	}
}
