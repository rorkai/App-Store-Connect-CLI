package asc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"unsafe"
)

// The tests in this file check the generated client code (the *_gen.go files
// listed in openapi_bindings/) against docs/openapi/latest.json, which they
// read independently of the generator.

var contractSpecPath = filepath.Join("..", "..", "docs", "openapi", "latest.json")

type contractFamily struct {
	Tag        string               `json:"tag"`
	Types      []string             `json:"types"`
	Operations []*contractOpBinding `json:"operations"`
}

type contractOpBinding struct {
	Operation  string   `json:"operation"`
	Name       string   `json:"name"`
	Params     []string `json:"params"`
	Check      string   `json:"check"`
	CheckFirst bool     `json:"checkFirst"`
	Single     bool     `json:"single"`
	Deviations []struct {
		Problem string `json:"problem"`
		Reason  string `json:"reason"`
	} `json:"deviations"`
}

type contractSchema struct {
	Ref        string                     `json:"$ref"`
	Type       string                     `json:"type"`
	Enum       []string                   `json:"enum"`
	Properties map[string]*contractSchema `json:"properties"`
	Required   []string                   `json:"required"`
	Items      *contractSchema            `json:"items"`
}

type contractOperation struct {
	ID         string `json:"operationId"`
	Parameters []struct {
		Name string `json:"name"`
		In   string `json:"in"`
	} `json:"parameters"`
	RequestBody *contractMedia            `json:"requestBody"`
	Responses   map[string]*contractMedia `json:"responses"`
	method      string
	path        string
}

type contractMedia struct {
	Content map[string]struct {
		Schema *contractSchema `json:"schema"`
	} `json:"content"`
}

func (m *contractMedia) schema(spec *contractSpec) *contractSchema {
	if m == nil {
		return nil
	}
	return spec.resolve(m.Content["application/json"].Schema)
}

const contractResourceID = "resource-1"

// contractResponse is the success response of an operation: a document with
// one resource of the schema's type that has every attribute the schema
// defines, or one data item with every property when the data is not a
// resource.
type contractResponse struct {
	status   int
	body     string
	resource *contractSchema // nil for a response without a document
}

// isResource reports whether the response's data item is a resource with an
// ID, not a row such as a metrics data point.
func (r contractResponse) isResource() bool {
	return r.resource != nil && r.resource.Properties["id"] != nil
}

func (op *contractOperation) response(spec *contractSpec) (contractResponse, error) {
	for _, status := range []int{http.StatusOK, http.StatusCreated} {
		schema := op.Responses[strconv.Itoa(status)].schema(spec)
		if schema == nil {
			continue
		}
		data := spec.property(schema, "data")
		if data == nil {
			return contractResponse{}, fmt.Errorf("response %d has no data", status)
		}
		resource := data
		if data.Type == "array" {
			resource = spec.resolve(data.Items)
		}
		response := contractResponse{status: status, resource: resource}
		item := spec.sample(resource)
		if response.isResource() {
			resourceItem := map[string]any{"id": contractResourceID}
			if types := spec.property(resource, "type"); types != nil && len(types.Enum) > 0 {
				resourceItem["type"] = types.Enum[0]
			}
			if attrs := spec.property(resource, "attributes"); attrs != nil {
				resourceItem["attributes"] = spec.sample(attrs)
			}
			item = resourceItem
		}
		var document any = map[string]any{"data": item}
		if data.Type == "array" {
			document = map[string]any{"data": []any{item}}
		}
		body, err := json.Marshal(document)
		response.body = string(body)
		return response, err
	}
	return contractResponse{status: http.StatusNoContent}, nil
}

// sample is a non-zero JSON value of the schema's type.
func (s *contractSpec) sample(schema *contractSchema) any {
	schema = s.resolve(schema)
	switch {
	case schema == nil:
		return nil
	case len(schema.Enum) > 0:
		return schema.Enum[0]
	case schema.Type == "string":
		return "value"
	case schema.Type == "integer":
		return 2
	case schema.Type == "number":
		return 2.5
	case schema.Type == "boolean":
		return true
	case schema.Type == "array":
		return []any{s.sample(schema.Items)}
	}
	if len(schema.Properties) == 0 {
		return map[string]any{"key": "value"}
	}
	object := map[string]any{}
	for name, prop := range schema.Properties {
		object[name] = s.sample(prop)
	}
	return object
}

