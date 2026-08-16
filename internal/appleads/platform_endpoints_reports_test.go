package appleads

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestPlatformReportsOptimizationEndpointSpecs(t *testing.T) {
	wants := map[string]struct {
		method       string
		path         string
		bodyKind     BodyKind
		bodyType     string
		responseType string
		confirm      bool
	}{
		"reports apps ad-groups":                {"POST", "v1/reports/apps/adgroups/query", BodyObject, "AppsReportingRequest", "AppsAdGroupReportResponse", false},
		"reports apps ads":                      {"POST", "v1/reports/apps/ads/query", BodyObject, "AppsReportingRequest", "AppsAdReportResponse", false},
		"reports apps campaigns":                {"POST", "v1/reports/apps/campaigns/query", BodyObject, "AppsReportingRequest", "AppsCampaignReportResponse", false},
		"reports apps keywords":                 {"POST", "v1/reports/apps/keywords/query", BodyObject, "AppsReportingRequest", "AppsKeywordReportResponse", false},
		"reports apps search-terms":             {"POST", "v1/reports/apps/searchterms/query", BodyObject, "AppsReportingRequest", "AppsSearchTermReportResponse", false},
		"reports brands ad-groups":              {"POST", "v1/reports/business-brands/adgroups/query", BodyObject, "BrandsReportingRequest", "BrandsAdGroupReportResponse", false},
		"reports brands ads":                    {"POST", "v1/reports/business-brands/ads/query", BodyObject, "BrandsReportingRequest", "BrandsAdReportResponse", false},
		"reports brands campaigns":              {"POST", "v1/reports/business-brands/campaigns/query", BodyObject, "BrandsReportingRequest", "BrandsCampaignReportResponse", false},
		"reports brands keywords":               {"POST", "v1/reports/business-brands/keywords/query", BodyObject, "BrandsReportingRequest", "BrandsKeywordReportResponse", false},
		"reports brands search-terms":           {"POST", "v1/reports/business-brands/searchterms/query", BodyObject, "BrandsReportingRequest", "BrandsSearchTermReportResponse", false},
		"insights impression-share find":        {"POST", "v1/insights/apps/impression-share/query", BodyObject, "ImpressionShareQueryRequest", "ImpressionShareQueryResponse", false},
		"insights search-term-popularity find":  {"POST", "v1/insights/apps/search-term-popularity/query", BodyObject, "SearchTermPopularityQueryRequest", "SearchTermPopularityQueryResponse", false},
		"recommendations daily-budgets apply":   {"POST", "v1/recommendations/daily-budgets/apply", BodyArray, "[ApplyDailyCapRecommendation]", "RecommendationApplyDailyBudgetResponse", false},
		"recommendations daily-budgets dismiss": {"POST", "v1/recommendations/daily-budgets/dismiss", BodyArray, "[ApplyDailyCapRecommendation]", "RecommendationDismissDailyBudgetResponse", false},
		"recommendations daily-budgets find":    {"POST", "v1/recommendations/daily-budgets/query", BodyObject, "RecommendationQueryRequest", "RecommendationQueryDailyBudgetResponse", false},
		"recommendations target-cpas apply":     {"POST", "v1/recommendations/target-cpas/apply", BodyArray, "[ApplyTargetCpaRecommendation]", "RecommendationApplyTargetCpaResponse", false},
		"recommendations target-cpas dismiss":   {"POST", "v1/recommendations/target-cpas/dismiss", BodyArray, "[ApplyTargetCpaRecommendation]", "RecommendationDismissTargetCpaResponse", false},
		"recommendations target-cpas find":      {"POST", "v1/recommendations/target-cpas/query", BodyObject, "RecommendationQueryRequest", "RecommendationQueryTargetCpaResponse", false},
		"suggestions categories find":           {"POST", "v1/suggestions/categories/query", BodyObject, "RecommendationQueryRequest", "RecommendationQueryCategorySuggestionResponse", false},
		"suggestions keywords find":             {"POST", "v1/suggestions/keywords/query", BodyObject, "RecommendationQueryRequest", "RecommendationQueryKeywordSuggestionResponse", false},
		"suggestions phrases find":              {"POST", "v1/suggestions/phrases/query", BodyObject, "RecommendationQueryRequest", "RecommendationQueryPhraseSuggestionResponse", false},
		"suggestions target-cpas find":          {"POST", "v1/suggestions/target-cpas/query", BodyObject, "RecommendationQueryRequest", "RecommendationQueryTargetCpaSuggestionResponse", false},
		"change-history find":                   {"POST", "v1/change-history/query", BodyObject, "AuditQuery", "AuditSummaryResponse", false},
		"change-history view":                   {"GET", "v1/change-history/{detailId}", BodyNone, "", "ChangeDetailsResponse", false},
	}

	specs := platformReportsOptimizationEndpointSpecs()
	if got, want := len(specs), 24; got != want {
		t.Fatalf("reports/optimization spec count = %d, want %d", got, want)
	}
	for _, spec := range specs {
		key := strings.Join(spec.CommandPath, " ")
		want, ok := wants[key]
		if !ok {
			t.Fatalf("unexpected reports/optimization command %q", key)
		}
		if spec.Method != want.method || spec.Path != want.path || spec.BodyKind != want.bodyKind || spec.BodyType != want.bodyType || spec.ResponseType != want.responseType {
			t.Fatalf("%s contract = %s %s body=%q/%q response=%q", key, spec.Method, spec.Path, spec.BodyKind, spec.BodyType, spec.ResponseType)
		}
		if spec.Version != APIVersionPlatformV1 || spec.Context != ContextAdAccount {
			t.Fatalf("%s version/context = %q/%v", key, spec.Version, spec.Context)
		}
		isQuery := spec.Method == "POST" && strings.HasSuffix(spec.Path, "/query")
		if spec.BodyKind != BodyNone && spec.BodyOptional && !isQuery {
			t.Fatalf("%s body optional is only supported for POST /query endpoints", key)
		}
		if spec.RequiresConfirm != want.confirm {
			t.Fatalf("%s confirmation = %t, want %t", key, spec.RequiresConfirm, want.confirm)
		}
		wantRiskConfirm := strings.HasPrefix(spec.Path, "v1/recommendations/") && (strings.HasSuffix(spec.Path, "/apply") || strings.HasSuffix(spec.Path, "/dismiss"))
		if spec.RiskConfirm != wantRiskConfirm {
			t.Fatalf("%s risk confirmation = %t, want %t", key, spec.RiskConfirm, wantRiskConfirm)
		}
		if spec.Method == "POST" && strings.HasSuffix(spec.Path, "/query") && !spec.RetrySafe {
			t.Fatalf("%s must be retry-safe", key)
		}
		wantPaginate := spec.Name == "platform-get-change-history-detail"
		if spec.SupportsPaginate != wantPaginate {
			t.Fatalf("%s pagination support = %t, want %t", key, spec.SupportsPaginate, wantPaginate)
		}
		delete(wants, key)
	}
	if len(wants) != 0 {
		t.Fatalf("missing reports/optimization commands: %+v", wants)
	}
}

