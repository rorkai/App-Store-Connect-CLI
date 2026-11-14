package web

import (
	"encoding/json"
	"go/ast"
	"os"
	"sort"
	"strings"
	"testing"
)

const webAPISchemaPath = "../../docs/openapi/web/latest.json"

var schemaMethods = []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}

type schemaOperation struct {
	method string
	path   string
	server string
	op     map[string]any
}

func (o schemaOperation) String() string {
	return strings.ToUpper(o.method) + " " + o.server + o.path
}

func loadWebAPISchema(t *testing.T) map[string]any {
	t.Helper()
	data, err := os.ReadFile(webAPISchemaPath)
	if err != nil {
		t.Fatalf("read %s: %v", webAPISchemaPath, err)
	}
	var spec map[string]any
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatalf("parse %s: %v", webAPISchemaPath, err)
	}
	return spec
}

func schemaOperations(t *testing.T, spec map[string]any) []schemaOperation {
	t.Helper()
	defaultServer := serverURL(spec["servers"])
	if defaultServer == "" {
		t.Fatal("schema has no top-level server")
	}
	paths, _ := spec["paths"].(map[string]any)
	var ops []schemaOperation
	for path, rawItem := range paths {
		item, _ := rawItem.(map[string]any)
		server := defaultServer
		if s := serverURL(item["servers"]); s != "" {
			server = s
		}
		for _, method := range schemaMethods {
			if op, ok := item[method].(map[string]any); ok {
				ops = append(ops, schemaOperation{method: method, path: path, server: server, op: op})
			}
		}
	}
	sort.Slice(ops, func(i, j int) bool { return ops[i].String() < ops[j].String() })
	return ops
}

func serverURL(raw any) string {
	servers, _ := raw.([]any)
	if len(servers) == 0 {
		return ""
	}
	server, _ := servers[0].(map[string]any)
	value, _ := server["url"].(string)
	return strings.TrimRight(value, "/")
}

func normalizeTemplate(value string) string {
	return strings.TrimRight(placeholderPattern.ReplaceAllString(value, "{}"), "/")
}

func TestWebAPISchemaIsValid(t *testing.T) {
	spec := loadWebAPISchema(t)
	if spec["openapi"] != "3.0.1" {
		t.Errorf("openapi = %v, want 3.0.1 like docs/openapi/latest.json", spec["openapi"])
	}

	var refs []string
	collectRefs(spec, &refs)
	for _, ref := range refs {
		if !resolvesJSONPointer(spec, ref) {
			t.Errorf("$ref %q does not resolve", ref)
		}
	}

	seenIDs := map[string]string{}
	paths, _ := spec["paths"].(map[string]any)
	for _, op := range schemaOperations(t, spec) {
		id, _ := op.op["operationId"].(string)
		if id == "" {
			t.Errorf("%s has no operationId", op)
		} else if previous, ok := seenIDs[id]; ok {
			t.Errorf("operationId %q is used by %s and %s", id, previous, op)
		}
		seenIDs[id] = op.String()

		declared := map[string]bool{}
		item, _ := paths[op.path].(map[string]any)
		for _, params := range []any{item["parameters"], op.op["parameters"]} {
			list, _ := params.([]any)
			for _, raw := range list {
				param, _ := raw.(map[string]any)
				if param["in"] == "path" {
					name, _ := param["name"].(string)
					declared[name] = true
				}
			}
		}
		for _, match := range placeholderPattern.FindAllString(op.path, -1) {
			name := strings.Trim(match, "{}")
			if !declared[name] {
				t.Errorf("%s has no path parameter %q", op, name)
			}
			delete(declared, name)
		}
		for name := range declared {
			t.Errorf("%s declares path parameter %q that is not in the path", op, name)
		}
	}
}

func collectRefs(node any, refs *[]string) {
	switch value := node.(type) {
	case map[string]any:
		for key, child := range value {
			if ref, ok := child.(string); ok && key == "$ref" {
				*refs = append(*refs, ref)
				continue
			}
			collectRefs(child, refs)
		}
	case []any:
		for _, child := range value {
			collectRefs(child, refs)
		}
	}
}

func resolvesJSONPointer(root any, ref string) bool {
	pointer, ok := strings.CutPrefix(ref, "#/")
	if !ok {
		return false
	}
	node := root
	for _, token := range strings.Split(pointer, "/") {
		token = strings.ReplaceAll(strings.ReplaceAll(token, "~1", "/"), "~0", "~")
		object, ok := node.(map[string]any)
		if !ok {
			return false
		}
		if node, ok = object[token]; !ok {
			return false
		}
	}
	return true
}

