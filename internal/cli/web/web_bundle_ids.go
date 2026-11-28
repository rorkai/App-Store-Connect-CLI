package web

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
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

// newWebBundleIDsCommand returns the Bundle ID command group.
func newWebBundleIDsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web bundle-ids", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "bundle-ids",
		ShortUsage: "asc web bundle-ids <subcommand> [flags]",
		ShortHelp:  "Manage Bundle IDs via web-session endpoints.",
		LongHelp: `WEB SESSION WORKFLOWS

Manage Bundle ID operations that are only available through Apple web-session endpoints.

`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newWebBundleIDsListCommand(d),
			newWebBundleIDsViewCommand(d),
			newWebBundleIDCapabilitiesCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newWebBundleIDsListCommand lists Bundle IDs visible to the selected Developer
// Portal team. It intentionally exposes the first web collection only; the
// endpoint's captured 1000-resource request is not a pagination guarantee.
func newWebBundleIDsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web bundle-ids list", flag.ExitOnError)
	authFlags := bindWebSessionFlags(fs)
	portalFlags := bindDeveloperPortalFlags(fs)
	output := d.BindOutputFlags(fs)

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
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := validateDeveloperPortalFlagsWith(d, portalFlags); err != nil {
				return err
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return d.UsageError(err.Error())
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommandWith(ctx, d, authFlags)
			defer cancel()
			if err != nil {
				return withWebAuthHint(err, "web bundle-ids list")
			}

			var result *webcore.DeveloperBundleIDsListResult
			err = withWebSpinnerWith(d, "Loading Developer Portal Bundle IDs", func() error {
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
			persistDeveloperPortalSessionWith(d, session)

			return d.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderDeveloperBundleIDsTable(d, result) },
				func() error { return renderDeveloperBundleIDsMarkdown(d, result) },
			)
		},
	}
}

// newWebBundleIDsViewCommand reads one opaque Developer Portal Bundle ID resource
// and the capability graph returned by Apple's detail endpoint.
func newWebBundleIDsViewCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web bundle-ids view", flag.ExitOnError)
	bundleID := fs.String("bundle-id", "", "Opaque Developer Portal Bundle ID resource ID")
	authFlags := bindWebSessionFlags(fs)
	portalFlags := bindDeveloperPortalFlags(fs)
	output := d.BindOutputFlags(fs)

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
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedBundleID := strings.TrimSpace(*bundleID)
			if resolvedBundleID == "" {
				return d.UsageError("--bundle-id is required")
			}
			if err := validateDeveloperPortalFlagsWith(d, portalFlags); err != nil {
				return err
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return d.UsageError(err.Error())
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommandWith(ctx, d, authFlags)
			defer cancel()
			if err != nil {
				return withWebAuthHint(err, "web bundle-ids view")
			}

			var result *webcore.DeveloperBundleIDGetResult
			err = withWebSpinnerWith(d, "Loading Developer Portal Bundle ID", func() error {
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
			persistDeveloperPortalSessionWith(d, session)

			return d.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderDeveloperBundleIDTable(d, result) },
				func() error { return renderDeveloperBundleIDMarkdown(d, result) },
			)
		},
	}
}

// newWebBundleIDCapabilitiesCommand returns the Bundle ID capabilities group.
func newWebBundleIDCapabilitiesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web bundle-ids capabilities", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "capabilities",
		ShortUsage: "asc web bundle-ids capabilities <subcommand> [flags]",
		ShortHelp:  "Manage Bundle ID capabilities via web sessions.",
		LongHelp: `WEB SESSION WORKFLOWS

Manage Bundle ID capabilities through Apple's App Store Connect and Developer
Portal web-session endpoints.

`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newWebBundleIDCapabilitiesListCommand(d),
			newWebBundleIDCapabilitiesEnableCommand(d),
			newWebBundleIDCapabilitiesDisableCommand(d),
			newWebBundleIDCapabilitiesSyncAppClipCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newWebBundleIDCapabilitiesDisableCommand disables a Developer Portal-only
