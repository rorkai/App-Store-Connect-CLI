package backgroundassets

import (
	"context"
	"flag"
	"fmt"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// NewBackgroundAssetsCommand returns the background assets command group,
// built on d.
func NewBackgroundAssetsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("background-assets", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "background-assets",
		ShortUsage: "asc background-assets <subcommand> [flags]",
		ShortHelp:  "Manage background assets.",
		LongHelp: `Manage background assets.

Examples:
  asc background-assets list --app "APP_ID"
  asc background-assets view --id "ASSET_ID"
  asc background-assets create --app "APP_ID" --asset-pack-identifier "com.example.assetpack"
  asc background-assets update --id "ASSET_ID" --archived true
  asc background-assets versions list --background-asset-id "ASSET_ID"
  asc background-assets app-store-releases view --id "RELEASE_ID"
  asc background-assets upload-files create --version-id "VERSION_ID" --file "./asset.zip" --asset-type ASSET
  asc background-assets submit --app "APP_ID" --background-asset-id "ASSET_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newBackgroundAssetsListCommand(d),
			newBackgroundAssetsGetCommand(d),
			newBackgroundAssetsCreateCommand(d),
			newBackgroundAssetsUpdateCommand(d),
			newBackgroundAssetsVersionsCommand(d),
			newBackgroundAssetsAppStoreReleasesCommand(d),
			newBackgroundAssetsExternalBetaReleasesCommand(d),
			newBackgroundAssetsInternalBetaReleasesCommand(d),
			newBackgroundAssetsUploadFilesCommand(d),
			newBackgroundAssetsSubmitCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func newBackgroundAssetsListCommand(d *shared.Deps) *ffcli.Command {
	var (
		sortBy               *string
		versionsPlatforms    []string
		archivedFilter       []string
		assetPackIdentifiers []string
		versionsLocales      []string
	)

	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc background-assets list --app \"APP_ID\" [flags]",
		ShortHelp:  "List background assets for an app.",
		LongHelp: `List background assets for an app.

Examples:
  asc background-assets list --app "APP_ID"
  asc background-assets list --app "APP_ID" --archived false
  asc background-assets list --app "APP_ID" --versions-locale "en-US,ja"
  asc background-assets list --app "APP_ID" --versions-platform "IOS,VISION_OS" --sort "-createdDate"
  asc background-assets list --app "APP_ID" --paginate`,
		ParentFlag:           "app",
		ParentUsage:          "App Store Connect app ID (or ASC_APP_ID)",
		ResolveApp:           true,
		RequiredDiagnostic:   true,
		LimitMax:             backgroundAssetsMaxLimit,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		NextConflicts:        []string{"archived", "asset-pack-identifier", "versions-locale", "versions-platform", "sort"},
		Deps:                 d,
		Flags: func(fs *flag.FlagSet) func() error {
			archived := fs.String("archived", "", "Filter by archived state (true/false)")
			assetPackIdentifier := fs.String("asset-pack-identifier", "", "Filter by asset pack identifier(s), comma-separated")
			versionsLocale := fs.String("versions-locale", "", "Filter by uploaded version locale(s), comma-separated (e.g., en-US,ja)")
			versionsPlatform := fs.String("versions-platform", "", "Filter by version platform(s), comma-separated: "+strings.Join(shared.PlatformList(), ", "))
			sortBy = fs.String("sort", "", "Sort by: "+strings.Join(backgroundAssetSortValues, ", "))
			return func() error {
				var err error
				versionsPlatforms, err = parseBackgroundAssetEnumCSV("--versions-platform", *versionsPlatform, shared.PlatformList())
				if err != nil {
					return d.UsageErrorf("background-assets list: %v", err)
				}
				if err := shared.ValidateSort(*sortBy, backgroundAssetSortValues...); err != nil {
					return d.UsageErrorf("background-assets list: %v", err)
				}

				archivedFilter = nil
				if strings.TrimSpace(*archived) != "" {
					value, err := shared.ParseBoolFlag(*archived, "--archived")
					if err != nil {
						return d.UsageErrorf("background-assets list: %v", err)
					}
					archivedFilter = []string{strconv.FormatBool(value)}
				}

				assetPackIdentifiers = shared.SplitCSV(*assetPackIdentifier)
				versionsLocales = shared.SplitCSV(*versionsLocale)
				return nil
			}
		},
		FetchPage: func(ctx context.Context, client *asc.Client, appID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.BackgroundAssetsOption{
				asc.Limit[asc.BackgroundAssetsOption](limit),
				asc.NextURL[asc.BackgroundAssetsOption](next),
				asc.WithBackgroundAssetsFilterVersionsPlatforms(versionsPlatforms),
				asc.WithBackgroundAssetsSort(*sortBy),
			}
			if len(archivedFilter) > 0 {
				opts = append(opts, asc.WithBackgroundAssetsFilterArchived(archivedFilter))
			}
			if len(assetPackIdentifiers) > 0 {
				opts = append(opts, asc.WithBackgroundAssetsFilterAssetPackIdentifier(assetPackIdentifiers))
			}
			if len(versionsLocales) > 0 {
				opts = append(opts, asc.WithBackgroundAssetsFilterVersionsLocale(versionsLocales))
			}
			return client.GetBackgroundAssets(ctx, appID, opts...)
		},
	})
}

func newBackgroundAssetsGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:       d,
		Name:       "view",
		ShortUsage: "asc background-assets view --id \"ASSET_ID\"",
		ShortHelp:  "View a background asset by ID.",
		LongHelp: `View a background asset by ID.

Examples:
  asc background-assets view --id "ASSET_ID"`,
		IDUsage:            "Background asset ID",
		IDType:             "backgroundAssets",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetBackgroundAsset(ctx, id)
		},
	})
}

func newBackgroundAssetsCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	assetPackIdentifier := fs.String("asset-pack-identifier", "", "Asset pack identifier")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc background-assets create --app \"APP_ID\" --asset-pack-identifier \"ASSET_PACK_ID\"",
		ShortHelp:  "Create a background asset.",
		LongHelp: `Create a background asset.

Examples:
  asc background-assets create --app "APP_ID" --asset-pack-identifier "com.example.assetpack"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}

			assetPackIdentifierValue := strings.TrimSpace(*assetPackIdentifier)
			if assetPackIdentifierValue == "" {
				return d.RequiredFlag("--asset-pack-identifier")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("background-assets create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.CreateBackgroundAsset(requestCtx, resolvedAppID, assetPackIdentifierValue)
			if err != nil {
				return fmt.Errorf("background-assets create: failed to create: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func newBackgroundAssetsUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("update", flag.ExitOnError)

	assetID := shared.BindResourceIDFlag(fs, "id", "backgroundAssets", "Background asset ID")
	archived := fs.String("archived", "", "Set archived state (true/false)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc background-assets update --id \"ASSET_ID\" --archived true",
		ShortHelp:  "Update a background asset.",
		LongHelp: `Update a background asset.

Examples:
  asc background-assets update --id "ASSET_ID" --archived true`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			assetIDValue := strings.TrimSpace(*assetID)
			if assetIDValue == "" {
				return d.RequiredFlag("--id")
			}

			if strings.TrimSpace(*archived) == "" {
				return d.RequiredFlag("--archived")
			}
			archivedValue, err := shared.ParseBoolFlag(*archived, "--archived")
			if err != nil {
				return d.UsageErrorf("background-assets update: %v", err)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("background-assets update: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			attrs := asc.BackgroundAssetUpdateAttributes{Archived: &archivedValue}
			resp, err := client.UpdateBackgroundAsset(requestCtx, assetIDValue, attrs)
			if err != nil {
				return fmt.Errorf("background-assets update: failed to update: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
