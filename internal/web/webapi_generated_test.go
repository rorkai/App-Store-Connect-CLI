package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
)

// The tests in this file check the generated client code (the *_gen.go files
// listed in openapi_bindings/) against docs/openapi/web/latest.json, which
// they read independently of the generator.

type generatedFamily struct {
	Tag        string              `json:"tag"`
	Operations []*generatedBinding `json:"operations"`
}

type generatedBinding struct {
	Operation  string   `json:"operation"`
	Name       string   `json:"name"`
	Params     []string `json:"params"`
	Deviations []struct {
		Problem string `json:"problem"`
		Reason  string `json:"reason"`
	} `json:"deviations"`
}

func loadGeneratedFamilies(t *testing.T) []*generatedFamily {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("openapi_bindings", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no bindings in openapi_bindings")
	}
	var families []*generatedFamily
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var family generatedFamily
		if err := json.Unmarshal(data, &family); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		families = append(families, &family)
	}
	return families
}

// schemaOperationsByID indexes the schema's operations and adds the
// parameters of each path item to its operations.
func schemaOperationsByID(t *testing.T) map[string]schemaOperation {
	t.Helper()
	spec := loadWebAPISchema(t)
	paths, _ := spec["paths"].(map[string]any)
	byID := map[string]schemaOperation{}
	for _, op := range schemaOperations(t, spec) {
		id, _ := op.op["operationId"].(string)
		item, _ := paths[op.path].(map[string]any)
		shared, _ := item["parameters"].([]any)
		own, _ := op.op["parameters"].([]any)
		op.op = maps.Clone(op.op)
		op.op["parameters"] = append(slices.Clone(own), shared...)
		byID[id] = op
	}
	return byID
}

// TestGeneratedClientBindingsExistInSchema fails when the schema no longer
// has an operation the bindings generate code for, or marks it as one that
// stays hand-written, so a schema change names each generated method it
// breaks.
func TestGeneratedClientBindingsExistInSchema(t *testing.T) {
	ops := schemaOperationsByID(t)
	for _, family := range loadGeneratedFamilies(t) {
		for _, b := range family.Operations {
			op, ok := ops[b.Operation]
			if !ok {
				t.Errorf("%s: %s is bound to operation %s, which %s no longer has", family.Tag, b.Name, b.Operation, webAPISchemaPath)
				continue
			}
			for _, mark := range []string{"x-asc-hand-written", "x-asc-not-implemented"} {
				if op.op[mark] == true {
					t.Errorf("%s: %s is bound to operation %s, which is marked %s", family.Tag, b.Name, b.Operation, mark)
				}
			}
		}
	}
}

// TestGeneratedClientMatchesSchema calls every generated method with synthetic
// arguments and checks the request it sends against the operation in the
// schema: HTTP method, URL, query parameter names and body, and that the
// method returns what the operation responds with. A difference recorded in
// the bindings as a deviation must be exactly one the method still produces,
// so the records can only shrink on purpose.
func TestGeneratedClientMatchesSchema(t *testing.T) {
	t.Cleanup(DisableRequestPacingForTesting())
	ops := schemaOperationsByID(t)
	operations, deviations := 0, 0
	for _, family := range loadGeneratedFamilies(t) {
		for _, b := range family.Operations {
			operations++
			deviations += len(b.Deviations)
			op, ok := ops[b.Operation]
			if !ok {
				t.Errorf("%s (%s): operation not in %s", b.Operation, b.Name, webAPISchemaPath)
				continue
			}
			for _, problem := range unrecordedProblems(family.Tag, b, checkGeneratedOperation(op, b)) {
				t.Errorf("%s (%s), method %s: %s", b.Operation, op, b.Name, problem)
			}
		}
	}
	if operations == 0 {
		t.Fatal("no operations in the bindings")
	}
	t.Logf("%d generated operations checked, %d recorded deviations", operations, deviations)
}

// unrecordedProblems returns the problems the bindings do not record as
// deviations, and a problem for each recorded deviation that has no reason or
// that the method no longer produces.
func unrecordedProblems(tag string, b *generatedBinding, problems []string) []string {
	recorded := map[string]bool{}
	var out []string
	for _, d := range b.Deviations {
		switch {
		case strings.TrimSpace(d.Reason) == "":
			out = append(out, fmt.Sprintf("recorded deviation %q in openapi_bindings/%s.json has no reason", d.Problem, tag))
		case recorded[d.Problem]:
			out = append(out, fmt.Sprintf("deviation %q is recorded twice in openapi_bindings/%s.json", d.Problem, tag))
		case !slices.Contains(problems, d.Problem):
			out = append(out, fmt.Sprintf("recorded deviation %q no longer happens; remove it from openapi_bindings/%s.json", d.Problem, tag))
		}
		recorded[d.Problem] = true
	}
	for _, problem := range problems {
		if !recorded[problem] {
			out = append(out, problem)
		}
	}
	return out
}

