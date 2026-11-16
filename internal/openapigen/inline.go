package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// maxInline bounds how many helpers deep a method is inlined.
const maxInline = 4

// funcKey names a hand-written function or Client method in pkg.funcs.
func funcKey(fn *ast.FuncDecl) string {
	if fn.Recv == nil {
		return "func " + fn.Name.Name
	}
	if len(fn.Recv.List) == 1 && exprString(fn.Recv.List[0].Type) == "*Client" {
		return "method " + fn.Name.Name
	}
	return ""
}

// inline returns a Client method whose body only returns the result of an
// unexported helper of the package with the helper's body in place of the
// call, parameters bound to the arguments, repeated while the result again
// only returns a helper's result, and string literal arguments of fmt.Sprintf
// and fmt.Errorf folded into the format. It returns nil when the method
// delegates to no helper or a call cannot be inlined as written.
func (p *pkg) inline(d *decl) (*ast.FuncDecl, []byte, []*decl) {
	fn, fset, src := d.node.(*ast.FuncDecl), p.fset, d.src
	var via []*decl
	for range maxInline {
		callee, call := p.delegate(fn)
		if callee == nil {
			break
		}
		body, ok := p.bind(fn, call, callee)
		if !ok {
			return nil, nil, nil
		}
		off := func(pos token.Pos) int { return fset.Position(pos).Offset - fset.Position(fn.Pos()).Offset }
		next := append(append(slices.Clip(src[:off(fn.Body.Lbrace)]), '{'), append(body, '}')...)
		parsed, nfset := parseFunc(next)
		if parsed == nil {
			return nil, nil, nil
		}
		fn, fset, src = parsed, nfset, next
		via = append(via, callee)
	}
	if len(via) == 0 {
		return nil, nil, nil
	}
	src = foldFormats(fn, fset, src)
	fn, _ = parseFunc(src)
	if fn == nil {
		return nil, nil, nil
	}
	return fn, src, via
}

// parseFunc parses the source of one function declaration.
func parseFunc(src []byte) (*ast.FuncDecl, *token.FileSet) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", "package asc\n"+string(src), parser.ParseComments)
	if err != nil || len(f.Decls) != 1 {
		return nil, nil
	}
	fn, ok := f.Decls[0].(*ast.FuncDecl)
	if !ok {
		return nil, nil
	}
	return fn, fset
}

// delegate returns the unexported helper whose result is all a function
// returns, and the call.
func (p *pkg) delegate(fn *ast.FuncDecl) (*decl, *ast.CallExpr) {
	if fn.Body == nil || len(fn.Body.List) != 1 {
		return nil, nil
	}
	ret, ok := fn.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return nil, nil
	}
	call, ok := ret.Results[0].(*ast.CallExpr)
	if !ok {
		return nil, nil
	}
	key := ""
	switch f := call.Fun.(type) {
	case *ast.SelectorExpr:
		if recv, ok := f.X.(*ast.Ident); ok && fn.Recv != nil && len(fn.Recv.List[0].Names) == 1 && recv.Name == fn.Recv.List[0].Names[0].Name {
			key = "method " + f.Sel.Name
		}
	case *ast.Ident:
		key = "func " + f.Name
	case *ast.IndexExpr:
		if id, ok := f.X.(*ast.Ident); ok {
			key = "func " + id.Name
		}
	case *ast.IndexListExpr:
		if id, ok := f.X.(*ast.Ident); ok {
			key = "func " + id.Name
		}
	}
	callee := p.funcs[key]
	if callee == nil || ast.IsExported(callee.name) || callee.node.(*ast.FuncDecl).Body == nil {
		return nil, nil
	}
	return callee, call
}

