package subscriptions

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// SubscriptionsPricingPlanAvailabilityCommand returns the plan availability command group.
func SubscriptionsPricingPlanAvailabilityCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pricing plan-availability", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "plan-availability",
		ShortUsage: "asc subscriptions pricing plan-availability <subcommand> [flags]",
		ShortHelp:  "Show and set subscription plan territory availability.",
		LongHelp: `Show and set subscription plan availability.

App Store Connect API 4.4 replaced the deprecated Subscription availability
resource with subscriptionPlanAvailabilities, one per billing plan (UPFRONT and
MONTHLY). These commands read and write that resource directly.

Examples:
  asc subscriptions pricing plan-availability show --subscription-id "SUB_ID"
  asc subscriptions pricing plan-availability set --subscription-id "SUB_ID" --plan-type UPFRONT --territories "United States,Canada" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			SubscriptionsPricingPlanAvailabilityShowCommand(),
			SubscriptionsPricingPlanAvailabilitySetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// SubscriptionsPricingPlanAvailabilityShowCommand reads plan availability for a subscription.
func SubscriptionsPricingPlanAvailabilityShowCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pricing plan-availability show", flag.ExitOnError)

	subscriptionID := shared.BindResourceIDFlag(fs, "subscription-id", "subscriptions", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "show",
		ShortUsage: "asc subscriptions pricing plan-availability show --subscription-id \"SUB_ID\" [flags]",
		ShortHelp:  "Show plan availability and available territories.",
		LongHelp: `Show every billing plan availability for a subscription with its available
territories included.

Apple caps the included availableTerritories linkages at 50 per plan
availability and reports the real count in the response paging metadata.
show warns on stderr when that cap truncates the included list; do not pass
the truncated include to set --territories. set reads the complete
relationship before replacing it.

Examples:
  asc subscriptions pricing plan-availability show --subscription-id "SUB_ID"
  asc subscriptions pricing plan-availability show --app "APP_ID" --subscription-id "com.example.pro.yearly"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("subscriptions pricing plan-availability show does not accept positional arguments")
			}
			id := strings.TrimSpace(*subscriptionID)
			if id == "" {
				return shared.UsageError("--subscription-id is required")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions pricing plan-availability show: %w", err)
			}
			id, err = resolveSubscriptionLookupIDWithTimeout(ctx, client, *appID, id)
			if err != nil {
				return err
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetSubscriptionPlanAvailabilitiesForSubscription(
				requestCtx,
				id,
				asc.WithSubscriptionPlanAvailabilitiesIncludeAvailableTerritories(),
			)
			if err != nil {
				return fmt.Errorf("subscriptions pricing plan-availability show: failed to fetch: %w", err)
			}
			warnTruncatedPlanAvailabilityTerritories(resp)

			return shared.PrintOutputWithRenderers(
				resp,
				*output.Output,
				*output.Pretty,
				func() error { return asc.PrintSubscriptionPlanAvailabilityShowTable(resp) },
				func() error { return asc.PrintSubscriptionPlanAvailabilityShowMarkdown(resp) },
			)
		},
	}
}

