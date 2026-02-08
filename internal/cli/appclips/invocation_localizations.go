package appclips

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// AppClipInvocationLocalizationsCommand returns the invocations localizations command group.
func AppClipInvocationLocalizationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "localizations",
		ShortUsage: "asc app-clips invocations localizations <subcommand> [flags]",
		ShortHelp:  "Manage beta App Clip invocation localizations.",
		LongHelp: `Manage beta App Clip invocation localizations.

Examples:
  asc app-clips invocations localizations list --invocation-id "INVOCATION_ID"
  asc app-clips invocations localizations create --invocation-id "INVOCATION_ID" --locale "en-US" --title "Try it"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AppClipInvocationLocalizationsListCommand(),
			AppClipInvocationLocalizationsCreateCommand(),
			AppClipInvocationLocalizationsUpdateCommand(),
			AppClipInvocationLocalizationsDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppClipInvocationLocalizationsListCommand lists localizations for an invocation.
func AppClipInvocationLocalizationsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	invocationID := fs.String("invocation-id", "", "Invocation ID")
	limit := fs.Int("limit", 0, "Maximum included localizations (1-200)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc app-clips invocations localizations list --invocation-id \"INVOCATION_ID\" [flags]",
		ShortHelp:  "List localizations for a beta App Clip invocation.",
		LongHelp: `List localizations for a beta App Clip invocation.

Examples:
  asc app-clips invocations localizations list --invocation-id "INVOCATION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("app-clips invocations localizations list: --limit must be between 1 and 200")
			}

			invocationValue := strings.TrimSpace(*invocationID)
			if invocationValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --invocation-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips invocations localizations list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBetaAppClipInvocationLocalizations(requestCtx, invocationValue, *limit)
			if err != nil {
				if asc.IsNotFound(err) {
					empty := &asc.BetaAppClipInvocationLocalizationsResponse{Data: []asc.Resource[asc.BetaAppClipInvocationLocalizationAttributes]{}}
					return shared.PrintOutput(empty, *output, *pretty)
				}
				return fmt.Errorf("app-clips invocations localizations list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// AppClipInvocationLocalizationsCreateCommand creates a localization.
func AppClipInvocationLocalizationsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	invocationID := fs.String("invocation-id", "", "Invocation ID")
	locale := fs.String("locale", "", "Locale (e.g., en-US)")
	title := fs.String("title", "", "Title")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc app-clips invocations localizations create --invocation-id \"INVOCATION_ID\" --locale \"en-US\" --title \"Try it\"",
		ShortHelp:  "Create a beta App Clip invocation localization.",
		LongHelp: `Create a beta App Clip invocation localization.

Examples:
  asc app-clips invocations localizations create --invocation-id "INVOCATION_ID" --locale "en-US" --title "Try it"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			invocationValue := strings.TrimSpace(*invocationID)
			if invocationValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --invocation-id is required")
				return flag.ErrHelp
			}

			localeValue := strings.TrimSpace(*locale)
			if localeValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --locale is required")
				return flag.ErrHelp
			}

			titleValue := strings.TrimSpace(*title)
			if titleValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --title is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips invocations localizations create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.BetaAppClipInvocationLocalizationCreateAttributes{
				Locale: localeValue,
				Title:  titleValue,
			}

			resp, err := client.CreateBetaAppClipInvocationLocalization(requestCtx, invocationValue, attrs)
			if err != nil {
				return fmt.Errorf("app-clips invocations localizations create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// AppClipInvocationLocalizationsUpdateCommand updates a localization.
func AppClipInvocationLocalizationsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "Localization ID")
	title := fs.String("title", "", "Title")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc app-clips invocations localizations update --localization-id \"LOC_ID\" [flags]",
		ShortHelp:  "Update a beta App Clip invocation localization.",
		LongHelp: `Update a beta App Clip invocation localization.

Examples:
  asc app-clips invocations localizations update --localization-id "LOC_ID" --title "Try it"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			locValue := strings.TrimSpace(*localizationID)
			if locValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return flag.ErrHelp
			}

			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})
			if !visited["title"] {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			titleValue := strings.TrimSpace(*title)
			attrs := &asc.BetaAppClipInvocationLocalizationUpdateAttributes{Title: &titleValue}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips invocations localizations update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateBetaAppClipInvocationLocalization(requestCtx, locValue, attrs)
			if err != nil {
				return fmt.Errorf("app-clips invocations localizations update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// AppClipInvocationLocalizationsDeleteCommand deletes a localization.
func AppClipInvocationLocalizationsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "Localization ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc app-clips invocations localizations delete --localization-id \"LOC_ID\" --confirm",
		ShortHelp:  "Delete a beta App Clip invocation localization.",
		LongHelp: `Delete a beta App Clip invocation localization.

Examples:
  asc app-clips invocations localizations delete --localization-id "LOC_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			locValue := strings.TrimSpace(*localizationID)
			if locValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required to delete")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips invocations localizations delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteBetaAppClipInvocationLocalization(requestCtx, locValue); err != nil {
				return fmt.Errorf("app-clips invocations localizations delete: failed to delete: %w", err)
			}

			result := &asc.BetaAppClipInvocationLocalizationDeleteResult{
				ID:      locValue,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}
