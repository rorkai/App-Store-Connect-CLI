package asc

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// contractQueryParam is a query parameter of an operation as the schema
// defines it.
type contractQueryParam struct {
	Name    string          `json:"name"`
	In      string          `json:"in"`
	Schema  *contractSchema `json:"schema"`
	Explode *bool           `json:"explode"`
}

// loadContractQueryParams reads the query parameters of every operation from
// the snapshot, by operationId.
var loadContractQueryParams = sync.OnceValues(func() (map[string]map[string]contractQueryParam, error) {
	data, err := os.ReadFile(contractSpecPath)
	if err != nil {
		return nil, err
	}
	var raw struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	out := map[string]map[string]contractQueryParam{}
	for path, item := range raw.Paths {
		var shared []contractQueryParam
		if rawParams, ok := item["parameters"]; ok {
			if err := json.Unmarshal(rawParams, &shared); err != nil {
				return nil, fmt.Errorf("%s parameters: %w", path, err)
			}
		}
		for method, rawOp := range item {
			if method == "parameters" {
				continue
			}
			var op struct {
				ID         string               `json:"operationId"`
				Parameters []contractQueryParam `json:"parameters"`
			}
			if err := json.Unmarshal(rawOp, &op); err != nil {
				return nil, fmt.Errorf("%s %s: %w", method, path, err)
			}
			params := map[string]contractQueryParam{}
			for _, p := range append(op.Parameters, shared...) {
				if p.In == "query" {
					params[p.Name] = p
				}
			}
			out[op.ID] = params
		}
	}
	return out, nil
})

type contractQueryFamily struct {
	Tag     string `json:"tag"`
	Queries []struct {
		Operation string   `json:"operation"`
		Also      []string `json:"also"`
		Name      string   `json:"name"`
		Params    []struct {
			Name string `json:"name"`
			contractBoundParam
		} `json:"params"`
		Deviations []struct {
			Operation string `json:"operation"`
			Problem   string `json:"problem"`
			Reason    string `json:"reason"`
		} `json:"deviations"`
	} `json:"queries"`
}

// contractBoundParam is how the bindings say a query parameter's option
// cleans and sends its value.
type contractBoundParam struct {
	Normalize string `json:"normalize"`
	Type      string `json:"type"`
}

