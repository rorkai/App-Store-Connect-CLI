package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// maxInline bounds how many helpers deep a method is inlined.
const maxInline = 4

// funcKey names a hand-written function or Client method in pkg.funcs.
func funcKey(fn *ast.FuncDecl) string {
	switch {
	case fn.Recv == nil:
		return "func " + fn.Name.Name
	case len(fn.Recv.List) == 1 && exprString(fn.Recv.List[0].Type) == "*Client":
		return "method " + fn.Name.Name
	}
	return ""
}

// inline returns a Client method whose body only returns the result of an
// unexported helper with the helper's body in place of the call, repeated
// while that body again only returns a helper's result, and with string
// literal arguments of fmt.Sprintf and fmt.Errorf folded into the format. It
// returns nil when the method delegates to no helper or a call cannot be
// inlined as written.
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
		lbrace := fset.Position(fn.Body.Lbrace).Offset - fset.Position(fn.Pos()).Offset
		src = slices.Concat(src[:lbrace], []byte("{"), body, []byte("}"))
		if fn, fset = parseFunc(src); fn == nil {
			return nil, nil, nil
		}
		via = append(via, callee)
	}
	if len(via) == 0 {
		return nil, nil, nil
	}
	src = foldFormats(fn, fset, src)
	if fn, _ = parseFunc(src); fn == nil {
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
	fn, _ := f.Decls[0].(*ast.FuncDecl)
	return fn, fset
}

// delegate returns the unexported helper whose result is all a method
// returns, and the call.
func (p *pkg) delegate(fn *ast.FuncDecl) (*decl, *ast.CallExpr) {
	if len(fn.Body.List) != 1 {
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
		if recv := fn.Recv.List[0].Names; len(recv) == 1 && exprString(f.X) == recv[0].Name {
			key = "method " + f.Sel.Name
		}
	case *ast.Ident:
		key = "func " + f.Name
	case *ast.IndexExpr:
		key = "func " + exprString(f.X)
	case *ast.IndexListExpr:
		key = "func " + exprString(f.X)
	}
	callee := p.funcs[key]
	if callee == nil || ast.IsExported(callee.name) || callee.node.(*ast.FuncDecl).Body == nil {
		return nil, nil
	}
	return callee, call
}

