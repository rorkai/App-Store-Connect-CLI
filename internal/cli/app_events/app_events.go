package app_events

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// NewAppEventsCommand returns the app events command group.
func NewAppEventsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("app-events", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app-events",
		ShortUsage: "asc app-events <subcommand> [flags]",
		ShortHelp:  "Manage App Store in-app events.",
		LongHelp: `Manage App Store in-app events.

Examples:
  asc app-events list --app "APP_ID"
  asc app-events view --event-id "EVENT_ID"
  asc app-events create --app "APP_ID" --name "Summer Challenge" --event-type CHALLENGE --start "2026-06-01T00:00:00Z" --end "2026-06-30T23:59:59Z" --territories all
  asc app-events update --event-id "EVENT_ID" --priority HIGH
  asc app-events delete --event-id "EVENT_ID" --confirm
  asc app-events links --event-id "EVENT_ID"`,
		FlagSet:   fs,
		UsageFunc: d.VisibleUsageFunc,
		Subcommands: []*ffcli.Command{
			newAppEventsListCommand(d),
			newAppEventsGetCommand(d),
			newAppEventsCreateCommand(d),
			newAppEventsUpdateCommand(d),
			newAppEventsDeleteCommand(d),
			newAppEventLocalizationsCommand(d),
			newAppEventsRelationshipsCommand(d),
			newAppEventScreenshotsCommand(d),
			newAppEventVideoClipsCommand(d),
			newAppEventsSubmitCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newAppEventsListCommand returns the app events list subcommand.
func newAppEventsListCommand(d *shared.Deps) *ffcli.Command {
	var (
		ids                                                              *string
		localizationsLimit                                               *int
		stateValues, fieldValues, localizationFieldValues, includeValues []string
	)

	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc app-events list [flags]",
		ShortHelp:  "List in-app events for an app.",
		LongHelp: `List in-app events for an app.

Examples:
  asc app-events list --app "APP_ID"
  asc app-events list --app "APP_ID" --event-state PUBLISHED --include localizations --localizations-limit 10
  asc app-events list --app "APP_ID" --paginate`,
		ParentFlag:           "app",
		ParentUsage:          "App Store Connect app ID (or ASC_APP_ID env)",
		ResolveApp:           true,
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		NextConflicts:        []string{"event-state", "id", "fields", "localization-fields", "include", "localizations-limit"},
		Deps:                 d,
		Flags: func(fs *flag.FlagSet) func() error {
			eventState := fs.String("event-state", "", "Filter by lifecycle state(s), comma-separated: "+strings.Join(appEventStateList(), ", "))
			ids = fs.String("id", "", "Filter by app event ID(s), comma-separated")
			fields := fs.String("fields", "", "Fields to include: "+strings.Join(appEventFieldsList(), ", "))
			localizationFields := fs.String("localization-fields", "", "Fields to include for localizations: "+strings.Join(appEventLocalizationFieldsList(), ", "))
			include := fs.String("include", "", "Include related resources: "+strings.Join(appEventIncludeList(), ", "))
			localizationsLimit = fs.Int("localizations-limit", 0, "Maximum included localizations (1-50)")
			return func() error {
				for _, selection := range []struct {
					name  string
					value string
				}{
					{name: "event-state", value: *eventState},
					{name: "id", value: *ids},
					{name: "fields", value: *fields},
					{name: "localization-fields", value: *localizationFields},
					{name: "include", value: *include},
				} {
					if !appEventFlagWasProvided(fs, selection.name) {
						continue
					}
					if err := validateAppEventCSV(selection.value, "--"+selection.name); err != nil {
						return d.UsageErrorf("app-events list: %v", err)
					}
				}
				localizationsLimitProvided := appEventFlagWasProvided(fs, "localizations-limit")
				if localizationsLimitProvided && (*localizationsLimit < 1 || *localizationsLimit > 50) {
					return d.UsageErrorf("app-events list: --localizations-limit must be between 1 and 50")
				}

				var err error
				stateValues, err = normalizeAppEventStates(*eventState)
				if err != nil {
					return d.UsageErrorf("app-events list: %v", err)
				}
				fieldValues, err = shared.NormalizeSelection(*fields, appEventFieldsList(), "--fields")
				if err != nil {
					return d.UsageErrorf("app-events list: %v", err)
				}
				localizationFieldValues, err = shared.NormalizeSelection(*localizationFields, appEventLocalizationFieldsList(), "--localization-fields")
				if err != nil {
					return d.UsageErrorf("app-events list: %v", err)
				}
				includeValues, err = shared.NormalizeSelection(*include, appEventIncludeList(), "--include")
				if err != nil {
					return d.UsageErrorf("app-events list: %v", err)
				}
				if len(localizationFieldValues) > 0 && !shared.HasInclude(includeValues, "localizations") {
					return d.UsageError("app-events list: --localization-fields requires --include localizations")
				}
				if *localizationsLimit > 0 && !shared.HasInclude(includeValues, "localizations") {
					return d.UsageError("app-events list: --localizations-limit requires --include localizations")
				}
				return nil
			}
		},
		FetchPage: func(ctx context.Context, client *asc.Client, appID string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetAppEvents(ctx, appID,
				asc.WithAppEventsStates(stateValues),
				asc.WithAppEventsIDs(shared.SplitCSV(*ids)),
				asc.WithAppEventsFields(fieldValues),
				asc.WithAppEventsLocalizationFields(localizationFieldValues),
				asc.WithAppEventsInclude(includeValues),
				asc.WithAppEventsLocalizationsLimit(*localizationsLimit),
				asc.Limit[asc.AppEventsOption](limit),
				asc.NextURL[asc.AppEventsOption](next),
			)
		},
	})
}

