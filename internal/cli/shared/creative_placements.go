package shared

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

var placementLocalizationID = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// CreativePlacementsCommand manages reusable library placements for a localization.
func CreativePlacementsCommand(resource, prefix, label string) *ffcli.Command {
	subcommands := []*ffcli.Command{creativePlacementsListCommand(resource, prefix, label), creativePlacementViewCommand(prefix), creativePlacementCreateCommand(resource, prefix, label)}
	// Official ordering requests accept version, CPP, and PPO parents only.
	if resource != "appEventLocalizations" {
		subcommands = append(subcommands, creativePlacementReorderCommand(resource, prefix, label))
	}
	subcommands = append(subcommands, creativePlacementSwapCommand(resource, prefix, label), creativePlacementDeleteCommand(prefix))
	return &ffcli.Command{Name: "placements", ShortHelp: "Manage localized asset library placements.", FlagSet: flag.NewFlagSet("placements", flag.ExitOnError), UsageFunc: DefaultUsageFunc, Subcommands: subcommands, Exec: func(context.Context, []string) error { return flag.ErrHelp }}
}

func creativePlacementsListCommand(resource, prefix, label string) *ffcli.Command {
	fs := flag.NewFlagSet("placements list", flag.ExitOnError)
	id := BindResourceIDFlag(fs, "localization-id", resource, label+" ID or API self-link")
	listTypes := creativePlacementFilterTypes(resource)
	kind := fs.String("placement-type", "", "Filter by placement types, comma-separated: "+strings.Join(listTypes, ", "))
	group := fs.String("placement-group", "", "Filter by placement group IDs, comma-separated")
	includeValues := []string{"image", "video", "appEventLocalization", "appStoreVersionLocalization", "appCustomProductPageLocalization", "appStoreVersionExperimentTreatmentLocalization"}
	include := fs.String("include", "", "Include related resources, comma-separated: "+strings.Join(includeValues, ", "))
	filters := BindPlacementFilters(fs, resource)
	limit := fs.Int("limit", 0, "Maximum results per page (1-200; 0 uses server default)")
	next := fs.String("next", "", "Fetch a links.next URL instead of selecting a localization")
	paginate := fs.Bool("paginate", false, "Fetch all pages and aggregate the collection")
	output := BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "list", ShortHelp: "List asset placements for " + label + ".",
		ShortUsage: "asc " + prefix + " placements list [flags]", FlagSet: fs, UsageFunc: DefaultUsageFunc,
		LongHelp: "List asset placements for " + label + ". These placement reads are documented in Apple's published OpenAPI 4.5.1. Apple enforces availability and permissions for the selected localization. Empty data means no matching asset is assigned. JSON preserves Apple's full envelope and included media. Filter and sort flags cannot be combined with --next.\n\nExamples:\n  asc " + prefix + " placements list --localization-id ID --placement-type " + listTypes[0] + "\n  asc " + prefix + " placements list --localization-id ID --placement-type " + listTypes[1] + " --include image,video\n  asc " + prefix + " placements list --localization-id ID --paginate --output table\n  asc " + prefix + " placements list --localization-id ID --state FAILED --sort -lastModifiedDate",
		Exec: func(ctx context.Context, args []string) error {
			if len(args) != 0 {
				return UsageErrorf(prefix+" placements list: unexpected argument %q", args[0])
			}
			if err := ValidateBoundOutputFlags(fs); err != nil {
				return UsageErrorf(prefix+" placements list: %v", err)
			}
			if *limit < 0 || *limit > 200 {
				return UsageError(prefix + " placements list: --limit must be between 1 and 200 (0 uses server default)")
			}
			if err := ValidateNextURL(*next); err != nil {
				return UsageErrorf(prefix+" placements list: %v", err)
			}
			if err := RejectNextFlagConflicts(fs, *next, prefix+" placements list", append([]string{"localization-id", "placement-type", "placement-group", "include", "limit"}, filters.Names()...)...); err != nil {
				return err
			}
			kinds, err := validatedPlacementCSV(*kind, listTypes)
			if err != nil {
				return UsageErrorf(prefix+" placements list: --placement-type %v", err)
			}
			groups, err := placementIDsCSV(*group)
			if err != nil {
				return UsageErrorf(prefix+" placements list: --placement-group %v", err)
			}
			includes, err := validatedPlacementCSV(*include, includeValues)
			if err != nil {
				return UsageErrorf(prefix+" placements list: --include %v", err)
			}
			q := url.Values{}
			if err := filters.Apply(q); err != nil {
				return UsageErrorf(prefix+" placements list: %v", err)
			}
			target := *next
			if target == "" {
				localizationID := strings.TrimSpace(*id)
				if localizationID == "" {
					return UsageError(prefix + " placements list: --localization-id is required")
				}
				if !placementLocalizationID.MatchString(localizationID) {
					return UsageError(prefix + " placements list: --localization-id must be a resource ID")
				}
				target = "/v1/" + resource + "/" + url.PathEscape(localizationID) + "/placements"
				if kinds != "" {
					q.Set("filter[placementType]", kinds)
				}
				if len(groups) > 0 {
					q.Set("filter[placementGroup]", strings.Join(groups, ","))
				}
				if includes != "" {
					q.Set("include", includes)
				}
				pageLimit := *limit
				if *paginate && pageLimit == 0 {
					pageLimit = 200
				}
				if pageLimit > 0 {
					q.Set("limit", strconv.Itoa(pageLimit))
				}
				if len(q) > 0 {
					target += "?" + q.Encode()
				}
			}
			client, err := GetASCClient()
			if err != nil {
				return fmt.Errorf(prefix+" placements list: %w", err)
			}
			var response []byte
			if *paginate {
				response, err = client.RawPaginatedGET(ctx, target, ContextWithTimeout)
			} else {
				requestCtx, cancel := ContextWithTimeout(ctx)
				defer cancel()
				response, err = client.RawRequest(requestCtx, "GET", target, nil)
			}
			if err != nil {
				return fmt.Errorf(prefix+" placements list: %w", err)
			}
			headers, rows, err := asc.CreativePlacementRows(response)
			if err != nil {
				return err
			}
			return PrintOutputRows(json.RawMessage(response), *output.Output, *output.Pretty, headers, rows)
		},
	}
}