// bind returns the body of the callee, without its braces, with each of its
// parameters replaced by the argument the call passes. An argument must be an
// identifier or a string literal, a parameter the callee assigns must get a
// parameter of the caller, and the callee must declare no name a parameter or
// the caller uses.
func (p *pkg) bind(caller *ast.FuncDecl, call *ast.CallExpr, callee *decl) ([]byte, bool) {
	cfn := callee.node.(*ast.FuncDecl)
	args := map[string]string{}
	var params []*ast.Field
	if cfn.Recv != nil {
		params = append(params, cfn.Recv.List...)
	}
	params = append(params, cfn.Type.Params.List...)
	var exprs []ast.Expr
	if cfn.Recv != nil {
		exprs = append(exprs, call.Fun.(*ast.SelectorExpr).X)
	}
	exprs = append(exprs, call.Args...)
	var names []string
	for _, field := range params {
		for _, n := range field.Names {
			names = append(names, n.Name)
		}
		if len(field.Names) == 0 {
			return nil, false
		}
	}
	variadic := len(cfn.Type.Params.List) > 0
	if variadic {
		_, variadic = cfn.Type.Params.List[len(cfn.Type.Params.List)-1].Type.(*ast.Ellipsis)
	}
	if len(names) != len(exprs) || variadic != call.Ellipsis.IsValid() {
		return nil, false
	}
	callerParams := map[string]bool{}
	for _, field := range caller.Type.Params.List {
		for _, n := range field.Names {
			callerParams[n.Name] = true
		}
	}
	bound := map[string]int{}
	for i, name := range names {
		switch e := exprs[i].(type) {
		case *ast.Ident:
			args[name] = e.Name
			bound[e.Name]++
		case *ast.BasicLit:
			if e.Kind != token.STRING {
				return nil, false
			}
			args[name] = e.Value
		default:
			return nil, false
		}
	}
	if tp := cfn.Type.TypeParams; tp != nil {
		var types []ast.Expr
		switch f := call.Fun.(type) {
		case *ast.IndexExpr:
			types = []ast.Expr{f.Index}
		case *ast.IndexListExpr:
			types = f.Indices
		}
		i := 0
		for _, field := range tp.List {
			for _, n := range field.Names {
				if i >= len(types) {
					return nil, false
				}
				args[n.Name] = exprString(types[i])
				i++
			}
		}
		if i != len(types) {
			return nil, false
		}
	}

	skip := map[*ast.Ident]bool{}
	ok := true
	ast.Inspect(cfn.Body, func(x ast.Node) bool {
		switch n := x.(type) {
		case *ast.SelectorExpr:
			skip[n.Sel] = true
		case *ast.CompositeLit:
			for _, elt := range n.Elts {
				if kv, isKV := elt.(*ast.KeyValueExpr); isKV {
					if key, isIdent := kv.Key.(*ast.Ident); isIdent {
						skip[key] = true
					}
				}
			}
		case *ast.AssignStmt:
			for _, lhs := range n.Lhs {
				id, isIdent := lhs.(*ast.Ident)
				switch {
				case !isIdent:
				case n.Tok == token.DEFINE:
					if _, isParam := args[id.Name]; isParam || callerParams[id.Name] || bound[id.Name] > 0 {
						ok = false
					}
				case args[id.Name] != "":
					if !callerParams[args[id.Name]] || bound[args[id.Name]] != 1 {
						ok = false
					}
				}
			}
		case *ast.IncDecStmt, *ast.FuncLit, *ast.RangeStmt, *ast.DeclStmt, *ast.TypeSwitchStmt, *ast.LabeledStmt:
			ok = false
		}
		return ok
	})
	if !ok {
		return nil, false
	}
	type edit struct {
		from, to int
		text     string
	}
	src := callee.file.src
	var edits []edit
	ast.Inspect(cfn.Body, func(x ast.Node) bool {
		if id, isIdent := x.(*ast.Ident); isIdent && !skip[id] {
			if text, isParam := args[id.Name]; isParam {
				edits = append(edits, edit{p.fset.Position(id.Pos()).Offset, p.fset.Position(id.End()).Offset, text})
			}
		}
		return true
	})
	start, end := p.fset.Position(cfn.Body.Lbrace).Offset+1, p.fset.Position(cfn.Body.Rbrace).Offset
	var out []byte
	at := start
	for _, e := range edits {
		out = append(append(out, src[at:e.from]...), e.text...)
		at = e.to
	}
	return append(out, src[at:end]...), true
}

