package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	Tag        string    `json:"tag"`
	File       string    `json:"file"`
	Types      []string  `json:"types,omitempty"`
	Operations []*OpBind `json:"operations,omitempty"`
}

// Check styles for path parameters.
const (
	checkInline  = ""        // strings.TrimSpace inside the path expression
	checkTrim    = "trim"    // trim the parameter before use
	checkRequire = "require" // trim, then reject a blank value
)

// OpBind binds one spec operation to one client method. Empty fields take the
// default rule documented next to each field.
type OpBind struct {
	Operation string   `json:"operation"`
	Name      string   `json:"name"`
	Params    []string `json:"params,omitempty"` // Go names of the path parameters, in path order
	Check     string   `json:"check,omitempty"`  // checkInline, checkTrim or checkRequire
	// Messages are the errors for blank parameters; default "<param> is required".
	Messages    []string `json:"messages,omitempty"`
	Options     string   `json:"options,omitempty"`     // option type of a paged read
	Query       string   `json:"query,omitempty"`       // query builder of a paged read
	NextContext string   `json:"nextContext,omitempty"` // prefix of a rejected next URL error
	ParseErr    string   `json:"parseErr,omitempty"`    // default "failed to parse response"
	Response    string   `json:"response,omitempty"`    // Go decode type; default the spec schema name
	IDs         string   `json:"ids,omitempty"`         // ID list parameter of a relationship change
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
	sort.Slice(f.Operations, func(i, j int) bool { return f.Operations[i].Operation < f.Operations[j].Operation })
	fields := []string{`"tag": ` + jsonString(f.Tag), `"file": ` + jsonString(f.File)}
	if len(f.Types) > 0 {
		var types []string
		for _, t := range f.Types {
			types = append(types, jsonString(t))
		}
		fields = append(fields, "\"types\": [\n    "+strings.Join(types, ",\n    ")+"\n  ]")
	}
	if len(f.Operations) > 0 {
		var ops []string
		for _, op := range f.Operations {
			var buf bytes.Buffer
			enc := json.NewEncoder(&buf)
			enc.SetEscapeHTML(false)
			if err := enc.Encode(op); err != nil {
				return nil, err
			}
			ops = append(ops, strings.TrimSpace(buf.String()))
		}
		fields = append(fields, "\"operations\": [\n    "+strings.Join(ops, ",\n    ")+"\n  ]")
	}
	return []byte("{\n  " + strings.Join(fields, ",\n  ") + "\n}\n"), nil
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
