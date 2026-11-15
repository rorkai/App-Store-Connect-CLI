package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestQueryRegistryMatchesGolden(t *testing.T) {
	spec, bindings := loadTestdata(t)
	got, err := renderQueryRegistry(spec, bindings)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "openapi_queries_gen_test.go.golden"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("registry differs from testdata/openapi_queries_gen_test.go.golden:\n%s", got)
	}
	if none, err := renderQueryRegistry(spec, &Bindings{}); err != nil || none != nil {
		t.Errorf("registry without queries = %q, %v; want nothing", none, err)
	}
}

func TestGenerateRejectsMalformedQueryBindings(t *testing.T) {
	spec, _ := loadTestdata(t)
	param := func(name, field string) *QueryParam { return &QueryParam{Name: name, Field: field} }
	teeth := func(builders ...*QueryBuilder) *QueryBind {
		return &QueryBind{Operation: "zippers_teeth_getToManyRelated", Name: "X", Params: []*QueryParam{param("fields[widgets]", "")}, Builders: builders}
	}
	tests := []struct {
		name  string
		query *QueryBind
	}{
		{"unknown operation", &QueryBind{Operation: "widgets_missing", Name: "X"}},
		{"not a read", &QueryBind{Operation: "widgets_createInstance", Name: "X", Params: []*QueryParam{param("include", "")}}},
		{"no name", &QueryBind{Operation: "widgets_getCollection"}},
		{"parameter the operation does not define", &QueryBind{Operation: "widgets_getCollection", Name: "X", Params: []*QueryParam{param("filter[size]", "sizes")}}},
		{"limit bound as a parameter", &QueryBind{Operation: "widgets_getCollection", Name: "X", Params: []*QueryParam{param("limit", "limit")}}},
		{"parameter that is not a string array", &QueryBind{Operation: "widgets_owner_getToOneRelated", Name: "X", Params: []*QueryParam{param("exists[name]", "named")}}},
		{"array sent as repeated values", &QueryBind{Operation: "widgets_getCollection", Name: "X", Params: []*QueryParam{param("filter[color]", "colors")}}},
		{"array without explode, which defaults to repeated values", &QueryBind{Operation: "widgets_getCollection", Name: "X", Params: []*QueryParam{param("filter[shape]", "shapes")}}},
		{"parameter bound twice", &QueryBind{Operation: "widgets_getCollection", Name: "X", Params: []*QueryParam{param("include", ""), param("include", "other")}}},
		{"filter without a field name", &QueryBind{Operation: "widgets_getCollection", Name: "X", Params: []*QueryParam{param("filter[name]", "")}}},
		{"a query that sends nothing", &QueryBind{Operation: "widgets_owner_getToOneRelated", Name: "X"}},
		{"also names an unknown operation", &QueryBind{Operation: "zippers_getCollection", Also: []string{"widgets_missing"}, Name: "X"}},
		{"also names a write", &QueryBind{Operation: "zippers_getCollection", Also: []string{"widgets_createInstance"}, Name: "X"}},
		{"operation listed twice", &QueryBind{Operation: "zippers_getCollection", Also: []string{"zippers_getCollection"}, Name: "X"}},
		{"parameter none of the reads defines", &QueryBind{Operation: "zippers_getCollection", Also: []string{"yoyos_zippers_getToManyRelated"}, Name: "X", Params: []*QueryParam{param("filter[size]", "sizes")}}},
		{"parameter another read sends as repeated values", &QueryBind{Operation: "zippers_getCollection", Also: []string{"widgets_zippers_getToManyRelated"}, Name: "X", Params: []*QueryParam{param("filter[color]", "colors")}}},
		{"unknown normalize", &QueryBind{Operation: "zippers_getCollection", Name: "X", Params: []*QueryParam{{Name: "filter[kind]", Field: "kinds", Normalize: "lower"}}}},
		{"unknown type", &QueryBind{Operation: "zippers_sprockets_getToManyRelated", Name: "X", Params: []*QueryParam{{Name: "granularity", Field: "granularity", Type: "float"}}}},
		{"string option for an integer", &QueryBind{Operation: "zippers_sprockets_getToManyRelated", Name: "X", Params: []*QueryParam{{Name: "limit[teeth]", Field: "teeth", Type: "string"}}}},
		{"integer option for a boolean", &QueryBind{Operation: "zippers_sprockets_getToManyRelated", Name: "X", Params: []*QueryParam{{Name: "exists[owner]", Field: "owner", Type: "int"}}}},
		{"boolean option for a string", &QueryBind{Operation: "zippers_sprockets_getToManyRelated", Name: "X", Params: []*QueryParam{{Name: "granularity", Field: "granularity", Type: "bool"}}}},
		{"integer option for an array", &QueryBind{Operation: "zippers_sprockets_getToManyRelated", Name: "X", Params: []*QueryParam{{Name: "filter[label]", Field: "label", Type: "int"}}}},
		{"string option for an array sent as repeated values", &QueryBind{Operation: "zippers_sprockets_getToManyRelated", Name: "X", Params: []*QueryParam{{Name: "filter[color]", Field: "color", Type: "string"}}}},
		{"string option that normalizes a list", &QueryBind{Operation: "zippers_sprockets_getToManyRelated", Name: "X", Params: []*QueryParam{{Name: "filter[label]", Field: "label", Type: "string", Normalize: "upper"}}}},
		{"builder without a name", teeth(&QueryBuilder{Operation: "teeth_getInstance"})},
		{"builder named as the query's own", teeth(&QueryBuilder{Operation: "teeth_getInstance", Name: "X"})},
		{"builder named twice", teeth(&QueryBuilder{Operation: "teeth_getInstance", Name: "Y"}, &QueryBuilder{Operation: "widgets_owner_getToOneRelated", Name: "Y"})},
		{"builder of an unknown operation", teeth(&QueryBuilder{Operation: "widgets_missing", Name: "Y"})},
		{"builder of a write", teeth(&QueryBuilder{Operation: "widgets_createInstance", Name: "Y"})},
		{"builder of a read the query's own builder serves", teeth(&QueryBuilder{Operation: "zippers_teeth_getToManyRelated", Name: "Y"})},
		{"builder whose reads define no bound parameter", teeth(&QueryBuilder{Operation: "widgets_getInstance", Name: "Y"})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fam := &Family{Tag: "Widgets", Queries: []*QueryBind{tt.query}}
			if _, err := Generate(spec, fam); err == nil {
				t.Fatal("expected an error")
			}
			if _, err := renderQueryRegistry(spec, &Bindings{Families: []*Family{fam}}); tt.query.Operation == "widgets_missing" && err == nil {
				t.Fatal("registry rendered an unknown operation")
			}
		})
	}
}

