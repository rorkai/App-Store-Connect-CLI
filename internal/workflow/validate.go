package workflow

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
)

// ValidationCode classifies validation failures.
type ValidationCode string

const (
	ErrNoWorkflows                 ValidationCode = "no_workflows"
	ErrInvalidWorkflowName         ValidationCode = "invalid_workflow_name"
	ErrEmptySteps                  ValidationCode = "empty_steps"
	ErrStepNoAction                ValidationCode = "step_no_action"
	ErrStepEmptyRun                ValidationCode = "step_empty_run"
	ErrStepConflict                ValidationCode = "step_run_and_workflow"
	ErrWorkflowNotFound            ValidationCode = "workflow_not_found"
	ErrCyclicReference             ValidationCode = "cyclic_reference"
	ErrStepWithOnRun               ValidationCode = "step_with_on_run"
	ErrStepOutputsOnWorkflow       ValidationCode = "step_outputs_on_workflow"
	ErrStepOutputsRequireName      ValidationCode = "step_outputs_require_name"
	ErrDuplicateOutputProducerName ValidationCode = "duplicate_output_producer_name"
	ErrInvalidOutputName           ValidationCode = "invalid_output_name"
	ErrInvalidOutputExpr           ValidationCode = "invalid_output_expr"
)

// ValidationError describes a structured workflow validation failure.
type ValidationError struct {
	Code     ValidationCode `json:"code"`
	Workflow string         `json:"workflow,omitempty"`
	Step     int            `json:"step,omitempty"`
	Message  string         `json:"message"`
}

func (e *ValidationError) Error() string {
	return e.Message
}

