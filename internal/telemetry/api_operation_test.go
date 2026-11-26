package telemetry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/schema"
)

func TestClassifyAPIPathTemplate(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/v1/apps", "/v1/apps"},
		{"/v1/apps/1234567890", "/v1/apps/{id}"},
		{"/v1/apps/1234567890/builds", "/v1/apps/{id}/builds"},
		{"/v1/apps/1234567890/relationships/builds", "/v1/apps/{id}/relationships/builds"},
		{"/v1/territoryAvailabilities/USA", "/v1/territoryAvailabilities/{id}"},
		// The schema has no /v1/territories/{id}: a path no template matches is
		// "other", never the path itself.
		{"/v1/territories/USA", "other"},
		{"/v2/appAvailabilities/APP-ID/territoryAvailabilities", "/v2/appAvailabilities/{id}/territoryAvailabilities"},
		{"/v3/appPricePoints/eyJzIjoiMSJ9", "/v3/appPricePoints/{id}"},
		{"/v1/apps/1234567890/builds?limit=5&filter[version]=1.2.3", "/v1/apps/{id}/builds"},
		{"https://api.appstoreconnect.apple.com/v1/apps/1234567890?include=builds", "/v1/apps/{id}"},
		{"/v1/apps/", "/v1/apps"},
		{"/v1/appz", "other"},
		{"/v1/apps/1234567890/buildsz", "other"},
		{"/v9/apps", "other"},
		{"", "other"},
	}
	for _, test := range tests {
		if got := classifyAPIPathTemplate(test.path); got != test.want {
			t.Errorf("classifyAPIPathTemplate(%q) = %q, want %q", test.path, got, test.want)
		}
	}
}

func TestClassifyAPIQueryParams(t *testing.T) {
	many := make([]string, 0, 80)
	for i := 0; i < 80; i++ {
		many = append(many, fmt.Sprintf("fields[unknownType%02d]", i))
	}
	knownMany := knownQueryParameterNames(t, 66)
	tests := []struct {
		name  string
		names []string
		want  []string
	}{
		{name: "none", names: []string{}, want: []string{}},
		{name: "schema names sorted", names: []string{"limit", "include", "fields[apps]", "filter[bundleId]"}, want: []string{"fields[apps]", "filter[bundleId]", "include", "limit"}},
		{name: "cursor", names: []string{"cursor", "limit"}, want: []string{"cursor", "limit"}},
		{name: "unknown names collapse into one other", names: []string{"SECRET-NAME", "x", "limit", "filter[secret]"}, want: []string{"limit", "other"}},
		{name: "duplicates", names: []string{"limit", "limit"}, want: []string{"limit"}},
		{name: "eighty unknown names", names: many, want: []string{"other"}},
		{name: "more than 64 schema names keep the first 64 after sorting", names: knownMany, want: knownMany[:64]},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := classifyAPIQueryParams(test.names)
			if !ok || !reflect.DeepEqual(got, test.want) {
				t.Fatalf("classifyAPIQueryParams() = %v, %t; want %v", got, ok, test.want)
			}
		})
	}
}

var (
	collectorPathTemplate = regexp.MustCompile(`^/v[1-9]/[a-z][A-Za-z0-9]*(/\{id\}(/[a-z][A-Za-z0-9]*){0,2})?$`)
	collectorQueryName    = regexp.MustCompile(`^[a-z][A-Za-z0-9]*(\[[a-z][A-Za-z0-9]*(\.[a-z][A-Za-z0-9]*)*\])?$`)
)

