package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// queryIndex is what query adoption needs to know about the client package:
// the hand-written functions without a receiver, the types that have methods,
// and every function or variable that names a query builder, with the reads
// whose paths it mentions.
type queryIndex struct {
	p        *pkg
	funcs    map[string]*decl   // hand-written functions without a receiver
	returns  map[string][]*decl // the same, by result type
	methodOn map[string]bool    // types with methods
	uses     map[string][]queryUse
}

// queryUse is a declaration that names a query builder.
type queryUse struct {
	name   string
	method bool
	reads  []string // GET operations whose path the declaration mentions
}

func newQueryIndex(spec *Spec, p *pkg, dir string) (*queryIndex, error) {
	qi := &queryIndex{p: p, funcs: map[string]*decl{}, returns: map[string][]*decl{}, methodOn: map[string]bool{}, uses: map[string][]queryUse{}}
	reads := map[string]string{}
	for _, tag := range spec.Tags() {
		for _, op := range spec.Operations(tag) {
			if op.Method == "GET" {
				reads[pathFormat(op.Path)] = op.ID
			}
		}
	}
	for _, pf := range p.files {
		for _, node := range pf.ast.Decls {
			qi.index(pf, node, reads)
		}
	}
	generated, err := filepath.Glob(filepath.Join(dir, "*_gen.go"))
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	for _, path := range generated {
		src, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		f, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			return nil, err
		}
		for _, node := range f.Decls {
			qi.index(nil, node, reads)
		}
	}
	return qi, nil
}

// index records a declaration; pf is nil for a generated file, whose
// declarations are only recorded as uses.
func (qi *queryIndex) index(pf *pkgFile, node ast.Decl, reads map[string]string) {
	use := queryUse{}
	if fn, ok := node.(*ast.FuncDecl); ok {
		use.name = fn.Name.Name
		if fn.Recv != nil && len(fn.Recv.List) == 1 {
			recv := strings.TrimPrefix(exprString(fn.Recv.List[0].Type), "*")
			qi.methodOn[recv] = true
			use.method = recv == "Client"
		} else if pf != nil {
			d := &decl{file: pf, node: fn, name: fn.Name.Name, src: qi.p.slice(pf, fn.Pos(), fn.End())}
			qi.funcs[d.name] = d
			if fn.Type.Results != nil && len(fn.Type.Results.List) == 1 {
				result := exprString(fn.Type.Results.List[0].Type)
				qi.returns[result] = append(qi.returns[result], d)
			}
		}
	}
	var builders []string
	ast.Inspect(node, func(x ast.Node) bool {
		switch x := x.(type) {
		case *ast.BasicLit:
			if id, ok := reads[stringLit(x)]; ok && !slices.Contains(use.reads, id) {
				use.reads = append(use.reads, id)
			}
		case *ast.Ident:
			if isBuilderName(x.Name) && x.Name != use.name && !slices.Contains(builders, x.Name) {
				builders = append(builders, x.Name)
			}
		}
		return true
	})
	if use.name == "" {
		use.name = "a package-level declaration"
	}
	for _, b := range builders {
		qi.uses[b] = append(qi.uses[b], use)
	}
}

func isBuilderName(name string) bool {
	return strings.HasPrefix(name, "build") && strings.HasSuffix(name, "Query") && len(name) > len("buildQuery")
}

// adopt finds, for each read of a tag, the one query builder its methods call,
// derives the query binding from the hand-written option type, query struct,
// With functions and builder, and adopts all of them only when every one is
// the same tokens as the generated code.
func (qi *queryIndex) adopt(spec *Spec, b *Bindings, tag string, ops []*Operation, report io.Writer) []*QueryBind {
	bound := map[string]bool{}
	for _, f := range b.Families {
		for _, q := range f.Queries {
			bound[q.Operation] = true
		}
	}
	var builderNames []string
	for name := range qi.uses {
		builderNames = append(builderNames, name)
	}
	sort.Strings(builderNames)
	var adopted []*QueryBind
	for _, op := range ops {
		if op.Method != "GET" {
			continue
		}
		var builders []string
		for _, name := range builderNames {
			for _, use := range qi.uses[name] {
				if slices.Contains(use.reads, op.ID) && !slices.Contains(builders, name) {
					builders = append(builders, name)
				}
			}
		}
		switch {
		case bound[op.ID]:
			fmt.Fprintf(report, "%s\t%s\tquery already generated\n", tag, op.ID)
		case len(builders) == 0:
		case len(builders) > 1:
			fmt.Fprintf(report, "%s\t%s\tquery %s\tkept: the read uses several builders\n", tag, op.ID, strings.Join(builders, ", "))
		default:
			q, parts, reason := qi.derive(spec, op, builders[0])
			if reason != "" {
				fmt.Fprintf(report, "%s\t%s\tquery %s\tkept: %s\n", tag, op.ID, builders[0], reason)
				continue
			}
			for _, d := range parts {
				d.adopt()
			}
			adopted = append(adopted, q)
			fmt.Fprintf(report, "%s\t%s\tquery %s\tgenerated\n", tag, op.ID, builders[0])
		}
	}
	return adopted
}

