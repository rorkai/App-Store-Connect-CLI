package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// Report prints the hand-written parts of the client package in dir that the
// spec does not back: *Client methods that send a request whose HTTP method
// and path are no spec operation, and resource attribute structs that lack a
// spec attribute and neither keep unmodeled attributes through apiAttributes
// nor encode themselves.
// It is a list to read, not a check.
func Report(spec *Spec, dir string, w io.Writer) error {
	src, err := loadReportSource(dir)
	if err != nil {
		return err
	}
	var lines []string
	for _, f := range src.funcs {
		if f.generated {
			continue
		}
		for _, t := range f.templates {
			if len(t.verbs) == 0 {
				// A helper that takes the HTTP method as an argument is
				// resolved with the methods its callers pass.
				for _, caller := range src.callers[f.name] {
					withVerbs := template{verbs: src.verbsOf(caller), path: t.path}
					if !caller.generated && len(withVerbs.verbs) > 0 && !spec.matchesTemplate(withVerbs, caller.literalsWith(f), false) {
						lines = append(lines, fmt.Sprintf("method\t%s\t%s %s (through %s)\tno spec operation", caller.name, strings.Join(withVerbs.verbs, ","), t.path, f.name))
					}
				}
				continue
			}
			if spec.matchesTemplate(t, f.literals, false) {
				continue
			}
			if spec.matchesTemplate(t, nil, true) && len(src.callers[f.name]) > 0 {
				// A helper that takes a path segment as an argument is
				// resolved through the literals its callers pass.
				for _, caller := range src.callers[f.name] {
					if !caller.generated && !spec.matchesTemplate(t, caller.literalsWith(f), false) {
						lines = append(lines, fmt.Sprintf("method\t%s\t%s %s (through %s)\tno spec operation", caller.name, strings.Join(t.verbs, ","), t.path, f.name))
					}
				}
				continue
			}
			lines = append(lines, fmt.Sprintf("method\t%s\t%s %s\tno spec operation", f.name, strings.Join(t.verbs, ","), t.path))
		}
	}
	for _, name := range src.attributeTypes() {
		for _, resource := range src.resourcesOf(spec, name) {
			if missing := src.missingAttributes(spec, name, resource); len(missing) > 0 {
				lines = append(lines, fmt.Sprintf("type\t%s\t%s\tlacks %s", name, resource, strings.Join(missing, ", ")))
			}
		}
	}
	sort.Strings(lines)
	lines = dedupe(lines)
	for _, line := range lines {
		fmt.Fprintln(w, line)
	}
	fmt.Fprintf(w, "%d hand-written methods or types without spec backing\n", len(lines))
	return nil
}

// template is one request a function builds from a path literal.
type template struct {
	verbs []string
	path  string // fmt format such as /v1/apps/%s
}

type reportFunc struct {
	name      string
	method    bool // a *Client method rather than a package function
	generated bool
	literals  map[string]bool
	verbs     []string
	paths     []string
	callees   []string
	templates []template
}

// literalsWith is the string literals of f and of a callee it passes them to.
func (f *reportFunc) literalsWith(callee *reportFunc) map[string]bool {
	out := map[string]bool{}
	for _, m := range []map[string]bool{f.literals, callee.literals} {
		for k := range m {
			out[k] = true
		}
	}
	return out
}

type reportSource struct {
	funcs   map[string]*reportFunc
	callers map[string][]*reportFunc
	types   map[string]*ast.TypeSpec
	aliases map[string]ast.Expr // Go alias name to its type, such as Response[XAttributes]
	hand    map[string]bool     // types declared outside generated files
	custom  map[string]bool     // types with their own MarshalJSON or UnmarshalJSON
}