// TestAPIOperationClassifiersReturnOnlySchemaNames feeds identifiers, values,
// full URLs, encoded paths, and junk to the classifiers and requires every
// output to be a template or query name of the embedded schema, "cursor", or
// "other", and to pass the collector's patterns.
func TestAPIOperationClassifiersReturnOnlySchemaNames(t *testing.T) {
	inputs := []string{
		"/v1/apps/1234567890",
		"/v1/apps/SECRET-APP-ID/builds",
		"/v1/apps/user@example.com",
		"/v1/apps/%2Fetc%2Fpasswd",
		"/v1/apps/..%2F..%2Fsecret",
		"/v1/apps/a b c",
		"https://api.appstoreconnect.apple.com/v1/apps/SECRET?filter[name]=SECRET-VALUE",
		"https://user:pass@example.com/v1/apps/SECRET#fragment",
		"http://[::1]:namedport/v1/apps",
		"//api.appstoreconnect.apple.com/v1/apps/SECRET",
		"/v1/SECRET",
		"SECRET",
		"\x00\x01\u202e",
		"/v1/apps/" + strings.Repeat("A", 4096),
		"limit=SECRET",
		"filter[name]=SECRET",
		"Fields[Apps]",
		"filter[bundleId].SECRET",
		"",
	}
	assertAllowed := func(kind, input, output string, schemaName func(string) bool, pattern *regexp.Regexp) {
		t.Helper()
		if output == apiOperationOther || output == apiQueryCursor {
			return
		}
		if !schemaName(output) || !pattern.MatchString(output) || len(output) > 128 {
			t.Errorf("%s(%q) = %q, which is not a schema name the collector accepts", kind, input, output)
		}
	}
	isTemplate := func(value string) bool {
		template, ok, err := schema.PathTemplate(value)
		return err == nil && ok && template == value
	}
	isQueryName := func(value string) bool {
		known, err := schema.IsQueryParameterName(value)
		return err == nil && known
	}
	for _, input := range inputs {
		assertAllowed("classifyAPIPathTemplate", input, classifyAPIPathTemplate(input), isTemplate, collectorPathTemplate)
		names, ok := classifyAPIQueryParams([]string{input})
		if !ok {
			t.Fatalf("classifyAPIQueryParams(%q) could not read the schema", input)
		}
		for _, name := range names {
			assertAllowed("classifyAPIQueryParams", input, name, isQueryName, collectorQueryName)
		}
	}
}

func TestBuildEventAPIOperationFields(t *testing.T) {
	clearContextEnv(t)
	setTelemetryTestHome(t)

	method := func(value string) *string { return &value }
	names := func(values ...string) *[]string {
		if values == nil {
			values = []string{}
		}
		return &values
	}
	tests := []struct {
		name         string
		command      string
		context      EventContext
		wantMethod   *string
		wantTemplate *string
		wantQuery    *[]string
	}{
		{
			name:         "api request",
			command:      "asc api",
			context:      EventContext{APIMethod: "GET", APIPath: "/v1/apps/1234567890/builds", APIQueryNames: []string{"limit", "filter[version]", "fields[builds]"}},
			wantMethod:   method("GET"),
			wantTemplate: method("/v1/apps/{id}/builds"),
			wantQuery:    names("fields[builds]", "filter[version]", "limit"),
		},
		{
			name:         "api request without query parameters",
			command:      "asc api",
			context:      EventContext{APIMethod: "DELETE", APIPath: "/v1/betaGroups/GROUP-ID", APIQueryNames: []string{}},
			wantMethod:   method("DELETE"),
			wantTemplate: method("/v1/betaGroups/{id}"),
			wantQuery:    names(),
		},
		{
			name:         "api request whose query names were not recorded",
			command:      "asc api",
			context:      EventContext{APIMethod: "PATCH", APIPath: "/v1/apps/1234567890"},
			wantMethod:   method("PATCH"),
			wantTemplate: method("/v1/apps/{id}"),
		},
		{
			name:    "api failure before METHOD and PATH were parsed",
			command: "asc api",
		},
		{
			name:    "api with a method outside the four",
			command: "asc api",
			context: EventContext{APIMethod: "PUT", APIPath: "/v1/apps", APIQueryNames: []string{"limit"}},
		},
		{
			name:    "another command never carries the fields",
			command: "asc builds list",
			context: EventContext{APIMethod: "GET", APIPath: "/v1/apps", APIQueryNames: []string{"limit"}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			event, ok := BuildEventWithContext(test.command, "1.2.3", time.Second, 0, test.context)
			if !ok {
				t.Fatal("BuildEventWithContext() returned no event")
			}
			if event.SchemaVersion != 8 {
				t.Fatalf("SchemaVersion = %d, want 8", event.SchemaVersion)
			}
			if !reflect.DeepEqual(event.APIMethod, test.wantMethod) ||
				!reflect.DeepEqual(event.APIPathTemplate, test.wantTemplate) ||
				!reflect.DeepEqual(event.APIQueryParams, test.wantQuery) {
				t.Fatalf("api fields = %v %v %v, want %v %v %v",
					event.APIMethod, event.APIPathTemplate, event.APIQueryParams,
					test.wantMethod, test.wantTemplate, test.wantQuery)
			}

			encoded, err := json.Marshal(event)
			if err != nil {
				t.Fatal(err)
			}
			var payload map[string]any
			if err := json.Unmarshal(encoded, &payload); err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"api_method", "api_path_template", "api_query_params"} {
				if _, exists := payload[key]; !exists {
					t.Fatalf("%s missing from %s", key, encoded)
				}
			}
			if test.wantQuery != nil && len(*test.wantQuery) == 0 && !strings.Contains(string(encoded), `"api_query_params":[]`) {
				t.Fatalf("empty query names must be [] on the wire: %s", encoded)
			}
		})
	}
}