func TestPlatformReportsOptimizationMatchesIndependentFixture(t *testing.T) {
	file, err := os.Open("testdata/platform_v1_endpoints.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}

	fixture := map[string][]string{}
	for _, record := range records[1:] {
		if len(record) < 14 {
			t.Fatalf("fixture row has %d columns, want at least 14: %#v", len(record), record)
		}
		collection := record[0]
		path := record[4]
		if !strings.HasPrefix(path, "/v1/reports/") && collection != "insights-endpoints" && collection != "recommendations-endpoints" && collection != "suggestions-endpoints" && collection != "change-history-endpoints" {
			continue
		}
		fixture[record[12]] = record
	}
	if got, want := len(fixture), 24; got != want {
		t.Fatalf("reports/optimization fixture count = %d, want %d", got, want)
	}

	for _, spec := range platformReportsOptimizationEndpointSpecs() {
		command := strings.Join(spec.CommandPath, " ")
		record, ok := fixture[command]
		if !ok {
			t.Fatalf("%s missing from independent fixture", command)
		}
		if spec.Method != record[3] || spec.Path != strings.TrimPrefix(record[4], "/") {
			t.Fatalf("%s method/path = %s %s, fixture = %s %s", command, spec.Method, spec.Path, record[3], record[4])
		}
		wantBodyKind := BodyNone
		switch record[6] {
		case "JSON object":
			wantBodyKind = BodyObject
		case "JSON array":
			wantBodyKind = BodyArray
		case "none":
		default:
			t.Fatalf("%s unexpected fixture body kind %q", command, record[6])
		}
		if spec.BodyKind != wantBodyKind || spec.BodyType != fixtureNone(record[7]) {
			t.Fatalf("%s body = %q/%q, fixture = %q/%q", command, spec.BodyKind, spec.BodyType, wantBodyKind, record[7])
		}
		isQuery := spec.Method == "POST" && strings.HasSuffix(spec.Path, "/query")
		wantOptional := isQuery && record[8] != "yes"
		if spec.BodyKind == BodyNone {
			wantOptional = false
		}
		if spec.BodyOptional != wantOptional {
			t.Fatalf("%s body optional = %t, want %t (SDK body required = %q)", command, spec.BodyOptional, wantOptional, record[8])
		}
		if spec.ResponseType != strings.TrimPrefix(record[9], "200 ") {
			t.Fatalf("%s response = %q, fixture = %q", command, spec.ResponseType, record[9])
		}
		if spec.Context != ContextAdAccount || record[10] != "ad-account" {
			t.Fatalf("%s context = %v, fixture = %q", command, spec.Context, record[10])
		}
		if spec.RequiresConfirm != (record[11] == "yes") {
			t.Fatalf("%s confirmation = %t, fixture destructive = %q", command, spec.RequiresConfirm, record[11])
		}
		delete(fixture, command)
	}
	if len(fixture) != 0 {
		t.Fatalf("fixture commands missing from implementation: %+v", fixture)
	}
}

func fixtureNone(value string) string {
	if value == "none" {
		return ""
	}
	return value
}

func TestPlatformChangeHistoryDetailParameters(t *testing.T) {
	spec, ok := PlatformEndpointByCommandPath("change-history", "view")
	if !ok {
		t.Fatal("missing change-history view")
	}
	if len(spec.PathParams) != 1 || spec.PathParams[0].Name != "detailId" || spec.PathParams[0].Flag != "detail-id" || spec.PathParams[0].Type != ParamString || !spec.PathParams[0].Required {
		t.Fatalf("detail path params = %+v", spec.PathParams)
	}
	if len(spec.QueryParams) != 2 || spec.QueryParams[0].Name != "limit" || spec.QueryParams[0].Flag != "limit" || spec.QueryParams[0].Type != ParamInt || spec.QueryParams[1].Name != "offset" || spec.QueryParams[1].Flag != "offset" || spec.QueryParams[1].Type != ParamInt {
		t.Fatalf("detail query params = %+v", spec.QueryParams)
	}
	if spec.QueryParams[0].Default != 100 {
		t.Fatalf("detail limit default = %d, want 100", spec.QueryParams[0].Default)
	}
	if !spec.SupportsPaginate {
		t.Fatal("change-history view must support --paginate")
	}
}

func TestPlatformReportsOptimizationPayloadGuidance(t *testing.T) {
	tests := []struct {
		path        []string
		fileExample string
		want        []string
	}{
		{path: []string{"reports", "apps", "campaigns"}, fileExample: "report.json", want: []string{"nested timeRange", "{offset,pageSize}", "EMPTY_METRICS", "filters"}},
		{path: []string{"insights", "impression-share", "find"}, fileExample: "query.json", want: []string{"promotedObjectId", "UTC", "maximum 30 days", "FIRST_SLOT", "pageSize max 5000"}},
		{path: []string{"insights", "search-term-popularity", "find"}, fileExample: "query.json", want: []string{"timeRange", "WEEKLY_SUN_SAT", "MONTHLY", "2 sort fields"}},
		{path: []string{"suggestions", "phrases", "find"}, fileExample: "query.json", want: []string{"queryType SUGGESTION", "queryType SEARCH", "promotedObjectType", "exception"}},
		{path: []string{"suggestions", "keywords", "find"}, fileExample: "query.json", want: []string{"promotedObjectId", "promotedObjectType", "pageSize max 1000"}},
		{path: []string{"recommendations", "daily-budgets", "apply"}, fileExample: "recommendations.json", want: []string{"non-empty array", "require --confirm"}},
	}
	for _, test := range tests {
		spec, ok := PlatformEndpointByCommandPath(test.path...)
		if !ok {
			t.Fatalf("missing %q", strings.Join(test.path, " "))
		}
		if spec.BodyFileExample != test.fileExample {
			t.Errorf("%s body file example = %q, want %q", strings.Join(test.path, " "), spec.BodyFileExample, test.fileExample)
		}
		for _, want := range test.want {
			if !strings.Contains(spec.BodyHint, want) {
				t.Errorf("%s body hint = %q, want %q", strings.Join(test.path, " "), spec.BodyHint, want)
			}
		}
	}
}

func TestPlatformReportsOptimizationStarterPayloads(t *testing.T) {
	// Commands whose help must carry a starter payload verified against the
	// worked examples in Apple's Platform API documentation.
	required := [][]string{
		{"reports", "apps", "campaigns"},
		{"reports", "brands", "campaigns"},
		{"insights", "impression-share", "find"},
		{"insights", "search-term-popularity", "find"},
		{"suggestions", "phrases", "find"},
		{"suggestions", "keywords", "find"},
		{"recommendations", "daily-budgets", "find"},
		{"recommendations", "daily-budgets", "apply"},
		{"recommendations", "target-cpas", "apply"},
	}
	for _, path := range required {
		spec, ok := PlatformEndpointByCommandPath(path...)
		if !ok {
			t.Fatalf("missing %q", strings.Join(path, " "))
		}
		if strings.TrimSpace(spec.BodyExample) == "" {
			t.Errorf("%s has no starter payload", strings.Join(path, " "))
		}
	}

	for _, spec := range PlatformEndpointSpecs() {
		example := strings.TrimSpace(spec.BodyExample)
		if example == "" {
			continue
		}
		var payload any
		if err := json.Unmarshal([]byte(example), &payload); err != nil {
			t.Errorf("%s starter payload is not valid JSON: %v", spec.Name, err)
			continue
		}
		switch spec.BodyKind {
		case BodyObject:
			if _, ok := payload.(map[string]any); !ok {
				t.Errorf("%s starter payload must be a JSON object", spec.Name)
			}
		case BodyArray:
			if _, ok := payload.([]any); !ok {
				t.Errorf("%s starter payload must be a JSON array", spec.Name)
			}
		}
	}
}

func TestInsightStarterPayloadsUseUTCTimeZone(t *testing.T) {
	paths := [][]string{
		{"insights", "impression-share", "find"},
		{"insights", "search-term-popularity", "find"},
	}

	for _, path := range paths {
		t.Run(strings.Join(path, " "), func(t *testing.T) {
			spec, ok := PlatformEndpointByCommandPath(path...)
			if !ok {
				t.Fatalf("missing %q", strings.Join(path, " "))
			}

			var payload struct {
				TimeRange struct {
					TimeZone string `json:"timeZone"`
				} `json:"timeRange"`
			}
			if err := json.Unmarshal([]byte(spec.BodyExample), &payload); err != nil {
				t.Fatalf("starter payload is not a JSON object: %v", err)
			}
			if payload.TimeRange.TimeZone != "UTC" {
				t.Errorf("timeRange.timeZone = %q, want UTC", payload.TimeRange.TimeZone)
			}
		})
	}
}

func TestSearchTermPopularityStarterPayloadUsesRuntimeSortKey(t *testing.T) {
	spec, ok := PlatformEndpointByCommandPath("insights", "search-term-popularity", "find")
	if !ok {
		t.Fatal("missing search-term-popularity find")
	}

	var payload struct {
		Sorting []map[string]json.RawMessage `json:"sorting"`
	}
	if err := json.Unmarshal([]byte(spec.BodyExample), &payload); err != nil {
		t.Fatalf("starter payload is not a JSON object: %v", err)
	}
	if len(payload.Sorting) != 1 {
		t.Fatalf("sorting has %d entries, want 1", len(payload.Sorting))
	}
	if _, ok := payload.Sorting[0]["order"]; ok {
		t.Fatal("search term popularity starter payload uses documentation-only order key")
	}
	if got := string(payload.Sorting[0]["sortOrder"]); got != `"ASC"` {
		t.Fatalf("sorting sortOrder = %s, want ASC", got)
	}
}

func TestRecommendationDismissStarterPayloadsExcludeApplyOnlyFields(t *testing.T) {
	tests := []struct {
		path       []string
		applyField string
	}{
		{path: []string{"recommendations", "daily-budgets", "dismiss"}, applyField: "appliedDailyBudget"},
		{path: []string{"recommendations", "target-cpas", "dismiss"}, applyField: "appliedTargetCPA"},
	}

	for _, test := range tests {
		t.Run(strings.Join(test.path, " "), func(t *testing.T) {
			spec, ok := PlatformEndpointByCommandPath(test.path...)
			if !ok {
				t.Fatalf("missing %q", strings.Join(test.path, " "))
			}

			var payload []map[string]json.RawMessage
			if err := json.Unmarshal([]byte(spec.BodyExample), &payload); err != nil {
				t.Fatalf("starter payload is not a JSON object array: %v", err)
			}
			if len(payload) != 1 {
				t.Fatalf("starter payload has %d items, want 1", len(payload))
			}
			if _, ok := payload[0][test.applyField]; ok {
				t.Fatalf("dismiss starter payload includes apply-only field %q", test.applyField)
			}
			for _, field := range []string{"id", "promotedObjectId", "promotedObjectType"} {
				if _, ok := payload[0][field]; !ok {
					t.Errorf("dismiss starter payload is missing %q", field)
				}
			}
		})
	}
}