func loadReportSource(dir string) (*reportSource, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil, err
	}
	src := &reportSource{funcs: map[string]*reportFunc{}, callers: map[string][]*reportFunc{}, types: map[string]*ast.TypeSpec{}, aliases: map[string]ast.Expr{}, hand: map[string]bool{}, custom: map[string]bool{}}
	var bodies []*ast.FuncDecl
	fset := token.NewFileSet()
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		generated := bytes.HasPrefix(data, []byte(header))
		file, err := parser.ParseFile(fset, path, data, 0)
		if err != nil {
			return nil, err
		}
		for _, d := range file.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if d.Recv != nil && len(d.Recv.List) == 1 && (d.Name.Name == "MarshalJSON" || d.Name.Name == "UnmarshalJSON") {
					src.custom[strings.TrimPrefix(exprString(d.Recv.List[0].Type), "*")] = true
				}
				method := d.Recv != nil && len(d.Recv.List) == 1 && exprString(d.Recv.List[0].Type) == "*Client"
				if d.Recv != nil && !method || d.Body == nil {
					continue
				}
				src.funcs[d.Name.Name] = &reportFunc{name: d.Name.Name, method: method, generated: generated}
				bodies = append(bodies, d)
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					src.types[ts.Name.Name] = ts
					if ts.Assign.IsValid() {
						src.aliases[ts.Name.Name] = ts.Type
					}
					if !generated {
						src.hand[ts.Name.Name] = true
					}
				}
			}
		}
	}
	for _, d := range bodies {
		src.index(src.funcs[d.Name.Name], d.Body)
	}
	for _, f := range src.funcs {
		f.templates = src.templatesOf(f)
	}
	return src, nil
}

func (src *reportSource) index(f *reportFunc, body *ast.BlockStmt) {
	f.literals = map[string]bool{}
	seen := map[string]bool{}
	ast.Inspect(body, func(x ast.Node) bool {
		switch x := x.(type) {
		case *ast.BinaryExpr:
			if path, ok := concatPath(x); ok {
				f.paths = append(f.paths, path)
				ast.Inspect(x, func(y ast.Node) bool {
					if lit, ok := y.(*ast.BasicLit); ok {
						f.literals[stringLit(lit)] = true
					}
					return true
				})
				return false
			}
		case *ast.BasicLit:
			v := stringLit(x)
			f.literals[v] = true
			// A literal that ends in "/" is a prefix to parse or
			// concatenate onto, not a request path by itself.
			if strings.HasPrefix(v, "/v") && !strings.HasSuffix(v, "/") {
				path, _, _ := strings.Cut(v, "?")
				f.paths = append(f.paths, path)
			} else if httpMethods[v] {
				f.verbs = append(f.verbs, v)
			}
		case *ast.SelectorExpr:
			if v, ok := strings.CutPrefix(exprString(x), "http.Method"); ok {
				f.verbs = append(f.verbs, strings.ToUpper(v))
			}
		case *ast.Ident:
			if x.Name == "getJSON" {
				f.verbs = append(f.verbs, "GET")
			}
		case *ast.CallExpr:
			name := ""
			switch fun := x.Fun.(type) {
			case *ast.SelectorExpr:
				if _, ok := fun.X.(*ast.Ident); ok {
					name = fun.Sel.Name
				}
			case *ast.Ident:
				name = fun.Name
			case *ast.IndexExpr:
				if id, ok := fun.X.(*ast.Ident); ok {
					name = id.Name
				}
			}
			if callee := src.funcs[name]; callee != nil && callee != f && !seen[name] {
				seen[name] = true
				f.callees = append(f.callees, name)
				src.callers[name] = append(src.callers[name], f)
			}
		}
		return true
	})
	f.verbs = dedupe(sortedCopy(f.verbs))
}

// concatPath renders a path built with +, such as "/v1/apps/" + id, as a
// format with %s for each part that is not a literal.
func concatPath(x *ast.BinaryExpr) (string, bool) {
	if x.Op != token.ADD {
		return "", false
	}
	var b strings.Builder
	var walk func(e ast.Expr)
	walk = func(e ast.Expr) {
		if bin, ok := e.(*ast.BinaryExpr); ok && bin.Op == token.ADD {
			walk(bin.X)
			walk(bin.Y)
			return
		}
		if lit, ok := e.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			b.WriteString(stringLit(lit))
			return
		}
		b.WriteString("%s")
	}
	walk(x)
	path, _, _ := strings.Cut(b.String(), "?")
	return path, strings.HasPrefix(path, "/v")
}

// templatesOf pairs the path literals of f with the HTTP methods it sends. A
// template without methods, such as one in a helper that takes the method as
// an argument or only returns the path, is left to its callers.
func (src *reportSource) templatesOf(f *reportFunc) []template {
	var out []template
	verbs := src.verbsOf(f)
	for _, p := range dedupe(sortedCopy(f.paths)) {
		out = append(out, template{verbs: verbs, path: p})
	}
	return out
}

