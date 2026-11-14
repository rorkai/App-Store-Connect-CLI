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
		Operation  string `json:"operation"`
		Name       string `json:"name"`
		Deviations []struct {
			Problem string `json:"problem"`
			Reason  string `json:"reason"`
		} `json:"deviations"`
	} `json:"queries"`
}

// TestGeneratedQueriesMatchSchema calls every generated query builder with
// each of its With options set to a synthetic list and checks that the
// option sends exactly the parameter the schema defines for it, with the
// list trimmed, without blank items and encoded as the schema says; that
// limit is sent for an operation that pages; and that
// every query parameter of the operation has an option. Differences recorded
// in the bindings as deviations are excused exactly as for methods.
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
			queries++
			deviations += len(b.Deviations)
			bound[b.Operation] = true
			opParams, ok := params[b.Operation]
			if !ok {
				t.Errorf("query %s (%s): operation not in %s", b.Name, b.Operation, contractSpecPath)
				continue
			}
			binding := &contractOpBinding{Operation: b.Operation, Name: b.Name}
			for _, d := range b.Deviations {
				binding.Deviations = append(binding.Deviations, struct {
					Problem string `json:"problem"`
					Reason  string `json:"reason"`
				}{d.Problem, d.Reason})
			}
			for _, problem := range unrecordedContractProblems(family.Tag, binding, checkGeneratedQuery(b.Operation, opParams)) {
				t.Errorf("query of %s (build%sQuery): %s", b.Operation, b.Name, problem)
			}
		}
	}
	for _, op := range slices.Sorted(maps.Keys(generatedQueries)) {
		if !bound[op] {
			t.Errorf("generatedQueries has %s, which no bindings file binds", op)
		}
	}
	if queries != len(generatedQueries) {
		t.Errorf("%d queries in the bindings, %d in generatedQueries", queries, len(generatedQueries))
	}
	t.Logf("%d generated queries checked, %d recorded deviations", queries, deviations)
}

// contractQueryValues is the synthetic list every With option is called
// with; a generated option sends it as contractQuerySent.
var (
	contractQueryValues = []string{" contract-1 ", "", "contract-2"}
	contractQuerySent   = []string{"contract-1", "contract-2"}
)

// checkGeneratedQuery returns every difference between what the generated
// query of an operation sends and what the schema defines.
func checkGeneratedQuery(operation string, params map[string]contractQueryParam) []string {
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
		if pages {
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
		if typ.NumIn() != 1 || typ.In(0) != reflect.TypeFor[[]string]() || typ.NumOut() != 1 || typ.Out(0).Kind() != reflect.Func ||
			typ.Out(0).NumIn() != 1 || typ.Out(0).In(0) != queryType {
			problems = append(problems, fmt.Sprintf("%s is a %s, not a func([]string) option of %s", label, typ, queryType))
			continue
		}
		set := option.Call([]reflect.Value{reflect.ValueOf(slices.Clone(contractQueryValues))})[0]
		got := send(func(q reflect.Value) { set.Call([]reflect.Value{q}) })
		want := url.Values{}
		maps.Copy(want, base)
		want[name] = contractQueryEncoding(param)
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

// contractQueryEncoding is how the schema sends the list: an array in form
// style without explode is one comma-separated value.
func contractQueryEncoding(param contractQueryParam) []string {
	if param.Schema != nil && param.Schema.Type == "array" && param.Explode != nil && !*param.Explode {
		return []string{strings.Join(contractQuerySent, ",")}
	}
	return slices.Clone(contractQuerySent)
}
