package app_events

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var appEventsClientFactory = shared.GetASCClient

// AppEventsCommand returns the app events command group.
func AppEventsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app-events", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app-events",
		ShortUsage: "asc app-events <subcommand> [flags]",
		ShortHelp:  "Manage App Store in-app events.",
		LongHelp: `Manage App Store in-app events.

Examples:
  asc app-events list --app "APP_ID"
  asc app-events get --event-id "EVENT_ID"
  asc app-events create --app "APP_ID" --name "Summer Challenge" --event-type CHALLENGE --start "2026-06-01T00:00:00Z" --end "2026-06-30T23:59:59Z"
  asc app-events update --event-id "EVENT_ID" --priority HIGH
  asc app-events delete --event-id "EVENT_ID" --confirm
  asc app-events links --event-id "EVENT_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.VisibleUsageFunc,
		Subcommands: []*ffcli.Command{
			AppEventsListCommand(),
			AppEventsGetCommand(),
			AppEventsCreateCommand(),
			AppEventsUpdateCommand(),
			AppEventsDeleteCommand(),
			AppEventLocalizationsCommand(),
			AppEventsRelationshipsCommand(),
			AppEventScreenshotsCommand(),
			AppEventVideoClipsCommand(),
			AppEventsSubmitCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppEventsListCommand returns the app events list subcommand.
func AppEventsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc app-events list [flags]",
		ShortHelp:  "List in-app events for an app.",
		LongHelp: `List in-app events for an app.