// SubscriptionsPricingPlanAvailabilitySetCommand sets the complete territory set for a plan.
func SubscriptionsPricingPlanAvailabilitySetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pricing plan-availability set", flag.ExitOnError)

	subscriptionID := shared.BindResourceIDFlag(fs, "subscription-id", "subscriptions", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	planType := fs.String("plan-type", "", "Billing plan: MONTHLY or UPFRONT")
	territories := shared.BindOnceCSVFlag(fs, "territories", "Complete desired territory list, comma-separated")
	lastBool := &lastVisitedBoolFlag{}
	availableInNewFlag := bindVisitedBoolFlag(fs, lastBool, "available-in-new-territories", "Make the plan available in new territories automatically; UPFRONT only")
	confirmFlag := bindVisitedBoolFlag(fs, lastBool, "confirm", "Confirm replacing the plan's territory availability")
	availableInNew := &availableInNewFlag.value
	confirm := &confirmFlag.value
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: "asc subscriptions pricing plan-availability set --subscription-id \"SUB_ID\" --plan-type UPFRONT --territories \"USA,CAN\" --confirm [flags]",
		ShortHelp:  "Set the complete territory availability for a billing plan.",
		LongHelp: `Set the complete territory availability for one subscription billing plan.

--territories is the desired state: territories missing from the list are
removed and territories in the list are added. The CLI reads the current
territories first, reports the added, removed, and unchanged sets, and skips the
write entirely when the plan already matches (changed:false). The plan
availability is created when the subscription has none for --plan-type.

Emptying availability is rejected here because it removes the subscription from
sale; use "asc web subscriptions availability remove-from-sale" for that flow.

--available-in-new-territories applies to UPFRONT plans only. USA and Singapore
are removed from MONTHLY requests because Apple excludes those storefronts.

Examples:
  asc subscriptions pricing plan-availability set --subscription-id "SUB_ID" --plan-type UPFRONT --territories "United States,Canada" --confirm
  asc subscriptions pricing plan-availability set --subscription-id "SUB_ID" --plan-type UPFRONT --territories "USA,CAN" --available-in-new-territories --confirm
  asc subscriptions pricing plan-availability set --subscription-id "SUB_ID" --plan-type MONTHLY --territories "Norway,Germany" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := rejectAmbiguousTrailingConfirm(args, lastBool.name); err != nil {
				return err
			}
			if err := shared.RecoverBoolFlagTailArgs(fs, args, availableInNew); err != nil {
				return err
			}

			id := strings.TrimSpace(*subscriptionID)
			if id == "" {
				return shared.UsageError("--subscription-id is required")
			}
			if strings.TrimSpace(*planType) == "" {
				return shared.UsageError("--plan-type is required")
			}
			normalizedPlanType, err := normalizeSubscriptionPlanType(*planType)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if !territories.Provided() {
				return shared.UsageError("--territories is required")
			}
			territoryIDs, err := shared.NormalizeASCTerritoryCSV(territories.String())
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if len(territoryIDs) == 0 {
				return shared.UsageError(
					"--territories must list at least one territory; use \"asc web subscriptions availability remove-from-sale\" to remove a subscription from sale",
				)
			}
			availableInNewProvided := flagWasProvided(fs, "available-in-new-territories")
			if normalizedPlanType == asc.SubscriptionPlanTypeMonthly && availableInNewProvided {
				return shared.UsageError("--available-in-new-territories is not supported for MONTHLY plan availability")
			}
			var excludedTerritoryIDs []string
			if normalizedPlanType == asc.SubscriptionPlanTypeMonthly {
				territoryIDs, excludedTerritoryIDs = filterMonthlyCommitmentTerritories(territoryIDs)
				if len(territoryIDs) == 0 {
					return shared.UsageError("no eligible monthly-commitment territories remain after excluding USA and Singapore")
				}
			}
			if !*confirm {
				return shared.UsageError("--confirm is required")
			}
			printMonthlyCommitmentTerritoryWarning(excludedTerritoryIDs)

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions pricing plan-availability set: %w", err)
			}
			id, err = resolveSubscriptionLookupIDWithTimeout(ctx, client, *appID, id)
			if err != nil {
				return err
			}

			listCtx, listCancel := shared.ContextWithTimeout(ctx)
			existing, err := client.GetSubscriptionPlanAvailabilitiesForSubscription(listCtx, id)
			listCancel()
			if err != nil {
				return fmt.Errorf("subscriptions pricing plan-availability set: failed to fetch plan availabilities: %w", err)
			}

			desiredTerritoryIDs := sortedUniqueTerritoryIDs(territoryIDs)
			plan, planFound := findSubscriptionPlanAvailabilityByType(existing, normalizedPlanType)

			var currentTerritoryIDs []string
			if planFound {
				currentTerritoryIDs, err = subscriptionPlanAvailabilityTerritories(ctx, client, plan.ID)
				if err != nil {
					return fmt.Errorf("subscriptions pricing plan-availability set: failed to fetch available territories: %w", err)
				}
				currentTerritoryIDs = sortedUniqueTerritoryIDs(currentTerritoryIDs)
			}

			added, removed, unchanged := diffTerritoryIDs(currentTerritoryIDs, desiredTerritoryIDs)
			attributeChange := planFound &&
				availableInNewProvided &&
				(plan.Attributes.AvailableInNewTerritories == nil || *plan.Attributes.AvailableInNewTerritories != *availableInNew)

			if planFound && len(added) == 0 && len(removed) == 0 && !attributeChange {
				result := &asc.SubscriptionPlanAvailabilitySetResult{
					SubscriptionID:            id,
					PlanAvailabilityID:        plan.ID,
					PlanType:                  string(normalizedPlanType),
					Changed:                   false,
					AvailableInNewTerritories: plan.Attributes.AvailableInNewTerritories,
					AddedTerritories:          added,
					RemovedTerritories:        removed,
					UnchangedTerritories:      unchanged,
					ExcludedTerritories:       excludedTerritoryIDs,
					AvailableTerritories:      currentTerritoryIDs,
				}
				return shared.PrintOutput(result, *output.Output, *output.Pretty)
			}

			var updateAttrs *asc.SubscriptionPlanAvailabilityUpdateAttributes
			if availableInNewProvided {
				updateAttrs = &asc.SubscriptionPlanAvailabilityUpdateAttributes{AvailableInNewTerritories: availableInNew}
			}

			var resp *asc.SubscriptionPlanAvailabilityResponse
			if planFound {
				updateCtx, updateCancel := shared.ContextWithTimeout(ctx)
				resp, err = client.UpdateSubscriptionPlanAvailability(updateCtx, plan.ID, desiredTerritoryIDs, updateAttrs)
				updateCancel()
				if err != nil {
					return fmt.Errorf("subscriptions pricing plan-availability set: failed to update plan availability: %w", err)
				}
			} else {
				createAttrs := asc.SubscriptionPlanAvailabilityAttributes{PlanType: normalizedPlanType}
				if availableInNewProvided {
					createAttrs.AvailableInNewTerritories = availableInNew
				}
				createCtx, createCancel := shared.ContextWithTimeout(ctx)
				resp, err = client.CreateSubscriptionPlanAvailability(createCtx, id, desiredTerritoryIDs, createAttrs)
				createCancel()
				if err != nil {
					return fmt.Errorf("subscriptions pricing plan-availability set: failed to create plan availability: %w", err)
				}
			}
			if resp == nil || strings.TrimSpace(resp.Data.ID) == "" {
				return fmt.Errorf("subscriptions pricing plan-availability set: App Store Connect returned no plan availability ID")
			}
			planAvailabilityID := strings.TrimSpace(resp.Data.ID)

			verifiedTerritoryIDs, err := subscriptionPlanAvailabilityTerritories(ctx, client, planAvailabilityID)
			if err != nil {
				return fmt.Errorf("subscriptions pricing plan-availability set: failed to verify available territories: %w", err)
			}
			verifiedTerritoryIDs = sortedUniqueTerritoryIDs(verifiedTerritoryIDs)
			missing, unexpected, _ := diffTerritoryIDs(verifiedTerritoryIDs, desiredTerritoryIDs)
			if len(missing) > 0 || len(unexpected) > 0 {
				return fmt.Errorf(
					"subscriptions pricing plan-availability set: plan availability %q does not match the requested territories after the write (missing: %s; unexpected: %s)",
					planAvailabilityID,
					formatTerritoryList(missing),
					formatTerritoryList(unexpected),
				)
			}

			readCtx, readCancel := shared.ContextWithTimeout(ctx)
			verified, err := client.GetSubscriptionPlanAvailability(readCtx, planAvailabilityID)
			readCancel()
			if err != nil {
				return fmt.Errorf("subscriptions pricing plan-availability set: failed to verify plan availability: %w", err)
			}
			if verified == nil {
				return fmt.Errorf("subscriptions pricing plan-availability set: App Store Connect returned no plan availability after the write")
			}
			verifiedAvailableInNew := verified.Data.Attributes.AvailableInNewTerritories
			if availableInNewProvided && (verifiedAvailableInNew == nil || *verifiedAvailableInNew != *availableInNew) {
				return fmt.Errorf(
					"subscriptions pricing plan-availability set: plan availability %q reports availableInNewTerritories=%s after requesting %t",
					planAvailabilityID,
					formatOptionalBool(verifiedAvailableInNew),
					*availableInNew,
				)
			}

			result := &asc.SubscriptionPlanAvailabilitySetResult{
				SubscriptionID:            id,
				PlanAvailabilityID:        planAvailabilityID,
				PlanType:                  string(normalizedPlanType),
				Changed:                   true,
				Created:                   !planFound,
				AvailableInNewTerritories: verifiedAvailableInNew,
				AddedTerritories:          added,
				RemovedTerritories:        removed,
				UnchangedTerritories:      unchanged,
				ExcludedTerritories:       excludedTerritoryIDs,
				AvailableTerritories:      verifiedTerritoryIDs,
			}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func findSubscriptionPlanAvailabilityByType(
	resp *asc.SubscriptionPlanAvailabilitiesResponse,
	planType asc.SubscriptionPlanType,
) (asc.Resource[asc.SubscriptionPlanAvailabilityAttributes], bool) {
	if resp == nil {
		return asc.Resource[asc.SubscriptionPlanAvailabilityAttributes]{}, false
	}
	for _, item := range resp.Data {
		if item.Attributes.PlanType == planType {
			return item, true
		}
	}
	return asc.Resource[asc.SubscriptionPlanAvailabilityAttributes]{}, false
}

func sortedUniqueTerritoryIDs(territoryIDs []string) []string {
	unique := make([]string, 0, len(territoryIDs))
	seen := make(map[string]struct{}, len(territoryIDs))
	for _, territoryID := range territoryIDs {
		territoryID = strings.ToUpper(strings.TrimSpace(territoryID))
		if territoryID == "" {
			continue
		}
		if _, ok := seen[territoryID]; ok {
			continue
		}
		seen[territoryID] = struct{}{}
		unique = append(unique, territoryID)
	}
	sort.Strings(unique)
	return unique
}

// diffTerritoryIDs reports the territories to add, to remove, and already in
// place when moving from current to desired.
func diffTerritoryIDs(current []string, desired []string) (added []string, removed []string, unchanged []string) {
	currentSet := make(map[string]struct{}, len(current))
	for _, territoryID := range current {
		currentSet[territoryID] = struct{}{}
	}
	desiredSet := make(map[string]struct{}, len(desired))
	for _, territoryID := range desired {
		desiredSet[territoryID] = struct{}{}
	}

	added = make([]string, 0)
	unchanged = make([]string, 0)
	for _, territoryID := range desired {
		if _, ok := currentSet[territoryID]; ok {
			unchanged = append(unchanged, territoryID)
			continue
		}
		added = append(added, territoryID)
	}

	removed = make([]string, 0)
	for _, territoryID := range current {
		if _, ok := desiredSet[territoryID]; !ok {
			removed = append(removed, territoryID)
		}
	}
	return added, removed, unchanged
}

func formatTerritoryList(territoryIDs []string) string {
	if len(territoryIDs) == 0 {
		return "none"
	}
	return strings.Join(territoryIDs, ",")
}

type lastVisitedBoolFlag struct {
	name string
}

type visitedBoolFlag struct {
	value bool
	name  string
	last  *lastVisitedBoolFlag
}

func (f *visitedBoolFlag) Set(value string) error {
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return err
	}
	f.value = parsed
	if f.last != nil {
		f.last.name = f.name
	}
	return nil
}

func (f *visitedBoolFlag) String() string {
	if f == nil {
		return "false"
	}
	return strconv.FormatBool(f.value)
}

func (f *visitedBoolFlag) IsBoolFlag() bool {
	return true
}

func bindVisitedBoolFlag(fs *flag.FlagSet, last *lastVisitedBoolFlag, name, usage string) *visitedBoolFlag {
	value := &visitedBoolFlag{name: name, last: last}
	fs.Var(value, name, usage)
	return value
}

func rejectAmbiguousTrailingConfirm(args []string, lastBoolName string) error {
	if len(args) == 0 || lastBoolName != "confirm" {
		return nil
	}
	token := strings.TrimSpace(args[0])
	if _, err := strconv.ParseBool(token); err != nil {
		return nil
	}
	return shared.UsageErrorf("--confirm %s is ambiguous after another boolean flag; use --confirm=%s", token, token)
}

func warnTruncatedPlanAvailabilityTerritories(resp *asc.SubscriptionPlanAvailabilitiesResponse) {
	if resp == nil {
		return
	}
	for _, item := range resp.Data {
		ids, total, known := asc.SubscriptionPlanAvailabilityIncludedTerritories(item.Relationships)
		if !known || total <= len(ids) {
			continue
		}
		fmt.Fprintf(
			os.Stderr,
			"Warning: plan availability %q includes %d of %d availableTerritories; Apple caps include=availableTerritories at %d. Do not pass this incomplete list to set --territories; set reads the complete relationship before replacing it.\n",
			item.ID,
			len(ids),
			total,
			asc.SubscriptionPlanAvailabilityIncludedTerritoriesLimit,
		)
	}
}

func formatOptionalBool(value *bool) string {
	if value == nil {
		return "unset"
	}
	return fmt.Sprintf("%t", *value)
}
