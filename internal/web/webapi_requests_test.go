package web

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// This file finds every request the package can send by reading its source.
// It starts at each exported function and method, follows calls into
// unexported functions of the package, and evaluates the method and URL
// arguments of each call that hands a request to net/http.

// requestSink describes a function that sends a request. Argument indexes
// refer to the call's arguments; base is -1 when the URL argument is absolute.
type requestSink struct{ method, base, url int }

// requestSinks are keyed by "Receiver.method" for methods of this package and
// by "package.Function" for other packages. Every http.NewRequest call outside
// these functions is a sink of its own.
var requestSinks = map[string]requestSink{
	"Client.doRequestBaseWithHTTPClient": {method: 3, base: 2, url: 4},
	"Client.doDeveloperPortalHTTP":       {method: 1, base: -1, url: 2},
	"appleauth.DoTwoFactorJSONRequest":   {method: 4, base: -1, url: 5},
}

// clientBaseURL stands for the Client.baseURL field, which each constructor
// sets to a different service root.
const clientBaseURL = "{baseURL}"

// anyOrigin prefixes a path whose origin the source cannot tell; it matches
// the path on any server of the schema.
const anyOrigin = "*"

var placeholderPattern = regexp.MustCompile(`\{[^}]*\}`)

const (
	maxWalkDepth    = 12
	maxEvalDepth    = 64
	maxAlternatives = 32
)

// requestGroup is one call to a sink as seen from one exported root. When the
// method or the URL is chosen at run time, the group lists every value the
// code can produce; the walk cannot tell which method goes with which URL.
type requestGroup struct {
	Root    string
	Methods []string
	// URLs holds absolute URL templates without query strings. Each entry
	// lists the alternatives for one URL: several when the URL starts at
	// Client.baseURL.
	URLs [][]string
}

type clientSource struct {
	files   []*ast.File
	imports map[string]bool
	values  map[string]ast.Expr
	funcs   map[string]*ast.FuncDecl
	methods map[string][]*ast.FuncDecl
	// baseURLs are the values Client composite literals give baseURL.
	baseURLs []string

	groups []requestGroup
	// serverProvided lists the roots that send a request to a URL taken
	// from a response, such as a signed download link.
	serverProvided map[string]bool
}

// binding is a function parameter bound to the argument expression of the
// call being followed, evaluated lazily in the caller's frame.
type binding struct {
	expr ast.Expr
	fr   *frame
}

type frame struct {
	fn       *ast.FuncDecl
	params   map[string]binding
	visiting map[string]bool
}

// collectClientRequests reads the non-test Go files of this package.
func collectClientRequests(t *testing.T) *clientSource {
	t.Helper()
	src := parseClientSource(t)
	var roots []*ast.FuncDecl
	for _, fn := range src.funcs {
		if ast.IsExported(fn.Name.Name) && fn.Body != nil {
			roots = append(roots, fn)
		}
	}
	for _, fns := range src.methods {
		for _, fn := range fns {
			if ast.IsExported(fn.Name.Name) && fn.Body != nil {
				roots = append(roots, fn)
			}
		}
	}
	sort.Slice(roots, func(i, j int) bool { return funcName(roots[i]) < funcName(roots[j]) })
	for _, fn := range roots {
		src.walk(funcName(fn), fn, &frame{fn: fn}, map[*ast.FuncDecl]bool{}, 0)
	}
	return src
}