// foldFormats puts each string literal argument of an fmt.Sprintf or
// fmt.Errorf call whose format has only %s verbs into the format, so that
// fmt.Sprintf("/v1/x/%s/%s", id, "y") reads fmt.Sprintf("/v1/x/%s/y", id),
// and a Sprintf left with no arguments becomes its format. A literal holding
// a % is left alone.
func foldFormats(fn *ast.FuncDecl, fset *token.FileSet, src []byte) []byte {
	type edit struct {
		from, to int
		text     string
	}
	off := func(pos token.Pos) int { return fset.Position(pos).Offset - fset.Position(fn.Pos()).Offset }
	var edits []edit
	ast.Inspect(fn.Body, func(x ast.Node) bool {
		call, ok := x.(*ast.CallExpr)
		if !ok || call.Ellipsis.IsValid() || len(call.Args) < 2 {
			return true
		}
		name := exprString(call.Fun)
		if name != "fmt.Sprintf" && name != "fmt.Errorf" {
			return true
		}
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		format, err := strconv.Unquote(lit.Value)
		if err != nil || strings.Count(format, "%") != strings.Count(format, "%s") || strings.Count(format, "%s") != len(call.Args)-1 {
			return true
		}
		parts := strings.Split(format, "%s")
		var b strings.Builder
		var rest []string
		folded := false
		for i, arg := range call.Args[1:] {
			b.WriteString(parts[i])
			if v := stringLit(arg); v != "" && !strings.Contains(v, "%") {
				b.WriteString(v)
				folded = true
				continue
			}
			b.WriteString("%s")
			rest = append(rest, string(src[off(arg.Pos()):off(arg.End())]))
		}
		b.WriteString(parts[len(parts)-1])
		if !folded {
			return true
		}
		text := strconv.Quote(b.String())
		if len(rest) > 0 || name == "fmt.Errorf" {
			text = name + "(" + strings.Join(append([]string{text}, rest...), ", ") + ")"
		}
		edits = append(edits, edit{off(call.Pos()), off(call.End()), text})
		return false
	})
	var out []byte
	at := 0
	for _, e := range edits {
		out = append(append(out, src[at:e.from]...), e.text...)
		at = e.to
	}
	return append(out, src[at:]...)
}

// removeUnusedHelpers removes each helper that adopted methods were inlined
// through and that no remaining declaration of the package names any more,
// repeating until none is left to remove.
func (p *pkg) removeUnusedHelpers(dir string, report io.Writer) error {
	var helpers []*decl
	for _, ds := range p.methods {
		for _, d := range ds {
			if d.adopted {
				for _, h := range d.via {
					if !slices.Contains(helpers, h) {
						helpers = append(helpers, h)
					}
				}
			}
		}
	}
	sort.Slice(helpers, func(i, j int) bool { return helpers[i].name < helpers[j].name })
	generated, err := filepath.Glob(filepath.Join(dir, "*_gen.go"))
	if err != nil {
		return err
	}
	var gen []byte
	for _, path := range generated {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		gen = append(gen, data...)
	}
	for removed := true; removed; {
		removed = false
		for _, h := range helpers {
			if h.adopted || p.names(h) || regexp.MustCompile(`\b`+h.name+`\b`).Match(gen) {
				continue
			}
			h.adopt()
			removed = true
			fmt.Fprintf(report, "helper %s\tremoved, no caller left\n", h.name)
		}
	}
	return nil
}

// names reports whether a declaration of the package other than the helper
// itself and the removed ones mentions the helper's name.
func (p *pkg) names(h *decl) bool {
	for _, pf := range p.files {
		for _, node := range pf.ast.Decls {
			if node == h.node || slices.Contains(pf.removed, ast.Node(node)) {
				continue
			}
			found := false
			ast.Inspect(node, func(x ast.Node) bool {
				if id, ok := x.(*ast.Ident); ok && id.Name == h.name {
					found = true
				}
				return !found
			})
			if found {
				return true
			}
		}
	}
	return false
}
