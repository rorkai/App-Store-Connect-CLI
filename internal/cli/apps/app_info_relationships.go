package apps

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

// AppInfoRelationshipsCommand returns the app-info relationships command group.
func AppInfoRelationshipsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app-info relationships", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "relationships",
		ShortUsage: "asc app-info relationships <subcommand> [flags]",
		ShortHelp:  "Get App Info category relationships.",
		LongHelp: `Get App Info category relationships.

Examples:
  asc app-info relationships primary-category --id "APP_INFO_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			appInfoCategoryRelationshipCommand(
				"primary-category",
				"Get the primary category for an app info.",
				func(ctx context.Context, client *asc.Client, id string) (*asc.AppCategoryResponse, error) {
					return client.GetAppInfoPrimaryCategory(ctx, id)
				},
			),
			appInfoCategoryRelationshipCommand(
				"primary-subcategory-one",
				"Get the primary subcategory one for an app info.",
				func(ctx context.Context, client *asc.Client, id string) (*asc.AppCategoryResponse, error) {
					return client.GetAppInfoPrimarySubcategoryOne(ctx, id)
				},
			),
			appInfoCategoryRelationshipCommand(
				"primary-subcategory-two",
				"Get the primary subcategory two for an app info.",
				func(ctx context.Context, client *asc.Client, id string) (*asc.AppCategoryResponse, error) {
					return client.GetAppInfoPrimarySubcategoryTwo(ctx, id)
				},
			),
			appInfoCategoryRelationshipCommand(
				"secondary-category",
				"Get the secondary category for an app info.",
				func(ctx context.Context, client *asc.Client, id string) (*asc.AppCategoryResponse, error) {
					return client.GetAppInfoSecondaryCategory(ctx, id)
				},
			),
			appInfoCategoryRelationshipCommand(
				"secondary-subcategory-one",
				"Get the secondary subcategory one for an app info.",
				func(ctx context.Context, client *asc.Client, id string) (*asc.AppCategoryResponse, error) {
					return client.GetAppInfoSecondarySubcategoryOne(ctx, id)
				},
			),
			appInfoCategoryRelationshipCommand(
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

func appInfoCategoryRelationshipCommand(name, shortHelp string, fetch appInfoCategoryFetcher) *ffcli.Command {
	fs := flag.NewFlagSet("app-info relationships "+name, flag.ExitOnError)

	appInfoID := fs.String("id", "", "App Info ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       name,
		ShortUsage: fmt.Sprintf("asc app-info relationships %s --id \"APP_INFO_ID\"", name),
		ShortHelp:  shortHelp,
		LongHelp: fmt.Sprintf(`%s

Examples:
  asc app-info relationships %s --id "APP_INFO_ID"`, shortHelp, name),
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*appInfoID)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-info relationships %s: %w", name, err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := fetch(requestCtx, client, idValue)
			if err != nil {
				return fmt.Errorf("app-info relationships %s: failed to fetch: %w", name, err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
