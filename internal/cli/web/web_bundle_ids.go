package web

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
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

var disableDeveloperBundleIDCapabilityFn = func(ctx context.Context, client *webcore.Client, req webcore.DeveloperBundleIDCapabilityDisableRequest) (*asc.DeveloperBundleIDCapabilityDisableResult, error) {
	return client.DisableDeveloperBundleIDCapability(ctx, req)
}

var listDeveloperBundleIDsFn = func(ctx context.Context, client *webcore.Client) (*webcore.DeveloperBundleIDsListResult, error) {
	return client.ListDeveloperBundleIDs(ctx)
}

var getDeveloperBundleIDFn = func(ctx context.Context, client *webcore.Client, bundleID string) (*webcore.DeveloperBundleIDGetResult, error) {
	return client.GetDeveloperBundleID(ctx, bundleID)
}

// WebBundleIDsCommand returns the Bundle ID command group.
func WebBundleIDsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web bundle-ids", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "bundle-ids",
		ShortUsage: "asc web bundle-ids <subcommand> [flags]",
		ShortHelp:  "Manage Bundle IDs via web-session endpoints.",
		LongHelp: `WEB SESSION WORKFLOWS

Manage Bundle ID operations that are only available through Apple web-session endpoints.

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebBundleIDsListCommand(),
			WebBundleIDsViewCommand(),
			WebBundleIDCapabilitiesCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebBundleIDsListCommand lists Bundle IDs visible to the selected Developer
// Portal team. It intentionally exposes the first web collection only; the
// endpoint's captured 1000-resource request is not a pagination guarantee.
func WebBundleIDsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web bundle-ids list", flag.ExitOnError)
	authFlags := bindWebSessionFlags(fs)
	portalFlags := bindDeveloperPortalFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc web bundle-ids list [flags]",
		ShortHelp:  "List Bundle IDs via a Developer Portal web session.",
		LongHelp: `List Bundle IDs via a Developer Portal web session.

WEB SESSION WORKFLOWS

List the iOS and Mac Bundle IDs visible to the selected Apple Developer team
through the Developer Portal web-session endpoint. The command requests the
captured 1000-resource collection and returns any links.next value Apple
provides in JSON; pagination is not exposed by this first read-only slice.

The ID column is Apple's opaque Bundle ID resource ID. Pass that value to
"asc web bundle-ids view --bundle-id" or the capability commands.

Examples:
  asc web bundle-ids list --output table
  asc web bundle-ids list --developer-team "TEAM_ID" --output json

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := validateDeveloperPortalFlags(portalFlags); err != nil {
				return err
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return withWebAuthHint(err, "web bundle-ids list")
			}

			var result *webcore.DeveloperBundleIDsListResult
			err = withWebSpinner("Loading Developer Portal Bundle IDs", func() error {
				var listErr error
				result, listErr = listDeveloperBundleIDsFn(requestCtx, newDeveloperPortalClient(session, portalFlags))
				return listErr
			})
			if err != nil {
				return withWebAuthHint(err, "web bundle-ids list")
			}
			if result == nil {
				return fmt.Errorf("web bundle-ids list failed: missing list result")
			}
			persistDeveloperPortalSession(session)

			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderDeveloperBundleIDsTable(result) },
				func() error { return renderDeveloperBundleIDsMarkdown(result) },
			)
		},
	}
}

// WebBundleIDsViewCommand reads one opaque Developer Portal Bundle ID resource
// and the capability graph returned by Apple's detail endpoint.
func WebBundleIDsViewCommand() *ffcli.Command {
	return newWebBundleIDsViewCommand(flag.ExitOnError)
}