func TestQueryRegistryRejectsAnOperationWithTwoQueries(t *testing.T) {
	spec, _ := loadTestdata(t)
	fam := &Family{Tag: "Widgets", Queries: []*QueryBind{
		{Operation: "zippers_getCollection", Name: "Zippers"},
		{Operation: "yoyos_zippers_getToManyRelated", Also: []string{"zippers_getCollection"}, Name: "YoyoZippers"},
	}}
	if _, err := renderQueryRegistry(spec, &Bindings{Families: []*Family{fam}}); err == nil || !strings.Contains(err.Error(), "zippers_getCollection has two queries") {
		t.Fatalf("err = %v, want zippers_getCollection has two queries", err)
	}
}

func TestLowerInitial(t *testing.T) {
	tests := map[string]string{
		"GCGroups":      "gcGroups",
		"IAPOfferCodes": "iapOfferCodes",
		"CiArtifacts":   "ciArtifacts",
		"AppClips":      "appClips",
		"X":             "x",
	}
	for in, want := range tests {
		if got := lowerInitial(in); got != want {
			t.Errorf("lowerInitial(%q) = %q, want %q", in, got, want)
		}
	}
}

const handWrittenQueries = `package asc

import (
	"context"
	"fmt"
	"net/url"
)

// GetWidgets logs, so it stays hand-written.
func (c *Client) GetWidgets(ctx context.Context, opts ...WidgetsOption) (*WidgetsResponse, error) {
	query := applyOptions(opts)
	fmt.Println("listing widgets")
	return getJSON[WidgetsResponse](ctx, c, readRequest{Path: "/v1/widgets", Query: buildWidgetsQuery(query), Next: query.nextURL})
}

// GetWidgetOwner reads the owner of a widget.
func (c *Client) GetWidgetOwner(ctx context.Context, widgetID string, opts ...OwnerOption) (*WidgetResponse, error) {
	query := applyOptions(opts)
	return getJSON[WidgetResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/widgets/%s/owner", widgetID), Query: buildWidgetOwnerQuery(query)})
}

// WidgetsOption configures GetWidgets.
type WidgetsOption func(*widgetsQuery)

// OwnerOption configures GetWidgetOwner.
type OwnerOption func(*widgetOwnerQuery)

type widgetsQuery struct {
	listQuery
	names        []string
	fields       []string
	gadgetFields []string
	include      []string
}

type widgetOwnerQuery struct {
	fields []string
}

// WithWidgetsNames filters widgets by name.
func WithWidgetsNames(names []string) WidgetsOption {
	return func(q *widgetsQuery) { q.names = normalizeList(names) }
}

// WithWidgetsFields selects widget fields.
func WithWidgetsFields(fields []string) WidgetsOption {
	return func(q *widgetsQuery) { q.fields = normalizeList(fields) }
}

// WithWidgetsGadgetFields selects gadget fields.
func WithWidgetsGadgetFields(fields []string) WidgetsOption {
	return func(q *widgetsQuery) { q.gadgetFields = normalizeList(fields) }
}

// WithWidgetsIncluded includes related resources.
func WithWidgetsIncluded(include []string) WidgetsOption {
	return func(q *widgetsQuery) { q.include = normalizeList(include) }
}

// WithWidgetOwnerFields upper-cases and de-duplicates its values, so the owner query stays hand-written.
func WithWidgetOwnerFields(fields []string) OwnerOption {
	return func(q *widgetOwnerQuery) { q.fields = normalizeUniqueList(normalizeUpperList(fields)) }
}

func buildWidgetsQuery(query *widgetsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[name]", query.names)
	addCSV(values, "fields[widgets]", query.fields)
	addCSV(values, "fields[gadgets]", query.gadgetFields)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildWidgetOwnerQuery(query *widgetOwnerQuery) string {
	values := url.Values{}
	addCSV(values, "fields[widgets]", query.fields)
	return values.Encode()
}
`

