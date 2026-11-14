package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// QueryBind binds the query parameters of one paged or filtered read to its
// option type, query struct, With functions and query builder. Name is the
// base the default names grow from: build<Name>Query, <Name>Option and the
// query struct <name>Query (leading initialism lowered).
type QueryBind struct {
	Operation string        `json:"operation"`
	Name      string        `json:"name"`
	Option    string        `json:"option,omitempty"` // default <Name>Option
	Params    []*QueryParam `json:"params,omitempty"`
	// Deviations are known differences from the schema that the query test
	// in internal/asc pins. They change no generated code.
	Deviations []Deviation `json:"deviations,omitempty"`
}

// QueryParam is one query parameter with an option. The parameter must be an
// array of strings in the spec: the option normalizes the list and the
// builder sends it comma-separated.
type QueryParam struct {
	Name string `json:"name"`
	// Field is the query struct field; default "include" for include,
	// "fields" for fields[<the read's resource type>] and
	// <resource>Fields for other fields[...] parameters.
	Field string `json:"field,omitempty"`
	With  string `json:"with,omitempty"` // default With<Name><Field>
	Arg   string `json:"arg,omitempty"`  // With argument; default the field name
}

// queryRegistry is the test file that lists every generated query builder and
// its options for the schema test in internal/asc.
const queryRegistry = "openapi_queries_gen_test.go"

func (q *QueryBind) option() string {
	if q.Option != "" {
		return q.Option
	}
	return q.Name + "Option"
}

func (q *QueryBind) builder() string { return "build" + q.Name + "Query" }

func (q *QueryBind) structName() string { return lowerInitial(q.Name) + "Query" }

var leadingUpper = regexp.MustCompile(`^[A-Z]+`)

// lowerInitial lowers the leading initialism of a name: "GCGroups" ->
// "gcGroups", "AppClips" -> "appClips".
func lowerInitial(name string) string {
	run := leadingUpper.FindString(name)
	if len(run) > 1 && len(run) < len(name) {
		run = run[:len(run)-1]
	}
	return strings.ToLower(run) + name[len(run):]
}

// queryParam is a query parameter of an operation in the spec.
type queryParam struct {
	Name   string `json:"name"`
	In     string `json:"in"`
	Schema struct {
		Type  string `json:"type"`
		Items *struct {
			Type string `json:"type"`
		} `json:"items"`
	} `json:"schema"`
}

// queryParams returns the query parameters of an operation, path-level ones
// included, in spec order.
func (s *Spec) queryParams(op *Operation) ([]queryParam, error) {
	item := s.Paths[op.Path]
	var out []queryParam
	for _, raw := range []json.RawMessage{item[strings.ToLower(op.Method)], item["parameters"]} {
		if raw == nil {
			continue
		}
		var params []queryParam
		if raw[0] == '{' {
			var withParams struct {
				Parameters []queryParam `json:"parameters"`
			}
			if err := json.Unmarshal(raw, &withParams); err != nil {
				return nil, err
			}
			params = withParams.Parameters
		} else if err := json.Unmarshal(raw, &params); err != nil {
			return nil, err
		}
		for _, p := range params {
			if p.In == "query" {
				out = append(out, p)
			}
		}
	}
	return out, nil
}

// field is the query struct field of a parameter: the binding's, else the
// rule documented on QueryParam.Field.
func (q *QueryBind) field(spec *Spec, op *Operation, p *QueryParam) string {
	if p.Field != "" {
		return p.Field
	}
	if p.Name == "include" {
		return "include"
	}
	typ, ok := strings.CutPrefix(p.Name, "fields[")
	if !ok || !strings.HasSuffix(typ, "]") {
		return ""
	}
	typ = strings.TrimSuffix(typ, "]")
	if typ == readType(spec, op) {
		return "fields"
	}
	resource, version := singular(spec, typ)
	return lowerInitial(resource+version) + "Fields"
}

// readType is the JSON:API type of the resources a read returns.
func readType(spec *Spec, op *Operation) string {
	resource, _, ok := spec.resourceOf(op.Response)
	if !ok {
		return ""
	}
	sch, _ := spec.Schema(resource)
	if sch == nil {
		return ""
	}
	if t := sch.Properties.Get("type"); t != nil && len(t.Enum) == 1 {
		return t.Enum[0]
	}
	return ""
}

func (q *QueryBind) with(field string, p *QueryParam) string {
	if p.With != "" {
		return p.With
	}
	return "With" + q.Name + goName(field)
}

// queryPart is one generated declaration of a query.
type queryPart struct {
	name string
	src  string
}

