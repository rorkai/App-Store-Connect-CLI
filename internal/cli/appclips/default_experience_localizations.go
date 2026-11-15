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

// newAppClipDefaultExperienceLocalizationsCommand returns the localizations command group.
func newAppClipDefaultExperienceLocalizationsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("localizations", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "localizations",
		ShortUsage: "asc app-clips default-experiences localizations <subcommand> [flags]",
		ShortHelp:  "Manage App Clip default experience localizations.",
		LongHelp: `Manage App Clip default experience localizations.

Examples:
  asc app-clips default-experiences localizations list --experience-id "EXP_ID"
  asc app-clips default-experiences localizations create --experience-id "EXP_ID" --locale "en-US" --subtitle "Try it"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newAppClipDefaultExperienceLocalizationsListCommand(d),
			newAppClipDefaultExperienceLocalizationsGetCommand(d),
			newAppClipDefaultExperienceLocalizationsCreateCommand(d),
			newAppClipDefaultExperienceLocalizationsUpdateCommand(d),
			newAppClipDefaultExperienceLocalizationsDeleteCommand(d),
			newAppClipDefaultExperienceLocalizationHeaderImageRelationshipCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newAppClipDefaultExperienceLocalizationsListCommand lists localizations.
func newAppClipDefaultExperienceLocalizationsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("list", flag.ExitOnError)

	experienceID := shared.BindResourceIDFlag(fs, "experience-id", "appClipDefaultExperiences", "Default experience ID")
	locale := fs.String("locale", "", "Filter by locale(s), comma-separated")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc app-clips default-experiences localizations list --experience-id \"EXP_ID\" [flags]",
		ShortHelp:  "List localizations for a default experience.",
		LongHelp: `List localizations for a default experience.

Examples:
  asc app-clips default-experiences localizations list --experience-id "EXP_ID"
  asc app-clips default-experiences localizations list --experience-id "EXP_ID" --locale "en-US"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "app-clips default-experiences localizations list", 200, "locale"); err != nil {
				return err
			}

			experienceValue := strings.TrimSpace(*experienceID)
			if experienceValue == "" && strings.TrimSpace(*next) == "" {
				return d.RequiredFlag("--experience-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("app-clips default-experiences localizations list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.AppClipDefaultExperienceLocalizationsOption{
				asc.Limit[asc.AppClipDefaultExperienceLocalizationsOption](*limit),
				asc.NextURL[asc.AppClipDefaultExperienceLocalizationsOption](*next),
				asc.WithAppClipDefaultExperienceLocalizationsLocales(shared.SplitCSV(*locale)),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.AppClipDefaultExperienceLocalizationsOption](200))
				firstPage, err := client.GetAppClipDefaultExperienceLocalizations(requestCtx, experienceValue, paginateOpts...)
				if err != nil {
					if asc.IsNotFound(err) {
						empty := &asc.AppClipDefaultExperienceLocalizationsResponse{Data: []asc.Resource[asc.AppClipDefaultExperienceLocalizationAttributes]{}}
						return d.PrintOutput(empty, *output.Output, *output.Pretty)
					}
					return fmt.Errorf("app-clips default-experiences localizations list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppClipDefaultExperienceLocalizations(ctx, experienceValue, asc.NextURL[asc.AppClipDefaultExperienceLocalizationsOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("app-clips default-experiences localizations list: %w", err)
				}

				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppClipDefaultExperienceLocalizations(requestCtx, experienceValue, opts...)
			if err != nil {
				if asc.IsNotFound(err) {
					empty := &asc.AppClipDefaultExperienceLocalizationsResponse{Data: []asc.Resource[asc.AppClipDefaultExperienceLocalizationAttributes]{}}
					return d.PrintOutput(empty, *output.Output, *output.Pretty)
				}
				return fmt.Errorf("app-clips default-experiences localizations list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newAppClipDefaultExperienceLocalizationsGetCommand gets a localization by ID.
func newAppClipDefaultExperienceLocalizationsGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc app-clips default-experiences localizations view --localization-id \"LOC_ID\"",
		ShortHelp:  "View a localization by ID.",
		LongHelp: `View a localization by ID.

Examples:
  asc app-clips default-experiences localizations view --localization-id "LOC_ID"`,
		IDFlag:             "localization-id",
		IDUsage:            "Localization ID",
		IDType:             "appClipDefaultExperienceLocalizations",
		RequiredDiagnostic: true,
		Deps:               d,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetAppClipDefaultExperienceLocalization(ctx, id)
		},
	})
}

