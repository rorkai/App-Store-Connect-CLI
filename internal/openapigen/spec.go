package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// Spec is the part of an OpenAPI 3 document the generator reads.
type Spec struct {
	Paths      map[string]map[string]json.RawMessage `json:"paths"`
	Components struct {
		Schemas map[string]*Schema `json:"schemas"`
	} `json:"components"`

	ops map[string]*Operation
}

// Operation is one HTTP method on one path.
type Operation struct {
	ID       string
	Tag      string
	Method   string // upper case
	Path     string // template, such as /v1/apps/{id}/builds
	Request  string // request body schema name, if any
	Response string // success response schema name; empty for 204
}

// Schema is the part of a JSON schema the generator reads.
type Schema struct {
	Ref        string     `json:"$ref"`
	Type       string     `json:"type"`
	Enum       []string   `json:"enum"`
	Required   []string   `json:"required"`
	Items      *Schema    `json:"items"`
	Properties Properties `json:"properties"`
}

// Property is one named schema property; order follows the spec file.
type Property struct {
	Name   string
	Schema *Schema
}

// Properties keeps object properties in spec order, which sets Go field order.
type Properties []Property

// UnmarshalJSON reads an object while keeping key order.
func (p *Properties) UnmarshalJSON(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	if _, err := dec.Token(); err != nil {
		return err
	}
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return err
		}
		var s Schema
		if err := dec.Decode(&s); err != nil {
			return err
		}
		*p = append(*p, Property{Name: key.(string), Schema: &s})
	}
	return nil
}

// Get returns the named property or nil.
func (p Properties) Get(name string) *Schema {
	for _, prop := range p {
		if prop.Name == name {
			return prop.Schema
		}
	}
	return nil
}

func (s *Schema) requires(name string) bool {
	for _, r := range s.Required {
		if r == name {
			return true
		}
	}
	return false
}

func refName(ref string) string {
	return ref[strings.LastIndex(ref, "/")+1:]
}

// LoadSpec reads and indexes an OpenAPI document.
func LoadSpec(path string) (*Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var spec Spec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	spec.ops = map[string]*Operation{}
	for path, methods := range spec.Paths {
		for method, raw := range methods {
			switch method {
			case "get", "post", "patch", "delete", "put":
			default:
				continue
			}
			op, err := parseOperation(path, strings.ToUpper(method), raw)
			if err != nil {
				return nil, err
			}
			spec.ops[op.ID] = op
		}
	}
	return &spec, nil
}

type mediaSchema struct {
	Content map[string]struct {
		Schema Schema `json:"schema"`
	} `json:"content"`
}

func (m *mediaSchema) ref() string {
	if m == nil {
		return ""
	}
	return refName(m.Content["application/json"].Schema.Ref)
}

func parseOperation(path, method string, raw json.RawMessage) (*Operation, error) {
	var op struct {
		ID          string                  `json:"operationId"`
		Tags        []string                `json:"tags"`
		RequestBody *mediaSchema            `json:"requestBody"`
		Responses   map[string]*mediaSchema `json:"responses"`
	}
	if err := json.Unmarshal(raw, &op); err != nil {
		return nil, fmt.Errorf("parse %s %s: %w", method, path, err)
	}
	out := &Operation{
		ID:      op.ID,
		Method:  method,
		Path:    path,
		Request: op.RequestBody.ref(),
	}
	if len(op.Tags) > 0 {
		out.Tag = op.Tags[0]
	}
	for _, status := range []string{"200", "201"} {
		if r := op.Responses[status].ref(); r != "" {
			out.Response = r
			break
		}
	}
	return out, nil
}

// Operation returns the operation with the given operationId.
func (s *Spec) Operation(id string) (*Operation, bool) {
	op, ok := s.ops[id]
	return op, ok
}

// Operations returns every operation of a tag, sorted by operationId.
func (s *Spec) Operations(tag string) []*Operation {
	var out []*Operation
	for _, op := range s.ops {
		if op.Tag == tag {
			out = append(out, op)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Schema resolves a schema name.
func (s *Spec) Schema(name string) (*Schema, bool) {
	sch, ok := s.Components.Schemas[name]
	return sch, ok
}

// resourceNamed returns the unversioned resource schema with the given
// JSON:API type, such as "GameCenterGroup" for "gameCenterGroups".
func (s *Spec) resourceNamed(jsonType string) string {
	var names []string
	for name, sch := range s.Components.Schemas {
		t := sch.Properties.Get("type")
		if t != nil && len(t.Enum) == 1 && t.Enum[0] == jsonType && sch.Properties.Get("attributes") != nil && !versionSuffix.MatchString(name) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return ""
	}
	return names[0]
}

// resourceOf returns the resource schema a response document's data points at,
// and whether data is a collection.
func (s *Spec) resourceOf(document string) (string, bool, bool) {
	sch, ok := s.Schema(document)
	if !ok {
		return "", false, false
	}
	data := sch.Properties.Get("data")
	switch {
	case data == nil:
		return "", false, false
	case data.Ref != "":
		return refName(data.Ref), false, true
	case data.Type == "array" && data.Items != nil && data.Items.Ref != "":
		return refName(data.Items.Ref), true, true
	}
	return "", false, false
}

// Tags returns every tag that has operations, sorted.
func (s *Spec) Tags() []string {
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
