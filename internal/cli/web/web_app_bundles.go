package web

import (
	"context"
	"flag"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

var listWebAppBundlesFn = func(ctx context.Context, client *webcore.Client) (*webcore.AppBundlesListResponse, error) {
	return client.ListAppBundles(ctx)
}

var getWebAppBundleFn = func(ctx context.Context, client *webcore.Client, bundleID string) (*webcore.AppBundleDetail, error) {
	return client.GetAppBundle(ctx, bundleID)
}

// WebAppBundlesCommand returns the app bundles command group.
func WebAppBundlesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web app-bundles", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app-bundles",
		ShortUsage: "asc web app-bundles <subcommand> [flags]",
		ShortHelp:  "Inspect App Store app bundles via web sessions.",
		LongHelp: `WEB SESSION WORKFLOWS

Inspect App Store app bundles, the sets of apps sold together. The public App
Store Connect API has no app bundle resources.

Examples:
  asc web app-bundles list
  asc web app-bundles view --id "BUNDLE_ID" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebAppBundlesListCommand(),
			WebAppBundlesViewCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebAppBundlesListCommand lists app bundles using the internal web API.
func WebAppBundlesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web app-bundles list", flag.ExitOnError)
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc web app-bundles list [flags]",
		ShortHelp:  "List app bundles and their review status.",
		LongHelp: `List every app bundle the web session can see, with its platforms and review
status. All pages are always fetched.

Examples:
  asc web app-bundles list
  asc web app-bundles list --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("web app-bundles list does not accept positional arguments")
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return withWebAuthHint(err, "web app-bundles list")
			}

			result, err := listWebAppBundlesFn(requestCtx, newWebClientFn(session))
			if err != nil {
				return withWebAuthHint(err, "web app-bundles list")
			}

			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderAppBundles(result, asc.RenderTable) },
				func() error { return renderAppBundles(result, asc.RenderMarkdown) },
			)
		},
	}
}

// WebAppBundlesViewCommand shows one bundle and its member apps.
func WebAppBundlesViewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web app-bundles view", flag.ExitOnError)
	id := fs.String("id", "", "App bundle ID")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc web app-bundles view --id BUNDLE_ID [flags]",
		ShortHelp:  "View an app bundle's status and member apps.",
		LongHelp: `View an app bundle's status, sale state, and member apps in display order.

Examples:
  asc web app-bundles view --id "BUNDLE_ID"
  asc web app-bundles view --id "BUNDLE_ID" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("web app-bundles view does not accept positional arguments")
			}
			bundleID := strings.TrimSpace(*id)
			if bundleID == "" {
				return shared.UsageError("--id is required")
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return withWebAuthHint(err, "web app-bundles view")
			}

			result, err := getWebAppBundleFn(requestCtx, newWebClientFn(session), bundleID)
			if err != nil {
				return withWebAuthHint(err, "web app-bundles view")
			}

			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderAppBundleDetail(result, asc.RenderTable) },
				func() error { return renderAppBundleDetail(result, asc.RenderMarkdown) },
			)
		},
	}
}

func renderAppBundles(result *webcore.AppBundlesListResponse, render func([]string, [][]string)) error {
	rows := make([][]string, 0, len(result.Data))
	for _, bundle := range result.Data {
		rows = append(rows, []string{
			shared.OrNA(bundle.ID),
			shared.OrNA(bundle.Name),
			shared.OrNA(strings.Join(bundle.Platforms, ",")),
			shared.OrNA(bundle.Status),
		})
	}
	render([]string{"ID", "Name", "Platforms", "Status"}, rows)
	return nil
}

func renderAppBundleDetail(result *webcore.AppBundleDetail, render func([]string, [][]string)) error {
	render(
		[]string{"ID", "State", "Cleared For Sale", "Can Submit", "Can Remove From Review"},
		[][]string{{
			shared.OrNA(result.ID),
			shared.OrNA(result.State),
			strconv.FormatBool(result.ClearedForSale),
			strconv.FormatBool(result.CanSubmit),
			strconv.FormatBool(result.CanRemoveFromReview),
		}},
	)
	rows := make([][]string, 0, len(result.Apps))
	for i, app := range result.Apps {
		rows = append(rows, []string{
			strconv.Itoa(i + 1),
			shared.OrNA(app.ID),
			shared.OrNA(app.Name),
			shared.OrNA(app.Type),
			shared.OrNA(app.PriceTier),
		})
	}
	render([]string{"Position", "App ID", "Name", "Type", "Price Tier"}, rows)
	return nil
}
