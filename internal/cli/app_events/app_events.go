package app_events

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

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
  asc app-events relationships --event-id "EVENT_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
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
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc app-events list [flags]",
		ShortHelp:  "List in-app events for an app.",
		LongHelp: `List in-app events for an app.

Examples:
  asc app-events list --app "APP_ID"
  asc app-events list --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("app-events list: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("app-events list: %w", err)
			}

			resolvedAppID := resolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("app-events list: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
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
				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetAppEvents(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("app-events list: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// AppEventsGetCommand returns the app events get subcommand.
func AppEventsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	eventID := fs.String("event-id", "", "App event ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc app-events get --event-id \"EVENT_ID\"",
		ShortHelp:  "Get an in-app event by ID.",
		LongHelp: `Get an in-app event by ID.

Examples:
  asc app-events get --event-id "EVENT_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*eventID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --event-id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("app-events get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppEvent(requestCtx, id)
			if err != nil {
				return fmt.Errorf("app-events get: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
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
	purchaseRequirement := fs.String("purchase-requirement", "", "Purchase requirement")
	primaryLocale := fs.String("primary-locale", "", "Primary locale (e.g., en-US)")
	priority := fs.String("priority", "", "Priority: "+strings.Join(asc.ValidAppEventPriorities, ", "))
	purpose := fs.String("purpose", "", "Purpose: "+strings.Join(asc.ValidAppEventPurposes, ", "))
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc app-events create [flags]",
		ShortHelp:  "Create a new in-app event.",
		LongHelp: `Create a new in-app event.

Examples:
  asc app-events create --app "APP_ID" --name "Summer Challenge" --event-type CHALLENGE --start "2026-06-01T00:00:00Z" --end "2026-06-30T23:59:59Z"
  asc app-events create --app "APP_ID" --name "Launch Party" --event-type PREMIERE --priority HIGH --purpose ATTRACT_NEW_USERS`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := resolveAppID(*appID)
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

			scheduleProvided := strings.TrimSpace(*start) != "" ||
				strings.TrimSpace(*end) != "" ||
				strings.TrimSpace(*publishStart) != "" ||
				strings.TrimSpace(*territories) != ""

			var schedules []asc.AppEventTerritorySchedule
			if scheduleProvided {
				startValue, err := normalizeRFC3339(*start, "--start", true)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err.Error())
					return flag.ErrHelp
				}
				endValue, err := normalizeRFC3339(*end, "--end", true)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err.Error())
					return flag.ErrHelp
				}
				publishValue, err := normalizeRFC3339(*publishStart, "--publish-start", false)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err.Error())
					return flag.ErrHelp
				}
				territoryValues := splitCSVUpper(*territories)
				schedule := buildAppEventTerritorySchedule(territoryValues, publishValue, startValue, endValue)
				schedules = []asc.AppEventTerritorySchedule{schedule}
			}

			attrs := asc.AppEventCreateAttributes{
				ReferenceName:       nameValue,
				Badge:               normalizedBadge,
				DeepLink:            strings.TrimSpace(*deepLink),
				PurchaseRequirement: strings.TrimSpace(*purchaseRequirement),
				PrimaryLocale:       strings.TrimSpace(*primaryLocale),
				Priority:            normalizedPriority,
				Purpose:             normalizedPurpose,
				TerritorySchedules:  schedules,
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("app-events create: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateAppEvent(requestCtx, resolvedAppID, attrs)
			if err != nil {
				return fmt.Errorf("app-events create: failed to create: %w", err)
			}

			return printOutput(resp, *output, *pretty)
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
	purchaseRequirement := fs.String("purchase-requirement", "", "Purchase requirement")
	primaryLocale := fs.String("primary-locale", "", "Primary locale (e.g., en-US)")
	priority := fs.String("priority", "", "Priority: "+strings.Join(asc.ValidAppEventPriorities, ", "))
	purpose := fs.String("purpose", "", "Purpose: "+strings.Join(asc.ValidAppEventPurposes, ", "))
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc app-events update [flags]",
		ShortHelp:  "Update an in-app event.",
		LongHelp: `Update an in-app event.

Examples:
  asc app-events update --event-id "EVENT_ID" --priority HIGH
  asc app-events update --event-id "EVENT_ID" --name "New Name" --event-type SPECIAL_EVENT`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
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

			if strings.TrimSpace(*purchaseRequirement) != "" {
				value := strings.TrimSpace(*purchaseRequirement)
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

			scheduleProvided := strings.TrimSpace(*start) != "" ||
				strings.TrimSpace(*end) != "" ||
				strings.TrimSpace(*publishStart) != "" ||
				strings.TrimSpace(*territories) != ""
			if scheduleProvided {
				startValue, err := normalizeRFC3339(*start, "--start", true)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err.Error())
					return flag.ErrHelp
				}
				endValue, err := normalizeRFC3339(*end, "--end", true)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err.Error())
					return flag.ErrHelp
				}
				publishValue, err := normalizeRFC3339(*publishStart, "--publish-start", false)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err.Error())
					return flag.ErrHelp
				}
				territoryValues := splitCSVUpper(*territories)
				schedule := buildAppEventTerritorySchedule(territoryValues, publishValue, startValue, endValue)
				attrs.TerritorySchedules = []asc.AppEventTerritorySchedule{schedule}
				hasUpdate = true
			}

			if !hasUpdate {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("app-events update: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateAppEvent(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("app-events update: failed to update: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// AppEventsDeleteCommand returns the app events delete subcommand.
func AppEventsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	eventID := fs.String("event-id", "", "App event ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc app-events delete --event-id \"EVENT_ID\" --confirm",
		ShortHelp:  "Delete an in-app event.",
		LongHelp: `Delete an in-app event.

Examples:
  asc app-events delete --event-id "EVENT_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
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

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("app-events delete: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteAppEvent(requestCtx, id); err != nil {
				return fmt.Errorf("app-events delete: failed to delete: %w", err)
			}

			result := &asc.AppEventDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return printOutput(result, *output, *pretty)
		},
	}
}
