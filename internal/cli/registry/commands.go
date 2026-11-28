package registry

import (
	"context"
	"encoding/json"
	"flag"
	"slices"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type commandInventory struct {
	SchemaVersion int             `json:"schema_version"`
	Families      []commandFamily `json:"families,omitempty"`
	Family        string          `json:"family,omitempty"`
	Commands      []commandEntry  `json:"commands,omitempty"`
}

type commandFamily struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type commandEntry struct {
	Path        []string                  `json:"path"`
	Usage       string                    `json:"usage"`
	Description string                    `json:"description"`
	HasChildren bool                      `json:"has_subcommands"`
	Flags       []commandFlag             `json:"flags"`
	Constraints shared.CommandConstraints `json:"constraints"`
	commandBehavior
	RuntimeRequirements *shared.CommandRuntimeRequirements `json:"runtimeRequirements,omitempty"`
}

type commandFlag struct {
	Name        string `json:"name"`
	TakesValue  bool   `json:"takes_value"`
	Description string `json:"description"`
	shared.FlagMetadata
}

func (c *Catalog) newCommandsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("commands", flag.ExitOnError)
	family := fs.String("family", "", "Exact command path to describe (for example 'review details')")
	format := fs.String("output", "json", "Output format (json)")
	pretty := fs.Bool("pretty", false, "Indent the JSON output")
	return &ffcli.Command{
		Name:       "commands",
		ShortUsage: "asc commands [--family PATH] [--output json]",
		ShortHelp:  "Discover registered command families and their inputs.",
		LongHelp: `List command families from the CLI registry. Use --family to describe
an exact path and its visible subcommands, including their flags and usage.
Flags include parsing types and declared defaults. Only static defaults are
shown; dynamic and unclassified defaults have no value. Parser defaults do not
describe later environment or account fallbacks. Unknown metadata is incomplete.
Deprecated, removed, and compatibility aliases are omitted from listings;
an exact alias path can still be inspected with --family.
An unknown write intent or human interaction marks an unclassified command.
Constraints describe declared shared validation only; coverage is partial or
unknown. Custom validation and runtime app-ID fallback may add requirements.
Runtime requirements are included only where declared; omission means unknown.
Discovery does not probe the host or check tool availability.
Runtime validation and approval requirements still apply.

Examples:
  asc commands --output json
  asc commands --family 'review details' --pretty
  asc commands --family 'web auth login'`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(_ context.Context, args []string) error {
			if len(args) != 0 {
				return d.UsageError("commands does not accept positional arguments; use --family")
			}
			if strings.TrimSpace(*format) != "json" {
				return d.UsageError("commands supports only --output json")
			}
			result := commandInventory{SchemaVersion: 1}
			path := strings.Fields(*family)
			if len(path) == 0 {
				for _, entry := range c.MetadataCommands() {
					if shared.HiddenFromDiscovery(entry) {
						continue
					}
					result.Families = append(result.Families, commandFamily{entry.Name, entry.ShortHelp})
				}
			} else {
				selected := findExactCommand(c.CommandsFor(path[0]), path)
				if selected == nil {
					return d.UsageError("unknown command family; run asc commands to list command families")
				}
				result.Family = strings.Join(path, " ")
				appendCommandEntries(&result.Commands, selected, path)
			}
			encoder := json.NewEncoder(d.IO.Out)
			if *pretty {
				encoder.SetIndent("", "  ")
			}
			return encoder.Encode(result)
		},
	}
}

func findExactCommand(commands []*ffcli.Command, path []string) *ffcli.Command {
	var selected *ffcli.Command
	for _, part := range path {
		i := slices.IndexFunc(commands, func(command *ffcli.Command) bool {
			return command != nil && command.Name == part
		})
		if i == -1 {
			return nil
		}
		selected = commands[i]
		commands = selected.Subcommands
	}
	return selected
}

func appendCommandEntries(entries *[]commandEntry, command *ffcli.Command, path []string) {
	entry := commandEntry{
		Path:        path,
		Usage:       command.ShortUsage,
		Description: command.ShortHelp,
		HasChildren: slices.ContainsFunc(command.Subcommands, func(child *ffcli.Command) bool {
			return !shared.HiddenFromDiscovery(child)
		}),
		Flags:               []commandFlag{},
		RuntimeRequirements: shared.CommandRuntimeRequirementsFor(command.FlagSet),
		Constraints:         shared.CommandConstraintsFor(command.FlagSet),
		commandBehavior:     behaviorForCommand(strings.Join(path, " ")),
	}
	for _, option := range shared.VisibleHelpFlags(command.FlagSet) {
		boolean, isBoolean := option.Value.(interface{ IsBoolFlag() bool })
		entry.Flags = append(entry.Flags, commandFlag{
			Name:         "--" + option.Name,
			TakesValue:   !isBoolean || !boolean.IsBoolFlag(),
			Description:  option.Usage,
			FlagMetadata: shared.DescribeFlagValue(option.Value),
		})
	}
	*entries = append(*entries, entry)
	for _, child := range command.Subcommands {
		if !shared.HiddenFromDiscovery(child) {
			appendCommandEntries(entries, child, append(slices.Clone(path), child.Name))
		}
	}
}
