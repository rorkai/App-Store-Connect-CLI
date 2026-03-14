package apps

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// AppsInfoRelationshipsCommand returns the apps info relationships command group.
func AppsInfoRelationshipsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("apps info relationships", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "relationships",
		ShortUsage: "asc apps info relationships <subcommand> [flags]",
		ShortHelp:  "Get App Info category relationships.",
		LongHelp: `Get App Info category relationships.

Examples:
  asc apps info relationships primary-category --app "APP_ID"
  asc apps info relationships primary-category --info-id "APP_INFO_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			appsInfoCategoryRelationshipCommand(
				"primary-category",
				"Get the primary category for an app info.",
				func(ctx context.Context, client *asc.Client, id string) (*asc.AppCategoryResponse, error) {
					return client.GetAppInfoPrimaryCategory(ctx, id)
				},
			),
			appsInfoCategoryRelationshipCommand(
				"primary-subcategory-one",
				"Get the primary subcategory one for an app info.",
				func(ctx context.Context, client *asc.Client, id string) (*asc.AppCategoryResponse, error) {
					return client.GetAppInfoPrimarySubcategoryOne(ctx, id)
				},
			),
			appsInfoCategoryRelationshipCommand(
				"primary-subcategory-two",
				"Get the primary subcategory two for an app info.",
				func(ctx context.Context, client *asc.Client, id string) (*asc.AppCategoryResponse, error) {
					return client.GetAppInfoPrimarySubcategoryTwo(ctx, id)
				},
			),
			appsInfoCategoryRelationshipCommand(
				"secondary-category",
				"Get the secondary category for an app info.",
				func(ctx context.Context, client *asc.Client, id string) (*asc.AppCategoryResponse, error) {
					return client.GetAppInfoSecondaryCategory(ctx, id)
				},
			),
			appsInfoCategoryRelationshipCommand(
				"secondary-subcategory-one",
				"Get the secondary subcategory one for an app info.",
				func(ctx context.Context, client *asc.Client, id string) (*asc.AppCategoryResponse, error) {
					return client.GetAppInfoSecondarySubcategoryOne(ctx, id)
				},
			),
			appsInfoCategoryRelationshipCommand(
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

func appsInfoCategoryRelationshipCommand(name, shortHelp string, fetch appInfoCategoryFetcher) *ffcli.Command {
	fs := flag.NewFlagSet("apps info relationships "+name, flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	infoID := fs.String("info-id", "", "App Info ID (optional override)")
	legacyID := fs.String("id", "", "Deprecated alias for --info-id")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       name,
		ShortUsage: fmt.Sprintf("asc apps info relationships %s [flags]", name),
		ShortHelp:  shortHelp,
		LongHelp: fmt.Sprintf(`%s

Examples:
  asc apps info relationships %s --app "APP_ID"
  asc apps info relationships %s --info-id "APP_INFO_ID"`, shortHelp, name, name),
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			infoIDValue, err := resolveInfoIDFlags(*infoID, *legacyID, "--id")
			if err != nil {
				return shared.UsageError(err.Error())
			}
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" && infoIDValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --app or --info-id is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("apps info relationships %s: %w", name, err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resolvedInfoID, err := shared.ResolveAppInfoID(requestCtx, client, resolvedAppID, infoIDValue)
			if err != nil {
				return fmt.Errorf("apps info relationships %s: %w", name, err)
			}

			resp, err := fetch(requestCtx, client, resolvedInfoID)
			if err != nil {
				return fmt.Errorf("apps info relationships %s: failed to fetch: %w", name, err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
