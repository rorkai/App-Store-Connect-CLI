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
// the same tokens as the generated code. A builder that several reads call is
// adopted with the tag of the read it is bound to (see derive).
func (qi *queryIndex) adopt(spec *Spec, b *Bindings, tag string, ops []*Operation, report io.Writer) []*QueryBind {
	bound := map[string]bool{}
	for _, f := range b.Families {
		for _, q := range f.Queries {
			for _, id := range q.reads() {
				bound[id] = true
			}
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
			q, parts, reason := qi.derive(spec, builders[0])
			if reason == "" {
				if home, _ := spec.Operation(q.Operation); home.Tag != tag {
					reason = fmt.Sprintf("shared with %s, adopted with tag %s", q.Operation, home.Tag)
				}
				for _, id := range q.reads() {
					if bound[id] {
						reason = id + " already has a generated query"
					}
				}
			}
			if reason != "" {
				fmt.Fprintf(report, "%s\t%s\tquery %s\tkept: %s\n", tag, op.ID, builders[0], reason)
				continue
			}
			for _, d := range parts {
				d.adopt()
			}
			for _, id := range q.reads() {
				bound[id] = true
			}
			adopted = append(adopted, q)
			fmt.Fprintf(report, "%s\t%s\tquery %s\tgenerated\n", tag, op.ID, builders[0])
		}
	}
	return adopted
}

