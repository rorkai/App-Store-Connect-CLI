package bundleids

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"howett.net/plist"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type desiredEntitlement struct {
	spec  entitlementCapability
	value any
}

func BundleIDsCapabilitiesReconcileCommand() *ffcli.Command {
	fs := flag.NewFlagSet("reconcile", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "reconcile",
		ShortUsage: "asc bundle-ids capabilities reconcile <plan|apply> [flags]",
		ShortHelp:  "Reconcile bundle ID capabilities from an entitlements file.",
		LongHelp: `Reconcile bundle ID capabilities from an entitlements file.

plan shows add, update, keep, and unmanaged actions. apply requires --confirm.
Existing sub-feature settings, such as broadcast push, are preserved. Removal
requires --allow-remove and only applies to capability types this mapping knows.

Examples:
  asc bundle-ids capabilities reconcile plan --bundle "BUNDLE_ID" --entitlements ./App/App.entitlements
  asc bundle-ids capabilities reconcile apply --bundle "BUNDLE_ID" --entitlements ./App/App.entitlements --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			capabilityReconcileCommand("plan", false),
			capabilityReconcileCommand("apply", true),
		},
		Exec: func(context.Context, []string) error { return flag.ErrHelp },
	}
}

func capabilityReconcileCommand(name string, apply bool) *ffcli.Command {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	bundleID := fs.String("bundle", "", "Bundle ID resource ID or identifier")
	entitlements := fs.String("entitlements", "", "Path to an entitlements plist")
	ignoreUnknown := fs.Bool("ignore-unknown", false, "Ignore entitlement keys this mapping does not know")
	allowRemove := fs.Bool("allow-remove", false, "Remove mapped capabilities that the entitlements file does not request")
	confirm := fs.Bool("confirm", false, "Apply capability changes")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       name,
		ShortUsage: "asc bundle-ids capabilities reconcile " + name + " --bundle BUNDLE_ID --entitlements PATH",
		ShortHelp:  "Reconcile capabilities from entitlements.",
		FlagSet:    fs,
		UsageFunc:  shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := shared.RejectPositionalArgs(args); err != nil {
				return err
			}
			if apply && !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.UsageError("--confirm is required")
			}
			bundleValue := strings.TrimSpace(*bundleID)
			if bundleValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --bundle is required")
				return shared.MissingRequiredUsageError("--bundle")
			}
			path := strings.TrimSpace(*entitlements)
			if path == "" {
				fmt.Fprintln(os.Stderr, "Error: --entitlements is required")
				return shared.MissingRequiredUsageError("--entitlements")
			}
			desired, err := readDesiredEntitlements(path, *ignoreUnknown)
			if err != nil {
				return err
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("bundle-ids capabilities reconcile: %w", err)
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			resolved, err := resolveCapabilityBundleID(requestCtx, client, bundleValue)
			if err != nil {
				return fmt.Errorf("bundle-ids capabilities reconcile: %w", err)
			}
			existing, err := listBundleCapabilities(requestCtx, client, resolved)
			if err != nil {
				return fmt.Errorf("bundle-ids capabilities reconcile: %w", err)
			}
			plan := buildCapabilityReconcilePlan(resolved, desired, existing, *allowRemove)
			if apply {
				if err := applyCapabilityReconcilePlan(requestCtx, client, resolved, plan); err != nil {
					return fmt.Errorf("bundle-ids capabilities reconcile: %w", err)
				}
			}
			return shared.PrintOutput(plan, *output.Output, *output.Pretty)
		},
	}
}

func readDesiredEntitlements(path string, ignoreUnknown bool) ([]desiredEntitlement, error) {
	file, err := shared.OpenExistingNoFollow(path)
	if err != nil {
		return nil, fmt.Errorf("bundle-ids capabilities reconcile: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("bundle-ids capabilities reconcile: read entitlements: %w", err)
	}
	var raw map[string]any
	if _, err := plist.Unmarshal(data, &raw); err != nil {
		return nil, shared.UsageErrorf("entitlements file is not a plist: %v", err)
	}
	catalog := map[string]entitlementCapability{}
	for _, item := range entitlementCapabilityCatalog() {
		catalog[item.Key] = item
	}
	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	selected := make([]desiredEntitlement, 0)
	seen := map[string]struct{}{}
	for _, key := range keys {
		item, ok := catalog[key]
		if !ok {
			if !ignoreUnknown {
				return nil, shared.UsageErrorf("unknown entitlement key %q; pass --ignore-unknown to skip it", key)
			}
			selected = append(selected, desiredEntitlement{spec: entitlementCapability{Key: key}, value: raw[key]})
			continue
		}
		if item.Capability != "" {
			if _, ok := seen[item.Capability]; ok {
				continue
			}
			seen[item.Capability] = struct{}{}
		}
		selected = append(selected, desiredEntitlement{spec: item, value: raw[key]})
	}
	return selected, nil
}

func buildCapabilityReconcilePlan(bundleID string, desired []desiredEntitlement, existing []asc.Resource[asc.BundleIDCapabilityAttributes], allowRemove bool) *asc.CapabilityReconcilePlan {
	byType := map[string]asc.Resource[asc.BundleIDCapabilityAttributes]{}
	for _, item := range existing {
		byType[strings.ToUpper(item.Attributes.CapabilityType)] = item
	}
	requested := map[string]struct{}{}
	actions := make([]asc.CapabilityReconcileAction, 0)
	for _, item := range desired {
		if item.spec.WebCommand != "" {
			actions = append(actions, asc.CapabilityReconcileAction{Action: "needsWebSession", Entitlement: item.spec.Key, Command: item.spec.WebCommand})
			continue
		}
		if item.spec.Capability == "" {
			actions = append(actions, asc.CapabilityReconcileAction{Action: "ignored", Entitlement: item.spec.Key})
			continue
		}
		requested[item.spec.Capability] = struct{}{}
		var desiredSettings []asc.CapabilitySetting
		if item.spec.Settings != nil {
			desiredSettings = item.spec.Settings(item.value)
		}
		current, ok := byType[item.spec.Capability]
		if !ok {
			actions = append(actions, asc.CapabilityReconcileAction{Action: "add", Capability: item.spec.Capability, Entitlement: item.spec.Key, Settings: desiredSettings})
			continue
		}
		merged, changed := mergeCapabilitySettings(current.Attributes.Settings, desiredSettings)
		action := "keep"
		if changed {
			action = "update"
		}
		actions = append(actions, asc.CapabilityReconcileAction{Action: action, Capability: item.spec.Capability, Entitlement: item.spec.Key, CapabilityID: current.ID, Settings: merged})
	}
	for _, item := range existing {
		capability := strings.ToUpper(item.Attributes.CapabilityType)
		if _, ok := requested[capability]; ok {
			continue
		}
		action := "unmanaged"
		if allowRemove && mappedCapability(capability) {
			action = "remove"
		}
		actions = append(actions, asc.CapabilityReconcileAction{Action: action, Capability: capability, CapabilityID: item.ID})
	}
	sort.SliceStable(actions, func(i, j int) bool {
		if actions[i].Action != actions[j].Action {
			return actions[i].Action < actions[j].Action
		}
		return actions[i].Capability+actions[i].Entitlement < actions[j].Capability+actions[j].Entitlement
	})
	return &asc.CapabilityReconcilePlan{BundleID: bundleID, Actions: actions}
}

func resolveCapabilityBundleID(ctx context.Context, client *asc.Client, value string) (string, error) {
	if !strings.Contains(value, ".") {
		return value, nil
	}
	resp, err := client.GetBundleIDs(ctx, asc.WithBundleIDsFilterIdentifier(value))
	if err != nil {
		return "", err
	}
	if len(resp.Data) == 0 {
		return "", fmt.Errorf("bundle ID not found: %s", value)
	}
	return resp.Data[0].ID, nil
}

func listBundleCapabilities(ctx context.Context, client *asc.Client, bundleID string) ([]asc.Resource[asc.BundleIDCapabilityAttributes], error) {
	first, err := client.GetBundleIDCapabilities(ctx, bundleID)
	if err != nil {
		return nil, err
	}
	paginated, err := asc.PaginateAll(ctx, first, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetBundleIDCapabilities(ctx, bundleID, asc.WithBundleIDCapabilitiesNextURL(nextURL))
	})
	if err != nil {
		return nil, err
	}
	resp, ok := paginated.(*asc.BundleIDCapabilitiesResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected capabilities response")
	}
	return resp.Data, nil
}

func applyCapabilityReconcilePlan(ctx context.Context, client *asc.Client, bundleID string, plan *asc.CapabilityReconcilePlan) error {
	for _, action := range plan.Actions {
		switch action.Action {
		case "add":
			if _, err := client.CreateBundleIDCapability(ctx, bundleID, asc.BundleIDCapabilityCreateAttributes{CapabilityType: action.Capability, Settings: action.Settings}); err != nil {
				return err
			}
		case "update":
			if _, err := client.UpdateBundleIDCapability(ctx, action.CapabilityID, asc.BundleIDCapabilityUpdateAttributes{CapabilityType: action.Capability, Settings: action.Settings}); err != nil {
				return err
			}
		case "remove":
			if err := client.DeleteBundleIDCapability(ctx, action.CapabilityID); err != nil {
				return err
			}
		}
	}
	return nil
}
