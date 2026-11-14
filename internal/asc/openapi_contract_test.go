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

const contractSpecPath = "../../docs/openapi/latest.json"

type contractFamily struct {
	Tag        string               `json:"tag"`
	Types      []string             `json:"types"`
	Operations []*contractOpBinding `json:"operations"`
}

type contractOpBinding struct {
	Operation string   `json:"operation"`
	Name      string   `json:"name"`
	Params    []string `json:"params"`
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

// response is the success status of the operation and a body of the shape
// its schema defines: a document whose data is a list or an object.
func (op *contractOperation) response(spec *contractSpec) (int, string) {
	for _, status := range []int{http.StatusOK, http.StatusCreated} {
		if schema := op.Responses[strconv.Itoa(status)].schema(spec); schema != nil {
			if data := spec.property(schema, "data"); data != nil && data.Type == "array" {
				return status, `{"data":[]}`
			}
			return status, `{"data":{}}`
		}
	}
	return http.StatusNoContent, ""
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
		for method, rawOp := range item {
			if method == "parameters" {
				continue
			}
			var op contractOperation
			if err := json.Unmarshal(rawOp, &op); err != nil {
				return nil, fmt.Errorf("%s %s: %w", method, path, err)
			}
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
// snapshot: HTTP method, path, query parameter names and body shape.
func TestGeneratedClientMatchesSchema(t *testing.T) {
	spec := mustLoadContractSpec(t)
	operations := 0
	for _, family := range loadContractFamilies(t) {
		for _, b := range family.Operations {
			operations++
			op, ok := spec.operations[b.Operation]
			if !ok {
				t.Errorf("%s (%s): operation not in %s", b.Operation, b.Name, contractSpecPath)
				continue
			}
			for _, problem := range checkGeneratedOperation(spec, op, b) {
				t.Errorf("%s (%s %s), method %s: %s", b.Operation, op.method, op.path, b.Name, problem)
			}
		}
	}
	if operations == 0 {
		t.Fatal("no operations in the bindings")
	}
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
		if opt.Kind() != reflect.Func || opt.NumIn() != 1 || opt.In(0).Kind() != reflect.Pointer || !opt.In(0).Implements(reflect.TypeFor[pager]()) {
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
func (c *contractCall) invoke(args []reflect.Value, status int, response string) ([]contractRequest, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
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
			return jsonResponse(status, response), nil
		})},
		keyID:      "KEY123",
		issuerID:   "ISS456",
		privateKey: key,
	}
	out := reflect.ValueOf(client).MethodByName(c.name).Call(append([]reflect.Value{reflect.ValueOf(context.Background())}, args...))
	callErr, _ := out[len(out)-1].Interface().(error)
	return requests, callErr
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
			page := q.Interface().(pager).page()
			page.limit = contractLimit
			page.nextURL = ""
		}))
	}
	status, response := op.response(spec)
	requests, callErr := call.invoke(args, status, response)
	if fillErr != nil {
		return []string{"cannot fill the options: " + fillErr.Error()}
	}
	if len(requests) != 1 {
		return []string{fmt.Sprintf("sent %v, want one request (error: %v)", requests, callErr)}
	}
	req := requests[0]

	var problems []string
	if callErr != nil {
		problems = append(problems, fmt.Sprintf("failed on a %d response of the schema's shape %s: %v", status, response, callErr))
	}
	if req.method != op.method {
		problems = append(problems, fmt.Sprintf("sent %s, the operation is %s", req.method, op.method))
	}
	if want := contractPath(op.path, call.pathArgs); req.url.Path != want {
		problems = append(problems, fmt.Sprintf("sent path %s, want %s", req.url.Path, want))
	}
	problems = append(problems, checkContractQuery(op, req.url.Query(), call.query != nil)...)
	if call.query != nil {
		problems = append(problems, checkContractNextURL(call, op.path, status, response)...)
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
		if !allowed[name] {
			unknown = append(unknown, name)
		}
	}
	sort.Strings(unknown)
	var problems []string
	if len(unknown) > 0 {
		problems = append(problems, fmt.Sprintf("sent query parameters %v that the operation does not define", unknown))
	}
	if paged && allowed["limit"] && query.Get("limit") != strconv.Itoa(contractLimit) {
		problems = append(problems, fmt.Sprintf("sent limit=%q with a limit option of %d", query.Get("limit"), contractLimit))
	}
	return problems
}

// checkContractNextURL calls a paged read with a next URL and blank path
// parameters, as pagination does, and checks that it fetches that URL.
func checkContractNextURL(call *contractCall, path string, status int, response string) []string {
	next := "https://api.appstoreconnect.apple.com" + contractPath(path, call.pathArgs) + "?cursor=contract"
	var args []reflect.Value
	for _, typ := range call.params {
		args = append(args, reflect.Zero(typ))
	}
	args = append(args, call.option(func(q reflect.Value) {
		q.Interface().(pager).page().nextURL = next
	}))
	requests, err := call.invoke(args, status, response)
	if err != nil || len(requests) != 1 || requests[0].url.String() != next {
		return []string{fmt.Sprintf("with a next URL and blank path parameters sent %v, want one request to %s (error: %v)", requests, next, err)}
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
	case reflect.Float32, reflect.Float64:
		v.SetFloat(1)
	case reflect.Pointer:
		v.Set(reflect.New(v.Type().Elem()))
		return fillContractValue(v.Elem(), where)
	case reflect.Slice:
		v.Set(reflect.MakeSlice(v.Type(), 1, 1))
		return fillContractValue(v.Index(0), where+"[0]")
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
