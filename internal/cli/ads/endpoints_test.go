package ads

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/appleads"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

func TestAdsV5CommandRegistersEveryLegacyEndpointSpec(t *testing.T) {
	root := AdsCommand()
	for _, spec := range appleads.EndpointSpecs() {
		path := append([]string{"v5"}, spec.CommandPath...)
		cmd := findCommand(root, path...)
		if cmd == nil {
			t.Fatalf("missing command asc ads %s", strings.Join(path, " "))
		}
		if cmd.Exec == nil {
			t.Fatalf("command asc ads %s has no Exec", strings.Join(path, " "))
		}
		assertSpecFlags(t, cmd, spec)

		if spec.DefaultListAlias {
			alias := findCommand(root, "v5", spec.CommandPath[0])
			if alias == nil {
				t.Fatalf("missing default list alias asc ads v5 %s", spec.CommandPath[0])
			}
			if alias.Exec == nil {
				t.Fatalf("default list alias asc ads v5 %s has no Exec", spec.CommandPath[0])
			}
			assertSpecFlags(t, alias, spec)
		}
	}
}

func TestAdsRootRegistersPlatformV1AsDefault(t *testing.T) {
	root := AdsCommand()
	if findCommand(root, "platform") != nil {
		t.Fatal("nested Platform compatibility namespace must not exist before release")
	}
	for _, spec := range appleads.PlatformEndpointSpecs() {
		cmd := findCommand(root, spec.CommandPath...)
		if cmd == nil || cmd.Exec == nil {
			t.Fatalf("missing executable command asc ads %s", strings.Join(spec.CommandPath, " "))
		}
		assertSpecFlags(t, cmd, spec)
		if spec.Context == appleads.ContextAdAccount && cmd.FlagSet.Lookup("ad-account") == nil {
			t.Fatalf("asc ads %s missing --ad-account", strings.Join(spec.CommandPath, " "))
		}
		if cmd.FlagSet.Lookup("org") != nil {
			t.Fatalf("asc ads %s must not expose legacy --org", strings.Join(spec.CommandPath, " "))
		}
		if (strings.Join(spec.CommandPath, " ") == "ad-accounts view" || strings.Join(spec.CommandPath, " ") == "ad-accounts update") && cmd.FlagSet.Lookup("id") != nil {
			t.Fatalf("asc ads %s must use --ad-account for both the path and context", strings.Join(spec.CommandPath, " "))
		}
	}
	if findCommand(root, "api", "request") == nil {
		t.Fatal("missing root Platform v1 raw request command")
	}
	if findCommand(root, "v5", "api", "request") == nil {
		t.Fatal("missing deprecated v5 raw request command")
	}
}

func TestAdsRawResponseCommandsExposeJSONOnlyOutput(t *testing.T) {
	tests := []struct {
		path []string
		args []string
	}{
		{path: []string{"campaigns", "find"}},
		{path: []string{"reports", "apps", "campaigns"}},
		{path: []string{"api", "request"}},
		{path: []string{"assets", "upload"}},
		{path: []string{"campaigns", "pause"}, args: []string{"--campaign", "1"}},
		{path: []string{"v5", "campaigns", "list"}},
		{path: []string{"v5", "campaigns", "pause"}, args: []string{"--campaign", "1", "--confirm"}},
		{path: []string{"v5", "reports", "preset"}},
		{path: []string{"v5", "api", "request"}},
	}
	for _, test := range tests {
		t.Run(strings.Join(test.path, " "), func(t *testing.T) {
			root := AdsCommand()
			path := test.path
			cmd := findCommand(root, path...)
			if cmd == nil {
				t.Fatalf("missing command asc ads %s", strings.Join(path, " "))
			}
			output := cmd.FlagSet.Lookup("output")
			if output == nil {
				t.Fatalf("asc ads %s missing --output", strings.Join(path, " "))
			}
			if output.DefValue != "json" {
				t.Fatalf("asc ads %s --output default = %q, want json", strings.Join(path, " "), output.DefValue)
			}
			args := append(append([]string(nil), test.args...), "--output", "table")
			if err := cmd.Parse(args); err != nil {
				t.Fatalf("asc ads %s parse error: %v", strings.Join(path, " "), err)
			}
			if err := cmd.Exec(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "unsupported format: table") {
				t.Fatalf("asc ads %s accepted --output table for a raw response: %v", strings.Join(path, " "), err)
			}
		})
	}

	for _, format := range []string{"table", "markdown"} {
		output := format
		pretty := false
		if _, err := validateAdsRawOutput(shared.OutputFlags{Output: &output, Pretty: &pretty}); err == nil || !strings.Contains(err.Error(), "unsupported format: "+format) {
			t.Fatalf("validateAdsRawOutput(%q) error = %v", format, err)
		}
	}
}

func TestPlatformAppSearchValidationAndRepeatedStoreFronts(t *testing.T) {
	spec, ok := appleads.PlatformEndpointByCommandPath("apps", "search")
	if !ok {
		t.Fatal("missing platform apps search")
	}
	fs, flags := bindEndpointFlags(spec, "test")
	if _, err := collectQuery(spec, flags); err == nil || !strings.Contains(err.Error(), "--query, --cpids, or --return-owned-apps") {
		t.Fatalf("empty search error = %v", err)
	}
	if err := fs.Set("query", "ab"); err != nil {
		t.Fatal(err)
	}
	if _, err := collectQuery(spec, flags); err == nil || !strings.Contains(err.Error(), "at least 3 characters") {
		t.Fatalf("short query error = %v", err)
	}
	if err := fs.Set("query", "test app"); err != nil {
		t.Fatal(err)
	}
	if err := fs.Set("store-fronts", "us, gB"); err != nil {
		t.Fatal(err)
	}
	if err := fs.Set("store-fronts", "ca"); err != nil {
		t.Fatal(err)
	}
	query, err := collectQuery(spec, flags)
	if err != nil {
		t.Fatalf("collectQuery() error: %v", err)
	}
	if got := query["storeFronts"]; len(got) != 3 || got[0] != "US" || got[1] != "GB" || got[2] != "CA" {
		t.Fatalf("storeFronts = %#v, want repeated US, GB, and CA", got)
	}
}

func TestPlatformAppSearchRejectsInvalidStoreFronts(t *testing.T) {
	spec, _ := appleads.PlatformEndpointByCommandPath("apps", "search")
	for _, storefronts := range []string{"US,,GB", "USA", "U1"} {
		t.Run(storefronts, func(t *testing.T) {
			fs, flags := bindEndpointFlags(spec, "test")
			if err := fs.Set("query", "test"); err != nil {
				t.Fatal(err)
			}
			if err := fs.Set("store-fronts", storefronts); err != nil {
				t.Fatal(err)
			}
			if _, err := collectQuery(spec, flags); err == nil {
				t.Fatalf("storefronts %q unexpectedly accepted", storefronts)
			}
		})
	}
}

func TestPlatformAppSearchRejectsInvalidRepeatedStoreFrontOccurrence(t *testing.T) {
	spec, ok := appleads.PlatformEndpointByCommandPath("apps", "search")
	if !ok {
		t.Fatal("missing platform apps search")
	}
	fs, flags := bindEndpointFlags(spec, "test")
	if err := fs.Set("query", "test"); err != nil {
		t.Fatal(err)
	}
	if err := fs.Set("store-fronts", "US"); err != nil {
		t.Fatal(err)
	}
	if err := fs.Set("store-fronts", "CA,USA"); err != nil {
		t.Fatal(err)
	}
	if _, err := collectQuery(spec, flags); err == nil {
		t.Fatal("invalid storefront in a later repeated occurrence unexpectedly accepted")
	}
}

func TestEndpointHelpDocumentsJSONBodyMetadata(t *testing.T) {
	root := AdsCommand()
	specs := []struct {
		prefix []string
		specs  []appleads.EndpointSpec
	}{
		{prefix: nil, specs: appleads.PlatformEndpointSpecs()},
		{prefix: []string{"v5"}, specs: appleads.EndpointSpecs()},
	}
	for _, group := range specs {
		for _, spec := range group.specs {
			if spec.BodyKind == appleads.BodyNone {
				continue
			}
			path := append(append([]string(nil), group.prefix...), spec.CommandPath...)
			cmd := findCommand(root, path...)
			if cmd == nil {
				t.Fatalf("missing command asc ads %s", strings.Join(path, " "))
			}
			for _, want := range []string{
				"Schema: " + spec.BodyType,
				"Shape: " + bodyShape(spec.BodyKind),
				"Required: " + bodyRequired(spec),
			} {
				if !strings.Contains(cmd.LongHelp, want) {
					t.Fatalf("asc ads %s LongHelp = %q, want %q", strings.Join(path, " "), cmd.LongHelp, want)
				}
			}
		}
	}
}

func TestEndpointHelpUsesMultipartShapeWithoutJSONPrefix(t *testing.T) {
	help := endpointBodyHelp(appleads.EndpointSpec{
		BodyKind: appleads.BodyMultipart,
		BodyType: "UploadAsset",
	})
	if !strings.Contains(help, "Schema: UploadAsset") {
		t.Fatalf("multipart help = %q, want schema", help)
	}
	if !strings.Contains(help, "Shape: multipart/form-data") {
		t.Fatalf("multipart help = %q, want multipart/form-data shape", help)
	}
	if strings.Contains(help, "JSON multipart") {
		t.Fatalf("multipart help = %q, must not call multipart JSON", help)
	}
}

func TestEndpointHelpKeepsArraySchemaReadable(t *testing.T) {
	root := AdsCommand()
	cmd := findCommand(root, "v5", "targeting-keywords", "create-bulk")
	if cmd == nil {
		t.Fatal("missing targeting-keywords create-bulk")
	}
	for _, want := range []string{"Schema: [Keyword]", "Shape: JSON array", "Required: yes"} {
		if !strings.Contains(cmd.LongHelp, want) {
			t.Fatalf("targeting-keywords create-bulk LongHelp = %q, want %q", cmd.LongHelp, want)
		}
	}
}

func TestPlatformEndpointHelpIncludesAgentPayloadGuidance(t *testing.T) {
	root := AdsCommand()
	tests := []struct {
		path []string
		want []string
	}{
		{
			path: []string{"campaigns", "create"},
			want: []string{
				"Required fields:",
				"adAccountId",
				"promotedObjectType",
				"promotedObjectId",
				`{"status":"PAUSED"}`,
				"--confirm",
			},
		},
		{
			path: []string{"targeting-keywords", "find"},
			want: []string{
				"--file query.json",
				"id, adGroupId, or campaignId",
			},
		},
		{
			path: []string{"negative-keywords", "find"},
			want: []string{
				"--file query.json",
				"campaignId with an adGroupId IS_NULL filter",
			},
		},
		{
			path: []string{"insights", "search-term-popularity", "find"},
			want: []string{
				"Starter payload (query.json):",
				`"granularity": "WEEKLY_SUN_SAT"`,
			},
		},
		{
			path: []string{"reports", "apps", "campaigns"},
			want: []string{
				"Starter payload (report.json):",
				`"timeZone": "ORTZ"`,
			},
		},
		{
			path: []string{"budget-orders", "create"},
			want: []string{
				"--confirm",
				"line of credit (LOC)",
			},
		},
	}
	for _, test := range tests {
		cmd := findCommand(root, test.path...)
		if cmd == nil {
			t.Fatalf("missing command asc ads %s", strings.Join(test.path, " "))
		}
		for _, want := range test.want {
			if !strings.Contains(cmd.LongHelp, want) {
				t.Errorf("asc ads %s LongHelp = %q, want %q", strings.Join(test.path, " "), cmd.LongHelp, want)
			}
		}
		if test.path[len(test.path)-1] == "find" && strings.Contains(cmd.LongHelp, "[--file query.json]") {
			t.Errorf("asc ads %s LongHelp = %q, selector file must be shown as required", strings.Join(test.path, " "), cmd.LongHelp)
		}
	}
}

