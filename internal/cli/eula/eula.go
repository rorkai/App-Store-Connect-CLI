package eula

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

// EULACommand returns the end user license agreements command with subcommands.
func EULACommand() *ffcli.Command {
	fs := flag.NewFlagSet("eula", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "eula",
		ShortUsage: "asc eula <subcommand> [flags]",
		ShortHelp:  "Manage End User License Agreements (EULA).",
		LongHelp: `Manage End User License Agreements (EULA).

Examples:
  asc eula get --id "EULA_ID"
  asc eula get --app "APP_ID"
  asc eula list --app "APP_ID"
  asc eula create --app "APP_ID" --agreement-text "Terms..." --territory "USA,CAN"
  asc eula update --id "EULA_ID" --agreement-text "Updated terms"
  asc eula update --id "EULA_ID" --territory "USA,CAN"
  asc eula delete --id "EULA_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			EULAGetCommand(),
			EULAListCommand(),
			EULACreateCommand(),
			EULAUpdateCommand(),
			EULADeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// EULAGetCommand returns the eula get subcommand.
func EULAGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	id := fs.String("id", "", "EULA ID")
	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc eula get --id \"EULA_ID\" | asc eula get --app \"APP_ID\"",
		ShortHelp:  "Get an EULA by ID or app.",
		LongHelp: `Get an End User License Agreement (EULA).

Examples:
  asc eula get --id "EULA_ID"
  asc eula get --app "APP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			appValue := ""
			if idValue == "" {
				appValue = shared.ResolveAppID(*appID)
			}
			if idValue == "" && appValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id or --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}
			if idValue != "" && strings.TrimSpace(*appID) != "" {
				fmt.Fprintln(os.Stderr, "Error: --id and --app are mutually exclusive")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("eula get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			var resp *asc.EndUserLicenseAgreementResponse
			if appValue != "" {
				resp, err = client.GetEndUserLicenseAgreementForApp(requestCtx, appValue)
			} else {
				resp, err = client.GetEndUserLicenseAgreement(requestCtx, idValue)
			}
			if err != nil {
				return fmt.Errorf("eula get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// EULAListCommand returns the eula list subcommand.
func EULAListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc eula list --app \"APP_ID\"",
		ShortHelp:  "List the EULA for an app.",
		LongHelp: `List the End User License Agreement (EULA) for an app.

Examples:
  asc eula list --app "APP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			appValue := shared.ResolveAppID(*appID)
			if appValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("eula list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetEndUserLicenseAgreementForApp(requestCtx, appValue)
			if err != nil {
				return fmt.Errorf("eula list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// EULACreateCommand returns the eula create subcommand.
func EULACreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	agreementText := fs.String("agreement-text", "", "Agreement text")
	territories := fs.String("territory", "", "Territory IDs, comma-separated")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc eula create --app \"APP_ID\" --agreement-text \"Terms\" --territory \"USA,CAN\"",
		ShortHelp:  "Create an EULA for an app.",
		LongHelp: `Create an End User License Agreement (EULA).

Examples:
  asc eula create --app "APP_ID" --agreement-text "Terms..." --territory "USA,CAN"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			appValue := shared.ResolveAppID(*appID)
			if appValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}
			agreementValue := strings.TrimSpace(*agreementText)
			if agreementValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --agreement-text is required")
				return flag.ErrHelp
			}

			territoryIDs := shared.SplitCSV(*territories)
			if len(territoryIDs) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --territory is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("eula create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateEndUserLicenseAgreement(requestCtx, appValue, agreementValue, territoryIDs)
			if err != nil {
				return fmt.Errorf("eula create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// EULAUpdateCommand returns the eula update subcommand.
func EULAUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	id := fs.String("id", "", "EULA ID")
	agreementText := fs.String("agreement-text", "", "Agreement text")
	territories := fs.String("territory", "", "Territory IDs, comma-separated")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc eula update --id \"EULA_ID\" [--agreement-text \"Terms\"] [--territory \"USA,CAN\"]",
		ShortHelp:  "Update an EULA.",
		LongHelp: `Update an End User License Agreement (EULA).

Examples:
  asc eula update --id "EULA_ID" --agreement-text "Updated terms"
  asc eula update --id "EULA_ID" --territory "USA,CAN"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			var agreementValue *string
			if strings.TrimSpace(*agreementText) != "" {
				value := strings.TrimSpace(*agreementText)
				agreementValue = &value
			}

			territoryIDs := shared.SplitCSV(*territories)
			if agreementValue == nil && len(territoryIDs) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --agreement-text or --territory is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("eula update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateEndUserLicenseAgreement(requestCtx, idValue, agreementValue, territoryIDs)
			if err != nil {
				return fmt.Errorf("eula update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// EULADeleteCommand returns the eula delete subcommand.
func EULADeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	id := fs.String("id", "", "EULA ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc eula delete --id \"EULA_ID\" --confirm",
		ShortHelp:  "Delete an EULA.",
		LongHelp: `Delete an End User License Agreement (EULA).

Examples:
  asc eula delete --id "EULA_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("eula delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteEndUserLicenseAgreement(requestCtx, idValue); err != nil {
				return fmt.Errorf("eula delete: failed to delete: %w", err)
			}

			result := &asc.EndUserLicenseAgreementDeleteResult{
				ID:      idValue,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
