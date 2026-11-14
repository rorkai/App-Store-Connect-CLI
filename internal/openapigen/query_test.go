package main

import (
	"bytes"
	"os"
	"path/filepath"
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
		{"parameter bound twice", &QueryBind{Operation: "widgets_getCollection", Name: "X", Params: []*QueryParam{param("include", ""), param("include", "other")}}},
		{"filter without a field name", &QueryBind{Operation: "widgets_getCollection", Name: "X", Params: []*QueryParam{param("filter[name]", "")}}},
		{"a query that sends nothing", &QueryBind{Operation: "widgets_owner_getToOneRelated", Name: "X"}},
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

// WithWidgetOwnerFields upper-cases its values, so the owner query stays hand-written.
func WithWidgetOwnerFields(fields []string) OwnerOption {
	return func(q *widgetOwnerQuery) { q.fields = normalizeUpperList(fields) }
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
	want, err := encodeLine(testdata.Families[0].Queries[0])
	if err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(dir, "bindings", "Widgets.json")); !strings.Contains(got, "\"queries\": [\n    "+want+"\n  ]") {
		t.Errorf("bindings after adopt:\n%s\nwant the query %s", got, want)
	}

	generated := readFile(t, filepath.Join(dir, "widgets_gen.go"))
	if !strings.Contains(generated, "func buildWidgetsQuery(") || strings.Contains(generated, "OwnerOption") {
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
