package reviews

import (
	"flag"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/ascterritory"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const (
	reviewResponseStateAny         = "any"
	reviewResponseStateUnresponded = "unresponded"
	reviewResponseStateResponded   = "responded"
)

// reviewSorts lists the sort values Apple accepts on every customer review
// listing endpoint.
var reviewSorts = []string{"rating", "-rating", "createdDate", "-createdDate"}

// The include and fields values Apple accepts on every customer review read.
var (
	reviewFields          = []string{"rating", "title", "body", "reviewerNickname", "createdDate", "territory", "response", "reviewTerritory"}
	reviewIncludes        = []string{"response", "reviewTerritory"}
	reviewResponseFields  = []string{"responseBody", "lastModifiedDate", "state", "review"}
	reviewTerritoryFields = []string{"currency"}
)

// ReviewFilterFlags holds the customer review filter flags. Apple exposes the
// same filter, sort, and include surface on GET /v1/apps/{id}/customerReviews
// and GET /v1/appStoreVersions/{id}/customerReviews, so every command that
// lists customer reviews binds these flags from here and gets identical names,
// help text, and validation.
type ReviewFilterFlags struct {
	*ReviewSelectionFlags
	Stars              string
	Territory          string
	ReviewTerritory    string
	Sort               string
	ResponseState      string
	OnlyUnresponded    bool
	IncludeResponse    bool
	starsSet           bool
	territorySet       bool
	reviewTerritorySet bool
	sortSet            bool
	responseStateSet   bool
}

// ReviewSelectionFlags holds the include and fields flags that Apple accepts on
// the customer review listings and on GET /v1/customerReviews/{id}.
type ReviewSelectionFlags struct {
	Fields             string
	Include            string
	ResponseFields     string
	TerritoryFields    string
	fieldsSet          bool
	includeSet         bool
	responseFieldsSet  bool
	territoryFieldsSet bool
}

type trackedReviewStringFlag struct {
	name           string
	value          *string
	set            *bool
	commaSeparated bool
}

func (f *trackedReviewStringFlag) Set(value string) error {
	if *f.set {
		if f.commaSeparated {
			return fmt.Errorf(
				"--%s specified multiple times; pass one comma-separated list, for example --%s %q",
				f.name, f.name, *f.value+","+value,
			)
		}
		return fmt.Errorf("--%s specified multiple times; pass it once", f.name)
	}
	*f.value = value
	*f.set = true
	return nil
}

func (f *trackedReviewStringFlag) String() string {
	if f == nil || f.value == nil {
		return ""
	}
	return *f.value
}

func bindTrackedReviewStringFlag(fs *flag.FlagSet, name string, value *string, provided *bool, defaultValue string, commaSeparated bool, usage string) {
	*value = defaultValue
	fs.Var(&trackedReviewStringFlag{name: name, value: value, set: provided, commaSeparated: commaSeparated}, name, usage)
}

// BindReviewFilterFlags registers the shared customer review filter flags on fs.
func BindReviewFilterFlags(fs *flag.FlagSet) *ReviewFilterFlags {
	filters := &ReviewFilterFlags{ReviewSelectionFlags: BindReviewSelectionFlags(fs)}
	bindTrackedReviewStringFlag(fs, "stars", &filters.Stars, &filters.starsSet, "", true, "Filter by star ratings, comma-separated (1-5)")
	bindTrackedReviewStringFlag(fs, "territory", &filters.Territory, &filters.territorySet, "", true, "Filter by App Store territories, comma-separated (e.g., USA,GBR)")
	bindTrackedReviewStringFlag(fs, "review-territory", &filters.ReviewTerritory, &filters.reviewTerritorySet, "", true, "Filter by related review territory IDs, comma-separated (e.g., USA,GBR)")
	bindTrackedReviewStringFlag(fs, "sort", &filters.Sort, &filters.sortSet, "", false, "Sort by "+strings.Join(reviewSorts, ", "))
	bindTrackedReviewStringFlag(fs, "response-state", &filters.ResponseState, &filters.responseStateSet, reviewResponseStateAny, false, "Filter by response state: any, unresponded/unreplied, responded/replied")
	fs.BoolVar(&filters.OnlyUnresponded, "only-unresponded", false, "Only list reviews without a published response")
	fs.BoolVar(&filters.IncludeResponse, "include-response", false, "Include customer review response relationships")
	return filters
}

// BindReviewSelectionFlags registers the include and fields flags on fs.
func BindReviewSelectionFlags(fs *flag.FlagSet) *ReviewSelectionFlags {
	selection := &ReviewSelectionFlags{}
	bindTrackedReviewStringFlag(fs, "fields", &selection.Fields, &selection.fieldsSet, "", true, "Comma-separated customer review fields: "+strings.Join(reviewFields, ","))
	bindTrackedReviewStringFlag(fs, "include", &selection.Include, &selection.includeSet, "", true, "Comma-separated relationships to include: "+strings.Join(reviewIncludes, ","))
	bindTrackedReviewStringFlag(fs, "response-fields", &selection.ResponseFields, &selection.responseFieldsSet, "", true, "Comma-separated customer review response fields: "+strings.Join(reviewResponseFields, ","))
	bindTrackedReviewStringFlag(fs, "territory-fields", &selection.TerritoryFields, &selection.territoryFieldsSet, "", true, "Comma-separated included territory fields: "+strings.Join(reviewTerritoryFields, ","))
	return selection
}

