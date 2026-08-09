package web

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

var syncAppClipBundleIDCapabilityFn = func(ctx context.Context, client *webcore.Client, req webcore.AppClipBundleIDCapabilitySyncRequest) (*webcore.AppClipBundleIDCapabilitySyncResult, error) {
	return client.SyncAppClipBundleIDCapability(ctx, req)
}

var enableDeveloperBundleIDCapabilityFn = func(ctx context.Context, client *webcore.Client, req webcore.DeveloperBundleIDCapabilityEnableRequest) (*webcore.DeveloperBundleIDCapabilityEnableResult, error) {
	return client.EnableDeveloperBundleIDCapability(ctx, req)
}

// WebBundleIDsCommand returns the Bundle ID command group.
func WebBundleIDsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web bundle-ids", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "bundle-ids",
		ShortUsage: "asc web bundle-ids <subcommand> [flags]",
		ShortHelp:  "Manage Bundle IDs via web-session endpoints.",
		LongHelp: `WEB SESSION WORKFLOWS

Manage Bundle ID operations that are only available through Apple web-session web-session endpoints.

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebBundleIDCapabilitiesCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebBundleIDCapabilitiesCommand returns the Bundle ID capabilities group.
func WebBundleIDCapabilitiesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web bundle-ids capabilities", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "capabilities",
		ShortUsage: "asc web bundle-ids capabilities <subcommand> [flags]",
		ShortHelp:  "Manage Bundle ID capabilities via web sessions.",
		LongHelp: `WEB SESSION WORKFLOWS

Manage Bundle ID capabilities through Apple's App Store Connect and Developer
Portal web-session endpoints.

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebBundleIDCapabilitiesEnableCommand(),
			WebBundleIDCapabilitiesSyncAppClipCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebBundleIDCapabilitiesEnableCommand enables a Developer Portal-only Bundle
