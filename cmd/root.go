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

// rootGettingStartedSamples teaches the discovery loop on the first help
// screen: find the right command, diagnose credentials, locate an app, then
// inspect it. Every sample is a copy-paste-valid long-form invocation, and
// placeholders stay bare uppercase so shells do not read them as redirection.
const rootGettingStartedSamples = `  Find the command, with examples:
    asc search "upload a build" --output json
  Diagnose local auth configuration:
    asc auth doctor
  List your apps and their IDs:
    asc apps list --paginate --output table
  Show a one-screen release overview:
    asc status --app APP_ID

  Add --help to any command; replace placeholders like APP_ID with real values.
  Any flag that takes a value accepts @env:NAME or @file:PATH; escape a literal @ as @@.
  --output and the root --profile, --report, --report-file are read literally.`

// rootLongHelp renders the GETTING STARTED block shown between USAGE and the
// grouped command listing.
func rootLongHelp() string {
	return shared.Bold("GETTING STARTED") + "\n" + rootGettingStartedSamples
}

// RootCommand returns the root command
func RootCommand(version string) *ffcli.Command {
	catalog := registry.NewCatalog(version)
	root := newRootCommand(version, catalog.All())
	catalog.SetCompletionRootFlagSet(root.FlagSet)
	return root
}

func rootCommandForArgs(version string, args []string) *ffcli.Command {
	catalog := registry.NewCatalog(version)
	root := newRootCommand(version, catalog.MetadataCommands())
	catalog.SetCompletionRootFlagSet(root.FlagSet)
	// Command discovery must understand the same liberal `--bool false` form
	// as final parsing. Otherwise the separated value looks positional and the
	// lazy catalog can materialize the wrong command tree.
	commandName := getCommandName(root, normalizeSpacedBooleanFlags(root, args))
	parts := strings.Fields(commandName)
	if len(parts) < 2 {
		return root
	}

	root.Subcommands = catalog.CommandsFor(parts[1])
	for _, subcommand := range root.Subcommands {
		if strings.EqualFold(subcommand.Name, parts[1]) {
			shared.WrapCommandOutputValidation(subcommand)
			break
		}
	}
	return root
}

func newRootCommand(version string, subcommands []*ffcli.Command) *ffcli.Command {
	versionRequested = false
	root := &ffcli.Command{
		Name:        "asc",
		ShortUsage:  "asc <subcommand> [flags]",
		ShortHelp:   "asc is a fast, lightweight CLI for App Store Connect from Rork.",
		LongHelp:    rootLongHelp(),
		FlagSet:     flag.NewFlagSet("asc", flag.ExitOnError),
		UsageFunc:   RootUsageFunc,
		Subcommands: subcommands,
	}

	for _, subcommand := range subcommands {
		shared.WrapCommandOutputValidation(subcommand)
	}

	root.FlagSet.BoolVar(&versionRequested, "version", false, "Print version and exit")
	shared.BindRootFlags(root.FlagSet)

	var (
		rootSubcommandNames     []string
		rootSubcommandNamesOnce sync.Once
	)

	root.Exec = func(ctx context.Context, args []string) error {
		if versionRequested {
			fmt.Fprintln(os.Stdout, version)
			return nil
		}
		if len(args) > 0 {
			rootSubcommandNamesOnce.Do(func() {
				rootSubcommandNames = make([]string, 0, len(root.Subcommands))
				for _, sub := range root.Subcommands {
					if shouldHideRootCommand(sub) {
						continue
					}
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
