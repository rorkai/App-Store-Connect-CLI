package web

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

var (
	getWebAppCompatibilityFn = func(ctx context.Context, client *webcore.Client, appID string) (*webcore.AppCompatibility, error) {
		return client.GetAppCompatibility(ctx, appID)
	}
	updateWebAppCompatibilityFn = func(ctx context.Context, client *webcore.Client, appID string, iosAppOnMac, iosAppOnVisionPro *bool) (*webcore.AppCompatibility, error) {
		return client.UpdateAppCompatibility(ctx, appID, iosAppOnMac, iosAppOnVisionPro)
	}
)

// newWebAppsCompatibilityCommand returns the web app compatibility command group.
func newWebAppsCompatibilityCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web apps compatibility", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "compatibility",
		ShortUsage: "asc web apps compatibility <subcommand> [flags]",
		ShortHelp:  "Manage App Store Mac and Vision Pro opt-in settings.",
		LongHelp: `WEB SESSION WORKFLOWS

Manage App Store compatibility opt-in settings for iPhone and iPad apps on
Apple silicon Mac and Apple Vision Pro using Apple's internal web API.

`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newWebAppsCompatibilityViewCommand(d),
			newWebAppsCompatibilityEditCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newWebAppsCompatibilityViewCommand returns the compatibility view command.
func newWebAppsCompatibilityViewCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web apps compatibility view", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	authFlags := bindWebSessionFlags(fs)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc web apps compatibility view --app APP_ID [flags]",
		ShortHelp:  "View App Store Mac and Vision Pro opt-in settings.",
		FlagSet:    fs,
		UsageFunc:  d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := strings.TrimSpace(shared.ResolveAppID(*appID))
			if resolvedAppID == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommandWith(ctx, d, authFlags)
			defer cancel()
			if err != nil {
				return err
			}

			var result *webcore.AppCompatibility
			err = withWebSpinnerWith(d, "Fetching App Store compatibility", func() error {
				var err error
				result, err = getWebAppCompatibilityFn(requestCtx, newWebClientFn(session), resolvedAppID)
				return err
			})
			if err != nil {
				return withWebAuthHint(err, "web apps compatibility view")
			}

			return printWebAppCompatibility(d, result, *output.Output, *output.Pretty)
		},
	}
}

// newWebAppsCompatibilityEditCommand returns the compatibility edit command.
func newWebAppsCompatibilityEditCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web apps compatibility edit", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	var iosAppOnMac shared.OptionalBool
	var iosAppOnVisionPro shared.OptionalBool
	fs.Var(&iosAppOnMac, "ios-app-on-mac", "Opt iPhone/iPad app into Mac App Store compatibility: true or false")
	fs.Var(&iosAppOnVisionPro, "ios-app-on-vision-pro", "Opt iPhone/iPad app into Apple Vision Pro App Store compatibility: true or false")
	authFlags := bindWebSessionFlags(fs)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "edit",
		ShortUsage: "asc web apps compatibility edit --app APP_ID [--ios-app-on-mac true|false] [--ios-app-on-vision-pro true|false] [flags]",
		ShortHelp:  "Edit App Store Mac and Vision Pro opt-in settings.",
		FlagSet:    fs,
		UsageFunc:  d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := strings.TrimSpace(shared.ResolveAppID(*appID))
			if resolvedAppID == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}
			if !iosAppOnMac.IsSet() && !iosAppOnVisionPro.IsSet() {
				return d.UpdateFlagRequired(fs, &iosAppOnMac, &iosAppOnVisionPro)
			}

			var macValue *bool
			if iosAppOnMac.IsSet() {
				value := iosAppOnMac.Value()
				macValue = &value
			}
			var visionValue *bool
			if iosAppOnVisionPro.IsSet() {
				value := iosAppOnVisionPro.Value()
				visionValue = &value
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommandWith(ctx, d, authFlags)
			defer cancel()
			if err != nil {
				return err
			}

			var result *webcore.AppCompatibility
			err = withWebSpinnerWith(d, "Updating App Store compatibility", func() error {
				var err error
				result, err = updateWebAppCompatibilityFn(requestCtx, newWebClientFn(session), resolvedAppID, macValue, visionValue)
				return err
			})
			if err != nil {
				return withWebAuthHint(err, "web apps compatibility edit")
			}

			return printWebAppCompatibility(d, result, *output.Output, *output.Pretty)
		},
	}
}

func printWebAppCompatibility(d *shared.Deps, result *webcore.AppCompatibility, output string, pretty bool) error {
	return d.PrintOutputWithRenderers(
		result,
		output,
		pretty,
		func() error {
			asc.RenderTableTo(d.IO.Out, []string{"field", "value"}, webAppCompatibilityRows(result))
			return nil
		},
		func() error {
			asc.RenderMarkdownTo(d.IO.Out, []string{"field", "value"}, webAppCompatibilityRows(result))
			return nil
		},
	)
}

func webAppCompatibilityRows(result *webcore.AppCompatibility) [][]string {
	if result == nil {
		return nil
	}
	return [][]string{
		{"app_id", result.AppID},
		{"ios_app_on_mac", formatWebCompatibilityBool(result.IOSAppOnMac)},
		{"ios_app_on_vision_pro", formatWebCompatibilityBool(result.IOSAppOnVisionPro)},
	}
}

func formatWebCompatibilityBool(value *bool) string {
	if value == nil {
		return "unknown"
	}
	return fmt.Sprintf("%t", *value)
}