// derive returns the binding of a builder's query and the hand-written
// declarations it replaces, or the reason the query stays hand-written. Every
// use of the builder must be a client method of one read; the query
// is bound to the read with the shortest path (then the first operationId)
// and lists the others as Also. When several builders take the same query
// struct, the one named after the struct is the query's own and the others
// are its Builders, each bound to its reads the same way.
func (qi *queryIndex) derive(spec *Spec, builder string) (*QueryBind, []*decl, string) {
	reads, reason := qi.builderReads(spec, builder)
	if reason != "" {
		return nil, nil, reason
	}
	bd := qi.funcs[builder]
	if bd == nil || bd.adopted {
		return nil, nil, "no hand-written builder"
	}
	fn := bd.node.(*ast.FuncDecl)
	if !takesOneQuery(fn) {
		return nil, nil, "builder does not take one query"
	}
	structName := strings.TrimPrefix(exprString(fn.Type.Params.List[0].Type), "*")
	group := []string{builder}
	for name, d := range qi.funcs {
		if name != builder && isBuilderName(name) && strings.Contains(exprString(d.node.(*ast.FuncDecl).Type), "*"+structName+")") {
			group = append(group, name)
		}
	}
	sort.Strings(group)
	if len(group) > 1 {
		own := slices.IndexFunc(group, func(name string) bool { return builderQuery(name).structName() == structName })
		if own < 0 {
			return nil, nil, fmt.Sprintf("query struct %s is not named after one of its builders %s", structName, strings.Join(group, ", "))
		}
		builder = group[own]
		group = append(append([]string{builder}, group[:own]...), group[own+1:]...)
		if reads, reason = qi.builderReads(spec, builder); reason != "" {
			return nil, nil, reason
		}
	}
	op := reads[0]
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

	q := builderQuery(builder)
	q.Operation = op.ID
	for _, read := range reads[1:] {
		q.Also = append(q.Also, read.ID)
	}
	builders := []*decl{qi.funcs[builder]}
	for _, name := range group[1:] {
		d := qi.funcs[name]
		if d.adopted || !takesOneQuery(d.node.(*ast.FuncDecl)) {
			return nil, nil, name + " does not take one query"
		}
		reads, reason := qi.builderReads(spec, name)
		if reason != "" {
			return nil, nil, reason
		}
		extra := &QueryBuilder{Operation: reads[0].ID, Name: builderQuery(name).Name}
		for _, read := range reads[1:] {
			extra.Also = append(extra.Also, read.ID)
		}
		q.Builders = append(q.Builders, extra)
		builders = append(builders, d)
	}
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
	parts := map[string]*decl{od.name: od, structName: sd}
	fieldTypes := map[string]string{}
	if st, ok := sd.node.(*ast.GenDecl).Specs[0].(*ast.TypeSpec).Type.(*ast.StructType); ok {
		for _, f := range st.Fields.List {
			for _, n := range f.Names {
				fieldTypes[n.Name] = exprString(f.Type)
			}
		}
	}
	sentFrom := map[string]string{}
	for _, b := range builders {
		parts[b.name] = b
		bfn := b.node.(*ast.FuncDecl)
		for _, stmt := range bfn.Body.List {
			p := sentParam(stmt, bfn.Type.Params.List[0].Names[0].Name)
			if p == nil {
				continue
			}
			if field, ok := sentFrom[p.Name]; ok {
				if field != p.Field {
					return nil, nil, fmt.Sprintf("%s sends %s from %s, not %s", b.name, p.Name, p.Field, field)
				}
				continue
			}
			sentFrom[p.Name] = p.Field
			if reason := deriveParam(spec, q, op, stmt, p, fieldTypes, withs, parts); reason != "" {
				return nil, nil, reason
			}
		}
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

// deriveParam completes the binding of a parameter a builder statement sends
// and adds it to the query, with the With function that sets its field, or
// returns the reason it cannot.
func deriveParam(spec *Spec, q *QueryBind, op *Operation, stmt ast.Stmt, p *QueryParam, fieldTypes map[string]string, withs, parts map[string]*decl) string {
	if _, ok := stmt.(*ast.IfStmt); ok {
		typ, ok := scalarOfField[fieldTypes[p.Field]]
		if !ok {
			return fmt.Sprintf("field %s is a %s", p.Field, fieldTypes[p.Field])
		}
		p.Type = typ
	}
	field := p.Field
	with := settingWith(withs, field)
	if with == nil {
		return "no option sets " + field
	}
	delete(withs, with.name)
	parts[with.name] = with
	wf := with.node.(*ast.FuncDecl)
	if len(wf.Type.Params.List) == 1 && len(wf.Type.Params.List[0].Names) == 1 {
		p.Arg = wf.Type.Params.List[0].Names[0].Name
	}
	if p.Type == "" {
		p.Normalize = normalizeOf(wf, field)
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
	return ""
}

// builderReads returns the reads whose client methods use a builder, the one
// with the shortest path (then the first operationId) first, or the reason
// it is used otherwise or not at all.
func (qi *queryIndex) builderReads(spec *Spec, builder string) ([]*Operation, string) {
	var reads []*Operation
	for _, use := range qi.uses[builder] {
		if !use.method || len(use.reads) != 1 {
			return nil, "used by " + use.name
		}
		if op, _ := spec.Operation(use.reads[0]); !slices.Contains(reads, op) {
			reads = append(reads, op)
		}
	}
	if len(reads) == 0 {
		return nil, "no read uses " + builder
	}
	sort.Slice(reads, func(i, j int) bool {
		a, b := strings.Count(reads[i].Path, "/"), strings.Count(reads[j].Path, "/")
		return a < b || a == b && reads[i].ID < reads[j].ID
	})
	return reads, ""
}

// builderQuery is the query a builder named build<Name>Query is the own
// builder of.
func builderQuery(builder string) *QueryBind {
	return &QueryBind{Name: strings.TrimSuffix(strings.TrimPrefix(builder, "build"), "Query")}
}

func takesOneQuery(fn *ast.FuncDecl) bool {
	return len(fn.Type.Params.List) == 1 && len(fn.Type.Params.List[0].Names) == 1
}

// scalarOfField is the QueryParam.Type of a query field by its Go type.
var scalarOfField = map[string]string{"string": "string", "int": "int", "*bool": "bool"}

// sentParam returns the parameter a builder statement sends from a field of
// its query, with the parameter name and the field: a list sent by addCSV, or
// a value sent by values.Set inside an if. It returns nil for any other
// statement, which the token comparison then rejects.
func sentParam(stmt ast.Stmt, query string) *QueryParam {
	var call *ast.CallExpr
	switch stmt := stmt.(type) {
	case *ast.ExprStmt:
		c, ok := stmt.X.(*ast.CallExpr)
		if !ok || exprString(c.Fun) != "addCSV" || len(c.Args) != 3 {
			return nil
		}
		if sel, ok := c.Args[2].(*ast.SelectorExpr); ok {
			return &QueryParam{Name: stringLit(c.Args[1]), Field: sel.Sel.Name}
		}
		return nil
	case *ast.IfStmt:
		if stmt.Init != nil || stmt.Else != nil || len(stmt.Body.List) != 1 {
			return nil
		}
		set, ok := stmt.Body.List[0].(*ast.ExprStmt)
		if !ok {
			return nil
		}
		if call, ok = set.X.(*ast.CallExpr); !ok || exprString(call.Fun) != "values.Set" || len(call.Args) != 2 {
			return nil
		}
	default:
		return nil
	}
	var field string
	ast.Inspect(call.Args[1], func(x ast.Node) bool {
		if sel, ok := x.(*ast.SelectorExpr); ok && field == "" && exprString(sel.X) == query {
			field = sel.Sel.Name
		}
		return field == ""
	})
	if field == "" || stringLit(call.Args[0]) == "" {
		return nil
	}
	return &QueryParam{Name: stringLit(call.Args[0]), Field: field}
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

// normalizeOf returns the listNormalizers key of the function a With
// function assigns the query field with, or "" when it is another.
func normalizeOf(with *ast.FuncDecl, field string) string {
	key := ""
	ast.Inspect(with, func(x ast.Node) bool {
		a, ok := x.(*ast.AssignStmt)
		if !ok || len(a.Lhs) != 1 || len(a.Rhs) != 1 {
			return true
		}
		sel, ok := a.Lhs[0].(*ast.SelectorExpr)
		call, isCall := a.Rhs[0].(*ast.CallExpr)
		if !ok || sel.Sel.Name != field || !isCall {
			return true
		}
		for k, fn := range listNormalizers {
			if exprString(call.Fun) == fn {
				key = k
			}
		}
		return false
	})
	return key
}