func TestPlatformReportAndOptimizationHelpIncludesPayloadRules(t *testing.T) {
	root := AdsCommand()
	tests := []struct {
		path []string
		want []string
	}{
		{path: []string{"reports", "apps", "campaigns"}, want: []string{"--file report.json", "Schema: AppsReportingRequest", "nested timeRange", "{offset,pageSize}"}},
		{path: []string{"insights", "impression-share", "find"}, want: []string{"--file query.json", "promotedObjectId", "FIRST_SLOT", "maximum 30 days"}},
		{path: []string{"suggestions", "phrases", "find"}, want: []string{"queryType SUGGESTION", "queryType SEARCH", "Apple's generic request schema"}},
		{path: []string{"recommendations", "daily-budgets", "apply"}, want: []string{"--file recommendations.json", "non-empty array", "--confirm"}},
	}
	for _, test := range tests {
		command := findCommand(root, test.path...)
		if command == nil {
			t.Fatalf("missing asc ads %s", strings.Join(test.path, " "))
		}
		for _, want := range test.want {
			if !strings.Contains(command.LongHelp, want) {
				t.Errorf("asc ads %s help = %q, want %q", strings.Join(test.path, " "), command.LongHelp, want)
			}
		}
	}
}

func TestPlatformAppSearchHelpDocumentsMinimumQueryLength(t *testing.T) {
	command := findCommand(AdsCommand(), "apps", "search")
	if command == nil {
		t.Fatal("missing asc ads apps search")
	}
	for _, want := range []string{"at least 3 alphanumeric characters", "2 for CJK", "punctuation-only"} {
		if !strings.Contains(command.LongHelp, want) {
			t.Fatalf("asc ads apps search help = %q, want %q", command.LongHelp, want)
		}
	}
}

func TestPlatformNegativeKeywordResourceFlagsAreSemantic(t *testing.T) {
	root := AdsCommand()
	for _, action := range []string{"view", "update", "delete"} {
		cmd := findCommand(root, "negative-keywords", action)
		if cmd == nil {
			t.Fatalf("missing direct v1 negative-keywords %s command", action)
		}
		if cmd.FlagSet.Lookup("negative-keyword") == nil {
			t.Fatalf("negative-keywords %s missing --negative-keyword", action)
		}
		if cmd.FlagSet.Lookup("keyword") != nil {
			t.Fatalf("negative-keywords %s must not expose the targeting --keyword flag", action)
		}
	}

	legacy := findCommand(root, "v5", "campaign-negative-keywords", "view")
	if legacy == nil {
		t.Fatal("missing deprecated v5 campaign-negative-keywords view command")
	}
	if legacy.FlagSet.Lookup("negative-keyword") == nil || legacy.FlagSet.Lookup("keyword") == nil {
		t.Fatal("deprecated v5 negative keyword command must keep --keyword as a compatibility alias")
	}
}

func TestPlatformKeywordQueriesRequireSelectorBodyBeforeAuth(t *testing.T) {
	setAdsResolverTestEnv(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing-config.json"))
	root := AdsCommand()
	for _, path := range [][]string{
		{"targeting-keywords", "find"},
		{"negative-keywords", "find"},
	} {
		spec, ok := appleads.PlatformEndpointByCommandPath(path...)
		if !ok {
			t.Fatalf("missing platform endpoint %q", strings.Join(path, " "))
		}
		if !spec.CLIRequiresBody {
			t.Fatalf("%q must require a selector body at the CLI boundary", strings.Join(path, " "))
		}
		cmd := findCommand(root, path...)
		if cmd == nil || cmd.FlagSet.Lookup("file") == nil {
			t.Fatalf("asc ads %s must expose --file", strings.Join(path, " "))
		}
		_, flags := bindEndpointFlags(spec, strings.Join(path, " "))
		err := executeEndpoint(context.Background(), spec, flags)
		if !errors.Is(err, flag.ErrHelp) {
			t.Errorf("%q missing body error = %v, want pre-auth --file usage error", strings.Join(path, " "), err)
		}
		if got, want := err.Error(), flag.ErrHelp.Error(); got != want {
			t.Errorf("%q missing body error = %q, want exact %q", strings.Join(path, " "), got, want)
		}
	}
}

func TestPlatformQueryMigrationValidation(t *testing.T) {
	var querySpecs []appleads.EndpointSpec
	schemaCounts := map[string]int{}
	for _, spec := range appleads.PlatformEndpointSpecs() {
		if spec.Method == "POST" && strings.HasSuffix(spec.Path, "/query") {
			querySpecs = append(querySpecs, spec)
			schemaCounts[spec.BodyType]++
		}
	}
	if got, want := len(querySpecs), 38; got != want {
		t.Fatalf("Platform query endpoint count = %d, want %d", got, want)
	}
	wantSchemaCounts := map[string]int{
		"QueryRequest":                        16,
		"AppsReportingRequest":                5,
		"BrandsReportingRequest":              5,
		"RecommendationQueryRequest":          6,
		"AuditQuery":                          1,
		"CreativeRejectionReasonQueryRequest": 1,
		"EligibilityQueryRequest":             1,
		"ImpressionShareQueryRequest":         1,
		"PolicyAssignmentQueryRequest":        1,
		"SearchTermPopularityQueryRequest":    1,
	}
	if !reflect.DeepEqual(schemaCounts, wantSchemaCounts) {
		t.Fatalf("Platform query schemas = %#v, want %#v", schemaCounts, wantSchemaCounts)
	}
	for schema := range wantSchemaCounts {
		wantFields := schema == "AppsReportingRequest" || schema == "BrandsReportingRequest"
		wantTimeRange := wantFields || schema == "ImpressionShareQueryRequest" || schema == "SearchTermPopularityQueryRequest"
		wantReport := wantFields
		if got := platformQuerySupportsFields(schema); got != wantFields {
			t.Errorf("platformQuerySupportsFields(%q) = %v, want %v", schema, got, wantFields)
		}
		if got := platformQuerySupportsTimeRange(schema); got != wantTimeRange {
			t.Errorf("platformQuerySupportsTimeRange(%q) = %v, want %v", schema, got, wantTimeRange)
		}
		if got := platformQueryIsReport(schema); got != wantReport {
			t.Errorf("platformQueryIsReport(%q) = %v, want %v", schema, got, wantReport)
		}
	}

	validV1 := json.RawMessage(`{"filters":[{"field":"id","operator":"EQUALS","value":"123"}],"sorting":[{"field":"id","order":"DESC"}],"pagination":{"offset":0,"pageSize":5}}`)
	legacyConditions := json.RawMessage(`{"conditions":null}`)
	for _, spec := range querySpecs {
		t.Run(strings.Join(spec.CommandPath, "-"), func(t *testing.T) {
			if err := validateEndpointBody(spec, validV1, false); err != nil {
				t.Fatalf("valid Platform v1 query rejected: %v", err)
			}
			fieldsErr := validatePlatformQueryMigration(spec, json.RawMessage(`{"fields":null}`))
			if platformQuerySupportsFields(spec.BodyType) {
				if fieldsErr != nil {
					t.Fatalf("%s must accept its documented top-level fields member: %v", spec.BodyType, fieldsErr)
				}
			} else if fieldsErr == nil || !strings.Contains(fieldsErr.Error(), `has no field-projection member`) {
				t.Fatalf("%s fields error = %v, want schema-specific rejection", spec.BodyType, fieldsErr)
			}
			err := validateEndpointBody(spec, legacyConditions, false)
			if err == nil || !strings.Contains(err.Error(), `"conditions" -> "filters"`) {
				t.Fatalf("legacy conditions error = %v, want migration hint", err)
			}
		})
	}

	campaigns, ok := appleads.PlatformEndpointByCommandPath("campaigns", "find")
	if !ok {
		t.Fatal("missing campaigns find")
	}
	if err := validateEndpointBody(campaigns, json.RawMessage(`{"filters":[{"field":"name","operator":"STARTS_WITH","value":"x"}],"sorting":[{"field":"id","order":"DESC"}]}`), false); err != nil {
		t.Fatalf("renamed v1 operator and sort order rejected: %v", err)
	}
	for _, test := range []struct {
		name string
		body string
		want string
	}{
		{name: "filter values", body: `{"filters":[{"field":"id","operator":"IN","values":null}]}`, want: `"values" -> "value"`},
		{name: "order by", body: `{"orderBy":null}`, want: `"orderBy" -> "sorting"`},
		{name: "sort order", body: `{"sorting":[{"field":"id","sortOrder":null}]}`, want: `"sortOrder" -> "order"`},
		{name: "pagination limit", body: `{"pagination":{"limit":null}}`, want: `"pagination.limit" -> "pagination.pageSize"`},
		{name: "legacy starts with", body: `{"filters":[{"field":"name","operator":"STARTSWITH","value":"x"}]}`, want: `"STARTSWITH"/"ENDSWITH" -> "STARTS_WITH"/"ENDS_WITH"`},
		{name: "legacy sort direction", body: `{"sorting":[{"field":"id","order":"DESCENDING"}]}`, want: `"ASCENDING"/"DESCENDING" -> "ASC"/"DESC"`},
		{name: "unsupported field projection", body: `{"fields":null}`, want: `QueryRequest has no field-projection member`},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validateEndpointBody(campaigns, json.RawMessage(test.body), false)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validateEndpointBody() error = %v, want %q", err, test.want)
			}
		})
	}

	report, ok := appleads.PlatformEndpointByCommandPath("reports", "apps", "campaigns")
	if !ok {
		t.Fatal("missing app campaign report")
	}
	if err := validateEndpointBody(report, json.RawMessage(`{"fields":["campaignId"],"timeRange":{"start":"2026-08-01","end":"2026-08-02"},"options":{"includeRows":["GRAND_TOTAL"]}}`), false); err != nil {
		t.Fatalf("valid v1 report fields rejected: %v", err)
	}
	reportErr := validateEndpointBody(report, json.RawMessage(`{"startTime":null,"endTime":null,"timeZone":null,"granularity":null,"returnRecordsWithNoMetrics":null,"returnRowTotals":null,"returnGrandTotals":null,"selector":{"fields":null,"conditions":[{"field":"id","operator":"STARTSWITH","values":null}],"orderBy":[{"field":"id","sortOrder":"DESCENDING"}],"pagination":{"limit":null}}}`), false)
	for _, want := range []string{
		`"selector"`,
		`"conditions" -> "filters"`,
		`"values" -> "value"`,
		`"orderBy" -> "sorting"`,
		`"sortOrder" -> "order"`,
		`"pagination.limit" -> "pagination.pageSize"`,
		`"STARTSWITH"/"ENDSWITH" -> "STARTS_WITH"/"ENDS_WITH"`,
		`"ASCENDING"/"DESCENDING" -> "ASC"/"DESC"`,
		`"selector.fields" -> top-level "fields"`,
		`"startTime" -> "timeRange.start"`,
		`"endTime" -> "timeRange.end"`,
		`"timeZone" -> "timeRange.timeZone"`,
		`"granularity" -> "timeRange.granularity"`,
		`"returnRecordsWithNoMetrics": true -> add "EMPTY_METRICS"`,
		`remove "returnRowTotals"`,
		`"returnGrandTotals": true -> add "GRAND_TOTAL"`,
	} {
		if reportErr == nil || !strings.Contains(reportErr.Error(), want) {
			t.Fatalf("legacy report selector error = %v, want complete migration hint containing %q", reportErr, want)
		}
	}

	brandReport, ok := appleads.PlatformEndpointByCommandPath("reports", "brands", "campaigns")
	if !ok {
		t.Fatal("missing brand campaign report")
	}
	if err := validateEndpointBody(brandReport, json.RawMessage(`{"returnRecordsWithNoMetrics":null}`), false); err == nil || !strings.Contains(err.Error(), `BrandsOptions doesn't support EMPTY_METRICS`) {
		t.Fatalf("brand empty-metrics migration error = %v, want unsupported hint", err)
	}

	impressionShare, ok := appleads.PlatformEndpointByCommandPath("insights", "impression-share", "find")
	if !ok {
		t.Fatal("missing impression-share query")
	}
	impressionErr := validateEndpointBody(impressionShare, json.RawMessage(`{"name":null,"dateRange":null,"startTime":null}`), false)
	for _, want := range []string{`remove "name"`, `"dateRange" -> explicit "timeRange.start" and "timeRange.end"`, `"startTime" -> "timeRange.start"`} {
		if impressionErr == nil || !strings.Contains(impressionErr.Error(), want) {
			t.Fatalf("impression-share migration error = %v, want %q", impressionErr, want)
		}
	}

	createAd, ok := appleads.PlatformEndpointByCommandPath("ads", "create")
	if !ok {
		t.Fatal("missing ads create")
	}
	if err := validateEndpointBody(createAd, json.RawMessage(`{"conditions":null,"values":null,"orderBy":null}`), true); err != nil {
		t.Fatalf("non-query body must not use the query migration guard: %v", err)
	}

	legacyQuery := appleads.EndpointSpec{Version: appleads.APIVersionCampaignV5, Method: "POST", Path: "v5/campaigns/query"}
	if err := validateEndpointBody(legacyQuery, json.RawMessage(`{"conditions":null,"orderBy":null}`), false); err != nil {
		t.Fatalf("legacy v5 query body must not use the Platform API migration guard: %v", err)
	}
}