type generatedRequest struct {
	method string
	url    string
	query  map[string][]string
	body   []byte
}

// checkGeneratedOperation returns every difference between what the method
// sends and what the schema defines for the operation. It checks operations
// without a request or response body, the only ones the web target
// generates; any other operation fails until this test checks its bodies.
func checkGeneratedOperation(op schemaOperation, b *generatedBinding) []string {
	method := reflect.ValueOf(&Client{}).MethodByName(b.Name)
	if !method.IsValid() {
		return []string{"no such method on *Client"}
	}
	if op.op["requestBody"] != nil || hasResponseBody(op) {
		return []string{"the operation has a request or response body, which this test does not check yet"}
	}
	typ := method.Type()
	if typ.NumIn() == 0 || typ.In(0) != reflect.TypeFor[context.Context]() {
		return []string{"first parameter is not a context.Context"}
	}
	args := []reflect.Value{reflect.ValueOf(context.Background())}
	var pathArgs []string
	for i := 1; i < typ.NumIn(); i++ {
		in := typ.In(i)
		if in.Kind() != reflect.String {
			return []string{fmt.Sprintf("cannot be called with synthetic arguments: parameter %d has type %s", i, in)}
		}
		value := fmt.Sprintf("path-%d", len(pathArgs)+1)
		pathArgs = append(pathArgs, value)
		args = append(args, reflect.ValueOf(value).Convert(in))
	}
	if len(pathArgs) != len(b.Params) {
		return []string{fmt.Sprintf("%d string parameters, bindings name %d path parameters", len(pathArgs), len(b.Params))}
	}

	var requests []generatedRequest
	client := NewClient(&AuthSession{Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var body []byte
		if req.Body != nil {
			body, _ = io.ReadAll(req.Body)
		}
		u := *req.URL
		u.RawQuery = ""
		requests = append(requests, generatedRequest{method: req.Method, url: u.String(), query: req.URL.Query(), body: body})
		return &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
	})}})
	out := reflect.ValueOf(client).MethodByName(b.Name).Call(args)
	callErr, _ := out[len(out)-1].Interface().(error)

	var problems []string
	if len(out) != 1 || typ.Out(0) != reflect.TypeFor[error]() {
		problems = append(problems, fmt.Sprintf("returns %s for an operation without a response body", typ))
	}
	if len(requests) != 1 {
		return append(problems, fmt.Sprintf("sent %d requests, want one (error: %v)", len(requests), callErr))
	}
	req := requests[0]
	if callErr != nil {
		problems = append(problems, fmt.Sprintf("failed on an empty success response: %v", callErr))
	}
	if want := strings.ToUpper(op.method); req.method != want {
		problems = append(problems, fmt.Sprintf("sent %s, the operation is %s", req.method, want))
	}
	if want := op.server + generatedPath(op.path, pathArgs); req.url != want {
		problems = append(problems, fmt.Sprintf("sent URL %s, want %s", req.url, want))
	}
	problems = append(problems, checkGeneratedQuery(op, req.query)...)
	if len(req.body) > 0 {
		problems = append(problems, fmt.Sprintf("sent a body %s, the operation has none", req.body))
	}
	return problems
}

func hasResponseBody(op schemaOperation) bool {
	responses, _ := op.op["responses"].(map[string]any)
	for status, raw := range responses {
		response, _ := raw.(map[string]any)
		if strings.HasPrefix(status, "2") && response["content"] != nil {
			return true
		}
	}
	return false
}

func generatedPath(template string, args []string) string {
	for _, arg := range args {
		start, end := strings.Index(template, "{"), strings.Index(template, "}")
		if start < 0 || end < start {
			break
		}
		template = template[:start] + arg + template[end+1:]
	}
	return template
}

// checkGeneratedQuery checks that a request sends only query parameters the
// operation defines and every one it requires.
func checkGeneratedQuery(op schemaOperation, query map[string][]string) []string {
	defined := map[string]bool{}
	var required []string
	params, _ := op.op["parameters"].([]any)
	for _, raw := range params {
		param, _ := raw.(map[string]any)
		name, _ := param["name"].(string)
		if param["in"] != "query" {
			continue
		}
		defined[name] = true
		if param["required"] == true {
			required = append(required, name)
		}
	}
	var unknown, missing []string
	for name := range query {
		if !defined[name] {
			unknown = append(unknown, name)
		}
	}
	for _, name := range required {
		if _, ok := query[name]; !ok {
			missing = append(missing, name)
		}
	}
	sort.Strings(unknown)
	sort.Strings(missing)
	var problems []string
	if len(unknown) > 0 {
		problems = append(problems, fmt.Sprintf("sent query parameters %v that the operation does not define", unknown))
	}
	if len(missing) > 0 {
		problems = append(problems, fmt.Sprintf("did not send query parameters %v that the operation requires", missing))
	}
	return problems
}
