package main

import (
	"bytes"
	"cmp"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/scanner"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
)

var httpMethods = map[string]bool{"GET": true, "POST": true, "PATCH": true, "DELETE": true, "PUT": true}

// pkgFile is one hand-written file of the client package.
type pkgFile struct {
	path    string
	src     []byte
	ast     *ast.File
	removed []ast.Node
}

type pkg struct {
	fset    *token.FileSet
	files   []*pkgFile
	methods map[string][]*decl // by HTTP method and path literal, such as "GET /v1/apps/%s"
	types   map[string]*decl
	funcs   map[string]*decl // functions and Client methods, by funcKey
}

type decl struct {
	file    *pkgFile
	node    ast.Node // *ast.FuncDecl or single-spec *ast.GenDecl
	name    string
	src     []byte // from the func or type keyword to the end, without the doc comment
	adopted bool
	// fn is the method with the helpers in via inlined, and src its source,
	// when the method only returns a helper's result.
	fn  *ast.FuncDecl
	via []*decl
}

// funcDecl is the method as adoption compares it.
func (d *decl) funcDecl() *ast.FuncDecl {
	if d.fn != nil {
		return d.fn
	}
	return d.node.(*ast.FuncDecl)
}

func (d *decl) adopt() {
	d.adopted = true
	d.file.removed = append(d.file.removed, d.node)
}

// Adopt finds the hand-written methods and types of the given tags whose
// source equals what the generator renders, adds their bindings, and reports
// every operation and type it leaves hand-written, with the reason. It writes
// nothing: it returns the new contents of each hand-written file it would
// change, nil for a file left empty, and the families whose bindings grew.
func Adopt(spec *Spec, b *Bindings, dir string, tags []string, report io.Writer) (map[string][]byte, []*Family, error) {
	p, err := loadPackage(dir)
	if err != nil {
		return nil, nil, err
	}
	qi, err := newQueryIndex(spec, p, dir)
	if err != nil {
		return nil, nil, err
	}
	var adopted []*Family
	bound := map[string]bool{}
	for _, f := range b.Families {
		for _, op := range f.Operations {
			bound[op.Operation] = true
		}
	}
	for _, tag := range tags {
		ops := spec.Operations(tag)
		if len(ops) == 0 {
			return nil, nil, fmt.Errorf("no operations tagged %s", tag)
		}
		var adoptedOps []*OpBind
		var adoptedTypes []string
		// view holds the relationships records the family has and the ones
		// the hand-written data types of its creates declare.
		view := &Family{}
		// fixed holds the Go type of each schema the family's generated
		// types reference.
		fixed := map[string]string{}
		var generated []*OpBind
		tidied := false
		for _, f := range b.Families {
			if f.Tag == tag {
				tidied = dropDefaultResponses(spec, f, report)
				view.Relationships = slices.Clone(f.Relationships)
				view.RefTypes = slices.Clone(f.RefTypes)
				view.GoTypes = slices.Clone(f.GoTypes)
				view.Types = f.Types
				generated = f.Operations
				for _, name := range f.Types {
					fixRefTypes(spec, f, name, fixed)
				}
			}
		}
		recorded := len(view.Relationships)
		recordedRefs := len(view.RefTypes)
		recordedGoTypes := len(view.GoTypes)
		p.recordRelationships(spec, ops, view)
		for _, op := range ops {
			candidates := p.methods[op.Method+" "+pathFormat(op.Path)]
			if bound[op.ID] {
				fmt.Fprintf(report, "%s\t%s\talready generated\n", tag, op.ID)
				continue
			}
			if len(candidates) == 0 {
				fmt.Fprintf(report, "%s\t%s\tno method (new name would be %s)\n", tag, op.ID, defaultName(spec, op))
				continue
			}
			for _, d := range candidates {
				if d.adopted {
					continue
				}
				trial := p.withVersionGoTypes(spec, view, op, d)
				bind, reason := p.deriveOp(spec, trial, op, d)
				if reason == "" {
					_, reason = p.resourceGoType(spec, trial, bind)
				}
				if reason != "" {
					fmt.Fprintf(report, "%s\t%s\t%s\tkept: %s\n", tag, op.ID, d.name, reason)
					continue
				}
				view.GoTypes = trial.GoTypes
				d.adopt()
				adoptedOps = append(adoptedOps, bind)
				fmt.Fprintf(report, "%s\t%s\t%s\tgenerated\n", tag, op.ID, d.name)
			}
		}
		for _, b := range slices.Concat(generated, adoptedOps) {
			p.recordResourceGoType(spec, view, b)
		}
		names := typeCandidates(spec, ops)
		for _, r := range view.Relationships[recorded:] {
			names = append(names, strings.TrimPrefix(r.Type, "*"))
		}
		for _, name := range names {
			if typ := view.goType(name); typ != goTypeName(name) {
				fmt.Fprintf(report, "%s\ttype %s\tis Go type %s\n", tag, name, typ)
				continue
			}
			d, ok := p.types[goTypeName(name)]
			if !ok || d.adopted {
				continue
			}
			refs, reason := refTypesOf(spec, view, fixed, name, d)
			if reason != "" {
				fmt.Fprintf(report, "%s\ttype %s\tkept: %s\n", tag, name, reason)
				continue
			}
			withRefs := *view
			withRefs.RefTypes = refs
			src, err := renderType(spec, &withRefs, name)
			if err != nil {
				fmt.Fprintf(report, "%s\ttype %s\tkept: %v\n", tag, name, err)
				continue
			}
			if diff := tokenDiff(d.src, []byte(src)); diff != "" {
				fmt.Fprintf(report, "%s\ttype %s\tkept: differs at %s\n", tag, name, diff)
				continue
			}
			d.adopt()
			adoptedTypes = append(adoptedTypes, name)
			view.RefTypes = refs
			fixRefTypes(spec, view, name, fixed)
			fmt.Fprintf(report, "%s\ttype %s\tgenerated\n", tag, name)
		}
		var adoptedRels []*RelBind
		for _, r := range view.Relationships[recorded:] {
			if slices.Contains(adoptedTypes, r.Data) || slices.Contains(adoptedTypes, strings.TrimPrefix(r.Type, "*")) || slices.ContainsFunc(adoptedOps, func(op *OpBind) bool {
				o, _ := spec.Operation(op.Operation)
				return strings.TrimSuffix(o.Request, "Request")+"Data" == r.Data
			}) {
				adoptedRels = append(adoptedRels, r)
			}
		}
		adoptedQueries := qi.adopt(spec, b, tag, ops, report)
		if len(adoptedOps)+len(adoptedTypes)+len(adoptedQueries) > 0 || len(view.GoTypes) > recordedGoTypes || tidied {
			fam := b.Family(tag, fileName(tag))
			fam.Operations = append(fam.Operations, adoptedOps...)
			fam.Types = append(fam.Types, adoptedTypes...)
			fam.Relationships = append(fam.Relationships, adoptedRels...)
			fam.Queries = append(fam.Queries, adoptedQueries...)
			fam.RefTypes = append(fam.RefTypes, view.RefTypes[recordedRefs:]...)
			fam.GoTypes = append(fam.GoTypes, view.GoTypes[recordedGoTypes:]...)
			adopted = append(adopted, fam)
		}
	}
	p.removeUnusedHelpers(report)
	edits, err := p.edits()
	return edits, adopted, err
}