// newAppEventsGetCommand returns the app events get subcommand.
func newAppEventsGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc app-events view --event-id \"EVENT_ID\"",
		ShortHelp:  "View an in-app event by ID.",
		LongHelp: `View an in-app event by ID.

Examples:
  asc app-events view --event-id "EVENT_ID"`,
		IDFlag:             "event-id",
		IDUsage:            "App event ID",
		IDType:             "appEvents",
		RequiredDiagnostic: true,
		Deps:               d,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetAppEvent(ctx, id)
		},
	})
}

// newAppEventsCreateCommand returns the app events create subcommand.
func newAppEventsCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	name := fs.String("name", "", "Reference name")
	eventType := fs.String("event-type", "", "Event type: "+strings.Join(asc.ValidAppEventBadges, ", "))
	start := fs.String("start", "", "Event start time (RFC3339); events last 15 minutes to 31 days")
	end := fs.String("end", "", "Event end time (RFC3339)")
	publishStart := fs.String("publish-start", "", "Publish start time (RFC3339), at most 14 days before --start and no later than --end")
	territories := shared.BindOnceCSVFlag(fs, "territories", "Territory codes (comma-separated; required with a schedule), or all for every territory the app is available in")
	deepLink := fs.String("deep-link", "", "URL that Open on the event card leads to; App Store Connect asks for one before review")
	purchaseRequirement := fs.String("purchase-requirement", "", "Purchase requirement (currently supported: "+supportedAppEventPurchaseRequirementValues()+")")
	primaryLocale := fs.String("primary-locale", "", "Primary locale (e.g., en-US)")
	priority := fs.String("priority", "", "Priority: "+strings.Join(asc.ValidAppEventPriorities, ", "))
	purpose := fs.String("purpose", "", "Purpose: "+strings.Join(asc.ValidAppEventPurposes, ", "))
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc app-events create [flags]",
		ShortHelp:  "Create a new in-app event.",
		LongHelp: `Create a new in-app event.

Examples:
  asc app-events create --app "APP_ID" --name "Summer Challenge"
  asc app-events create --app "APP_ID" --name "Summer Challenge" --event-type CHALLENGE --start "2026-06-01T00:00:00Z" --end "2026-06-30T23:59:59Z" --territories all
  asc app-events create --app "APP_ID" --name "Launch Party" --event-type PREMIERE --priority HIGH --purpose ATTRACT_NEW_USERS
  asc app-events create --app "APP_ID" --name "Retro Challenge" --event-type LIVE_EVENT --priority HIGH --purpose APPROPRIATE_FOR_ALL_USERS`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := d.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}

			nameValue := strings.TrimSpace(*name)
			if nameValue == "" {
				return d.RequiredFlag("--name")
			}

			normalizedBadge, err := normalizeAppEventBadge(*eventType)
			if err != nil {
				fmt.Fprintln(d.IO.ErrOut, "Error:", err.Error())
				return flag.ErrHelp
			}

			normalizedPriority, err := normalizeAppEventPriority(*priority)
			if err != nil {
				fmt.Fprintln(d.IO.ErrOut, "Error:", err.Error())
				return flag.ErrHelp
			}

			normalizedPurpose, err := normalizeAppEventPurpose(*purpose)
			if err != nil {
				fmt.Fprintln(d.IO.ErrOut, "Error:", err.Error())
				return flag.ErrHelp
			}

			normalizedPurchaseRequirement, err := normalizeAppEventPurchaseRequirement(*purchaseRequirement)
			if err != nil {
				fmt.Fprintln(d.IO.ErrOut, "Error:", err.Error())
				return flag.ErrHelp
			}
			if err := validateAppEventPurchaseRequirement(normalizedPurchaseRequirement); err != nil {
				fmt.Fprintln(d.IO.ErrOut, "Error:", err.Error())
				return flag.ErrHelp
			}

			eventSchedule, err := normalizeAppEventTerritorySchedule(*start, *end, *publishStart, territories.String(), true)
			if err != nil {
				fmt.Fprintln(d.IO.ErrOut, "Error:", err.Error())
				return flag.ErrHelp
			}
			schedule, scheduleProvided := eventSchedule.schedule, eventSchedule.provided

			attrs := asc.AppEventCreateAttributes{
				ReferenceName:       nameValue,
				Badge:               normalizedBadge,
				DeepLink:            strings.TrimSpace(*deepLink),
				PurchaseRequirement: normalizedPurchaseRequirement,
				PrimaryLocale:       strings.TrimSpace(*primaryLocale),
				Priority:            normalizedPriority,
				Purpose:             normalizedPurpose,
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("app-events create: %w", err)
			}

			if eventSchedule.allTerritories {
				// Resolve the territories before creating anything, so a failed
				// lookup leaves no event behind.
				territoryCtx, cancelTerritories := d.Timeout(ctx)
				defer cancelTerritories()

				territoryIDs, err := shared.FetchAvailableTerritoryIDs(territoryCtx, client, resolvedAppID)
				if err != nil {
					return fmt.Errorf("app-events create: --territories all: %w", err)
				}
				if len(territoryIDs) == 0 {
					return fmt.Errorf("app-events create: --territories all: app %q is not available in any territory", resolvedAppID)
				}
				schedule.Territories = territoryIDs
			}

			createCtx, cancelCreate := d.Timeout(ctx)
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

				updateCtx, cancelUpdate := d.Timeout(ctx)
				defer cancelUpdate()

				resp, err = client.UpdateAppEvent(updateCtx, createdID, asc.AppEventUpdateAttributes{
					TerritorySchedules: []asc.AppEventTerritorySchedule{schedule},
				})
				if err != nil {
					verifyCtx, cancelVerify := d.Timeout(ctx)
					defer cancelVerify()

					verifiedResp, verifyErr := client.GetAppEvent(verifyCtx, createdID)
					if verifyErr == nil {
						if appEventHasTerritorySchedule(verifiedResp, schedule) {
							resp = verifiedResp
						} else {
							cleanupCtx, cancelCleanup := d.Timeout(ctx)
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

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newAppEventsUpdateCommand returns the app events update subcommand.
func newAppEventsUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("update", flag.ExitOnError)

	eventID := shared.BindResourceIDFlag(fs, "event-id", "appEvents", "App event ID")
	name := fs.String("name", "", "Reference name")
	eventType := fs.String("event-type", "", "Event type: "+strings.Join(asc.ValidAppEventBadges, ", "))
	start := fs.String("start", "", "Event start time (RFC3339); events last 15 minutes to 31 days")
	end := fs.String("end", "", "Event end time (RFC3339)")
	publishStart := fs.String("publish-start", "", "Publish start time (RFC3339), at most 14 days before --start and no later than --end")
	territories := shared.BindOnceCSVFlag(fs, "territories", "Territory codes (comma-separated; required with a schedule); all is supported only by create")
	deepLink := fs.String("deep-link", "", "URL that Open on the event card leads to; App Store Connect asks for one before review")
	purchaseRequirement := fs.String("purchase-requirement", "", "Purchase requirement (currently supported: "+supportedAppEventPurchaseRequirementValues()+")")
	primaryLocale := fs.String("primary-locale", "", "Primary locale (e.g., en-US)")
	priority := fs.String("priority", "", "Priority: "+strings.Join(asc.ValidAppEventPriorities, ", "))
	purpose := fs.String("purpose", "", "Purpose: "+strings.Join(asc.ValidAppEventPurposes, ", "))
	output := d.BindOutputFlags(fs)

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
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*eventID)
			if id == "" {
				return d.RequiredFlag("--event-id")
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
				normalized, err := normalizeAppEventBadge(*eventType)
				if err != nil {
					fmt.Fprintln(d.IO.ErrOut, "Error:", err.Error())
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
				fmt.Fprintln(d.IO.ErrOut, "Error:", err.Error())
				return flag.ErrHelp
			}
			if err := validateAppEventPurchaseRequirement(normalizedPurchaseRequirement); err != nil {
				fmt.Fprintln(d.IO.ErrOut, "Error:", err.Error())
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
					fmt.Fprintln(d.IO.ErrOut, "Error:", err.Error())
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
					fmt.Fprintln(d.IO.ErrOut, "Error:", err.Error())
					return flag.ErrHelp
				}
				if normalized != "" {
					attrs.Purpose = &normalized
					hasUpdate = true
				}
			}

			eventSchedule, err := normalizeAppEventTerritorySchedule(*start, *end, *publishStart, territories.String(), false)
			if err != nil {
				fmt.Fprintln(d.IO.ErrOut, "Error:", err.Error())
				return flag.ErrHelp
			}
			if eventSchedule.provided {
				attrs.TerritorySchedules = []asc.AppEventTerritorySchedule{eventSchedule.schedule}
				hasUpdate = true
			}

			if !hasUpdate {
				return d.UpdateFlagRequired(fs, name, eventType, start, end, publishStart, territories, deepLink, purchaseRequirement, primaryLocale, priority, purpose)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("app-events update: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.UpdateAppEvent(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("app-events update: failed to update: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newAppEventsDeleteCommand returns the app events delete subcommand.
func newAppEventsDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Name:       "delete",
		ShortUsage: "asc app-events delete --event-id \"EVENT_ID\" --confirm",
		ShortHelp:  "Delete an in-app event.",
		LongHelp: `Delete an in-app event.

Examples:
  asc app-events delete --event-id "EVENT_ID" --confirm`,
		IDFlag:             "event-id",
		IDUsage:            "App event ID",
		IDType:             "appEvents",
		RequiredDiagnostic: true,
		Deps:               d,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteAppEvent(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
	})
}
