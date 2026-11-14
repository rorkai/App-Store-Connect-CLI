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
  asc eula view --id "EULA_ID"
  asc eula view --app "APP_ID"
  asc eula list --app "APP_ID"
  asc eula create --app "APP_ID" --agreement-text "Terms..." --territory "US,Canada"
  asc eula update --id "EULA_ID" --agreement-text "Updated terms"
  asc eula update --id "EULA_ID" --territory "US,Canada"
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

// EULAGetCommand returns the eula view subcommand.
func EULAGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("view", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "endUserLicenseAgreements", "EULA ID")
	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc eula view --id \"EULA_ID\" | asc eula view --app \"APP_ID\"",
		ShortHelp:  "View an EULA by ID or app.",
		LongHelp: `View an End User License Agreement (EULA).

Examples:
  asc eula view --id "EULA_ID"
  asc eula view --app "APP_ID"`,
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
				return shared.MissingRequiredUsageError("")
			}
			if idValue != "" && strings.TrimSpace(*appID) != "" {
				fmt.Fprintln(os.Stderr, "Error: --id and --app are mutually exclusive")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("eula view: %w", err)
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
				return fmt.Errorf("eula view: failed to fetch: %w", err)
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
				return shared.MissingRequiredUsageError("--app")
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
	territories := shared.BindOnceCSVFlag(fs, "territory", "Territory inputs, comma-separated (accepts alpha-2, alpha-3, or exact English country names)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc eula create --app \"APP_ID\" --agreement-text \"Terms\" --territory \"US,Canada\"",
		ShortHelp:  "Create an EULA for an app.",
		LongHelp: `Create an End User License Agreement (EULA).

Examples:
  asc eula create --app "APP_ID" --agreement-text "Terms..." --territory "US,Canada"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			appValue := shared.ResolveAppID(*appID)
			if appValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}
			agreementValue := strings.TrimSpace(*agreementText)
			if agreementValue == "" {
				return shared.RequiredFlag("--agreement-text")
			}

			territoryIDs, err := shared.NormalizeASCTerritoryCSV(territories.String())
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if len(territoryIDs) == 0 {
				return shared.RequiredFlag("--territory")
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

	id := shared.BindResourceIDFlag(fs, "id", "endUserLicenseAgreements", "EULA ID")
	agreementText := fs.String("agreement-text", "", "Agreement text")
	territories := shared.BindOnceCSVFlag(fs, "territory", "Territory inputs, comma-separated (accepts alpha-2, alpha-3, or exact English country names)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc eula update --id \"EULA_ID\" [--agreement-text \"Terms\"] [--territory \"US,Canada\"]",
		ShortHelp:  "Update an EULA.",
		LongHelp: `Update an End User License Agreement (EULA).

Examples:
  asc eula update --id "EULA_ID" --agreement-text "Updated terms"
  asc eula update --id "EULA_ID" --territory "US,Canada"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				return shared.RequiredFlag("--id")
			}

			var agreementValue *string
			if strings.TrimSpace(*agreementText) != "" {
				value := strings.TrimSpace(*agreementText)
				agreementValue = &value
			}

			territoryIDs, err := shared.NormalizeASCTerritoryCSV(territories.String())
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if agreementValue == nil && len(territoryIDs) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --agreement-text or --territory is required")
				return shared.MissingRequiredUsageError("")
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
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		FlagSetName: "delete",
		Name:        "delete",
		ShortUsage:  "asc eula delete --id \"EULA_ID\" --confirm",
		ShortHelp:   "Delete an EULA.",
		LongHelp: `Delete an End User License Agreement (EULA).

Examples:
  asc eula delete --id "EULA_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "EULA ID",
		IDType:             "endUserLicenseAgreements",
		RequiredDiagnostic: true,
		ErrorPrefix:        "eula delete",
		DeleteErrorPrefix:  "eula delete: failed to delete",
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteEndUserLicenseAgreement(ctx, id)
		},
		Result: func(id string) any {
			return &asc.EndUserLicenseAgreementDeleteResult{ID: id, Deleted: true}
		},
	})
}
