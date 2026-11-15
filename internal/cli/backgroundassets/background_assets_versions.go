package backgroundassets

import (
	"context"
	"flag"
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
		UsageFunc: d.DefaultUsageFunc,
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
	var (
		locale             *string
		sortBy             *string
		platforms          []string
		states             []string
		versions           []string
		internalBetaStates []string
		externalBetaStates []string
		appStoreStates     []string
	)

	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
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
		ParentFlag:           "background-asset-id",
		ParentUsage:          "Background asset ID",
		ParentType:           "backgroundAssets",
		RequiredDiagnostic:   true,
		LimitMax:             backgroundAssetsMaxLimit,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		NextConflicts:        []string{"locale", "platform", "state", "version", "internal-beta-state", "external-beta-state", "app-store-state", "sort"},
		Deps:                 d,
		Flags: func(fs *flag.FlagSet) func() error {
			locale = fs.String("locale", "", "Filter by locale(s), comma-separated (e.g., en-US,ja)")
			platform := fs.String("platform", "", "Filter by platform(s), comma-separated: "+strings.Join(shared.PlatformList(), ", "))
			state := fs.String("state", "", "Filter by state(s), comma-separated: "+strings.Join(backgroundAssetVersionStates, ", "))
			version := fs.String("version", "", "Filter by version(s), comma-separated")
			internalBetaState := fs.String("internal-beta-state", "", "Filter by internal beta release state(s), comma-separated: "+strings.Join(backgroundAssetInternalBetaState, ", "))
			externalBetaState := fs.String("external-beta-state", "", "Filter by external beta release state(s), comma-separated: "+strings.Join(backgroundAssetExternalBetaState, ", "))
			appStoreState := fs.String("app-store-state", "", "Filter by App Store release state(s), comma-separated: "+strings.Join(backgroundAssetAppStoreState, ", "))
			sortBy = fs.String("sort", "", "Sort by: "+strings.Join(backgroundAssetVersionSortValues, ", "))
			return func() error {
				var err error
				platforms, err = parseBackgroundAssetEnumCSV("--platform", *platform, shared.PlatformList())
				if err != nil {
					return d.UsageErrorf("background-assets versions list: %v", err)
				}
				states, err = parseBackgroundAssetEnumCSV("--state", *state, backgroundAssetVersionStates)
				if err != nil {
					return d.UsageErrorf("background-assets versions list: %v", err)
				}
				versions = shared.SplitCSV(*version)
				if strings.TrimSpace(*version) != "" && len(versions) == 0 {
					return d.UsageErrorf("background-assets versions list: --version must include at least one value")
				}
				internalBetaStates, err = parseBackgroundAssetEnumCSV("--internal-beta-state", *internalBetaState, backgroundAssetInternalBetaState)
				if err != nil {
					return d.UsageErrorf("background-assets versions list: %v", err)
				}
				externalBetaStates, err = parseBackgroundAssetEnumCSV("--external-beta-state", *externalBetaState, backgroundAssetExternalBetaState)
				if err != nil {
					return d.UsageErrorf("background-assets versions list: %v", err)
				}
				appStoreStates, err = parseBackgroundAssetEnumCSV("--app-store-state", *appStoreState, backgroundAssetAppStoreState)
				if err != nil {
					return d.UsageErrorf("background-assets versions list: %v", err)
				}
				if err := shared.ValidateSort(*sortBy, backgroundAssetVersionSortValues...); err != nil {
					return d.UsageErrorf("background-assets versions list: %v", err)
				}
				return nil
			}
		},
		FetchPage: func(ctx context.Context, client *asc.Client, assetID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.BackgroundAssetVersionsOption{
				asc.Limit[asc.BackgroundAssetVersionsOption](limit),
				asc.NextURL[asc.BackgroundAssetVersionsOption](next),
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
			return client.GetBackgroundAssetVersions(ctx, assetID, opts...)
		},
	})
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