// newAppClipDefaultExperienceLocalizationsCreateCommand creates a localization.
func newAppClipDefaultExperienceLocalizationsCreateCommand(d *shared.Deps) *ffcli.Command {
	var attrs asc.AppClipDefaultExperienceLocalizationCreateAttributes
	return shared.BuildCreateCommand(shared.WriteCommandConfig{
		Name:       "create",
		ShortUsage: "asc app-clips default-experiences localizations create --experience-id \"EXP_ID\" --locale \"en-US\" [flags]",
		ShortHelp:  "Create a localization for a default experience.",
		LongHelp: `Create a localization for a default experience.

Examples:
  asc app-clips default-experiences localizations create --experience-id "EXP_ID" --locale "en-US" --subtitle "Try it"`,
		IDFlag:  "experience-id",
		IDUsage: "Default experience ID",
		IDType:  "appClipDefaultExperiences",
		Fields: []shared.AttributeFlag{
			shared.RequiredStringFlag(&attrs.Locale, "locale", "Locale (e.g., en-US)"),
			shared.OptionalStringFlag(&attrs.Subtitle, "subtitle", "Subtitle"),
		},
		Deps: d,
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.CreateAppClipDefaultExperienceLocalization(ctx, id, attrs)
		},
	})
}

// newAppClipDefaultExperienceLocalizationsUpdateCommand updates a localization.
func newAppClipDefaultExperienceLocalizationsUpdateCommand(d *shared.Deps) *ffcli.Command {
	var subtitle *string
	return shared.BuildUpdateCommand(shared.WriteCommandConfig{
		Name:       "update",
		ShortUsage: "asc app-clips default-experiences localizations update --localization-id \"LOC_ID\" [flags]",
		ShortHelp:  "Update a localization.",
		LongHelp: `Update a localization.

Examples:
  asc app-clips default-experiences localizations update --localization-id "LOC_ID" --subtitle "Try it"`,
		IDFlag:  "localization-id",
		IDUsage: "Localization ID",
		IDType:  "appClipDefaultExperienceLocalizations",
		Fields: []shared.AttributeFlag{
			shared.ExplicitStringFlag(&subtitle, "subtitle", "Subtitle"),
		},
		Deps: d,
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.UpdateAppClipDefaultExperienceLocalization(ctx, id, &asc.AppClipDefaultExperienceLocalizationUpdateAttributes{Subtitle: subtitle})
		},
	})
}

// newAppClipDefaultExperienceLocalizationsDeleteCommand deletes a localization.
func newAppClipDefaultExperienceLocalizationsDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Name:       "delete",
		ShortUsage: "asc app-clips default-experiences localizations delete --localization-id \"LOC_ID\" --confirm",
		ShortHelp:  "Delete a localization.",
		LongHelp: `Delete a localization.

Examples:
  asc app-clips default-experiences localizations delete --localization-id "LOC_ID" --confirm`,
		IDFlag:             "localization-id",
		IDUsage:            "Localization ID",
		IDType:             "appClipDefaultExperienceLocalizations",
		RequiredDiagnostic: true,
		Deps:               d,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteAppClipDefaultExperienceLocalization(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
	})
}

// newAppClipDefaultExperienceLocalizationHeaderImageRelationshipCommand returns the header image relationship command.
func newAppClipDefaultExperienceLocalizationHeaderImageRelationshipCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("header-image-relationship", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "appClipDefaultExperienceLocalizations", "Localization ID")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "header-image-relationship",
		ShortUsage: "asc app-clips default-experiences localizations header-image-relationship --localization-id \"LOC_ID\"",
		ShortHelp:  "Get header image relationship for a localization.",
		LongHelp: `Get header image relationship for a localization.

Examples:
  asc app-clips default-experiences localizations header-image-relationship --localization-id "LOC_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			locValue := strings.TrimSpace(*localizationID)
			if locValue == "" {
				return d.RequiredFlag("--localization-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("app-clips default-experiences localizations header-image-relationship: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetAppClipDefaultExperienceLocalizationHeaderImageRelationship(requestCtx, locValue)
			if err != nil {
				return fmt.Errorf("app-clips default-experiences localizations header-image-relationship: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