func TestAdoptQueriesReplacesOnlyQueriesTheGeneratorReproduces(t *testing.T) {
	dir := writeHandWritten(t, map[string]string{"queries.go": handWrittenQueries})
	var report bytes.Buffer
	if err := run(filepath.Join("testdata", "spec.json"), filepath.Join(dir, "bindings"), dir, false, "Widgets", false, &report); err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{
		"Widgets\twidgets_getCollection\tquery buildWidgetsQuery\tgenerated",
		"Widgets\twidgets_owner_getToOneRelated\tquery buildWidgetOwnerQuery\tkept: WithWidgetOwnerFields differs at",
	} {
		if !strings.Contains(report.String(), line) {
			t.Errorf("report lacks %q:\n%s", line, report.String())
		}
	}

	_, testdata := loadTestdata(t)
	lines, err := jsonLines(testdata.Families[0].Queries[:1])
	if err != nil {
		t.Fatal(err)
	}
	want := lines[0]
	if got := readFile(t, filepath.Join(dir, "bindings", "Widgets.json")); !strings.Contains(got, "\"queries\": [\n    "+want+"\n  ]") {
		t.Errorf("bindings after adopt:\n%s\nwant the query %s", got, want)
	}

	generated := readFile(t, filepath.Join(dir, "widgets_gen.go"))
	if !strings.Contains(generated, "func buildWidgetsQuery(") || strings.Contains(generated, "type OwnerOption") || strings.Contains(generated, "func buildWidgetOwnerQuery(") {
		t.Errorf("widgets_gen.go:\n%s", generated)
	}
	queries := readFile(t, filepath.Join(dir, "queries.go"))
	for _, gone := range []string{"WidgetsOption func", "widgetsQuery struct", "func WithWidgets", "func buildWidgetsQuery"} {
		if strings.Contains(queries, gone) {
			t.Errorf("queries.go still declares %q:\n%s", gone, queries)
		}
	}
	for _, kept := range []string{"func (c *Client) GetWidgets(", "OwnerOption func", "func WithWidgetOwnerFields(", "func buildWidgetOwnerQuery(", `"net/url"`} {
		if !strings.Contains(queries, kept) {
			t.Errorf("queries.go lost %q:\n%s", kept, queries)
		}
	}
	if registry := readFile(t, filepath.Join(dir, queryRegistry)); !strings.Contains(registry, `"widgets_getCollection": {`) {
		t.Errorf("%s:\n%s", queryRegistry, registry)
	}
	if err := run(filepath.Join("testdata", "spec.json"), filepath.Join(dir, "bindings"), dir, true, "", false, &report); err != nil {
		t.Fatalf("check right after adopt: %v", err)
	}
}

