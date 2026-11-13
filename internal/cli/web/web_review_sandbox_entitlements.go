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

// WebReviewSandboxEntitlementsCommand returns the command group for the App
// Sandbox Information section of a macOS version's App Review details.
func WebReviewSandboxEntitlementsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web review sandbox-entitlements", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "sandbox-entitlements",
		ShortUsage: "asc web review sandbox-entitlements <subcommand> [flags]",
		ShortHelp:  "Manage macOS App Sandbox Information for App Review.",
		LongHelp: `WEB SESSION WORKFLOWS

Manage the App Sandbox Information section of a macOS App Store version's App
Review details. Each row pairs an entitlement key with the usage information
that explains it to App Review. The public App Store Connect API does not
expose this section.

Every version-scoped command takes --version-id, accepts only MAC_OS versions,
and requires the version to already have App Review details.

Examples:
  asc web review sandbox-entitlements catalog
  asc web review sandbox-entitlements list --version-id "VERSION_ID"
  asc web review sandbox-entitlements set --version-id "VERSION_ID" --entitlement "com.apple.security.network.server" --justification "Accepts connections from paired devices on the local network." --confirm
  asc web review sandbox-entitlements delete --version-id "VERSION_ID" --entitlement "com.apple.security.network.server" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebReviewSandboxEntitlementsCatalogCommand(),
			WebReviewSandboxEntitlementsListCommand(),
			WebReviewSandboxEntitlementsSetCommand(),
			WebReviewSandboxEntitlementsDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebReviewSandboxEntitlementsCatalogCommand lists the entitlement keys App
// Store Connect accepts in the App Sandbox Information section.
func WebReviewSandboxEntitlementsCatalogCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web review sandbox-entitlements catalog", flag.ExitOnError)
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "catalog",
		ShortUsage: "asc web review sandbox-entitlements catalog [flags]",
		ShortHelp:  "List the entitlement keys App Sandbox Information accepts.",
		LongHelp: `WEB SESSION WORKFLOWS

List the entitlement keys that App Store Connect accepts in the App Sandbox
Information section, with Apple's optional attribute. JSON output preserves
Apple's raw JSON:API envelope.

Example:
  asc web review sandbox-entitlements catalog --output json`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := validateWebReviewSandboxCommonInput(output); err != nil {
				return err
			}
			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return withWebAuthHint(err, "web review sandbox-entitlements catalog")
			}
			client := newWebClientFn(session)

			var result *webcore.AppSandboxEntitlementsResult
			err = withWebSpinner("Loading App Sandbox entitlement keys", func() error {
				result, err = client.ListAppSandboxEntitlements(requestCtx)
				return err
			})
			if err != nil {
				return withWebAuthHint(err, "web review sandbox-entitlements catalog")
			}
			rows := make([][]string, 0, len(result.Data))
			for _, entitlement := range result.Data {
				optional := ""
				if entitlement.Optional != nil {
					optional = fmt.Sprintf("%t", *entitlement.Optional)
				}
				rows = append(rows, []string{entitlement.ID, optional})
			}
			return shared.PrintOutputRows(result, *output.Output, *output.Pretty, []string{"Entitlement", "Optional"}, rows)
		},
	}
}

// WebReviewSandboxEntitlementsListCommand lists the App Sandbox Information
// rows of one macOS version.
func WebReviewSandboxEntitlementsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web review sandbox-entitlements list", flag.ExitOnError)
	versionID := shared.BindResourceIDFlag(fs, "version-id", "appStoreVersions", "macOS App Store version ID (required)")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: `asc web review sandbox-entitlements list --version-id "VERSION_ID" [flags]`,
		ShortHelp:  "List the App Sandbox Information rows of a macOS version.",
		LongHelp: `WEB SESSION WORKFLOWS

List every App Sandbox Information row of one macOS version: the entitlement
key, its usage information, and the row ID. JSON output preserves Apple's raw
JSON:API envelope.

Example:
  asc web review sandbox-entitlements list --version-id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := validateWebReviewSandboxCommonInput(output); err != nil {
				return err
			}
			version := strings.TrimSpace(*versionID)
			if version == "" {
				return shared.UsageError("--version-id is required")
			}
			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return withWebAuthHint(err, "web review sandbox-entitlements list")
			}
			client := newWebClientFn(session)

			var result *webcore.AppSandboxEntitlementUsagesResult
			err = withWebSpinner("Loading App Sandbox Information", func() error {
				target, err := resolveWebReviewSandboxTarget(requestCtx, client, version)
				if err != nil {
					return err
				}
				result, err = client.ListAppSandboxEntitlementUsages(requestCtx, target.ReviewDetailID)
				return err
			})
			if err != nil {
				return withWebAuthHint(err, "web review sandbox-entitlements list")
			}
			rows := make([][]string, 0, len(result.Data))
			for _, usage := range result.Data {
				rows = append(rows, []string{usage.EntitlementID, usage.Justification, usage.ID})
			}
			return shared.PrintOutputRows(result, *output.Output, *output.Pretty, []string{"Entitlement", "Justification", "Usage ID"}, rows)
		},
	}
}

