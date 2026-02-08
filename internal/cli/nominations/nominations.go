package nominations

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// NominationsCommand returns the nominations command with subcommands.
func NominationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("nominations", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "nominations",
		ShortUsage: "asc nominations <subcommand> [flags]",
		ShortHelp:  "Manage featuring nominations.",
		LongHelp: `Manage featuring nominations.

Examples:
  asc nominations list --status DRAFT
  asc nominations get --id "NOMINATION_ID"
  asc nominations create --app "APP_ID" --name "Launch" --type APP_LAUNCH --description "New launch" --submitted=false --publish-start-date "2026-02-01T08:00:00Z"
  asc nominations update --id "NOMINATION_ID" --notes "Updated notes"
  asc nominations delete --id "NOMINATION_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			NominationsListCommand(),
			NominationsGetCommand(),
			NominationsCreateCommand(),
			NominationsUpdateCommand(),
			NominationsDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// NominationsListCommand returns the nominations list subcommand.
func NominationsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("nominations list", flag.ExitOnError)

	appIDs := fs.String("app", "", "Filter by related app ID(s), comma-separated")
	status := fs.String("status", "", "Filter by status/state(s), comma-separated: "+strings.Join(nominationStateList(), ", "))
	nomType := fs.String("type", "", "Filter by type(s), comma-separated: "+strings.Join(nominationTypeList(), ", "))
	sort := fs.String("sort", "", "Sort by: "+strings.Join(nominationSortList(), ", "))
	fields := fs.String("fields", "", "Fields to include: "+strings.Join(nominationFieldsList(), ", "))
	include := fs.String("include", "", "Include related resources: "+strings.Join(nominationIncludeList(), ", "))
	inAppEventsLimit := fs.Int("in-app-events-limit", 0, "Maximum included in-app events (1-50)")
	relatedAppsLimit := fs.Int("related-apps-limit", 0, "Maximum included related apps (1-50)")
	supportedTerritoriesLimit := fs.Int("supported-territories-limit", 0, "Maximum included supported territories (1-200)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc nominations list --status STATE [flags]",
		ShortHelp:  "List featuring nominations.",
		LongHelp: `List featuring nominations.

Examples:
  asc nominations list --status DRAFT
  asc nominations list --status DRAFT --type APP_LAUNCH
  asc nominations list --app "APP_ID" --status SUBMITTED --output table
  asc nominations list --include relatedApps --related-apps-limit 10`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("nominations list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("nominations list: %w", err)
			}
			if err := shared.ValidateSort(*sort, nominationSortList()...); err != nil {
				return fmt.Errorf("nominations list: %w", err)
			}
			if *inAppEventsLimit != 0 && (*inAppEventsLimit < 1 || *inAppEventsLimit > 50) {
				return fmt.Errorf("nominations list: --in-app-events-limit must be between 1 and 50")
			}
			if *relatedAppsLimit != 0 && (*relatedAppsLimit < 1 || *relatedAppsLimit > 50) {
				return fmt.Errorf("nominations list: --related-apps-limit must be between 1 and 50")
			}
			if *supportedTerritoriesLimit != 0 && (*supportedTerritoriesLimit < 1 || *supportedTerritoriesLimit > 200) {
				return fmt.Errorf("nominations list: --supported-territories-limit must be between 1 and 200")
			}

			statusValues, err := normalizeNominationStates(shared.SplitCSVUpper(*status))
			if err != nil {
				return fmt.Errorf("nominations list: %w", err)
			}
			if len(statusValues) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --status is required")
				return flag.ErrHelp
			}

			typeValues, err := normalizeNominationTypes(shared.SplitCSVUpper(*nomType))
			if err != nil {
				return fmt.Errorf("nominations list: %w", err)
			}

			fieldsValue, err := normalizeNominationFields(*fields)
			if err != nil {
				return fmt.Errorf("nominations list: %w", err)
			}

			includeValues, err := normalizeNominationInclude(*include)
			if err != nil {
				return fmt.Errorf("nominations list: %w", err)
			}

			if *inAppEventsLimit != 0 && !shared.HasInclude(includeValues, "inAppEvents") {
				fmt.Fprintf(os.Stderr, "Error: --in-app-events-limit requires --include inAppEvents\n\n")
				return flag.ErrHelp
			}
			if *relatedAppsLimit != 0 && !shared.HasInclude(includeValues, "relatedApps") {
				fmt.Fprintf(os.Stderr, "Error: --related-apps-limit requires --include relatedApps\n\n")
				return flag.ErrHelp
			}
			if *supportedTerritoriesLimit != 0 && !shared.HasInclude(includeValues, "supportedTerritories") {
				fmt.Fprintf(os.Stderr, "Error: --supported-territories-limit requires --include supportedTerritories\n\n")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("nominations list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.NominationsOption{
				asc.WithNominationsTypes(typeValues),
				asc.WithNominationsStates(statusValues),
				asc.WithNominationsRelatedApps(shared.SplitCSV(*appIDs)),
				asc.WithNominationsLimit(*limit),
				asc.WithNominationsNextURL(*next),
			}
			if strings.TrimSpace(*sort) != "" {
				opts = append(opts, asc.WithNominationsSort(*sort))
			}
			if len(fieldsValue) > 0 {
				opts = append(opts, asc.WithNominationsFields(fieldsValue))
			}
			if len(includeValues) > 0 {
				opts = append(opts, asc.WithNominationsInclude(includeValues))
			}
			if *inAppEventsLimit > 0 {
				opts = append(opts, asc.WithNominationsInAppEventsLimit(*inAppEventsLimit))
			}
			if *relatedAppsLimit > 0 {
				opts = append(opts, asc.WithNominationsRelatedAppsLimit(*relatedAppsLimit))
			}
			if *supportedTerritoriesLimit > 0 {
				opts = append(opts, asc.WithNominationsSupportedTerritoriesLimit(*supportedTerritoriesLimit))
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithNominationsLimit(200))
				firstPage, err := client.GetNominations(requestCtx, paginateOpts...)
				if err != nil {
					return fmt.Errorf("nominations list: failed to fetch: %w", err)
				}

				nominations, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetNominations(ctx, asc.WithNominationsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("nominations list: %w", err)
				}

				return shared.PrintOutput(nominations, *output, *pretty)
			}

			resp, err := client.GetNominations(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("nominations list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// NominationsGetCommand returns the nominations get subcommand.
func NominationsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("nominations get", flag.ExitOnError)

	nominationID := fs.String("id", "", "Nomination ID (required)")
	fields := fs.String("fields", "", "Fields to include: "+strings.Join(nominationFieldsList(), ", "))
	include := fs.String("include", "", "Include related resources: "+strings.Join(nominationIncludeList(), ", "))
	inAppEventsLimit := fs.Int("in-app-events-limit", 0, "Maximum included in-app events (1-50)")
	relatedAppsLimit := fs.Int("related-apps-limit", 0, "Maximum included related apps (1-50)")
	supportedTerritoriesLimit := fs.Int("supported-territories-limit", 0, "Maximum included supported territories (1-200)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc nominations get --id NOMINATION_ID [flags]",
		ShortHelp:  "Get a featuring nomination by ID.",
		LongHelp: `Get a featuring nomination by ID.

Examples:
  asc nominations get --id "NOMINATION_ID"
  asc nominations get --id "NOMINATION_ID" --include relatedApps`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*nominationID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if *inAppEventsLimit != 0 && (*inAppEventsLimit < 1 || *inAppEventsLimit > 50) {
				return fmt.Errorf("nominations get: --in-app-events-limit must be between 1 and 50")
			}
			if *relatedAppsLimit != 0 && (*relatedAppsLimit < 1 || *relatedAppsLimit > 50) {
				return fmt.Errorf("nominations get: --related-apps-limit must be between 1 and 50")
			}
			if *supportedTerritoriesLimit != 0 && (*supportedTerritoriesLimit < 1 || *supportedTerritoriesLimit > 200) {
				return fmt.Errorf("nominations get: --supported-territories-limit must be between 1 and 200")
			}

			fieldsValue, err := normalizeNominationFields(*fields)
			if err != nil {
				return fmt.Errorf("nominations get: %w", err)
			}

			includeValues, err := normalizeNominationInclude(*include)
			if err != nil {
				return fmt.Errorf("nominations get: %w", err)
			}

			if *inAppEventsLimit != 0 && !shared.HasInclude(includeValues, "inAppEvents") {
				fmt.Fprintf(os.Stderr, "Error: --in-app-events-limit requires --include inAppEvents\n\n")
				return flag.ErrHelp
			}
			if *relatedAppsLimit != 0 && !shared.HasInclude(includeValues, "relatedApps") {
				fmt.Fprintf(os.Stderr, "Error: --related-apps-limit requires --include relatedApps\n\n")
				return flag.ErrHelp
			}
			if *supportedTerritoriesLimit != 0 && !shared.HasInclude(includeValues, "supportedTerritories") {
				fmt.Fprintf(os.Stderr, "Error: --supported-territories-limit requires --include supportedTerritories\n\n")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("nominations get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.NominationsOption{}
			if len(fieldsValue) > 0 {
				opts = append(opts, asc.WithNominationsFields(fieldsValue))
			}
			if len(includeValues) > 0 {
				opts = append(opts, asc.WithNominationsInclude(includeValues))
			}
			if *inAppEventsLimit > 0 {
				opts = append(opts, asc.WithNominationsInAppEventsLimit(*inAppEventsLimit))
			}
			if *relatedAppsLimit > 0 {
				opts = append(opts, asc.WithNominationsRelatedAppsLimit(*relatedAppsLimit))
			}
			if *supportedTerritoriesLimit > 0 {
				opts = append(opts, asc.WithNominationsSupportedTerritoriesLimit(*supportedTerritoriesLimit))
			}

			resp, err := client.GetNomination(requestCtx, trimmedID, opts...)
			if err != nil {
				return fmt.Errorf("nominations get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// NominationsCreateCommand returns the nominations create subcommand.
func NominationsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("nominations create", flag.ExitOnError)

	appID := fs.String("app", "", "Related app ID(s), comma-separated (or ASC_APP_ID)")
	name := fs.String("name", "", "Nomination name (required)")
	nomType := fs.String("type", "", "Nomination type (required): "+strings.Join(nominationTypeList(), ", "))
	description := fs.String("description", "", "Nomination description (required)")
	submitted := fs.Bool("submitted", false, "Submit nomination now (true/false)")
	publishStartDate := fs.String("publish-start-date", "", "Publish start date (RFC3339, required)")
	publishEndDate := fs.String("publish-end-date", "", "Publish end date (RFC3339)")
	deviceFamilies := fs.String("device-families", "", "Device families, comma-separated: "+strings.Join(nominationDeviceFamilyList(), ", "))
	locales := fs.String("locales", "", "Locales, comma-separated")
	supplementalMaterialsURIs := fs.String("supplemental-materials-uris", "", "Supplemental material URIs, comma-separated")
	hasInAppEvents := fs.Bool("has-in-app-events", false, "Indicate in-app events are included")
	launchInSelectMarketsFirst := fs.Bool("launch-in-select-markets-first", false, "Launch in select markets first")
	notes := fs.String("notes", "", "Internal notes")
	preOrderEnabled := fs.Bool("pre-order-enabled", false, "Enable pre-order")
	inAppEvents := fs.String("in-app-events", "", "In-app event IDs, comma-separated")
	supportedTerritories := fs.String("supported-territories", "", "Supported territory IDs, comma-separated")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc nominations create --app APP_ID --name NAME --type TYPE --description DESC --submitted [true|false] --publish-start-date RFC3339 [flags]",
		ShortHelp:  "Create a featuring nomination.",
		LongHelp: `Create a featuring nomination.

Examples:
  asc nominations create --app "APP_ID" --name "Launch" --type APP_LAUNCH --description "New launch" --submitted=false --publish-start-date "2026-02-01T08:00:00Z"
  asc nominations create --app "APP_ID" --name "Update" --type APP_ENHANCEMENTS --description "Major update" --submitted=true --publish-start-date "2026-03-01T08:00:00Z" --publish-end-date "2026-04-01T08:00:00Z"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})

			relatedApps := shared.SplitCSV(shared.ResolveAppID(*appID))
			if len(relatedApps) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			trimmedName := strings.TrimSpace(*name)
			if trimmedName == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return flag.ErrHelp
			}

			trimmedDescription := strings.TrimSpace(*description)
			if trimmedDescription == "" {
				fmt.Fprintln(os.Stderr, "Error: --description is required")
				return flag.ErrHelp
			}

			if !visited["submitted"] {
				fmt.Fprintln(os.Stderr, "Error: --submitted is required")
				return flag.ErrHelp
			}

			if strings.TrimSpace(*nomType) == "" {
				fmt.Fprintln(os.Stderr, "Error: --type is required")
				return flag.ErrHelp
			}

			normalizedType, err := normalizeNominationType(*nomType)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				return flag.ErrHelp
			}

			normalizedPublishStart, err := normalizeNominationPublishDate("--publish-start-date", *publishStartDate, true)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				return flag.ErrHelp
			}

			deviceFamilyValues, err := normalizeNominationDeviceFamilies(shared.SplitCSVUpper(*deviceFamilies))
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				return flag.ErrHelp
			}

			normalizedPublishEnd, err := normalizeNominationPublishDate("--publish-end-date", *publishEndDate, false)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("nominations create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.NominationCreateAttributes{
				Name:             trimmedName,
				Type:             asc.NominationType(normalizedType),
				Description:      trimmedDescription,
				Submitted:        *submitted,
				PublishStartDate: normalizedPublishStart,
			}
			if normalizedPublishEnd != "" {
				attrs.PublishEndDate = &normalizedPublishEnd
			}
			if len(deviceFamilyValues) > 0 {
				attrs.DeviceFamilies = normalizeNominationDeviceFamilyAttributes(deviceFamilyValues)
			}
			if localesValue := shared.SplitCSV(*locales); len(localesValue) > 0 {
				attrs.Locales = localesValue
			}
			if supplementalValue := shared.SplitCSV(*supplementalMaterialsURIs); len(supplementalValue) > 0 {
				attrs.SupplementalMaterialsURIs = supplementalValue
			}
			if visited["has-in-app-events"] {
				value := *hasInAppEvents
				attrs.HasInAppEvents = &value
			}
			if visited["launch-in-select-markets-first"] {
				value := *launchInSelectMarketsFirst
				attrs.LaunchInSelectMarketsFirst = &value
			}
			if visited["notes"] {
				value := strings.TrimSpace(*notes)
				attrs.Notes = &value
			}
			if visited["pre-order-enabled"] {
				value := *preOrderEnabled
				attrs.PreOrderEnabled = &value
			}

			relationships := asc.NominationRelationships{
				RelatedApps: buildNominationRelationshipList(asc.ResourceTypeApps, relatedApps),
			}
			if inAppEventIDs := shared.SplitCSV(*inAppEvents); len(inAppEventIDs) > 0 {
				relationships.InAppEvents = buildNominationRelationshipList(asc.ResourceTypeAppEvents, inAppEventIDs)
			}
			if territoryIDs := shared.SplitCSV(*supportedTerritories); len(territoryIDs) > 0 {
				relationships.SupportedTerritories = buildNominationRelationshipList(asc.ResourceTypeTerritories, territoryIDs)
			}

			resp, err := client.CreateNomination(requestCtx, attrs, relationships)
			if err != nil {
				return fmt.Errorf("nominations create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// NominationsUpdateCommand returns the nominations update subcommand.
func NominationsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("nominations update", flag.ExitOnError)

	nominationID := fs.String("id", "", "Nomination ID (required)")
	name := fs.String("name", "", "Nomination name")
	nomType := fs.String("type", "", "Nomination type: "+strings.Join(nominationTypeList(), ", "))
	description := fs.String("description", "", "Nomination description")
	submitted := fs.Bool("submitted", false, "Submit nomination now (true/false)")
	archived := fs.Bool("archived", false, "Archive nomination (true/false)")
	publishStartDate := fs.String("publish-start-date", "", "Publish start date (RFC3339)")
	publishEndDate := fs.String("publish-end-date", "", "Publish end date (RFC3339)")
	deviceFamilies := fs.String("device-families", "", "Device families, comma-separated: "+strings.Join(nominationDeviceFamilyList(), ", "))
	locales := fs.String("locales", "", "Locales, comma-separated")
	supplementalMaterialsURIs := fs.String("supplemental-materials-uris", "", "Supplemental material URIs, comma-separated")
	hasInAppEvents := fs.Bool("has-in-app-events", false, "Indicate in-app events are included")
	launchInSelectMarketsFirst := fs.Bool("launch-in-select-markets-first", false, "Launch in select markets first")
	notes := fs.String("notes", "", "Internal notes")
	preOrderEnabled := fs.Bool("pre-order-enabled", false, "Enable pre-order")
	appIDs := fs.String("app", "", "Replace related app ID(s), comma-separated")
	inAppEvents := fs.String("in-app-events", "", "Replace in-app event IDs, comma-separated")
	supportedTerritories := fs.String("supported-territories", "", "Replace supported territory IDs, comma-separated")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc nominations update --id NOMINATION_ID --submitted [true|false] [flags]",
		ShortHelp:  "Update a featuring nomination.",
		LongHelp: `Update a featuring nomination.

Note: --submitted or --archived is required by the API.

Examples:
  asc nominations update --id "NOMINATION_ID" --notes "Updated notes"
  asc nominations update --id "NOMINATION_ID" --type NEW_CONTENT --publish-start-date "2026-03-01T08:00:00Z"
  asc nominations update --id "NOMINATION_ID" --archived=true`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*nominationID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})

			hasAttributeUpdates := visited["name"] ||
				visited["type"] ||
				visited["description"] ||
				visited["submitted"] ||
				visited["archived"] ||
				visited["publish-start-date"] ||
				visited["publish-end-date"] ||
				visited["device-families"] ||
				visited["locales"] ||
				visited["supplemental-materials-uris"] ||
				visited["has-in-app-events"] ||
				visited["launch-in-select-markets-first"] ||
				visited["notes"] ||
				visited["pre-order-enabled"]
			hasRelationshipUpdates := visited["app"] || visited["in-app-events"] || visited["supported-territories"]

			if !hasAttributeUpdates && !hasRelationshipUpdates {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}
			if !visited["submitted"] && !visited["archived"] {
				fmt.Fprintln(os.Stderr, "Error: --submitted or --archived is required")
				return flag.ErrHelp
			}

			var attrs *asc.NominationUpdateAttributes
			if hasAttributeUpdates {
				attrsValue := asc.NominationUpdateAttributes{}
				if visited["name"] {
					value := strings.TrimSpace(*name)
					attrsValue.Name = &value
				}
				if visited["type"] {
					if strings.TrimSpace(*nomType) == "" {
						fmt.Fprintln(os.Stderr, "Error: --type is required")
						return flag.ErrHelp
					}
					normalized, err := normalizeNominationType(*nomType)
					if err != nil {
						fmt.Fprintln(os.Stderr, "Error:", err)
						return flag.ErrHelp
					}
					nomTypeValue := asc.NominationType(normalized)
					attrsValue.Type = &nomTypeValue
				}
				if visited["description"] {
					value := strings.TrimSpace(*description)
					attrsValue.Description = &value
				}
				if visited["submitted"] {
					value := *submitted
					attrsValue.Submitted = &value
				}
				if visited["archived"] {
					value := *archived
					attrsValue.Archived = &value
				}
				if visited["publish-start-date"] {
					normalized, err := normalizeNominationPublishDate("--publish-start-date", *publishStartDate, true)
					if err != nil {
						fmt.Fprintln(os.Stderr, "Error:", err)
						return flag.ErrHelp
					}
					attrsValue.PublishStartDate = &normalized
				}
				if visited["publish-end-date"] {
					normalized, err := normalizeNominationPublishDate("--publish-end-date", *publishEndDate, true)
					if err != nil {
						fmt.Fprintln(os.Stderr, "Error:", err)
						return flag.ErrHelp
					}
					attrsValue.PublishEndDate = &normalized
				}
				if visited["device-families"] {
					deviceFamilyValues, err := normalizeNominationDeviceFamilies(shared.SplitCSVUpper(*deviceFamilies))
					if err != nil {
						fmt.Fprintln(os.Stderr, "Error:", err)
						return flag.ErrHelp
					}
					if len(deviceFamilyValues) == 0 {
						return fmt.Errorf("nominations update: --device-families is required")
					}
					attrsValue.DeviceFamilies = normalizeNominationDeviceFamilyAttributes(deviceFamilyValues)
				}
				if visited["locales"] {
					localesValue := shared.SplitCSV(*locales)
					if len(localesValue) == 0 {
						return fmt.Errorf("nominations update: --locales is required")
					}
					attrsValue.Locales = localesValue
				}
				if visited["supplemental-materials-uris"] {
					supplementalValue := shared.SplitCSV(*supplementalMaterialsURIs)
					if len(supplementalValue) == 0 {
						return fmt.Errorf("nominations update: --supplemental-materials-uris is required")
					}
					attrsValue.SupplementalMaterialsURIs = supplementalValue
				}
				if visited["has-in-app-events"] {
					value := *hasInAppEvents
					attrsValue.HasInAppEvents = &value
				}
				if visited["launch-in-select-markets-first"] {
					value := *launchInSelectMarketsFirst
					attrsValue.LaunchInSelectMarketsFirst = &value
				}
				if visited["notes"] {
					value := strings.TrimSpace(*notes)
					attrsValue.Notes = &value
				}
				if visited["pre-order-enabled"] {
					value := *preOrderEnabled
					attrsValue.PreOrderEnabled = &value
				}
				attrs = &attrsValue
			}

			var relationships *asc.NominationRelationships
			if hasRelationshipUpdates {
				relationshipValue := asc.NominationRelationships{}
				if visited["app"] {
					appValues := shared.SplitCSV(*appIDs)
					if len(appValues) == 0 {
						return fmt.Errorf("nominations update: --app is required")
					}
					relationshipValue.RelatedApps = buildNominationRelationshipList(asc.ResourceTypeApps, appValues)
				}
				if visited["in-app-events"] {
					eventValues := shared.SplitCSV(*inAppEvents)
					if len(eventValues) == 0 {
						return fmt.Errorf("nominations update: --in-app-events is required")
					}
					relationshipValue.InAppEvents = buildNominationRelationshipList(asc.ResourceTypeAppEvents, eventValues)
				}
				if visited["supported-territories"] {
					territoryValues := shared.SplitCSV(*supportedTerritories)
					if len(territoryValues) == 0 {
						return fmt.Errorf("nominations update: --supported-territories is required")
					}
					relationshipValue.SupportedTerritories = buildNominationRelationshipList(asc.ResourceTypeTerritories, territoryValues)
				}
				relationships = &relationshipValue
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("nominations update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateNomination(requestCtx, trimmedID, attrs, relationships)
			if err != nil {
				return fmt.Errorf("nominations update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// NominationsDeleteCommand returns the nominations delete subcommand.
func NominationsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("nominations delete", flag.ExitOnError)

	nominationID := fs.String("id", "", "Nomination ID (required)")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc nominations delete --id NOMINATION_ID --confirm",
		ShortHelp:  "Delete a featuring nomination.",
		LongHelp: `Delete a featuring nomination.

Examples:
  asc nominations delete --id "NOMINATION_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*nominationID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required to delete")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("nominations delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteNomination(requestCtx, trimmedID); err != nil {
				return fmt.Errorf("nominations delete: failed to delete: %w", err)
			}

			result := &asc.NominationDeleteResult{
				ID:      trimmedID,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}

func normalizeNominationType(value string) (string, error) {
	trimmed := strings.ToUpper(strings.TrimSpace(value))
	if trimmed == "" {
		return "", fmt.Errorf("--type is required")
	}
	if _, ok := nominationTypes[trimmed]; !ok {
		return "", fmt.Errorf("--type must be one of: %s", strings.Join(nominationTypeList(), ", "))
	}
	return trimmed, nil
}

func normalizeNominationTypes(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	for _, value := range values {
		if _, ok := nominationTypes[value]; !ok {
			return nil, fmt.Errorf("--type must be one of: %s", strings.Join(nominationTypeList(), ", "))
		}
	}
	return values, nil
}

func normalizeNominationStates(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	for _, value := range values {
		if _, ok := nominationStates[value]; !ok {
			return nil, fmt.Errorf("--status must be one of: %s", strings.Join(nominationStateList(), ", "))
		}
	}
	return values, nil
}

func normalizeNominationFields(value string) ([]string, error) {
	fields := shared.SplitCSV(value)
	if len(fields) == 0 {
		return nil, nil
	}

	allowed := map[string]struct{}{}
	for _, field := range nominationFieldsList() {
		allowed[field] = struct{}{}
	}
	for _, field := range fields {
		if _, ok := allowed[field]; !ok {
			return nil, fmt.Errorf("--fields must be one of: %s", strings.Join(nominationFieldsList(), ", "))
		}
	}

	return fields, nil
}

func normalizeNominationInclude(value string) ([]string, error) {
	values := shared.SplitCSV(value)
	if len(values) == 0 {
		return nil, nil
	}

	allowed := map[string]struct{}{}
	for _, include := range nominationIncludeList() {
		allowed[include] = struct{}{}
	}
	for _, include := range values {
		if _, ok := allowed[include]; !ok {
			return nil, fmt.Errorf("--include must be one of: %s", strings.Join(nominationIncludeList(), ", "))
		}
	}

	return values, nil
}

func normalizeNominationDeviceFamilies(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	for _, value := range values {
		if _, ok := nominationDeviceFamilies[value]; !ok {
			return nil, fmt.Errorf("--device-families must be one of: %s", strings.Join(nominationDeviceFamilyList(), ", "))
		}
	}
	return values, nil
}

func normalizeNominationPublishDate(flagName, value string, required bool) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		if required {
			return "", fmt.Errorf("%s is required", flagName)
		}
		return "", nil
	}
	if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return parsed.Format(time.RFC3339), nil
	}
	if parsed, err := time.Parse(time.RFC3339Nano, trimmed); err == nil {
		return parsed.Format(time.RFC3339Nano), nil
	}
	return "", fmt.Errorf("%s must be in RFC3339 format (e.g., 2026-02-01T08:00:00Z)", flagName)
}

func normalizeNominationDeviceFamilyAttributes(values []string) []asc.DeviceFamily {
	families := make([]asc.DeviceFamily, 0, len(values))
	for _, value := range values {
		families = append(families, asc.DeviceFamily(value))
	}
	return families
}

func buildNominationRelationshipList(resourceType asc.ResourceType, ids []string) *asc.RelationshipList {
	if len(ids) == 0 {
		return nil
	}
	data := make([]asc.ResourceData, 0, len(ids))
	for _, id := range ids {
		data = append(data, asc.ResourceData{
			Type: resourceType,
			ID:   id,
		})
	}
	return &asc.RelationshipList{Data: data}
}

var nominationTypes = map[string]struct{}{
	"APP_LAUNCH":       {},
	"APP_ENHANCEMENTS": {},
	"NEW_CONTENT":      {},
}

var nominationStates = map[string]struct{}{
	"DRAFT":     {},
	"SUBMITTED": {},
	"ARCHIVED":  {},
}

var nominationDeviceFamilies = map[string]struct{}{
	"IPHONE":      {},
	"IPAD":        {},
	"APPLE_TV":    {},
	"APPLE_WATCH": {},
	"MAC":         {},
	"VISION":      {},
}

func nominationTypeList() []string {
	return []string{"APP_LAUNCH", "APP_ENHANCEMENTS", "NEW_CONTENT"}
}

func nominationStateList() []string {
	return []string{"DRAFT", "SUBMITTED", "ARCHIVED"}
}

func nominationSortList() []string {
	return []string{
		"lastModifiedDate",
		"-lastModifiedDate",
		"publishStartDate",
		"-publishStartDate",
		"publishEndDate",
		"-publishEndDate",
		"name",
		"-name",
		"type",
		"-type",
	}
}

func nominationFieldsList() []string {
	return []string{
		"name",
		"type",
		"description",
		"createdDate",
		"lastModifiedDate",
		"submittedDate",
		"state",
		"publishStartDate",
		"publishEndDate",
		"deviceFamilies",
		"locales",
		"supplementalMaterialsUris",
		"hasInAppEvents",
		"launchInSelectMarketsFirst",
		"notes",
		"preOrderEnabled",
		"relatedApps",
		"createdByActor",
		"lastModifiedByActor",
		"submittedByActor",
		"inAppEvents",
		"supportedTerritories",
	}
}

func nominationIncludeList() []string {
	return []string{
		"relatedApps",
		"createdByActor",
		"lastModifiedByActor",
		"submittedByActor",
		"inAppEvents",
		"supportedTerritories",
	}
}

func nominationDeviceFamilyList() []string {
	return []string{"IPHONE", "IPAD", "APPLE_TV", "APPLE_WATCH", "MAC", "VISION"}
}