func TestAdoptQueriesKeepsABuilderUsedOutsideItsOperation(t *testing.T) {
	shared := handWrittenQueries + `
func widgetsQueryForLogs() string { return buildWidgetsQuery(&widgetsQuery{}) }
`
	dir := writeHandWritten(t, map[string]string{"queries.go": shared})
	var report bytes.Buffer
	if err := run(filepath.Join("testdata", "spec.json"), filepath.Join(dir, "bindings"), dir, false, "Widgets", true, &report); err != nil {
		t.Fatal(err)
	}
	if want := "Widgets\twidgets_getCollection\tquery buildWidgetsQuery\tkept: used by widgetsQueryForLogs"; !strings.Contains(report.String(), want) {
		t.Errorf("report lacks %q:\n%s", want, report.String())
	}
}

const handWrittenSharedQuery = `package asc

import (
	"context"
	"fmt"
	"net/url"
)

// GetZippers logs, so it stays hand-written.
func (c *Client) GetZippers(ctx context.Context, opts ...ZippersOption) (*WidgetsResponse, error) {
	query := applyOptions(opts)
	fmt.Println("listing zippers")
	return getJSON[WidgetsResponse](ctx, c, readRequest{Path: "/v1/zippers", Query: buildZippersQuery(query), Next: query.nextURL})
}

// GetWidgetZippers logs, so it stays hand-written.
func (c *Client) GetWidgetZippers(ctx context.Context, widgetID string, opts ...ZippersOption) (*WidgetsResponse, error) {
	query := applyOptions(opts)
	fmt.Println("listing widget zippers")
	return getJSON[WidgetsResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/widgets/%s/zippers", widgetID), Query: buildZippersQuery(query), Next: query.nextURL})
}

// GetYoyoZippers logs, so it stays hand-written.
func (c *Client) GetYoyoZippers(ctx context.Context, yoyoID string, opts ...ZippersOption) (*WidgetsResponse, error) {
	query := applyOptions(opts)
	fmt.Println("listing yoyo zippers")
	return getJSON[WidgetsResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/yoyos/%s/zippers", yoyoID), Query: buildZippersQuery(query), Next: query.nextURL})
}

// ZippersOption configures the zipper reads.
type ZippersOption func(*zippersQuery)

type zippersQuery struct {
	listQuery
	kinds  []string
	makers []string
	fields []string
}

// WithZippersKinds filters by kind.
func WithZippersKinds(kinds []string) ZippersOption {
	return func(q *zippersQuery) { q.kinds = normalizeUpperList(kinds) }
}

// WithZippersMakers filters by maker.
func WithZippersMakers(makerIDs []string) ZippersOption {
	return func(q *zippersQuery) { q.makers = normalizeUniqueList(makerIDs) }
}

// WithZippersFields selects widget fields.
func WithZippersFields(fields []string) ZippersOption {
	return func(q *zippersQuery) { q.fields = normalizeList(fields) }
}

func buildZippersQuery(query *zippersQuery) string {
	values := url.Values{}
	addCSV(values, "filter[kind]", query.kinds)
	addCSV(values, "filter[maker]", query.makers)
	addCSV(values, "fields[widgets]", query.fields)
	addLimit(values, query.limit)
	return values.Encode()
}
`