// renderQuery renders the option type, the query struct, one With function
// per parameter and the query builder of a read, in that order.
func renderQuery(spec *Spec, q *QueryBind) ([]queryPart, error) {
	op, ok := spec.Operation(q.Operation)
	if !ok {
		return nil, fmt.Errorf("operation not in spec")
	}
	if op.Method != "GET" {
		return nil, fmt.Errorf("query of a %s operation", op.Method)
	}
	if q.Name == "" {
		return nil, fmt.Errorf("query without a name")
	}
	specParams, err := spec.queryParams(op)
	if err != nil {
		return nil, err
	}
	defined := map[string]queryParam{}
	for _, p := range specParams {
		defined[p.Name] = p
	}
	_, paged := defined["limit"]

	option, structName := q.option(), q.structName()
	var fields, sends []string
	if paged {
		fields = append(fields, "listQuery")
	}
	var parts []queryPart
	seen := map[string]bool{}
	for _, p := range q.Params {
		sp, ok := defined[p.Name]
		switch {
		case !ok:
			return nil, fmt.Errorf("parameter %s is not defined by the operation", p.Name)
		case p.Name == "limit":
			return nil, fmt.Errorf("limit is set by Limit, not by a parameter binding")
		case sp.Schema.Type != "array" || sp.Schema.Items == nil || sp.Schema.Items.Type != "string":
			return nil, fmt.Errorf("parameter %s is not an array of strings", p.Name)
		case seen[p.Name]:
			return nil, fmt.Errorf("parameter %s is bound twice", p.Name)
		}
		seen[p.Name] = true
		field := q.field(spec, op, p)
		if field == "" {
			return nil, fmt.Errorf("parameter %s needs a field name", p.Name)
		}
		with, arg := q.with(field, p), p.Arg
		if arg == "" {
			arg = field
		}
		fields = append(fields, field+" []string")
		parts = append(parts, queryPart{with, fmt.Sprintf("// %s sets %s.\nfunc %s(%s []string) %s {\nreturn func(q *%s) {\nq.%s = normalizeList(%s)\n}\n}\n\n",
			with, p.Name, with, arg, option, structName, field, arg)})
		sends = append(sends, fmt.Sprintf("addCSV(values, %s, query.%s)", strconv.Quote(p.Name), field))
	}
	if paged {
		sends = append(sends, "addLimit(values, query.limit)")
	}
	if len(sends) == 0 {
		return nil, fmt.Errorf("query sends nothing")
	}
	head := []queryPart{
		{option, fmt.Sprintf("// %s sets the query of %s %s (%s).\ntype %s func(*%s)\n\n", option, op.Method, op.Path, op.ID, option, structName)},
		{structName, fmt.Sprintf("type %s struct {\n%s\n}\n\n", structName, strings.Join(fields, "\n"))},
	}
	builder := queryPart{q.builder(), fmt.Sprintf("func %s(query *%s) string {\nvalues := url.Values{}\n%s\nreturn values.Encode()\n}\n\n",
		q.builder(), structName, strings.Join(sends, "\n"))}
	return append(append(head, parts...), builder), nil
}

// renderQueries renders the queries of a family.
func renderQueries(spec *Spec, fam *Family) (string, error) {
	var out strings.Builder
	for _, q := range fam.Queries {
		parts, err := renderQuery(spec, q)
		if err != nil {
			return "", fmt.Errorf("%s: query %s: %w", fam.Tag, q.Operation, err)
		}
		for _, part := range parts {
			out.WriteString(part.src)
		}
	}
	return out.String(), nil
}

// renderQueryRegistry renders the test file that lists every generated query
// by operation: its builder and its With functions by parameter. It returns
// nil when no family has a query.
func renderQueryRegistry(spec *Spec, bindings *Bindings) ([]byte, error) {
	var queries []*QueryBind
	for _, fam := range bindings.Families {
		queries = append(queries, fam.Queries...)
	}
	if len(queries) == 0 {
		return nil, nil
	}
	sort.Slice(queries, func(i, j int) bool { return queries[i].Operation < queries[j].Operation })
	var out strings.Builder
	out.WriteString(header + "\n\npackage asc\n\n")
	out.WriteString("// generatedQuery is a generated query builder and its With functions by\n// query parameter.\ntype generatedQuery struct {\nbuild any\noptions map[string]any\n}\n\n")
	out.WriteString("// generatedQueries lists the generated queries by operation for the schema test.\nvar generatedQueries = map[string]generatedQuery{\n")
	for _, q := range queries {
		op, ok := spec.Operation(q.Operation)
		if !ok {
			return nil, fmt.Errorf("query %s: operation not in spec", q.Operation)
		}
		fmt.Fprintf(&out, "%s: {\nbuild: %s,\noptions: map[string]any{\n", strconv.Quote(q.Operation), q.builder())
		for _, p := range q.Params {
			fmt.Fprintf(&out, "%s: %s,\n", strconv.Quote(p.Name), q.with(q.field(spec, op, p), p))
		}
		out.WriteString("},\n},\n")
	}
	out.WriteString("}\n")
	return format.Source([]byte(out.String()))
}

// planQueryRegistry adds the query registry to the planned writes when it
// differs from disk, and marks it stale when no family has a query.
func planQueryRegistry(spec *Spec, bindings *Bindings, dir string, writes map[string][]byte, stale *[]string) error {
	src, err := renderQueryRegistry(spec, bindings)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, queryRegistry)
	old, readErr := readGenerated(path)
	switch {
	case src == nil && readErr == nil && old != nil:
		*stale = append(*stale, path)
	case src != nil && (readErr != nil || string(old) != string(src)):
		writes[path] = src
	}
	return nil
}

// readGenerated reads a file the generator wrote; it returns nil for a file
// without the generated header.
func readGenerated(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil || !bytes.HasPrefix(data, []byte(header)) {
		return nil, err
	}
	return data, nil
}

// encodeLine encodes a binding as one JSON line.
func encodeLine(v any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}