// dropDefaultResponses clears each response a family binds that is the
// type the generator decodes into anyway, and reports whether it cleared one.
func dropDefaultResponses(spec *Spec, fam *Family, report io.Writer) bool {
	dropped := false
	for _, b := range fam.Operations {
		if op, ok := spec.Operation(b.Operation); ok && b.Response != "" && b.Response == fam.goType(op.Response) {
			fmt.Fprintf(report, "%s\t%s\t%s\tresponse %s is the default, dropped\n", fam.Tag, b.Operation, b.Name, b.Response)
			b.Response = ""
			dropped = true
		}
	}
	return dropped
}

func loadPackage(dir string) (*pkg, error) {
	p := &pkg{fset: token.NewFileSet(), methods: map[string][]*decl{}, types: map[string]*decl{}, funcs: map[string]*decl{}}
	paths, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil, err
	}
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if bytes.HasPrefix(src, []byte(header)) {
			continue
		}
		f, err := parser.ParseFile(p.fset, path, src, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		pf := &pkgFile{path: path, src: src, ast: f}
		p.files = append(p.files, pf)
		for _, node := range f.Decls {
			if fn, ok := node.(*ast.FuncDecl); ok && funcKey(fn) != "" {
				p.funcs[funcKey(fn)] = &decl{file: pf, node: fn, name: fn.Name.Name, src: p.slice(pf, fn.Pos(), fn.End())}
			}
		}
	}
	for _, pf := range p.files {
		for _, node := range pf.ast.Decls {
			p.index(pf, node)
		}
	}
	return p, nil
}

func (p *pkg) slice(pf *pkgFile, from, to token.Pos) []byte {
	return pf.src[p.fset.Position(from).Offset:p.fset.Position(to).Offset]
}