func parseClientSource(t *testing.T) *clientSource {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package directory: %v", err)
	}
	src := &clientSource{
		imports:        map[string]bool{},
		values:         map[string]ast.Expr{},
		funcs:          map[string]*ast.FuncDecl{},
		methods:        map[string][]*ast.FuncDecl{},
		serverProvided: map[string]bool{},
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		src.files = append(src.files, file)
	}
	for _, file := range src.files {
		for _, imp := range file.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			name := path[strings.LastIndex(path, "/")+1:]
			if imp.Name != nil {
				name = imp.Name.Name
			}
			src.imports[name] = true
		}
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					if vs, ok := spec.(*ast.ValueSpec); ok && len(vs.Values) == len(vs.Names) {
						for i, n := range vs.Names {
							src.values[n.Name] = vs.Values[i]
						}
					}
				}
			case *ast.FuncDecl:
				if d.Recv == nil {
					src.funcs[d.Name.Name] = d
				} else {
					src.methods[d.Name.Name] = append(src.methods[d.Name.Name], d)
				}
			}
		}
	}
	for _, file := range src.files {
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			if ident, ok := lit.Type.(*ast.Ident); !ok || ident.Name != "Client" {
				return true
			}
			for _, elt := range lit.Elts {
				if kv, ok := elt.(*ast.KeyValueExpr); ok {
					if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "baseURL" {
						src.baseURLs = append(src.baseURLs, src.eval(kv.Value, nil, 0)...)
					}
				}
			}
			return true
		})
	}
	src.baseURLs = uniqueSorted(src.baseURLs)
	if len(src.baseURLs) == 0 {
		t.Fatal("no Client literal sets baseURL")
	}
	return src
}

func funcName(fn *ast.FuncDecl) string {
	if rt := receiverType(fn); rt != "" {
		return rt + "." + fn.Name.Name
	}
	return fn.Name.Name
}

func receiverType(fn *ast.FuncDecl) string {
	if fn == nil || fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}
	t := fn.Recv.List[0].Type
	if star, ok := t.(*ast.StarExpr); ok {
		t = star.X
	}
	if ident, ok := t.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

func receiverName(fn *ast.FuncDecl) string {
	if fn == nil || fn.Recv == nil || len(fn.Recv.List) != 1 || len(fn.Recv.List[0].Names) != 1 {
		return ""
	}
	return fn.Recv.List[0].Names[0].Name
}

// walk records the requests fn sends. Exported callees are roots of their
// own, so each request is attributed to the closest exported function.
func (s *clientSource) walk(root string, fn *ast.FuncDecl, fr *frame, stack map[*ast.FuncDecl]bool, depth int) {
	if stack[fn] || depth > maxWalkDepth {
		return
	}
	stack[fn] = true
	defer delete(stack, fn)
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "http" && strings.HasPrefix(sel.Sel.Name, "NewRequest") {
				if sel.Sel.Name == "NewRequestWithContext" {
					s.record(root, call.Args[1], nil, call.Args[2], fr, depth)
				} else {
					s.record(root, call.Args[0], nil, call.Args[1], fr, depth)
				}
				return true
			}
		}
		callee, key := s.resolveCallee(call)
		if sink, ok := requestSinks[key]; ok {
			var base ast.Expr
			if sink.base >= 0 {
				base = call.Args[sink.base]
			}
			s.record(root, call.Args[sink.method], base, call.Args[sink.url], fr, depth)
			return true
		}
		if callee == nil || callee.Body == nil || ast.IsExported(callee.Name.Name) {
			return true
		}
		s.walk(root, callee, bind(callee, call, fr), stack, depth+1)
		return true
	})
}

// resolveCallee returns the package function a call reaches, if any, and the
// key it would have in requestSinks.
func (s *clientSource) resolveCallee(call *ast.CallExpr) (*ast.FuncDecl, string) {
	fun := call.Fun
	switch generic := fun.(type) {
	case *ast.IndexExpr:
		fun = generic.X
	case *ast.IndexListExpr:
		fun = generic.X
	}
	switch fun := fun.(type) {
	case *ast.Ident:
		return s.funcs[fun.Name], fun.Name
	case *ast.SelectorExpr:
		if x, ok := fun.X.(*ast.Ident); ok && s.imports[x.Name] {
			return nil, x.Name + "." + fun.Sel.Name
		}
		if candidates := s.methods[fun.Sel.Name]; len(candidates) == 1 {
			return candidates[0], funcName(candidates[0])
		}
	}
	return nil, ""
}

func bind(callee *ast.FuncDecl, call *ast.CallExpr, fr *frame) *frame {
	child := &frame{fn: callee, params: map[string]binding{}}
	i := 0
	for _, field := range callee.Type.Params.List {
		for _, name := range field.Names {
			if i < len(call.Args) {
				child.params[name.Name] = binding{expr: call.Args[i], fr: fr}
			}
			i++
		}
	}
	return child
}

