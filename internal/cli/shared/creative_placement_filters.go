package shared

import (
	"flag"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
)

// placementStateToken stays structural because Apple returns states such as ACTIVE that the OpenAPI enum omits.
var placementStateToken = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// assetPlacementTypes is the OpenAPI enum for the image and video placement reads.
var assetPlacementTypes = []string{
	"APP_SCREENSHOT", "IMESSAGE_APP_SCREENSHOT", "APP_PREVIEW", "PRODUCT_PAGE_HEADER_ASSET", "APP_STORE_SEARCH_RESULTS_ASSET",
	"SEARCH_RESULTS_ADS_ASSET", "TODAY_TAB_ADS_ASSET", "EVENT_CARD_ASSET", "EVENT_DETAILS_PAGE_ASSET", "RETENTION_MESSAGE_ASSET",
}

type placementFilter struct {
	name, param, usage string
	// omittedFor is the parent whose operation does not accept this filter.
	omittedFor string
	// assetOnly marks flags that localization lists declare themselves.
	assetOnly bool
	valid     func(string) bool
}

var placementFilterTable = []placementFilter{
	{name: "placement-type", param: "filter[placementType]", usage: "Filter by placement types, comma-separated: " + strings.Join(assetPlacementTypes, ", "), assetOnly: true, valid: func(token string) bool { return slices.Contains(assetPlacementTypes, token) }},
	{name: "placement-group", param: "filter[placementGroup]", usage: "Filter by placement group IDs, comma-separated", assetOnly: true, valid: creativePlacementID.MatchString},
	{name: "state", param: "filter[state]", usage: "Filter by placement states, comma-separated (Apple validates values)", valid: placementStateToken.MatchString},
	{name: "image-id", param: "filter[image]", usage: "Filter by library image IDs, comma-separated", omittedFor: "appAssetLibraryImages", valid: creativePlacementID.MatchString},
	{name: "video-id", param: "filter[video]", usage: "Filter by library video IDs, comma-separated", omittedFor: "appAssetLibraryVideos", valid: creativePlacementID.MatchString},
	{name: "version-localization", param: "filter[appStoreVersionLocalization]", usage: "Filter by version localization IDs, comma-separated", omittedFor: "appStoreVersionLocalizations", valid: creativePlacementID.MatchString},
	{name: "custom-page-localization", param: "filter[appCustomProductPageLocalization]", usage: "Filter by custom product page localization IDs, comma-separated", omittedFor: "appCustomProductPageLocalizations", valid: creativePlacementID.MatchString},
	{name: "treatment-localization", param: "filter[appStoreVersionExperimentTreatmentLocalization]", usage: "Filter by experiment treatment localization IDs, comma-separated", omittedFor: "appStoreVersionExperimentTreatmentLocalizations", valid: creativePlacementID.MatchString},
	{name: "event-localization", param: "filter[appEventLocalization]", usage: "Filter by in-app event localization IDs, comma-separated", omittedFor: "appEventLocalizations", valid: creativePlacementID.MatchString},
	{name: "placement-id", param: "filter[id]", usage: "Filter by placement IDs, comma-separated", valid: creativePlacementID.MatchString},
	{name: "sort", param: "sort", usage: "Sort by createdDate, lastModifiedDate or placementGroupPosition (comma-separated; prefix - for descending)", valid: func(token string) bool {
		value := strings.TrimPrefix(token, "-")
		return value == "createdDate" || value == "lastModifiedDate" || value == "placementGroupPosition"
	}},
}

type boundPlacementFilter struct {
	placementFilter
	value *string
}

// PlacementFilters holds the filter and sort flags of one placement list.
type PlacementFilters struct {
	fs    *flag.FlagSet
	bound []boundPlacementFilter
}

// BindPlacementFilters binds the filter and sort flags resource's placement list accepts.
func BindPlacementFilters(fs *flag.FlagSet, resource string) *PlacementFilters {
	isAsset := resource == "appAssetLibraryImages" || resource == "appAssetLibraryVideos"
	filters := &PlacementFilters{fs: fs}
	for _, filter := range placementFilterTable {
		if filter.omittedFor == resource || (filter.assetOnly && !isAsset) {
			continue
		}
		filters.bound = append(filters.bound, boundPlacementFilter{filter, fs.String(filter.name, "", filter.usage)})
	}
	return filters
}

// Names returns the bound flag names for --next conflict checks.
func (filters *PlacementFilters) Names() []string {
	names := make([]string, len(filters.bound))
	for i, filter := range filters.bound {
		names[i] = filter.name
	}
	return names
}

// Apply validates the provided flags and adds them to query. Omitted flags add nothing.
func (filters *PlacementFilters) Apply(query url.Values) error {
	provided := map[string]bool{}
	filters.fs.Visit(func(f *flag.Flag) { provided[f.Name] = true })
	for _, filter := range filters.bound {
		if !provided[filter.name] {
			continue
		}
		tokens := strings.Split(*filter.value, ",")
		for i, token := range tokens {
			tokens[i] = strings.TrimSpace(token)
			if !filter.valid(tokens[i]) {
				return fmt.Errorf("--%s contains an empty or unsupported token %q", filter.name, tokens[i])
			}
		}
		query.Set(filter.param, strings.Join(tokens, ","))
	}
	return nil
}
