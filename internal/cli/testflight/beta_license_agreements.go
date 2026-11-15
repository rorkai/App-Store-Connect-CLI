package testflight

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newBetaLicenseAgreementsCommand returns the beta license agreements command group.
func newBetaLicenseAgreementsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("beta-license-agreements", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "beta-license-agreements",
		ShortUsage: "asc testflight beta-license-agreements <subcommand> [flags]",
		ShortHelp:  "Manage TestFlight beta license agreements.",
		LongHelp: `Manage TestFlight beta license agreements.

Examples:
  asc testflight beta-license-agreements list --app "APP_ID"
  asc testflight beta-license-agreements view --id "AGREEMENT_ID"
  asc testflight beta-license-agreements view --app "APP_ID"
  asc testflight beta-license-agreements update --id "AGREEMENT_ID" --agreement-text "Updated terms"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newBetaLicenseAgreementsListCommand(d),
			newBetaLicenseAgreementsGetCommand(d),
			newBetaLicenseAgreementsUpdateCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newBetaLicenseAgreementsListCommand returns the beta license agreements list subcommand.
func newBetaLicenseAgreementsListCommand(d *shared.Deps) *ffcli.Command {
	var appIDs, fields, appFields, include *string

	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Deps:       d,
		Name:       "list",
		ShortUsage: "asc testflight beta-license-agreements list [flags]",
		ShortHelp:  "List beta license agreements.",
		LongHelp: `List beta license agreements.

Examples:
  asc testflight beta-license-agreements list
  asc testflight beta-license-agreements list --app "APP_ID"
  asc testflight beta-license-agreements list --limit 50
  asc testflight beta-license-agreements list --paginate`,
		TopLevel:             true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		NextConflicts:        []string{"app", "fields", "app-fields", "include"},
		ErrorPrefix:          "beta-license-agreements list",
		Flags: func(fs *flag.FlagSet) func() error {
			appIDs = fs.String("app", "", "App Store Connect app ID(s), comma-separated")
			fields = fs.String("fields", "", "Fields to include (betaLicenseAgreements), comma-separated")
			appFields = fs.String("app-fields", "", "App fields to include, comma-separated")
			include = fs.String("include", "", "Include related resources (e.g., app), comma-separated")
			return nil
		},
		FetchPage: func(ctx context.Context, client *asc.Client, _ string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetBetaLicenseAgreements(ctx,
				asc.WithBetaLicenseAgreementsAppIDs(shared.SplitCSV(*appIDs)),
				asc.WithBetaLicenseAgreementsFields(shared.SplitCSV(*fields)),
				asc.WithBetaLicenseAgreementsAppFields(shared.SplitCSV(*appFields)),
				asc.WithBetaLicenseAgreementsInclude(shared.SplitCSV(*include)),
				asc.Limit[asc.BetaLicenseAgreementsOption](limit),
				asc.NextURL[asc.BetaLicenseAgreementsOption](next),
			)
		},
	})
}

// newBetaLicenseAgreementsGetCommand returns the beta license agreements get subcommand.
func newBetaLicenseAgreementsGetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("view", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "betaLicenseAgreements", "Beta license agreement ID")
	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	fields := fs.String("fields", "", "Fields to include (betaLicenseAgreements), comma-separated")
	appFields := fs.String("app-fields", "", "App fields to include, comma-separated")
	include := fs.String("include", "", "Include related resources (e.g., app), comma-separated")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc testflight beta-license-agreements view --id \"AGREEMENT_ID\" | --app \"APP_ID\"",
		ShortHelp:  "View a beta license agreement by ID or app.",
		LongHelp: `View a beta license agreement by ID or app.

Examples:
  asc testflight beta-license-agreements view --id "AGREEMENT_ID"
  asc testflight beta-license-agreements view --app "APP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			appValue := ""
			if idValue == "" {
				appValue = shared.ResolveAppID(*appID)
			}
			if idValue == "" && appValue == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --id or --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("")
			}
			if idValue != "" && strings.TrimSpace(*appID) != "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --id and --app are mutually exclusive")
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticConflictingInput, "")
			}
			if appValue != "" && (strings.TrimSpace(*appFields) != "" || strings.TrimSpace(*include) != "") {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app-fields and --include are only valid with --id")
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticConflictingInput, "")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("beta-license-agreements view: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			if appValue != "" {
				resp, err := client.GetBetaLicenseAgreementForApp(requestCtx, appValue, shared.SplitCSV(*fields))
				if err != nil {
					return fmt.Errorf("beta-license-agreements view: failed to fetch: %w", err)
				}
				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			opts := []asc.BetaLicenseAgreementOption{
				asc.WithBetaLicenseAgreementFields(shared.SplitCSV(*fields)),
				asc.WithBetaLicenseAgreementAppFields(shared.SplitCSV(*appFields)),
				asc.WithBetaLicenseAgreementInclude(shared.SplitCSV(*include)),
			}
			resp, err := client.GetBetaLicenseAgreement(requestCtx, idValue, opts...)
			if err != nil {
				return fmt.Errorf("beta-license-agreements view: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newBetaLicenseAgreementsUpdateCommand returns the beta license agreements update subcommand.
func newBetaLicenseAgreementsUpdateCommand(d *shared.Deps) *ffcli.Command {
	var agreementText string
	return shared.BuildUpdateCommand(shared.WriteCommandConfig{
		Deps:       d,
		Name:       "update",
		ShortUsage: "asc testflight beta-license-agreements update --id \"AGREEMENT_ID\" --agreement-text \"Text\"",
		ShortHelp:  "Update a beta license agreement.",
		LongHelp: `Update a beta license agreement.

Examples:
  asc testflight beta-license-agreements update --id "AGREEMENT_ID" --agreement-text "Updated terms"`,
		ErrorPrefix: "beta-license-agreements update",
		IDUsage:     "Beta license agreement ID",
		IDType:      "betaLicenseAgreements",
		Fields: []shared.AttributeFlag{
			shared.RequiredStringFlag(&agreementText, "agreement-text", "Updated agreement text"),
		},
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.UpdateBetaLicenseAgreement(ctx, id, &agreementText)
		},
	})
}
