package appclips

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newAppClipInvocationLocalizationsCommand returns the invocations localizations command group.
func newAppClipInvocationLocalizationsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("localizations", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "localizations",
		ShortUsage: "asc app-clips invocations localizations <subcommand> [flags]",
		ShortHelp:  "Manage beta App Clip invocation localizations.",
		LongHelp: `Manage beta App Clip invocation localizations.

Examples:
  asc app-clips invocations localizations list --invocation-id "INVOCATION_ID"
  asc app-clips invocations localizations create --invocation-id "INVOCATION_ID" --locale "en-US" --title "Try it"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newAppClipInvocationLocalizationsListCommand(d),
			newAppClipInvocationLocalizationsCreateCommand(d),
			newAppClipInvocationLocalizationsUpdateCommand(d),
			newAppClipInvocationLocalizationsDeleteCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newAppClipInvocationLocalizationsListCommand lists localizations for an invocation.
func newAppClipInvocationLocalizationsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("list", flag.ExitOnError)

	invocationID := shared.BindResourceIDFlag(fs, "invocation-id", "betaAppClipInvocations", "Invocation ID")
	limit := fs.Int("limit", 0, "Maximum included localizations (1-200)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc app-clips invocations localizations list --invocation-id \"INVOCATION_ID\" [flags]",
		ShortHelp:  "List localizations for a beta App Clip invocation.",
		LongHelp: `List localizations for a beta App Clip invocation.

Examples:
  asc app-clips invocations localizations list --invocation-id "INVOCATION_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "app-clips invocations localizations list", 200); err != nil {
				return err
			}

			invocationValue := strings.TrimSpace(*invocationID)
			if invocationValue == "" {
				return d.RequiredFlag("--invocation-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("app-clips invocations localizations list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetBetaAppClipInvocationLocalizations(requestCtx, invocationValue, *limit)
			if err != nil {
				if asc.IsNotFound(err) {
					empty := &asc.BetaAppClipInvocationLocalizationsResponse{Data: []asc.Resource[asc.BetaAppClipInvocationLocalizationAttributes]{}}
					return d.PrintOutput(empty, *output.Output, *output.Pretty)
				}
				return fmt.Errorf("app-clips invocations localizations list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newAppClipInvocationLocalizationsCreateCommand creates a localization.
func newAppClipInvocationLocalizationsCreateCommand(d *shared.Deps) *ffcli.Command {
	var attrs asc.BetaAppClipInvocationLocalizationCreateAttributes
	return shared.BuildCreateCommand(shared.WriteCommandConfig{
		Name:       "create",
		ShortUsage: "asc app-clips invocations localizations create --invocation-id \"INVOCATION_ID\" --locale \"en-US\" --title \"Try it\"",
		ShortHelp:  "Create a beta App Clip invocation localization.",
		LongHelp: `Create a beta App Clip invocation localization.

Examples:
  asc app-clips invocations localizations create --invocation-id "INVOCATION_ID" --locale "en-US" --title "Try it"`,
		IDFlag:  "invocation-id",
		IDUsage: "Invocation ID",
		IDType:  "betaAppClipInvocations",
		Fields: []shared.AttributeFlag{
			shared.RequiredStringFlag(&attrs.Locale, "locale", "Locale (e.g., en-US)"),
			shared.RequiredStringFlag(&attrs.Title, "title", "Title"),
		},
		Deps: d,
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.CreateBetaAppClipInvocationLocalization(ctx, id, attrs)
		},
	})
}

// newAppClipInvocationLocalizationsUpdateCommand updates a localization.
func newAppClipInvocationLocalizationsUpdateCommand(d *shared.Deps) *ffcli.Command {
	var attrs asc.BetaAppClipInvocationLocalizationUpdateAttributes
	return shared.BuildUpdateCommand(shared.WriteCommandConfig{
		Name:       "update",
		ShortUsage: "asc app-clips invocations localizations update --localization-id \"LOC_ID\" [flags]",
		ShortHelp:  "Update a beta App Clip invocation localization.",
		LongHelp: `Update a beta App Clip invocation localization.

Examples:
  asc app-clips invocations localizations update --localization-id "LOC_ID" --title "Try it"`,
		IDFlag:  "localization-id",
		IDUsage: "Localization ID",
		IDType:  "betaAppClipInvocationLocalizations",
		Fields: []shared.AttributeFlag{
			shared.ExplicitStringFlag(&attrs.Title, "title", "Title"),
		},
		Deps: d,
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.UpdateBetaAppClipInvocationLocalization(ctx, id, &attrs)
		},
	})
}

// newAppClipInvocationLocalizationsDeleteCommand deletes a localization.
func newAppClipInvocationLocalizationsDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Name:       "delete",
		ShortUsage: "asc app-clips invocations localizations delete --localization-id \"LOC_ID\" --confirm",
		ShortHelp:  "Delete a beta App Clip invocation localization.",
		LongHelp: `Delete a beta App Clip invocation localization.

Examples:
  asc app-clips invocations localizations delete --localization-id "LOC_ID" --confirm`,
		IDFlag:             "localization-id",
		IDUsage:            "Localization ID",
		IDType:             "betaAppClipInvocationLocalizations",
		RequiredDiagnostic: true,
		Deps:               d,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteBetaAppClipInvocationLocalization(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
	})
}