func TestAdoptQueriesSharesABuilderAcrossReads(t *testing.T) {
	dir := writeHandWritten(t, map[string]string{"queries.go": handWrittenSharedQuery})
	spec, bindings := filepath.Join("testdata", "spec.json"), filepath.Join(dir, "bindings")
	var report bytes.Buffer
	if err := run(spec, bindings, dir, false, "Yoyos", false, &report); err != nil {
		t.Fatal(err)
	}
	if want := "Yoyos\tyoyos_zippers_getToManyRelated\tquery buildZippersQuery\tkept: shared with zippers_getCollection, adopted with tag Widgets"; !strings.Contains(report.String(), want) {
		t.Errorf("report lacks %q:\n%s", want, report.String())
	}
	if queries := readFile(t, filepath.Join(dir, "queries.go")); !strings.Contains(queries, "func buildZippersQuery(") {
		t.Fatalf("adopting Yoyos removed the shared builder:\n%s", queries)
	}

	report.Reset()
	if err := run(spec, bindings, dir, false, "Widgets", false, &report); err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{
		"Widgets\twidgets_zippers_getToManyRelated\tquery buildZippersQuery\tgenerated",
		"Widgets\tzippers_getCollection\tquery already generated",
	} {
		if !strings.Contains(report.String(), line) {
			t.Errorf("report lacks %q:\n%s", line, report.String())
		}
	}
	_, testdata := loadTestdata(t)
	lines, err := jsonLines(testdata.Families[0].Queries[2:3])
	if err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(bindings, "Widgets.json")); !strings.Contains(got, "\"queries\": [\n    "+lines[0]+"\n  ]") {
		t.Errorf("bindings after adopt:\n%s\nwant the query %s", got, lines[0])
	}
	if generated := readFile(t, filepath.Join(dir, "widgets_gen.go")); strings.Count(generated, "func buildZippersQuery(") != 1 || strings.Count(generated, "type ZippersOption ") != 1 {
		t.Errorf("widgets_gen.go:\n%s", generated)
	}
	queries := readFile(t, filepath.Join(dir, "queries.go"))
	for _, gone := range []string{"ZippersOption func", "zippersQuery struct", "func WithZippers", "func buildZippersQuery"} {
		if strings.Contains(queries, gone) {
			t.Errorf("queries.go still declares %q:\n%s", gone, queries)
		}
	}
	registry := readFile(t, filepath.Join(dir, queryRegistry))
	for _, op := range []string{"zippers_getCollection", "widgets_zippers_getToManyRelated", "yoyos_zippers_getToManyRelated"} {
		if !strings.Contains(registry, strconv.Quote(op)+": {\n\t\tbuild: buildZippersQuery,") {
			t.Errorf("%s lacks %s:\n%s", queryRegistry, op, registry)
		}
	}

	report.Reset()
	if err := run(spec, bindings, dir, false, "Yoyos", true, &report); err != nil {
		t.Fatal(err)
	}
	if want := "Yoyos\tyoyos_zippers_getToManyRelated\tquery already generated"; !strings.Contains(report.String(), want) {
		t.Errorf("report lacks %q:\n%s", want, report.String())
	}
	if err := run(spec, bindings, dir, true, "", false, &report); err != nil {
		t.Fatalf("check right after adopt: %v", err)
	}
}