// Bundle ID capability while preserving all existing capability relationships.
func newWebBundleIDCapabilitiesDisableCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web bundle-ids capabilities disable", flag.ExitOnError)

	bundleID := fs.String("bundle-id", "", "Opaque Developer Portal Bundle ID resource ID")
	capability := fs.String("capability", "", "Capability ID returned by web bundle-ids capabilities list")
	confirm := fs.Bool("confirm", false, "Confirm disabling this Bundle ID capability")
	authFlags := bindWebSessionFlags(fs)
	portalFlags := bindDeveloperPortalFlags(fs)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "disable",
		ShortUsage: "asc web bundle-ids capabilities disable --bundle-id BUNDLE_RESOURCE_ID --capability CAPABILITY_ID --confirm [flags]",
		ShortHelp:  "Disable a Bundle ID capability through Developer Portal.",
		LongHelp: `WEB SESSION WORKFLOWS

Disable a Bundle ID capability that is available through Apple Developer Portal.

The command loads capability metadata and the complete current Bundle ID
capability graph before saving. Existing settings and relationships are
preserved. A fresh read must prove the same Bundle ID and a complete included
capability graph, retain every pre-existing unrelated capability resource, and
either keep the same target resource IDs with every target disabled or show
that Apple removed all target resources before the command returns a success
receipt. Missing or partial target graphs remain unverified. If the capability
is already disabled, the command returns an already-disabled result without a
PATCH.

Use "asc web bundle-ids capabilities list --bundle-id BUNDLE_RESOURCE_ID" to
inspect capability IDs available to the selected team and App ID. Apple approval
and account permissions still apply.

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
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedBundleID := strings.TrimSpace(*bundleID)
			resolvedCapability := strings.ToUpper(strings.TrimSpace(*capability))
			switch {
			case resolvedBundleID == "":
				return d.UsageError("--bundle-id is required")
			case resolvedCapability == "":
				return d.UsageError("--capability is required")
			case !*confirm:
				return d.RequireConfirm(*confirm)
			}
			if err := validateDeveloperPortalFlagsWith(d, portalFlags); err != nil {
				return err
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return d.UsageError(err.Error())
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommandWith(ctx, d, authFlags)
			defer cancel()
			if err != nil {
				return err
			}
			client := newDeveloperPortalClient(session, portalFlags)

			var result *asc.DeveloperBundleIDCapabilityDisableResult
			err = withWebSpinnerWith(d, "Disabling Developer Portal Bundle ID capability", func() error {
				var disableErr error
				result, disableErr = disableDeveloperBundleIDCapabilityFn(requestCtx, client, webcore.DeveloperBundleIDCapabilityDisableRequest{
					BundleID:   resolvedBundleID,
					Capability: resolvedCapability,
				})
				return disableErr
			})
			// Persist after every PATCH attempt so a later inspection uses any
			// refreshed cookies and the same selected Developer Portal team.
			persistDeveloperPortalSessionWith(d, session)
			if err != nil {
				var unverified *webcore.DeveloperBundleIDCapabilityUnverifiedError
				if errors.As(err, &unverified) {
					fmt.Fprintf(d.IO.ErrOut, "Warning: disabling %s on Bundle ID %s may have changed the App ID, which can invalidate existing provisioning profiles. Inspect the Bundle ID before retrying.\n", resolvedCapability, resolvedBundleID)
				}
				return withWebAuthHint(err, "web bundle-ids capabilities disable")
			}
			if result == nil {
				return fmt.Errorf("web bundle-ids capabilities disable failed: missing disable result")
			}
			if result.Changed {
				fmt.Fprintf(d.IO.ErrOut, "Warning: disabling %s on Bundle ID %s changed the App ID, which invalidates existing provisioning profiles that contain it. Regenerate affected profiles before the next signed build.\n", result.Capability, result.BundleID)
			}

			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// newWebBundleIDCapabilitiesEnableCommand enables a Developer Portal-only Bundle
// ID capability while preserving all existing capability relationships.
func newWebBundleIDCapabilitiesEnableCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web bundle-ids capabilities enable", flag.ExitOnError)

	bundleID := fs.String("bundle-id", "", "Opaque Developer Portal Bundle ID resource ID")
	capability := fs.String("capability", "", "Capability ID returned by web bundle-ids capabilities list")
	associationsJSON := fs.String("associations-json", "", "Optional JSON object mapping metadata association names to resource ID arrays")
	settingsJSON := fs.String("settings-json", "", "Optional JSON array of capability settings; omitted settings preserve current values")
	confirm := fs.Bool("confirm", false, "Confirm enabling this Bundle ID capability")
	authFlags := bindWebSessionFlags(fs)
	portalFlags := bindDeveloperPortalFlags(fs)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "enable",
		ShortUsage: "asc web bundle-ids capabilities enable --bundle-id BUNDLE_RESOURCE_ID --capability CAPABILITY_ID --confirm [flags]",
		ShortHelp:  "Enable a Bundle ID capability through Developer Portal.",
		LongHelp: `WEB SESSION WORKFLOWS