func TestPlatformQueryMigrationValidationPrecedesAuth(t *testing.T) {
	setAdsResolverTestEnv(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing-config.json"))
	spec, ok := appleads.PlatformEndpointByCommandPath("campaigns", "find")
	if !ok {
		t.Fatal("missing campaigns find")
	}
	file := filepath.Join(t.TempDir(), "query.json")
	if err := os.WriteFile(file, []byte(`{"conditions":[{"field":"id","operator":"EQUALS","values":["123"]}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	fs, flags := bindEndpointFlags(spec, "campaigns find")
	if err := fs.Set("file", file); err != nil {
		t.Fatal(err)
	}
	err := executeEndpoint(context.Background(), spec, flags)
	if err == nil || !strings.Contains(err.Error(), `"conditions" -> "filters"`) || strings.Contains(err.Error(), "configuration not found") {
		t.Fatalf("executeEndpoint() error = %v, want migration validation before auth", err)
	}
}

func TestPlatformKeywordQuerySelectorValidation(t *testing.T) {
	tests := []struct {
		name    string
		path    []string
		body    string
		wantErr string
	}{
		{
			name:    "targeting empty filters",
			path:    []string{"targeting-keywords", "find"},
			body:    `{"filters":[]}`,
			wantErr: "id, adGroupId, or campaignId",
		},
		{
			name:    "targeting legacy v5 conditions selector",
			path:    []string{"targeting-keywords", "find"},
			body:    `{"conditions":[{"field":"campaignId","operator":"EQUALS","values":["campaign-1"]}]}`,
			wantErr: `"conditions" -> "filters"`,
		},
		{
			name:    "negative legacy v5 conditions selector",
			path:    []string{"negative-keywords", "find"},
			body:    `{"conditions":[{"field":"adGroupId","operator":"EQUALS","values":["ad-group-1"]}]}`,
			wantErr: `"conditions" -> "filters"`,
		},
		{
			name:    "targeting null legacy conditions key",
			path:    []string{"targeting-keywords", "find"},
			body:    `{"filters":[{"field":"campaignId","operator":"EQUALS","value":"campaign-1"}],"conditions":null}`,
			wantErr: `"conditions" -> "filters"`,
		},
		{
			name:    "targeting null legacy values key in filter",
			path:    []string{"targeting-keywords", "find"},
			body:    `{"filters":[{"field":"campaignId","operator":"EQUALS","value":"campaign-1","values":null}]}`,
			wantErr: `"values" -> "value"`,
		},
		{
			name:    "targeting legacy v5 values in filter",
			path:    []string{"targeting-keywords", "find"},
			body:    `{"filters":[{"field":"campaignId","operator":"IN","values":["campaign-1"]}]}`,
			wantErr: `"values" -> "value"`,
		},
		{
			name:    "negative legacy v5 values in filter",
			path:    []string{"negative-keywords", "find"},
			body:    `{"filters":[{"field":"adGroupId","operator":"IN","values":["ad-group-1"]}]}`,
			wantErr: `"values" -> "value"`,
		},
		{
			name:    "targeting irrelevant filter",
			path:    []string{"targeting-keywords", "find"},
			body:    `{"filters":[{"field":"name","operator":"EQUALS","value":"ignored"}]}`,
			wantErr: "id, adGroupId, or campaignId",
		},
		{
			name: "targeting id",
			path: []string{"targeting-keywords", "find"},
			body: `{"filters":[{"field":"id","operator":"EQUALS","value":"keyword-1"}]}`,
		},
		{
			name: "targeting ad group",
			path: []string{"targeting-keywords", "find"},
			body: `{"filters":[{"field":"adGroupId","operator":"EQUALS","value":"ad-group-1"}]}`,
		},
		{
			name: "targeting campaign",
			path: []string{"targeting-keywords", "find"},
			body: `{"filters":[{"field":"campaignId","operator":"EQUALS","value":"campaign-1"}]}`,
		},
		{
			name:    "negative empty filters",
			path:    []string{"negative-keywords", "find"},
			body:    `{"filters":[]}`,
			wantErr: "id or adGroupId",
		},
		{
			name:    "negative irrelevant filter",
			path:    []string{"negative-keywords", "find"},
			body:    `{"filters":[{"field":"name","operator":"EQUALS","value":"ignored"}]}`,
			wantErr: "id or adGroupId",
		},
		{
			name: "negative id",
			path: []string{"negative-keywords", "find"},
			body: `{"filters":[{"field":"id","operator":"EQUALS","value":"negative-keyword-1"}]}`,
		},
		{
			name: "negative ad group",
			path: []string{"negative-keywords", "find"},
			body: `{"filters":[{"field":"adGroupId","operator":"EQUALS","value":"ad-group-1"}]}`,
		},
		{
			name:    "negative campaign without null ad group",
			path:    []string{"negative-keywords", "find"},
			body:    `{"filters":[{"field":"campaignId","operator":"EQUALS","value":"campaign-1"}]}`,
			wantErr: "campaignId plus an adGroupId filter with operator IS_NULL",
		},
		{
			name:    "negative null ad group without campaign",
			path:    []string{"negative-keywords", "find"},
			body:    `{"filters":[{"field":"adGroupId","operator":"IS_NULL"}]}`,
			wantErr: "campaignId plus an adGroupId filter with operator IS_NULL",
		},
		{
			name: "negative campaign",
			path: []string{"negative-keywords", "find"},
			body: `{"filters":[{"field":"campaignId","operator":"EQUALS","value":"campaign-1"},{"field":"adGroupId","operator":"IS_NULL"}]}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			spec, ok := appleads.PlatformEndpointByCommandPath(test.path...)
			if !ok {
				t.Fatalf("missing platform endpoint %q", strings.Join(test.path, " "))
			}
			err := validateEndpointBody(spec, json.RawMessage(test.body), false)
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("validateEndpointBody() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("validateEndpointBody() error = %v, want %q", err, test.wantErr)
			}
		})
	}
}