func (p *pkg) index(pf *pkgFile, node ast.Decl) {
	switch n := node.(type) {
	case *ast.FuncDecl:
		if n.Recv == nil || len(n.Recv.List) != 1 || exprString(n.Recv.List[0].Type) != "*Client" {
			return
		}
		d := p.funcs[funcKey(n)]
		if fn, src, via := p.inline(d); fn != nil {
			d.fn, d.src, d.via = fn, src, via
		}
		var paths, verbs []string
		ast.Inspect(d.funcDecl().Body, func(x ast.Node) bool {
			switch x := x.(type) {
			case *ast.BasicLit:
				if v := stringLit(x); strings.HasPrefix(v, "/v") {
					paths = append(paths, v)
				} else if httpMethods[v] {
					verbs = append(verbs, v)
				}
			case *ast.SelectorExpr:
				if v, ok := strings.CutPrefix(exprString(x), "http.Method"); ok {
					verbs = append(verbs, strings.ToUpper(v))
				}
			case *ast.Ident:
				if x.Name == "getJSON" {
					verbs = append(verbs, "GET")
				}
			}
			return true
		})
		seen := map[string]bool{}
		for _, path := range paths {
			for _, verb := range verbs {
				if key := verb + " " + path; !seen[key] {
					seen[key] = true
					p.methods[key] = append(p.methods[key], d)
				}
			}
		}
	case *ast.GenDecl:
		if n.Tok != token.TYPE || len(n.Specs) != 1 || n.Lparen.IsValid() {
			return
		}
		ts := n.Specs[0].(*ast.TypeSpec)
		p.types[ts.Name.Name] = &decl{file: pf, node: n, name: ts.Name.Name, src: p.slice(pf, n.Pos(), n.End())}
	}
}

// exprString prints an expression such as a parameter type.
func exprString(expr ast.Expr) string {
	var buf bytes.Buffer
	if err := format.Node(&buf, token.NewFileSet(), expr); err != nil {
		return ""
	}
	return buf.String()
}

func pathFormat(template string) string {
	var b strings.Builder
	for {
		start := strings.Index(template, "{")
		if start < 0 {
			b.WriteString(template)
			return b.String()
		}
		end := strings.Index(template, "}")
		b.WriteString(template[:start] + "%s")
		template = template[end+1:]
	}
}

// deriveOp reads the binding choices out of a hand-written method, renders
// the method from them, and accepts the binding only when the two are the
// same tokens.
func (p *pkg) deriveOp(spec *Spec, fam *Family, op *Operation, d *decl) (*OpBind, string) {
	fn := d.funcDecl()
	b := &OpBind{Operation: op.ID, Name: fn.Name.Name}
	argFields := attributeArgs(fn.Body)
	for i, field := range fn.Type.Params.List {
		typ := exprString(field.Type)
		if len(field.Names) == 1 && argFields[field.Names[0].Name] != "" {
			arg, reason := deriveArg(spec, op, field.Names[0].Name, argFields[field.Names[0].Name], typ)
			if reason != "" {
				return nil, reason
			}
			b.Args = append(b.Args, arg)
			continue
		}
		switch {
		case i == 0 && typ == "context.Context":
		case typ == "string":
			for _, n := range field.Names {
				if argFields[n.Name] == "" {
					b.Params = append(b.Params, n.Name)
					continue
				}
				arg, reason := deriveArg(spec, op, n.Name, argFields[n.Name], typ)
				if reason != "" {
					return nil, reason
				}
				arg.Param = true
				b.Args = append(b.Args, arg)
			}
		case typ == "[]string" && op.Method == "GET":
		case typ == "[]string" && len(field.Names) == 1:
			b.IDs = field.Names[0].Name
		case strings.HasPrefix(typ, "..."):
			b.Options = strings.TrimPrefix(typ, "...")
		case strings.HasSuffix(typ, "Attributes"):
			if strings.HasPrefix(typ, "*") {
				b.Attrs = attrsPointer
			}
		default:
			return nil, "parameter type " + typ
		}
	}

	var messages, links []string
	var trimmed, next, attrsByValue, builtPath bool
	pathErr := ""
	var trimAt, optionsAt token.Pos
	list := &IDList{}
	var optional []optionalIf
	ast.Inspect(fn.Body, func(x ast.Node) bool {
		if o, ok := readOptionalLink(x); ok {
			optional = append(optional, o)
			return false
		}
		if b.IDs != "" && strings.HasSuffix(op.Request, "LinkagesRequest") && readIDList(x, b.IDs, list) {
			return false
		}
		switch n := x.(type) {
		case *ast.KeyValueExpr:
			key, _ := n.Key.(*ast.Ident)
			if key == nil {
				return true
			}
			switch key.Name {
			case "Query":
				if call, ok := n.Value.(*ast.CallExpr); ok {
					b.Query = exprString(call.Fun)
				}
			case "Next":
				next = true
			case "NextContext":
				b.NextContext = stringLit(n.Value)
			case "ParseErr":
				b.ParseErr = stringLit(n.Value)
			case "Attributes":
				_, attrsByValue = n.Value.(*ast.Ident)
			default:
				if lit := compositeLit(n.Value); lit != nil && exprString(lit.Type) == "Relationship" {
					links = append(links, key.Name)
				}
			}
		case *ast.IndexExpr:
			if fun := exprString(n.X); (fun == "getJSON" || fun == "sendJSON") && exprString(n.Index) != goTypeName(op.Response) {
				b.Response = exprString(n.Index)
			}
		case *ast.AssignStmt:
			if exprString(n.Lhs[0]) == "payload.Data.Attributes" {
				b.SkipEmptyAttributes = true
			}
			call, ok := n.Rhs[0].(*ast.CallExpr)
			switch {
			case !ok:
			case exprString(call.Fun) == "strings.TrimSpace" && b.Check == checkInline:
				b.Check = checkTrim
				trimAt = n.Pos()
			case exprString(call.Fun) == "applyOptions" && !optionsAt.IsValid():
				optionsAt = n.Pos()
			}
		case *ast.CallExpr:
			if exprString(n.Fun) == "strings.TrimSpace" {
				trimmed = true
			}
			if exprString(n.Fun) == "fmt.Errorf" && len(n.Args) == 1 {
				messages = append(messages, stringLit(n.Args[0]))
				b.Check = checkRequire
			}
			if exprString(n.Fun) == "resourcePath" {
				builtPath = true
			}
			if exprString(n.Fun) == "fmt.Errorf" && len(n.Args) == 2 && exprString(n.Args[1]) == "err" {
				pathErr, _ = strings.CutSuffix(stringLit(n.Args[0]), ": %w")
			}
		}
		return true
	})
	for _, o := range optional {
		b.Params = slices.DeleteFunc(b.Params, func(p string) bool { return p == o.param })
		for _, name := range linkNames(spec, op, []string{o.field}) {
			b.Optional = append(b.Optional, OptionalLink{Param: o.param, Link: name})
		}
		b.RelationshipsWhenSet = b.RelationshipsWhenSet || o.whenSet
	}
	if !trimmed && len(b.stringParams()) > 0 {
		b.Check = checkRaw
	}
	if builtPath {
		b.Check, b.PathErr = checkPath, pathErr
	}
	if *list != (IDList{}) {
		b.IDList = list
	}
	b.CheckFirst = b.Options != "" && trimAt.IsValid() && optionsAt.IsValid() && trimAt < optionsAt
	b.Links = linkNames(spec, op, links)
	b.Single = op.Method == "GET" && b.Query != "" && !next
	if data, err := requestData(spec, op.Request); attrsByValue && b.Attrs == "" && err == nil && !data.requires("attributes") {
		b.Attrs = attrsValue
	}
	for i, msg := range messages {
		if params := b.stringParams(); i >= len(params) || msg != params[i]+" is required" {
			b.Messages = messages
			break
		}
	}

	src, err := renderMethod(spec, fam, b)
	if err != nil {
		return nil, err.Error()
	}
	if diff := tokenDiff(d.src, []byte(src)); diff != "" {
		return nil, "differs at " + diff
	}
	return b, ""
}

