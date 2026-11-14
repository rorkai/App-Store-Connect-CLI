package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

// The web target generates parts of the web interface client in internal/web
// from docs/openapi/web/latest.json. That schema was derived from the client,
// so the target renders the client's own request idiom and adopts a
// hand-written method only when it renders the same tokens.

const webHeader = generatedPrefix + "from the App Store Connect web interface schema; DO NOT EDIT."

// webIrisV1 is the path prefix of Client.baseURL, which NewClient sets and
// c.doRequest sends to.
const webIrisV1 = "/iris/v1"

// webSpec is the part of the web interface schema the web target reads.
type webSpec struct {
	ops map[string]*webOperation
}

// webOperation is one HTTP method on one path of the web interface schema.
type webOperation struct {
	ID             string
	Tag            string
	Method         string // upper case
	Path           string // template with its full prefix, such as /iris/v1/apps/{id}
	Body           bool   // has a request body
	Response       bool   // has a 2XX response body the client reads
	Sources        []string
	HandWritten    bool
	NotImplemented bool
}

func loadWebSpec(path string) (*webSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	spec := &webSpec{ops: map[string]*webOperation{}}
	for path, methods := range doc.Paths {
		for method, raw := range methods {
			switch method {
			case "get", "post", "patch", "delete", "put", "head":
			default:
				continue
			}
			var op struct {
				ID          string   `json:"operationId"`
				Tags        []string `json:"tags"`
				RequestBody any      `json:"requestBody"`
				Responses   map[string]struct {
					Content map[string]any `json:"content"`
				} `json:"responses"`
				Sources        []string `json:"x-asc-source"`
				HandWritten    bool     `json:"x-asc-hand-written"`
				NotImplemented bool     `json:"x-asc-not-implemented"`
			}
			if err := json.Unmarshal(raw, &op); err != nil {
				return nil, fmt.Errorf("parse %s %s: %w", method, path, err)
			}
			out := &webOperation{
				ID:             op.ID,
				Method:         strings.ToUpper(method),
				Path:           path,
				Body:           op.RequestBody != nil,
				Response:       len(op.Responses["2XX"].Content) > 0,
				Sources:        op.Sources,
				HandWritten:    op.HandWritten,
				NotImplemented: op.NotImplemented,
			}
			if len(op.Tags) > 0 {
				out.Tag = op.Tags[0]
			}
			spec.ops[out.ID] = out
		}
	}
	return spec, nil
}