Enable a Bundle ID capability that is available through Apple Developer Portal.

The command loads Developer Portal capability metadata and the complete current
Bundle ID capability graph before saving. Existing settings and relationships
are preserved. If the requested capability is already enabled, the command
returns an already-enabled result when the supplied settings and associations
already match, without sending a PATCH. Required settings must be supplied with
--settings-json and are checked against Apple's metadata. Use --associations-json
for metadata-declared linked identifiers. Assignments with dedicated App Groups,
Apple Pay, or iCloud workflows use those commands or Developer Portal.
Capabilities that require an opt-in must be configured in Developer Portal first.

Use "asc web bundle-ids capabilities list --bundle-id BUNDLE_RESOURCE_ID" to
inspect capability IDs available to the selected team and App ID. Apple approval
and account permissions still apply.

Examples:
  asc web bundle-ids capabilities enable --bundle-id "BUNDLE_RESOURCE_ID" --capability "CARPLAY_PLAYABLE_CONTENT" --confirm
  asc web bundle-ids capabilities enable --bundle-id "BUNDLE_RESOURCE_ID" --capability "PRIVATE_CLOUD_COMPUTE" --confirm

Authentication:
  This command needs Developer Portal cookies and CSRF headers derived from the
  user-owned Apple web session. If a cached App Store Connect-only session cannot
  be promoted, clear only its cached session and log in again with the same binary:
  asc web auth logout --apple-id "user@example.com"
  asc web auth login --apple-id "user@example.com"