// ID capability while preserving all existing capability relationships.
func WebBundleIDCapabilitiesEnableCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web bundle-ids capabilities enable", flag.ExitOnError)

	bundleID := fs.String("bundle-id", "", "Opaque Developer Portal Bundle ID resource ID")
	capability := fs.String("capability", "", "Developer Portal capability ID (supported: PRIVATE_CLOUD_COMPUTE)")
	confirm := fs.Bool("confirm", false, "Confirm enabling this Bundle ID capability")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "enable",
		ShortUsage: "asc web bundle-ids capabilities enable --bundle-id BUNDLE_RESOURCE_ID --capability PRIVATE_CLOUD_COMPUTE --confirm [flags]",
		ShortHelp:  "Enable a Developer Portal-only Bundle ID capability.",
		LongHelp: `WEB SESSION WORKFLOWS

Enable a Bundle ID capability that is exposed by Apple Developer Portal but is
absent from the public App Store Connect capability enum.

The command loads Developer Portal capability metadata and the complete current
Bundle ID capability graph before saving. Existing settings and relationships
are preserved. If the requested capability is already enabled, the command
returns an already-enabled result without sending a PATCH.

Currently supported capability IDs:
  PRIVATE_CLOUD_COMPUTE

Example:
  asc web bundle-ids capabilities enable --bundle-id "BUNDLE_RESOURCE_ID" --capability "PRIVATE_CLOUD_COMPUTE" --confirm

Authentication:
  This command needs Developer Portal cookies and CSRF headers derived from the
  user-owned Apple web session. If a cached App Store Connect-only session cannot
  be promoted, clear only its cached session and log in again with the same binary:
  asc web auth logout --apple-id "user@example.com"
  asc web auth login --apple-id "user@example.com"

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("web bundle-ids capabilities enable does not accept positional arguments")
			}

			resolvedBundleID := strings.TrimSpace(*bundleID)
			resolvedCapability := strings.ToUpper(strings.TrimSpace(*capability))
			switch {
			case resolvedBundleID == "":
				return shared.UsageError("--bundle-id is required")
			case resolvedCapability == "":
				return shared.UsageError("--capability is required")
			case !*confirm:
				return shared.UsageError("--confirm is required")
			case resolvedCapability != "PRIVATE_CLOUD_COMPUTE":
				return shared.UsageErrorf("unsupported Developer Portal capability %q (supported: PRIVATE_CLOUD_COMPUTE)", resolvedCapability)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			session, err := resolveWebSessionForCommand(requestCtx, authFlags)
			if err != nil {
				return err
			}
			client := newWebClientFn(session)

			var result *webcore.DeveloperBundleIDCapabilityEnableResult
			err = withWebSpinner("Enabling Developer Portal Bundle ID capability", func() error {
				var enableErr error
				result, enableErr = enableDeveloperBundleIDCapabilityFn(requestCtx, client, webcore.DeveloperBundleIDCapabilityEnableRequest{
					BundleID:   resolvedBundleID,
					Capability: resolvedCapability,
				})
				return enableErr
			})
			if err != nil {
				return withWebAuthHint(err, "web bundle-ids capabilities enable")
			}
			if result == nil {
				return fmt.Errorf("web bundle-ids capabilities enable failed: missing enable result")
			}
			// Developer Portal bootstrap can add origin-specific cookies to the
			// shared jar. Cache them best-effort after the operation succeeds.
			_ = persistWebSessionFn(session)

			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderDeveloperBundleIDCapabilityEnableTable(result) },
				func() error { return renderDeveloperBundleIDCapabilityEnableMarkdown(result) },
			)
		},
	}
}

// WebBundleIDCapabilitiesSyncAppClipCommand syncs one App Clip capability relationship.
func WebBundleIDCapabilitiesSyncAppClipCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web bundle-ids capabilities sync-app-clip", flag.ExitOnError)

	bundleID := fs.String("bundle-id", "", "Opaque App Clip Bundle ID resource ID")
	parentBundleID := fs.String("parent-bundle-id", "", "Opaque parent app Bundle ID resource ID")
	capability := fs.String("capability", "", "Capability ID (for example: PUSH_NOTIFICATIONS)")
	settingsJSON := fs.String("settings-json", "", "Optional JSON array of capability settings")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "sync-app-clip",
		ShortUsage: "asc web bundle-ids capabilities sync-app-clip --bundle-id BUNDLE_ID --parent-bundle-id PARENT_BUNDLE_ID --capability CAPABILITY [flags]",
		ShortHelp:  "Sync an App Clip capability with parentBundleId.",
		LongHelp: `WEB SESSION WORKFLOWS

Patch an App Clip Bundle ID capability through Apple's Bundle ID update
payload and include the parentBundleId relationship required for App Clip
targets. This mirrors the App Store Connect web-session shape used for App Clip
Bundle IDs, not the public API-key capability endpoint.