// ValidateReviewNextFlagConflicts rejects query flags that a links.next URL
// already encodes. Inspecting the FlagSet preserves explicit empty and zero
// values, which must not be silently ignored either.
func ValidateReviewNextFlagConflicts(next string, fs *flag.FlagSet, ownerFlag string) error {
	if strings.TrimSpace(next) == "" || fs == nil {
		return nil
	}

	setFlags := make(map[string]struct{})
	fs.Visit(func(f *flag.Flag) {
		setFlags[f.Name] = struct{}{}
	})
	for _, name := range []string{
		ownerFlag,
		"stars",
		"territory",
		"review-territory",
		"sort",
		"response-state",
		"only-unresponded",
		"include-response",
		"fields",
		"include",
		"response-fields",
		"territory-fields",
		"limit",
	} {
		if _, ok := setFlags[name]; ok {
			return shared.WithDiagnostic(
				shared.UsageErrorf("--next cannot be combined with --%s", name),
				shared.DiagnosticConflictingInput,
				"",
			)
		}
	}
	return nil
}

// ReviewOptions validates the bound flags and returns the matching query
// options. Invalid input is reported as a usage failure carrying the offending
// parameter so both review listings fail the same way.
func (f *ReviewFilterFlags) ReviewOptions() ([]asc.ReviewOption, error) {
	if f.starsSet && strings.TrimSpace(f.Stars) == "" {
		return nil, shared.WithDiagnostic(shared.UsageError(errInvalidReviewStars().Error()), shared.DiagnosticInvalidInput, "--stars")
	}
	if f.territorySet && strings.TrimSpace(f.Territory) == "" {
		return nil, shared.WithDiagnostic(shared.UsageError("--territory must be a valid App Store territory code"), shared.DiagnosticInvalidInput, "--territory")
	}
	if f.reviewTerritorySet && strings.TrimSpace(f.ReviewTerritory) == "" {
		return nil, shared.WithDiagnostic(shared.UsageError("--review-territory must be a valid App Store territory code"), shared.DiagnosticInvalidInput, "--review-territory")
	}
	if f.sortSet && strings.TrimSpace(f.Sort) == "" {
		return nil, shared.WithDiagnostic(shared.UsageError("--sort must be one of: "+strings.Join(reviewSorts, ", ")), shared.DiagnosticInvalidInput, "--sort")
	}
	if f.responseStateSet && strings.TrimSpace(f.ResponseState) == "" {
		return nil, shared.WithDiagnostic(shared.UsageError("--response-state must be one of: any, unresponded, unreplied, responded, replied"), shared.DiagnosticInvalidInput, "--response-state")
	}
	ratings, err := normalizeReviewStars(f.Stars)
	if err != nil {
		return nil, shared.WithDiagnostic(shared.UsageError(err.Error()), shared.DiagnosticInvalidInput, "--stars")
	}
	territory, err := normalizeReviewTerritories(f.Territory, "--territory")
	if err != nil {
		return nil, shared.WithDiagnostic(
			shared.UsageError(err.Error()),
			shared.DiagnosticInvalidInput,
			"--territory",
		)
	}
	reviewTerritory, err := normalizeReviewTerritories(f.ReviewTerritory, "--review-territory")
	if err != nil {
		return nil, shared.WithDiagnostic(shared.UsageError(err.Error()), shared.DiagnosticInvalidInput, "--review-territory")
	}
	if err := shared.ValidateSort(f.Sort, reviewSorts...); err != nil {
		return nil, shared.WithDiagnostic(shared.NewValidationError(err), shared.DiagnosticInvalidInput, "--sort")
	}
	responseState, err := normalizeReviewResponseState(f.ResponseState)
	if err != nil {
		return nil, shared.WithDiagnostic(shared.UsageError(err.Error()), shared.DiagnosticInvalidInput, "--response-state")
	}
	if f.OnlyUnresponded {
		if responseState == reviewResponseStateResponded {
			return nil, shared.WithDiagnostic(shared.UsageError("--only-unresponded cannot be combined with --response-state responded"), shared.DiagnosticConflictingInput, "")
		}
		responseState = reviewResponseStateUnresponded
	}
	selectionOpts, err := f.ReviewSelectionFlags.ReviewOptions()
	if err != nil {
		return nil, err
	}

	opts := []asc.ReviewOption{
		asc.WithRatings(ratings),
		asc.WithTerritory(territory),
		asc.WithReviewTerritory(reviewTerritory),
	}
	if strings.TrimSpace(f.Sort) != "" {
		opts = append(opts, asc.WithReviewSort(f.Sort))
	}
	if exists, ok := publishedResponseExistsFilter(responseState); ok {
		opts = append(opts, asc.WithPublishedResponseExists(exists))
	}
	if f.IncludeResponse {
		opts = append(opts, asc.WithReviewIncludeResponse())
	}
	return append(opts, selectionOpts...), nil
}

