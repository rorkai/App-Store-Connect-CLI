package web

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func bindJSONOnlyOutputFlags(fs *flag.FlagSet) shared.OutputFlags {
	return shared.BindOutputFlagsWithAllowed(fs, "output", "json", "Output format: json", "json")
}

func flagProvided(fs *flag.FlagSet, name string) bool {
	if fs == nil {
		return false
	}
	provided := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			provided = true
		}
	})
	return provided
}

func webXcodeCloudWorkflowOptionsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web xcode-cloud workflows options", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "options",
		ShortUsage: "asc web xcode-cloud workflows options <subcommand> [flags]",
		ShortHelp:  "[experimental] Inspect private workflow editor option endpoints.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Inspect Xcode Cloud workflow editor option endpoints from Apple's private CI API.
These endpoints surface UI-driven data such as team workflow defaults, product
configuration recommendations, schemes, test destinations, and Slack notification
targets. JSON output only.

` + webWarningText + `

Examples:
  asc web xcode-cloud workflows options team-config --apple-id "user@example.com"
  asc web xcode-cloud workflows options product-config --product-id "UUID" --apple-id "user@example.com"
  asc web xcode-cloud workflows options schemes --product-id "UUID" --container-file-path "App.xcodeproj" --apple-id "user@example.com"
  asc web xcode-cloud workflows options test-destinations --xcode-version "latest:stable" --apple-id "user@example.com"
  asc web xcode-cloud workflows options slack-channels --apple-id "user@example.com"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			webXcodeCloudWorkflowOptionsTeamConfigCommand(),
			webXcodeCloudWorkflowOptionsBuildVersionsCommand(),
			webXcodeCloudWorkflowOptionsProductConfigCommand(),
			webXcodeCloudWorkflowOptionsSchemesCommand(),
			webXcodeCloudWorkflowOptionsTestDestinationsCommand(),
			webXcodeCloudWorkflowOptionsSlackProviderCommand(),
			webXcodeCloudWorkflowOptionsSlackChannelsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func webXcodeCloudWorkflowOptionsTeamConfigCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web xcode-cloud workflows options team-config", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := bindJSONOnlyOutputFlags(fs)

	return &ffcli.Command{
		Name:       "team-config",
		ShortUsage: "asc web xcode-cloud workflows options team-config [flags]",
		ShortHelp:  "[experimental] Show team workflow editor configuration.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Show team-level workflow editor configuration, including available platforms
and default timezone settings. JSON output only.

` + webWarningText,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			return executeWorkflowOptionsFetch(
				ctx,
				sessionFlags,
				&output,
				"Loading Xcode Cloud workflow team configuration",
				"xcode-cloud workflows options team-config",
				func(ctx context.Context, client *webcore.Client, teamID string) (json.RawMessage, error) {
					return client.GetCIConfigurationOptions(ctx, teamID)
				},
			)
		},
	}
}

func webXcodeCloudWorkflowOptionsBuildVersionsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web xcode-cloud workflows options build-versions", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := bindJSONOnlyOutputFlags(fs)

	return &ffcli.Command{
		Name:       "build-versions",
		ShortUsage: "asc web xcode-cloud workflows options build-versions [flags]",
		ShortHelp:  "[experimental] Show workflow build version defaults.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Show workflow build version defaults/options exposed by the private CI API.
JSON output only.

` + webWarningText,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			return executeWorkflowOptionsFetch(
				ctx,
				sessionFlags,
				&output,
				"Loading Xcode Cloud workflow build versions",
				"xcode-cloud workflows options build-versions",
				func(ctx context.Context, client *webcore.Client, teamID string) (json.RawMessage, error) {
					return client.GetCIBuildVersions(ctx, teamID)
				},
			)
		},
	}
}

func webXcodeCloudWorkflowOptionsProductConfigCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web xcode-cloud workflows options product-config", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := bindJSONOnlyOutputFlags(fs)
	productID := fs.String("product-id", "", "Xcode Cloud product ID (required)")

	return &ffcli.Command{
		Name:       "product-config",
		ShortUsage: "asc web xcode-cloud workflows options product-config --product-id ID [flags]",
		ShortHelp:  "[experimental] Show product workflow configuration options.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Show product-level workflow configuration options, such as repository defaults
and container file path recommendations. JSON output only.

` + webWarningText,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			pid := strings.TrimSpace(*productID)
			if pid == "" {
				fmt.Fprintln(os.Stderr, "Error: --product-id is required")
				return flag.ErrHelp
			}

			return executeWorkflowOptionsFetch(
				ctx,
				sessionFlags,
				&output,
				"Loading Xcode Cloud product workflow configuration",
				"xcode-cloud workflows options product-config",
				func(ctx context.Context, client *webcore.Client, teamID string) (json.RawMessage, error) {
					return client.GetCIProductConfigurationOptions(ctx, teamID, pid)
				},
			)
		},
	}
}

func webXcodeCloudWorkflowOptionsSchemesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web xcode-cloud workflows options schemes", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := bindJSONOnlyOutputFlags(fs)
	productID := fs.String("product-id", "", "Xcode Cloud product ID (required)")
	containerFilePath := fs.String("container-file-path", "", "Container file path to filter schemes")
	limit := fs.Int("limit", 0, "Maximum schemes to return")
	continuationOffset := fs.String("continuation-offset", "", "Fetch next page using a continuation offset")

	return &ffcli.Command{
		Name:       "schemes",
		ShortUsage: "asc web xcode-cloud workflows options schemes --product-id ID [flags]",
		ShortHelp:  "[experimental] Show available workflow schemes.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Show available schemes and test plans for a product's container file paths.
JSON output only.

` + webWarningText,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			pid := strings.TrimSpace(*productID)
			if pid == "" {
				fmt.Fprintln(os.Stderr, "Error: --product-id is required")
				return flag.ErrHelp
			}
			if flagProvided(fs, "limit") && *limit <= 0 {
				fmt.Fprintln(os.Stderr, "Error: --limit must be greater than 0 when provided")
				return flag.ErrHelp
			}

			return executeWorkflowOptionsFetch(
				ctx,
				sessionFlags,
				&output,
				"Loading Xcode Cloud workflow schemes",
				"xcode-cloud workflows options schemes",
				func(ctx context.Context, client *webcore.Client, teamID string) (json.RawMessage, error) {
					return client.GetCISchemes(ctx, teamID, pid, *containerFilePath, *limit, *continuationOffset)
				},
			)
		},
	}
}

