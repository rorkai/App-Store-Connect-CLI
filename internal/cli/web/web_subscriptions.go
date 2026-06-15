package web

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

var (
	listWebSubscriptionPlanAvailabilitiesFn = func(ctx context.Context, client *webcore.Client, subscriptionID string) ([]webcore.SubscriptionPlanAvailability, error) {
		return client.ListSubscriptionPlanAvailabilities(ctx, subscriptionID)
	}
	removeWebSubscriptionPlanAvailabilityFromSaleFn = func(ctx context.Context, client *webcore.Client, planAvailabilityID string) (*webcore.SubscriptionPlanAvailability, error) {
		return client.RemoveSubscriptionPlanAvailabilityFromSale(ctx, planAvailabilityID)
	}
)

type webSubscriptionRemoveFromSaleResult struct {
	SubscriptionID            string   `json:"subscriptionId"`
	PlanAvailabilityID        string   `json:"planAvailabilityId"`
	PlanType                  string   `json:"planType,omitempty"`
	RemovedFromSale           bool     `json:"removedFromSale"`
	AvailableInNewTerritories bool     `json:"availableInNewTerritories"`
	AvailableTerritories      []string `json:"availableTerritories"`
	RequiresAccountHolderRole bool     `json:"requiresAccountHolderRole"`
}

// WebSubscriptionsCommand returns the web subscriptions command group.
func WebSubscriptionsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web subscriptions", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "subscriptions",
		ShortUsage: "asc web subscriptions <subcommand> [flags]",
		ShortHelp:  "[experimental] Manage subscriptions via web sessions.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Manage subscription workflows that App Store Connect exposes only through web-session endpoints.

` + webWarningText,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebSubscriptionsAvailabilityCommand(),
			WebSubscriptionsPricingCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebSubscriptionsAvailabilityCommand returns the web subscription availability command group.
func WebSubscriptionsAvailabilityCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web subscriptions availability", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "availability",
		ShortUsage: "asc web subscriptions availability <subcommand> [flags]",
		ShortHelp:  "[experimental] Manage subscription sale availability via web sessions.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Manage subscription sale availability through Apple's internal web API.

` + webWarningText,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebSubscriptionsAvailabilityRemoveFromSaleCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebSubscriptionsAvailabilityRemoveFromSaleCommand removes an approved subscription from sale.
func WebSubscriptionsAvailabilityRemoveFromSaleCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web subscriptions availability remove-from-sale", flag.ExitOnError)

	appID := fs.String("app", "", "App ID; enables product ID or exact name lookup")
	subscriptionID := fs.String("subscription-id", "", "Subscription ID, product ID, or exact current name")
	planAvailabilityID := fs.String("plan-availability-id", "", "Subscription plan availability ID")
	confirm := fs.Bool("confirm", false, "Confirm removing the subscription from sale")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "remove-from-sale",
		ShortUsage: "asc web subscriptions availability remove-from-sale --subscription-id SUB_ID --confirm [flags]",
		ShortHelp:  "[experimental] Remove an auto-renewable subscription from sale.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Remove an approved auto-renewable subscription from sale using the same internal web API flow
as App Store Connect. Apple allows this action only for Account Holder users.

Examples:
  asc web subscriptions availability remove-from-sale --subscription-id "SUB_ID" --confirm
  asc web subscriptions availability remove-from-sale --app "APP_ID" --subscription-id "com.example.monthly" --confirm
  asc web subscriptions availability remove-from-sale --subscription-id "SUB_ID" --plan-availability-id "PLAN_AVAILABILITY_ID" --confirm

