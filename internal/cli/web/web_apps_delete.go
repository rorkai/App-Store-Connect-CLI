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

var getWebAppRemovalStateFn = func(ctx context.Context, client *webcore.Client, appID string) (*webcore.AppRemovalState, error) {
	return client.GetAppRemovalState(ctx, appID)
}

var appRemovalBlockedStates = map[string]struct{}{
	"READY_FOR_REVIEW":   {},
	"WAITING_FOR_REVIEW": {},
	"IN_REVIEW":          {},
	"METADATA_REJECTED":  {},
	"REJECTED":           {},
}

// WebAppsDeleteCommand removes apps using the internal web API.
func WebAppsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web apps delete", flag.ExitOnError)

	app := fs.String("app", "", "App Store Connect app ID or exact bundle ID")
	expectedBundleID := fs.String("expected-bundle-id", "", "Require the app bundle ID to match before deleting")
	expectedName := fs.String("expected-name", "", "Require the app name to match before deleting")
	confirm := fs.Bool("confirm", false, "Confirm deleting this app (required unless --dry-run)")
	dryRun := fs.Bool("dry-run", false, "Check removal eligibility without mutating")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc web apps delete --app APP_ID_OR_BUNDLE_ID [--dry-run | --confirm] [flags]",
		ShortHelp:  "Delete an app via Apple web API.",
		LongHelp: `WEB SESSION WORKFLOWS

Delete an app through Apple's web API using a web-session login.

The public App Store Connect API does not expose app deletion. This command uses
the same web-session PATCH observed from App Store Connect: it marks the app
resource as removed. Pass an explicit App Store Connect app ID, or an exact
bundle ID for a web API lookup before deletion. Exact identity guard flags can
be used to stop before mutation if the resolved app is not the one expected.

Before mutating, the command reads current app and availability state and fails
closed when an observable Apple removal prerequisite is unmet. After the PATCH
it re-reads the app and only reports success when the server confirms
removed=true. --dry-run runs that preflight without mutating.

Observable prerequisites: removed from sale in all territories, not in a
blocking review state, and not using a non-App-Store marketplace attribute.
In-app purchases still on sale, alternative-marketplace Integrations
membership, in-progress app transfers, and app-bundle membership are not
checked because those are not exposed by the existing web readers used here.

Examples:
  asc web apps delete --app "1234567890" --dry-run
  asc web apps delete --app "1234567890" --confirm
  asc web apps delete --app "com.example.throwaway" --confirm
  asc web apps delete --app "1234567890" --expected-bundle-id "com.example.throwaway" --confirm
  asc web apps delete --app "1234567890" --expected-name "Throwaway" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			selector := strings.TrimSpace(*app)
			wantBundleID := strings.TrimSpace(*expectedBundleID)
			wantName := strings.TrimSpace(*expectedName)
			if selector == "" {
				return shared.UsageError("--app is required")
			}
			if err := shared.RequireConfirmUnlessDryRun(*dryRun, *confirm); err != nil {
				return err
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return err
			}
			client := newWebClientFn(session)

			appID, _, err := resolveWebAppDeleteTarget(requestCtx, client, selector)
			if err != nil {
				return err
			}

			snapshot, err := getWebAppRemovalStateFn(requestCtx, client, appID)
			if err != nil {
				return withWebAuthHint(err, "web apps delete")
			}
			if snapshot == nil || strings.TrimSpace(snapshot.ID) == "" {
				return fmt.Errorf("web apps delete failed: app %q could not be loaded", appID)
			}

			if err := validateWebAppDeleteGuards(appResponseFromRemovalState(snapshot), wantBundleID, wantName); err != nil {
				return err
			}
			if !snapshot.RemovedKnown {
				return fmt.Errorf("web apps delete failed: could not confirm removed for app %q; Apple omitted or mistyped the removed attribute", snapshot.ID)
			}
			if snapshot.Removed {
				return printWebAppDeleteResult(webAppDeleteResultFromState(snapshot, *dryRun), *output.Output, *output.Pretty)
			}
			if err := validateWebAppDeleteRemovalState(snapshot); err != nil {
				return err
			}

			availability, err := getWebAppAvailabilityFn(requestCtx, client, snapshot.ID)
			if err != nil && !webcore.IsAppAvailabilityNotFound(err) {
				return withWebAuthHint(fmt.Errorf("web apps delete failed: could not read availability for app %q: %w", snapshot.ID, err), "web apps delete")
			}
			if err := validateWebAppDeleteAvailability(snapshot.ID, availability); err != nil {
				return err
			}

			if *dryRun {
				return printWebAppDeleteResult(webAppDeleteResultFromState(snapshot, true), *output.Output, *output.Pretty)
			}

			if _, err := deleteWebAppFn(requestCtx, client, snapshot.ID); err != nil {
				return withWebAuthHint(err, "web apps delete")
			}

			verified, err := getWebAppRemovalStateFn(requestCtx, client, snapshot.ID)
			if err != nil {
				return withWebAuthHint(fmt.Errorf("web apps delete failed: could not re-read app %q after PATCH: %w", snapshot.ID, err), "web apps delete")
			}
			if verified == nil || !verified.RemovedKnown || !verified.Removed {
				return fmt.Errorf("web apps delete failed: Apple did not confirm app %q is removed after PATCH", snapshot.ID)
			}

			return printWebAppDeleteResult(webAppDeleteResultFromState(verified, false), *output.Output, *output.Pretty)
		},
	}
}

func resolveWebAppDeleteTarget(ctx context.Context, client *webcore.Client, selector string) (string, *webcore.AppResponse, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return "", nil, fmt.Errorf("app selector is required")
	}
	if shared.IsNumericAppID(selector) {
		return selector, nil, nil
	}

	app, err := findWebAppFn(ctx, client, selector)
	if err != nil {
		return "", nil, withWebAuthHint(err, "web apps delete")
	}
	if app == nil || strings.TrimSpace(app.Data.ID) == "" {
		return "", nil, fmt.Errorf("web apps delete failed: app %q not found by exact bundle ID; pass the App Store Connect app ID", selector)
	}
	actualBundleID := webAppAttrString(app, "bundleId")
	if actualBundleID == "" {
		return "", nil, fmt.Errorf("web apps delete failed: Apple did not return a bundle ID for app %q; pass the App Store Connect app ID", selector)
	}
	if actualBundleID != selector {
		return "", nil, fmt.Errorf("web apps delete failed: exact bundle ID lookup for %q returned %q; pass the App Store Connect app ID", selector, actualBundleID)
	}
	return strings.TrimSpace(app.Data.ID), app, nil
}

func validateWebAppDeleteGuards(app *webcore.AppResponse, expectedBundleID, expectedName string) error {
	if expectedBundleID == "" && expectedName == "" {
		return nil
	}
	if app == nil {
		return fmt.Errorf("web apps delete failed: app identity could not be loaded")
	}
	if expectedBundleID != "" {
		actual := webAppAttrString(app, "bundleId")
		if actual == "" {
			return fmt.Errorf("web apps delete failed: expected bundle ID %q, but Apple did not return a bundle ID for app %q", expectedBundleID, strings.TrimSpace(app.Data.ID))
		}
		if actual != expectedBundleID {
			return fmt.Errorf("web apps delete failed: expected bundle ID %q, got %q for app %q", expectedBundleID, actual, strings.TrimSpace(app.Data.ID))
		}
	}
	if expectedName != "" {
		actual := webAppAttrString(app, "name")
		if actual == "" {
			return fmt.Errorf("web apps delete failed: expected name %q, but Apple did not return a name for app %q", expectedName, strings.TrimSpace(app.Data.ID))
		}
		if actual != expectedName {
			return fmt.Errorf("web apps delete failed: expected name %q, got %q for app %q", expectedName, actual, strings.TrimSpace(app.Data.ID))
		}
	}
	return nil
}

func validateWebAppDeleteRemovalState(state *webcore.AppRemovalState) error {
	if state == nil {
		return fmt.Errorf("web apps delete failed: app identity could not be loaded")
	}
	status := strings.ToUpper(strings.TrimSpace(state.AppStoreLegacyStatus))
	if _, blocked := appRemovalBlockedStates[status]; blocked {
		return fmt.Errorf("web apps delete failed: app %q is in %s; Apple does not allow removal while an app is Ready for Review, Waiting for Review, In Review, Metadata Rejected, or Rejected", state.ID, status)
	}
	for _, versionState := range state.VersionStates {
		normalized := strings.ToUpper(strings.TrimSpace(versionState))
		if _, blocked := appRemovalBlockedStates[normalized]; blocked {
			return fmt.Errorf("web apps delete failed: app %q has a version in %s; Apple does not allow removal while an app is Ready for Review, Waiting for Review, In Review, Metadata Rejected, or Rejected", state.ID, normalized)
		}
	}
	if !state.DisplayableVersionsLoaded {
		return fmt.Errorf("web apps delete failed: could not confirm displayableVersions for app %q; Apple omitted the version linkage or included payload", state.ID)
	}
	if strings.TrimSpace(state.AppStoreLegacyStatus) == "" && len(state.VersionStates) == 0 {
		return fmt.Errorf("web apps delete failed: could not confirm appStoreLegacyStatus for app %q; Apple omitted the app-level review status and no displayable version states were present", state.ID)
	}
	marketplace := strings.ToUpper(strings.TrimSpace(state.Marketplace))
	if marketplace != "" && marketplace != "APP_STORE" {
		return fmt.Errorf("web apps delete failed: app %q is still distributed via marketplace %q; remove it from alternative marketplace distribution first", state.ID, strings.TrimSpace(state.Marketplace))
	}
	return nil
}

func validateWebAppDeleteAvailability(appID string, availability *webcore.AppAvailability) error {
	if availability == nil {
		return nil
	}
	if len(availability.AvailableTerritories) > 0 {
		return fmt.Errorf("web apps delete failed: app %q is still available in territories %s; remove it from sale in all territories first", appID, strings.Join(availability.AvailableTerritories, ", "))
	}
	if !availability.AvailableTerritoriesLoaded {
		return fmt.Errorf("web apps delete failed: could not confirm availableTerritories for app %q; Apple omitted or nulled the territory linkage", appID)
	}
	if !availability.AvailableInNewTerritoriesKnown {
		return fmt.Errorf("web apps delete failed: could not confirm availableInNewTerritories for app %q; Apple omitted or mistyped the new-territory setting", appID)
	}
	if availability.AvailableInNewTerritories {
		return fmt.Errorf("web apps delete failed: app %q is still available in new territories; disable new-territory availability first", appID)
	}
	return nil
}

func appResponseFromRemovalState(state *webcore.AppRemovalState) *webcore.AppResponse {
	if state == nil {
		return nil
	}
	resp := &webcore.AppResponse{}
	resp.Data.ID = strings.TrimSpace(state.ID)
	resp.Data.Type = "apps"
	resp.Data.Attributes = map[string]any{
		"name":     state.Name,
		"bundleId": state.BundleID,
		"removed":  state.Removed,
	}
	return resp
}

func webAppDeleteResultFromState(state *webcore.AppRemovalState, dryRun bool) asc.WebAppDeleteResult {
	if state == nil {
		return asc.WebAppDeleteResult{DryRun: dryRun}
	}
	return asc.WebAppDeleteResult{
		AppID:    strings.TrimSpace(state.ID),
		Name:     strings.TrimSpace(state.Name),
		BundleID: strings.TrimSpace(state.BundleID),
		Removed:  state.Removed,
		DryRun:   dryRun,
	}
}

func webAppAttrString(app *webcore.AppResponse, key string) string {
	if app == nil || app.Data.Attributes == nil {
		return ""
	}
	value, ok := app.Data.Attributes[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func printWebAppDeleteResult(result asc.WebAppDeleteResult, format string, pretty bool) error {
	return shared.PrintOutput(&result, format, pretty)
}
