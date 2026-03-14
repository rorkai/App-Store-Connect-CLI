// Package workflow is a standalone workflow runner for .asc/workflow.json files.
// It has zero imports from the rest of the codebase. Only depends on Go stdlib
// plus tidwall/jsonc for JSONC comment support in load.go.
package workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// Definition is the top-level .asc/workflow.json schema.
type Definition struct {
	Env       map[string]string   `json:"env,omitempty"`
	BeforeAll string              `json:"before_all,omitempty"`
	AfterAll  string              `json:"after_all,omitempty"`
	Error     string              `json:"error,omitempty"`
	Workflows map[string]Workflow `json:"workflows"`
}

// Workflow is a named automation sequence.
type Workflow struct {
	Description string            `json:"description,omitempty"`
	Private     bool              `json:"private,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Steps       []Step            `json:"steps"`
}

// Step is one executable action in a workflow.
// Bare JSON strings unmarshal to Step{Run: "..."} as shorthand.
type Step struct {
	Run      string            `json:"run,omitempty"`
	Workflow string            `json:"workflow,omitempty"`
	Name     string            `json:"name,omitempty"`
	If       string            `json:"if,omitempty"`
	With     map[string]string `json:"with,omitempty"`
	Outputs  map[string]string `json:"outputs,omitempty"`
}

// UnmarshalJSON handles the flexible step format:
//   - bare string → Step{Run: "..."}
//   - object → normal unmarshal
func (s *Step) UnmarshalJSON(data []byte) error {
	// encoding/json passes already-trimmed tokens to UnmarshalJSON.
	if bytes.Equal(data, []byte("null")) {
		return fmt.Errorf("step must be a string or object, not null")
	}

	var raw string
	if err := json.Unmarshal(data, &raw); err == nil {
		*s = Step{Run: raw}
		return nil
	}

	type stepAlias Step
	var alias stepAlias
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&alias); err != nil {
		return fmt.Errorf("step must be a string or object: %w", err)
	}
	// Ensure there is exactly one JSON value.
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("step must be a single JSON value: trailing data")
	}
	*s = Step(alias)
	return nil
}