Examples:
  asc app-events list --app "APP_ID"
  asc app-events list --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("app-events list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("app-events list: %w", err)
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := appEventsClientFactory()
			if err != nil {
				return fmt.Errorf("app-events list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AppEventsOption{
				asc.WithAppEventsLimit(*limit),
				asc.WithAppEventsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAppEventsLimit(200))
				firstPage, err := client.GetAppEvents(requestCtx, resolvedAppID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("app-events list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppEvents(ctx, resolvedAppID, asc.WithAppEventsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("app-events list: %w", err)
				}
				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppEvents(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("app-events list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppEventsGetCommand returns the app events get subcommand.
func AppEventsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	eventID := fs.String("event-id", "", "App event ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc app-events get --event-id \"EVENT_ID\"",
		ShortHelp:  "Get an in-app event by ID.",
		LongHelp: `Get an in-app event by ID.

Examples:
  asc app-events get --event-id "EVENT_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*eventID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --event-id is required")
				return flag.ErrHelp
			}

			client, err := appEventsClientFactory()
			if err != nil {
				return fmt.Errorf("app-events get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppEvent(requestCtx, id)
			if err != nil {
				return fmt.Errorf("app-events get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppEventsCreateCommand returns the app events create subcommand.
func AppEventsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	name := fs.String("name", "", "Reference name")
	eventType := fs.String("event-type", "", "Event type: "+strings.Join(asc.ValidAppEventBadges, ", "))
	start := fs.String("start", "", "Event start time (RFC3339)")
	end := fs.String("end", "", "Event end time (RFC3339)")
	publishStart := fs.String("publish-start", "", "Publish start time (RFC3339)")
	territories := fs.String("territories", "", "Territory codes (comma-separated)")
	deepLink := fs.String("deep-link", "", "Deep link URL")
	purchaseRequirement := fs.String("purchase-requirement", "", "Purchase requirement (currently supported: "+supportedAppEventPurchaseRequirementValues()+")")
	primaryLocale := fs.String("primary-locale", "", "Primary locale (e.g., en-US)")
	priority := fs.String("priority", "", "Priority: "+strings.Join(asc.ValidAppEventPriorities, ", "))
	purpose := fs.String("purpose", "", "Purpose: "+strings.Join(asc.ValidAppEventPurposes, ", "))
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc app-events create [flags]",
		ShortHelp:  "Create a new in-app event.",
		LongHelp: `Create a new in-app event.

Examples:
  asc app-events create --app "APP_ID" --name "Summer Challenge" --event-type CHALLENGE --start "2026-06-01T00:00:00Z" --end "2026-06-30T23:59:59Z"
  asc app-events create --app "APP_ID" --name "Launch Party" --event-type PREMIERE --priority HIGH --purpose ATTRACT_NEW_USERS
  asc app-events create --app "APP_ID" --name "Retro Challenge" --event-type LIVE_EVENT --priority HIGH --purpose APPROPRIATE_FOR_ALL_USERS`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			nameValue := strings.TrimSpace(*name)
			if nameValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return flag.ErrHelp
			}

			normalizedBadge, err := normalizeAppEventBadge(*eventType, true)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err.Error())
				return flag.ErrHelp
			}

			normalizedPriority, err := normalizeAppEventPriority(*priority)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err.Error())
				return flag.ErrHelp
			}

			normalizedPurpose, err := normalizeAppEventPurpose(*purpose)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err.Error())
				return flag.ErrHelp
			}

			normalizedPurchaseRequirement, err := normalizeAppEventPurchaseRequirement(*purchaseRequirement)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err.Error())
				return flag.ErrHelp
			}
			if err := validateAppEventPurchaseRequirement(normalizedPurchaseRequirement); err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err.Error())
				return flag.ErrHelp
			}

			schedule, scheduleProvided, err := normalizeAppEventTerritorySchedule(*start, *end, *publishStart, *territories)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err.Error())
				return flag.ErrHelp
			}

			attrs := asc.AppEventCreateAttributes{
				ReferenceName:       nameValue,
				Badge:               normalizedBadge,
				DeepLink:            strings.TrimSpace(*deepLink),
				PurchaseRequirement: normalizedPurchaseRequirement,
				PrimaryLocale:       strings.TrimSpace(*primaryLocale),
				Priority:            normalizedPriority,
				Purpose:             normalizedPurpose,
			}

			client, err := appEventsClientFactory()
			if err != nil {
				return fmt.Errorf("app-events create: %w", err)
			}

			createCtx, cancelCreate := shared.ContextWithTimeout(ctx)
			defer cancelCreate()

			resp, err := client.CreateAppEvent(createCtx, resolvedAppID, attrs)
			if err != nil {
				return fmt.Errorf("app-events create: failed to create: %w", err)
			}
			if scheduleProvided {
				createdID := strings.TrimSpace(resp.Data.ID)
				if createdID == "" {
					return fmt.Errorf("app-events create: event was created but the response did not include an id; unable to apply schedule")
				}

				updateCtx, cancelUpdate := shared.ContextWithTimeout(ctx)
				defer cancelUpdate()

				resp, err = client.UpdateAppEvent(updateCtx, createdID, asc.AppEventUpdateAttributes{
					TerritorySchedules: []asc.AppEventTerritorySchedule{schedule},
				})
				if err != nil {
					verifyCtx, cancelVerify := shared.ContextWithTimeout(ctx)
					defer cancelVerify()

					verifiedResp, verifyErr := client.GetAppEvent(verifyCtx, createdID)
					if verifyErr == nil {
						if appEventHasTerritorySchedule(verifiedResp, schedule) {
							resp = verifiedResp
						} else {
							cleanupCtx, cancelCleanup := shared.ContextWithTimeout(ctx)
							defer cancelCleanup()

							cleanupErr := client.DeleteAppEvent(cleanupCtx, createdID)
							if cleanupErr != nil {
								return fmt.Errorf("app-events create: created event %q but failed to apply schedule: %w (verification confirmed the schedule is still missing; cleanup also failed: %s)", createdID, err, cleanupErr.Error())
							}

							return fmt.Errorf("app-events create: failed to apply schedule after creating event %q; verification confirmed the schedule was not applied and the event was deleted so the command is safe to retry: %w", createdID, err)
						}
					} else {
						return fmt.Errorf("app-events create: created event %q but could not confirm whether the schedule update succeeded: %w (verification failed: %s)", createdID, err, verifyErr.Error())
					}
				}
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppEventsUpdateCommand returns the app events update subcommand.
func AppEventsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	eventID := fs.String("event-id", "", "App event ID")
	name := fs.String("name", "", "Reference name")
	eventType := fs.String("event-type", "", "Event type: "+strings.Join(asc.ValidAppEventBadges, ", "))
	start := fs.String("start", "", "Event start time (RFC3339)")
	end := fs.String("end", "", "Event end time (RFC3339)")
	publishStart := fs.String("publish-start", "", "Publish start time (RFC3339)")
	territories := fs.String("territories", "", "Territory codes (comma-separated)")
	deepLink := fs.String("deep-link", "", "Deep link URL")
	purchaseRequirement := fs.String("purchase-requirement", "", "Purchase requirement (currently supported: "+supportedAppEventPurchaseRequirementValues()+")")
	primaryLocale := fs.String("primary-locale", "", "Primary locale (e.g., en-US)")
	priority := fs.String("priority", "", "Priority: "+strings.Join(asc.ValidAppEventPriorities, ", "))
	purpose := fs.String("purpose", "", "Purpose: "+strings.Join(asc.ValidAppEventPurposes, ", "))
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc app-events update [flags]",
		ShortHelp:  "Update an in-app event.",
		LongHelp: `Update an in-app event.

Examples:
  asc app-events update --event-id "EVENT_ID" --priority HIGH
  asc app-events update --event-id "EVENT_ID" --name "New Name" --event-type SPECIAL_EVENT
  asc app-events update --event-id "EVENT_ID" --purchase-requirement NO_COST_ASSOCIATED --primary-locale en-US`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*eventID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --event-id is required")
				return flag.ErrHelp
			}

			var (
				attrs     asc.AppEventUpdateAttributes
				hasUpdate bool
			)

			if strings.TrimSpace(*name) != "" {
				value := strings.TrimSpace(*name)
				attrs.ReferenceName = &value
				hasUpdate = true
			}

			if strings.TrimSpace(*eventType) != "" {
				normalized, err := normalizeAppEventBadge(*eventType, false)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err.Error())
					return flag.ErrHelp
				}
				if normalized != "" {
					attrs.Badge = &normalized
					hasUpdate = true
				}
			}

			if strings.TrimSpace(*deepLink) != "" {
				value := strings.TrimSpace(*deepLink)
				attrs.DeepLink = &value
				hasUpdate = true
			}

			normalizedPurchaseRequirement, err := normalizeAppEventPurchaseRequirement(*purchaseRequirement)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err.Error())
				return flag.ErrHelp
			}
			if err := validateAppEventPurchaseRequirement(normalizedPurchaseRequirement); err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err.Error())
				return flag.ErrHelp
			}
			if normalizedPurchaseRequirement != "" {
				value := normalizedPurchaseRequirement
				attrs.PurchaseRequirement = &value
				hasUpdate = true
			}

			if strings.TrimSpace(*primaryLocale) != "" {
				value := strings.TrimSpace(*primaryLocale)
				attrs.PrimaryLocale = &value
				hasUpdate = true
			}

			if strings.TrimSpace(*priority) != "" {
				normalized, err := normalizeAppEventPriority(*priority)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err.Error())
					return flag.ErrHelp
				}
				if normalized != "" {
					attrs.Priority = &normalized
					hasUpdate = true
				}
			}

			if strings.TrimSpace(*purpose) != "" {
				normalized, err := normalizeAppEventPurpose(*purpose)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err.Error())
					return flag.ErrHelp
				}
				if normalized != "" {
					attrs.Purpose = &normalized
					hasUpdate = true
				}
			}

			schedule, scheduleProvided, err := normalizeAppEventTerritorySchedule(*start, *end, *publishStart, *territories)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err.Error())
				return flag.ErrHelp
			}
			if scheduleProvided {
				attrs.TerritorySchedules = []asc.AppEventTerritorySchedule{schedule}
				hasUpdate = true
			}

			if !hasUpdate {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			client, err := appEventsClientFactory()
			if err != nil {
				return fmt.Errorf("app-events update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateAppEvent(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("app-events update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppEventsDeleteCommand returns the app events delete subcommand.
func AppEventsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	eventID := fs.String("event-id", "", "App event ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc app-events delete --event-id \"EVENT_ID\" --confirm",
		ShortHelp:  "Delete an in-app event.",
		LongHelp: `Delete an in-app event.

Examples:
  asc app-events delete --event-id "EVENT_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*eventID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --event-id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := appEventsClientFactory()
			if err != nil {
				return fmt.Errorf("app-events delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteAppEvent(requestCtx, id); err != nil {
				return fmt.Errorf("app-events delete: failed to delete: %w", err)
			}

			result := &asc.AppEventDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