func TestPlatformKeywordQuerySelectorValidationPrecedesAuth(t *testing.T) {
	setAdsResolverTestEnv(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing-config.json"))
	for _, path := range [][]string{
		{"targeting-keywords", "find"},
		{"negative-keywords", "find"},
	} {
		t.Run(strings.Join(path, "-"), func(t *testing.T) {
			spec, ok := appleads.PlatformEndpointByCommandPath(path...)
			if !ok {
				t.Fatalf("missing platform endpoint %q", strings.Join(path, " "))
			}
			file := filepath.Join(t.TempDir(), "query.json")
			if err := os.WriteFile(file, []byte(`{"filters":[{"field":"name","operator":"EQUALS","value":"ignored"}]}`), 0o600); err != nil {
				t.Fatal(err)
			}
			fs, flags := bindEndpointFlags(spec, strings.Join(path, " "))
			if err := fs.Set("file", file); err != nil {
				t.Fatal(err)
			}
			err := executeEndpoint(context.Background(), spec, flags)
			if err == nil || !strings.Contains(err.Error(), "id") || strings.Contains(err.Error(), "configuration not found") {
				t.Fatalf("executeEndpoint() error = %v, want selector validation before auth", err)
			}
		})
	}
}

func TestPlatformCampaignAndBudgetRiskConfirmationPrecedesAuth(t *testing.T) {
	setAdsResolverTestEnv(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing-config.json"))

	campaign, ok := appleads.PlatformEndpointByCommandPath("campaigns", "create")
	if !ok {
		t.Fatal("missing platform campaign create")
	}
	if !campaign.RiskConfirm || campaign.RiskConfirmBodyField != "status" || campaign.RiskConfirmBodyValue != "PAUSED" {
		t.Fatalf("campaign risk metadata = %+v, want paused exception", campaign)
	}
	for _, test := range []struct {
		name    string
		payload string
		confirm bool
		want    string
	}{
		{name: "missing status", payload: `{ "name": "agent-test" }`, want: `--confirm is required unless status is explicitly "PAUSED"; otherwise acknowledge potential Apple Ads spend, billing, delivery, targeting, or access impact`},
		{name: "enabled", payload: `{ "status": "ENABLED" }`, want: `--confirm is required unless status is explicitly "PAUSED"; otherwise acknowledge potential Apple Ads spend, billing, delivery, targeting, or access impact`},
		{name: "paused", payload: `{ "status": "PAUSED" }`, want: "ads: configuration not found; run 'asc ads auth login' to store Apple Ads credentials, set ASC_ADS_* environment credentials, or pass --ads-profile"},
		{name: "enabled confirmed", payload: `{ "status": "ENABLED" }`, confirm: true, want: "ads: configuration not found; run 'asc ads auth login' to store Apple Ads credentials, set ASC_ADS_* environment credentials, or pass --ads-profile"},
	} {
		t.Run(test.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "campaign.json")
			if err := os.WriteFile(file, []byte(test.payload), 0o600); err != nil {
				t.Fatal(err)
			}
			_, flags := bindEndpointFlags(campaign, "campaigns create")
			*flags.file = file
			if flags.confirm != nil {
				*flags.confirm = test.confirm
			}
			err := executeEndpoint(context.Background(), campaign, flags)
			if err == nil {
				t.Fatalf("campaign create unexpectedly succeeded, want %q", test.want)
			}
			if !errors.Is(err, flag.ErrHelp) && strings.HasPrefix(test.want, "--confirm") {
				t.Fatalf("campaign create error = %v, want usage error", err)
			}
			if got := err.Error(); got != test.want {
				t.Fatalf("campaign create error = %q, want exact %q", got, test.want)
			}
		})
	}

	budget, ok := appleads.PlatformEndpointByCommandPath("budget-orders", "create")
	if !ok {
		t.Fatal("missing platform shared-budget create")
	}
	if !budget.RiskConfirm || budget.RiskConfirmBodyField != "" {
		t.Fatalf("budget risk metadata = %+v, want unconditional confirmation", budget)
	}
	file := filepath.Join(t.TempDir(), "budget.json")
	if err := os.WriteFile(file, []byte(`{"name":"agent-test"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, flags := bindEndpointFlags(budget, "budget-orders create")
	*flags.file = file
	err := executeEndpoint(context.Background(), budget, flags)
	if !errors.Is(err, flag.ErrHelp) || err.Error() != "--confirm is required to acknowledge potential Apple Ads spend, billing, delivery, targeting, or access impact" {
		t.Fatalf("budget create error = %v, want exact pre-auth confirmation usage error", err)
	}
}

func TestPlatformCampaignUpdateAndBudgetUpdateConfirmBeforeAuth(t *testing.T) {
	setAdsResolverTestEnv(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing-config.json"))

	campaign, ok := appleads.PlatformEndpointByCommandPath("campaigns", "update")
	if !ok {
		t.Fatal("missing platform campaigns update")
	}
	if !campaign.RiskConfirm {
		t.Fatal("campaigns update must carry spend-risk confirmation metadata")
	}
	cmd := findCommand(AdsCommand(), "campaigns", "update")
	if cmd == nil {
		t.Fatal("missing direct v1 campaigns update command")
	}
	for _, want := range []string{
		"only name and status=PAUSED",
		"dailyBudget",
		"budgetOrder",
		"targeting",
		"bid",
		"start",
		"resume",
		"enabled",
		"--confirm",
	} {
		if !strings.Contains(cmd.LongHelp, want) {
			t.Fatalf("campaigns update LongHelp = %q, missing %q", cmd.LongHelp, want)
		}
	}

	tests := []struct {
		name    string
		payload string
		confirm bool
		want    string
	}{
		{name: "paused name-only update is safe", payload: `{"name":"paused-name","status":"PAUSED"}`, want: "ads: configuration not found; run 'asc ads auth login' to store Apple Ads credentials, set ASC_ADS_* environment credentials, or pass --ads-profile"},
		{name: "missing status", payload: `{"name":"name-only"}`, want: `--confirm is required unless status is "PAUSED" and only non-spend fields are changed`},
		{name: "enabled status", payload: `{"status":"ENABLED"}`, want: `--confirm is required unless status is "PAUSED" and only non-spend fields are changed`},
		{name: "daily budget", payload: `{"status":"PAUSED","dailyBudget":1}`, want: `--confirm is required unless status is "PAUSED" and only non-spend fields are changed`},
		{name: "budget order", payload: `{"status":"PAUSED","budgetOrder":"b1"}`, want: `--confirm is required unless status is "PAUSED" and only non-spend fields are changed`},
		{name: "targeting", payload: `{"status":"PAUSED","targeting":{}}`, want: `--confirm is required unless status is "PAUSED" and only non-spend fields are changed`},
		{name: "bid", payload: `{"status":"PAUSED","bid":1}`, want: `--confirm is required unless status is "PAUSED" and only non-spend fields are changed`},
		{name: "start", payload: `{"status":"PAUSED","start":"2026-08-15"}`, want: `--confirm is required unless status is "PAUSED" and only non-spend fields are changed`},
		{name: "resume", payload: `{"status":"PAUSED","resume":true}`, want: `--confirm is required unless status is "PAUSED" and only non-spend fields are changed`},
		{name: "enabled field", payload: `{"status":"PAUSED","enabled":true}`, want: `--confirm is required unless status is "PAUSED" and only non-spend fields are changed`},
		{name: "confirmed spend update", payload: `{"status":"ENABLED","dailyBudget":1}`, confirm: true, want: "ads: configuration not found; run 'asc ads auth login' to store Apple Ads credentials, set ASC_ADS_* environment credentials, or pass --ads-profile"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "campaign-update.json")
			if err := os.WriteFile(file, []byte(test.payload), 0o600); err != nil {
				t.Fatal(err)
			}
			_, flags := bindEndpointFlags(campaign, "campaigns update")
			if err := flags.flagSet.Set("campaign", "c1"); err != nil {
				t.Fatal(err)
			}
			*flags.file = file
			if flags.confirm != nil {
				*flags.confirm = test.confirm
			}
			err := executeEndpoint(context.Background(), campaign, flags)
			if err == nil {
				t.Fatalf("campaign update unexpectedly succeeded, want %q", test.want)
			}
			if got := err.Error(); got != test.want {
				t.Fatalf("campaign update error = %q, want exact %q", got, test.want)
			}
			if strings.HasPrefix(test.want, "--confirm") && !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("campaign update error = %v, want usage classification", err)
			}
		})
	}

	budget, ok := appleads.PlatformEndpointByCommandPath("budget-orders", "update")
	if !ok {
		t.Fatal("missing platform budget-orders update")
	}
	if !budget.RiskConfirm || budget.RiskConfirmBodyField != "" {
		t.Fatalf("budget update risk metadata = %+v, want unconditional confirmation", budget)
	}
	_, budgetFlags := bindEndpointFlags(budget, "budget-orders update")
	if err := budgetFlags.flagSet.Set("budget-order", "b1"); err != nil {
		t.Fatal(err)
	}
	if err := executeEndpoint(context.Background(), budget, budgetFlags); !errors.Is(err, flag.ErrHelp) || err.Error() != "--confirm is required to acknowledge potential Apple Ads spend, billing, delivery, targeting, or access impact" {
		t.Fatalf("budget update error = %v, want exact pre-auth confirmation usage error", err)
	}
}

func TestPlatformSpendRiskMutationsRequireConfirmationBeforeAuth(t *testing.T) {
	setAdsResolverTestEnv(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing-config.json"))
	for _, command := range []string{
		"ad-accounts create",
		"ad-groups create",
		"ad-groups update",
		"ads create",
		"ads update",
		"budget-orders create",
		"budget-orders update",
		"targeting-keywords create",
		"targeting-keywords update",
		"targeting-keywords create-bulk",
		"targeting-keywords update-bulk",
		"negative-keywords create",
		"negative-keywords update",
		"negative-keywords create-bulk",
		"negative-keywords update-bulk",
	} {
		t.Run(command, func(t *testing.T) {
			spec, ok := appleads.PlatformEndpointByCommandPath(strings.Fields(command)...)
			if !ok {
				t.Fatalf("missing %q", command)
			}
			if !spec.RiskConfirm || spec.RiskConfirmBodyField != "" {
				t.Fatalf("%q risk metadata = %+v, want unconditional confirmation", command, spec)
			}
			_, flags := bindEndpointFlags(spec, command)
			for _, param := range spec.PathParams {
				if param.Required && !param.ContextValue {
					if err := flags.flagSet.Set(param.Flag, "resource-id"); err != nil {
						t.Fatal(err)
					}
				}
			}
			err := executeEndpoint(context.Background(), spec, flags)
			if !errors.Is(err, flag.ErrHelp) || err.Error() != "--confirm is required to acknowledge potential Apple Ads spend, billing, delivery, targeting, or access impact" {
				t.Fatalf("%q error = %v, want exact pre-auth confirmation usage error", command, err)
			}
		})
	}
}

func TestPlatformRecommendationRiskConfirmationPrecedesAuth(t *testing.T) {
	setAdsResolverTestEnv(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing-config.json"))

	for _, path := range [][]string{
		{"recommendations", "daily-budgets", "apply"},
		{"recommendations", "daily-budgets", "dismiss"},
		{"recommendations", "target-cpas", "apply"},
		{"recommendations", "target-cpas", "dismiss"},
	} {
		t.Run(strings.Join(path, "-"), func(t *testing.T) {
			spec, ok := appleads.PlatformEndpointByCommandPath(path...)
			if !ok {
				t.Fatalf("missing platform endpoint %q", strings.Join(path, " "))
			}
			if spec.RequiresConfirm || !spec.RiskConfirm {
				t.Fatalf("%q confirmation metadata = requires=%t risk=%t", strings.Join(path, " "), spec.RequiresConfirm, spec.RiskConfirm)
			}
			_, flags := bindEndpointFlags(spec, strings.Join(path, " "))
			err := executeEndpoint(context.Background(), spec, flags)
			if !errors.Is(err, flag.ErrHelp) || !strings.Contains(err.Error(), "spend, billing") {
				t.Fatalf("%q error = %v, want pre-auth spend-risk confirmation", strings.Join(path, " "), err)
			}
		})
	}
}

func TestConfirmationHelpDistinguishesOperationalRiskFromDeletion(t *testing.T) {
	root := AdsCommand()
	for _, test := range []struct {
		path []string
		want string
	}{
		{path: []string{"campaigns", "create"}, want: "spend, billing"},
		{path: []string{"budget-orders", "create"}, want: "spend, billing"},
		{path: []string{"v5", "targeting-keywords", "create-bulk"}, want: "spend, billing"},
		{path: []string{"campaigns", "delete"}, want: "Confirm deletion"},
		{path: []string{"recommendations", "daily-budgets", "apply"}, want: "spend, billing"},
		{path: []string{"recommendations", "daily-budgets", "dismiss"}, want: "spend, billing"},
		{path: []string{"recommendations", "target-cpas", "apply"}, want: "spend, billing"},
		{path: []string{"recommendations", "target-cpas", "dismiss"}, want: "spend, billing"},
		{path: []string{"api", "request"}, want: "spend, billing"},
	} {
		cmd := findCommand(root, test.path...)
		if cmd == nil {
			t.Fatalf("missing command asc ads %s", strings.Join(test.path, " "))
		}
		confirm := cmd.FlagSet.Lookup("confirm")
		if confirm == nil || !strings.Contains(confirm.Usage, test.want) {
			t.Fatalf("asc ads %s --confirm usage = %q, want %q", strings.Join(test.path, " "), valueFlagUsage(confirm), test.want)
		}
	}
}

func valueFlagUsage(f *flag.Flag) string {
	if f == nil {
		return ""
	}
	return f.Usage
}

func TestPlatformAppsSearchHelpExplainsModesAndDefaults(t *testing.T) {
	root := AdsCommand()
	search := findCommand(root, "apps", "search")
	if search == nil {
		t.Fatal("missing asc ads apps search")
	}
	for _, want := range []string{
		"At least one of --query, --cpids, or --return-owned-apps is required",
		`--query "Example"`,
		`--cpids "123456,789012"`,
		"--return-owned-apps",
	} {
		if !strings.Contains(search.LongHelp, want) {
			t.Fatalf("apps search LongHelp = %q, want %q", search.LongHelp, want)
		}
	}

	for _, test := range []struct {
		name string
		want string
	}{
		{name: "query", want: "Free-text app name or developer-name search"},
		{name: "cpids", want: "Comma-separated iTunes content provider IDs"},
		{name: "store-fronts", want: "ISO 3166-1 alpha-2 storefront codes"},
		{name: "return-owned-apps", want: "Return apps owned by this organization"},
		{name: "limit", want: "Maximum results to return"},
		{name: "offset", want: "Zero-based result offset for pagination"},
	} {
		flag := search.FlagSet.Lookup(test.name)
		if flag == nil {
			t.Fatalf("apps search missing --%s", test.name)
		}
		if !strings.Contains(flag.Usage, test.want) {
			t.Fatalf("--%s usage = %q, want %q", test.name, flag.Usage, test.want)
		}
	}
	if got := search.FlagSet.Lookup("limit").DefValue; got != "20" {
		t.Fatalf("apps search --limit default = %q, want 20", got)
	}
}

func bodyShape(kind appleads.BodyKind) string {
	switch kind {
	case appleads.BodyObject:
		return "JSON object"
	case appleads.BodyArray:
		return "JSON array"
	case appleads.BodyMultipart:
		return "multipart/form-data"
	default:
		return string(kind)
	}
}

func bodyRequired(spec appleads.EndpointSpec) string {
	if spec.BodyOptional && !spec.CLIRequiresBody {
		return "no"
	}
	return "yes"
}

func TestPlatformAdAccountUpdateBodySafeguards(t *testing.T) {
	spec, ok := appleads.PlatformEndpointByCommandPath("ad-accounts", "update")
	if !ok {
		t.Fatal("missing platform ad-account update")
	}

	if err := validateEndpointBody(spec, json.RawMessage(`{"productFeatures":["APPSTORE_APP_MANUAL"]}`), true); err == nil || !strings.Contains(err.Error(), "productFeatures") {
		t.Fatalf("productFeatures error = %v", err)
	}
	delegations := json.RawMessage(`{"delegations":[{"resourceId":"123","resourceType":"CONTENT_PROVIDER"}]}`)
	if err := validateEndpointBody(spec, delegations, false); err == nil || !strings.Contains(err.Error(), "--confirm") {
		t.Fatalf("delegation confirm error = %v", err)
	}
	if err := validateEndpointBody(spec, delegations, true); err != nil {
		t.Fatalf("confirmed delegations error: %v", err)
	}
	if err := validateEndpointBody(spec, json.RawMessage(`{"delegations":[{"resourceId":"123"}]}`), true); err == nil || !strings.Contains(err.Error(), "resourceType") {
		t.Fatalf("delegation shape error = %v", err)
	}
	if spec.RequiresConfirm {
		t.Fatal("ad-account update must not require confirmation for every payload")
	}
	if err := validateEndpointBody(spec, json.RawMessage(`{"name":"Renamed"}`), false); err != nil {
		t.Fatalf("name-only update without confirmation error: %v", err)
	}
}

func TestPlatformAdAccountCreateRequiresOneProductFeature(t *testing.T) {
	spec, _ := appleads.PlatformEndpointByCommandPath("ad-accounts", "create")
	for _, test := range []struct {
		name    string
		body    string
		wantErr bool
	}{
		{name: "empty", body: `{"name":"Account","productFeatures":[]}`, wantErr: true},
		{name: "both", body: `{"name":"Account","productFeatures":["APPSTORE_APP_MANUAL","BUSINESS_BRAND_MANUAL"]}`, wantErr: true},
		{name: "invalid", body: `{"name":"Account","productFeatures":["OTHER"]}`, wantErr: true},
		{name: "app", body: `{"name":"Account","productFeatures":["APPSTORE_APP_MANUAL"]}`},
		{name: "brand", body: `{"name":"Account","productFeatures":["BUSINESS_BRAND_MANUAL"]}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validateEndpointBody(spec, json.RawMessage(test.body), true)
			if (err != nil) != test.wantErr {
				t.Fatalf("error = %v, wantErr %t", err, test.wantErr)
			}
		})
	}
}