func (s *clientSource) record(root string, methodExpr, baseExpr, urlExpr ast.Expr, fr *frame, depth int) {
	group := requestGroup{Root: root, Methods: s.eval(methodExpr, fr, depth)}
	var bases []string
	if baseExpr != nil {
		bases = s.eval(baseExpr, fr, depth)
	}
	for _, target := range s.eval(urlExpr, fr, depth) {
		if rank(target) == 0 {
			s.serverProvided[root] = true
			continue
		}
		full := []string{target}
		if baseExpr != nil && !isAbsoluteURL(target) {
			full = concat(bases, full)
		}
		for _, u := range full {
			u = strings.TrimRight(stripQuery(u), "/")
			alternatives := []string{u}
			if rest, ok := strings.CutPrefix(u, clientBaseURL); ok {
				alternatives = nil
				for _, base := range s.baseURLs {
					alternatives = append(alternatives, strings.TrimRight(base, "/")+rest)
				}
			}
			if !isAbsoluteURL(alternatives[0]) {
				path, ok := pathAfterUnknownOrigin(alternatives[0])
				if !ok {
					s.serverProvided[root] = true
					continue
				}
				alternatives = []string{anyOrigin + path}
			}
			if !containsURL(group.URLs, alternatives) {
				group.URLs = append(group.URLs, alternatives)
			}
		}
	}
	if len(group.URLs) > 0 {
		s.groups = append(s.groups, group)
	}
}

func containsURL(urls [][]string, alternatives []string) bool {
	for _, u := range urls {
		if strings.Join(u, " ") == strings.Join(alternatives, " ") {
			return true
		}
	}
	return false
}

// pathAfterUnknownOrigin returns the static path of a URL whose origin the
// source cannot tell, such as one parsed from the base URL at run time.
func pathAfterUnknownOrigin(value string) (string, bool) {
	if !strings.HasPrefix(value, "{") {
		return "", false
	}
	path := value[strings.Index(value, "}")+1:]
	return path, strings.HasPrefix(path, "/") && rank(path) == 2
}

func isAbsoluteURL(value string) bool {
	return strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "http://")
}

func stripQuery(value string) string {
	if i := strings.IndexByte(value, '?'); i >= 0 {
		return value[:i]
	}
	return value
}

// rank orders the values an expression can take: 2 when the value starts
// with static text, 1 when it holds only placeholders but several path
// segments (such as "/{type}/{id}"), and 0 otherwise, such as a link taken
// from a response.
func rank(value string) int {
	trimmed := strings.TrimLeft(stripQuery(value), "/")
	if trimmed != "" && !strings.HasPrefix(trimmed, "{") {
		return 2
	}
	segments := 0
	for _, segment := range strings.Split(trimmed, "/") {
		if segment == "" {
			continue
		}
		if placeholderPattern.ReplaceAllString(segment, "") != "" {
			return 1
		}
		segments++
	}
	if segments >= 2 {
		return 1
	}
	return 0
}

// preferStatic keeps the highest-ranked alternatives: a value computed from a
// response, a fallback for an overridden base URL, or the empty value a
// variable starts with is not another endpoint.
func preferStatic(values []string) []string {
	values = uniqueSorted(values)
	best := 0
	for _, v := range values {
		best = max(best, rank(v))
	}
	var kept []string
	for _, v := range values {
		if rank(v) == best && (v != "" || len(values) == 1) {
			kept = append(kept, v)
		}
	}
	if len(kept) > maxAlternatives {
		kept = kept[:maxAlternatives]
	}
	return kept
}

func uniqueSorted(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range values {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}

func concat(left, right []string) []string {
	var out []string
	for _, l := range left {
		for _, r := range right {
			out = append(out, l+r)
		}
	}
	return out
}

func placeholder(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return "{" + e.Name + "}"
	case *ast.SelectorExpr:
		return "{" + e.Sel.Name + "}"
	case *ast.CallExpr:
		return placeholder(e.Fun)
	case *ast.IndexExpr:
		return placeholder(e.X)
	case *ast.StarExpr:
		return placeholder(e.X)
	}
	return "{value}"
}

