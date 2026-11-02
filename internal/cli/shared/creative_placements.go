package shared

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

var placementLocalizationID = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// CreativePlacementsCommand manages reusable library placements for a localization.
func CreativePlacementsCommand(resource, prefix, label string) *ffcli.Command {
	subcommands := []*ffcli.Command{creativePlacementsListCommand(resource, prefix, label), creativePlacementCreateCommand(resource, prefix, label)}
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
	listTypes := creativePlacementTypes(resource)
	if resource == "appCustomProductPageLocalizations" {
		listTypes = append(listTypes, "IMESSAGE_APP_SCREENSHOT")
	}
	kind := fs.String("placement-type", "", "Filter by placement types, comma-separated: "+strings.Join(listTypes, ", "))
	group := fs.String("placement-group", "", "Filter by placement group IDs, comma-separated")
	include := fs.String("include", "", "Include related media, comma-separated: image, video")
	includedFields := BindIncludedFieldFlags(fs, map[string]string{"image": "appAssetLibraryImages", "video": "appAssetLibraryVideos"})
	sort := fs.String("sort", "", "Sort by placementGroupPosition")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200; 0 uses server default)")
	next := fs.String("next", "", "Fetch a links.next URL instead of selecting a localization")
	paginate := fs.Bool("paginate", false, "Fetch all pages and aggregate the collection")
	output := BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "list", ShortHelp: "List asset placements for " + label + ".",
		ShortUsage: "asc " + prefix + " placements list [flags]", FlagSet: fs, UsageFunc: DefaultUsageFunc,
		LongHelp: "List asset placements for " + label + ". These placement reads are documented in Apple's published OpenAPI 4.5.1. Apple enforces availability and permissions for the selected localization. Empty data means no matching asset is assigned. JSON preserves Apple's full envelope and included media.\n\nExamples:\n  asc " + prefix + " placements list --localization-id ID --placement-type " + listTypes[0] + "\n  asc " + prefix + " placements list --localization-id ID --placement-type " + listTypes[1] + " --include image,video\n  asc " + prefix + " placements list --localization-id ID --paginate --output table",
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
			if err := RejectNextFlagConflicts(fs, *next, prefix+" placements list", "localization-id", "placement-type", "placement-group", "include", "sort", "limit"); err != nil {
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
			includes, err := validatedPlacementCSV(*include, []string{"image", "video"})
			if err != nil {
				return UsageErrorf(prefix+" placements list: --include %v", err)
			}
			included, err := includedFields.Resolve(prefix+" placements list", *next, SplitCSV(includes))
			if err != nil {
				return err
			}
			if *sort != "" && *sort != "placementGroupPosition" {
				return UsageError(prefix + " placements list: --sort must be placementGroupPosition")
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
				q := url.Values{}
				if kinds != "" {
					q.Set("filter[placementType]", kinds)
				}
				if len(groups) > 0 {
					q.Set("filter[placementGroup]", strings.Join(groups, ","))
				}
				if includes != "" {
					q.Set("include", includes)
				}
				included.AddTo(q)
				if *sort != "" {
					q.Set("sort", *sort)
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
