package apps

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newAppsInfoRelationshipsCommand returns the apps info relationships command group.
func newAppsInfoRelationshipsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("apps info relationships", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "relationships",
		ShortUsage: "asc apps info relationships <subcommand> [flags]",
		ShortHelp:  "Get App Info category relationships.",
		LongHelp: `Get App Info category relationships.

Examples:
  asc apps info relationships view --app "APP_ID"
  asc apps info relationships primary-category --app "APP_ID"
  asc apps info relationships primary-category --info-id "APP_INFO_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			appsInfoRelationshipsViewCommand(d),
			appsInfoCategoryRelationshipCommand(d,
				"primary-category",
				"Get the primary category for an app info.",
				func(ctx context.Context, client *asc.Client, id string) (*asc.AppCategoryResponse, error) {
					return client.GetAppInfoPrimaryCategory(ctx, id)
				},
			),
			appsInfoCategoryRelationshipCommand(d,
				"primary-subcategory-one",
				"Get the primary subcategory one for an app info.",
				func(ctx context.Context, client *asc.Client, id string) (*asc.AppCategoryResponse, error) {
					return client.GetAppInfoPrimarySubcategoryOne(ctx, id)
				},
			),
			appsInfoCategoryRelationshipCommand(d,
				"primary-subcategory-two",
				"Get the primary subcategory two for an app info.",
				func(ctx context.Context, client *asc.Client, id string) (*asc.AppCategoryResponse, error) {
					return client.GetAppInfoPrimarySubcategoryTwo(ctx, id)
				},
			),
			appsInfoCategoryRelationshipCommand(d,
				"secondary-category",
				"Get the secondary category for an app info.",
				func(ctx context.Context, client *asc.Client, id string) (*asc.AppCategoryResponse, error) {
					return client.GetAppInfoSecondaryCategory(ctx, id)
				},
			),
			appsInfoCategoryRelationshipCommand(d,
				"secondary-subcategory-one",
				"Get the secondary subcategory one for an app info.",
				func(ctx context.Context, client *asc.Client, id string) (*asc.AppCategoryResponse, error) {
					return client.GetAppInfoSecondarySubcategoryOne(ctx, id)
				},
			),
			appsInfoCategoryRelationshipCommand(d,
				"secondary-subcategory-two",
				"Get the secondary subcategory two for an app info.",
				func(ctx context.Context, client *asc.Client, id string) (*asc.AppCategoryResponse, error) {
					return client.GetAppInfoSecondarySubcategoryTwo(ctx, id)
				},
			),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

type appInfoCategoryFetcher func(ctx context.Context, client *asc.Client, appInfoID string) (*asc.AppCategoryResponse, error)

func appsInfoCategoryRelationshipCommand(d *shared.Deps, name, shortHelp string, fetch appInfoCategoryFetcher) *ffcli.Command {
	fs := d.NewFlagSet("apps info relationships "+name, flag.ExitOnError)

	appID := shared.BindResourceIDFlag(fs, "app", "apps", "App Store Connect app ID (or ASC_APP_ID env)")
	infoID := shared.BindResourceIDFlag(fs, "info-id", "appInfos", "App Info ID (optional override)")
	shared.BindFlagAlias(fs, "id", "info-id")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       name,
		ShortUsage: fmt.Sprintf("asc apps info relationships %s [flags]", name),
		ShortHelp:  shortHelp,
		LongHelp: fmt.Sprintf(`%s

Examples:
  asc apps info relationships %s --app "APP_ID"
  asc apps info relationships %s --info-id "APP_INFO_ID"`, shortHelp, name, name),
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			infoIDValue := strings.TrimSpace(*infoID)
			resolvedAppID := d.ResolveAppID(*appID)
			if resolvedAppID == "" && infoIDValue == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app or --info-id is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("apps info relationships %s: %w", name, err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resolvedInfoID, err := d.ResolveAppInfoIDWithFlag(requestCtx, client, resolvedAppID, infoIDValue, "--info-id")
			if err != nil {
				return fmt.Errorf("apps info relationships %s: %w", name, err)
			}

			resp, err := fetch(requestCtx, client, resolvedInfoID)
			if err != nil {
				return fmt.Errorf("apps info relationships %s: failed to fetch: %w", name, err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// appInfoCategoryRelationships are the app info relationships the category
// subcommands read one at a time.
var appInfoCategoryRelationships = []string{
	"primaryCategory",
	"primarySubcategoryOne",
	"primarySubcategoryTwo",
	"secondaryCategory",
	"secondarySubcategoryOne",
	"secondarySubcategoryTwo",
}

// appsInfoRelationshipsViewCommand reads every category relationship of an
// app info in one request: the app info with those relationships included.
func appsInfoRelationshipsViewCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("apps info relationships view", flag.ExitOnError)

	appID := shared.BindResourceIDFlag(fs, "app", "apps", "App Store Connect app ID (or ASC_APP_ID env)")
	infoID := shared.BindResourceIDFlag(fs, "info-id", "appInfos", "App Info ID (optional override)")
	shared.BindFlagAlias(fs, "id", "info-id")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc apps info relationships view [flags]",
		ShortHelp:  "Get every category relationship for an app info.",
		LongHelp: `Get every category relationship for an app info: the app info with its
primary and secondary categories and subcategories included.

Examples:
  asc apps info relationships view --app "APP_ID"
  asc apps info relationships view --info-id "APP_INFO_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			infoIDValue := strings.TrimSpace(*infoID)
			resolvedAppID := d.ResolveAppID(*appID)
			if resolvedAppID == "" && infoIDValue == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app or --info-id is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("apps info relationships view: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resolvedInfoID, err := d.ResolveAppInfoIDWithFlag(requestCtx, client, resolvedAppID, infoIDValue, "--info-id")
			if err != nil {
				return fmt.Errorf("apps info relationships view: %w", err)
			}

			resp, err := client.GetAppInfo(requestCtx, resolvedInfoID, asc.WithAppInfoInclude(appInfoCategoryRelationships))
			if err != nil {
				return fmt.Errorf("apps info relationships view: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
