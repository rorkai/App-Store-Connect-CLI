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
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
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
	confirm := false
	if apply {
		fs.BoolVar(&confirm, "confirm", false, "Apply capability changes")
	}
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
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}
			if apply && !confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError("--confirm")
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
					if outputErr := shared.PrintOutput(plan, *output.Output, *output.Pretty); outputErr != nil {
						return fmt.Errorf("bundle-ids capabilities reconcile: %w (partial receipt output: %w)", err, outputErr)
					}
					return shared.NewReportedError(fmt.Errorf("bundle-ids capabilities reconcile: %w", err))
				}
			}
			return shared.PrintOutput(plan, *output.Output, *output.Pretty)
		},
	}
}

func readDesiredEntitlements(path string, ignoreUnknown bool) ([]desiredEntitlement, error) {
	file, err := rootfs.OpenFile(path)
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
	seen := map[string]int{}
	for _, key := range keys {
		item, ok := catalog[key]
		if !ok {
			if !ignoreUnknown {
				return nil, shared.UsageErrorf("unknown entitlement key %q; pass --ignore-unknown to skip it", key)
			}
			selected = append(selected, desiredEntitlement{spec: entitlementCapability{Key: key}, value: raw[key]})
			continue
		}
		requested, err := entitlementValueRequested(key, raw[key], item.ValueKind)
		if err != nil {
			return nil, err
		}
		if !requested {
			continue
		}
		if item.UnsupportedCapability != "" {
			return nil, shared.UsageErrorf("entitlement %q requires unsupported capability %s; no supported asc API or web command can enable it", key, item.UnsupportedCapability)
		}
		if key == "com.apple.developer.default-data-protection" {
			value, ok := raw[key].(string)
			if !ok {
				return nil, shared.UsageError("data protection entitlement must be a string")
			}
			if value == "NSFileProtectionNone" {
				continue
			}
			if _, ok := dataProtectionOptions[value]; !ok {
				return nil, shared.UsageErrorf("unsupported data protection entitlement value %q", value)
			}
		}
		if item.Capability != "" {
			if index, ok := seen[item.Capability]; ok {
				// Several entitlement keys can request one ASC capability. Retain
				// the key that also determines its settings, if one is present.
				if item.Settings != nil {
					selected[index] = desiredEntitlement{spec: item, value: raw[key]}
				}
				continue
			}
			seen[item.Capability] = len(selected)
		}
		selected = append(selected, desiredEntitlement{spec: item, value: raw[key]})
	}
	return selected, nil
}

func entitlementValueRequested(key string, value any, kind entitlementValueKind) (bool, error) {
	switch kind {
	case entitlementValueBoolean:
		enabled, ok := value.(bool)
		if !ok {
			return false, shared.UsageErrorf("entitlement %q must be a boolean", key)
		}
		return enabled, nil
	case entitlementValueString:
		text, ok := value.(string)
		if !ok || strings.TrimSpace(text) == "" {
			return false, shared.UsageErrorf("entitlement %q must be a non-empty string", key)
		}
		return true, nil
	case entitlementValueStringArray:
		values, ok := entitlementStringArray(value)
		if !ok {
			return false, shared.UsageErrorf("entitlement %q must be an array of strings", key)
		}
		return len(values) > 0, nil
	default:
		// For catalog entries whose exact value schema has not yet been
		// established, a false Boolean still must not request a capability.
		if enabled, ok := value.(bool); ok && !enabled {
			return false, nil
		}
		return true, nil
	}
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
			action := asc.CapabilityReconcileAction{Action: "needsWebSession", Entitlement: item.spec.Key}
			if quotedBundleID, ok := shared.ShellQuote(bundleID); ok {
				action.Command = item.spec.WebCommand + " --bundle-id " + quotedBundleID + " --confirm"
			} else {
				action.Error = "bundle ID cannot be safely rendered in a shell command"
			}
			actions = append(actions, action)
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
	bundle, err := client.GetBundleID(ctx, value)
	if err == nil {
		if bundle == nil || bundle.Data.ID == "" {
			return "", fmt.Errorf("bundle ID lookup returned no resource ID: %s", value)
		}
		return bundle.Data.ID, nil
	}
	if !asc.IsNotFound(err) {
		return "", err
	}
	resp, err := client.GetBundleIDs(ctx, asc.WithBundleIDsFilterIdentifier(value))
	if err != nil {
		return "", err
	}
	if len(resp.Data) == 0 {
		return "", fmt.Errorf("bundle ID not found: %s", value)
	}
	if len(resp.Data) > 1 {
		return "", fmt.Errorf("multiple bundle IDs found for identifier %q; use a resource ID", value)
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
	for index := range plan.Actions {
		action := &plan.Actions[index]
		if action.Action == "needsWebSession" {
			action.Status = "failed"
			action.Error = "requires a web session; run " + action.Command
			return fmt.Errorf("entitlement %s requires a web session", action.Entitlement)
		}
	}
	for index := range plan.Actions {
		action := &plan.Actions[index]
		var err error
		switch action.Action {
		case "add":
			var created *asc.BundleIDCapabilityResponse
			created, err = client.CreateBundleIDCapability(ctx, bundleID, asc.BundleIDCapabilityCreateAttributes{CapabilityType: action.Capability, Settings: action.Settings})
			if err == nil && created != nil {
				action.CapabilityID = created.Data.ID
			}
		case "update":
			_, err = client.UpdateBundleIDCapability(ctx, action.CapabilityID, asc.BundleIDCapabilityUpdateAttributes{CapabilityType: action.Capability, Settings: action.Settings})
		case "remove":
			err = client.DeleteBundleIDCapability(ctx, action.CapabilityID)
		default:
			continue
		}
		if err != nil {
			action.Status = "failed"
			action.Error = err.Error()
			return err
		}
		action.Status = "applied"
	}
	return nil
}
