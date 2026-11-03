package web

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"slices"
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

var createWebAppBundleFn = func(ctx context.Context, client *webcore.Client, name, sku string, appIDs []string, priceTier string) (*webcore.AppBundleDetail, error) {
	return client.CreateAppBundle(ctx, name, sku, appIDs, priceTier)
}

var listWebAppBundleEligibleAppsFn = func(ctx context.Context, client *webcore.Client) (*webcore.AppBundleEligibleAppsResponse, error) {
	return client.ListAppBundleEligibleApps(ctx)
}

var setWebAppBundleAppsFn = func(ctx context.Context, client *webcore.Client, bundleID string, appIDs []string) (*asc.WebAppBundleAppsResult, error) {
	return client.SetAppBundleApps(ctx, bundleID, appIDs)
}

var submitWebAppBundleFn = func(ctx context.Context, client *webcore.Client, bundleID string) (*asc.WebAppBundleReviewResult, error) {
	return client.SubmitAppBundle(ctx, bundleID)
}

var removeWebAppBundleFromReviewFn = func(ctx context.Context, client *webcore.Client, bundleID string) (*asc.WebAppBundleReviewResult, error) {
	return client.RemoveAppBundleFromReview(ctx, bundleID)
}

// WebAppBundlesCommand returns the app bundles command group.
func WebAppBundlesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web app-bundles", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app-bundles",
		ShortUsage: "asc web app-bundles <subcommand> [flags]",
		ShortHelp:  "Inspect and edit App Store app bundles via web sessions.",
		LongHelp: `WEB SESSION WORKFLOWS

Inspect and edit App Store app bundles, the sets of apps sold together. The
public App Store Connect API has no app bundle resources.

Examples:
  asc web app-bundles list
  asc web app-bundles view --id "BUNDLE_ID" --output table
  asc web app-bundles apps eligible --output table
  asc web app-bundles create --name "My Bundle" --sku "MY_BUNDLE" --apps "APP_ID_1,APP_ID_2" --confirm
  asc web app-bundles apps set --id "BUNDLE_ID" --apps "APP_ID_1,APP_ID_2" --confirm
  asc web app-bundles submit --id "BUNDLE_ID" --confirm
  asc web app-bundles remove-from-review --id "BUNDLE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebAppBundlesListCommand(),
			WebAppBundlesViewCommand(),
			WebAppBundlesCreateCommand(),
			WebAppBundlesAppsCommand(),
			webAppBundleReviewCommand("submit", "Submit an app bundle for App Review.", "Submit an app bundle for App Review. Apple must offer this: canSubmit in\nasc web app-bundles view has to be true. Reading the bundle back confirms the\nnew state.", submitWebAppBundleFn),
			webAppBundleReviewCommand("remove-from-review", "Remove an app bundle from App Review.", "Remove a submitted app bundle from App Review so it can be edited. Apple must\noffer this: canRemoveFromReview in asc web app-bundles view has to be true.\nReading the bundle back confirms the new state.", removeWebAppBundleFromReviewFn),
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

// WebAppBundlesCreateCommand creates an app bundle.
func WebAppBundlesCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web app-bundles create", flag.ExitOnError)
	name := fs.String("name", "", "Bundle name in the primary locale")
	sku := fs.String("sku", "", "Bundle SKU, unique in your account")
	apps := fs.String("apps", "", "Comma-separated app IDs, in the order they should appear")
	priceTier := fs.String("price-tier", "", "Price tier stem (default: the first tier Apple marks compatible with the apps)")
	confirm := fs.Bool("confirm", false, "Confirm creating the bundle (required)")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc web app-bundles create --name NAME --sku SKU --apps APP_ID,APP_ID --confirm [flags]",
		ShortHelp:  "Create an app bundle.",
		LongHelp: `Create an app bundle of approved apps, the way Apple's bundle page does. The
first app is the primary app. Apple fills the primary locale, metadata, and
compatible price tiers from the apps. The new bundle is not cleared for sale,
and Apple offers no way to delete a bundle.

Examples:
  asc web app-bundles apps eligible --output table
  asc web app-bundles create --name "My Bundle" --sku "MY_BUNDLE" --apps "APP_ID_1,APP_ID_2" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("web app-bundles create does not accept positional arguments")
			}
			if strings.TrimSpace(*name) == "" {
				return shared.UsageError("--name is required")
			}
			if strings.TrimSpace(*sku) == "" {
				return shared.UsageError("--sku is required")
			}
			appIDs, err := parseAppBundleAppIDs(*apps)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if !*confirm {
				return shared.UsageError("--confirm is required")
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return withWebAuthHint(err, "web app-bundles create")
			}

			result, err := createWebAppBundleFn(requestCtx, newWebClientFn(session), *name, *sku, appIDs, *priceTier)
			if err != nil {
				return withWebAuthHint(fmt.Errorf("web app-bundles create failed: %w", err), "web app-bundles create")
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

// WebAppBundlesAppsCommand returns the bundle membership command group.
func WebAppBundlesAppsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web app-bundles apps", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "apps",
		ShortUsage: "asc web app-bundles apps <subcommand> [flags]",
		ShortHelp:  "Add, remove, or reorder the apps in a bundle.",
		LongHelp: `Add, remove, or reorder the apps in a bundle.

Examples:
  asc web app-bundles apps eligible
  asc web app-bundles apps set --id "BUNDLE_ID" --apps "APP_ID_1,APP_ID_2" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebAppBundlesAppsEligibleCommand(),
			WebAppBundlesAppsSetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebAppBundlesAppsEligibleCommand lists apps Apple offers for bundles.
func WebAppBundlesAppsEligibleCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web app-bundles apps eligible", flag.ExitOnError)
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "eligible",
		ShortUsage: "asc web app-bundles apps eligible [flags]",
		ShortHelp:  "List the iOS and macOS apps that can join a bundle.",
		LongHelp: `List the iOS and macOS apps Apple offers for adding to an app bundle.

Examples:
  asc web app-bundles apps eligible
  asc web app-bundles apps eligible --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("web app-bundles apps eligible does not accept positional arguments")
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return withWebAuthHint(err, "web app-bundles apps eligible")
			}

			result, err := listWebAppBundleEligibleAppsFn(requestCtx, newWebClientFn(session))
			if err != nil {
				return withWebAuthHint(err, "web app-bundles apps eligible")
			}

			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderAppBundleEligibleApps(result, asc.RenderTable) },
				func() error { return renderAppBundleEligibleApps(result, asc.RenderMarkdown) },
			)
		},
	}
}