// withVersionGoTypes returns the family with a Go type record for each type
// the generated method names after a versioned schema, such as
// ZebraV2UpdateRequest, when the hand-written method names the type of the
// same schema without its version instead, ZebraUpdateRequest, and never the
// versioned one.
func (p *pkg) withVersionGoTypes(spec *Spec, fam *Family, op *Operation, d *decl) *Family {
	trial := *fam
	trial.GoTypes = slices.Clone(fam.GoTypes)
	base, ok := strings.CutSuffix(op.Request, "Request")
	if !ok {
		return &trial
	}
	idents := map[string]bool{}
	for _, tok := range goTokens(d.src) {
		idents[tok] = true
	}
	for _, name := range []string{op.Request, base + "Data", base + "Attributes"} {
		unversioned := withoutVersion(name)
		typ := goTypeName(unversioned)
		if unversioned == name || trial.goType(name) != goTypeName(name) || slices.Contains(fam.Types, name) || !idents[typ] || idents[goTypeName(name)] {
			continue
		}
		if !sameShape(typeSchema(spec, name), typeSchema(spec, unversioned)) {
			continue
		}
		if strings.HasSuffix(name, "Attributes") && !p.carries(spec, typ, attributesOf(spec, name)) {
			continue
		}
		trial.GoTypes = append(trial.GoTypes, &GoType{Schema: name, Type: typ})
	}
	return &trial
}

// typeSchema is the schema a request, data or attributes type is named after:
// the request, the data object of the request, or the attributes object of
// the request or resource.
func typeSchema(spec *Spec, name string) *Schema {
	if base, ok := strings.CutSuffix(name, "Data"); ok {
		data, _ := requestData(spec, base+"Request")
		return data
	}
	if strings.HasSuffix(name, "Attributes") {
		return attributesOf(spec, name)
	}
	sch, _ := spec.Schema(name)
	return sch
}

// sameShape reports whether two schemas agree in everything the generator
// reads: type, enum values, required names, list items and properties, with
// references equal by name.
func sameShape(a, b *Schema) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Ref != "" || b.Ref != "" {
		return a.Ref == b.Ref
	}
	if a.Type != b.Type || !slices.Equal(a.Enum, b.Enum) || !sameNames(a.Required, b.Required) || !sameShape(a.Items, b.Items) || len(a.Properties) != len(b.Properties) {
		return false
	}
	for _, prop := range a.Properties {
		if other := b.Properties.Get(prop.Name); other == nil || !sameShape(prop.Schema, other) {
			return false
		}
	}
	return true
}

func sameNames(a, b []string) bool {
	return slices.Equal(slices.Sorted(slices.Values(a)), slices.Sorted(slices.Values(b)))
}