func webXcodeCloudWorkflowOptionsTestDestinationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web xcode-cloud workflows options test-destinations", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := bindJSONOnlyOutputFlags(fs)
	xcodeVersion := fs.String("xcode-version", "", "Xcode version selector (required)")

	return &ffcli.Command{
		Name:       "test-destinations",
		ShortUsage: "asc web xcode-cloud workflows options test-destinations --xcode-version VALUE [flags]",
		ShortHelp:  "[experimental] Show workflow test destination options.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Show workflow test destination options for a given Xcode version. JSON output only.

` + webWarningText,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			version := strings.TrimSpace(*xcodeVersion)
			if version == "" {
				fmt.Fprintln(os.Stderr, "Error: --xcode-version is required")
				return flag.ErrHelp
			}

			return executeWorkflowOptionsFetch(
				ctx,
				sessionFlags,
				&output,
				"Loading Xcode Cloud workflow test destinations",
				"xcode-cloud workflows options test-destinations",
				func(ctx context.Context, client *webcore.Client, teamID string) (json.RawMessage, error) {
					return client.GetCITestDestinations(ctx, teamID, version)
				},
			)
		},
	}
}

func webXcodeCloudWorkflowOptionsSlackProviderCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web xcode-cloud workflows options slack-provider", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := bindJSONOnlyOutputFlags(fs)

	return &ffcli.Command{
		Name:       "slack-provider",
		ShortUsage: "asc web xcode-cloud workflows options slack-provider [flags]",
		ShortHelp:  "[experimental] Show Slack workflow notification integration state.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Show Slack integration state used by workflow notification post-actions.
JSON output only.

` + webWarningText,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			return executeWorkflowOptionsFetch(
				ctx,
				sessionFlags,
				&output,
				"Loading Xcode Cloud Slack provider configuration",
				"xcode-cloud workflows options slack-provider",
				func(ctx context.Context, client *webcore.Client, teamID string) (json.RawMessage, error) {
					return client.GetCISlackProvider(ctx, teamID)
				},
			)
		},
	}
}

func webXcodeCloudWorkflowOptionsSlackChannelsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web xcode-cloud workflows options slack-channels", flag.ExitOnError)
	sessionFlags := bindWebSessionFlags(fs)
	output := bindJSONOnlyOutputFlags(fs)

	return &ffcli.Command{
		Name:       "slack-channels",
		ShortUsage: "asc web xcode-cloud workflows options slack-channels [flags]",
		ShortHelp:  "[experimental] Show Slack channels available to workflow notifications.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Show Slack channels available for workflow notification post-actions.
JSON output only.

` + webWarningText,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			return executeWorkflowOptionsFetch(
				ctx,
				sessionFlags,
				&output,
				"Loading Xcode Cloud Slack channel options",
				"xcode-cloud workflows options slack-channels",
				func(ctx context.Context, client *webcore.Client, teamID string) (json.RawMessage, error) {
					return client.GetCISlackChannels(ctx, teamID)
				},
			)
		},
	}
}

func executeWorkflowOptionsFetch(
	ctx context.Context,
	sessionFlags webSessionFlags,
	output *shared.OutputFlags,
	spinnerLabel, errorPrefix string,
	fetch func(context.Context, *webcore.Client, string) (json.RawMessage, error),
) error {
	if err := printWorkflowOptionsOutput(nil, *output.Output, *output.Pretty, true); err != nil {
		return err
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	session, err := resolveWebSessionForCommand(requestCtx, sessionFlags)
	if err != nil {
		return err
	}
	teamID := strings.TrimSpace(session.PublicProviderID)
	if teamID == "" {
		return fmt.Errorf("%s failed: session has no public provider ID", errorPrefix)
	}

	client := newCIClientFn(session)
	var raw json.RawMessage
	err = withWebSpinner(spinnerLabel, func() error {
		var err error
		raw, err = fetch(requestCtx, client, teamID)
		return err
	})
	if err != nil {
		return withWebAuthHint(err, errorPrefix)
	}

	return printWorkflowOptionsOutput(raw, *output.Output, *output.Pretty, false)
}

func printWorkflowOptionsOutput(data json.RawMessage, format string, pretty, validateOnly bool) error {
	normalized := shared.NormalizeOutputFormat(format)
	if normalized == "" {
		normalized = "json"
	}
	if normalized != "json" {
		return fmt.Errorf("unsupported format: %s", normalized)
	}
	if validateOnly {
		return nil
	}
	if pretty {
		return asc.PrintPrettyJSON(data)
	}
	return asc.PrintJSON(data)
}