// WebReviewSandboxEntitlementsSetCommand creates or updates the usage
// information of one entitlement and verifies it with a fresh read.
func WebReviewSandboxEntitlementsSetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web review sandbox-entitlements set", flag.ExitOnError)
	versionID := shared.BindResourceIDFlag(fs, "version-id", "appStoreVersions", "macOS App Store version ID (required)")
	entitlement := fs.String("entitlement", "", "Entitlement key, for example com.apple.security.network.server (required)")
	justification := fs.String("justification", "", "Usage information that explains the entitlement to App Review (required)")
	confirm := fs.Bool("confirm", false, "Confirm writing the App Sandbox Information row (required)")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: `asc web review sandbox-entitlements set --version-id "VERSION_ID" --entitlement KEY --justification TEXT --confirm [flags]`,
		ShortHelp:  "Create or update the usage information of one entitlement.",
		LongHelp: `WEB SESSION WORKFLOWS

Create or update the App Sandbox Information row of one entitlement on a macOS
version. The command reads the current rows first: it updates the existing row
of the entitlement, creates one when none exists, and writes nothing when the
usage information already matches. A new entitlement key must be in the
catalog listed by "asc web review sandbox-entitlements catalog". Surrounding
whitespace of --justification is trimmed.

The result is verified with a fresh read. Rerunning the command is safe: it
starts from the current rows again.

Example:
  asc web review sandbox-entitlements set --version-id "VERSION_ID" --entitlement "com.apple.security.network.client" --justification "Downloads the demo film when the person asks for it." --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := validateWebReviewSandboxCommonInput(output); err != nil {
				return err
			}
			version, key, err := requireWebReviewSandboxRowFlags(*versionID, *entitlement)
			if err != nil {
				return err
			}
			text := strings.TrimSpace(*justification)
			if text == "" {
				return shared.UsageError("--justification is required")
			}
			if !*confirm {
				return shared.UsageError("--confirm is required")
			}
			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return withWebAuthHint(err, "web review sandbox-entitlements set")
			}
			client := newWebClientFn(session)

			var receipt *asc.WebReviewSandboxEntitlementResult
			err = withWebSpinner("Setting App Sandbox Information", func() error {
				target, err := resolveWebReviewSandboxTarget(requestCtx, client, version)
				if err != nil {
					return err
				}
				before, err := client.ListAppSandboxEntitlementUsages(requestCtx, target.ReviewDetailID)
				if err != nil {
					return err
				}
				existing, found, err := findWebReviewSandboxUsage(before, key)
				if err != nil {
					return err
				}
				if found && existing.Justification == text {
					receipt = webReviewSandboxReceipt(target, key, existing.ID, text, false, "unchanged")
					return nil
				}

				usageID, status := existing.ID, "updated"
				if found {
					if _, err := client.UpdateAppSandboxEntitlementUsage(requestCtx, existing.ID, text); err != nil {
						return err
					}
				} else {
					if err := requireWebReviewSandboxCatalogKey(requestCtx, client, key); err != nil {
						return err
					}
					created, err := client.CreateAppSandboxEntitlementUsage(requestCtx, target.ReviewDetailID, key, text)
					if err != nil {
						return err
					}
					usageID, status = created.ID, "created"
				}

				after, err := client.ListAppSandboxEntitlementUsages(requestCtx, target.ReviewDetailID)
				if err != nil {
					return fmt.Errorf("could not verify the %s row: %w", key, err)
				}
				current, found, err := findWebReviewSandboxUsage(after, key)
				if err != nil {
					return err
				}
				if !found || current.ID != usageID || current.Justification != text {
					return fmt.Errorf("the %s row does not hold the requested usage information after the write; rerun the command to reconcile", key)
				}
				receipt = webReviewSandboxReceipt(target, key, usageID, text, true, status)
				return nil
			})
			if err != nil {
				return withWebAuthHint(err, "web review sandbox-entitlements set")
			}
			return shared.PrintOutput(receipt, *output.Output, *output.Pretty)
		},
	}
}

// WebReviewSandboxEntitlementsDeleteCommand removes the App Sandbox
// Information row of one entitlement and verifies its absence.
func WebReviewSandboxEntitlementsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web review sandbox-entitlements delete", flag.ExitOnError)
	versionID := shared.BindResourceIDFlag(fs, "version-id", "appStoreVersions", "macOS App Store version ID (required)")
	entitlement := fs.String("entitlement", "", "Entitlement key of the row to remove (required)")
	confirm := fs.Bool("confirm", false, "Confirm removing the App Sandbox Information row (required)")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: `asc web review sandbox-entitlements delete --version-id "VERSION_ID" --entitlement KEY --confirm [flags]`,
		ShortHelp:  "Remove the App Sandbox Information row of one entitlement.",
		LongHelp: `WEB SESSION WORKFLOWS

Remove the App Sandbox Information row of one entitlement from a macOS
version, then verify with a fresh read that it is gone. When the version has
no row for the entitlement, nothing is written and the receipt reports
"unchanged".