// An older CLI may have left schema v5 records in the spool. They must be
// read back and sent exactly as they were written, without the v6 fields.
func TestSpoolReadsBackSchemaV5Records(t *testing.T) {
	v5Event := `{"event_id":"event-v5","schema_version":5,"asc_version":"5.4.0","os":"darwin","arch":"arm64",` +
		`"command_path":"asc builds list","command_family":"builds","duration_ms":12,"duration_bucket":"lt_100ms",` +
		`"exit_code":0,"success":true,"runtime_context":"local","invocation_source":"terminal","install_id":null,` +
		`"session_id":"session-1","invocation_shape":"leaf","error_kind":null,"failure_stage":null,` +
		`"failure_parameter":null,"diagnostic_code":null,"outcome_kind":"success","http_status":null,"attempted_child":null}`
	path := filepath.Join(t.TempDir(), "telemetry-spool.jsonl")
	line := `{"event":` + v5Event + `,"endpoint":"https://telemetry.example.test/events"}` + "\n"
	if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}

	records, err := testSpoolStore(path).snapshot(0)
	if err != nil {
		t.Fatalf("snapshot() error = %v", err)
	}
	if len(records) != 1 || records[0].Event.SchemaVersion != 5 {
		t.Fatalf("records = %+v, want the one v5 record", records)
	}
	resent, err := json.Marshal(records[0].Event)
	if err != nil {
		t.Fatal(err)
	}
	var want, got map[string]any
	if err := json.Unmarshal([]byte(v5Event), &want); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(resent, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resent v5 event = %s, want %s", resent, v5Event)
	}
}

func knownQueryParameterNames(t *testing.T, count int) []string {
	t.Helper()
	var names []string
	for _, candidate := range []string{"fields", "filter", "include", "limit", "sort", "exists"} {
		if known, _ := schema.IsQueryParameterName(candidate); known {
			names = append(names, candidate)
		}
	}
	for _, resource := range []string{"apps", "builds", "betaGroups", "betaTesters", "appStoreVersions", "appInfos", "inAppPurchases", "subscriptions", "subscriptionGroups", "territories", "users", "devices", "certificates", "profiles", "bundleIds", "preReleaseVersions", "appEvents", "reviewSubmissions", "appCustomProductPages", "gameCenterDetails", "appPricePoints", "appPrices", "customerReviews", "ciProducts", "ciWorkflows", "ciBuildRuns", "scmRepositories", "appClips", "appPreviewSets", "appScreenshotSets", "betaAppLocalizations", "betaBuildLocalizations", "buildBetaDetails", "appEncryptionDeclarations"} {
		for _, prefix := range []string{"fields", "limit", "filter"} {
			name := prefix + "[" + resource + "]"
			if known, _ := schema.IsQueryParameterName(name); known {
				names = append(names, name)
			}
		}
	}
	if len(names) < count {
		t.Fatalf("found only %d schema query names, want %d", len(names), count)
	}
	names = names[:count]
	sort.Strings(names)
	return names
}