func TestRawPlatformRequestUsesEndpointRiskMetadata(t *testing.T) {
	if !rawPlatformRequestRequiresConfirm("POST", "v1/ad-accounts", nil) {
		t.Fatal("ad-account create must require confirmation before payload/auth work")
	}
	if rawPlatformRequestRequiresConfirm("PUT", "v1/ad-accounts/123", json.RawMessage(`{"name":"Renamed"}`)) {
		t.Fatal("name-only ad-account update must not require confirmation")
	}
	if !rawPlatformRequestRequiresConfirm("PUT", "v1/ad-accounts/123", json.RawMessage(`{"delegations":[]}`)) {
		t.Fatal("delegation replacement must require confirmation")
	}
	if !rawPlatformRequestRequiresConfirm("POST", "v1/unknown-mutation", nil) {
		t.Fatal("unknown POST must fail closed behind confirmation")
	}
	if !rawPlatformRequestRequiresConfirm("POST", "v1/campaigns", nil) {
		t.Fatal("campaign create without a body must require confirmation")
	}
	if rawPlatformRequestRequiresConfirm("POST", "v1/campaigns", json.RawMessage(`{"status":"PAUSED"}`)) {
		t.Fatal("paused campaign create must use the safe body exception")
	}
	if rawPlatformRequestRequiresConfirm("POST", "v1/campaigns/query", nil) {
		t.Fatal("known read-only campaign query must not require confirmation")
	}
	if rawPlatformRequestRequiresConfirm("PUT", "v1/campaigns/campaign-1", json.RawMessage(`{"name":"Paused","status":"PAUSED"}`)) {
		t.Fatal("paused name-only campaign update must use the safe body exception")
	}
	if !rawPlatformRequestRequiresConfirm("PUT", "v1/campaigns/campaign-1", json.RawMessage(`{"status":"PAUSED","dailyBudget":1}`)) {
		t.Fatal("campaign budget update must require confirmation")
	}
	if got, want := rawPlatformRequestConfirmMessage("PUT", "v1/campaigns/campaign-1", json.RawMessage(`{"status":"PAUSED","dailyBudget":1}`)), `--confirm is required unless status is "PAUSED" and only non-spend fields are changed`; got != want {
		t.Fatalf("campaign update confirmation message = %q, want %q", got, want)
	}
	if rawPlatformRequestRequiresConfirm("POST", "v1/metadata/apps/supported-languages/query", nil) {
		t.Fatal("known read-only POST query must not require confirmation")
	}
	if rawPlatformRequestRequiresPrePayloadConfirmation("PUT", "v1/ad-accounts/123") {
		t.Fatal("body-scoped ad-account update confirmation must wait for the payload")
	}
	if rawPlatformRequestRequiresPrePayloadConfirmation("POST", "v1/metadata/apps/supported-languages/query") {
		t.Fatal("known read-only POST query must remain confirmation-free before the payload")
	}
	if !rawPlatformRequestRequiresPrePayloadConfirmation("POST", "v1/unknown-mutation") {
		t.Fatal("unknown POST must fail closed before payload work")
	}
	if rawPlatformRequestRequiresConfirm("GET", "v1/unknown-read", nil) {
		t.Fatal("unknown GET must remain confirmation-free")
	}
	want := "--confirm is required to acknowledge " + riskConfirmationImpact
	if got := rawPlatformRequestConfirmMessage("POST", "v1/ad-accounts", nil); got != want {
		t.Fatalf("ad-account create confirmation message = %q, want %q", got, want)
	}
}

func TestValidateRawPlatformAdAccountPathIDRejectsDeleteMismatch(t *testing.T) {
	err := validateRawPlatformAdAccountPathID("DELETE", "v1/ad-accounts/PATH_ACCOUNT", "CONTEXT_ACCOUNT")
	if err == nil || !strings.Contains(err.Error(), "must match the v1/ad-accounts path ID") {
		t.Fatalf("validateRawPlatformAdAccountPathID() error = %v, want path/context mismatch", err)
	}
}

func TestRiskConfirmationHonorsExplicitSafeBodyValue(t *testing.T) {
	spec := appleads.EndpointSpec{
		RiskConfirm:          true,
		RiskConfirmBodyField: "status",
		RiskConfirmBodyValue: "PAUSED",
	}
	if riskConfirmationRequired(spec, json.RawMessage(`{"status":"PAUSED"}`)) {
		t.Fatal("explicitly paused payload must not require spend confirmation")
	}
	for _, body := range []string{`{"status":"ENABLED"}`, `{}`, `{"status":123}`, `not-json`} {
		if !riskConfirmationRequired(spec, json.RawMessage(body)) {
			t.Fatalf("payload %s unexpectedly bypassed spend confirmation", body)
		}
	}
}

func TestConfirmHelpExplainsRiskImpact(t *testing.T) {
	want := "Acknowledge potential Apple Ads spend, billing, delivery, targeting, or access impact"
	spec := appleads.EndpointSpec{RiskConfirm: true}
	if got := confirmFlagUsage(spec); got != want {
		t.Fatalf("confirmFlagUsage() = %q, want %q", got, want)
	}
	command := PlatformAPIRequestCommand()
	if got := command.FlagSet.Lookup("confirm").Usage; got != want {
		t.Fatalf("raw Platform --confirm usage = %q, want %q", got, want)
	}
}