// verbsOf is the HTTP methods in f, or else those of the helpers it calls
// directly, such as getJSON.
func (src *reportSource) verbsOf(f *reportFunc) []string {
	if len(f.verbs) > 0 {
		return f.verbs
	}
	var verbs []string
	for _, name := range f.callees {
		verbs = append(verbs, src.funcs[name].verbs...)
	}
	return dedupe(sortedCopy(verbs))
}

// matchesTemplate reports whether some operation has one of the template's
// HTTP methods and its path. A %s matches a path parameter; where the spec
// path has a fixed segment, it matches a literal in literals, or any segment
// with wildcard.
func (s *Spec) matchesTemplate(t template, literals map[string]bool, wildcard bool) bool {
	segments := strings.Split(t.path, "/")
	for _, op := range s.ops {
		found := false
		for _, v := range t.verbs {
			found = found || v == op.Method
		}
		if !found {
			continue
		}
		specSegments := strings.Split(op.Path, "/")
		if len(specSegments) != len(segments) {
			continue
		}
		ok := true
		for i, seg := range segments {
			spec := specSegments[i]
			switch {
			case seg == spec:
			case seg == "%s" && strings.HasPrefix(spec, "{"):
			case seg == "%s" && (wildcard || literals[spec]):
			default:
				ok = false
			}
			if !ok {
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// attributeTypes lists the hand-written struct types named <X>Attributes.
func (src *reportSource) attributeTypes() []string {
	var out []string
	for name, ts := range src.types {
		if _, ok := ts.Type.(*ast.StructType); ok && src.hand[name] && strings.HasSuffix(name, "Attributes") {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// resourcesOf names the spec resources whose attributes object a Go type
// decodes: the resource of the same name, and the resource of every response
// schema whose Go alias is Response or SingleResponse of the type.
func (src *reportSource) resourcesOf(spec *Spec, name string) []string {
	var out []string
	base := strings.TrimSuffix(name, "Attributes")
	if sch, ok := spec.Schema(base); ok && sch.Properties.Get("attributes") != nil {
		out = append(out, base)
	}
	for alias, expr := range src.aliases {
		idx, ok := expr.(*ast.IndexExpr)
		if !ok || strings.TrimPrefix(exprString(idx.Index), "*") != name {
			continue
		}
		if generic := exprString(idx.X); generic != "Response" && generic != "SingleResponse" {
			continue
		}
		if resource, _, ok := spec.resourceOf(alias); ok {
			if sch, ok := spec.Schema(resource); ok && sch.Properties.Get("attributes") != nil {
				out = append(out, resource)
			}
		}
	}
	return dedupe(sortedCopy(out))
}

// missingAttributes lists the attributes of resource that the struct has no
// field for. A struct with an apiAttributes field keeps them all, and one
// that encodes itself is left to its own tests.
func (src *reportSource) missingAttributes(spec *Spec, name, resource string) []string {
	have, passThrough := src.jsonNames(name, map[string]bool{})
	if passThrough || src.custom[name] {
		return nil
	}
	sch, _ := spec.Schema(resource)
	var missing []string
	for _, prop := range sch.Properties.Get("attributes").Properties {
		if !have[prop.Name] {
			missing = append(missing, prop.Name)
		}
	}
	return missing
}

func (src *reportSource) jsonNames(name string, visiting map[string]bool) (map[string]bool, bool) {
	names := map[string]bool{}
	ts := src.types[name]
	if ts == nil || visiting[name] {
		return names, false
	}
	visiting[name] = true
	st, ok := ts.Type.(*ast.StructType)
	if !ok {
		return names, false
	}
	for _, field := range st.Fields.List {
		typ := strings.TrimPrefix(exprString(field.Type), "*")
		if typ == "apiAttributes" {
			return names, true
		}
		tag := ""
		if field.Tag != nil {
			unquoted, _ := strconv.Unquote(field.Tag.Value)
			tag, _, _ = strings.Cut(reflect.StructTag(unquoted).Get("json"), ",")
		}
		if len(field.Names) == 0 && tag == "" {
			embedded, passThrough := src.jsonNames(typ, visiting)
			if passThrough {
				return names, true
			}
			for k := range embedded {
				names[k] = true
			}
			continue
		}
		for _, n := range field.Names {
			switch {
			case tag == "-":
			case tag != "":
				names[tag] = true
			default:
				names[n.Name] = true
			}
		}
	}
	return names, false
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func dedupe(sorted []string) []string {
	var out []string
	for i, s := range sorted {
		if i == 0 || s != sorted[i-1] {
			out = append(out, s)
		}
	}
	return out
}