// TestWebAPISchemaCoversClientRequests keeps docs/openapi/web/latest.json in
// step with internal/web: every request the client can send must have an
// operation, and every operation must be sent by a client method unless it is
// marked x-asc-not-implemented.
func TestWebAPISchemaCoversClientRequests(t *testing.T) {
	src := collectClientRequests(t)
	spec := loadWebAPISchema(t)
	ops := schemaOperations(t, spec)
	assertClientCallsAreFollowed(t, src)

	byURL := map[string]int{}
	byPath := map[string][]int{}
	for i, op := range ops {
		byURL[op.method+" "+normalizeTemplate(op.server+op.path)] = i
		key := op.method + " " + normalizeTemplate(op.path)
		byPath[key] = append(byPath[key], i)
	}
	lookup := func(method string, alternatives []string) []int {
		method = strings.ToLower(method)
		var found []int
		for _, alternative := range alternatives {
			if path, ok := strings.CutPrefix(alternative, anyOrigin); ok {
				found = append(found, byPath[method+" "+normalizeTemplate(path)]...)
			} else if i, ok := byURL[method+" "+normalizeTemplate(alternative)]; ok {
				found = append(found, i)
			}
		}
		return found
	}

	reported := map[string]bool{}
	missing := func(root, methods, urls string) {
		message := root + " can send " + methods + " " + urls + ", but " + webAPISchemaPath + " has no such operation; add it"
		if !reported[message] {
			reported[message] = true
			t.Error(message)
		}
	}
	sources := make([]map[string]bool, len(ops))
	for i := range sources {
		sources[i] = map[string]bool{}
	}
	for _, group := range src.groups {
		methodsCovered := map[string]bool{}
		urlsCovered := make([]bool, len(group.URLs))
		for _, method := range group.Methods {
			for u, alternatives := range group.URLs {
				for _, i := range lookup(method, alternatives) {
					methodsCovered[method] = true
					urlsCovered[u] = true
					sources[i][group.Root] = true
				}
			}
		}
		if len(methodsCovered) == 0 {
			missing(group.Root, strings.Join(group.Methods, " or "), describeURLs(group.URLs))
			continue
		}
		for _, method := range group.Methods {
			if !methodsCovered[method] {
				missing(group.Root, method, "to one of "+describeURLs(group.URLs))
			}
		}
		for u, covered := range urlsCovered {
			if !covered {
				missing(group.Root, strings.Join(group.Methods, " or "), describeURLs(group.URLs[u:u+1]))
			}
		}
	}

	for i, op := range ops {
		notImplemented, _ := op.op["x-asc-not-implemented"].(bool)
		want := make([]string, 0, len(sources[i]))
		for root := range sources[i] {
			want = append(want, root)
		}
		sort.Strings(want)
		switch {
		case len(want) == 0 && !notImplemented:
			t.Errorf("%s lists %s, which no client method sends; remove it or mark it \"x-asc-not-implemented\": true", webAPISchemaPath, op)
			continue
		case len(want) > 0 && notImplemented:
			t.Errorf("%s marks %s as not implemented, but %s sends it", webAPISchemaPath, op, strings.Join(want, ", "))
		}
		if got := stringList(op.op["x-asc-source"]); strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s: x-asc-source = %q, want %q", op, got, want)
		}
		observed, _ := op.op["x-asc-observed"].(map[string]any)
		if _, ok := observed["code"].(string); !ok {
			t.Errorf("%s: x-asc-observed must record the date it was derived from code", op)
		}
	}

	var gotProvided []string
	entries, _ := spec["x-asc-server-provided-urls"].([]any)
	for _, raw := range entries {
		entry, _ := raw.(map[string]any)
		source, _ := entry["source"].(string)
		gotProvided = append(gotProvided, source)
	}
	sort.Strings(gotProvided)
	var wantProvided []string
	for root := range src.serverProvided {
		wantProvided = append(wantProvided, root)
	}
	sort.Strings(wantProvided)
	if strings.Join(gotProvided, ",") != strings.Join(wantProvided, ",") {
		t.Errorf("x-asc-server-provided-urls sources = %q, want %q (the methods that request a URL taken from a response)", gotProvided, wantProvided)
	}
}

func describeURLs(urls [][]string) string {
	parts := make([]string, 0, len(urls))
	for _, alternatives := range urls {
		parts = append(parts, strings.Join(alternatives, " or "))
	}
	return strings.Join(parts, ", ")
}

func stringList(raw any) []string {
	list, _ := raw.([]any)
	out := make([]string, 0, len(list))
	for _, item := range list {
		value, _ := item.(string)
		out = append(out, value)
	}
	return out
}

// assertClientCallsAreFollowed fails where the walk in webapi_requests_test.go
// would silently stop: a Client method whose name another type also uses,
// and a Client method used as a value instead of being called.
func assertClientCallsAreFollowed(t *testing.T, src *clientSource) {
	t.Helper()
	for name, fns := range src.methods {
		if len(fns) < 2 {
			continue
		}
		for _, fn := range fns {
			if receiverType(fn) == "Client" {
				t.Errorf("Client.%s shares its name with a method of another type; the request walk cannot follow calls to it", name)
			}
		}
	}
	for _, file := range src.files {
		called := map[ast.Expr]bool{}
		ast.Inspect(file, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				called[call.Fun] = true
			}
			return true
		})
		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok || called[sel] {
				return true
			}
			for _, fn := range src.methods[sel.Sel.Name] {
				if receiverType(fn) == "Client" && fn.Body != nil && !ast.IsExported(fn.Name.Name) {
					t.Errorf("Client.%s is used as a value; the request walk only follows direct calls", fn.Name.Name)
				}
			}
			return true
		})
	}
}