func TestPlatformCampaignDeleteCommandsRequireConfirmationFirst(t *testing.T) {
	setAdsResolverTestEnv(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing-config.json"))
	for _, command := range [][]string{
		{"campaigns", "delete"},
		{"ad-groups", "delete"},
		{"ads", "delete"},
		{"targeting-keywords", "delete"},
		{"negative-keywords", "delete"},
		{"budget-orders", "delete"},
	} {
		spec, ok := appleads.PlatformEndpointByCommandPath(command...)
		if !ok {
			t.Fatalf("missing %q", strings.Join(command, " "))
		}
		_, flags := bindEndpointFlags(spec, "test")
		if err := executeEndpoint(context.Background(), spec, flags); !errors.Is(err, flag.ErrHelp) || !strings.Contains(err.Error(), "--confirm is required") {
			t.Errorf("%q unconfirmed error = %v, want confirmation usage error", strings.Join(command, " "), err)
		}
	}
}

func TestPlatformGeoSearchSerializesFixtureQueryNames(t *testing.T) {
	spec, ok := appleads.PlatformEndpointByCommandPath("geo", "search")
	if !ok {
		t.Fatal("missing geo search")
	}
	fs, flags := bindEndpointFlags(spec, "test")
	if err := fs.Parse([]string{
		"--supply-source", "appstore",
		"--query", "San Francisco",
		"--entity", "Locality",
		"--country-code", "us",
		"--eligible",
		"--offset", "20",
		"--page-size", "50",
	}); err != nil {
		t.Fatal(err)
	}
	query, err := collectQuery(spec, flags)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := query.Encode(), "countrycode=US&eligible=true&entity=Locality&offset=20&pageSize=50&query=San+Francisco&supplySource=APPSTORE"; got != want {
		t.Fatalf("geo query = %q, want %q", got, want)
	}
	if err := fs.Set("page-size", "0"); err != nil {
		t.Fatal(err)
	}
	if _, err := collectQuery(spec, flags); err == nil || !strings.Contains(err.Error(), "--page-size must be greater than 0") {
		t.Fatalf("zero page size error = %v, want pre-network validation", err)
	}
	_, flags = bindEndpointFlags(spec, "test")
	*flags.queryStrings["supplySource"] = "MAPS"
	*flags.queryStrings["query"] = "x"
	if _, err := collectQuery(spec, flags); err == nil || !strings.Contains(err.Error(), "at least 2 characters") {
		t.Fatalf("short geo query error = %v", err)
	}
}

func TestPlatformCampaignPathIDsAreStringsButAdamIDIsNumeric(t *testing.T) {
	campaign, _ := appleads.PlatformEndpointByCommandPath("campaigns", "view")
	fs, flags := bindEndpointFlags(campaign, "test")
	if err := fs.Set("campaign", "campaign_external_01"); err != nil {
		t.Fatal(err)
	}
	params, err := collectPathParams(campaign, flags)
	if err != nil || params["id"] != "campaign_external_01" {
		t.Fatalf("campaign path params = %#v, error = %v", params, err)
	}

	locales, _ := appleads.PlatformEndpointByCommandPath("apps", "locales", "find")
	fs, flags = bindEndpointFlags(locales, "test")
	if err := fs.Set("adam-id", "not-a-number"); err != nil {
		t.Fatal(err)
	}
	if _, err := collectPathParams(locales, flags); err == nil || !strings.Contains(err.Error(), "--adam-id must be an integer") {
		t.Fatalf("adam ID error = %v", err)
	}
}

func TestPlatformCampaignPauseResumeWorkflowsUseStringIDsAndSafeConfirmation(t *testing.T) {
	setAdsResolverTestEnv(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing-config.json"))
	root := AdsCommand()
	for _, test := range []struct {
		name         string
		needsConfirm bool
		wantHelp     string
	}{
		{name: "pause", wantHelp: "does not require --confirm"},
		{name: "resume", needsConfirm: true, wantHelp: "requires --confirm"},
	} {
		name := test.name
		command := findCommand(root, "campaigns", name)
		if command == nil || command.Exec == nil {
			t.Fatalf("missing executable direct v1 campaigns %s", name)
		}
		if command.FlagSet.Lookup("ad-account") == nil || command.FlagSet.Lookup("org") != nil {
			t.Fatalf("direct v1 campaigns %s context flags are incorrect", name)
		}
		if err := command.FlagSet.Set("campaign", "campaign_external_01"); err != nil {
			t.Fatal(err)
		}
		err := command.Exec(context.Background(), nil)
		if test.needsConfirm {
			if !errors.Is(err, flag.ErrHelp) || err.Error() != "--confirm is required" {
				t.Fatalf("direct v1 campaigns %s unconfirmed error = %v, want confirmation usage error", name, err)
			}
		} else if errors.Is(err, flag.ErrHelp) && strings.Contains(err.Error(), "--confirm") {
			t.Fatalf("direct v1 campaigns %s unexpectedly requires confirmation: %v", name, err)
		}
		if !strings.Contains(command.LongHelp, `{"status":"`) || !strings.Contains(command.LongHelp, "PUT v1/campaigns/{id}") || !strings.Contains(command.LongHelp, test.wantHelp) {
			t.Fatalf("direct v1 campaigns %s help must document its status payload: %q", name, command.LongHelp)
		}
	}
}

func TestAdsCampaignsHelpReadsAsManagementSurface(t *testing.T) {
	root := AdsCommand()
	campaigns := findCommand(root, "v5", "campaigns")
	if campaigns == nil {
		t.Fatal("missing campaigns command")
	}
	if !strings.HasPrefix(campaigns.ShortHelp, "DEPRECATED:") || !strings.Contains(campaigns.ShortHelp, "asc ads campaigns find") {
		t.Fatalf("campaigns ShortHelp = %q, want deprecated management surface", campaigns.ShortHelp)
	}
	if campaigns.FlagSet.Lookup("campaign") != nil {
		t.Fatal("campaigns list alias should not expose workflow-only --campaign flag")
	}

	resume := findCommand(root, "v5", "campaigns", "resume")
	if resume == nil {
		t.Fatal("missing campaigns resume command")
	}
	campaignFlag := resume.FlagSet.Lookup("campaign")
	if campaignFlag == nil {
		t.Fatal("resume command missing --campaign flag")
	}
	if got := campaignFlag.Usage; got != "Apple Ads campaign ID (required)" {
		t.Fatalf("resume --campaign usage = %q, want operator-friendly wording", got)
	}
	if !strings.Contains(resume.LongHelp, "--campaign CAMPAIGN_ID --confirm --org ORG_ID") {
		t.Fatalf("resume LongHelp = %q, want campaign ID example", resume.LongHelp)
	}
}

func TestAdsCampaignUpdateHelpDocumentsRequiredEnvelope(t *testing.T) {
	root := AdsCommand()
	update := findCommand(root, "v5", "campaigns", "update")
	if update == nil {
		t.Fatal("missing campaigns update command")
	}

	for _, want := range []string{
		`Apple requires a "campaign" envelope for campaign updates.`,
		`{"campaign":{"status":"PAUSED"}}`,
	} {
		if !strings.Contains(update.LongHelp, want) {
			t.Fatalf("campaigns update LongHelp = %q, want %q", update.LongHelp, want)
		}
	}
}

func TestPlatformConditionalConfirmationAppearsOnceInHelp(t *testing.T) {
	root := AdsCommand()
	update := findCommand(root, "ad-accounts", "update")
	if update == nil {
		t.Fatal("missing direct v1 ad-account update command")
	}

	if got := strings.Count(update.LongHelp, "--confirm"); got != 1 {
		t.Fatalf("ad-accounts update LongHelp contains --confirm %d times, want once: %q", got, update.LongHelp)
	}
	if !strings.Contains(update.LongHelp, "[--confirm]") {
		t.Fatalf("ad-accounts update LongHelp = %q, want optional --confirm example", update.LongHelp)
	}
}

func TestCollectQueryValidatesEndpointSpecificLimitsAndEnums(t *testing.T) {
	customReports, _ := appleads.EndpointByCommandPath("impression-share-reports", "list")
	fs, flags := bindEndpointFlags(customReports, "test")
	if err := fs.Parse([]string{"--limit", "0"}); err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if _, err := collectQuery(customReports, flags); err == nil || !strings.Contains(err.Error(), "--limit must be between 1 and 50") {
		t.Fatalf("custom reports explicit zero limit error = %v, want min 1 error", err)
	}

	_, flags = bindEndpointFlags(customReports, "test")
	*flags.queryInts["limit"] = 51
	if _, err := collectQuery(customReports, flags); err == nil || !strings.Contains(err.Error(), "--limit must be between 1 and 50") {
		t.Fatalf("custom reports limit error = %v, want max 50 error", err)
	}

	productPages, _ := appleads.EndpointByCommandPath("product-pages", "list")
	_, flags = bindEndpointFlags(productPages, "test")
	*flags.pathStrings["adamId"] = "123456789"
	*flags.queryStrings["states"] = "VISIBLE,PAUSED"
	if _, err := collectQuery(productPages, flags); err == nil || !strings.Contains(err.Error(), "--states must be one of: HIDDEN, VISIBLE") {
		t.Fatalf("states error = %v, want enum validation", err)
	}
}

func TestCollectPathParamsRequiresDocumentedIdentifiers(t *testing.T) {
	campaign, _ := appleads.EndpointByCommandPath("campaigns", "view")
	_, flags := bindEndpointFlags(campaign, "test")
	if _, err := collectPathParams(campaign, flags); err == nil || !strings.Contains(err.Error(), "--campaign is required") {
		t.Fatalf("path error = %v, want campaign required", err)
	}

	*flags.pathStrings["campaignId"] = "123"
	params, err := collectPathParams(campaign, flags)
	if err != nil {
		t.Fatalf("collectPathParams() error: %v", err)
	}
	if params["campaignId"] != "123" {
		t.Fatalf("campaignId = %q, want 123", params["campaignId"])
	}

	*flags.pathStrings["campaignId"] = "not-a-number"
	if _, err := collectPathParams(campaign, flags); err == nil || !strings.Contains(err.Error(), "--campaign must be an integer") {
		t.Fatalf("path error = %v, want integer validation", err)
	}
}

func TestRawRequestRequiresOrgGuardrails(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		requiresOrg bool
		wantErr     string
	}{
		{name: "me does not need org", path: "v5/me", requiresOrg: false},
		{name: "me with query does not need org", path: "v5/me?fields=id", requiresOrg: false},
		{name: "acls does not need org", path: "https://api.searchads.apple.com/api/v5/acls", requiresOrg: false},
		{name: "absolute me with query does not need org", path: "https://api.searchads.apple.com/api/v5/me?fields=id", requiresOrg: false},
		{name: "campaigns needs org", path: "v5/campaigns", requiresOrg: true},
		{name: "reject non apple host", path: "https://example.com/api/v5/campaigns", wantErr: "Apple Ads v5 URL"},
		{name: "reject path traversal", path: "v5/../campaigns", wantErr: "path traversal"},
		{name: "reject wrong version", path: "v4/campaigns", wantErr: "start with v5/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requiresOrg, err := rawRequestRequiresOrg(tt.path)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want contains %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("rawRequestRequiresOrg() error: %v", err)
			}
			if requiresOrg != tt.requiresOrg {
				t.Fatalf("requiresOrg = %t, want %t", requiresOrg, tt.requiresOrg)
			}
		})
	}
}