// carries reports whether an attributes type has exactly the attribute names
// of a schema: a hand-written struct by its JSON field names, a generated
// type by the schema it is named after.
func (p *pkg) carries(spec *Spec, typ string, attrs *Schema) bool {
	if attrs == nil {
		return false
	}
	var want, got []string
	for _, prop := range attrs.Properties {
		want = append(want, prop.Name)
	}
	if d, ok := p.types[typ]; ok {
		st, ok := d.node.(*ast.GenDecl).Specs[0].(*ast.TypeSpec).Type.(*ast.StructType)
		if !ok {
			return false
		}
		for _, f := range st.Fields.List {
			if len(f.Names) != 1 {
				return false
			}
			if !f.Names[0].IsExported() {
				continue
			}
			tag := ""
			if f.Tag != nil {
				tag, _ = strconv.Unquote(f.Tag.Value)
			}
			if name, _, _ := strings.Cut(reflect.StructTag(tag).Get("json"), ","); name != "-" {
				got = append(got, cmp.Or(name, f.Names[0].Name))
			}
		}
	} else if generated := attributesOf(spec, typ); generated != nil {
		for _, prop := range generated.Properties {
			got = append(got, prop.Name)
		}
	}
	slices.Sort(want)
	slices.Sort(got)
	return slices.Equal(want, got)
}

// recordResourceGoType records the Go type of a versioned resource's
// attributes when a bound read or write decodes it into a response type whose
// attributes are the type of the same resource without its version.
func (p *pkg) recordResourceGoType(spec *Spec, fam *Family, b *OpBind) {
	if g, _ := p.resourceGoType(spec, fam, b); g != nil {
		fam.GoTypes = append(fam.GoTypes, g)
	}
}

// resourceGoType is the record recordResourceGoType adds for a binding, or
// the reason it refuses one: the response holds the attributes type of the
// resource without its version, but the two schemas differ or the type
// carries other attributes. A method refused here is not adopted, because it
// would return attributes its operation does not define.
func (p *pkg) resourceGoType(spec *Spec, fam *Family, b *OpBind) (*GoType, string) {
	op, ok := spec.Operation(b.Operation)
	if !ok || b.Response == "" {
		return nil, ""
	}
	resource, _, ok := spec.resourceOf(op.Response)
	name := resource + "Attributes"
	if !ok || fam.goType(name) != goTypeName(name) || slices.Contains(fam.Types, name) {
		return nil, ""
	}
	attrs := p.responseAttributes(spec, b.Response)
	unversioned := withoutVersion(name)
	if unversioned == name || attrs != goTypeName(unversioned) {
		return nil, ""
	}
	if !sameShape(attributesOf(spec, name), attributesOf(spec, unversioned)) || !p.carries(spec, attrs, attributesOf(spec, name)) {
		return nil, fmt.Sprintf("result %s holds %s, which cannot stand for %s", b.Response, attrs, name)
	}
	return &GoType{Schema: name, Type: attrs}, ""
}

// responseAttributes names the attributes type of a Go response type: the
// type argument of a hand-written SingleResponse or Response alias, or the
// attributes of the resource a generated response schema holds.
func (p *pkg) responseAttributes(spec *Spec, response string) string {
	if d, ok := p.types[response]; ok {
		ts := d.node.(*ast.GenDecl).Specs[0].(*ast.TypeSpec)
		if index, ok := ts.Type.(*ast.IndexExpr); ok && ts.Assign.IsValid() {
			if generic := exprString(index.X); generic == "SingleResponse" || generic == "Response" {
				return exprString(index.Index)
			}
		}
		return ""
	}
	if resource, _, ok := spec.resourceOf(response); ok {
		return goTypeName(resource + "Attributes")
	}
	return ""
}

// linkNames maps the Go fields of Relationship literals a create sets, in
// body order, to the relationship names of its request.
func linkNames(spec *Spec, op *Operation, fields []string) []string {
	data, err := requestData(spec, op.Request)
	if err != nil || data.Properties.Get("relationships") == nil {
		return nil
	}
	var links []string
	for _, field := range fields {
		for _, rel := range data.Properties.Get("relationships").Properties {
			if goName(rel.Name) == field {
				links = append(links, rel.Name)
			}
		}
	}
	return links
}

// recordRelationships adds to the family the relationships field of the
// hand-written data type of each create with relationships it does not
// record yet.
func (p *pkg) recordRelationships(spec *Spec, ops []*Operation, fam *Family) {
	for _, op := range ops {
		data, err := requestData(spec, op.Request)
		name := strings.TrimSuffix(op.Request, "Request") + "Data"
		recorded := slices.ContainsFunc(fam.Relationships, func(r *RelBind) bool { return r.Data == name })
		if (op.Method != "POST" && op.Method != "PATCH") || err != nil || data.Properties.Get("relationships") == nil || recorded {
			continue
		}
		d, ok := p.types[goTypeName(name)]
		if !ok {
			continue
		}
		st, ok := d.node.(*ast.GenDecl).Specs[0].(*ast.TypeSpec).Type.(*ast.StructType)
		if !ok {
			continue
		}
		for _, f := range st.Fields.List {
			if len(f.Names) == 1 && f.Names[0].Name == "Relationships" && f.Tag != nil {
				tag, _ := strconv.Unquote(f.Tag.Value)
				fam.Relationships = append(fam.Relationships, &RelBind{Data: name, Type: exprString(f.Type), OmitEmpty: reflect.StructTag(tag).Get("json") == "relationships,omitempty"})
			}
		}
	}
}