// Operations returns every operation of a tag, sorted by operationId.
func (s *webSpec) Operations(tag string) []*webOperation {
	var out []*webOperation
	for _, op := range s.ops {
		if op.Tag == tag {
			out = append(out, op)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Tags returns every tag that has operations, sorted.
func (s *webSpec) Tags() []string {
	seen := map[string]bool{}
	var out []string
	for _, op := range s.ops {
		if !seen[op.Tag] {
			seen[op.Tag] = true
			out = append(out, op.Tag)
		}
	}
	sort.Strings(out)
	return out
}

func runWeb(specPath, bindingsDir, dir string, check bool, adopt string, dryRun bool, report io.Writer) error {
	spec, err := loadWebSpec(specPath)
	if err != nil {
		return err
	}
	bindings, err := LoadBindings(bindingsDir)
	if err != nil {
		return err
	}
	planFiles := func() (map[string][]byte, []string, error) {
		return planTarget(bindings, dir, webHeader, func(fam *Family) ([]byte, error) { return GenerateWeb(spec, fam) })
	}
	if adopt == "" {
		writes, stale, err := planFiles()
		return writeOrCheck(writes, stale, err, check)
	}
	if check {
		return fmt.Errorf("-adopt and -check cannot be combined")
	}
	tags := strings.Split(adopt, ",")
	if adopt == "all" {
		tags = spec.Tags()
	}
	edits, adopted, err := AdoptWeb(spec, bindings, dir, tags, report)
	if err != nil || dryRun {
		return err
	}
	return writeAdopted(edits, adopted, bindingsDir, planFiles)
}

// GenerateWeb renders the Go file of one web family.
func GenerateWeb(spec *webSpec, fam *Family) ([]byte, error) {
	if len(fam.Types) > 0 {
		return nil, fmt.Errorf("%s: the web target generates no types", fam.Tag)
	}
	var body bytes.Buffer
	for _, b := range fam.Operations {
		src, err := renderWebMethod(spec, b)
		if err != nil {
			return nil, fmt.Errorf("%s: %s: %w", fam.Tag, b.Operation, err)
		}
		body.WriteString(src)
	}
	var out bytes.Buffer
	out.WriteString(webHeader + "\n\npackage web\n\n")
	var imports []string
	for _, imp := range []struct{ path, use string }{
		{"context", "context."}, {"fmt", "fmt."}, {"net/http", "http."}, {"net/url", "url."}, {"strings", "strings."},
	} {
		if strings.Contains(body.String(), imp.use) {
			imports = append(imports, strconv.Quote(imp.path))
		}
	}
	if len(imports) > 0 {
		out.WriteString("import (\n" + strings.Join(imports, "\n") + "\n)\n\n")
	}
	out.Write(body.Bytes())
	return format.Source(out.Bytes())
}

// renderWebMethod renders a delete without a request or response body, sent
// through c.doRequest: trim and require each path parameter, then send.
func renderWebMethod(spec *webSpec, b *OpBind) (string, error) {
	op, ok := spec.ops[b.Operation]
	switch {
	case !ok:
		return "", fmt.Errorf("operation not in schema")
	case op.HandWritten:
		return "", fmt.Errorf("operation is marked x-asc-hand-written")
	case op.NotImplemented:
		return "", fmt.Errorf("operation is marked x-asc-not-implemented")
	case len(b.Params) != strings.Count(op.Path, "{"):
		return "", fmt.Errorf("%d params for path %s", len(b.Params), op.Path)
	}
	path, ok := strings.CutPrefix(op.Path, webIrisV1+"/")
	if !ok {
		return "", fmt.Errorf("path %s is not under %s, where c.doRequest sends", op.Path, webIrisV1)
	}
	if op.Method != "DELETE" || op.Body || op.Response {
		return "", fmt.Errorf("the web target renders only a DELETE without request or response body, not %s with body %t and response %t", op.Method, op.Body, op.Response)
	}
	if b.Check != checkRequire {
		return "", fmt.Errorf("the web target renders only the %q check style, not %q", checkRequire, b.Check)
	}

	var out strings.Builder
	fmt.Fprintf(&out, "// %s sends %s %s (%s).\n", b.Name, op.Method, op.Path, op.ID)
	fmt.Fprintf(&out, "func (c *Client) %s(ctx context.Context, %s string) error {\n", b.Name, strings.Join(b.Params, ", "))
	for _, p := range b.Params {
		fmt.Fprintf(&out, "%s = strings.TrimSpace(%s)\n", p, p)
	}
	for i, p := range b.Params {
		msg := p + " is required"
		if i < len(b.Messages) {
			msg = b.Messages[i]
		}
		fmt.Fprintf(&out, "if %s == \"\" {\nreturn fmt.Errorf(%s)\n}\n", p, strconv.Quote(msg))
	}
	fmt.Fprintf(&out, "_, err := c.doRequest(ctx, http.Method%s, %s, nil)\nreturn err\n}\n\n", methodConst(op.Method), webPath("/"+path, b.Params))
	return out.String(), nil
}

// webPath joins the literal parts of a path template with the escaped path
// parameters: "/apps/" + url.PathEscape(appID) + "/dataUsages".
func webPath(template string, params []string) string {
	var parts []string
	for _, p := range params {
		start, end := strings.Index(template, "{"), strings.Index(template, "}")
		if start > 0 {
			parts = append(parts, strconv.Quote(template[:start]))
		}
		parts = append(parts, "url.PathEscape("+p+")")
		template = template[end+1:]
	}
	if template != "" {
		parts = append(parts, strconv.Quote(template))
	}
	return strings.Join(parts, "+")
}

// AdoptWeb is Adopt for the web target. The schema names the methods that
// send each operation (x-asc-source), so those are the candidates.
func AdoptWeb(spec *webSpec, b *Bindings, dir string, tags []string, report io.Writer) (map[string][]byte, []*Family, error) {
	p, err := loadPackage(dir)
	if err != nil {
		return nil, nil, err
	}
	bound := map[string]bool{}
	for _, f := range b.Families {
		for _, op := range f.Operations {
			bound[op.Operation] = true
		}
	}
	var adopted []*Family
	for _, tag := range tags {
		ops := spec.Operations(tag)
		if len(ops) == 0 {
			return nil, nil, fmt.Errorf("no operations tagged %s", tag)
		}
		var adoptedOps []*OpBind
		for _, op := range ops {
			switch {
			case bound[op.ID]:
				fmt.Fprintf(report, "%s\t%s\talready generated\n", tag, op.ID)
				continue
			case op.HandWritten:
				fmt.Fprintf(report, "%s\t%s\tnot generated: x-asc-hand-written\n", tag, op.ID)
				continue
			case op.NotImplemented:
				fmt.Fprintf(report, "%s\t%s\tnot generated: x-asc-not-implemented\n", tag, op.ID)
				continue
			}
			for _, source := range op.Sources {
				name, ok := strings.CutPrefix(source, "Client.")
				d := p.names[name]
				if !ok || d == nil {
					fmt.Fprintf(report, "%s\t%s\t%s\tkept: not a *Client method\n", tag, op.ID, source)
					continue
				}
				if d.adopted {
					continue
				}
				bind, reason := deriveWebOp(spec, op, d)
				if reason != "" {
					fmt.Fprintf(report, "%s\t%s\t%s\tkept: %s\n", tag, op.ID, d.name, reason)
					continue
				}
				d.adopt()
				adoptedOps = append(adoptedOps, bind)
				fmt.Fprintf(report, "%s\t%s\t%s\tgenerated\n", tag, op.ID, d.name)
			}
		}
		if len(adoptedOps) > 0 {
			fam := b.Family(tag, fileName(tag))
			fam.Operations = append(fam.Operations, adoptedOps...)
			adopted = append(adopted, fam)
		}
	}
	edits, err := p.edits()
	return edits, adopted, err
}

// deriveWebOp reads the binding choices out of a hand-written web method,
// renders the method from them, and accepts the binding only when the two are
// the same tokens.
func deriveWebOp(spec *webSpec, op *webOperation, d *decl) (*OpBind, string) {
	fn := d.node.(*ast.FuncDecl)
	b := &OpBind{Operation: op.ID, Name: fn.Name.Name}
	for i, field := range fn.Type.Params.List {
		switch typ := exprString(field.Type); {
		case i == 0 && typ == "context.Context":
		case typ == "string":
			for _, n := range field.Names {
				b.Params = append(b.Params, n.Name)
			}
		default:
			return nil, "parameter type " + typ
		}
	}
	var messages []string
	ast.Inspect(fn.Body, func(x ast.Node) bool {
		if call, ok := x.(*ast.CallExpr); ok && exprString(call.Fun) == "fmt.Errorf" && len(call.Args) == 1 {
			messages = append(messages, stringLit(call.Args[0]))
			b.Check = checkRequire
		}
		return true
	})
	for i, msg := range messages {
		if i >= len(b.Params) || msg != b.Params[i]+" is required" {
			b.Messages = messages
			break
		}
	}
	src, err := renderWebMethod(spec, b)
	if err != nil {
		return nil, err.Error()
	}
	if diff := tokenDiff(d.src, []byte(src)); diff != "" {
		return nil, "differs at " + diff
	}
	return b, ""
}
