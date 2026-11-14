package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Bindings lists what is generated and the Go-side choices the spec cannot
// supply, one file per family in a bindings directory. `-adopt` writes them
// from existing code; nobody edits them by hand.
type Bindings struct {
	Families []*Family
}

// Family is one spec tag generated into one file.
type Family struct {
	Tag           string     `json:"tag"`
	File          string     `json:"file"`
	Types         []string   `json:"types,omitempty"`
	Relationships []*RelBind `json:"relationships,omitempty"`
	Operations    []*OpBind  `json:"operations,omitempty"`
}

// RelBind is the relationships field of a create's data type, as the
// hand-written data type declares it. The data type, the relationships type
// it names and the create method all render from it.
type RelBind struct {
	Data      string `json:"data"`                // data type, such as "AppEventCreateData"
	Type      string `json:"type"`                // Go type of the field; "*R" for a pointer
	OmitEmpty bool   `json:"omitempty,omitempty"` // the field's tag has omitempty
}

// Check styles for path parameters.
const (
	checkInline  = ""        // strings.TrimSpace inside the path expression
	checkTrim    = "trim"    // trim the parameter before use
	checkRequire = "require" // trim, then reject a blank value
	checkRaw     = "raw"     // use the parameter as given
)

// OpBind binds one spec operation to one client method. Empty fields take the
// default rule documented next to each field.
type OpBind struct {
	Operation string `json:"operation"`
	Name      string `json:"name"`
	// Params are the Go names of the path parameters in path order, then of
	// the IDs a create links.
	Params []string `json:"params,omitempty"`
	Check  string   `json:"check,omitempty"` // checkInline, checkTrim, checkRequire or checkRaw
	// CheckFirst runs the trim and require checks of a paged read before its
	// options, so a next URL does not lift the require check.
	CheckFirst bool `json:"checkFirst,omitempty"`
	// Messages are the errors for blank parameters; default "<param> is required".
	Messages    []string `json:"messages,omitempty"`
	Options     string   `json:"options,omitempty"`     // option type of a paged read
	Query       string   `json:"query,omitempty"`       // query builder of a paged read
	NextContext string   `json:"nextContext,omitempty"` // prefix of a rejected next URL error
	ParseErr    string   `json:"parseErr,omitempty"`    // default "failed to parse response"
	Response    string   `json:"response,omitempty"`    // Go decode type; default the spec schema name
	IDs         string   `json:"ids,omitempty"`         // ID list parameter of a relationship change
	// Links are the to-one relationships a create sets, one for each of the
	// last len(Links) Params.
	Links []string `json:"links,omitempty"`
	// Deviations are known differences from the schema that the contract test
	// in internal/asc pins. They change no generated code.
	Deviations []Deviation `json:"deviations,omitempty"`
}

// Deviation is one problem text of the contract test that the method is known
// to produce, and why it is kept.
type Deviation struct {
	Problem string `json:"problem"`
	Reason  string `json:"reason"`
}

// LoadBindings reads every family file in a bindings directory. A missing
// directory holds no families.
func LoadBindings(dir string) (*Bindings, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return &Bindings{}, nil
	}
	if err != nil {
		return nil, err
	}
	b := &Bindings{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var f Family
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		if bindingsFile(dir, f.Tag) != path {
			return nil, fmt.Errorf("%s: holds tag %q, so it must be named %s.json", path, f.Tag, f.Tag)
		}
		b.Families = append(b.Families, &f)
	}
	return b, nil
}

// bindingsFile is the file that holds the bindings of a tag.
func bindingsFile(dir, tag string) string {
	return filepath.Join(dir, tag+".json")
}

// Encode returns the family file in a stable order, one operation or type per
// line so that a diff shows each binding that changed.
func (f *Family) Encode() ([]byte, error) {
	sort.Strings(f.Types)
	sort.Slice(f.Relationships, func(i, j int) bool { return f.Relationships[i].Data < f.Relationships[j].Data })
	sort.Slice(f.Operations, func(i, j int) bool { return f.Operations[i].Operation < f.Operations[j].Operation })
	fields := []string{`"tag": ` + jsonString(f.Tag), `"file": ` + jsonString(f.File)}
	if len(f.Types) > 0 {
		var types []string
		for _, t := range f.Types {
			types = append(types, jsonString(t))
		}
		fields = append(fields, "\"types\": [\n    "+strings.Join(types, ",\n    ")+"\n  ]")
	}
	rels, err := jsonLines(f.Relationships)
	if err != nil {
		return nil, err
	}
	ops, err := jsonLines(f.Operations)
	if err != nil {
		return nil, err
	}
	if len(rels) > 0 {
		fields = append(fields, "\"relationships\": [\n    "+strings.Join(rels, ",\n    ")+"\n  ]")
	}
	if len(ops) > 0 {
		fields = append(fields, "\"operations\": [\n    "+strings.Join(ops, ",\n    ")+"\n  ]")
	}
	return []byte("{\n  " + strings.Join(fields, ",\n  ") + "\n}\n"), nil
}

// jsonLines encodes each item as one line of JSON.
func jsonLines[T any](items []T) ([]string, error) {
	var lines []string
	for _, item := range items {
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(item); err != nil {
			return nil, err
		}
		lines = append(lines, strings.TrimSpace(buf.String()))
	}
	return lines, nil
}

func jsonString(s string) string {
	data, _ := json.Marshal(s)
	return string(data)
}

func (b *Bindings) produces(dir, path string) bool {
	for _, f := range b.Families {
		if filepath.Join(dir, f.File) == path {
			return true
		}
	}
	return false
}

// relationshipsOf returns the relationships record of a data type. A record
// must name a type such as "R" or "*R".
func (f *Family) relationshipsOf(data string) (*RelBind, error) {
	for _, r := range f.Relationships {
		if r.Data != data {
			continue
		}
		if !typeName.MatchString(r.Type) {
			return nil, fmt.Errorf("relationships of %s have type %q, not a type name", data, r.Type)
		}
		return r, nil
	}
	return nil, fmt.Errorf("no relationships record for %s", data)
}

var typeName = regexp.MustCompile(`^\*?[A-Z][A-Za-z0-9]*$`)

// relationshipsNamed returns the record whose field has the named Go type,
// or nil.
func (f *Family) relationshipsNamed(name string) *RelBind {
	for _, r := range f.Relationships {
		if strings.TrimPrefix(r.Type, "*") == name {
			return r
		}
	}
	return nil
}

// Family returns the family for a tag, adding it if missing.
func (b *Bindings) Family(tag, file string) *Family {
	for _, f := range b.Families {
		if f.Tag == tag {
			return f
		}
	}
	f := &Family{Tag: tag, File: file}
	b.Families = append(b.Families, f)
	return f
}