// refTypesOf returns the family's records of referenced schemas plus the ones
// a hand-written attributes type declares: the Go type of each attribute that
// references a schema, without its list or pointer. A schema that a generated
// type or another field already gives another Go type keeps the type
// hand-written, because that choice is per field.
func refTypesOf(spec *Spec, fam *Family, fixed map[string]string, name string, d *decl) ([]*RefType, string) {
	refs := slices.Clone(fam.RefTypes)
	attrs := attributesOf(spec, name)
	st, ok := d.node.(*ast.GenDecl).Specs[0].(*ast.TypeSpec).Type.(*ast.StructType)
	if attrs == nil || !ok {
		return refs, ""
	}
	fields := map[string]string{}
	for _, f := range st.Fields.List {
		if f.Tag == nil {
			continue
		}
		tag, _ := strconv.Unquote(f.Tag.Value)
		jsonName, _, _ := strings.Cut(reflect.StructTag(tag).Get("json"), ",")
		fields[jsonName] = strings.TrimPrefix(strings.TrimPrefix(exprString(f.Type), "[]"), "*")
	}
	for _, prop := range attrs.Properties {
		schema, typ := refSchema(prop.Schema), fields[prop.Name]
		if schema == "" || typ == "" {
			continue
		}
		current := (&Family{RefTypes: refs}).refType(schema)
		if want, ok := fixed[schema]; ok && want != typ {
			return nil, fmt.Sprintf("schema %s is %s here and %s in a generated type", schema, typ, want)
		}
		if typ == current {
			continue
		}
		if current != goTypeName(schema) {
			return nil, fmt.Sprintf("schema %s is %s here and %s in another field", schema, typ, current)
		}
		r := &RefType{Schema: schema, Type: typ}
		if err := checkRefType(spec, r); err != nil {
			return nil, err.Error()
		}
		refs = append(refs, r)
	}
	return refs, ""
}

// fixRefTypes adds the Go type of each schema a generated type references.
func fixRefTypes(spec *Spec, fam *Family, name string, fixed map[string]string) {
	attrs := attributesOf(spec, name)
	if attrs == nil {
		return
	}
	for _, prop := range attrs.Properties {
		if schema := refSchema(prop.Schema); schema != "" {
			fixed[schema] = fam.refType(schema)
		}
	}
}

// attributesOf returns the attributes schema an <X>Attributes type renders
// from, or nil.
func attributesOf(spec *Spec, name string) *Schema {
	base, ok := strings.CutSuffix(name, "Attributes")
	if !ok {
		return nil
	}
	if data, err := requestData(spec, base+"Request"); err == nil {
		return data.Properties.Get("attributes")
	}
	if resource, ok := spec.Schema(base); ok {
		return resource.Properties.Get("attributes")
	}
	return nil
}

// refSchema names the schema an attribute or its list items reference, or "".
func refSchema(s *Schema) string {
	if s.Type == "array" && s.Items != nil {
		s = s.Items
	}
	if s.Ref == "" {
		return ""
	}
	return refName(s.Ref)
}

// optionalIf is an optional link a create or update sets: the parameter, the
// Go field of the relationships it sets, and whether it allocates the
// relationships first.
type optionalIf struct {
	param, field string
	whenSet      bool
}

// readOptionalLink reads `if id := strings.TrimSpace(p); id != "" {...}` or
// `if p != "" {...}` whose body sets one relationships field to a
// Relationship, after allocating the relationships when they are nil.
func readOptionalLink(x ast.Node) (optionalIf, bool) {
	n, ok := x.(*ast.IfStmt)
	if !ok || n.Else != nil || len(n.Body.List) == 0 {
		return optionalIf{}, false
	}
	o := optionalIf{}
	cond := exprString(n.Cond)
	if init, ok := n.Init.(*ast.AssignStmt); ok && init.Tok == token.DEFINE && len(init.Rhs) == 1 && cond == `id != ""` && exprString(init.Lhs[0]) == "id" {
		call, ok := init.Rhs[0].(*ast.CallExpr)
		if !ok || exprString(call.Fun) != "strings.TrimSpace" || len(call.Args) != 1 {
			return optionalIf{}, false
		}
		o.param = exprString(call.Args[0])
	} else if p, ok := strings.CutSuffix(cond, ` != ""`); ok && n.Init == nil && !strings.ContainsAny(p, " .()") {
		o.param = p
	} else {
		return optionalIf{}, false
	}
	stmts := n.Body.List
	if first, ok := stmts[0].(*ast.IfStmt); ok && exprString(first.Cond) == "relationships == nil" && len(stmts) == 2 {
		o.whenSet, stmts = true, stmts[1:]
	}
	set, ok := stmts[0].(*ast.AssignStmt)
	if !ok || len(stmts) != 1 || len(set.Lhs) != 1 {
		return optionalIf{}, false
	}
	field, ok := strings.CutPrefix(exprString(set.Lhs[0]), "relationships.")
	if lit := compositeLit(set.Rhs[0]); !ok || lit == nil || exprString(lit.Type) != "Relationship" {
		return optionalIf{}, false
	}
	o.field = field
	return o, true
}