// TestGeneratedQueriesMatchSchema calls every generated query builder with
// each of its With options set to a synthetic value and checks, for every read
// the query serves, that the option sends exactly the parameter the schema
// defines for it: a list cleaned as its binding says (trimmed, without blank
// items, and de-duplicated or upper-cased), a trimmed string, a positive
// integer or a boolean, encoded as the schema says, and nothing for a blank
// string or an integer that is not positive; that limit is sent for an
// operation that pages; and that every query parameter of the operation has
// an option. Differences recorded in the bindings as deviations, on the read
// they concern, are excused exactly as for methods.
func TestGeneratedQueriesMatchSchema(t *testing.T) {
	params, err := loadContractQueryParams()
	if err != nil {
		t.Fatal(err)
	}
	paths, err := filepath.Glob(filepath.Join("openapi_bindings", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	bound := map[string]bool{}
	queries, deviations := 0, 0
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var family contractQueryFamily
		if err := json.Unmarshal(data, &family); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		for _, b := range family.Queries {
			deviations += len(b.Deviations)
			paramBindings := map[string]contractBoundParam{}
			for _, p := range b.Params {
				paramBindings[p.Name] = p.contractBoundParam
			}
			served := append([]string{b.Operation}, b.Also...)
			for _, d := range b.Deviations {
				if d.Operation != "" && !slices.Contains(b.Also, d.Operation) {
					t.Errorf("query %s: deviation %q is recorded on %s, which the query does not serve", b.Name, d.Problem, d.Operation)
				}
			}
			for _, operation := range served {
				queries++
				bound[operation] = true
				opParams, ok := params[operation]
				if !ok {
					t.Errorf("query %s (%s): operation not in %s", b.Name, operation, contractSpecPath)
					continue
				}
				binding := &contractOpBinding{Operation: operation, Name: b.Name}
				for _, d := range b.Deviations {
					if d.Operation == operation || d.Operation == "" && operation == b.Operation {
						binding.Deviations = append(binding.Deviations, struct {
							Problem string `json:"problem"`
							Reason  string `json:"reason"`
						}{d.Problem, d.Reason})
					}
				}
				for _, problem := range unrecordedContractProblems(family.Tag, binding, checkGeneratedQuery(operation, opParams, paramBindings)) {
					t.Errorf("query of %s (build%sQuery): %s", operation, b.Name, problem)
				}
			}
		}
	}
	for _, op := range slices.Sorted(maps.Keys(generatedQueries)) {
		if !bound[op] {
			t.Errorf("generatedQueries has %s, which no bindings file binds", op)
		}
	}
	if queries != len(generatedQueries) {
		t.Errorf("%d reads served by the queries in the bindings, %d in generatedQueries", queries, len(generatedQueries))
	}
	t.Logf("%d reads of generated queries checked, %d recorded deviations", queries, deviations)
}

// contractQueryValues is the synthetic list every list option is called
// with; a generated option sends it as contractQuerySent says for the
// option's normalize binding.
var (
	contractQueryValues = []string{" contract-1 ", "", "contract-2", "contract-1"}
	contractQuerySent   = map[string][]string{
		"":       {"contract-1", "contract-2", "contract-1"},
		"unique": {"contract-1", "contract-2"},
		"upper":  {"CONTRACT-1", "CONTRACT-2", "CONTRACT-1"},
	}
)

// contractScalar is how a scalar option of a type binding is checked: its
// argument type, the schema types it may send, and the cases it is called
// with, each a run of calls on one query and the value it then sends (nil
// when the parameter is left out).
type contractScalar struct {
	arg     reflect.Type
	schemas []string
	cases   []contractScalarCase
}

type contractScalarCase struct {
	calls []any
	sent  []string
}

var contractScalars = map[string]contractScalar{
	"string": {reflect.TypeFor[string](), []string{"string", "array"}, []contractScalarCase{
		{[]any{" contract-1 "}, []string{"contract-1"}},
		{[]any{""}, nil},
		{[]any{"   "}, nil},
		{[]any{"contract-1", " "}, []string{"contract-1"}},
	}},
	"int": {reflect.TypeFor[int](), []string{"integer"}, []contractScalarCase{
		{[]any{3}, []string{"3"}},
		{[]any{0}, nil},
		{[]any{-1}, nil},
		{[]any{3, 0}, []string{"3"}},
	}},
	"bool": {reflect.TypeFor[bool](), []string{"boolean"}, []contractScalarCase{
		{[]any{true}, []string{"true"}},
		{[]any{false}, []string{"false"}},
	}},
}

// checkGeneratedQuery returns every difference between what the generated
// query of an operation sends and what the schema defines. bound holds the
// binding of each parameter.
func checkGeneratedQuery(operation string, params map[string]contractQueryParam, bound map[string]contractBoundParam) []string {
	query, ok := generatedQueries[operation]
	if !ok {
		return []string{"not in generatedQueries"}
	}
	build := reflect.ValueOf(query.build)
	if build.Kind() != reflect.Func || build.Type().NumIn() != 1 || build.Type().In(0).Kind() != reflect.Pointer || build.Type().NumOut() != 1 || build.Type().Out(0).Kind() != reflect.String {
		return []string{fmt.Sprintf("builder %T is not a func(*query) string", query.build)}
	}
	queryType := build.Type().In(0)
	_, paged := params["limit"]
	pages := queryType.Implements(reflect.TypeFor[pager]())
	var problems []string
	switch {
	case paged && !pages:
		return []string{"the operation pages with limit, the query has no page size"}
	case !paged && pages:
		problems = append(problems, "the query has a page size, the operation defines no limit")
	}

	send := func(apply func(q reflect.Value)) url.Values {
		q := reflect.New(queryType.Elem())
		if paged {
			q.Interface().(pager).page().limit = contractLimit
		}
		apply(q)
		values, err := url.ParseQuery(build.Call([]reflect.Value{q})[0].String())
		if err != nil {
			return url.Values{"unparsable": {err.Error()}}
		}
		return values
	}
	base := url.Values{}
	if paged {
		base.Set("limit", strconv.Itoa(contractLimit))
	}
	if got := send(func(reflect.Value) {}); !reflect.DeepEqual(got, base) {
		problems = append(problems, fmt.Sprintf("sent %v with no option set, want %v", got, base))
	}

	for _, name := range slices.Sorted(maps.Keys(query.options)) {
		option := reflect.ValueOf(query.options[name])
		label := runtime.FuncForPC(option.Pointer()).Name()
		label = label[strings.LastIndex(label, ".")+1:]
		param, defined := params[name]
		if !defined {
			problems = append(problems, fmt.Sprintf("%s sets %s, which the operation does not define", label, name))
			continue
		}
		typ := option.Type()
		arg := reflect.TypeFor[[]string]()
		scalar, isScalar := contractScalars[bound[name].Type]
		switch {
		case isScalar:
			arg = scalar.arg
		case bound[name].Type != "":
			problems = append(problems, fmt.Sprintf("%s has unknown type %q", label, bound[name].Type))
			continue
		}
		if typ.NumIn() != 1 || typ.In(0) != arg || typ.NumOut() != 1 || typ.Out(0).Kind() != reflect.Func ||
			typ.Out(0).NumIn() != 1 || typ.Out(0).In(0) != queryType {
			problems = append(problems, fmt.Sprintf("%s is a %s, not a func(%s) option of %s", label, typ, arg, queryType))
			continue
		}
		if isScalar {
			problems = append(problems, checkScalarOption(label, name, param, scalar, option, send, base)...)
			continue
		}
		sent, ok := contractQuerySent[bound[name].Normalize]
		if !ok {
			problems = append(problems, fmt.Sprintf("%s has unknown normalize %q", label, bound[name].Normalize))
			continue
		}
		set := option.Call([]reflect.Value{reflect.ValueOf(slices.Clone(contractQueryValues))})[0]
		got := send(func(q reflect.Value) { set.Call([]reflect.Value{q}) })
		want := url.Values{}
		maps.Copy(want, base)
		want[name] = contractQueryEncoding(param, sent)
		if !reflect.DeepEqual(got, want) {
			problems = append(problems, fmt.Sprintf("%s(%q) sent %v, want %v", label, contractQueryValues, got, want))
		}
	}

	var missing []string
	for name := range params {
		if _, ok := query.options[name]; !ok && name != "limit" {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		problems = append(problems, fmt.Sprintf("query parameters %v have no option", missing))
	}
	return problems
}

// checkScalarOption calls a scalar option with each of its cases and returns
// every case that does not send the parameter as the schema encodes it, or
// does not leave it out.
func checkScalarOption(label, name string, param contractQueryParam, scalar contractScalar, option reflect.Value, send func(func(reflect.Value)) url.Values, base url.Values) []string {
	schemaType := ""
	if param.Schema != nil {
		schemaType = param.Schema.Type
	}
	if !slices.Contains(scalar.schemas, schemaType) {
		return []string{fmt.Sprintf("%s takes a %s, the operation defines %s as a %s", label, scalar.arg, name, schemaType)}
	}
	var problems []string
	for _, c := range scalar.cases {
		got := send(func(q reflect.Value) {
			for _, value := range c.calls {
				option.Call([]reflect.Value{reflect.ValueOf(value)})[0].Call([]reflect.Value{q})
			}
		})
		want := url.Values{}
		maps.Copy(want, base)
		if c.sent != nil {
			want[name] = contractQueryEncoding(param, c.sent)
		}
		if !reflect.DeepEqual(got, want) {
			var calls []string
			for _, value := range c.calls {
				calls = append(calls, fmt.Sprintf("%#v", value))
			}
			problems = append(problems, fmt.Sprintf("%s called with %s sent %v, want %v", label, strings.Join(calls, " then "), got, want))
		}
	}
	return problems
}

// contractQueryEncoding is how the schema sends the list: an array in form
// style without explode is one comma-separated value.
func contractQueryEncoding(param contractQueryParam, sent []string) []string {
	if param.Schema != nil && param.Schema.Type == "array" && param.Explode != nil && !*param.Explode {
		return []string{strings.Join(sent, ",")}
	}
	return slices.Clone(sent)
}