Example:
  asc web review sandbox-entitlements delete --version-id "VERSION_ID" --entitlement "com.apple.security.network.server" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := validateWebReviewSandboxCommonInput(output); err != nil {
				return err
			}
			version, key, err := requireWebReviewSandboxRowFlags(*versionID, *entitlement)
			if err != nil {
				return err
			}
			if !*confirm {
				return shared.UsageError("--confirm is required")
			}
			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return withWebAuthHint(err, "web review sandbox-entitlements delete")
			}
			client := newWebClientFn(session)

			var receipt *asc.WebReviewSandboxEntitlementResult
			err = withWebSpinner("Deleting App Sandbox Information", func() error {
				target, err := resolveWebReviewSandboxTarget(requestCtx, client, version)
				if err != nil {
					return err
				}
				before, err := client.ListAppSandboxEntitlementUsages(requestCtx, target.ReviewDetailID)
				if err != nil {
					return err
				}
				existing, found, err := findWebReviewSandboxUsage(before, key)
				if err != nil {
					return err
				}
				if !found {
					receipt = webReviewSandboxReceipt(target, key, "", "", false, "unchanged")
					return nil
				}
				if err := client.DeleteAppSandboxEntitlementUsage(requestCtx, existing.ID); err != nil {
					return err
				}
				after, err := client.ListAppSandboxEntitlementUsages(requestCtx, target.ReviewDetailID)
				if err != nil {
					return fmt.Errorf("could not verify the removal of the %s row: %w", key, err)
				}
				if _, stillFound, err := findWebReviewSandboxUsage(after, key); err != nil || stillFound {
					return fmt.Errorf("the %s row is still present after the delete; rerun the command to reconcile", key)
				}
				receipt = webReviewSandboxReceipt(target, key, existing.ID, "", true, "deleted")
				return nil
			})
			if err != nil {
				return withWebAuthHint(err, "web review sandbox-entitlements delete")
			}
			return shared.PrintOutput(receipt, *output.Output, *output.Pretty)
		},
	}
}

func validateWebReviewSandboxCommonInput(output shared.OutputFlags) error {
	if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
		return shared.UsageError(err.Error())
	}
	return nil
}

func requireWebReviewSandboxRowFlags(versionID, entitlement string) (string, string, error) {
	version := strings.TrimSpace(versionID)
	if version == "" {
		return "", "", shared.UsageError("--version-id is required")
	}
	key := strings.TrimSpace(entitlement)
	if key == "" {
		return "", "", shared.UsageError("--entitlement is required")
	}
	return version, key, nil
}

// resolveWebReviewSandboxTarget returns the review detail that holds the App
// Sandbox Information rows of a macOS version.
func resolveWebReviewSandboxTarget(ctx context.Context, client *webcore.Client, versionID string) (*webcore.AppStoreVersionReviewDetail, error) {
	target, err := client.GetAppStoreVersionReviewDetail(ctx, versionID)
	if err != nil {
		return nil, err
	}
	if target.Platform != string(asc.PlatformMacOS) {
		return nil, fmt.Errorf("version %q is on platform %q; App Sandbox Information applies only to %s versions", versionID, target.Platform, asc.PlatformMacOS)
	}
	if target.ReviewDetailID == "" {
		return nil, fmt.Errorf("version %q has no App Review details yet; create them first with asc review details-create --version-id %s --contact-first-name NAME --contact-phone PHONE", versionID, versionID)
	}
	return target, nil
}

func requireWebReviewSandboxCatalogKey(ctx context.Context, client *webcore.Client, key string) error {
	catalog, err := client.ListAppSandboxEntitlements(ctx)
	if err != nil {
		return err
	}
	for _, entitlement := range catalog.Data {
		if entitlement.ID == key {
			return nil
		}
	}
	return fmt.Errorf("entitlement %q is not in the App Sandbox catalog; run asc web review sandbox-entitlements catalog to list accepted keys", key)
}

func findWebReviewSandboxUsage(result *webcore.AppSandboxEntitlementUsagesResult, key string) (webcore.AppSandboxEntitlementUsage, bool, error) {
	var match webcore.AppSandboxEntitlementUsage
	found := false
	for _, usage := range result.Data {
		if usage.EntitlementID != key {
			continue
		}
		if found {
			return webcore.AppSandboxEntitlementUsage{}, false, fmt.Errorf("the version has several App Sandbox Information rows for %s; remove the extra rows in App Store Connect first", key)
		}
		match, found = usage, true
	}
	return match, found, nil
}

func webReviewSandboxReceipt(target *webcore.AppStoreVersionReviewDetail, key, usageID, justification string, changed bool, status string) *asc.WebReviewSandboxEntitlementResult {
	return &asc.WebReviewSandboxEntitlementResult{
		VersionID:      target.VersionID,
		ReviewDetailID: target.ReviewDetailID,
		Entitlement:    key,
		UsageID:        usageID,
		Justification:  justification,
		Changed:        changed,
		Status:         status,
	}
}