// compositeLit returns the literal of T{...} or &T{...}.
func compositeLit(expr ast.Expr) *ast.CompositeLit {
	if u, ok := expr.(*ast.UnaryExpr); ok && u.Op == token.AND {
		expr = u.X
	}
	lit, _ := expr.(*ast.CompositeLit)
	return lit
}

// readIDList records what a node does with the ID list of a relationship
// change. It returns true for the empty list check, whose message is not a
// path parameter message.
func readIDList(x ast.Node, ids string, l *IDList) bool {
	switch n := x.(type) {
	case *ast.AssignStmt:
		if call, ok := n.Rhs[0].(*ast.CallExpr); ok && exprString(call.Fun) == "normalizeList" && exprString(n.Lhs[0]) == ids {
			l.Normalize = true
		}
	case *ast.IfStmt:
		if exprString(n.Cond) != "len("+ids+") == 0" {
			return false
		}
		ast.Inspect(n.Body, func(y ast.Node) bool {
			if call, ok := y.(*ast.CallExpr); ok && exprString(call.Fun) == "fmt.Errorf" && len(call.Args) == 1 {
				l.Message = stringLit(call.Args[0])
			}
			return true
		})
		return true
	case *ast.RangeStmt:
		if exprString(n.X) != ids || n.Value == nil {
			return false
		}
		if exprString(n.Key) != "_" {
			l.Payload = payloadIndex
			return false
		}
		l.Item = exprString(n.Value)
		if l.Payload == payloadDefault {
			l.Payload = payloadAppend
		}
	case *ast.CompositeLit:
		switch exprString(n.Type) {
		case "RelationshipList":
			l.Payload = payloadResourceList
		case "[]RelationshipData":
			l.Payload = payloadNonNil
		}
	}
	return false
}

// attributeArgs maps each identifier an attributes literal sets a field to,
// by value or by address, to the field name.
func attributeArgs(body *ast.BlockStmt) map[string]string {
	out := map[string]string{}
	ast.Inspect(body, func(x ast.Node) bool {
		kv, ok := x.(*ast.KeyValueExpr)
		if !ok || exprString(kv.Key) != "Attributes" {
			return true
		}
		value := kv.Value
		if u, ok := value.(*ast.UnaryExpr); ok && u.Op == token.AND {
			value = u.X
		}
		lit, ok := value.(*ast.CompositeLit)
		if !ok {
			return true
		}
		for _, elt := range lit.Elts {
			field, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			arg := field.Value
			if u, ok := arg.(*ast.UnaryExpr); ok && u.Op == token.AND {
				arg = u.X
			}
			if id, ok := arg.(*ast.Ident); ok {
				out[id.Name] = exprString(field.Key)
			}
		}
		return true
	})
	return out
}

// deriveArg binds a parameter set as the request attribute whose Go field
// name is field.
func deriveArg(spec *Spec, op *Operation, name, field, typ string) (ArgBind, string) {
	data, err := requestData(spec, op.Request)
	if err != nil {
		return ArgBind{}, err.Error()
	}
	attrs := data.Properties.Get("attributes")
	if attrs == nil {
		return ArgBind{}, op.Request + " has no attributes"
	}
	for _, prop := range attrs.Properties {
		if goName(prop.Name) != field {
			continue
		}
		arg := ArgBind{Name: name, Type: typ}
		if prop.Name != name {
			arg.Attribute = prop.Name
		}
		if def, _, err := goType(prop.Schema); err == nil && def == typ {
			arg.Type = ""
		}
		return arg, ""
	}
	return ArgBind{}, fmt.Sprintf("field %s is no attribute of %s", field, op.Request)
}

func stringLit(expr ast.Expr) string {
	if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		if s, err := strconv.Unquote(lit.Value); err == nil {
			return s
		}
	}
	return ""
}