// WebAppBundlesAppsSetCommand replaces a bundle's member apps.
func WebAppBundlesAppsSetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web app-bundles apps set", flag.ExitOnError)
	id := fs.String("id", "", "App bundle ID")
	apps := fs.String("apps", "", "Comma-separated app IDs, in the order they should appear")
	confirm := fs.Bool("confirm", false, "Confirm changing the bundle's apps (required)")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: "asc web app-bundles apps set --id BUNDLE_ID --apps APP_ID,APP_ID --confirm [flags]",
		ShortHelp:  "Set a bundle's apps and their order.",
		LongHelp: `Set the apps in a bundle, in the order given. List every app the bundle should
keep: leaving one out removes it, and a new ID adds it. The first app is the
primary app, so moving it can change metadata Apple derives from it, such as the
copyright. Reading the bundle back confirms the change, and a matching list
sends nothing.

Apple only allows adding or removing apps until the bundle is approved. After
that the order can still change. Remove a bundle from review before editing it.

Examples:
  asc web app-bundles apps set --id "BUNDLE_ID" --apps "APP_ID_2,APP_ID_1" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("web app-bundles apps set does not accept positional arguments")
			}
			bundleID := strings.TrimSpace(*id)
			if bundleID == "" {
				return shared.UsageError("--id is required")
			}
			appIDs, err := parseAppBundleAppIDs(*apps)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if !*confirm {
				return shared.UsageError("--confirm is required")
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return withWebAuthHint(err, "web app-bundles apps set")
			}

			result, err := setWebAppBundleAppsFn(requestCtx, newWebClientFn(session), bundleID, appIDs)
			if err != nil {
				return withWebAuthHint(fmt.Errorf("web app-bundles apps set failed: %w", err), "web app-bundles apps set")
			}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func webAppBundleReviewCommand(name, shortHelp, longHelp string, run func(context.Context, *webcore.Client, string) (*asc.WebAppBundleReviewResult, error)) *ffcli.Command {
	commandName := "web app-bundles " + name
	fs := flag.NewFlagSet(commandName, flag.ExitOnError)
	id := fs.String("id", "", "App bundle ID")
	confirm := fs.Bool("confirm", false, "Confirm this review change (required)")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       name,
		ShortUsage: "asc " + commandName + " --id BUNDLE_ID --confirm [flags]",
		ShortHelp:  shortHelp,
		LongHelp:   longHelp + "\n\nExamples:\n  asc " + commandName + ` --id "BUNDLE_ID" --confirm`,
		FlagSet:    fs,
		UsageFunc:  shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError(commandName + " does not accept positional arguments")
			}
			bundleID := strings.TrimSpace(*id)
			if bundleID == "" {
				return shared.UsageError("--id is required")
			}
			if !*confirm {
				return shared.UsageError("--confirm is required")
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return withWebAuthHint(err, commandName)
			}

			result, err := run(requestCtx, newWebClientFn(session), bundleID)
			if err != nil {
				return withWebAuthHint(fmt.Errorf("%s failed: %w", commandName, err), commandName)
			}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func parseAppBundleAppIDs(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("--apps is required")
	}
	var ids []string
	for part := range strings.SplitSeq(raw, ",") {
		id := strings.TrimSpace(part)
		parsed, err := strconv.ParseUint(id, 10, 64)
		if err != nil || strconv.FormatUint(parsed, 10) != id {
			return nil, fmt.Errorf("--apps must contain numeric app IDs, got %q", id)
		}
		if slices.Contains(ids, id) {
			return nil, fmt.Errorf("--apps lists app %s twice", id)
		}
		ids = append(ids, id)
	}
	return ids, nil
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

func renderAppBundleEligibleApps(result *webcore.AppBundleEligibleAppsResponse, render func([]string, [][]string)) error {
	rows := make([][]string, 0, len(result.Data))
	for _, app := range result.Data {
		rows = append(rows, []string{shared.OrNA(app.ID), shared.OrNA(app.Name)})
	}
	render([]string{"ID", "Name"}, rows)
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
