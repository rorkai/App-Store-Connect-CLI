package main

import (
	"bytes"
	"encoding/json"
	"os"
	"sort"
)

// Bindings lists what is generated and the Go-side choices the spec cannot
// supply. `-adopt` writes it from existing code; nobody edits it by hand.
type Bindings struct {
	Families []*Family `json:"families"`
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

// LoadBindings reads a bindings file.
func LoadBindings(path string) (*Bindings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var b Bindings
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

// Encode returns the file contents in a stable order.
func (b *Bindings) Encode() ([]byte, error) {
	sort.Slice(b.Families, func(i, j int) bool { return b.Families[i].Tag < b.Families[j].Tag })
	for _, f := range b.Families {
		sort.Strings(f.Types)
		sort.Slice(f.Operations, func(i, j int) bool { return f.Operations[i].Operation < f.Operations[j].Operation })
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(b); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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