// eval returns the string values expr can take, with each part the source
// cannot tell written as {name}.
func (s *clientSource) eval(expr ast.Expr, fr *frame, depth int) []string {
	if depth > maxEvalDepth {
		return []string{placeholder(expr)}
	}
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			if v, err := strconv.Unquote(e.Value); err == nil {
				return []string{v}
			}
		}
		return []string{e.Value}
	case *ast.ParenExpr:
		return s.eval(e.X, fr, depth+1)
	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			return preferStatic(concat(s.eval(e.X, fr, depth+1), s.eval(e.Y, fr, depth+1)))
		}
	case *ast.Ident:
		return s.evalIdent(e.Name, fr, depth)
	case *ast.SelectorExpr:
		if x, ok := e.X.(*ast.Ident); ok {
			if x.Name == "http" && strings.HasPrefix(e.Sel.Name, "Method") {
				return []string{strings.ToUpper(strings.TrimPrefix(e.Sel.Name, "Method"))}
			}
			if e.Sel.Name == "baseURL" && s.isClient(x.Name, fr) {
				return []string{clientBaseURL}
			}
			if values := s.evalField(x.Name, e.Sel.Name, fr, depth); values != nil {
				return values
			}
		}
	case *ast.CallExpr:
		return s.evalCall(e, 0, fr, depth)
	}
	return []string{placeholder(expr)}
}

func (s *clientSource) isClient(name string, fr *frame) bool {
	if fr == nil {
		return false
	}
	if b, ok := fr.params[name]; ok {
		if ident, ok := b.expr.(*ast.Ident); ok {
			return s.isClient(ident.Name, b.fr)
		}
		return false
	}
	return receiverName(fr.fn) == name && receiverType(fr.fn) == "Client"
}

// evalField resolves name.field when name is a local variable assigned a
// composite literal that sets field.
func (s *clientSource) evalField(name, field string, fr *frame, depth int) []string {
	if fr == nil || fr.fn == nil {
		return nil
	}
	var values []string
	for _, rhs := range assignments(name, fr.fn.Body) {
		if u, ok := rhs.(*ast.UnaryExpr); ok && u.Op == token.AND {
			rhs = u.X
		}
		lit, ok := rhs.(*ast.CompositeLit)
		if !ok {
			continue
		}
		for _, elt := range lit.Elts {
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				if key, ok := kv.Key.(*ast.Ident); ok && key.Name == field {
					values = append(values, s.eval(kv.Value, fr, depth+1)...)
				}
			}
		}
	}
	if len(values) == 0 {
		return nil
	}
	return preferStatic(values)
}

// assignments returns the expressions assigned to name in body. For a
// multi-value assignment from one call, the entry is an IndexExpr whose
// index is the result position.
func assignments(name string, body *ast.BlockStmt) []ast.Expr {
	var out []ast.Expr
	ast.Inspect(body, func(n ast.Node) bool {
		switch st := n.(type) {
		case *ast.AssignStmt:
			if st.Tok != token.ASSIGN && st.Tok != token.DEFINE {
				return true
			}
			for i, lhs := range st.Lhs {
				if ident, ok := lhs.(*ast.Ident); ok && ident.Name == name {
					switch {
					case len(st.Lhs) == len(st.Rhs):
						out = append(out, st.Rhs[i])
					case len(st.Rhs) == 1:
						out = append(out, &ast.IndexExpr{X: st.Rhs[0], Index: &ast.BasicLit{Kind: token.INT, Value: strconv.Itoa(i)}})
					}
				}
			}
		case *ast.ValueSpec:
			for i, ident := range st.Names {
				if ident.Name == name && i < len(st.Values) {
					out = append(out, st.Values[i])
				}
			}
		case *ast.RangeStmt:
			if ident, ok := st.Value.(*ast.Ident); ok && ident.Name == name {
				if lit, ok := st.X.(*ast.CompositeLit); ok {
					out = append(out, lit.Elts...)
				}
			}
		}
		return true
	})
	return out
}

func appendedTo(name string, body *ast.BlockStmt) []ast.Expr {
	var out []ast.Expr
	ast.Inspect(body, func(n ast.Node) bool {
		if st, ok := n.(*ast.AssignStmt); ok && st.Tok == token.ADD_ASSIGN && len(st.Lhs) == 1 {
			if ident, ok := st.Lhs[0].(*ast.Ident); ok && ident.Name == name {
				out = append(out, st.Rhs[0])
			}
		}
		return true
	})
	return out
}

