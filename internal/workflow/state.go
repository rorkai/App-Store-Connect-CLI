package workflow

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type persistedHookState struct {
	Command string `json:"command,omitempty"`
	Status  string `json:"status,omitempty"`
}

type persistedRunHooks struct {
	BeforeAll *persistedHookState `json:"before_all,omitempty"`
	AfterAll  *persistedHookState `json:"after_all,omitempty"`
}

type persistedStepState struct {
	Name           string            `json:"name,omitempty"`
	Workflow       string            `json:"workflow,omitempty"`
	ParentWorkflow string            `json:"parent_workflow,omitempty"`
	Status         string            `json:"status,omitempty"`
	Outputs        map[string]string `json:"outputs,omitempty"`
}

type persistedRunState struct {
	RunID          string                        `json:"run_id"`
	Workflow       string                        `json:"workflow"`
	WorkflowFile   string                        `json:"workflow_file,omitempty"`
	DefinitionHash string                        `json:"definition_hash,omitempty"`
	Params         map[string]string             `json:"params,omitempty"`
	Status         string                        `json:"status,omitempty"`
	FailedStep     string                        `json:"failed_step,omitempty"`
	Hooks          *persistedRunHooks            `json:"hooks,omitempty"`
	Steps          map[string]persistedStepState `json:"steps,omitempty"`
	CreatedAt      string                        `json:"created_at,omitempty"`
	UpdatedAt      string                        `json:"updated_at,omitempty"`
}

func newPersistedRunState(workflowName, workflowFile, definitionHash string, params map[string]string) (persistedRunState, error) {
	runID, err := generateRunID(workflowName)
	if err != nil {
		return persistedRunState{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	return persistedRunState{
		RunID:          runID,
		Workflow:       workflowName,
		WorkflowFile:   workflowFile,
		DefinitionHash: definitionHash,
		Params:         cloneStringMap(params),
		Status:         "running",
		Steps:          map[string]persistedStepState{},
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

func loadRunState(path string) (*persistedRunState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read workflow run state: %w", err)
	}
	var state persistedRunState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse workflow run state: %w", err)
	}
	if state.Steps == nil {
		state.Steps = map[string]persistedStepState{}
	}
	return &state, nil
}

func saveRunState(path string, state persistedRunState) error {
	state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal workflow run state: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create workflow run state directory: %w", err)
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return fmt.Errorf("write workflow run state: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("persist workflow run state: %w", err)
	}
	return nil
}

func runStateFilePath(stateDir, runID string) string {
	return filepath.Join(stateDir, sanitizeStateToken(runID)+".json")
}

func definitionFingerprint(def *Definition) (string, error) {
	data, err := json.Marshal(def)
	if err != nil {
		return "", fmt.Errorf("marshal workflow definition: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func generateRunID(workflowName string) (string, error) {
	var suffix [4]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", fmt.Errorf("generate run id: %w", err)
	}
	return fmt.Sprintf(
		"%s-%s-%s",
		sanitizeStateToken(workflowName),
		time.Now().UTC().Format("20060102T150405Z"),
		hex.EncodeToString(suffix[:]),
	), nil
}

func sanitizeStateToken(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "unknown"
	}
	var b strings.Builder
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	result := strings.Trim(b.String(), "._")
	if result == "" {
		return "unknown"
	}
	return result
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneNestedStringMap(in map[string]map[string]string) map[string]map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]map[string]string, len(in))
	for k, v := range in {
		out[k] = cloneStringMap(v)
	}
	return out
}
