package localizations

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// LocalizationsPreviewSetsCommand returns the preview sets command group.
func LocalizationsPreviewSetsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("preview-sets", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "preview-sets",
		ShortUsage: "asc localizations preview-sets <subcommand> [flags]",
		ShortHelp:  "Manage preview sets for an App Store localization.",
		LongHelp: `Manage preview sets for an App Store localization.

Examples:
  asc localizations preview-sets list --localization-id "LOCALIZATION_ID"
  asc localizations preview-sets view --id "PREVIEW_SET_ID"
  asc localizations preview-sets links --localization-id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.VisibleUsageFunc,
		Subcommands: []*ffcli.Command{
			LocalizationsPreviewSetsListCommand(),
			LocalizationsPreviewSetsGetCommand(),
			LocalizationsPreviewSetsRelationshipsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// LocalizationsPreviewSetsListCommand returns the preview sets list subcommand.
func LocalizationsPreviewSetsListCommand() *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "localizations preview-sets list",
		Name:        "list",
		ShortUsage:  "asc localizations preview-sets list --localization-id \"LOCALIZATION_ID\"",
		ShortHelp:   "List preview sets for an App Store localization.",
		LongHelp: `List preview sets for an App Store localization.

Examples:
  asc localizations preview-sets list --localization-id "LOCALIZATION_ID"`,
		ParentFlag:  "localization-id",
		ParentUsage: "App Store version localization ID",
		ParentType:  "appStoreVersionLocalizations",
		LimitMax:    200,
		ErrorPrefix: "localizations preview-sets list",
		FetchPage: func(ctx context.Context, client *asc.Client, localizationID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.AppStoreVersionLocalizationPreviewSetsOption{
				asc.Limit[asc.AppStoreVersionLocalizationPreviewSetsOption](limit),
				asc.NextURL[asc.AppStoreVersionLocalizationPreviewSetsOption](next),
			}
			resp, err := client.GetAppStoreVersionLocalizationPreviewSets(ctx, localizationID, opts...)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch: %w", err)
			}
			return resp, nil
		},
	})
}

// LocalizationsPreviewSetsGetCommand returns the preview sets get subcommand.
func LocalizationsPreviewSetsGetCommand() *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "localizations preview-sets view",
		Name:        "view",
		ShortUsage:  "asc localizations preview-sets view --id \"PREVIEW_SET_ID\"",
		ShortHelp:   "View an app preview set by ID.",
		LongHelp: `View an app preview set by ID.

Examples:
  asc localizations preview-sets view --id "PREVIEW_SET_ID"`,
		IDFlag:      "id",
		IDUsage:     "App preview set ID",
		IDType:      "appPreviewSets",
		ErrorPrefix: "localizations preview-sets view",
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetAppPreviewSet(ctx, id)
		},
	})
}

// LocalizationsPreviewSetsRelationshipsCommand returns the preview sets links subcommand.
func LocalizationsPreviewSetsRelationshipsCommand() *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "localizations preview-sets links",
		Name:        "links",
		ShortUsage:  "asc localizations preview-sets links --localization-id \"LOCALIZATION_ID\"",
		ShortHelp:   "List preview set relationships for an App Store localization.",
		LongHelp: `List preview set relationships for an App Store localization.

Examples:
  asc localizations preview-sets links --localization-id "LOCALIZATION_ID"`,
		ParentFlag:  "localization-id",
		ParentUsage: "App Store version localization ID",
		ParentType:  "appStoreVersionLocalizations",
		LimitMax:    200,
		ErrorPrefix: "localizations preview-sets links",
		FetchPage: func(ctx context.Context, client *asc.Client, localizationID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.LinkagesOption{
				asc.Limit[asc.LinkagesOption](limit),
				asc.NextURL[asc.LinkagesOption](next),
			}
			resp, err := client.GetAppStoreVersionLocalizationPreviewSetsRelationships(ctx, localizationID, opts...)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch: %w", err)
			}
			return resp, nil
		},
	})
}

// LocalizationsScreenshotSetsCommand returns the screenshot sets command group.
func LocalizationsScreenshotSetsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("screenshot-sets", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "screenshot-sets",
		ShortUsage: "asc localizations screenshot-sets <subcommand> [flags]",
		ShortHelp:  "Manage screenshot sets for an App Store localization.",
		LongHelp: `Manage screenshot sets for an App Store localization.

Examples:
  asc localizations screenshot-sets list --localization-id "LOCALIZATION_ID"
  asc localizations screenshot-sets view --id "SCREENSHOT_SET_ID"
  asc localizations screenshot-sets delete --id "SCREENSHOT_SET_ID" --confirm
  asc localizations screenshot-sets links --localization-id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.VisibleUsageFunc,
		Subcommands: []*ffcli.Command{
			LocalizationsScreenshotSetsListCommand(),
			LocalizationsScreenshotSetsGetCommand(),
			LocalizationsScreenshotSetsDeleteCommand(),
			LocalizationsScreenshotSetsRelationshipsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// LocalizationsScreenshotSetsGetCommand returns the screenshot sets get subcommand.
func LocalizationsScreenshotSetsGetCommand() *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "localizations screenshot-sets view",
		Name:        "view",
		ShortUsage:  "asc localizations screenshot-sets view --id \"SCREENSHOT_SET_ID\"",
		ShortHelp:   "View an app screenshot set by ID.",
		LongHelp: `View an app screenshot set by ID.

Examples:
  asc localizations screenshot-sets view --id "SCREENSHOT_SET_ID"`,
		IDFlag:      "id",
		IDUsage:     "App screenshot set ID",
		IDType:      "appScreenshotSets",
		ErrorPrefix: "localizations screenshot-sets view",
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetAppScreenshotSet(ctx, id)
		},
	})
}