// derive returns the binding of a read's query and the hand-written
// declarations it replaces, or the reason the query stays hand-written.
func (qi *queryIndex) derive(spec *Spec, op *Operation, builder string) (*QueryBind, []*decl, string) {
	for _, use := range qi.uses[builder] {
		if !use.method || !slices.Equal(use.reads, []string{op.ID}) {
			return nil, nil, "used by " + use.name
		}
	}
	bd := qi.funcs[builder]
	if bd == nil || bd.adopted {
		return nil, nil, "no hand-written builder"
	}
	fn := bd.node.(*ast.FuncDecl)
	if len(fn.Type.Params.List) != 1 || len(fn.Type.Params.List[0].Names) != 1 {
		return nil, nil, "builder does not take one query"
	}
	structName := strings.TrimPrefix(exprString(fn.Type.Params.List[0].Type), "*")
	for name, d := range qi.funcs {
		if name != builder && isBuilderName(name) && strings.Contains(exprString(d.node.(*ast.FuncDecl).Type), "*"+structName+")") {
			return nil, nil, fmt.Sprintf("%s also builds from %s", name, structName)
		}
	}
	if qi.methodOn[structName] {
		return nil, nil, structName + " has methods"
	}
	sd := qi.p.types[structName]
	if sd == nil || sd.adopted {
		return nil, nil, structName + " is not a single type declaration"
	}
	var options []*decl
	for _, d := range qi.p.types {
		ts := d.node.(*ast.GenDecl).Specs[0].(*ast.TypeSpec)
		if ft, ok := ts.Type.(*ast.FuncType); ok && ft.Results == nil && len(ft.Params.List) == 1 && exprString(ft.Params.List[0].Type) == "*"+structName {
			options = append(options, d)
		}
	}
	if len(options) != 1 {
		return nil, nil, fmt.Sprintf("%d option types use %s", len(options), structName)
	}
	od := options[0]

	q := &QueryBind{Operation: op.ID, Name: strings.TrimSuffix(strings.TrimPrefix(builder, "build"), "Query")}
	if od.name != q.option() {
		q.Option = od.name
	}
	if q.structName() != structName {
		return nil, nil, fmt.Sprintf("query struct %s is not named %s", structName, q.structName())
	}
	withs := map[string]*decl{}
	for _, d := range qi.returns[od.name] {
		withs[d.name] = d
	}
	parts := map[string]*decl{od.name: od, structName: sd, builder: bd}
	for _, stmt := range fn.Body.List {
		call, ok := stmt.(*ast.ExprStmt)
		if !ok {
			continue
		}
		c, ok := call.X.(*ast.CallExpr)
		if !ok || exprString(c.Fun) != "addCSV" || len(c.Args) != 3 {
			continue
		}
		sel, ok := c.Args[2].(*ast.SelectorExpr)
		if !ok {
			continue
		}
		p := &QueryParam{Name: stringLit(c.Args[1]), Field: sel.Sel.Name}
		with := settingWith(withs, sel.Sel.Name)
		if with == nil {
			return nil, nil, "no option sets " + sel.Sel.Name
		}
		delete(withs, with.name)
		parts[with.name] = with
		wf := with.node.(*ast.FuncDecl)
		if len(wf.Type.Params.List) == 1 && len(wf.Type.Params.List[0].Names) == 1 {
			p.Arg = wf.Type.Params.List[0].Names[0].Name
		}
		if q.field(spec, op, &QueryParam{Name: p.Name}) == p.Field {
			p.Field = ""
		}
		if q.with(q.field(spec, op, p), p) != with.name {
			p.With = with.name
		}
		if p.Arg == q.field(spec, op, p) {
			p.Arg = ""
		}
		q.Params = append(q.Params, p)
	}
	for name := range withs {
		return nil, nil, name + " sets no parameter the builder sends"
	}

	rendered, err := renderQuery(spec, q)
	if err != nil {
		return nil, nil, err.Error()
	}
	var out []*decl
	for _, part := range rendered {
		d := parts[part.name]
		if d == nil {
			return nil, nil, "no hand-written " + part.name
		}
		if diff := tokenDiff(d.src, []byte(part.src)); diff != "" {
			return nil, nil, part.name + " differs at " + diff
		}
		out = append(out, d)
	}
	if len(out) != len(parts) {
		return nil, nil, "the generated query has fewer declarations"
	}
	return q, out, ""
}

// settingWith returns the With function that assigns the query field.
func settingWith(withs map[string]*decl, field string) *decl {
	var names []string
	for name := range withs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		found := false
		ast.Inspect(withs[name].node, func(x ast.Node) bool {
			if a, ok := x.(*ast.AssignStmt); ok && len(a.Lhs) == 1 {
				if sel, ok := a.Lhs[0].(*ast.SelectorExpr); ok && sel.Sel.Name == field {
					found = true
				}
			}
			return !found
		})
		if found {
			return withs[name]
		}
	}
	return nil
}