// validWorkflowName matches alphanumeric, hyphens, and underscores, starting with a letter.
var (
	validWorkflowName = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)
	validOutputName   = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*$`)
	validOutputExpr   = regexp.MustCompile(`^\$\.[a-zA-Z0-9_]+(?:\.[a-zA-Z0-9_]+)*$`)
)

// Validate checks a Definition for structural errors.
// Returns all validation errors found (not just the first).
func Validate(def *Definition) []*ValidationError {
	var errs []*ValidationError

	if len(def.Workflows) == 0 {
		errs = append(errs, &ValidationError{
			Code:    ErrNoWorkflows,
			Message: "workflow file must define at least one workflow",
		})
		return errs
	}

	// Sort workflow names for deterministic error ordering.
	names := slices.Sorted(maps.Keys(def.Workflows))

	for _, name := range names {
		if !validWorkflowName.MatchString(name) {
			errs = append(errs, &ValidationError{
				Code:     ErrInvalidWorkflowName,
				Workflow: name,
				Message:  fmt.Sprintf("workflow name %q must start with a letter and contain only letters, digits, hyphens, underscores", name),
			})
		}
	}

	outputProducerWorkflows := map[string]string{}

	for _, name := range names {
		wf := def.Workflows[name]
		if len(wf.Steps) == 0 {
			errs = append(errs, &ValidationError{
				Code:     ErrEmptySteps,
				Workflow: name,
				Message:  fmt.Sprintf("workflow %q must have at least one step", name),
			})
			continue
		}

		for i, step := range wf.Steps {
			idx := i + 1
			hasRun := strings.TrimSpace(step.Run) != ""
			hasWorkflow := strings.TrimSpace(step.Workflow) != ""
			hasRawRun := step.Run != ""

			if !hasRun && !hasWorkflow {
				if hasRawRun {
					errs = append(errs, &ValidationError{
						Code:     ErrStepEmptyRun,
						Workflow: name,
						Step:     idx,
						Message:  fmt.Sprintf("workflow %q step %d has empty run command", name, idx),
					})
				} else {
					errs = append(errs, &ValidationError{
						Code:     ErrStepNoAction,
						Workflow: name,
						Step:     idx,
						Message:  fmt.Sprintf("workflow %q step %d must have run or workflow", name, idx),
					})
				}
			}

			if hasRun && hasWorkflow {
				errs = append(errs, &ValidationError{
					Code:     ErrStepConflict,
					Workflow: name,
					Step:     idx,
					Message:  fmt.Sprintf("workflow %q step %d has both run and workflow (only one allowed)", name, idx),
				})
			}

			if hasRun && len(step.With) > 0 {
				errs = append(errs, &ValidationError{
					Code:     ErrStepWithOnRun,
					Workflow: name,
					Step:     idx,
					Message:  fmt.Sprintf("workflow %q step %d has 'with' on a run step (only allowed on workflow steps)", name, idx),
				})
			}

			if len(step.Outputs) > 0 {
				if hasWorkflow {
					errs = append(errs, &ValidationError{
						Code:     ErrStepOutputsOnWorkflow,
						Workflow: name,
						Step:     idx,
						Message:  fmt.Sprintf("workflow %q step %d has 'outputs' on a workflow step (only allowed on run steps)", name, idx),
					})
				}

				trimmedName := strings.TrimSpace(step.Name)
				if trimmedName == "" || !validWorkflowName.MatchString(trimmedName) {
					errs = append(errs, &ValidationError{
						Code:     ErrStepOutputsRequireName,
						Workflow: name,
						Step:     idx,
						Message:  fmt.Sprintf("workflow %q step %d must use a reference-safe 'name' when declaring outputs", name, idx),
					})
				} else {
					if prevWorkflow, exists := outputProducerWorkflows[trimmedName]; exists {
						errs = append(errs, &ValidationError{
							Code:     ErrDuplicateOutputProducerName,
							Workflow: name,
							Step:     idx,
							Message:  fmt.Sprintf("workflow %q step %d reuses output-producing step name %q already declared in workflow %q", name, idx, trimmedName, prevWorkflow),
						})
					} else {
						outputProducerWorkflows[trimmedName] = name
					}
				}

				for _, outputName := range slices.Sorted(maps.Keys(step.Outputs)) {
					if !validOutputName.MatchString(outputName) {
						errs = append(errs, &ValidationError{
							Code:     ErrInvalidOutputName,
							Workflow: name,
							Step:     idx,
							Message:  fmt.Sprintf("workflow %q step %d has invalid output name %q", name, idx, outputName),
						})
					}
					if !validOutputExpr.MatchString(strings.TrimSpace(step.Outputs[outputName])) {
						errs = append(errs, &ValidationError{
							Code:     ErrInvalidOutputExpr,
							Workflow: name,
							Step:     idx,
							Message:  fmt.Sprintf("workflow %q step %d output %q must use a JSON path like $.field", name, idx, outputName),
						})
					}
				}
			}

			if hasWorkflow {
				ref := strings.TrimSpace(step.Workflow)
				if _, ok := def.Workflows[ref]; !ok {
					errs = append(errs, &ValidationError{
						Code:     ErrWorkflowNotFound,
						Workflow: name,
						Step:     idx,
						Message:  fmt.Sprintf("workflow %q step %d references unknown workflow %q", name, idx, ref),
					})
				}
			}
		}
	}

	if cycleErr := detectCycles(def); cycleErr != nil {
		errs = append(errs, cycleErr)
	}

	return errs
}

// detectCycles performs DFS across all workflows to find circular references.
// Uses white(0)/gray(1)/black(2) coloring.
func detectCycles(def *Definition) *ValidationError {
	const (
		white = 0
		gray  = 1
		black = 2
	)

	colors := make(map[string]int, len(def.Workflows))
	var path []string

	var dfs func(name string) *ValidationError
	dfs = func(name string) *ValidationError {
		colors[name] = gray
		path = append(path, name)

		wf, ok := def.Workflows[name]
		if !ok {
			path = path[:len(path)-1]
			colors[name] = black
			return nil
		}

		for _, step := range wf.Steps {
			ref := strings.TrimSpace(step.Workflow)
			if ref == "" {
				continue
			}
			switch colors[ref] {
			case gray:
				cycleStart := -1
				for i, p := range path {
					if p == ref {
						cycleStart = i
						break
					}
				}
				cycle := append(slices.Clone(path[cycleStart:]), ref)
				return &ValidationError{
					Code:     ErrCyclicReference,
					Workflow: name,
					Message:  fmt.Sprintf("cyclic workflow reference: %s", strings.Join(cycle, " -> ")),
				}
			case white:
				if err := dfs(ref); err != nil {
					return err
				}
			}
		}

		path = path[:len(path)-1]
		colors[name] = black
		return nil
	}

	sortedNames := slices.Sorted(maps.Keys(def.Workflows))
	for _, name := range sortedNames {
		if colors[name] == white {
			if err := dfs(name); err != nil {
				return err
			}
		}
	}
	return nil
}