// bind returns the body of the callee, without its braces, with each
// parameter and type parameter replaced by what the call passes. An argument
// must be an identifier or a string literal, a parameter the callee assigns
// must get a parameter of the caller, the callee must declare no name of a
// parameter, of the caller's parameters or of what the call passes, and no
// literal key may share a parameter's name.
func (p *pkg) bind(caller *ast.FuncDecl, call *ast.CallExpr, callee *decl) ([]byte, bool) {
	cfn := callee.node.(*ast.FuncDecl)
	fields := cfn.Type.Params.List
	exprs := call.Args
	if cfn.Recv != nil {
		fields = append(slices.Clone(cfn.Recv.List), fields...)
		exprs = append([]ast.Expr{call.Fun.(*ast.SelectorExpr).X}, exprs...)
	}
	var names []string
	for _, field := range fields {
		if len(field.Names) == 0 {
			return nil, false
		}
		for _, n := range field.Names {
			names = append(names, n.Name)
		}
	}
	params := cfn.Type.Params.List
	if len(params) == 0 || len(names) != len(exprs) {
		return nil, false
	}
	if _, variadic := params[len(params)-1].Type.(*ast.Ellipsis); variadic != call.Ellipsis.IsValid() {
		return nil, false
	}
	args, passed := map[string]string{}, map[string]int{}
	for i, name := range names {
		switch e := exprs[i].(type) {
		case *ast.Ident:
			args[name] = e.Name
			passed[e.Name]++
		case *ast.BasicLit:
			if e.Kind != token.STRING {
				return nil, false
			}
			args[name] = e.Value
		default:
			return nil, false
		}
	}
	var typeArgs []ast.Expr
	switch f := call.Fun.(type) {
	case *ast.IndexExpr:
		typeArgs = []ast.Expr{f.Index}
	case *ast.IndexListExpr:
		typeArgs = f.Indices
	}
	var typeParams []string
	if cfn.Type.TypeParams != nil {
		for _, field := range cfn.Type.TypeParams.List {
			for _, n := range field.Names {
				typeParams = append(typeParams, n.Name)
			}
		}
	}
	if len(typeParams) != len(typeArgs) {
		return nil, false
	}
	for i, name := range typeParams {
		args[name] = exprString(typeArgs[i])
	}
	callerParams := map[string]bool{}
	for _, field := range caller.Type.Params.List {
		for _, n := range field.Names {
			callerParams[n.Name] = true
		}
	}

	skip := map[*ast.Ident]bool{}
	ok := true
	ast.Inspect(cfn.Body, func(x ast.Node) bool {
		switch n := x.(type) {
		case *ast.SelectorExpr:
			skip[n.Sel] = true
		case *ast.KeyValueExpr:
			if key, isIdent := n.Key.(*ast.Ident); isIdent {
				_, isParam := args[key.Name]
				ok = ok && !isParam
			}
		case *ast.AssignStmt:
			for _, lhs := range n.Lhs {
				id, isIdent := lhs.(*ast.Ident)
				if !isIdent {
					continue
				}
				_, isParam := args[id.Name]
				switch {
				case n.Tok == token.DEFINE:
					ok = ok && !isParam && !callerParams[id.Name] && passed[id.Name] == 0
				case isParam:
					ok = ok && callerParams[args[id.Name]] && passed[args[id.Name]] == 1
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
	var edits []spanEdit
	ast.Inspect(cfn.Body, func(x ast.Node) bool {
		if id, isIdent := x.(*ast.Ident); isIdent && !skip[id] {
			if text, isParam := args[id.Name]; isParam {
				edits = append(edits, spanEdit{p.fset.Position(id.Pos()).Offset, p.fset.Position(id.End()).Offset, text})
			}
		}
		return true
	})
	start := p.fset.Position(cfn.Body.Lbrace).Offset + 1
	body := callee.file.src[:p.fset.Position(cfn.Body.Rbrace).Offset]
	return applyEdits(body, edits)[start:], true
}

// spanEdit replaces the bytes from:to of a source.
type spanEdit struct {
	from, to int
	text     string
}

// applyEdits applies edits in ascending, non-overlapping order.
func applyEdits(src []byte, edits []spanEdit) []byte {
	var out []byte
	at := 0
	for _, e := range edits {
		out = append(append(out, src[at:e.from]...), e.text...)
		at = e.to
	}
	return append(out, src[at:]...)
}

// foldFormats puts each string literal argument of an fmt.Sprintf or
// fmt.Errorf call whose format has only %s verbs into the format, so that
// fmt.Sprintf("/v1/x/%s/%s", id, "y") reads fmt.Sprintf("/v1/x/%s/y", id),
// and a Sprintf left with no arguments becomes its format. A literal holding
// a % is left alone.
func foldFormats(fn *ast.FuncDecl, fset *token.FileSet, src []byte) []byte {
	off := func(pos token.Pos) int { return fset.Position(pos).Offset - fset.Position(fn.Pos()).Offset }
	var edits []spanEdit
	ast.Inspect(fn.Body, func(x ast.Node) bool {
		call, ok := x.(*ast.CallExpr)
		name := ""
		if ok {
			name = exprString(call.Fun)
		}
		if (name != "fmt.Sprintf" && name != "fmt.Errorf") || call.Ellipsis.IsValid() || len(call.Args) < 2 {
			return true
		}
		format, err := strconv.Unquote(exprString(call.Args[0]))
		verbs := strings.Count(format, "%s")
		if err != nil || strings.Count(format, "%") != verbs || verbs != len(call.Args)-1 {
			return true
		}
		parts := strings.Split(format, "%s")
		folded := parts[0]
		var rest []string
		for i, arg := range call.Args[1:] {
			if v := stringLit(arg); v != "" && !strings.Contains(v, "%") {
				folded += v + parts[i+1]
				continue
			}
			folded += "%s" + parts[i+1]
			rest = append(rest, string(src[off(arg.Pos()):off(arg.End())]))
		}
		if len(rest) == verbs {
			return true
		}
		text := strconv.Quote(folded)
		if len(rest) > 0 || name == "fmt.Errorf" {
			text = name + "(" + strings.Join(append([]string{text}, rest...), ", ") + ")"
		}
		edits = append(edits, spanEdit{off(call.Pos()), off(call.End()), text})
		return false
	})
	return applyEdits(src, edits)
}

// removeUnusedHelpers removes each helper an adopted method was inlined
// through once no remaining declaration names it, repeating while that frees
// another.
func (p *pkg) removeUnusedHelpers(report io.Writer) {
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
	for removed := true; removed; {
		removed = false
		for _, h := range helpers {
			if !h.adopted && !p.names(h) {
				h.adopt()
				removed = true
				fmt.Fprintf(report, "helper %s\tremoved, no caller left\n", h.name)
			}
		}
	}
}

// names reports whether a declaration of the package other than the helper
// and the removed ones mentions the helper's name.
func (p *pkg) names(h *decl) bool {
	for _, pf := range p.files {
		for _, node := range pf.ast.Decls {
			if node == h.node || slices.Contains(pf.removed, ast.Node(node)) {
				continue
			}
			found := false
			ast.Inspect(node, func(x ast.Node) bool {
				id, ok := x.(*ast.Ident)
				found = found || (ok && id.Name == h.name)
				return !found
			})
			if found {
				return true
			}
		}
	}
	return false
}