const handWrittenScalarQuery = `package asc

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// GetWidgetSprockets logs, so it stays hand-written.
func (c *Client) GetWidgetSprockets(ctx context.Context, zipperID string, opts ...WidgetSprocketsOption) (*WidgetsResponse, error) {
	query := applyOptions(opts)
	fmt.Println("listing sprockets")
	return getJSON[WidgetsResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/zippers/%s/sprockets", zipperID), Query: buildWidgetSprocketsQuery(query), Next: query.nextURL})
}

// WidgetSprocketsOption configures GetWidgetSprockets.
type WidgetSprocketsOption func(*widgetSprocketsQuery)

type widgetSprocketsQuery struct {
	listQuery
	label       string
	granularity string
	fields      []string
	ownerExists *bool
	teethLimit  int
}

// WithWidgetSprocketsLabel filters by label.
func WithWidgetSprocketsLabel(label string) WidgetSprocketsOption {
	return func(q *widgetSprocketsQuery) {
		if strings.TrimSpace(label) != "" {
			q.label = strings.TrimSpace(label)
		}
	}
}

// WithWidgetSprocketsGranularity sets the granularity.
func WithWidgetSprocketsGranularity(value string) WidgetSprocketsOption {
	return func(q *widgetSprocketsQuery) {
		if strings.TrimSpace(value) != "" {
			q.granularity = strings.TrimSpace(value)
		}
	}
}

// WithWidgetSprocketsFields selects widget fields.
func WithWidgetSprocketsFields(fields []string) WidgetSprocketsOption {
	return func(q *widgetSprocketsQuery) { q.fields = normalizeList(fields) }
}

// WithWidgetSprocketsTeethLimit limits the included teeth.
func WithWidgetSprocketsTeethLimit(limit int) WidgetSprocketsOption {
	return func(q *widgetSprocketsQuery) {
		if limit > 0 {
			q.teethLimit = limit
		}
	}
}

// WithWidgetSprocketsOwnerExists filters by whether an owner exists.
func WithWidgetSprocketsOwnerExists(exists bool) WidgetSprocketsOption {
	return func(q *widgetSprocketsQuery) { q.ownerExists = &exists }
}

func buildWidgetSprocketsQuery(query *widgetSprocketsQuery) string {
	values := url.Values{}
	if strings.TrimSpace(query.label) != "" {
		values.Set("filter[label]", strings.TrimSpace(query.label))
	}
	if strings.TrimSpace(query.granularity) != "" {
		values.Set("granularity", strings.TrimSpace(query.granularity))
	}
	addCSV(values, "fields[widgets]", query.fields)
	if query.ownerExists != nil {
		values.Set("exists[owner]", strconv.FormatBool(*query.ownerExists))
	}
	addLimit(values, query.limit)
	if query.teethLimit > 0 {
		values.Set("limit[teeth]", strconv.Itoa(query.teethLimit))
	}
	return values.Encode()
}
`

func TestAdoptQueriesAdoptsScalarOptions(t *testing.T) {
	dir := writeHandWritten(t, map[string]string{"queries.go": handWrittenScalarQuery})
	var report bytes.Buffer
	if err := run(filepath.Join("testdata", "spec.json"), filepath.Join(dir, "bindings"), dir, false, "Widgets", false, &report); err != nil {
		t.Fatal(err)
	}
	if want := "Widgets\tzippers_sprockets_getToManyRelated\tquery buildWidgetSprocketsQuery\tgenerated"; !strings.Contains(report.String(), want) {
		t.Errorf("report lacks %q:\n%s", want, report.String())
	}
	_, testdata := loadTestdata(t)
	lines, err := jsonLines(testdata.Families[0].Queries[3:4])
	if err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(dir, "bindings", "Widgets.json")); !strings.Contains(got, "\"queries\": [\n    "+lines[0]+"\n  ]") {
		t.Errorf("bindings after adopt:\n%s\nwant the query %s", got, lines[0])
	}
	queries := readFile(t, filepath.Join(dir, "queries.go"))
	for _, gone := range []string{"WidgetSprocketsOption func", "widgetSprocketsQuery struct", "func WithWidgetSprockets", "func buildWidgetSprocketsQuery", `"strconv"`} {
		if strings.Contains(queries, gone) {
			t.Errorf("queries.go still declares %q:\n%s", gone, queries)
		}
	}
	if err := run(filepath.Join("testdata", "spec.json"), filepath.Join(dir, "bindings"), dir, true, "", false, &report); err != nil {
		t.Fatalf("check right after adopt: %v", err)
	}
}