`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedBundleID := strings.TrimSpace(*bundleID)
			resolvedCapability := strings.ToUpper(strings.TrimSpace(*capability))
			switch {
			case resolvedBundleID == "":
				return d.UsageError("--bundle-id is required")
			case resolvedCapability == "":
				return d.UsageError("--capability is required")
			case !*confirm:
				return d.RequireConfirm(*confirm)
			}
			provided := make(map[string]bool)
			fs.Visit(func(value *flag.Flag) { provided[value.Name] = true })
			var settings []webcore.BundleIDCapabilitySetting
			if provided["settings-json"] {
				if strings.TrimSpace(*settingsJSON) == "" {
					return d.UsageError("--settings-json must be a JSON array, got empty input")
				}
				var err error
				settings, err = parseWebBundleIDCapabilitySettings(*settingsJSON)
				if err != nil {
					return d.UsageErrorf("--settings-json must be a JSON array: %v", err)
				}
			}
			var associations map[string][]string
			if provided["associations-json"] {
				var err error
				associations, err = parseWebBundleIDCapabilityAssociations(*associationsJSON)
				if err != nil {
					return d.UsageErrorf("--associations-json must be a JSON object of resource ID arrays: %v", err)
				}
			}
			if err := validateDeveloperPortalFlagsWith(d, portalFlags); err != nil {
				return err
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return d.UsageError(err.Error())
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommandWith(ctx, d, authFlags)
			defer cancel()
			if err != nil {
				return err
			}
			client := newDeveloperPortalClient(session, portalFlags)

			var result *webcore.DeveloperBundleIDCapabilityEnableResult
			err = withWebSpinnerWith(d, "Enabling Developer Portal Bundle ID capability", func() error {
				var enableErr error
				result, enableErr = enableDeveloperBundleIDCapabilityFn(requestCtx, client, webcore.DeveloperBundleIDCapabilityEnableRequest{
					BundleID:     resolvedBundleID,
					Capability:   resolvedCapability,
					Settings:     settings,
					Associations: associations,
				})
				return enableErr
			})
			// Persist after the PATCH attempt so a later retry without
			// --developer-team still targets the team that may have enabled
			// the capability even when Apple's response body is unreadable.
			persistDeveloperPortalSessionWith(d, session)
			if err != nil {
				var unverified *webcore.DeveloperBundleIDCapabilityUnverifiedError
				if errors.As(err, &unverified) {
					fmt.Fprintf(d.IO.ErrOut, "Warning: enabling %s on Bundle ID %s may have changed the App ID. Inspect the Bundle ID before retrying.\n", resolvedCapability, resolvedBundleID)
				}
				return withWebAuthHint(err, "web bundle-ids capabilities enable")
			}
			if result == nil {
				return fmt.Errorf("web bundle-ids capabilities enable failed: missing enable result")
			}

			return d.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderDeveloperBundleIDCapabilityEnableTable(d, result) },
				func() error { return renderDeveloperBundleIDCapabilityEnableMarkdown(d, result) },
			)
		},
	}
}

// webBundleIDSyncAppClipConfirmMigrationWarning explains the --confirm
// requirement to callers still using the pre-confirm invocation shape.
const webBundleIDSyncAppClipConfirmMigrationWarning = "Warning: web bundle-ids capabilities sync-app-clip now requires --confirm because syncing rewrites the App Clip Bundle ID capability graph and invalidates existing provisioning profiles; re-run with --confirm to acknowledge. No request was sent."

// newWebBundleIDCapabilitiesSyncAppClipCommand syncs one App Clip capability relationship.
func newWebBundleIDCapabilitiesSyncAppClipCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web bundle-ids capabilities sync-app-clip", flag.ExitOnError)

	bundleID := fs.String("bundle-id", "", "Opaque App Clip Bundle ID resource ID")
	parentBundleID := fs.String("parent-bundle-id", "", "Opaque parent app Bundle ID resource ID")
	capability := fs.String("capability", "", "Capability ID (for example: PUSH_NOTIFICATIONS)")
	settingsJSON := fs.String("settings-json", "", "Optional JSON array of capability settings")
	confirm := fs.Bool("confirm", false, "Confirm the sync; a changed App ID invalidates existing provisioning profiles")
	authFlags := bindWebSessionFlags(fs)
	output := d.BindOutputFlags(fs)

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
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedBundleID := strings.TrimSpace(*bundleID)
			resolvedParentBundleID := strings.TrimSpace(*parentBundleID)
			resolvedCapability := strings.ToUpper(strings.TrimSpace(*capability))
			if resolvedBundleID == "" {
				return d.UsageError("--bundle-id is required")
			}
			if resolvedParentBundleID == "" {
				return d.UsageError("--parent-bundle-id is required")
			}
			if resolvedCapability == "" {
				return d.UsageError("--capability is required")
			}

			settings, err := parseWebBundleIDCapabilitySettings(*settingsJSON)
			if err != nil {
				return d.UsageErrorf("--settings-json must be a JSON array of capability settings: %v", err)
			}
			if !*confirm {
				fmt.Fprintln(d.IO.ErrOut, webBundleIDSyncAppClipConfirmMigrationWarning)
				return d.RequireConfirm(*confirm)
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommandWith(ctx, d, authFlags)
			defer cancel()
			if err != nil {
				return err
			}

			client := newWebClientFn(session)
			var result *webcore.AppClipBundleIDCapabilitySyncResult
			err = withWebSpinnerWith(d, "Syncing App Clip Bundle ID capability", func() error {
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
				fmt.Fprintf(d.IO.ErrOut, "Warning: failed to persist refreshed web session: %v\n", err)
			}
			if result.Changed {
				fmt.Fprintf(d.IO.ErrOut, "Warning: syncing %s on Bundle ID %s changed the App ID, which invalidates existing provisioning profiles that contain it. Regenerate affected profiles before the next signed build.\n", result.Capability, result.BundleID)
			}

			return d.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderWebBundleIDCapabilitySyncTable(d, result) },
				func() error { return renderWebBundleIDCapabilitySyncMarkdown(d, result) },
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
	decoder.DisallowUnknownFields()
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

func renderWebBundleIDCapabilitySyncTable(d *shared.Deps, result *webcore.AppClipBundleIDCapabilitySyncResult) error {
	asc.RenderTableTo(d.IO.Out, webBundleIDCapabilitySyncHeaders(), webBundleIDCapabilitySyncRows(result))
	return nil
}

func renderWebBundleIDCapabilitySyncMarkdown(d *shared.Deps, result *webcore.AppClipBundleIDCapabilitySyncResult) error {
	asc.RenderMarkdownTo(d.IO.Out, webBundleIDCapabilitySyncHeaders(), webBundleIDCapabilitySyncRows(result))
	return nil
}

func renderDeveloperBundleIDCapabilityEnableTable(d *shared.Deps, result *webcore.DeveloperBundleIDCapabilityEnableResult) error {
	asc.RenderTableTo(d.IO.Out,
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

func renderDeveloperBundleIDCapabilityEnableMarkdown(d *shared.Deps, result *webcore.DeveloperBundleIDCapabilityEnableResult) error {
	asc.RenderMarkdownTo(d.IO.Out,
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

func renderDeveloperBundleIDsTable(d *shared.Deps, result *webcore.DeveloperBundleIDsListResult) error {
	if result == nil {
		asc.RenderTableTo(d.IO.Out, developerBundleIDHeaders(), nil)
		return nil
	}
	asc.RenderTableTo(d.IO.Out, developerBundleIDHeaders(), developerBundleIDRows(result.Data))
	return nil
}

func renderDeveloperBundleIDsMarkdown(d *shared.Deps, result *webcore.DeveloperBundleIDsListResult) error {
	if result == nil {
		asc.RenderMarkdownTo(d.IO.Out, developerBundleIDHeaders(), nil)
		return nil
	}
	asc.RenderMarkdownTo(d.IO.Out, developerBundleIDHeaders(), developerBundleIDRows(result.Data))
	return nil
}

func renderDeveloperBundleIDTable(d *shared.Deps, result *webcore.DeveloperBundleIDGetResult) error {
	if result == nil {
		asc.RenderTableTo(d.IO.Out, developerBundleIDDetailHeaders(), nil)
		return nil
	}
	warnDeveloperBundleIDIncludedOutput(d, result)
	asc.RenderTableTo(d.IO.Out, developerBundleIDDetailHeaders(), developerBundleIDDetailRows(result.Data))
	return nil
}

func renderDeveloperBundleIDMarkdown(d *shared.Deps, result *webcore.DeveloperBundleIDGetResult) error {
	if result == nil {
		asc.RenderMarkdownTo(d.IO.Out, developerBundleIDDetailHeaders(), nil)
		return nil
	}
	warnDeveloperBundleIDIncludedOutput(d, result)
	asc.RenderMarkdownTo(d.IO.Out, developerBundleIDDetailHeaders(), developerBundleIDDetailRows(result.Data))
	return nil
}

const developerBundleIDIncludedOutputWarning = "Warning: table or Markdown output omits included Bundle ID resources; use --output json to inspect the complete capability graph."

func warnDeveloperBundleIDIncludedOutput(d *shared.Deps, result *webcore.DeveloperBundleIDGetResult) {
	if result == nil || len(result.Included) == 0 {
		return
	}
	fmt.Fprintln(d.IO.ErrOut, developerBundleIDIncludedOutputWarning)
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

func parseWebBundleIDCapabilityAssociations(value string) (map[string][]string, error) {
	var associations map[string][]string
	decoder := json.NewDecoder(strings.NewReader(value))
	if err := decoder.Decode(&associations); err != nil {
		return nil, err
	}
	if associations == nil {
		return nil, fmt.Errorf("expected JSON object, got null")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("multiple JSON values are not supported")
	}
	for name, ids := range associations {
		if strings.TrimSpace(name) == "" || ids == nil {
			return nil, fmt.Errorf("association %q requires a resource ID array", name)
		}
	}
	return associations, nil
}