// ReviewOptions validates the include and fields flags. Fields of a relationship
// also include that relationship, matching --response-fields.
func (f *ReviewSelectionFlags) ReviewOptions() ([]asc.ReviewOption, error) {
	fields, err := normalizeReviewList(f.Fields, f.fieldsSet, "--fields", reviewFields)
	if err != nil {
		return nil, err
	}
	include, err := normalizeReviewList(f.Include, f.includeSet, "--include", reviewIncludes)
	if err != nil {
		return nil, err
	}
	responseFields, err := normalizeReviewList(f.ResponseFields, f.responseFieldsSet, "--response-fields", reviewResponseFields)
	if err != nil {
		return nil, err
	}
	territoryFields, err := normalizeReviewList(f.TerritoryFields, f.territoryFieldsSet, "--territory-fields", reviewTerritoryFields)
	if err != nil {
		return nil, err
	}

	opts := []asc.ReviewOption{
		asc.WithReviewFields(fields),
		asc.WithReviewInclude(include),
		asc.WithReviewResponseFields(responseFields),
		asc.WithReviewTerritoryFields(territoryFields),
	}
	if len(responseFields) > 0 {
		opts = append(opts, asc.WithReviewIncludeResponse())
	}
	if len(territoryFields) > 0 {
		opts = append(opts, asc.WithReviewInclude([]string{"reviewTerritory"}))
	}
	return opts, nil
}

func normalizeReviewTerritories(value, flagName string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}

	elements := strings.Split(value, ",")
	territories := make([]string, 0, len(elements))
	for _, element := range elements {
		normalized, err := ascterritory.Normalize(element)
		if err != nil {
			return "", fmt.Errorf("%s must be a valid App Store territory code or comma-separated list of codes", flagName)
		}
		territories = append(territories, normalized)
	}
	return strings.Join(territories, ","), nil
}

func normalizeReviewResponseState(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		normalized = reviewResponseStateAny
	}
	switch normalized {
	case reviewResponseStateAny, reviewResponseStateUnresponded, reviewResponseStateResponded:
		return normalized, nil
	case "unreplied":
		return reviewResponseStateUnresponded, nil
	case "replied":
		return reviewResponseStateResponded, nil
	default:
		return "", fmt.Errorf("--response-state must be one of: any, unresponded, unreplied, responded, replied")
	}
}

// normalizeReviewStars parses the comma-separated --stars value. Apple accepts
// filter[rating] as an array parameter, so callers can ask for several ratings
// in one request. An empty value means "no rating filter"; anything else must
// be a list where every element is a rating between 1 and 5. Empty elements
// (`1,,2`, `1,`) are rejected rather than skipped, so malformed input never
// silently narrows the filter.
func normalizeReviewStars(value string) ([]int, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}

	elements := strings.Split(value, ",")
	ratings := make([]int, 0, len(elements))
	for _, element := range elements {
		rating, err := strconv.Atoi(strings.TrimSpace(element))
		if err != nil || rating < 1 || rating > 5 {
			return nil, errInvalidReviewStars()
		}
		ratings = append(ratings, rating)
	}
	return ratings, nil
}

func errInvalidReviewStars() error {
	return fmt.Errorf("--stars must be a comma-separated list of star ratings: 1, 2, 3, 4, 5")
}

// normalizeReviewList parses a comma-separated include or fields value. An
// unset flag means "no selection"; anything else must be a list where every
// element is allowed. Empty elements (`responseBody,,state`, `,`) are rejected
// rather than skipped, so malformed input never silently changes the selection.
func normalizeReviewList(value string, set bool, flagName string, allowed []string) ([]string, error) {
	if !set {
		return nil, nil
	}

	elements := strings.Split(value, ",")
	normalized := make([]string, 0, len(elements))
	for _, element := range elements {
		element = strings.TrimSpace(element)
		if !slices.Contains(allowed, element) {
			return nil, shared.WithDiagnostic(
				shared.UsageErrorf("%s must be a comma-separated list of: %s", flagName, strings.Join(allowed, ",")),
				shared.DiagnosticInvalidInput,
				flagName,
			)
		}
		normalized = append(normalized, element)
	}
	return normalized, nil
}

func publishedResponseExistsFilter(responseState string) (bool, bool) {
	switch responseState {
	case reviewResponseStateUnresponded:
		return false, true
	case reviewResponseStateResponded:
		return true, true
	default:
		return false, false
	}
}