func TestAdoptQueriesKeepsScalarOptionsWrittenAnotherWay(t *testing.T) {
	for _, tt := range []struct{ name, from, to, kept string }{
		{"a blank string clears the field", "if strings.TrimSpace(label) != \"\" {\n\t\t\tq.label = strings.TrimSpace(label)\n\t\t}", "q.label = strings.TrimSpace(label)", "WithWidgetSprocketsLabel differs at"},
		{"an integer kept when not positive", "if limit > 0 {\n\t\t\tq.teethLimit = limit\n\t\t}", "q.teethLimit = limit", "WithWidgetSprocketsTeethLimit differs at"},
		{"a string sent untrimmed", "if strings.TrimSpace(query.granularity) != \"\" {\n\t\tvalues.Set(\"granularity\", strings.TrimSpace(query.granularity))", "if query.granularity != \"\" {\n\t\tvalues.Set(\"granularity\", query.granularity)", "buildWidgetSprocketsQuery differs at"},
		{"an integer field the builder does not know", "\tteethLimit  int\n", "\tteethLimit  int64\n", "field teethLimit is a int64"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(handWrittenScalarQuery, tt.from) {
				t.Fatalf("fixture lacks %q", tt.from)
			}
			src := strings.Replace(handWrittenScalarQuery, tt.from, tt.to, 1)
			dir := writeHandWritten(t, map[string]string{"queries.go": src})
			var report bytes.Buffer
			if err := run(filepath.Join("testdata", "spec.json"), filepath.Join(dir, "bindings"), dir, false, "Widgets", true, &report); err != nil {
				t.Fatal(err)
			}
			if want := "Widgets\tzippers_sprockets_getToManyRelated\tquery buildWidgetSprocketsQuery\tkept: " + tt.kept; !strings.Contains(report.String(), want) {
				t.Errorf("report lacks %q:\n%s", want, report.String())
			}
		})
	}
}

const handWrittenListAndDetailQuery = `package asc

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// GetZipperTeeth logs, so it stays hand-written.
func (c *Client) GetZipperTeeth(ctx context.Context, zipperID string, opts ...ZipperTeethOption) (*WidgetsResponse, error) {
	query := applyOptions(opts)
	fmt.Println("listing teeth")
	return getJSON[WidgetsResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/zippers/%s/teeth", zipperID), Query: buildZipperTeethQuery(query), Next: query.nextURL})
}

// GetTooth logs, so it stays hand-written.
func (c *Client) GetTooth(ctx context.Context, toothID string, opts ...ZipperTeethOption) (*WidgetResponse, error) {
	query := applyOptions(opts)
	fmt.Println("reading a tooth")
	return getJSON[WidgetResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/teeth/%s", toothID), Query: buildZipperToothQuery(query)})
}

// ZipperTeethOption configures the teeth reads.
type ZipperTeethOption func(*zipperTeethQuery)

type zipperTeethQuery struct {
	listQuery
	pitches    []string
	fields     []string
	sort       string
	gearsLimit int
}

// WithZipperTeethPitches filters by pitch.
func WithZipperTeethPitches(pitches []string) ZipperTeethOption {
	return func(q *zipperTeethQuery) { q.pitches = normalizeList(pitches) }
}

// WithZipperTeethFields selects widget fields.
func WithZipperTeethFields(fields []string) ZipperTeethOption {
	return func(q *zipperTeethQuery) { q.fields = normalizeList(fields) }
}

// WithZipperTeethSort sorts the teeth.
func WithZipperTeethSort(sort string) ZipperTeethOption {
	return func(q *zipperTeethQuery) {
		if strings.TrimSpace(sort) != "" {
			q.sort = strings.TrimSpace(sort)
		}
	}
}

// WithZipperTeethGearsLimit limits the included gears.
func WithZipperTeethGearsLimit(limit int) ZipperTeethOption {
	return func(q *zipperTeethQuery) {
		if limit > 0 {
			q.gearsLimit = limit
		}
	}
}

func buildZipperTeethQuery(query *zipperTeethQuery) string {
	values := url.Values{}
	addCSV(values, "filter[pitch]", query.pitches)
	addCSV(values, "fields[widgets]", query.fields)
	if strings.TrimSpace(query.sort) != "" {
		values.Set("sort", strings.TrimSpace(query.sort))
	}
	addLimit(values, query.limit)
	if query.gearsLimit > 0 {
		values.Set("limit[gears]", strconv.Itoa(query.gearsLimit))
	}
	return values.Encode()
}

func buildZipperToothQuery(query *zipperTeethQuery) string {
	values := url.Values{}
	addCSV(values, "fields[widgets]", query.fields)
	if query.gearsLimit > 0 {
		values.Set("limit[gears]", strconv.Itoa(query.gearsLimit))
	}
	return values.Encode()
}
`