// creativePlacementFilterTypes returns Apple's filter[placementType] enum, with
// the context's write types first. Reads are wider than writes: filter support
// does not establish creation support.
func creativePlacementFilterTypes(resource string) []string {
	kinds := creativePlacementTypes(resource)
	// The spec also lists RETENTION_MESSAGE_ASSET, but Apple returns 400 "is not a valid filter value" for it.
	for _, kind := range []string{"PRODUCT_PAGE_HEADER_ASSET", "APP_STORE_SEARCH_RESULTS_ASSET", "APP_SCREENSHOT", "APP_PREVIEW", "IMESSAGE_APP_SCREENSHOT", "EVENT_CARD_ASSET", "EVENT_DETAILS_PAGE_ASSET", "SEARCH_RESULTS_ADS_ASSET", "TODAY_TAB_ADS_ASSET"} {
		if !slices.Contains(kinds, kind) {
			kinds = append(kinds, kind)
		}
	}
	return kinds
}

// Enums for GET /v1/appAssetLibraryPlacements/{id}.
var (
	creativePlacementViewIncludes = []string{"image", "video", "appEventLocalization", "appStoreVersionLocalization", "appCustomProductPageLocalization", "appStoreVersionExperimentTreatmentLocalization"}
	creativePlacementViewFields   = []string{"mediaType", "placementType", "placementGroup", "createdDate", "lastModifiedDate", "state", "stateDetails", "image", "video", "appEventLocalization", "appStoreVersionLocalization", "appCustomProductPageLocalization", "appStoreVersionExperimentTreatmentLocalization"}
)

func creativePlacementViewCommand(prefix string) *ffcli.Command {
	fs := flag.NewFlagSet("placements view", flag.ExitOnError)
	id := BindResourceIDFlag(fs, "id", "appAssetLibraryPlacements", "Asset library placement ID or API self-link")
	include := fs.String("include", "", "Include related resources, comma-separated: "+strings.Join(creativePlacementViewIncludes, ", "))
	fields := fs.String("fields", "", "Placement fields to return, comma-separated: "+strings.Join(creativePlacementViewFields, ", "))
	output := BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "view", ShortHelp: "Read one asset placement by ID.",
		ShortUsage: "asc " + prefix + " placements view --id ID [flags]", FlagSet: fs, UsageFunc: DefaultUsageFunc,
		LongHelp: "Read one asset placement with the public GET endpoint. IDs are authoritative: Apple's endpoint has no localization parent, so this command does not check that the placement belongs to this command group's localization type. JSON preserves Apple's full envelope and included resources.\n\nExamples:\n  asc " + prefix + " placements view --id PLACEMENT_ID\n  asc " + prefix + " placements view --id PLACEMENT_ID --include image,video\n  asc " + prefix + " placements view --id PLACEMENT_ID --fields state,stateDetails --output table",
		Exec: func(ctx context.Context, args []string) error {
			if len(args) != 0 {
				return UsageErrorf(prefix+" placements view: unexpected argument %q", args[0])
			}
			if err := ValidateBoundOutputFlags(fs); err != nil {
				return UsageErrorf(prefix+" placements view: %v", err)
			}
			placementID := strings.TrimSpace(*id)
			if placementID == "" {
				return UsageError(prefix + " placements view: --id is required")
			}
			if !creativePlacementID.MatchString(placementID) {
				return UsageError(prefix + " placements view: --id must be a resource ID")
			}
			includes, err := validatedPlacementCSV(*include, creativePlacementViewIncludes)
			if err != nil {
				return UsageErrorf(prefix+" placements view: --include %v", err)
			}
			selectedFields, err := validatedPlacementCSV(*fields, creativePlacementViewFields)
			if err != nil {
				return UsageErrorf(prefix+" placements view: --fields %v", err)
			}
			target := "/v1/appAssetLibraryPlacements/" + url.PathEscape(placementID)
			q := url.Values{}
			if includes != "" {
				q.Set("include", includes)
			}
			if selectedFields != "" {
				q.Set("fields[appAssetLibraryPlacements]", selectedFields)
			}
			if len(q) > 0 {
				target += "?" + q.Encode()
			}
			client, err := GetASCClient()
			if err != nil {
				return fmt.Errorf(prefix+" placements view: %w", err)
			}
			requestCtx, cancel := ContextWithTimeout(ctx)
			defer cancel()
			response, err := client.RawRequest(requestCtx, "GET", target, nil)
			if err != nil {
				return fmt.Errorf(prefix+" placements view: %w", err)
			}
			headers, rows, err := asc.CreativePlacementResourceRows(response)
			if err != nil {
				return err
			}
			return PrintOutputRows(json.RawMessage(response), *output.Output, *output.Pretty, headers, rows)
		},
	}
}

func validatedPlacementCSV(raw string, allowed []string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	values := strings.Split(raw, ",")
	for i, value := range values {
		value = strings.TrimSpace(value)
		valid := false
		for _, candidate := range allowed {
			if value == candidate {
				valid = true
				break
			}
		}
		if !valid {
			return "", fmt.Errorf("contains unsupported value %q", value)
		}
		values[i] = value
	}
	return strings.Join(values, ","), nil
}
