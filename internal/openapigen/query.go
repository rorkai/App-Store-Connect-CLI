package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// QueryBind binds the query parameters of one paged or filtered read to its
// option type, query struct, With functions and query builder. Name is the
// base the default names grow from: build<Name>Query, <Name>Option and the
// query struct <name>Query (leading initialism lowered).
type QueryBind struct {
	Operation string `json:"operation"`
	// Also are the other reads whose methods call the same builder. The
	// naming rules follow Operation, and the query is generated once, in the
	// family of Operation.
	Also   []string      `json:"also,omitempty"`
	Name   string        `json:"name"`
	Option string        `json:"option,omitempty"` // default <Name>Option
	Params []*QueryParam `json:"params,omitempty"`
	// Builders are more builders of the same query struct and options, for
	// reads that define fewer of its parameters, such as the detail read of a
	// list. Each builder sends only the parameters its reads define.
	Builders []*QueryBuilder `json:"builders,omitempty"`
	// Deviations are known differences from the schema that the query test
	// in internal/asc pins. They change no generated code.
	Deviations []QueryDeviation `json:"deviations,omitempty"`
}

// QueryBuilder is one more builder of a query: build<Name>Query, serving
// Operation and Also.
type QueryBuilder struct {
	Operation string   `json:"operation"`
	Also      []string `json:"also,omitempty"`
	Name      string   `json:"name"`
}

// QueryDeviation is a Deviation of one read a query serves; Operation is
// empty for the query's own Operation.
type QueryDeviation struct {
	Operation string `json:"operation,omitempty"`
	Problem   string `json:"problem"`
	Reason    string `json:"reason"`
}

// QueryParam is one query parameter with an option. By default the parameter
// must be an array of strings sent in form style without explode: the option
// normalizes the list and the builder sends it comma-separated. Type binds a
// scalar option instead, one of the keys of scalarParams.
type QueryParam struct {
	Name string `json:"name"`
	// Field is the query struct field; default "include" for include,
	// "fields" for fields[<the read's resource type>] and
	// <resource>Fields for other fields[...] parameters.
	Field string `json:"field,omitempty"`
	With  string `json:"with,omitempty"` // default With<Name><Field>
	Arg   string `json:"arg,omitempty"`  // With argument; default the field name
	// Normalize names how the option cleans the list, one of the keys of
	// listNormalizers.
	Normalize string `json:"normalize,omitempty"`
	Type      string `json:"type,omitempty"`
}

// scalarParam is how a scalar option is declared, set and sent.
type scalarParam struct {
	goType  string
	schemas []string // the schema types it may send: "string" also sends one item of an array of strings
	set     string   // body of the option's closure; %[1]s is the argument, %[2]s the field, %[3]s the normalizer
	send    string   // builder statement; %[1]s is the field, %[2]s the quoted parameter name
	arg     string   // the argument's Go type when it is not goType without its pointer
	// normalizers are the functions the option may clean its value with, by
	// QueryParam.Normalize; none for an option that takes no Normalize.
	normalizers map[string]string
}

// sendString is how a string field is sent: trimmed, and not at all when
// blank.
const sendString = "if strings.TrimSpace(query.%[1]s) != \"\" {\nvalues.Set(%[2]s, strings.TrimSpace(query.%[1]s))\n}"

// scalarParams are the scalar options by QueryParam.Type. A string is
// trimmed and a blank one leaves the field as it was; an integer is kept only
// when positive and sent after limit; a boolean is sent as true or false once
// set. A csv option takes comma-separated values in one string and a csvList
// option a list; both keep the cleaned values as one comma-separated string,
// and leave the field as it was when no value is left.
var scalarParams = map[string]scalarParam{
	"string": {
		"string",
		[]string{"string", "array"},
		"if strings.TrimSpace(%[1]s) != \"\" {\nq.%[2]s = strings.TrimSpace(%[1]s)\n}",
		sendString, "", nil,
	},
	"int": {
		"int",
		[]string{"integer"},
		"if %[1]s > 0 {\nq.%[2]s = %[1]s\n}",
		"if query.%[1]s > 0 {\nvalues.Set(%[2]s, strconv.Itoa(query.%[1]s))\n}", "", nil,
	},
	"bool": {
		"*bool",
		[]string{"boolean"},
		"q.%[2]s = &%[1]s",
		"if query.%[1]s != nil {\nvalues.Set(%[2]s, strconv.FormatBool(*query.%[1]s))\n}", "", nil,
	},
	"csv": {
		"string",
		[]string{"array"},
		"normalized := %[3]s(%[1]s)\nif normalized != \"\" {\nq.%[2]s = normalized\n}",
		sendString, "",
		map[string]string{"": "normalizeCSVString", "upper": "normalizeUpperCSVString"},
	},
	"csvList": {
		"string",
		[]string{"array"},
		"normalized := %[3]s(%[1]s)\nif len(normalized) > 0 {\nq.%[2]s = strings.Join(normalized, \",\")\n}",
		sendString, "[]string", listNormalizers,
	},
}