func TestAdoptQueriesAdoptsBuildersThatShareAQuery(t *testing.T) {
	dir := writeHandWritten(t, map[string]string{"queries.go": handWrittenListAndDetailQuery})
	var report bytes.Buffer
	if err := run(filepath.Join("testdata", "spec.json"), filepath.Join(dir, "bindings"), dir, false, "Widgets", false, &report); err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{
		"Widgets\tteeth_getInstance\tquery buildZipperToothQuery\tgenerated",
		"Widgets\tzippers_teeth_getToManyRelated\tquery already generated",
	} {
		if !strings.Contains(report.String(), line) {
			t.Errorf("report lacks %q:\n%s", line, report.String())
		}
	}
	_, testdata := loadTestdata(t)
	lines, err := jsonLines(testdata.Families[0].Queries[4:5])
	if err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(dir, "bindings", "Widgets.json")); !strings.Contains(got, "\"queries\": [\n    "+lines[0]+"\n  ]") {
		t.Errorf("bindings after adopt:\n%s\nwant the query %s", got, lines[0])
	}
	queries := readFile(t, filepath.Join(dir, "queries.go"))
	for _, gone := range []string{"ZipperTeethOption func", "zipperTeethQuery struct", "func WithZipperTeeth", "func buildZipperTeethQuery", "func buildZipperToothQuery"} {
		if strings.Contains(queries, gone) {
			t.Errorf("queries.go still declares %q:\n%s", gone, queries)
		}
	}
	registry := readFile(t, filepath.Join(dir, queryRegistry))
	for op, builder := range map[string]string{"zippers_teeth_getToManyRelated": "buildZipperTeethQuery", "teeth_getInstance": "buildZipperToothQuery"} {
		if !strings.Contains(registry, strconv.Quote(op)+": {\n\t\tbuild: "+builder+",") {
			t.Errorf("%s lacks %s built by %s:\n%s", queryRegistry, op, builder, registry)
		}
	}
	if err := run(filepath.Join("testdata", "spec.json"), filepath.Join(dir, "bindings"), dir, true, "", false, &report); err != nil {
		t.Fatalf("check right after adopt: %v", err)
	}
}

func TestAdoptQueriesKeepsBuildersThatShareAQueryWrittenAnotherWay(t *testing.T) {
	for _, tt := range []struct{ name, from, to, kept string }{
		{"no builder named after the query struct", "buildZipperTeethQuery", "buildZipperTeethListQuery", "query struct zipperTeethQuery is not named after one of its builders buildZipperTeethListQuery, buildZipperToothQuery"},
		{"a detail builder that sends limit", "addCSV(values, \"fields[widgets]\", query.fields)\n\tif query.gearsLimit", "addCSV(values, \"fields[widgets]\", query.fields)\n\taddLimit(values, query.limit)\n\tif query.gearsLimit", "buildZipperToothQuery differs at"},
		{"builders that send one parameter from two fields", "if query.gearsLimit > 0 {\n\t\tvalues.Set(\"limit[gears]\", strconv.Itoa(query.gearsLimit))\n\t}\n\treturn values.Encode()\n}\n`", "if len(query.pitches) > 0 {\n\t\tvalues.Set(\"limit[gears]\", strconv.Itoa(len(query.pitches)))\n\t}\n\treturn values.Encode()\n}\n`", "buildZipperToothQuery sends limit[gears] from pitches, not gearsLimit"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			src := handWrittenListAndDetailQuery + "`"
			if !strings.Contains(src, tt.from) {
				t.Fatalf("fixture lacks %q", tt.from)
			}
			src = strings.TrimSuffix(strings.ReplaceAll(src, tt.from, tt.to), "`")
			dir := writeHandWritten(t, map[string]string{"queries.go": src})
			var report bytes.Buffer
			if err := run(filepath.Join("testdata", "spec.json"), filepath.Join(dir, "bindings"), dir, false, "Widgets", true, &report); err != nil {
				t.Fatal(err)
			}
			if want := "kept: " + tt.kept; !strings.Contains(report.String(), want) {
				t.Errorf("report lacks %q:\n%s", want, report.String())
			}
		})
	}
}