func TestRawPlatformRequestRequiresAdAccount(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		path     string
		requires bool
		wantErr  string
	}{
		{name: "me", method: "GET", path: "v1/me", requires: false},
		{name: "acls absolute", method: "GET", path: "https://api.ads.apple.com/v1/acls", requires: false},
		{name: "org", method: "GET", path: "v1/orgs/123", requires: false},
		{name: "advertiser resources", method: "GET", path: "v1/advertiser-resources?resourceType=APP", requires: false},
		{name: "create ad account", method: "POST", path: "v1/ad-accounts", requires: false},
		{name: "list ad accounts defaults scoped", method: "GET", path: "v1/ad-accounts", requires: true},
		{name: "view ad account", method: "GET", path: "v1/ad-accounts/123", requires: true},
		{name: "update ad account", method: "PUT", path: "v1/ad-accounts/123", requires: true},
		{name: "unknown defaults scoped", method: "GET", path: "v1/future-resource", requires: true},
		{name: "reject wrong host", method: "GET", path: "https://example.com/v1/me", wantErr: "Apple Ads Platform API v1 URL"},
		{name: "reject userinfo", method: "GET", path: "https://example.com@api.ads.apple.com/v1/me", wantErr: "Apple Ads Platform API v1 URL"},
		{name: "reject legacy version", method: "GET", path: "v5/me", wantErr: "start with v1/"},
		{name: "reject traversal", method: "GET", path: "v1/../me", wantErr: "path traversal"},
		{name: "reject encoded traversal", method: "GET", path: "v1/%2e%2e/campaigns", wantErr: "path traversal"},
		{name: "reject network path", method: "GET", path: "v1//example.com/campaigns", wantErr: "must not escape"},
		{name: "reject embedded absolute URL", method: "GET", path: "v1/https://example.com/campaigns", wantErr: "must not escape"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requires, err := rawPlatformRequestRequiresAdAccount(tt.method, tt.path)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("rawPlatformRequestRequiresAdAccount() error: %v", err)
			}
			if requires != tt.requires {
				t.Fatalf("requires = %t, want %t", requires, tt.requires)
			}
		})
	}
}

func TestRawPlatformRequestRejectsMultipartEndpoints(t *testing.T) {
	if got := rawPlatformRequestMultipartMessage("POST", "v1/assets/upload"); got == "" {
		t.Fatal("rawPlatformRequestMultipartMessage() = empty, want dedicated multipart guidance")
	} else {
		for _, want := range []string{"multipart", "asc ads assets upload"} {
			if !strings.Contains(got, want) {
				t.Fatalf("rawPlatformRequestMultipartMessage() = %q, want %q", got, want)
			}
		}
	}
	if got := rawPlatformRequestMultipartMessage("POST", "v1/campaigns"); got != "" {
		t.Fatalf("rawPlatformRequestMultipartMessage() for JSON endpoint = %q, want empty", got)
	}
}

func TestResolveAdAccountIDPrecedenceDoesNotUseLegacyOrg(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_ADS_AD_ACCOUNT_ID", "ENV_ACCOUNT")
	if err := config.SaveAt(configPath, &config.Config{Ads: config.AdsConfig{
		OrgID:       "LEGACY_ORG",
		AdAccountID: "CONFIG_ACCOUNT",
	}}); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	explicit := "FLAG_ACCOUNT"
	got, source, err := resolveAdAccountIDWithSource(commonFlags{AdAccount: &explicit}, appleads.Credentials{OrgID: "PROFILE_ORG", AdAccountID: "PROFILE_ACCOUNT"})
	if err != nil || got != "FLAG_ACCOUNT" || source != "--ad-account" {
		t.Fatalf("explicit resolution = %q %q %v", got, source, err)
	}
	got, source, err = resolveAdAccountIDWithSource(commonFlags{}, appleads.Credentials{OrgID: "PROFILE_ORG", AdAccountID: "PROFILE_ACCOUNT"})
	if err != nil || got != "ENV_ACCOUNT" || source != "ASC_ADS_AD_ACCOUNT_ID" {
		t.Fatalf("env resolution = %q %q %v", got, source, err)
	}
	t.Setenv("ASC_ADS_AD_ACCOUNT_ID", "")
	got, source, err = resolveAdAccountIDWithSource(commonFlags{}, appleads.Credentials{Profile: "ads", OrgID: "PROFILE_ORG", AdAccountID: "PROFILE_ACCOUNT"})
	if err != nil || got != "PROFILE_ACCOUNT" || source != "Ads profile ad_account_id" {
		t.Fatalf("profile resolution = %q %q %v", got, source, err)
	}
	got, source, err = resolveAdAccountIDWithSource(commonFlags{}, appleads.Credentials{Profile: "profile-without-account", OrgID: "PROFILE_ORG"})
	if err != nil || got != "" || source != "" {
		t.Fatalf("empty named profile must not inherit root ad account: %q %q %v", got, source, err)
	}
	got, source, err = resolveAdAccountIDWithSource(commonFlags{}, appleads.Credentials{OrgID: "PROFILE_ORG"})
	if err != nil || got != "CONFIG_ACCOUNT" || source != "ads.ad_account_id" {
		t.Fatalf("config resolution = %q %q %v", got, source, err)
	}
	if err := config.SaveAt(configPath, &config.Config{Ads: config.AdsConfig{OrgID: "LEGACY_ORG"}}); err != nil {
		t.Fatalf("SaveAt(legacy) error: %v", err)
	}
	got, source, err = resolveAdAccountIDWithSource(commonFlags{}, appleads.Credentials{OrgID: "PROFILE_ORG"})
	if err != nil || got != "" || source != "" {
		t.Fatalf("legacy org must not resolve as ad account: %q %q %v", got, source, err)
	}
}

func TestNamedAdsProfileWithoutAdAccountDoesNotInheritAnotherProfileDefault(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	setAdsResolverTestEnv(t)
	t.Setenv("ASC_ADS_AD_ACCOUNT_ID", "")
	if err := config.SaveAt(configPath, &config.Config{Ads: config.AdsConfig{
		DefaultKeyName: "profile-a",
		AdAccountID:    "ACCOUNT_A",
		Keys: []config.AdsCredential{
			{Name: "profile-a", ClientID: "A", TeamID: "T", KeyID: "K", PrivateKeyPath: "a.pem", AdAccountID: "ACCOUNT_A"},
			{Name: "profile-b", ClientID: "B", TeamID: "T", KeyID: "K", PrivateKeyPath: "b.pem"},
		},
	}}); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	profile := "profile-b"
	credentials, err := resolveCredentials(commonFlags{AdsProfile: &profile})
	if err != nil {
		t.Fatalf("resolveCredentials() error: %v", err)
	}
	got, source, err := resolveAdAccountIDWithSource(commonFlags{}, credentials)
	if err != nil || got != "" || source != "" {
		t.Fatalf("profile-b ad account = %q source=%q error=%v, want no inherited account", got, source, err)
	}
}

func TestAdsAuthDiscoveryPreservesInt64Identifiers(t *testing.T) {
	const largeID = "9007199254740993"
	if got := discoveryUserSummary(json.RawMessage(`{"userId":` + largeID + `}`)); got != largeID {
		t.Fatalf("discoveryUserSummary() = %q, want %q", got, largeID)
	}

	accounts, err := summarizePlatformACLAccounts(
		appleads.RawResponse(`{"result":{"acls":[{"adAccount":{"id":`+largeID+`,"orgId":`+largeID+`,"name":"Large"},"roles":["Admin"]}]}}`),
		largeID,
	)
	if err != nil {
		t.Fatalf("summarizePlatformACLAccounts() error: %v", err)
	}
	if len(accounts) != 1 || accounts[0].AdAccountID != largeID || accounts[0].OrgID != largeID || accounts[0].Name != "Large" || !accounts[0].Active {
		t.Fatalf("accounts = %+v, want exact active int64 identifiers", accounts)
	}
	if got := strings.Join(accounts[0].Roles, ","); got != "Admin" {
		t.Fatalf("roles = %q, want Admin", got)
	}
}

func TestPlatformOptionalBodyAndUnboundedLimit(t *testing.T) {
	spec := appleads.EndpointSpec{
		Name:         "platform-query",
		Method:       "POST",
		Path:         "v1/resources/query",
		Version:      appleads.APIVersionPlatformV1,
		Context:      appleads.ContextAdAccount,
		BodyKind:     appleads.BodyObject,
		BodyOptional: true,
		QueryParams: []appleads.ParamSpec{{
			Name: "limit",
			Flag: "limit",
			Type: appleads.ParamInt,
		}},
	}
	fs, flags := bindEndpointFlags(spec, "platform query")
	body, err := readBody(spec, flags)
	if err != nil || body != nil {
		t.Fatalf("optional body = %s error = %v, want nil", body, err)
	}
	if err := fs.Set("limit", "50000"); err != nil {
		t.Fatalf("Set(limit) error: %v", err)
	}
	query, err := collectQuery(spec, flags)
	if err != nil || query.Get("limit") != "50000" {
		t.Fatalf("query = %v error = %v", query, err)
	}
	if got := appleads.MaxPageLimit(spec); got != 0 {
		t.Fatalf("MaxPageLimit() = %d, want no v5 cap", got)
	}
}

func TestCollectQueryHonorsGenericIntegerMax(t *testing.T) {
	spec := appleads.EndpointSpec{
		Name:    "platform-page-size",
		Method:  "GET",
		Path:    "v1/resources",
		Version: appleads.APIVersionPlatformV1,
		QueryParams: []appleads.ParamSpec{{
			Name: "pageSize",
			Flag: "page-size",
			Type: appleads.ParamInt,
			Max:  50,
		}},
	}
	_, flags := bindEndpointFlags(spec, "platform page-size")
	*flags.queryInts["pageSize"] = 50
	query, err := collectQuery(spec, flags)
	if err != nil {
		t.Fatalf("collectQuery(max) error: %v", err)
	}
	if got := query.Get("pageSize"); got != "50" {
		t.Fatalf("pageSize at max = %q, want 50", got)
	}

	*flags.queryInts["pageSize"] = 51
	if _, err := collectQuery(spec, flags); err == nil || !strings.Contains(err.Error(), "--page-size must be at most 50") {
		t.Fatalf("collectQuery(max+1) error = %v, want max validation", err)
	}

	spec.QueryParams[0].Max = 0
	*flags.queryInts["pageSize"] = 50000
	query, err = collectQuery(spec, flags)
	if err != nil {
		t.Fatalf("collectQuery(unbounded) error: %v", err)
	}
	if got := query.Get("pageSize"); got != "50000" {
		t.Fatalf("unbounded pageSize = %q, want 50000", got)
	}
}