// argType is the Go type of a scalar option's argument.
func (s scalarParam) argType() string {
	if s.arg != "" {
		return s.arg
	}
	return strings.TrimPrefix(s.goType, "*")
}

// listNormalizers are the functions an option cleans its list with, by
// QueryParam.Normalize.
var listNormalizers = map[string]string{
	"":       "normalizeList",       // trim, drop blank values
	"unique": "normalizeUniqueList", // and drop repeated values
	"upper":  "normalizeUpperList",  // and upper-case the values
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

// operations are the reads the query's own builder serves, its own first.
func (q *QueryBind) operations() []string { return append([]string{q.Operation}, q.Also...) }

// queryBuilder is one builder of a query and the reads it serves; render
// fills in the parameters they define and whether one of them pages.
type queryBuilder struct {
	name       string
	operations []string
	defined    map[string]bool
	paged      bool
}

// builders are the query's own builder, then its Builders.
func (q *QueryBind) builders() []queryBuilder {
	out := []queryBuilder{{name: q.builder(), operations: q.operations()}}
	for _, b := range q.Builders {
		out = append(out, queryBuilder{name: "build" + b.Name + "Query", operations: append([]string{b.Operation}, b.Also...)})
	}
	return out
}

// reads are the reads every builder of the query serves.
func (q *QueryBind) reads() []string {
	var out []string
	for _, b := range q.builders() {
		out = append(out, b.operations...)
	}
	return out
}

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
	Name    string `json:"name"`
	In      string `json:"in"`
	Style   string `json:"style"`
	Explode *bool  `json:"explode"`
	Schema  struct {
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
	if q.Name == "" {
		return nil, fmt.Errorf("query without a name")
	}
	var ops []*Operation
	defined := map[string][]queryParam{}
	paged := false
	var builders []queryBuilder
	for i, b := range q.builders() {
		switch {
		case i > 0 && q.Builders[i-1].Name == "":
			return nil, fmt.Errorf("builder of %s without a name", b.operations[0])
		case slices.ContainsFunc(builders, func(other queryBuilder) bool { return other.name == b.name }):
			return nil, fmt.Errorf("builder %s bound twice", b.name)
		}
		b.defined = map[string]bool{}
		for _, id := range b.operations {
			op, ok := spec.Operation(id)
			switch {
			case !ok:
				return nil, fmt.Errorf("operation %s not in spec", id)
			case op.Method != "GET":
				return nil, fmt.Errorf("query of a %s operation %s", op.Method, id)
			case slices.Contains(ops, op):
				return nil, fmt.Errorf("operation %s listed twice", id)
			}
			ops = append(ops, op)
			specParams, err := spec.queryParams(op)
			if err != nil {
				return nil, err
			}
			for _, p := range specParams {
				defined[p.Name] = append(defined[p.Name], p)
				b.defined[p.Name] = true
				b.paged = b.paged || p.Name == "limit"
			}
		}
		paged = paged || b.paged
		builders = append(builders, b)
	}
	op := ops[0]

	option, structName := q.option(), q.structName()
	var fields []string
	if paged {
		fields = append(fields, "listQuery")
	}
	var parts []queryPart
	var sent []paramSend
	seen := map[string]bool{}
	for _, p := range q.Params {
		normalize, known := listNormalizers[p.Normalize]
		scalar, isScalar := scalarParams[p.Type]
		switch {
		case len(defined[p.Name]) == 0:
			return nil, fmt.Errorf("parameter %s is not defined by %s", p.Name, strings.Join(q.reads(), " or "))
		case p.Name == "limit":
			return nil, fmt.Errorf("limit is set by Limit, not by a parameter binding")
		case seen[p.Name]:
			return nil, fmt.Errorf("parameter %s is bound twice", p.Name)
		case !known:
			return nil, fmt.Errorf("parameter %s has unknown normalize %q", p.Name, p.Normalize)
		case p.Type != "" && !isScalar:
			return nil, fmt.Errorf("parameter %s has unknown type %q", p.Name, p.Type)
		case isScalar && p.Normalize != "" && scalar.normalizers[p.Normalize] == "":
			return nil, fmt.Errorf("parameter %s of type %s cannot normalize %q", p.Name, p.Type, p.Normalize)
		}
		for _, sp := range defined[p.Name] {
			if err := checkParamSchema(p, sp, scalar); err != nil {
				return nil, err
			}
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
		if isScalar {
			fields = append(fields, field+" "+scalar.goType)
			parts = append(parts, queryPart{with, fmt.Sprintf("// %s sets %s.\nfunc %s(%s %s) %s {\nreturn func(q *%s) {\n%s\n}\n}\n\n",
				with, p.Name, with, arg, scalar.argType(), option, structName, fmt.Sprintf(scalar.set, arg, field, scalar.normalizers[p.Normalize]))})
			sent = append(sent, paramSend{p.Name, fmt.Sprintf(scalar.send, field, strconv.Quote(p.Name)), p.Type == "int"})
			continue
		}
		fields = append(fields, field+" []string")
		parts = append(parts, queryPart{with, fmt.Sprintf("// %s sets %s.\nfunc %s(%s []string) %s {\nreturn func(q *%s) {\nq.%s = %s(%s)\n}\n}\n\n",
			with, p.Name, with, arg, option, structName, field, normalize, arg)})
		sent = append(sent, paramSend{p.Name, fmt.Sprintf("addCSV(values, %s, query.%s)", strconv.Quote(p.Name), field), false})
	}
	var reads []string
	for _, op := range ops {
		reads = append(reads, fmt.Sprintf("%s %s (%s)", op.Method, op.Path, op.ID))
	}
	served := reads[0]
	if n := len(reads); n > 1 {
		served = strings.Join(reads[:n-1], ", ") + " and " + reads[n-1]
	}
	parts = append([]queryPart{
		{option, fmt.Sprintf("// %s sets the query of %s.\ntype %s func(*%s)\n\n", option, served, option, structName)},
		{structName, fmt.Sprintf("type %s struct {\n%s\n}\n\n", structName, strings.Join(fields, "\n"))},
	}, parts...)
	for _, b := range builders {
		var sends, nestedLimits []string
		for _, s := range sent {
			switch {
			case !b.defined[s.name]:
			case s.nestedLimit:
				nestedLimits = append(nestedLimits, s.stmt)
			default:
				sends = append(sends, s.stmt)
			}
		}
		if b.paged {
			sends = append(sends, "addLimit(values, query.limit)")
		}
		sends = append(sends, nestedLimits...)
		if len(sends) == 0 {
			return nil, fmt.Errorf("%s sends nothing", b.name)
		}
		parts = append(parts, queryPart{b.name, fmt.Sprintf("func %s(query *%s) string {\nvalues := url.Values{}\n%s\nreturn values.Encode()\n}\n\n",
			b.name, structName, strings.Join(sends, "\n"))})
	}
	return parts, nil
}

// paramSend is the builder statement that sends a bound parameter; an
// integer is a nested limit, sent after limit.
type paramSend struct {
	name, stmt  string
	nestedLimit bool
}

// checkParamSchema checks that a read's parameter is sent as its binding
// sends it: an array of strings in form style without explode for a list or
// a string option, and the schema type of a scalar option.
func checkParamSchema(p *QueryParam, sp queryParam, scalar scalarParam) error {
	isArray := sp.Schema.Type == "array"
	if p.Type != "" && !slices.Contains(scalar.schemas, sp.Schema.Type) {
		return fmt.Errorf("parameter %s of type %s is a %s in the schema", p.Name, p.Type, sp.Schema.Type)
	}
	if p.Type != "" && !isArray {
		return nil
	}
	switch {
	case !isArray || sp.Schema.Items == nil || sp.Schema.Items.Type != "string":
		return fmt.Errorf("parameter %s is not an array of strings", p.Name)
	case sp.Explode == nil || *sp.Explode || (sp.Style != "" && sp.Style != "form"):
		return fmt.Errorf("parameter %s is not sent as one comma-separated value (form style, explode false)", p.Name)
	}
	return nil
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
	type entry struct {
		operation string
		query     *QueryBind
		builder   string
	}
	var entries []entry
	for _, fam := range bindings.Families {
		for _, q := range fam.Queries {
			for _, b := range q.builders() {
				for _, id := range b.operations {
					entries = append(entries, entry{id, q, b.name})
				}
			}
		}
	}
	if len(entries) == 0 {
		return nil, nil
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].operation < entries[j].operation })
	for i := 1; i < len(entries); i++ {
		if entries[i].operation == entries[i-1].operation {
			return nil, fmt.Errorf("operation %s has two queries", entries[i].operation)
		}
	}
	var out strings.Builder
	out.WriteString(header + "\n\npackage asc\n\n")
	out.WriteString("// generatedQuery is a generated query builder and its With functions by\n// query parameter.\ntype generatedQuery struct {\nbuild any\noptions map[string]any\n}\n\n")
	out.WriteString("// generatedQueries lists the generated queries by operation for the schema test.\nvar generatedQueries = map[string]generatedQuery{\n")
	for _, e := range entries {
		q := e.query
		op, ok := spec.Operation(q.Operation)
		if !ok {
			return nil, fmt.Errorf("query %s: operation not in spec", q.Operation)
		}
		fmt.Fprintf(&out, "%s: {\nbuild: %s,\noptions: map[string]any{\n", strconv.Quote(e.operation), e.builder)
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