func (s *clientSource) evalIdent(name string, fr *frame, depth int) []string {
	if fr != nil {
		if b, ok := fr.params[name]; ok {
			return s.eval(b.expr, b.fr, depth+1)
		}
		if fr.visiting[name] {
			return []string{"{" + name + "}"}
		}
		if rhs := assignments(name, fr.fn.Body); len(rhs) > 0 {
			if fr.visiting == nil {
				fr.visiting = map[string]bool{}
			}
			fr.visiting[name] = true
			defer delete(fr.visiting, name)
			var values []string
			for _, r := range rhs {
				if idx, ok := r.(*ast.IndexExpr); ok {
					if lit, ok := idx.Index.(*ast.BasicLit); ok && lit.Kind == token.INT {
						if call, ok := idx.X.(*ast.CallExpr); ok {
							i, _ := strconv.Atoi(lit.Value)
							values = append(values, s.evalCall(call, i, fr, depth+1)...)
							continue
						}
					}
				}
				values = append(values, s.eval(r, fr, depth+1)...)
			}
			for _, r := range appendedTo(name, fr.fn.Body) {
				values = append(values, concat(values, s.eval(r, fr, depth+1))...)
			}
			return preferStatic(values)
		}
		if isParam(fr.fn, name) {
			return []string{"{" + name + "}"}
		}
	}
	if expr, ok := s.values[name]; ok {
		return s.eval(expr, nil, depth+1)
	}
	return []string{"{" + name + "}"}
}

func isParam(fn *ast.FuncDecl, name string) bool {
	for _, field := range fn.Type.Params.List {
		for _, n := range field.Names {
			if n.Name == name {
				return true
			}
		}
	}
	return false
}

// evalCall returns the values result index of call can take. A few standard
// library helpers are applied; package functions are followed into their
// return statements.
func (s *clientSource) evalCall(call *ast.CallExpr, index int, fr *frame, depth int) []string {
	arg := func(i int) []string {
		if i >= len(call.Args) {
			return []string{""}
		}
		return s.eval(call.Args[i], fr, depth+1)
	}
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok && index == 0 {
		if pkg, ok := sel.X.(*ast.Ident); ok && s.imports[pkg.Name] {
			switch pkg.Name + "." + sel.Sel.Name {
			case "url.PathEscape", "url.QueryEscape", "strings.TrimSpace":
				return arg(0)
			case "strings.TrimRight", "strings.TrimSuffix":
				var out []string
				for _, v := range arg(0) {
					for _, cut := range arg(1) {
						if sel.Sel.Name == "TrimRight" {
							out = append(out, strings.TrimRight(v, cut))
						} else {
							out = append(out, strings.TrimSuffix(v, cut))
						}
					}
				}
				return out
			case "fmt.Sprintf":
				return s.sprintf(arg(0), call.Args[1:], fr, depth)
			}
			return []string{placeholder(call)}
		}
	}
	callee, _ := s.resolveCallee(call)
	if callee == nil || callee.Body == nil || depth > maxEvalDepth {
		return []string{placeholder(call)}
	}
	child := bind(callee, call, fr)
	var values []string
	ast.Inspect(callee.Body, func(n ast.Node) bool {
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		if ret, ok := n.(*ast.ReturnStmt); ok && index < len(ret.Results) {
			values = append(values, s.eval(ret.Results[index], child, depth+1)...)
		}
		return true
	})
	if len(values) == 0 {
		return []string{placeholder(call)}
	}
	return preferStatic(values)
}

func (s *clientSource) sprintf(formats []string, args []ast.Expr, fr *frame, depth int) []string {
	var out []string
	for _, format := range formats {
		results := []string{""}
		next := 0
		for i := 0; i < len(format); i++ {
			if format[i] != '%' || i+1 >= len(format) {
				results = concat(results, []string{format[i : i+1]})
				continue
			}
			i++
			if format[i] == '%' {
				results = concat(results, []string{"%"})
				continue
			}
			if next < len(args) {
				results = concat(results, s.eval(args[next], fr, depth+1))
			}
			next++
		}
		out = append(out, results...)
	}
	return out
}
