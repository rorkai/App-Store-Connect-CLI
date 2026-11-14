package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/scanner"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

var httpMethods = map[string]bool{"GET": true, "POST": true, "PATCH": true, "DELETE": true, "PUT": true}

// pkgFile is one hand-written file of the client package.
type pkgFile struct {
	path    string
	src     []byte
	removed []ast.Node
}

type pkg struct {
	fset    *token.FileSet
	files   []*pkgFile
	methods map[string][]*decl // by HTTP method and path literal, such as "GET /v1/apps/%s"
	types   map[string]*decl
}

type decl struct {
	file    *pkgFile
	node    ast.Node // *ast.FuncDecl or single-spec *ast.GenDecl
	name    string
	src     []byte // from the func or type keyword to the end, without the doc comment
	adopted bool
}

func (d *decl) adopt() {
	d.adopted = true
	d.file.removed = append(d.file.removed, d.node)
}

// Adopt finds the hand-written methods and types of the given tags whose
// source equals what the generator renders, adds their bindings, and reports
// every operation and type it leaves hand-written, with the reason. It writes
// nothing: it returns the new contents of each hand-written file it would
// change, nil for a file left empty.
func Adopt(spec *Spec, b *Bindings, dir string, tags []string, report io.Writer) (map[string][]byte, error) {
	p, err := loadPackage(dir)
	if err != nil {
		return nil, err
	}
	bound := map[string]bool{}
	for _, f := range b.Families {
		for _, op := range f.Operations {
			bound[op.Operation] = true
		}
	}
	for _, tag := range tags {
		ops := spec.Operations(tag)
		if len(ops) == 0 {
			return nil, fmt.Errorf("no operations tagged %s", tag)
		}
		var adoptedOps []*OpBind
		var adoptedTypes []string
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
				bind, reason := deriveOp(spec, op, d)
				if reason != "" {
					fmt.Fprintf(report, "%s\t%s\t%s\tkept: %s\n", tag, op.ID, d.name, reason)
					continue
				}
				d.adopt()
				adoptedOps = append(adoptedOps, bind)
				fmt.Fprintf(report, "%s\t%s\t%s\tgenerated\n", tag, op.ID, d.name)
			}
		}
		for _, name := range typeCandidates(spec, ops) {
			d, ok := p.types[name]
			if !ok || d.adopted {
				continue
			}
			src, err := renderType(spec, name)
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
			fmt.Fprintf(report, "%s\ttype %s\tgenerated\n", tag, name)
		}
		if len(adoptedOps)+len(adoptedTypes) > 0 {
			fam := b.Family(tag, fileName(tag))
			fam.Operations = append(fam.Operations, adoptedOps...)
			fam.Types = append(fam.Types, adoptedTypes...)
		}
	}
	return p.edits()
}

func loadPackage(dir string) (*pkg, error) {
	p := &pkg{fset: token.NewFileSet(), methods: map[string][]*decl{}, types: map[string]*decl{}}
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
		pf := &pkgFile{path: path, src: src}
		p.files = append(p.files, pf)
		for _, node := range f.Decls {
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
		d := &decl{file: pf, node: n, name: n.Name.Name, src: p.slice(pf, n.Pos(), n.End())}
		var paths, verbs []string
		ast.Inspect(n.Body, func(x ast.Node) bool {
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
func deriveOp(spec *Spec, op *Operation, d *decl) (*OpBind, string) {
	fn := d.node.(*ast.FuncDecl)
	b := &OpBind{Operation: op.ID, Name: fn.Name.Name}
	for i, field := range fn.Type.Params.List {
		typ := exprString(field.Type)
		switch {
		case i == 0 && typ == "context.Context":
		case typ == "string":
			for _, n := range field.Names {
				b.Params = append(b.Params, n.Name)
			}
		case typ == "[]string" && len(field.Names) == 1:
			b.IDs = field.Names[0].Name
		case strings.HasPrefix(typ, "..."):
			b.Options = strings.TrimPrefix(typ, "...")
		case strings.HasSuffix(typ, "Attributes"):
		default:
			return nil, "parameter type " + typ
		}
	}

	var messages []string
	ast.Inspect(fn.Body, func(x ast.Node) bool {
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
			case "NextContext":
				b.NextContext = stringLit(n.Value)
			case "ParseErr":
				b.ParseErr = stringLit(n.Value)
			}
		case *ast.IndexExpr:
			if fun := exprString(n.X); (fun == "getJSON" || fun == "sendJSON") && exprString(n.Index) != op.Response {
				b.Response = exprString(n.Index)
			}
		case *ast.AssignStmt:
			if call, ok := n.Rhs[0].(*ast.CallExpr); ok && exprString(call.Fun) == "strings.TrimSpace" && b.Check == checkInline {
				b.Check = checkTrim
			}
		case *ast.CallExpr:
			if exprString(n.Fun) == "fmt.Errorf" && len(n.Args) == 1 {
				messages = append(messages, stringLit(n.Args[0]))
				b.Check = checkRequire
			}
		}
		return true
	})
	for i, msg := range messages {
		if i >= len(b.Params) || msg != b.Params[i]+" is required" {
			b.Messages = messages
			break
		}
	}

	src, err := renderMethod(spec, b)
	if err != nil {
		return nil, err.Error()
	}
	if diff := tokenDiff(d.src, []byte(src)); diff != "" {
		return nil, "differs at " + diff
	}
	return b, ""
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
			cut = append(cut, [2]int{fset.Position(s.Pos()).Offset, fset.Position(s.End()).Offset})
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
