package assetlibrary

import (
	"flag"
	"net/url"
	"regexp"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var libraryStateToken = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

var libraryListFilterFlags = []string{"category", "state", "spec-id", "sort", "reference-name", "id"}

type libraryListFilters struct{ category, state, specID, sort, referenceName, id string }

func bindLibraryListFilters(fs *flag.FlagSet) *libraryListFilters {
	filters := &libraryListFilters{}
	fs.StringVar(&filters.category, "category", "", "Filter categories (CSV): CREATIVE_ASSETS, APP_SCREENSHOTS_AND_PREVIEWS")
	fs.StringVar(&filters.state, "state", "", "Filter asset states (CSV; provider validates values)")
	fs.StringVar(&filters.specID, "spec-id", "", "Filter asset specification IDs (comma-separated)")
	fs.StringVar(&filters.sort, "sort", "", "Sort by referenceName, createdDate or lastModifiedDate (CSV; prefix - for descending)")
	fs.StringVar(&filters.referenceName, "reference-name", "", "Filter internal reference names (CSV)")
	fs.StringVar(&filters.id, "id", "", "Filter asset IDs (comma-separated)")
	return filters
}

func (filters *libraryListFilters) query(fs *flag.FlagSet) (url.Values, error) {
	return filterQuery(fs, []filterField{
		{"category", "filter[category]", filters.category, func(token string) bool { return token == "CREATIVE_ASSETS" || token == "APP_SCREENSHOTS_AND_PREVIEWS" }},
		{"state", "filter[state]", filters.state, libraryStateToken.MatchString},
		{"spec-id", "filter[specId]", filters.specID, resourceIDPattern.MatchString},
		{"reference-name", "filter[referenceName]", filters.referenceName, func(string) bool { return true }},
		{"id", "filter[id]", filters.id, resourceIDPattern.MatchString},
		{"sort", "sort", filters.sort, func(token string) bool {
			value := strings.TrimPrefix(token, "-")
			return value == "referenceName" || value == "lastModifiedDate" || value == "createdDate"
		}},
	})
}

// specsFilters holds filters for GET /v1/appAssetLibraryRefData. Apple's
// OpenAPI defines no enum for these parameters, so values pass through.
type specsFilters struct{ placementType, placementProfileGroup, feature, spec string }

func bindSpecsFilters(fs *flag.FlagSet) *specsFilters {
	filters := &specsFilters{}
	fs.StringVar(&filters.placementType, "placement-type", "", "Filter placement types (CSV; provider validates values)")
	fs.StringVar(&filters.placementProfileGroup, "placement-profile-group", "", "Filter placement profile groups (CSV; provider validates values)")
	fs.StringVar(&filters.feature, "feature", "", "Filter features (CSV; provider validates values)")
	fs.StringVar(&filters.spec, "spec", "", "Filter specifications (CSV; provider validates values)")
	return filters
}

func (filters *specsFilters) query(fs *flag.FlagSet) (url.Values, error) {
	anyValue := func(string) bool { return true }
	return filterQuery(fs, []filterField{
		{"placement-type", "filter[placementTypes]", filters.placementType, anyValue},
		{"placement-profile-group", "filter[placementProfileGroups]", filters.placementProfileGroup, anyValue},
		{"feature", "filter[features]", filters.feature, anyValue},
		{"spec", "filter[specs]", filters.spec, anyValue},
	})
}

type filterField struct {
	flag, key, raw string
	valid          func(string) bool
}

func filterQuery(fs *flag.FlagSet, fields []filterField) (url.Values, error) {
	values := url.Values{}
	provided := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { provided[f.Name] = true })
	for _, field := range fields {
		if !provided[field.flag] {
			continue
		}
		tokens := strings.Split(field.raw, ",")
		for i, token := range tokens {
			tokens[i] = strings.TrimSpace(token)
			if tokens[i] == "" || !field.valid(tokens[i]) {
				return nil, shared.UsageErrorf("asset-library: --%s contains an empty or unsupported token %q", field.flag, tokens[i])
			}
		}
		values.Set(field.key, strings.Join(tokens, ","))
	}
	return values, nil
}