func newWebBundleIDsViewCommand(errorHandling flag.ErrorHandling) *ffcli.Command {
	fs := flag.NewFlagSet("web bundle-ids view", errorHandling)
	bundleID := fs.String("bundle-id", "", "Opaque Developer Portal Bundle ID resource ID")
	authFlags := bindWebSessionFlags(fs)
	portalFlags := bindDeveloperPortalFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc web bundle-ids view --bundle-id BUNDLE_RESOURCE_ID [flags]",
		ShortHelp:  "Inspect one Bundle ID via a Developer Portal web session.",
		LongHelp: `Inspect one Bundle ID via a Developer Portal web session.

WEB SESSION WORKFLOWS

Inspect one opaque Bundle ID resource and its included Developer Portal
capability resources. Pass an ID returned by "asc web bundle-ids list". This is
a read-only request; it does not alter the Bundle ID or invalidate profiles.

Examples:
  asc web bundle-ids view --bundle-id "BUNDLE_RESOURCE_ID" --output table
  asc web bundle-ids view --bundle-id "BUNDLE_RESOURCE_ID" --output json

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedBundleID := strings.TrimSpace(*bundleID)
			if resolvedBundleID == "" {
				return shared.UsageError("--bundle-id is required")
			}
			if err := validateDeveloperPortalFlags(portalFlags); err != nil {
				return err
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return withWebAuthHint(err, "web bundle-ids view")
			}

			var result *webcore.DeveloperBundleIDGetResult
			err = withWebSpinner("Loading Developer Portal Bundle ID", func() error {
				var getErr error
				result, getErr = getDeveloperBundleIDFn(requestCtx, newDeveloperPortalClient(session, portalFlags), resolvedBundleID)
				return getErr
			})
			if err != nil {
				return withWebAuthHint(err, "web bundle-ids view")
			}
			if result == nil {
				return fmt.Errorf("web bundle-ids view failed: missing view result")
			}
			persistDeveloperPortalSession(session)

			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderDeveloperBundleIDTable(result) },
				func() error { return renderDeveloperBundleIDMarkdown(result) },
			)
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
			WebBundleIDCapabilitiesDisableCommand(),
			WebBundleIDCapabilitiesSyncAppClipCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebBundleIDCapabilitiesDisableCommand disables a Developer Portal-only
// Bundle ID capability while preserving all existing capability relationships.
func WebBundleIDCapabilitiesDisableCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web bundle-ids capabilities disable", flag.ExitOnError)

	bundleID := fs.String("bundle-id", "", "Opaque Developer Portal Bundle ID resource ID")
	capability := fs.String("capability", "", "Developer Portal capability ID (supported: PRIVATE_CLOUD_COMPUTE)")
	confirm := fs.Bool("confirm", false, "Confirm disabling this Bundle ID capability")
	authFlags := bindWebSessionFlags(fs)
	portalFlags := bindDeveloperPortalFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "disable",
		ShortUsage: "asc web bundle-ids capabilities disable --bundle-id BUNDLE_RESOURCE_ID --capability PRIVATE_CLOUD_COMPUTE --confirm [flags]",
		ShortHelp:  "Disable a Developer Portal-only Bundle ID capability.",
		LongHelp: `WEB SESSION WORKFLOWS

Disable a Bundle ID capability that is exposed by Apple Developer Portal but is
absent from the public App Store Connect capability enum.

The command loads capability metadata and the complete current Bundle ID
capability graph before saving. Existing settings and relationships are
preserved. A fresh read must prove the same Bundle ID and a complete included
capability graph, retain every pre-existing unrelated capability resource, and
either keep the same target resource IDs with every target disabled or show
that Apple removed all target resources before the command returns a success
receipt. Missing or partial target graphs remain unverified. If the capability
is already disabled, the command returns an already-disabled result without a
PATCH.

Currently supported capability IDs:
  PRIVATE_CLOUD_COMPUTE

Example:
  asc web bundle-ids capabilities disable --bundle-id "BUNDLE_RESOURCE_ID" --capability "PRIVATE_CLOUD_COMPUTE" --confirm

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
			if err := validateDeveloperPortalFlags(portalFlags); err != nil {
				return err
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return err
			}
			client := newDeveloperPortalClient(session, portalFlags)

			var result *asc.DeveloperBundleIDCapabilityDisableResult
			err = withWebSpinner("Disabling Developer Portal Bundle ID capability", func() error {
				var disableErr error
				result, disableErr = disableDeveloperBundleIDCapabilityFn(requestCtx, client, webcore.DeveloperBundleIDCapabilityDisableRequest{
					BundleID:   resolvedBundleID,
					Capability: resolvedCapability,
				})
				return disableErr
			})
			// Persist after every PATCH attempt so a later inspection uses any
			// refreshed cookies and the same selected Developer Portal team.
			persistDeveloperPortalSession(session)
			if err != nil {
				var unverified *webcore.DeveloperBundleIDCapabilityUnverifiedError
				if errors.As(err, &unverified) {
					fmt.Fprintf(os.Stderr, "Warning: disabling %s on Bundle ID %s may have changed the App ID, which can invalidate existing provisioning profiles. Inspect the Bundle ID before retrying.\n", resolvedCapability, resolvedBundleID)
				}
				return withWebAuthHint(err, "web bundle-ids capabilities disable")
			}
			if result == nil {
				return fmt.Errorf("web bundle-ids capabilities disable failed: missing disable result")
			}
			if result.Changed {
				fmt.Fprintf(os.Stderr, "Warning: disabling %s on Bundle ID %s changed the App ID, which invalidates existing provisioning profiles that contain it. Regenerate affected profiles before the next signed build.\n", result.Capability, result.BundleID)
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
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
	portalFlags := bindDeveloperPortalFlags(fs)
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
			if err := validateDeveloperPortalFlags(portalFlags); err != nil {
				return err
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return err
			}
			client := newDeveloperPortalClient(session, portalFlags)

			var result *webcore.DeveloperBundleIDCapabilityEnableResult
			err = withWebSpinner("Enabling Developer Portal Bundle ID capability", func() error {
				var enableErr error
				result, enableErr = enableDeveloperBundleIDCapabilityFn(requestCtx, client, webcore.DeveloperBundleIDCapabilityEnableRequest{
					BundleID:   resolvedBundleID,
					Capability: resolvedCapability,
				})
				return enableErr
			})
			// Persist after the PATCH attempt so a later retry without
			// --developer-team still targets the team that may have enabled
			// the capability even when Apple's response body is unreadable.
			persistDeveloperPortalSession(session)
			if err != nil {
				return withWebAuthHint(err, "web bundle-ids capabilities enable")
			}
			if result == nil {
				return fmt.Errorf("web bundle-ids capabilities enable failed: missing enable result")
			}

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

// webBundleIDSyncAppClipConfirmMigrationWarning explains the --confirm
// requirement to callers still using the pre-confirm invocation shape.
const webBundleIDSyncAppClipConfirmMigrationWarning = "Warning: web bundle-ids capabilities sync-app-clip now requires --confirm because syncing rewrites the App Clip Bundle ID capability graph and invalidates existing provisioning profiles; re-run with --confirm to acknowledge. No request was sent."

// WebBundleIDCapabilitiesSyncAppClipCommand syncs one App Clip capability relationship.
func WebBundleIDCapabilitiesSyncAppClipCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web bundle-ids capabilities sync-app-clip", flag.ExitOnError)

	bundleID := fs.String("bundle-id", "", "Opaque App Clip Bundle ID resource ID")
	parentBundleID := fs.String("parent-bundle-id", "", "Opaque parent app Bundle ID resource ID")
	capability := fs.String("capability", "", "Capability ID (for example: PUSH_NOTIFICATIONS)")
	settingsJSON := fs.String("settings-json", "", "Optional JSON array of capability settings")
	confirm := fs.Bool("confirm", false, "Confirm the sync; a changed App ID invalidates existing provisioning profiles")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "sync-app-clip",
		ShortUsage: "asc web bundle-ids capabilities sync-app-clip --bundle-id BUNDLE_ID --parent-bundle-id PARENT_BUNDLE_ID --capability CAPABILITY --confirm [flags]",
		ShortHelp:  "Sync an App Clip capability with parentBundleId.",
		LongHelp: `WEB SESSION WORKFLOWS

Patch an App Clip Bundle ID capability through Apple's Bundle ID update
payload and include the parentBundleId relationship required for App Clip
targets. This mirrors the App Store Connect web-session shape used for App Clip
Bundle IDs, not the public API-key capability endpoint.

The command reads the complete current Bundle ID capability graph first. If the
capability is already enabled with the requested parentBundleId (and the
requested settings, when --settings-json is passed), it returns an
already-synced receipt with changed=false and sends no PATCH. Otherwise it
writes the graph back with every unrelated capability and relationship
preserved and only writable capability attributes included.

--confirm is required. When the command changes the App ID, Apple invalidates
existing provisioning profiles that contain it. Regenerate affected profiles
before the next signed build.

Examples:
  asc web bundle-ids capabilities sync-app-clip --bundle-id "CLIP_BUNDLE_ID" --parent-bundle-id "PARENT_BUNDLE_ID" --capability "PUSH_NOTIFICATIONS" --confirm
  asc web bundle-ids capabilities sync-app-clip --bundle-id "CLIP_BUNDLE_ID" --parent-bundle-id "PARENT_BUNDLE_ID" --capability "PUSH_NOTIFICATIONS" --settings-json '[{"key":"PUSH_NOTIFICATION_FEATURES","options":[{"key":"PUSH_NOTIFICATION_FEATURE_BROADCAST","enabled":true}]}]' --confirm

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
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
			if !*confirm {
				fmt.Fprintln(os.Stderr, webBundleIDSyncAppClipConfirmMigrationWarning)
				return shared.UsageError("--confirm is required")
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return err
			}

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
			// Apple may rotate session cookies on the read or the write; cache
			// them so the next command reuses the refreshed session.
			if err := persistWebSessionFn(session); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to persist refreshed web session: %v\n", err)
			}
			if result.Changed {
				fmt.Fprintf(os.Stderr, "Warning: syncing %s on Bundle ID %s changed the App ID, which invalidates existing provisioning profiles that contain it. Regenerate affected profiles before the next signed build.\n", result.Capability, result.BundleID)
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

func webBundleIDCapabilitySyncHeaders() []string {
	return []string{"Bundle ID", "Parent Bundle ID", "Capability", "Enabled", "Changed", "Status"}
}

func webBundleIDCapabilitySyncRows(result *webcore.AppClipBundleIDCapabilitySyncResult) [][]string {
	return [][]string{{
		result.BundleID,
		result.ParentBundleID,
		result.Capability,
		fmt.Sprintf("%t", result.Enabled),
		fmt.Sprintf("%t", result.Changed),
		result.Status,
	}}
}

func renderWebBundleIDCapabilitySyncTable(result *webcore.AppClipBundleIDCapabilitySyncResult) error {
	asc.RenderTable(webBundleIDCapabilitySyncHeaders(), webBundleIDCapabilitySyncRows(result))
	return nil
}

func renderWebBundleIDCapabilitySyncMarkdown(result *webcore.AppClipBundleIDCapabilitySyncResult) error {
	asc.RenderMarkdown(webBundleIDCapabilitySyncHeaders(), webBundleIDCapabilitySyncRows(result))
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

func developerBundleIDHeaders() []string {
	return []string{
		"ID",
		"Name",
		"Identifier",
		"Platform",
		"Bundle Type",
		"Wildcard",
		"Seed ID",
		"Entitlement Group",
		"Platform Name",
		"Created",
		"Modified",
	}
}

func developerBundleIDRows(resources []webcore.DeveloperBundleID) [][]string {
	rows := make([][]string, 0, len(resources))
	for _, resource := range resources {
		rows = append(rows, []string{
			shared.OrNA(resource.ID),
			shared.OrNA(developerBundleIDStringAttribute(resource.Attributes, "name")),
			shared.OrNA(developerBundleIDStringAttribute(resource.Attributes, "identifier")),
			shared.OrNA(developerBundleIDStringAttribute(resource.Attributes, "platform")),
			shared.OrNA(developerBundleIDStringAttribute(resource.Attributes, "bundleType")),
			shared.OrNA(developerBundleIDBoolAttribute(resource.Attributes, "wildcard")),
			shared.OrNA(developerBundleIDStringAttribute(resource.Attributes, "seedId")),
			shared.OrNA(developerBundleIDStringAttribute(resource.Attributes, "entitlementGroupName")),
			shared.OrNA(developerBundleIDStringAttribute(resource.Attributes, "platformName")),
			shared.OrNA(developerBundleIDStringAttribute(resource.Attributes, "dateCreated")),
			shared.OrNA(developerBundleIDStringAttribute(resource.Attributes, "dateModified")),
		})
	}
	return rows
}

func developerBundleIDDetailHeaders() []string {
	return []string{
		"ID",
		"Name",
		"Identifier",
		"Platform",
		"Seed ID",
		"Wildcard",
		"Delete",
		"Edit",
	}
}

func developerBundleIDDetailRows(resource webcore.DeveloperBundleID) [][]string {
	return [][]string{{
		shared.OrNA(resource.ID),
		shared.OrNA(developerBundleIDStringAttribute(resource.Attributes, "name")),
		shared.OrNA(developerBundleIDStringAttribute(resource.Attributes, "identifier")),
		shared.OrNA(developerBundleIDStringAttribute(resource.Attributes, "platform")),
		shared.OrNA(developerBundleIDStringAttribute(resource.Attributes, "seedId")),
		shared.OrNA(developerBundleIDBoolAttribute(resource.Attributes, "wildcard")),
		shared.OrNA(developerBundleIDBoolAttribute(resource.Attributes, "~permissions.delete")),
		shared.OrNA(developerBundleIDBoolAttribute(resource.Attributes, "~permissions.edit")),
	}}
}

func renderDeveloperBundleIDsTable(result *webcore.DeveloperBundleIDsListResult) error {
	if result == nil {
		asc.RenderTable(developerBundleIDHeaders(), nil)
		return nil
	}
	asc.RenderTable(developerBundleIDHeaders(), developerBundleIDRows(result.Data))
	return nil
}

func renderDeveloperBundleIDsMarkdown(result *webcore.DeveloperBundleIDsListResult) error {
	if result == nil {
		asc.RenderMarkdown(developerBundleIDHeaders(), nil)
		return nil
	}
	asc.RenderMarkdown(developerBundleIDHeaders(), developerBundleIDRows(result.Data))
	return nil
}

func renderDeveloperBundleIDTable(result *webcore.DeveloperBundleIDGetResult) error {
	if result == nil {
		asc.RenderTable(developerBundleIDDetailHeaders(), nil)
		return nil
	}
	warnDeveloperBundleIDIncludedOutput(result)
	asc.RenderTable(developerBundleIDDetailHeaders(), developerBundleIDDetailRows(result.Data))
	return nil
}

func renderDeveloperBundleIDMarkdown(result *webcore.DeveloperBundleIDGetResult) error {
	if result == nil {
		asc.RenderMarkdown(developerBundleIDDetailHeaders(), nil)
		return nil
	}
	warnDeveloperBundleIDIncludedOutput(result)
	asc.RenderMarkdown(developerBundleIDDetailHeaders(), developerBundleIDDetailRows(result.Data))
	return nil
}

const developerBundleIDIncludedOutputWarning = "Warning: table or Markdown output omits included Bundle ID resources; use --output json to inspect the complete capability graph."

func warnDeveloperBundleIDIncludedOutput(result *webcore.DeveloperBundleIDGetResult) {
	if result == nil || len(result.Included) == 0 {
		return
	}
	fmt.Fprintln(os.Stderr, developerBundleIDIncludedOutputWarning)
}

func developerBundleIDStringAttribute(attributes map[string]any, key string) string {
	if attributes == nil {
		return ""
	}
	value, ok := attributes[key]
	if !ok || value == nil {
		return ""
	}
	if stringValue, ok := value.(string); ok {
		return strings.TrimSpace(stringValue)
	}
	if boolValue, ok := value.(bool); ok {
		return strconv.FormatBool(boolValue)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func developerBundleIDBoolAttribute(attributes map[string]any, key string) string {
	if attributes == nil {
		return ""
	}
	value, ok := attributes[key]
	if !ok || value == nil {
		return ""
	}
	if boolValue, ok := value.(bool); ok {
		return strconv.FormatBool(boolValue)
	}
	return developerBundleIDStringAttribute(attributes, key)
}
