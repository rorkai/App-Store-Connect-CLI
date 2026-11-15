package backgroundassets

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func newBackgroundAssetsVersionsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("versions", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "versions",
		ShortUsage: "asc background-assets versions <subcommand> [flags]",
		ShortHelp:  "Manage background asset versions.",
		LongHelp: `Manage background asset versions.

Examples:
  asc background-assets versions list --background-asset-id "ASSET_ID"
  asc background-assets versions view --version-id "VERSION_ID"
  asc background-assets versions create --background-asset-id "ASSET_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newBackgroundAssetsVersionsListCommand(d),
			newBackgroundAssetsVersionsGetCommand(d),
			newBackgroundAssetsVersionsCreateCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func newBackgroundAssetsVersionsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("list", flag.ExitOnError)

	assetID := shared.BindResourceIDFlag(fs, "background-asset-id", "backgroundAssets", "Background asset ID")
	locale := fs.String("locale", "", "Filter by locale(s), comma-separated (e.g., en-US,ja)")
	platform := fs.String("platform", "", "Filter by platform(s), comma-separated: "+strings.Join(shared.PlatformList(), ", "))
	state := fs.String("state", "", "Filter by state(s), comma-separated: "+strings.Join(backgroundAssetVersionStates, ", "))
	version := fs.String("version", "", "Filter by version(s), comma-separated")
	internalBetaState := fs.String("internal-beta-state", "", "Filter by internal beta release state(s), comma-separated: "+strings.Join(backgroundAssetInternalBetaState, ", "))
	externalBetaState := fs.String("external-beta-state", "", "Filter by external beta release state(s), comma-separated: "+strings.Join(backgroundAssetExternalBetaState, ", "))
	appStoreState := fs.String("app-store-state", "", "Filter by App Store release state(s), comma-separated: "+strings.Join(backgroundAssetAppStoreState, ", "))
	sortBy := fs.String("sort", "", "Sort by: "+strings.Join(backgroundAssetVersionSortValues, ", "))
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc background-assets versions list --background-asset-id \"ASSET_ID\"",
		ShortHelp:  "List versions for a background asset.",
		LongHelp: `List versions for a background asset.

Examples:
  asc background-assets versions list --background-asset-id "ASSET_ID"
  asc background-assets versions list --background-asset-id "ASSET_ID" --locale "en-US"
  asc background-assets versions list --background-asset-id "ASSET_ID" --platform "IOS" --state "COMPLETE" --sort "-version"
  asc background-assets versions list --background-asset-id "ASSET_ID" --app-store-state "READY_FOR_REVIEW,IN_REVIEW"
  asc background-assets versions list --background-asset-id "ASSET_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "background-assets versions list", backgroundAssetsMaxLimit, "locale", "platform", "state", "version", "internal-beta-state", "external-beta-state", "app-store-state", "sort"); err != nil {
				return err
			}
			assetIDValue := strings.TrimSpace(*assetID)
			if assetIDValue == "" && strings.TrimSpace(*next) == "" {
				return d.RequiredFlag("--background-asset-id")
			}
			platforms, err := parseBackgroundAssetEnumCSV("--platform", *platform, shared.PlatformList())
			if err != nil {
				return d.UsageErrorf("background-assets versions list: %v", err)
			}
			states, err := parseBackgroundAssetEnumCSV("--state", *state, backgroundAssetVersionStates)
			if err != nil {
				return d.UsageErrorf("background-assets versions list: %v", err)
			}
			versions := shared.SplitCSV(*version)
			if strings.TrimSpace(*version) != "" && len(versions) == 0 {
				return d.UsageErrorf("background-assets versions list: --version must include at least one value")
			}
			internalBetaStates, err := parseBackgroundAssetEnumCSV("--internal-beta-state", *internalBetaState, backgroundAssetInternalBetaState)
			if err != nil {
				return d.UsageErrorf("background-assets versions list: %v", err)
			}
			externalBetaStates, err := parseBackgroundAssetEnumCSV("--external-beta-state", *externalBetaState, backgroundAssetExternalBetaState)
			if err != nil {
				return d.UsageErrorf("background-assets versions list: %v", err)
			}
			appStoreStates, err := parseBackgroundAssetEnumCSV("--app-store-state", *appStoreState, backgroundAssetAppStoreState)
			if err != nil {
				return d.UsageErrorf("background-assets versions list: %v", err)
			}
			if err := shared.ValidateSort(*sortBy, backgroundAssetVersionSortValues...); err != nil {
				return d.UsageErrorf("background-assets versions list: %v", err)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("background-assets versions list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.BackgroundAssetVersionsOption{
				asc.Limit[asc.BackgroundAssetVersionsOption](*limit),
				asc.NextURL[asc.BackgroundAssetVersionsOption](*next),
				asc.WithBackgroundAssetVersionsFilterPlatforms(platforms),
				asc.WithBackgroundAssetVersionsFilterState(states),
				asc.WithBackgroundAssetVersionsFilterVersion(versions),
				asc.WithBackgroundAssetVersionsFilterInternalBetaState(internalBetaStates),
				asc.WithBackgroundAssetVersionsFilterExternalBetaState(externalBetaStates),
				asc.WithBackgroundAssetVersionsFilterAppStoreState(appStoreStates),
				asc.WithBackgroundAssetVersionsSort(*sortBy),
			}
			if locales := shared.SplitCSV(*locale); len(locales) > 0 {
				opts = append(opts, asc.WithBackgroundAssetVersionsFilterLocale(locales))
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.BackgroundAssetVersionsOption](backgroundAssetsMaxLimit))
				firstPage, err := client.GetBackgroundAssetVersions(requestCtx, assetIDValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("background-assets versions list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetBackgroundAssetVersions(ctx, assetIDValue, asc.NextURL[asc.BackgroundAssetVersionsOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("background-assets versions list: %w", err)
				}

				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetBackgroundAssetVersions(requestCtx, assetIDValue, opts...)
			if err != nil {
				return fmt.Errorf("background-assets versions list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func newBackgroundAssetsVersionsGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:       d,
		Name:       "view",
		ShortUsage: "asc background-assets versions view --version-id \"VERSION_ID\"",
		ShortHelp:  "View a background asset version by ID.",
		LongHelp: `View a background asset version by ID.

Examples:
  asc background-assets versions view --version-id "VERSION_ID"`,
		IDFlag:             "version-id",
		IDUsage:            "Background asset version ID",
		IDType:             "backgroundAssetVersions",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetBackgroundAssetVersion(ctx, id)
		},
	})
}

func newBackgroundAssetsVersionsCreateCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildCreateCommand(shared.WriteCommandConfig{
		Name:       "create",
		ShortUsage: "asc background-assets versions create --background-asset-id \"ASSET_ID\"",
		ShortHelp:  "Create a background asset version.",
		LongHelp: `Create a background asset version.

Examples:
  asc background-assets versions create --background-asset-id "ASSET_ID"`,
		IDFlag:  "background-asset-id",
		IDUsage: "Background asset ID",
		IDType:  "backgroundAssets",
		Deps:    d,
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.CreateBackgroundAssetVersion(ctx, id)
		},
	})
}
