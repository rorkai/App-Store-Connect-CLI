package testflight

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

// BetaLicenseAgreementsCommand returns the beta license agreements command group.
func BetaLicenseAgreementsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("beta-license-agreements", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "beta-license-agreements",
		ShortUsage: "asc testflight beta-license-agreements <subcommand> [flags]",
		ShortHelp:  "Manage TestFlight beta license agreements.",
		LongHelp: `Manage TestFlight beta license agreements.

Examples:
  asc testflight beta-license-agreements list --app "APP_ID"
  asc testflight beta-license-agreements get --id "AGREEMENT_ID"
  asc testflight beta-license-agreements get --app "APP_ID"
  asc testflight beta-license-agreements update --id "AGREEMENT_ID" --agreement-text "Updated terms"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BetaLicenseAgreementsListCommand(),
			BetaLicenseAgreementsGetCommand(),
			BetaLicenseAgreementsUpdateCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BetaLicenseAgreementsListCommand returns the beta license agreements list subcommand.
func BetaLicenseAgreementsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	appIDs := fs.String("app", "", "App Store Connect app ID(s), comma-separated")
	fields := fs.String("fields", "", "Fields to include (betaLicenseAgreements), comma-separated")
	appFields := fs.String("app-fields", "", "App fields to include, comma-separated")
	include := fs.String("include", "", "Include related resources (e.g., app), comma-separated")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc testflight beta-license-agreements list [flags]",
		ShortHelp:  "List beta license agreements.",
		LongHelp: `List beta license agreements.

Examples:
  asc testflight beta-license-agreements list
  asc testflight beta-license-agreements list --app "APP_ID"
  asc testflight beta-license-agreements list --limit 50
  asc testflight beta-license-agreements list --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("beta-license-agreements list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("beta-license-agreements list: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-license-agreements list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.BetaLicenseAgreementsOption{
				asc.WithBetaLicenseAgreementsAppIDs(shared.SplitCSV(*appIDs)),
				asc.WithBetaLicenseAgreementsFields(shared.SplitCSV(*fields)),
				asc.WithBetaLicenseAgreementsAppFields(shared.SplitCSV(*appFields)),
				asc.WithBetaLicenseAgreementsInclude(shared.SplitCSV(*include)),
				asc.WithBetaLicenseAgreementsLimit(*limit),
				asc.WithBetaLicenseAgreementsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithBetaLicenseAgreementsLimit(200))
				firstPage, err := client.GetBetaLicenseAgreements(requestCtx, paginateOpts...)
				if err != nil {
					return fmt.Errorf("beta-license-agreements list: failed to fetch: %w", err)
				}
				agreements, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetBetaLicenseAgreements(ctx, asc.WithBetaLicenseAgreementsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("beta-license-agreements list: %w", err)
				}
				return shared.PrintOutput(agreements, *output, *pretty)
			}

			agreements, err := client.GetBetaLicenseAgreements(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("beta-license-agreements list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(agreements, *output, *pretty)
		},
	}
}

// BetaLicenseAgreementsGetCommand returns the beta license agreements get subcommand.
func BetaLicenseAgreementsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	id := fs.String("id", "", "Beta license agreement ID")
	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	fields := fs.String("fields", "", "Fields to include (betaLicenseAgreements), comma-separated")
	appFields := fs.String("app-fields", "", "App fields to include, comma-separated")
	include := fs.String("include", "", "Include related resources (e.g., app), comma-separated")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc testflight beta-license-agreements get --id \"AGREEMENT_ID\" | --app \"APP_ID\"",
		ShortHelp:  "Get a beta license agreement by ID or app.",
		LongHelp: `Get a beta license agreement by ID or app.

Examples:
  asc testflight beta-license-agreements get --id "AGREEMENT_ID"
  asc testflight beta-license-agreements get --app "APP_ID"`,
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
			if appValue != "" && (strings.TrimSpace(*appFields) != "" || strings.TrimSpace(*include) != "") {
				fmt.Fprintln(os.Stderr, "Error: --app-fields and --include are only valid with --id")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-license-agreements get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if appValue != "" {
				resp, err := client.GetBetaLicenseAgreementForApp(requestCtx, appValue, shared.SplitCSV(*fields))
				if err != nil {
					return fmt.Errorf("beta-license-agreements get: failed to fetch: %w", err)
				}
				return shared.PrintOutput(resp, *output, *pretty)
			}

			opts := []asc.BetaLicenseAgreementOption{
				asc.WithBetaLicenseAgreementFields(shared.SplitCSV(*fields)),
				asc.WithBetaLicenseAgreementAppFields(shared.SplitCSV(*appFields)),
				asc.WithBetaLicenseAgreementInclude(shared.SplitCSV(*include)),
			}
			resp, err := client.GetBetaLicenseAgreement(requestCtx, idValue, opts...)
			if err != nil {
				return fmt.Errorf("beta-license-agreements get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// BetaLicenseAgreementsUpdateCommand returns the beta license agreements update subcommand.
func BetaLicenseAgreementsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	id := fs.String("id", "", "Beta license agreement ID")
	agreementText := fs.String("agreement-text", "", "Updated agreement text")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc testflight beta-license-agreements update --id \"AGREEMENT_ID\" --agreement-text \"Text\"",
		ShortHelp:  "Update a beta license agreement.",
		LongHelp: `Update a beta license agreement.

Examples:
  asc testflight beta-license-agreements update --id "AGREEMENT_ID" --agreement-text "Updated terms"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			textValue := strings.TrimSpace(*agreementText)
			if textValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --agreement-text is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-license-agreements update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateBetaLicenseAgreement(requestCtx, idValue, &textValue)
			if err != nil {
				return fmt.Errorf("beta-license-agreements update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}