` + webWarningText,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("web subscriptions availability remove-from-sale does not accept positional arguments")
			}

			trimmedSubscriptionID := strings.TrimSpace(*subscriptionID)
			trimmedAppID := strings.TrimSpace(shared.ResolveAppID(*appID))
			trimmedPlanAvailabilityID := strings.TrimSpace(*planAvailabilityID)
			switch {
			case trimmedSubscriptionID == "":
				return shared.UsageError("--subscription-id is required")
			case !*confirm:
				return shared.UsageError("--confirm is required")
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			session, err := resolveWebSessionForCommand(requestCtx, authFlags)
			if err != nil {
				return err
			}
			client := newWebClientFn(session)

			if trimmedAppID != "" {
				subscriptions, err := loadReviewSubscriptionsWithLabel(requestCtx, client, trimmedAppID, "Loading subscriptions")
				if err != nil {
					return withWebAuthHint(err, "web subscriptions availability remove-from-sale")
				}
				selected, err := findReviewSubscription(subscriptions, trimmedSubscriptionID)
				if err != nil {
					return fmt.Errorf("subscription lookup for app %q failed: %w", trimmedAppID, err)
				}
				trimmedSubscriptionID = strings.TrimSpace(selected.ID)
			}

			availabilities, err := withWebSpinnerValue("Loading subscription plan availability", func() ([]webcore.SubscriptionPlanAvailability, error) {
				return listWebSubscriptionPlanAvailabilitiesFn(requestCtx, client, trimmedSubscriptionID)
			})
			if err != nil {
				return withWebAuthHint(err, "web subscriptions availability remove-from-sale")
			}

			var selected webcore.SubscriptionPlanAvailability
			if trimmedPlanAvailabilityID != "" {
				selected, err = findSubscriptionPlanAvailability(availabilities, trimmedPlanAvailabilityID)
				if err != nil {
					return fmt.Errorf("web subscriptions availability remove-from-sale failed: plan availability %q was not found for subscription %q", trimmedPlanAvailabilityID, trimmedSubscriptionID)
				}
			} else {
				selected, err = selectSubscriptionPlanAvailability(availabilities)
				if err != nil {
					return fmt.Errorf("web subscriptions availability remove-from-sale failed: %w", err)
				}
				trimmedPlanAvailabilityID = strings.TrimSpace(selected.ID)
			}

			removed, err := withWebSpinnerValue("Removing subscription from sale", func() (*webcore.SubscriptionPlanAvailability, error) {
				return removeWebSubscriptionPlanAvailabilityFromSaleFn(requestCtx, client, trimmedPlanAvailabilityID)
			})
			if err != nil {
				return withWebAccountHolderHint(err, "web subscriptions availability remove-from-sale")
			}
			if removed == nil || strings.TrimSpace(removed.ID) == "" {
				return fmt.Errorf("web subscriptions availability remove-from-sale failed: plan availability ID missing from response")
			}
			if !strings.EqualFold(strings.TrimSpace(removed.ID), trimmedPlanAvailabilityID) {
				return fmt.Errorf("web subscriptions availability remove-from-sale failed: Apple returned plan availability %q after patching %q", strings.TrimSpace(removed.ID), trimmedPlanAvailabilityID)
			}

			verifiedAvailabilities, err := withWebSpinnerValue("Verifying subscription removal from sale", func() ([]webcore.SubscriptionPlanAvailability, error) {
				return listWebSubscriptionPlanAvailabilitiesFn(requestCtx, client, trimmedSubscriptionID)
			})
			if err != nil {
				return withWebAuthHint(err, "web subscriptions availability remove-from-sale")
			}
			verified, err := findSubscriptionPlanAvailability(verifiedAvailabilities, trimmedPlanAvailabilityID)
			if err != nil {
				return fmt.Errorf("web subscriptions availability remove-from-sale failed: could not verify plan availability %q for subscription %q after patch", trimmedPlanAvailabilityID, trimmedSubscriptionID)
			}
			removedFromSale := subscriptionPlanAvailabilityRemovedFromSale(verified)
			if !removedFromSale {
				return fmt.Errorf(
					"web subscriptions availability remove-from-sale failed: plan availability %q is still available after patch (availableInNewTerritories=%t, availableTerritoriesLoaded=%t, availableTerritories=%d)",
					trimmedPlanAvailabilityID,
					verified.AvailableInNewTerritories,
					verified.AvailableTerritoriesLoaded,
					len(verified.AvailableTerritories),
				)
			}

			result := webSubscriptionRemoveFromSaleResult{
				SubscriptionID:            trimmedSubscriptionID,
				PlanAvailabilityID:        strings.TrimSpace(verified.ID),
				PlanType:                  strings.TrimSpace(verified.PlanType),
				RemovedFromSale:           removedFromSale,
				AvailableInNewTerritories: verified.AvailableInNewTerritories,
				AvailableTerritories:      verified.AvailableTerritories,
				RequiresAccountHolderRole: true,
			}
			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderWebSubscriptionRemoveFromSaleTable(result) },
				func() error { return renderWebSubscriptionRemoveFromSaleMarkdown(result) },
			)
		},
	}
}

func selectSubscriptionPlanAvailability(availabilities []webcore.SubscriptionPlanAvailability) (webcore.SubscriptionPlanAvailability, error) {
	if len(availabilities) == 0 {
		return webcore.SubscriptionPlanAvailability{}, fmt.Errorf("no subscription plan availability was found; pass --plan-availability-id if App Store Connect returned one elsewhere")
	}
	if len(availabilities) == 1 {
		return availabilities[0], nil
	}
	upfrontMatches := make([]webcore.SubscriptionPlanAvailability, 0, 1)
	for _, availability := range availabilities {
		if strings.EqualFold(strings.TrimSpace(availability.PlanType), "UPFRONT") {
			upfrontMatches = append(upfrontMatches, availability)
		}
	}
	if len(upfrontMatches) == 1 {
		return upfrontMatches[0], nil
	}
	return webcore.SubscriptionPlanAvailability{}, fmt.Errorf("multiple subscription plan availabilities matched; pass --plan-availability-id")
}

func findSubscriptionPlanAvailability(availabilities []webcore.SubscriptionPlanAvailability, planAvailabilityID string) (webcore.SubscriptionPlanAvailability, error) {
	planAvailabilityID = strings.TrimSpace(planAvailabilityID)
	for _, availability := range availabilities {
		if strings.EqualFold(strings.TrimSpace(availability.ID), planAvailabilityID) {
			return availability, nil
		}
	}
	return webcore.SubscriptionPlanAvailability{}, fmt.Errorf("plan availability not found")
}

func subscriptionPlanAvailabilityRemovedFromSale(availability webcore.SubscriptionPlanAvailability) bool {
	return !availability.AvailableInNewTerritories && availability.AvailableTerritoriesLoaded && len(availability.AvailableTerritories) == 0
}

func withWebAccountHolderHint(err error, operation string) error {
	if err == nil {
		return nil
	}
	var apiErr *webcore.APIError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusForbidden {
		return fmt.Errorf("%s failed: removing an approved auto-renewable subscription from sale requires the App Store Connect Account Holder role: %w", operation, err)
	}
	return withWebAuthHint(err, operation)
}

func renderWebSubscriptionRemoveFromSaleTable(result webSubscriptionRemoveFromSaleResult) error {
	asc.RenderTable(
		[]string{"Subscription ID", "Plan Availability ID", "Plan Type", "Removed From Sale", "Available In New Territories", "Available Territories"},
		[][]string{{
			result.SubscriptionID,
			result.PlanAvailabilityID,
			result.PlanType,
			fmt.Sprintf("%t", result.RemovedFromSale),
			fmt.Sprintf("%t", result.AvailableInNewTerritories),
			strings.Join(result.AvailableTerritories, ","),
		}},
	)
	return nil
}

func renderWebSubscriptionRemoveFromSaleMarkdown(result webSubscriptionRemoveFromSaleResult) error {
	fmt.Println("| Subscription ID | Plan Availability ID | Plan Type | Removed From Sale | Available In New Territories | Available Territories |")
	fmt.Println("|---|---|---|---|---|---|")
	fmt.Printf(
		"| %s | %s | %s | %t | %t | %s |\n",
		result.SubscriptionID,
		result.PlanAvailabilityID,
		result.PlanType,
		result.RemovedFromSale,
		result.AvailableInNewTerritories,
		strings.Join(result.AvailableTerritories, ","),
	)
	return nil
}
