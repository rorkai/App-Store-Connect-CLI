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

// Command returns the Asset Library command group.
func Command() *ffcli.Command {
	return group("asset-library", "Inspect Asset Library media and specifications, and upload images and videos.",
		readCommand("view", "Read an app's Asset Library.", "app", "/v1/apps/%s/assetLibrary", false),
		group("images", "Inspect and upload Asset Library images.",
			imagesUploadCommand(),
			readCommand("list", "List images in an Asset Library.", "library-id", "/v1/appAssetLibraries/%s/images", true),
			readCommand("view", "Read an Asset Library image.", "id", "/v1/appAssetLibraryImages/%s", false),
			readCommand("placements", "List where an Asset Library image is used.", "id", "/v1/appAssetLibraryImages/%s/placements", true),
			lifecycleCommand("rename", "appAssetLibraryImages"), lifecycleCommand("archive", "appAssetLibraryImages"), lifecycleCommand("unarchive", "appAssetLibraryImages"), lifecycleCommand("delete", "appAssetLibraryImages")),
		group("videos", "Inspect and upload Asset Library videos.",
			videosUploadCommand(),
			readCommand("list", "List videos in an Asset Library.", "library-id", "/v1/appAssetLibraries/%s/videos", true),
			readCommand("view", "Read an Asset Library video.", "id", "/v1/appAssetLibraryVideos/%s", false),
			readCommand("placements", "List where an Asset Library video is used.", "id", "/v1/appAssetLibraryVideos/%s/placements", true),
			lifecycleCommand("rename", "appAssetLibraryVideos"), lifecycleCommand("archive", "appAssetLibraryVideos"), lifecycleCommand("unarchive", "appAssetLibraryVideos"), lifecycleCommand("delete", "appAssetLibraryVideos"), lifecycleCommand("set-poster-frame", "appAssetLibraryVideos")),
		readCommand("specs", "Read Apple's current asset dimensions and placement policies.", "", "/v1/appAssetLibraryRefData", false))
}

func group(name, help string, children ...*ffcli.Command) *ffcli.Command {
	return &ffcli.Command{
		Name: name, ShortHelp: help,
		LongHelp: help + ` The public image and video list filters are documented in Apple's OpenAPI 4.5.1. Image and video upload is supported.

Use asc review items add with --item-type appAssetLibraryImages or
appAssetLibraryVideos to add assets to a review submission, then use
asc review submissions-submit --id SUBMISSION_ID --confirm to submit it.
Separate asset review requires an approved app version. For the first app
version, submit assets with that version.

Examples:
  asc asset-library view --app APP_ID
  asc asset-library images list --library-id LIBRARY_ID --paginate
  asc asset-library images view --id IMAGE_ID
  asc asset-library images upload --library-id LIBRARY_ID --file ./header.png
  asc asset-library images placements --id IMAGE_ID
  asc asset-library videos list --library-id LIBRARY_ID
  asc asset-library specs`,
		FlagSet: flag.NewFlagSet(name, flag.ExitOnError), UsageFunc: shared.DefaultUsageFunc,
		Subcommands: children, Exec: func(context.Context, []string) error { return flag.ErrHelp },
	}
}

var resourceIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func readCommand(name, help, selector, path string, collection bool) *ffcli.Command {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
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
		fs.StringVar(&next, "next", "", "Fetch a links.next URL instead of selecting a resource")
		fs.BoolVar(&paginate, "paginate", false, "Fetch all pages and aggregate the collection")
	}
	var filters *libraryListFilters
	if name == "list" && selector == "library-id" {
		filters = bindLibraryListFilters(fs)
	}
	var placementFilters *shared.PlacementFilters
	if name == "placements" {
		placementFilters = shared.BindPlacementFilters(fs, resourceType)
	}
	longHelp := ""
	if placementFilters != nil {
		longHelp = help + "\n\nFilter by placement type, placement group, state, media, localization or placement ID using comma-separated tokens. State values are validated by Apple. Sort by createdDate, lastModifiedDate or placementGroupPosition; prefix a field with - for descending order. Filter and sort flags cannot be combined with --next."
	}
	if filters != nil {
		longHelp = help + "\n\nFilter category, state, specification IDs, asset IDs and reference names using comma-separated tokens. State values are validated by Apple. Sort by referenceName, createdDate or lastModifiedDate; prefix a field with - for descending order. Filter and sort flags cannot be combined with --next."
	}
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: name, ShortHelp: help, FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		LongHelp: longHelp,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) != 0 {
				return shared.UsageErrorf("asset-library %s: unexpected argument %q", name, args[0])
			}
			if err := shared.ValidateBoundOutputFlags(fs); err != nil {
				return shared.UsageErrorf("asset-library %s: %v", name, err)
			}
			if limit < 0 || limit > 200 {
				return shared.UsageError("asset-library: --limit must be between 1 and 200 (or 0 for server default)")
			}
			if next != "" {
				if err := shared.ValidateNextURL(next); err != nil {
					return shared.UsageErrorf("asset-library: %v", err)
				}
				conflicts := []string{selector, "limit"}
				if filters != nil {
					conflicts = append(conflicts, libraryListFilterFlags...)
				}
				if placementFilters != nil {
					conflicts = append(conflicts, placementFilters.Names()...)
				}
				if err := shared.RejectNextFlagConflicts(fs, next, "asset-library", conflicts...); err != nil {
					return err
				}
			}
			query := url.Values{}
			if filters != nil {
				var err error
				query, err = filters.query(fs)
				if err != nil {
					return err
				}
			}
			if placementFilters != nil {
				if err := placementFilters.Apply(query); err != nil {
					return shared.UsageErrorf("asset-library: %v", err)
				}
			}
			resourceID := strings.TrimSpace(*id)
			if selector == "app" {
				resourceID = shared.ResolveAppID(resourceID)
			}
			target := path
			if selector != "" && next == "" {
				if resourceID == "" {
					return shared.UsageErrorf("asset-library: --%s is required", selector)
				}
				if !resourceIDPattern.MatchString(resourceID) {
					return shared.UsageErrorf("asset-library: --%s must be a resource ID", selector)
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
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("asset-library: %w", err)
			}
			var response []byte
			if paginate {
				response, err = client.RawPaginatedGET(ctx, target, shared.ContextWithTimeout)
			} else {
				requestCtx, cancel := shared.ContextWithTimeout(ctx)
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
					return asc.WriteMarkdown(headers, rows)
				}
				return asc.WriteTable(headers, rows)
			}
			return shared.PrintOutputWithRenderers(json.RawMessage(response), *output.Output, *output.Pretty, func() error { return render(false) }, func() error { return render(true) })
		},
	}
}