func TestResolveCredentialsPrefersExplicitProfileAndStrictRejectsMixedSources(t *testing.T) {
	asc.ResetConfigCacheForTest()
	t.Cleanup(asc.ResetConfigCacheForTest)

	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_ADS_BYPASS_KEYCHAIN", "1")
	if err := appleads.StoreCredentialsConfigAt("profile-a", appleads.Credentials{
		ClientID:       "CLIENT",
		TeamID:         "TEAM",
		KeyID:          "KEY",
		PrivateKeyPath: "private-key.pem",
		OrgID:          "ORG",
	}, configPath); err != nil {
		t.Fatalf("StoreCredentialsConfigAt() error: %v", err)
	}

	profileName := "profile-a"
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	credentials, err := resolveCredentials(commonFlags{AdsProfile: &profileName})
	if err != nil {
		t.Fatalf("resolveCredentials() error: %v", err)
	}
	if credentials.Profile != "profile-a" || credentials.AccessToken != "" || credentials.ClientID != "CLIENT" {
		t.Fatalf("credentials = %+v, want stored profile over access token", credentials)
	}

	t.Setenv("ASC_ADS_STRICT_AUTH", "1")
	_, err = resolveCredentials(commonFlags{AdsProfile: &profileName})
	if err == nil || !strings.Contains(err.Error(), "mixed Apple Ads authentication sources") {
		t.Fatalf("strict mixed source error = %v", err)
	}
}

func TestResolveClientRequiresOrgForOrgScopedEndpoints(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	_, err := resolveClient(context.Background(), commonFlags{}, true)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("resolveClient() error = %v, want usage error", err)
	}

	org := "123456"
	client, err := resolveClient(context.Background(), commonFlags{Org: &org}, true)
	if err != nil {
		t.Fatalf("resolveClient() with org error: %v", err)
	}
	if client == nil {
		t.Fatal("expected client")
	}
}

func TestResolveClientUsesStoredAdsOrgWithAccessToken(t *testing.T) {
	asc.ResetConfigCacheForTest()
	t.Cleanup(asc.ResetConfigCacheForTest)

	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	if err := config.SaveAt(configPath, &config.Config{
		Ads: config.AdsConfig{OrgID: "CONFIG_ORG"},
	}); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	client, err := resolveClient(context.Background(), commonFlags{}, true)
	if err != nil {
		t.Fatalf("resolveClient() error: %v", err)
	}
	if client == nil {
		t.Fatal("expected client")
	}
}

func TestEnvCredentialsRejectsInvalidPrivateKeyBase64(t *testing.T) {
	t.Setenv("ASC_ADS_CLIENT_ID", "CLIENT")
	t.Setenv("ASC_ADS_TEAM_ID", "TEAM")
	t.Setenv("ASC_ADS_KEY_ID", "KEY")
	t.Setenv("ASC_ADS_PRIVATE_KEY_B64", "not-base64")

	_, _, err := envCredentials()
	if err == nil || !strings.Contains(err.Error(), "ASC_ADS_PRIVATE_KEY_B64 is not valid base64") {
		t.Fatalf("envCredentials() error = %v, want invalid base64 error", err)
	}
}

func TestResolveCredentialsStrictRejectsAccessTokenAndKeyEnv(t *testing.T) {
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_ADS_STRICT_AUTH", "1")
	t.Setenv("ASC_ADS_CLIENT_ID", "CLIENT")
	t.Setenv("ASC_ADS_TEAM_ID", "TEAM")
	t.Setenv("ASC_ADS_KEY_ID", "KEY")
	t.Setenv("ASC_ADS_PRIVATE_KEY_PATH", "private-key.pem")

	_, err := resolveCredentials(commonFlags{})
	if err == nil || !strings.Contains(err.Error(), "mixed Apple Ads authentication sources") {
		t.Fatalf("resolveCredentials() error = %v, want mixed source error", err)
	}
}

func TestResolveCredentialsRejectsPartialEnvBeforeStoredFallback(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_ADS_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_ADS_CLIENT_ID", "CLIENT")
	if err := appleads.StoreCredentialsConfigAt("profile-a", appleads.Credentials{
		ClientID:       "STORED_CLIENT",
		TeamID:         "STORED_TEAM",
		KeyID:          "STORED_KEY",
		PrivateKeyPath: "stored-private-key.pem",
		OrgID:          "ORG",
	}, configPath); err != nil {
		t.Fatalf("StoreCredentialsConfigAt() error: %v", err)
	}

	_, err := resolveCredentials(commonFlags{})
	if err == nil || !strings.Contains(err.Error(), "incomplete Apple Ads environment credentials") {
		t.Fatalf("resolveCredentials() error = %v, want incomplete env error", err)
	}
}

func TestCollectQueryIncludesAllowedValidValues(t *testing.T) {
	productPages, _ := appleads.EndpointByCommandPath("product-pages", "list")
	_, flags := bindEndpointFlags(productPages, "test")
	*flags.queryStrings["states"] = "HIDDEN,VISIBLE"
	query, err := collectQuery(productPages, flags)
	if err != nil {
		t.Fatalf("collectQuery() error: %v", err)
	}
	want := url.Values{"states": {"HIDDEN,VISIBLE"}}
	if query.Encode() != want.Encode() {
		t.Fatalf("query = %s, want %s", query.Encode(), want.Encode())
	}
}

func TestEndpointHelpUsesOperatorFriendlyAuthDiscoveryNames(t *testing.T) {
	root := AdsCommand()
	tests := []struct {
		path []string
		want string
	}{
		{path: []string{"me"}, want: "View the current Apple Ads user."},
		{path: []string{"me", "view"}, want: "View the current Apple Ads user."},
		{path: []string{"acls"}, want: "List Apple Ads account ACLs."},
		{path: []string{"acls", "list"}, want: "List Apple Ads account ACLs."},
	}
	for _, test := range tests {
		cmd := findCommand(root, test.path...)
		if cmd == nil {
			t.Fatalf("missing command asc ads %s", strings.Join(test.path, " "))
		}
		if cmd.ShortHelp != test.want || !strings.Contains(cmd.LongHelp, test.want) {
			t.Fatalf("asc ads %s help mismatch: ShortHelp = %q; LongHelp = %q, want content %q", strings.Join(test.path, " "), cmd.ShortHelp, cmd.LongHelp, test.want)
		}

		legacyPath := append([]string{"v5"}, test.path...)
		legacy := findCommand(root, legacyPath...)
		if legacy == nil {
			t.Fatalf("missing command asc ads %s", strings.Join(legacyPath, " "))
		}
		if !strings.HasPrefix(legacy.ShortHelp, "DEPRECATED:") || !strings.Contains(legacy.LongHelp, test.want) {
			t.Fatalf("asc ads %s help mismatch: ShortHelp = %q, want DEPRECATED prefix; LongHelp = %q, want content %q", strings.Join(legacyPath, " "), legacy.ShortHelp, legacy.LongHelp, test.want)
		}
	}
}

func TestPlatformOptimizationHelpUsesOperatorFriendlyVerbs(t *testing.T) {
	root := AdsCommand()
	tests := []struct {
		path []string
		want string
	}{
		{path: []string{"recommendations", "target-cpas", "apply"}, want: "Apply target cpa recommendations."},
		{path: []string{"recommendations", "daily-budgets", "dismiss"}, want: "Dismiss daily budget recommendations."},
		{path: []string{"suggestions", "keywords", "find"}, want: "Find keyword suggestions."},
	}
	for _, test := range tests {
		cmd := findCommand(root, test.path...)
		if cmd == nil {
			t.Fatalf("missing command asc ads %s", strings.Join(test.path, " "))
		}
		if cmd.ShortHelp != test.want {
			t.Fatalf("asc ads %s ShortHelp = %q, want %q", strings.Join(test.path, " "), cmd.ShortHelp, test.want)
		}
	}
}

func findCommand(root *ffcli.Command, path ...string) *ffcli.Command {
	current := root
	for _, part := range path {
		var next *ffcli.Command
		for _, sub := range current.Subcommands {
			if sub.Name == part {
				next = sub
				break
			}
		}
		if next == nil {
			return nil
		}
		current = next
	}
	return current
}

func assertSpecFlags(t *testing.T, cmd *ffcli.Command, spec appleads.EndpointSpec) {
	t.Helper()
	for _, name := range []string{"ads-profile", "output"} {
		if cmd.FlagSet.Lookup(name) == nil {
			t.Fatalf("asc ads %s missing --%s", strings.Join(spec.CommandPath, " "), name)
		}
	}
	if spec.RequiresOrg && cmd.FlagSet.Lookup("org") == nil {
		t.Fatalf("asc ads %s missing --org", strings.Join(spec.CommandPath, " "))
	}
	for _, param := range spec.PathParams {
		if cmd.FlagSet.Lookup(param.Flag) == nil {
			t.Fatalf("asc ads %s missing --%s", strings.Join(spec.CommandPath, " "), param.Flag)
		}
	}
	for _, param := range spec.QueryParams {
		if cmd.FlagSet.Lookup(param.Flag) == nil {
			t.Fatalf("asc ads %s missing --%s", strings.Join(spec.CommandPath, " "), param.Flag)
		}
	}
	if spec.BodyKind != appleads.BodyNone && cmd.FlagSet.Lookup("file") == nil {
		t.Fatalf("asc ads %s missing --file", strings.Join(spec.CommandPath, " "))
	}
	if spec.RequiresConfirm && cmd.FlagSet.Lookup("confirm") == nil {
		t.Fatalf("asc ads %s missing --confirm", strings.Join(spec.CommandPath, " "))
	}
	if spec.RiskConfirm && cmd.FlagSet.Lookup("confirm") == nil {
		t.Fatalf("asc ads %s missing spend-risk --confirm", strings.Join(spec.CommandPath, " "))
	}
	if spec.SupportsPaginate && cmd.FlagSet.Lookup("paginate") == nil {
		t.Fatalf("asc ads %s missing --paginate", strings.Join(spec.CommandPath, " "))
	}
}

func TestReadBodyReadsStdinPayload(t *testing.T) {
	spec, ok := appleads.PlatformEndpointByCommandPath("insights", "search-term-popularity", "find")
	if !ok {
		t.Fatal("missing insights search-term-popularity find")
	}

	payload := `{"timeRange":{"start":"2026-07-05","end":"2026-08-08","granularity":"WEEKLY_SUN_SAT"}}`
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	previous := os.Stdin
	os.Stdin = reader
	t.Cleanup(func() {
		os.Stdin = previous
		reader.Close()
	})
	go func() {
		defer writer.Close()
		_, _ = writer.WriteString(payload)
	}()

	_, flags := bindEndpointFlags(spec, "insights search-term-popularity find")
	*flags.file = "-"
	body, err := readBody(spec, flags)
	if err != nil {
		t.Fatalf("readBody(stdin) error: %v", err)
	}
	if string(body) != payload {
		t.Fatalf("readBody(stdin) = %q, want %q", string(body), payload)
	}
}