type contractSpec struct {
	operations map[string]*contractOperation
	schemas    map[string]*contractSchema
}

var loadContractSpec = sync.OnceValues(func() (*contractSpec, error) {
	data, err := os.ReadFile(contractSpecPath)
	if err != nil {
		return nil, err
	}
	var raw struct {
		Paths      map[string]map[string]json.RawMessage `json:"paths"`
		Components struct {
			Schemas map[string]*contractSchema `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	spec := &contractSpec{operations: map[string]*contractOperation{}, schemas: raw.Components.Schemas}
	for path, item := range raw.Paths {
		var shared contractOperation
		if rawParams, ok := item["parameters"]; ok {
			if err := json.Unmarshal(rawParams, &shared.Parameters); err != nil {
				return nil, fmt.Errorf("%s parameters: %w", path, err)
			}
		}
		for method, rawOp := range item {
			if method == "parameters" {
				continue
			}
			var op contractOperation
			if err := json.Unmarshal(rawOp, &op); err != nil {
				return nil, fmt.Errorf("%s %s: %w", method, path, err)
			}
			op.Parameters = append(op.Parameters, shared.Parameters...)
			op.method = strings.ToUpper(method)
			op.path = path
			spec.operations[op.ID] = &op
		}
	}
	return spec, nil
})

func mustLoadContractSpec(t *testing.T) *contractSpec {
	t.Helper()
	spec, err := loadContractSpec()
	if err != nil {
		t.Fatal(err)
	}
	return spec
}

func (s *contractSpec) resolve(schema *contractSchema) *contractSchema {
	for schema != nil && schema.Ref != "" {
		schema = s.schemas[strings.TrimPrefix(schema.Ref, "#/components/schemas/")]
	}
	return schema
}

func (s *contractSpec) property(schema *contractSchema, name string) *contractSchema {
	schema = s.resolve(schema)
	if schema == nil {
		return nil
	}
	return s.resolve(schema.Properties[name])
}

func loadContractFamilies(t *testing.T) []*contractFamily {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("openapi_bindings", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no bindings in openapi_bindings")
	}
	var families []*contractFamily
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var family contractFamily
		if err := json.Unmarshal(data, &family); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		families = append(families, &family)
	}
	return families
}

// TestGeneratedClientBindingsExistInSchema fails when the snapshot no longer
// has an operation or schema the bindings generate code for, so a snapshot
// update names each generated method and type it breaks.
func TestGeneratedClientBindingsExistInSchema(t *testing.T) {
	spec := mustLoadContractSpec(t)
	for _, family := range loadContractFamilies(t) {
		for _, b := range family.Operations {
			if _, ok := spec.operations[b.Operation]; !ok {
				t.Errorf("%s: %s is bound to operation %s, which %s no longer has", family.Tag, b.Name, b.Operation, contractSpecPath)
			}
		}
		for _, name := range family.Types {
			if !spec.hasTypeSchema(name) {
				t.Errorf("%s: type %s has no schema in %s", family.Tag, name, contractSpecPath)
			}
		}
	}
}

// hasTypeSchema follows the generator's type names: a response or request type
// is the schema of that name, <X>Data is the data object of <X>Request, and
// <X>Attributes is the attributes object of <X>Request or of resource <X>.
func (s *contractSpec) hasTypeSchema(name string) bool {
	if strings.HasSuffix(name, "Response") || strings.HasSuffix(name, "Request") {
		return s.schemas[name] != nil
	}
	if base, ok := strings.CutSuffix(name, "Data"); ok {
		return s.property(s.schemas[base+"Request"], "data") != nil
	}
	if base, ok := strings.CutSuffix(name, "Attributes"); ok {
		if data := s.property(s.schemas[base+"Request"], "data"); data != nil {
			return s.property(data, "attributes") != nil
		}
		return s.property(s.schemas[base], "attributes") != nil
	}
	return false
}

// TestGeneratedClientMatchesSchema calls every generated method with synthetic
// arguments and checks the request it sends against the operation in the
// snapshot: HTTP method, path, query parameter names and body shape. A
// difference recorded in the bindings as a deviation must be exactly one the
// method still produces, so the records can only shrink on purpose.
func TestGeneratedClientMatchesSchema(t *testing.T) {
	spec := mustLoadContractSpec(t)
	operations, deviations := 0, 0
	for _, family := range loadContractFamilies(t) {
		for _, b := range family.Operations {
			operations++
			deviations += len(b.Deviations)
			op, ok := spec.operations[b.Operation]
			if !ok {
				t.Errorf("%s (%s): operation not in %s", b.Operation, b.Name, contractSpecPath)
				continue
			}
			for _, problem := range unrecordedContractProblems(family.Tag, b, checkGeneratedOperation(spec, op, b)) {
				t.Errorf("%s (%s %s), method %s: %s", b.Operation, op.method, op.path, b.Name, problem)
			}
		}
	}
	if operations == 0 {
		t.Fatal("no operations in the bindings")
	}
	t.Logf("%d generated operations checked, %d recorded deviations", operations, deviations)
}

// unrecordedContractProblems returns the problems the bindings do not record
// as deviations, and a problem for each recorded deviation that has no reason
// or that the method no longer produces.
func unrecordedContractProblems(tag string, b *contractOpBinding, problems []string) []string {
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

const contractLimit = 7

// contractCall is a generated method and synthetic arguments for it.
type contractCall struct {
	name     string
	params   []reflect.Type // after ctx, without the options
	args     []reflect.Value
	pathArgs []string
	ids      []string
	attrs    []byte       // JSON of the attributes argument, if any
	query    reflect.Type // the options' func type; nil without options
}

// newContractCall builds arguments from the method's parameter types: path
// parameters (string, in path order), an ID list ([]string), attributes (a
// struct with every field set) and options (one option that sets every field
// of the query).
func newContractCall(name string, typ reflect.Type, b *contractOpBinding) (*contractCall, error) {
	c := &contractCall{name: name}
	if typ.NumIn() == 0 || typ.In(0) != reflect.TypeFor[context.Context]() {
		return nil, fmt.Errorf("first parameter is not a context.Context")
	}
	last := typ.NumIn()
	if typ.IsVariadic() {
		last--
		opt := typ.In(last).Elem()
		if opt.Kind() != reflect.Func || opt.NumIn() != 1 || opt.In(0).Kind() != reflect.Pointer || (!b.Single && !opt.In(0).Implements(reflect.TypeFor[pager]())) {
			return nil, fmt.Errorf("options %s are not a func of a paged query", opt)
		}
		c.query = opt
	}
	for i := 1; i < last; i++ {
		in := typ.In(i)
		c.params = append(c.params, in)
		switch {
		case in.Kind() == reflect.String:
			value := fmt.Sprintf("path-%d", len(c.pathArgs)+1)
			c.pathArgs = append(c.pathArgs, value)
			c.args = append(c.args, reflect.ValueOf(value).Convert(in))
		case in == reflect.TypeFor[[]string]():
			c.ids = []string{"related-1", "related-2"}
			c.args = append(c.args, reflect.ValueOf(c.ids))
		case in.Kind() == reflect.Struct:
			value := reflect.New(in).Elem()
			if err := fillContractValue(value, in.Name()); err != nil {
				return nil, err
			}
			attrs, err := json.Marshal(value.Interface())
			if err != nil {
				return nil, err
			}
			c.attrs = attrs
			c.args = append(c.args, value)
		default:
			return nil, fmt.Errorf("parameter %d has type %s", i, in)
		}
	}
	if len(c.pathArgs) != len(b.Params) {
		return nil, fmt.Errorf("%d string parameters, bindings name %d path parameters", len(c.pathArgs), len(b.Params))
	}
	return c, nil
}

// option returns one option value that runs set on the query.
func (c *contractCall) option(set func(q reflect.Value)) reflect.Value {
	return reflect.MakeFunc(c.query, func(in []reflect.Value) []reflect.Value {
		set(in[0])
		return nil
	})
}

type contractRequest struct {
	method string
	url    *url.URL
	body   []byte
}

func (r contractRequest) String() string { return r.method + " " + r.url.String() }

// invoke calls the method on a client that records every request and answers
// each with the given response; args follow ctx and may end with one option.
// It returns the requests, the method's result other than its error, and the
// error.
func (c *contractCall) invoke(args []reflect.Value, response contractResponse) ([]contractRequest, any, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	var mu sync.Mutex
	var requests []contractRequest
	client := &Client{
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			var body []byte
			if req.Body != nil {
				body, _ = io.ReadAll(req.Body)
			}
			mu.Lock()
			requests = append(requests, contractRequest{method: req.Method, url: req.URL, body: body})
			mu.Unlock()
			return jsonResponse(response.status, response.body), nil
		})},
		keyID:      "KEY123",
		issuerID:   "ISS456",
		privateKey: key,
	}
	out := reflect.ValueOf(client).MethodByName(c.name).Call(append([]reflect.Value{reflect.ValueOf(context.Background())}, args...))
	callErr, _ := out[len(out)-1].Interface().(error)
	var result any
	if len(out) == 2 {
		result = out[0].Interface()
	}
	return requests, result, callErr
}

// checkGeneratedOperation returns every difference between what the method
// sends and what the schema defines for the operation.
func checkGeneratedOperation(spec *contractSpec, op *contractOperation, b *contractOpBinding) []string {
	method := reflect.ValueOf(&Client{}).MethodByName(b.Name)
	if !method.IsValid() {
		return []string{"no such method on *Client"}
	}
	call, err := newContractCall(b.Name, method.Type(), b)
	if err != nil {
		return []string{"cannot be called with synthetic arguments: " + err.Error()}
	}

	var fillErr error
	args := call.args
	if call.query != nil {
		args = append(slices.Clone(args), call.option(func(q reflect.Value) {
			fillErr = fillContractValue(q.Elem(), q.Elem().Type().Name())
			if b.Single {
				return
			}
			page := q.Interface().(pager).page()
			page.limit = contractLimit
			page.nextURL = ""
		}))
	}
	response, err := op.response(spec)
	if err != nil {
		return []string{err.Error()}
	}
	requests, result, callErr := call.invoke(args, response)
	if fillErr != nil {
		return []string{"cannot fill the options: " + fillErr.Error()}
	}
	if len(requests) != 1 {
		return []string{fmt.Sprintf("sent %v, want one request (error: %v)", requests, callErr)}
	}
	req := requests[0]

	var problems []string
	if callErr != nil {
		problems = append(problems, fmt.Sprintf("failed on the schema's %d response %s: %v", response.status, response.body, callErr))
	} else {
		problems = append(problems, checkContractResult(spec, result, response)...)
	}
	if req.method != op.method {
		problems = append(problems, fmt.Sprintf("sent %s, the operation is %s", req.method, op.method))
	}
	if want := contractPath(op.path, call.pathArgs); req.url.Path != want {
		problems = append(problems, fmt.Sprintf("sent path %s, want %s", req.url.Path, want))
	}
	problems = append(problems, checkContractQuery(op, req.url.Query(), call.query != nil && !b.Single)...)
	if b.Single && len(req.url.Query()) == 0 {
		problems = append(problems, "sent no query, the method reads one resource with a query argument")
	}
	if call.query != nil && !b.Single {
		problems = append(problems, checkContractNextURL(call, op.path, response, b.CheckFirst && b.Check == "require")...)
	}
	return append(problems, checkContractBody(spec, op, req.body, call)...)
}

func contractPath(template string, args []string) string {
	for _, arg := range args {
		start, end := strings.Index(template, "{"), strings.Index(template, "}")
		if start < 0 || end < start {
			break
		}
		template = template[:start] + arg + template[end+1:]
	}
	return template
}

func checkContractQuery(op *contractOperation, query url.Values, paged bool) []string {
	allowed := map[string]bool{}
	for _, p := range op.Parameters {
		if p.In == "query" {
			allowed[p.Name] = true
		}
	}
	var unknown []string
	for name := range query {
		if !allowed[name] && !allowsMapKeyName(allowed, name) {
			unknown = append(unknown, name)
		}
	}
	sort.Strings(unknown)
	var problems []string
	if len(unknown) > 0 {
		problems = append(problems, fmt.Sprintf("sent query parameters %v that the operation does not define", unknown))
	}
	if allowed["limit"] && !paged {
		problems = append(problems, "the operation pages with limit, the method takes no options")
	}
	if paged && allowed["limit"] && query.Get("limit") != strconv.Itoa(contractLimit) {
		problems = append(problems, fmt.Sprintf("sent limit=%q with a limit option of %d", query.Get("limit"), contractLimit))
	}
	return problems
}

// allowsMapKeyName reports whether a name built from a caller's map key, such
// as fields[<key>], is a parameter the operation defines for some key.
func allowsMapKeyName(allowed map[string]bool, name string) bool {
	prefix, suffix, ok := strings.Cut(name, contractMapKey)
	if !ok {
		return false
	}
	for param := range allowed {
		if len(param) > len(prefix)+len(suffix) && strings.HasPrefix(param, prefix) && strings.HasSuffix(param, suffix) {
			return true
		}
	}
	return false
}

// checkContractNextURL calls a paged read with a next URL and blank path
// parameters, as pagination does, and checks that it fetches that URL. A read
// that requires its path parameters before applying options gets them.
func checkContractNextURL(call *contractCall, path string, response contractResponse, requiresPathParams bool) []string {
	next := "https://api.appstoreconnect.apple.com" + contractPath(path, call.pathArgs) + "?cursor=contract"
	args, with := slices.Clone(call.args), "the path parameters"
	if !requiresPathParams {
		args, with = nil, "blank path parameters"
		for _, typ := range call.params {
			args = append(args, reflect.Zero(typ))
		}
	}
	args = append(args, call.option(func(q reflect.Value) {
		q.Interface().(pager).page().nextURL = next
	}))
	requests, _, err := call.invoke(args, response)
	if err != nil || len(requests) != 1 || requests[0].url.String() != next {
		return []string{fmt.Sprintf("with a next URL and %s sent %v, want one request to %s (error: %v)", with, requests, next, err)}
	}
	return nil
}

// checkContractResult checks that the method returns the resource of the
// response: its ID, and every attribute the schema defines and no other, so a
// method that decodes into another resource's type fails. A data item that is
// not a resource, such as a metrics row, must keep every property instead.
func checkContractResult(spec *contractSpec, result any, response contractResponse) []string {
	if response.resource == nil {
		if result != nil {
			return []string{fmt.Sprintf("returns %T for an operation without a response document", result)}
		}
		return nil
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return []string{fmt.Sprintf("cannot encode the result %T: %v", result, err)}
	}
	var document struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(encoded, &document); err != nil {
		return []string{fmt.Sprintf("result %T is not a document: %v", result, err)}
	}
	var items []map[string]json.RawMessage
	if err := json.Unmarshal(document.Data, &items); err != nil {
		var item map[string]json.RawMessage
		if err := json.Unmarshal(document.Data, &item); err != nil {
			return []string{fmt.Sprintf("result %T has data %s", result, document.Data)}
		}
		items = []map[string]json.RawMessage{item}
	}
	if len(items) != 1 {
		return []string{fmt.Sprintf("result %T does not hold the response's one data item: %s", result, encoded)}
	}
	got := items[0]
	want := response.resource
	if !response.isResource() {
		return compareContractNames(fmt.Sprintf("result %T has data item properties", result), "the schema's item has", got, want)
	}
	var id string
	_ = json.Unmarshal(got["id"], &id)
	if id != contractResourceID {
		return []string{fmt.Sprintf("result %T does not hold the response resource: %s", result, encoded)}
	}
	var attrs map[string]json.RawMessage
	_ = json.Unmarshal(got["attributes"], &attrs)
	return compareContractNames(fmt.Sprintf("result %T has attributes", result), "the schema's resource has", attrs, spec.property(want, "attributes"))
}

func compareContractNames(gotLabel, wantLabel string, values map[string]json.RawMessage, schema *contractSchema) []string {
	var want, got []string
	if schema != nil {
		for name := range schema.Properties {
			want = append(want, name)
		}
	}
	for name := range values {
		got = append(got, name)
	}
	sort.Strings(want)
	sort.Strings(got)
	if !slices.Equal(got, want) {
		return []string{fmt.Sprintf("%s %v, %s %v", gotLabel, got, wantLabel, want)}
	}
	return nil
}

// checkContractBody checks the request body against the request schema: the
// data type, the attribute and relationship names, the required ones, and for
// a relationship change the linkages of the IDs passed.
func checkContractBody(spec *contractSpec, op *contractOperation, body []byte, call *contractCall) []string {
	schema := op.RequestBody.schema(spec)
	if schema == nil {
		if len(body) > 0 {
			return []string{fmt.Sprintf("sent a body %s, the operation has none", body)}
		}
		return nil
	}
	if len(body) == 0 {
		return []string{"sent no body, the operation requires one"}
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return []string{fmt.Sprintf("body is not a JSON object: %v", err)}
	}
	problems := checkContractNames("body", payload, schema)
	dataSchema := spec.property(schema, "data")
	if dataSchema == nil || payload["data"] == nil {
		return append(problems, "body or schema has no data")
	}

	if dataSchema.Type == "array" {
		item := spec.resolve(dataSchema.Items)
		var linkages []struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		}
		if err := json.Unmarshal(payload["data"], &linkages); err != nil {
			return append(problems, fmt.Sprintf("data is not a list of linkages: %v", err))
		}
		var ids []string
		for _, l := range linkages {
			ids = append(ids, l.ID)
			if types := spec.property(item, "type"); types == nil || !slices.Contains(types.Enum, l.Type) {
				problems = append(problems, fmt.Sprintf("linkage type %q is not one the schema defines", l.Type))
			}
		}
		if !slices.Equal(ids, call.ids) {
			problems = append(problems, fmt.Sprintf("sent linkage IDs %v, passed %v", ids, call.ids))
		}
		return problems
	}

	var data map[string]json.RawMessage
	if err := json.Unmarshal(payload["data"], &data); err != nil {
		return append(problems, fmt.Sprintf("data is not an object: %v", err))
	}
	problems = append(problems, checkContractNames("data", data, dataSchema)...)
	var typ string
	_ = json.Unmarshal(data["type"], &typ)
	if types := spec.property(dataSchema, "type"); types == nil || !slices.Contains(types.Enum, typ) {
		problems = append(problems, fmt.Sprintf("data.type %q is not one of the schema's types", typ))
	}
	if spec.property(dataSchema, "id") != nil && len(call.pathArgs) > 0 {
		var id string
		_ = json.Unmarshal(data["id"], &id)
		if id != call.pathArgs[0] {
			problems = append(problems, fmt.Sprintf("data.id is %q, want the path ID %q", id, call.pathArgs[0]))
		}
	}
	for _, part := range []string{"attributes", "relationships"} {
		if data[part] == nil {
			continue
		}
		var values map[string]json.RawMessage
		if err := json.Unmarshal(data[part], &values); err != nil {
			problems = append(problems, fmt.Sprintf("data.%s is not an object: %v", part, err))
			continue
		}
		problems = append(problems, checkContractNames("data."+part, values, spec.property(dataSchema, part))...)
	}
	if call.attrs != nil && !jsonEqual(data["attributes"], call.attrs) {
		problems = append(problems, fmt.Sprintf("data.attributes is %s, the attributes argument was %s", data["attributes"], call.attrs))
	}
	return problems
}

func jsonEqual(a, b []byte) bool {
	var x, y any
	return json.Unmarshal(a, &x) == nil && json.Unmarshal(b, &y) == nil && reflect.DeepEqual(x, y)
}

// checkContractNames checks that an object sends only names the schema
// defines and every name it requires.
func checkContractNames(where string, values map[string]json.RawMessage, schema *contractSchema) []string {
	if schema == nil {
		return []string{fmt.Sprintf("sent %s, which the schema does not define", where)}
	}
	var problems []string
	var unknown []string
	for name := range values {
		if _, ok := schema.Properties[name]; !ok {
			unknown = append(unknown, name)
		}
	}
	sort.Strings(unknown)
	if len(unknown) > 0 {
		problems = append(problems, fmt.Sprintf("%s has %v, which the schema does not define", where, unknown))
	}
	for _, name := range schema.Required {
		if values[name] == nil {
			problems = append(problems, fmt.Sprintf("%s lacks %q, which the schema requires", where, name))
		}
	}
	return problems
}

// contractMapKey is the key of every map fillContractValue fills. A query
// parameter whose name holds it was named by the caller's map key.
const contractMapKey = "contract-key"

// fillContractValue sets every field of a struct, exported or not, to a
// non-zero value, so a request built from it sends everything it can.
func fillContractValue(v reflect.Value, where string) error {
	if !v.CanSet() {
		v = reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem()
	}
	switch v.Kind() {
	case reflect.String:
		v.SetString("contract")
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(1)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(1)
	case reflect.Float32, reflect.Float64:
		v.SetFloat(1)
	case reflect.Pointer:
		v.Set(reflect.New(v.Type().Elem()))
		return fillContractValue(v.Elem(), where)
	case reflect.Slice:
		v.Set(reflect.MakeSlice(v.Type(), 1, 1))
		return fillContractValue(v.Index(0), where+"[0]")
	case reflect.Array:
		for i := range v.Len() {
			if err := fillContractValue(v.Index(i), fmt.Sprintf("%s[%d]", where, i)); err != nil {
				return err
			}
		}
	case reflect.Map:
		key := reflect.New(v.Type().Key()).Elem()
		if key.Kind() == reflect.String {
			key.SetString(contractMapKey)
		} else if err := fillContractValue(key, where+" key"); err != nil {
			return err
		}
		elem := reflect.New(v.Type().Elem()).Elem()
		if err := fillContractValue(elem, where+"["+fmt.Sprint(key)+"]"); err != nil {
			return err
		}
		v.Set(reflect.MakeMapWithSize(v.Type(), 1))
		v.SetMapIndex(key, elem)
	case reflect.Func:
		fn, err := contractFunc(v.Type(), where)
		if err != nil {
			return err
		}
		v.Set(fn)
	case reflect.Struct:
		for i := range v.NumField() {
			if err := fillContractValue(v.Field(i), where+"."+v.Type().Field(i).Name); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("%s has kind %s", where, v.Kind())
	}
	return nil
}

// contractFunc returns a func of the type that returns, for each result, its
// first argument of that type, or else a filled value; so a request-context
// func passes the context through and returns a cancel func.
func contractFunc(typ reflect.Type, where string) (reflect.Value, error) {
	from := make([]int, typ.NumOut())
	filled := make([]reflect.Value, typ.NumOut())
	for i := range typ.NumOut() {
		from[i] = slices.IndexFunc(slices.Collect(typ.Ins()), func(in reflect.Type) bool { return in.AssignableTo(typ.Out(i)) })
		if from[i] >= 0 {
			continue
		}
		filled[i] = reflect.New(typ.Out(i)).Elem()
		if err := fillContractValue(filled[i], fmt.Sprintf("%s result %d", where, i)); err != nil {
			return reflect.Value{}, err
		}
	}
	return reflect.MakeFunc(typ, func(in []reflect.Value) []reflect.Value {
		out := slices.Clone(filled)
		for i, j := range from {
			if j >= 0 {
				out[i] = in[j]
			}
		}
		return out
	}), nil
}
