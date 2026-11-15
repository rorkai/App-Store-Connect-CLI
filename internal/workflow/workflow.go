// Package workflow is a standalone workflow runner for .asc/workflow.json files.
// It has zero imports from the rest of the codebase. Only depends on Go stdlib
// plus tidwall/jsonc for JSONC comment support in load.go.
package workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
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

// RetryPolicy configures explicit bounded retries for a run step.
type RetryPolicy struct {
	MaxAttempts int    `json:"max_attempts"`
	Delay       string `json:"delay"`
}

// Step is one executable action in a workflow.
// Bare JSON strings unmarshal to Step{Run: "..."} as shorthand.
type Step struct {
	Run                 string            `json:"run,omitempty"`
	Workflow            string            `json:"workflow,omitempty"`
	Name                string            `json:"name,omitempty"`
	If                  string            `json:"if,omitempty"`
	With                map[string]string `json:"with,omitempty"`
	Outputs             map[string]string `json:"outputs,omitempty"`
	Retry               *RetryPolicy      `json:"retry,omitempty"`
	Timeout             *string           `json:"timeout,omitempty"`
	Env                 map[string]string `json:"env,omitempty"`
	ContinueOnError     bool              `json:"continue_on_error,omitempty"`
	retryExplicitNull   bool
	timeoutExplicitNull bool

	continueOnErrorExplicitNull bool
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
	retryNull, timeoutNull, continueOnErrorNull, err := explicitNullPolicyFields(data)
	if err != nil {
		return fmt.Errorf("step must be an object: %w", err)
	}
	s.retryExplicitNull = retryNull
	s.timeoutExplicitNull = timeoutNull
	s.continueOnErrorExplicitNull = continueOnErrorNull
	return nil
}

func explicitNullPolicyFields(data []byte) (bool, bool, bool, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	start, err := dec.Token()
	if err != nil {
		return false, false, false, err
	}
	if delim, ok := start.(json.Delim); !ok || delim != '{' {
		return false, false, false, fmt.Errorf("expected object")
	}

	var retryNull, timeoutNull, continueOnErrorNull bool
	for dec.More() {
		keyToken, err := dec.Token()
		if err != nil {
			return false, false, false, err
		}
		key, ok := keyToken.(string)
		if !ok {
			return false, false, false, fmt.Errorf("expected object field name")
		}
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return false, false, false, err
		}
		isNull := bytes.Equal(bytes.TrimSpace(value), []byte("null"))
		switch {
		case strings.EqualFold(key, "retry"):
			retryNull = isNull
		case strings.EqualFold(key, "timeout"):
			timeoutNull = isNull
		case strings.EqualFold(key, "continue_on_error"):
			continueOnErrorNull = isNull
		}
	}
	if _, err := dec.Token(); err != nil {
		return false, false, false, err
	}
	return retryNull, timeoutNull, continueOnErrorNull, nil
}