// typeCandidates names the types the operations of a tag read and write.
func typeCandidates(spec *Spec, ops []*Operation) []string {
	seen := map[string]bool{}
	var out []string
	add := func(name string) {
		if name != "" && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	for _, op := range ops {
		action := op.ID[strings.LastIndex(op.ID, "_")+1:]
		switch action {
		case "getCollection", "getInstance", "createInstance", "updateInstance":
			if resource, _, ok := spec.resourceOf(op.Response); ok {
				add(resource + "Attributes")
				add(op.Response)
			}
		}
		if base, ok := strings.CutSuffix(op.Request, "Request"); ok && (action == "createInstance" || action == "updateInstance") {
			add(op.Request)
			add(base + "Data")
			add(base + "Attributes")
		}
	}
	sort.Strings(out)
	return out
}

// tokenDiff compares two sources as Go tokens, ignoring comments, layout,
// optional commas and semicolons, string quoting, and whether an HTTP method is
// written as http.MethodX or "X". It returns "" when they are the same.
func tokenDiff(a, b []byte) string {
	ta, tb := goTokens(a), goTokens(b)
	for i := range min(len(ta), len(tb)) {
		if ta[i] != tb[i] {
			return fmt.Sprintf("token %d: %q vs generated %q", i, ta[i], tb[i])
		}
	}
	if len(ta) != len(tb) {
		return fmt.Sprintf("length %d vs generated %d", len(ta), len(tb))
	}
	return ""
}

func goTokens(src []byte) []string {
	var s scanner.Scanner
	fset := token.NewFileSet()
	s.Init(fset.AddFile("", -1, len(src)), src, nil, 0)
	var out []string
	for {
		_, tok, lit := s.Scan()
		switch tok {
		case token.EOF:
			return out
		case token.SEMICOLON:
			continue
		case token.RBRACE, token.RPAREN, token.RBRACK:
			if n := len(out); n > 0 && out[n-1] == "," {
				out = out[:n-1]
			}
		case token.STRING:
			if v, err := strconv.Unquote(lit); err == nil {
				lit = strconv.Quote(v)
			}
		case token.IDENT:
			if n := len(out); n >= 2 && out[n-1] == "." && out[n-2] == "http" && strings.HasPrefix(lit, "Method") {
				out = append(out[:n-2], strconv.Quote(strings.ToUpper(strings.TrimPrefix(lit, "Method"))))
				continue
			}
		}
		if lit != "" {
			out = append(out, lit)
		} else {
			out = append(out, tok.String())
		}
	}
}

// edits deletes adopted declarations with their doc comments and drops
// imports nothing uses any more, returning the new file contents.
func (p *pkg) edits() (map[string][]byte, error) {
	out := map[string][]byte{}
	for _, pf := range p.files {
		if len(pf.removed) == 0 {
			continue
		}
		src := pf.src
		sort.Slice(pf.removed, func(i, j int) bool { return pf.removed[i].Pos() > pf.removed[j].Pos() })
		for _, node := range pf.removed {
			start := node.Pos()
			switch n := node.(type) {
			case *ast.FuncDecl:
				if n.Doc != nil {
					start = n.Doc.Pos()
				}
			case *ast.GenDecl:
				if n.Doc != nil {
					start = n.Doc.Pos()
				}
			}
			from, to := p.fset.Position(start).Offset, p.fset.Position(node.End()).Offset
			src = append(src[:from:from], src[to:]...)
		}
		edited, err := pruneImports(src)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", pf.path, err)
		}
		out[pf.path] = edited
	}
	return out, nil
}

// pruneImports removes unused imports and formats the file. It returns nil
// when the file has nothing left but its package clause and imports.
func pruneImports(src []byte) ([]byte, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	used := map[string]bool{}
	ast.Inspect(f, func(x ast.Node) bool {
		if sel, ok := x.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok {
				used[id.Name] = true
			}
		}
		return true
	})
	var cut [][2]int
	other := false
	for _, d := range f.Decls {
		gen, ok := d.(*ast.GenDecl)
		if !ok || gen.Tok != token.IMPORT {
			other = true
			continue
		}
		var unused []ast.Spec
		for _, s := range gen.Specs {
			spec := s.(*ast.ImportSpec)
			path, _ := strconv.Unquote(spec.Path.Value)
			name := path[strings.LastIndex(path, "/")+1:]
			if spec.Name != nil {
				name = spec.Name.Name
			}
			if name != "_" && !used[name] {
				unused = append(unused, s)
			}
		}
		if len(unused) == len(gen.Specs) {
			cut = append(cut, [2]int{fset.Position(gen.Pos()).Offset, fset.Position(gen.End()).Offset})
			continue
		}
		for _, s := range unused {
			cut = append(cut, wholeLines(src, fset.Position(s.Pos()).Offset, fset.Position(s.End()).Offset))
		}
	}
	if !other {
		return nil, nil
	}
	for i := len(cut) - 1; i >= 0; i-- {
		src = append(src[:cut[i][0]:cut[i][0]], src[cut[i][1]:]...)
	}
	return format.Source(src)
}

// wholeLines widens the span from:to to the lines it is on when nothing else
// is on them, so a removed import leaves no blank line that would split its
// import group.
func wholeLines(src []byte, from, to int) [2]int {
	start := bytes.LastIndexByte(src[:from], '\n') + 1
	end := bytes.IndexByte(src[to:], '\n')
	if end < 0 || len(bytes.TrimSpace(src[start:from])) > 0 || len(bytes.TrimSpace(src[to:to+end])) > 0 {
		return [2]int{from, to}
	}
	return [2]int{start, to + end + 1}
}