Examples:
  asc web bundle-ids capabilities sync-app-clip --bundle-id "CLIP_BUNDLE_ID" --parent-bundle-id "PARENT_BUNDLE_ID" --capability "PUSH_NOTIFICATIONS"
  asc web bundle-ids capabilities sync-app-clip --bundle-id "CLIP_BUNDLE_ID" --parent-bundle-id "PARENT_BUNDLE_ID" --capability "PUSH_NOTIFICATIONS" --settings-json '[{"key":"PUSH_NOTIFICATION_FEATURES","options":[{"key":"PUSH_NOTIFICATION_FEATURE_BROADCAST","enabled":true}]}]'

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("web bundle-ids capabilities sync-app-clip does not accept positional arguments")
			}

			resolvedBundleID := strings.TrimSpace(*bundleID)
			resolvedParentBundleID := strings.TrimSpace(*parentBundleID)
			resolvedCapability := strings.ToUpper(strings.TrimSpace(*capability))
			if resolvedBundleID == "" {
				return shared.UsageError("--bundle-id is required")
			}
			if resolvedParentBundleID == "" {
				return shared.UsageError("--parent-bundle-id is required")
			}
			if resolvedCapability == "" {
				return shared.UsageError("--capability is required")
			}

			settings, err := parseWebBundleIDCapabilitySettings(*settingsJSON)
			if err != nil {
				return shared.UsageErrorf("--settings-json must be a JSON array of capability settings: %v", err)
			}

			session, err := resolveWebSessionForCommand(ctx, authFlags)
			if err != nil {
				return err
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			client := newWebClientFn(session)
			var result *webcore.AppClipBundleIDCapabilitySyncResult
			err = withWebSpinner("Syncing App Clip Bundle ID capability", func() error {
				var err error
				result, err = syncAppClipBundleIDCapabilityFn(requestCtx, client, webcore.AppClipBundleIDCapabilitySyncRequest{
					BundleID:         resolvedBundleID,
					ParentBundleID:   resolvedParentBundleID,
					Capability:       resolvedCapability,
					Enabled:          true,
					Settings:         settings,
					SettingsProvided: strings.TrimSpace(*settingsJSON) != "",
				})
				return err
			})
			if err != nil {
				return withWebAuthHint(err, "web bundle-ids capabilities sync-app-clip")
			}
			if result == nil {
				return fmt.Errorf("web bundle-ids capabilities sync-app-clip failed: missing sync result")
			}

			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderWebBundleIDCapabilitySyncTable(result) },
				func() error { return renderWebBundleIDCapabilitySyncMarkdown(result) },
			)
		},
	}
}

func parseWebBundleIDCapabilitySettings(value string) ([]webcore.BundleIDCapabilitySetting, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return []webcore.BundleIDCapabilitySetting{}, nil
	}
	var settings []webcore.BundleIDCapabilitySetting
	decoder := json.NewDecoder(strings.NewReader(value))
	if err := decoder.Decode(&settings); err != nil {
		return nil, err
	}
	if settings == nil {
		return nil, fmt.Errorf("expected JSON array, got null")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("multiple JSON values are not supported")
	}
	return settings, nil
}

func renderWebBundleIDCapabilitySyncTable(result *webcore.AppClipBundleIDCapabilitySyncResult) error {
	asc.RenderTable(
		[]string{"Bundle ID", "Parent Bundle ID", "Capability", "Enabled"},
		[][]string{{
			result.BundleID,
			result.ParentBundleID,
			result.Capability,
			fmt.Sprintf("%t", result.Enabled),
		}},
	)
	return nil
}

func renderWebBundleIDCapabilitySyncMarkdown(result *webcore.AppClipBundleIDCapabilitySyncResult) error {
	asc.RenderMarkdown(
		[]string{"Bundle ID", "Parent Bundle ID", "Capability", "Enabled"},
		[][]string{{
			result.BundleID,
			result.ParentBundleID,
			result.Capability,
			fmt.Sprintf("%t", result.Enabled),
		}},
	)
	return nil
}

func renderDeveloperBundleIDCapabilityEnableTable(result *webcore.DeveloperBundleIDCapabilityEnableResult) error {
	asc.RenderTable(
		[]string{"Bundle ID", "Capability", "Enabled", "Changed", "Status"},
		[][]string{{
			result.BundleID,
			result.Capability,
			fmt.Sprintf("%t", result.Enabled),
			fmt.Sprintf("%t", result.Changed),
			result.Status,
		}},
	)
	return nil
}

func renderDeveloperBundleIDCapabilityEnableMarkdown(result *webcore.DeveloperBundleIDCapabilityEnableResult) error {
	asc.RenderMarkdown(
		[]string{"Bundle ID", "Capability", "Enabled", "Changed", "Status"},
		[][]string{{
			result.BundleID,
			result.Capability,
			fmt.Sprintf("%t", result.Enabled),
			fmt.Sprintf("%t", result.Changed),
			result.Status,
		}},
	)
	return nil
}
