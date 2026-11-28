// Package assetlibrary exposes live-verified public Asset Library operations.
package assetlibrary

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
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// NewCommand returns the Asset Library command group, built on d.
func NewCommand(d *shared.Deps) *ffcli.Command {
	return group(d, "asset-library", "Inspect Asset Library media and specifications, and upload images and videos.",
		group(d, "manifest", "Plan multi-locale creative manifests.", manifestPlanCommand(d), manifestApplyCommand(d)),
		readCommand(d, "asset-library", "view", "Read an app's Asset Library.", "app", "/v1/apps/%s/assetLibrary", false),
		group(d, "images", "Inspect and upload Asset Library images.",
			imagesUploadCommand(d),
			readCommand(d, "asset-library images", "list", "List images in an Asset Library.", "library-id", "/v1/appAssetLibraries/%s/images", true),
			readCommand(d, "asset-library images", "view", "Read an Asset Library image.", "id", "/v1/appAssetLibraryImages/%s", false),
			readCommand(d, "asset-library images", "placements", "List where an Asset Library image is used.", "id", "/v1/appAssetLibraryImages/%s/placements", true),
			lifecycleCommand(d, "rename", "appAssetLibraryImages"), lifecycleCommand(d, "archive", "appAssetLibraryImages"), lifecycleCommand(d, "unarchive", "appAssetLibraryImages"), lifecycleCommand(d, "delete", "appAssetLibraryImages")),
		group(d, "videos", "Inspect and upload Asset Library videos.",
			videosUploadCommand(d),
			readCommand(d, "asset-library videos", "list", "List videos in an Asset Library.", "library-id", "/v1/appAssetLibraries/%s/videos", true),
			readCommand(d, "asset-library videos", "view", "Read an Asset Library video.", "id", "/v1/appAssetLibraryVideos/%s", false),
			readCommand(d, "asset-library videos", "placements", "List where an Asset Library video is used.", "id", "/v1/appAssetLibraryVideos/%s/placements", true),
			lifecycleCommand(d, "rename", "appAssetLibraryVideos"), lifecycleCommand(d, "archive", "appAssetLibraryVideos"), lifecycleCommand(d, "unarchive", "appAssetLibraryVideos"), lifecycleCommand(d, "delete", "appAssetLibraryVideos"), lifecycleCommand(d, "set-poster-frame", "appAssetLibraryVideos")),
		readCommand(d, "asset-library", "specs", "Read Apple's current asset dimensions and placement policies.", "", "/v1/appAssetLibraryRefData", false))
}

func group(d *shared.Deps, name, help string, children ...*ffcli.Command) *ffcli.Command {
	return &ffcli.Command{
		Name: name, ShortHelp: help,
		LongHelp: help + ` The public image and video list filters are documented in Apple's OpenAPI 4.5.1. Image and video upload is supported.

Use asc review items add with --item-type appAssetLibraryImages or
appAssetLibraryVideos to add assets to a review submission, then use
asc review submissions-submit --id SUBMISSION_ID --confirm to submit it.
Separate asset review requires an approved app version. For the first app
version, place the assets on the version with asc localizations placements
create and submit the version: placed assets are reviewed with it, and adding
them as review items fails with "Asset is locked by an embedded review
submission".

Examples:
  asc asset-library view --app APP_ID
  asc asset-library images list --library-id LIBRARY_ID --paginate
  asc asset-library images view --id IMAGE_ID
  asc asset-library images upload --library-id LIBRARY_ID --file ./header.png
  asc asset-library images placements --id IMAGE_ID
  asc asset-library videos list --library-id LIBRARY_ID
  asc asset-library specs
  asc asset-library specs --placement-type APP_SCREENSHOT --feature APP_STORE_VERSIONS`,
		FlagSet: d.NewFlagSet(name, flag.ExitOnError), UsageFunc: d.DefaultUsageFunc,
		Subcommands: children, Exec: func(context.Context, []string) error { return flag.ErrHelp },
	}
}

var resourceIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func readCommand(d *shared.Deps, parent, name, help, selector, path string, collection bool) *ffcli.Command {
	fs := d.NewFlagSet(name, flag.ExitOnError)
	id := new(string)
	resourceType := "appAssetLibraries"
	if selector != "" {
		switch selector {
		case "app":
			resourceType = "apps"
		case "id":
			resourceType = "appAssetLibraryImages"
			if strings.Contains(path, "/appAssetLibraryVideos/") {
				resourceType = "appAssetLibraryVideos"
			}
		}
		id = shared.BindResourceIDFlag(fs, selector, resourceType, "Resource ID or API self-link (use asset-library view --app to find the library ID)")
	}
	var limit int
	var next string
	var paginate bool
	if collection {
		fs.IntVar(&limit, "limit", 0, "Maximum results per page (1-200; 0 uses server default)")
		fs.StringVar(&next, "next", "", shared.NextFlagUsage)
		fs.BoolVar(&paginate, "paginate", false, shared.PaginateFlagUsage)
	}
	var filters *libraryListFilters
	if name == "list" && selector == "library-id" {
		filters = bindLibraryListFilters(fs)
	}
	var specs *specsFilters
	if name == "specs" {
		specs = bindSpecsFilters(fs)
	}
	var placementFilters *shared.PlacementFilters
	if name == "placements" {
		placementFilters = shared.BindPlacementFilters(fs, resourceType)
	}
	longHelp := ""
	if specs != nil {
		longHelp = help + "\n\nFilter placement types, placement profile groups, features and specifications using comma-separated tokens. Apple validates values."
	}
	if placementFilters != nil {
		longHelp = help + "\n\nFilter by placement type, placement group, state, media, localization or placement ID using comma-separated tokens. State values are validated by Apple. Sort by createdDate, lastModifiedDate or placementGroupPosition; prefix a field with - for descending order. Filter and sort flags cannot be combined with --next."
	}
	if filters != nil {
		longHelp = help + "\n\nFilter category, state, specification IDs, asset IDs and reference names using comma-separated tokens. State values are validated by Apple. Sort by referenceName, createdDate or lastModifiedDate; prefix a field with - for descending order. Filter and sort flags cannot be combined with --next."
	}
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: name, ShortHelp: help, FlagSet: fs, UsageFunc: d.DefaultUsageFunc,
		LongHelp: longHelp,
		Exec: func(ctx context.Context, args []string) error {
			if err := shared.ValidateBoundOutputFlags(fs); err != nil {
				return d.UsageErrorf("asset-library %s: %v", name, err)
			}
			if collection {
				conflicts := []string{"limit"}
				if filters != nil {
					conflicts = append(conflicts, libraryListFilterFlags...)
				}
				if placementFilters != nil {
					conflicts = append(conflicts, placementFilters.Names()...)
				}
				if err := d.ValidatePaginationFlags(ctx, fs, parent+" "+name, 200, conflicts...); err != nil {
					return err
				}
			}
			resourceID := strings.TrimSpace(*id)
			if selector == "app" {
				resourceID = d.ResolveAppID(resourceID)
			}
			if selector != "" && next == "" && resourceID == "" {
				return d.UsageErrorf("asset-library: --%s is required", selector)
			}
			query := url.Values{}
			var err error
			switch {
			case filters != nil:
				query, err = filters.query(d, fs)
			case specs != nil:
				query, err = specs.query(d, fs)
			}
			if err != nil {
				return err
			}
			if placementFilters != nil {
				if err := placementFilters.Apply(query); err != nil {
					return d.UsageErrorf("asset-library: %v", err)
				}
			}
			target := path
			if selector != "" && next == "" {
				if !resourceIDPattern.MatchString(resourceID) {
					return d.UsageErrorf("asset-library: --%s must be a resource ID", selector)
				}
				target = fmt.Sprintf(path, url.PathEscape(resourceID))
			}
			if next != "" {
				target = next
			} else {
				pageLimit := limit
				if paginate && pageLimit == 0 {
					pageLimit = 200
				}
				if pageLimit > 0 {
					query.Set("limit", strconv.Itoa(pageLimit))
				}
				if len(query) > 0 {
					target += "?" + query.Encode()
				}
			}
			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("asset-library: %w", err)
			}
			var response []byte
			if paginate {
				response, err = client.RawPaginatedGET(ctx, target, d.Timeout)
			} else {
				requestCtx, cancel := d.Timeout(ctx)
				defer cancel()
				response, err = client.RawRequest(requestCtx, "GET", target, nil)
			}
			if err != nil {
				return fmt.Errorf("asset-library: %w", err)
			}
			if err := asc.ValidateAssetLibraryResponse(response); err != nil {
				return err
			}
			render := func(markdown bool) error {
				headers, rows, err := asc.AssetLibraryRows(response)
				if err != nil {
					return err
				}
				if markdown {
					return asc.WriteMarkdownTo(d.IO.Out, headers, rows)
				}
				return asc.WriteTableTo(d.IO.Out, headers, rows)
			}
			return d.PrintOutputWithRenderers(json.RawMessage(response), *output.Output, *output.Pretty, func() error { return render(false) }, func() error { return render(true) })
		},
	}
}
