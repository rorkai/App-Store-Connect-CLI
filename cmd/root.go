package cmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/registry"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared/suggest"
)

var versionRequested bool

// RootCommand returns the root command
func RootCommand(version string) *ffcli.Command {
	versionRequested = false
	root := &ffcli.Command{
		Name:        "asc",
		ShortUsage:  "asc <subcommand> [flags]",
		ShortHelp:   "Unofficial. asc is a fast, lightweight cli for App Store Connect. Built by AI agents, for AI agents.",
		LongHelp:    "",
		FlagSet:     flag.NewFlagSet("asc", flag.ExitOnError),
		UsageFunc:   RootUsageFunc,
		Subcommands: registry.Subcommands(version),
	}

	root.FlagSet.BoolVar(&versionRequested, "version", false, "Print version and exit")
	shared.BindRootFlags(root.FlagSet)

	var (
		rootSubcommandNames     []string
		rootSubcommandNamesOnce sync.Once
	)

	root.Exec = func(ctx context.Context, args []string) error {
		if versionRequested {
			_, _ = fmt.Fprintln(os.Stdout, version)
			return nil
		}
		if len(args) > 0 {
			rootSubcommandNamesOnce.Do(func() {
				rootSubcommandNames = make([]string, 0, len(root.Subcommands))
				for _, sub := range root.Subcommands {
					rootSubcommandNames = append(rootSubcommandNames, sub.Name)
				}
			})
			unknown := shared.SanitizeTerminal(args[0])
			fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", unknown)
			if suggestions := suggest.Commands(args[0], rootSubcommandNames); len(suggestions) > 0 {
				for i, suggestion := range suggestions {
					suggestions[i] = shared.SanitizeTerminal(suggestion)
				}
				fmt.Fprintf(os.Stderr, "Did you mean: %s\n\n", strings.Join(suggestions, ", "))
			}
		}
		return flag.ErrHelp
	}

	return root
}