// LocalizationsScreenshotSetsDeleteCommand returns the screenshot sets delete subcommand.
func LocalizationsScreenshotSetsDeleteCommand() *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		FlagSetName: "localizations screenshot-sets delete",
		Name:        "delete",
		ShortUsage:  "asc localizations screenshot-sets delete --id \"SCREENSHOT_SET_ID\" --confirm",
		ShortHelp:   "Delete a screenshot set by ID.",
		LongHelp: `Delete a screenshot set by ID.

Examples:
  asc localizations screenshot-sets delete --id "SCREENSHOT_SET_ID" --confirm`,
		IDFlag:                 "id",
		IDUsage:                "App screenshot set ID",
		IDType:                 "appScreenshotSets",
		RequiredDiagnostic:     true,
		ConfirmRequiredMessage: "--confirm is required to delete",
		ErrorPrefix:            "localizations screenshot-sets delete",
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteAppScreenshotSet(ctx, id)
		},
		Result: func(id string) any {
			return &asc.AssetDeleteResult{ID: id, Deleted: true}
		},
	})
}

// LocalizationsScreenshotSetsListCommand returns the screenshot sets list subcommand.
func LocalizationsScreenshotSetsListCommand() *ffcli.Command {
	include := new(string)
	var included asc.IncludedFields
	cmd := shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "localizations screenshot-sets list",
		Name:        "list",
		ShortUsage:  "asc localizations screenshot-sets list --localization-id \"LOCALIZATION_ID\" [--include appScreenshots]",
		ShortHelp:   "List screenshot sets for an App Store localization.",
		LongHelp: `List screenshot sets for an App Store localization.

--include appScreenshots adds each set's screenshots to the response's
"included" array.

Examples:
  asc localizations screenshot-sets list --localization-id "LOCALIZATION_ID"
  asc localizations screenshot-sets list --localization-id "LOCALIZATION_ID" --include appScreenshots`,
		ParentFlag:  "localization-id",
		ParentUsage: "App Store version localization ID",
		ParentType:  "appStoreVersionLocalizations",
		LimitMax:    200,
		ErrorPrefix: "localizations screenshot-sets list",
		FetchPage: func(ctx context.Context, client *asc.Client, localizationID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.AppStoreVersionLocalizationScreenshotSetsOption{
				asc.Limit[asc.AppStoreVersionLocalizationScreenshotSetsOption](limit),
				asc.NextURL[asc.AppStoreVersionLocalizationScreenshotSetsOption](next),
			}
			if *include != "" {
				opts = append(opts, asc.WithAppStoreVersionLocalizationScreenshotSetsIncludeScreenshots(), asc.WithIncludedFields[asc.AppStoreVersionLocalizationScreenshotSetsOption](included))
			}
			resp, err := client.GetAppStoreVersionLocalizationScreenshotSets(ctx, localizationID, opts...)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch: %w", err)
			}
			return resp, nil
		},
	})
	cmd.FlagSet.StringVar(include, "include", "", "Include related resources: appScreenshots")
	includedFields := shared.BindIncludedFieldFlags(cmd.FlagSet, map[string]string{"appScreenshots": "appScreenshots"})
	list := cmd.Exec
	cmd.Exec = func(ctx context.Context, args []string) error {
		*include = strings.TrimSpace(*include)
		if *include != "" && *include != "appScreenshots" {
			return shared.UsageError("localizations screenshot-sets list: --include must be appScreenshots")
		}
		if *include != "" && strings.TrimSpace(cmd.FlagSet.Lookup("next").Value.String()) != "" {
			return shared.UsageError("localizations screenshot-sets list: --include cannot be combined with --next; the next URL keeps the original include")
		}
		var err error
		included, err = includedFields.Resolve("localizations screenshot-sets list", cmd.FlagSet.Lookup("next").Value.String(), shared.SplitCSV(*include))
		if err != nil {
			return err
		}
		return list(ctx, args)
	}
	return cmd
}

// LocalizationsScreenshotSetsRelationshipsCommand returns the screenshot sets links subcommand.
func LocalizationsScreenshotSetsRelationshipsCommand() *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "localizations screenshot-sets links",
		Name:        "links",
		ShortUsage:  "asc localizations screenshot-sets links --localization-id \"LOCALIZATION_ID\"",
		ShortHelp:   "List screenshot set relationships for an App Store localization.",
		LongHelp: `List screenshot set relationships for an App Store localization.

Examples:
  asc localizations screenshot-sets links --localization-id "LOCALIZATION_ID"`,
		ParentFlag:  "localization-id",
		ParentUsage: "App Store version localization ID",
		ParentType:  "appStoreVersionLocalizations",
		LimitMax:    200,
		ErrorPrefix: "localizations screenshot-sets links",
		FetchPage: func(ctx context.Context, client *asc.Client, localizationID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.LinkagesOption{
				asc.Limit[asc.LinkagesOption](limit),
				asc.NextURL[asc.LinkagesOption](next),
			}
			resp, err := client.GetAppStoreVersionLocalizationScreenshotSetsRelationships(ctx, localizationID, opts...)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch: %w", err)
			}
			return resp, nil
		},
	})
}
