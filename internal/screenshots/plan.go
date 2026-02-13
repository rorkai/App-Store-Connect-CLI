package screenshots

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

var (
	// ErrPlanRead indicates plan file read failure.
	ErrPlanRead = errors.New("read plan")
	// ErrPlanParseJSON indicates plan JSON decode failure.
	ErrPlanParseJSON = errors.New("parse plan JSON")
)

// StepAction is one supported automation step.
type StepAction string

const (
	ActionLaunch      StepAction = "launch"
	ActionTap         StepAction = "tap"
	ActionType        StepAction = "type"
	ActionKeySequence StepAction = "key_sequence"
	ActionWait        StepAction = "wait"
	ActionWaitFor     StepAction = "wait_for"
	ActionScreenshot  StepAction = "screenshot"
)

// PlanValidationCode classifies validation failures.
type PlanValidationCode string

const (
	PlanErrUnsupportedVersion PlanValidationCode = "unsupported_version"
	PlanErrMissingBundleID    PlanValidationCode = "missing_bundle_id"
	PlanErrMissingSteps       PlanValidationCode = "missing_steps"
	PlanErrNegativeDelay      PlanValidationCode = "negative_post_action_delay"
	PlanErrMissingAction      PlanValidationCode = "missing_action"
	PlanErrTapMissingTarget   PlanValidationCode = "tap_missing_target"
	PlanErrTypeMissingText    PlanValidationCode = "type_missing_text"
	PlanErrKeycodesMissing    PlanValidationCode = "keycodes_missing"
	PlanErrKeycodeInvalid     PlanValidationCode = "keycode_invalid"
	PlanErrWaitMissingMS      PlanValidationCode = "wait_missing_duration_ms"
	PlanErrWaitForMissingBy   PlanValidationCode = "wait_for_missing_matcher"
	PlanErrWaitForNegTimeout  PlanValidationCode = "wait_for_negative_timeout"
	PlanErrWaitForNegPoll     PlanValidationCode = "wait_for_negative_poll"
	PlanErrScreenshotNoName   PlanValidationCode = "screenshot_missing_name"
	PlanErrUnsupportedAction  PlanValidationCode = "unsupported_action"
)

// PlanValidationError describes a structured plan validation failure.
type PlanValidationError struct {
	Code    PlanValidationCode
	Step    int
	Message string
}

func (e *PlanValidationError) Error() string {
	return e.Message
}

func newPlanValidationError(code PlanValidationCode, step int, message string) error {
	return &PlanValidationError{Code: code, Step: step, Message: message}
}

// Plan defines a deterministic screenshot automation sequence.
type Plan struct {
	Version  int          `json:"version"`
	App      PlanApp      `json:"app"`
	Defaults PlanDefaults `json:"defaults,omitempty"`
	Steps    []PlanStep   `json:"steps"`
}

// PlanApp contains app/simulator defaults for a run.
type PlanApp struct {
	BundleID  string `json:"bundle_id"`
	UDID      string `json:"udid,omitempty"`
	OutputDir string `json:"output_dir,omitempty"`
}

// PlanDefaults defines default timing behavior.
type PlanDefaults struct {
	PostActionDelayMS int `json:"post_action_delay_ms,omitempty"`
}

// PlanStep is one executable action in the plan.
type PlanStep struct {
	Action         StepAction `json:"action"`
	Name           *string    `json:"name,omitempty"`
	Label          *string    `json:"label,omitempty"`
	ID             *string    `json:"id,omitempty"`
	Contains       *string    `json:"contains,omitempty"`
	Text           *string    `json:"text,omitempty"`
	Keycodes       []int      `json:"keycodes,omitempty"`
	X              *float64   `json:"x,omitempty"`
	Y              *float64   `json:"y,omitempty"`
	DurationMS     *int       `json:"duration_ms,omitempty"`
	TimeoutMS      *int       `json:"timeout_ms,omitempty"`
	PollIntervalMS *int       `json:"poll_interval_ms,omitempty"`
}

// LoadPlan reads and validates a plan file.
func LoadPlan(path string) (*Plan, error) {
	plan, err := LoadPlanUnvalidated(path)
	if err != nil {
		return nil, err
	}
	if err := validatePlan(plan); err != nil {
		return nil, err
	}
	return plan, nil
}

// LoadPlanUnvalidated reads and parses a plan file without validation.
func LoadPlanUnvalidated(path string) (*Plan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrPlanRead, err)
	}

	var plan Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrPlanParseJSON, err)
	}

	if plan.Version == 0 {
		plan.Version = 1
	}
	return &plan, nil
}

func validatePlan(plan *Plan) error {
	if plan.Version != 1 {
		return newPlanValidationError(
			PlanErrUnsupportedVersion,
			0,
			fmt.Sprintf("unsupported plan version %d (expected 1)", plan.Version),
		)
	}
	if strings.TrimSpace(plan.App.BundleID) == "" {
		return newPlanValidationError(PlanErrMissingBundleID, 0, "app.bundle_id is required")
	}
	if len(plan.Steps) == 0 {
		return newPlanValidationError(PlanErrMissingSteps, 0, "at least one step is required")
	}
	if plan.Defaults.PostActionDelayMS < 0 {
		return newPlanValidationError(PlanErrNegativeDelay, 0, "defaults.post_action_delay_ms must be >= 0")
	}

	for i := range plan.Steps {
		step := &plan.Steps[i]
		idx := i + 1
		action := StepAction(strings.TrimSpace(strings.ToLower(string(step.Action))))
		step.Action = action

		if action == "" {
			return newPlanValidationError(
				PlanErrMissingAction,
				idx,
				fmt.Sprintf("steps[%d].action is required", idx),
			)
		}
		switch action {
		case ActionLaunch:
			// no additional fields
		case ActionTap:
			hasLabel := hasString(step.Label)
			hasID := hasString(step.ID)
			hasCoords := step.X != nil && step.Y != nil
			if !hasLabel && !hasID && !hasCoords {
				return newPlanValidationError(
					PlanErrTapMissingTarget,
					idx,
					fmt.Sprintf("steps[%d] tap requires label, id, or x+y coordinates", idx),
				)
			}
		case ActionType:
			if !hasString(step.Text) {
				return newPlanValidationError(
					PlanErrTypeMissingText,
					idx,
					fmt.Sprintf("steps[%d] type requires text", idx),
				)
			}
		case ActionKeySequence:
			if len(step.Keycodes) == 0 {
				return newPlanValidationError(
					PlanErrKeycodesMissing,
					idx,
					fmt.Sprintf("steps[%d] key_sequence requires keycodes", idx),
				)
			}
			for _, keycode := range step.Keycodes {
				if keycode <= 0 {
					return newPlanValidationError(
						PlanErrKeycodeInvalid,
						idx,
						fmt.Sprintf("steps[%d] key_sequence keycodes must be > 0", idx),
					)
				}
			}
		case ActionWait:
			if step.DurationMS == nil || *step.DurationMS <= 0 {
				return newPlanValidationError(
					PlanErrWaitMissingMS,
					idx,
					fmt.Sprintf("steps[%d] wait requires duration_ms > 0", idx),
				)
			}
		case ActionWaitFor:
			hasLabel := hasString(step.Label)
			hasID := hasString(step.ID)
			hasContains := hasString(step.Contains)
			if !hasLabel && !hasID && !hasContains {
				return newPlanValidationError(
					PlanErrWaitForMissingBy,
					idx,
					fmt.Sprintf("steps[%d] wait_for requires label, id, or contains", idx),
				)
			}
			if step.TimeoutMS != nil && *step.TimeoutMS < 0 {
				return newPlanValidationError(
					PlanErrWaitForNegTimeout,
					idx,
					fmt.Sprintf("steps[%d] wait_for timeout_ms must be >= 0", idx),
				)
			}
			if step.PollIntervalMS != nil && *step.PollIntervalMS < 0 {
				return newPlanValidationError(
					PlanErrWaitForNegPoll,
					idx,
					fmt.Sprintf("steps[%d] wait_for poll_interval_ms must be >= 0", idx),
				)
			}
		case ActionScreenshot:
			if !hasString(step.Name) {
				return newPlanValidationError(
					PlanErrScreenshotNoName,
					idx,
					fmt.Sprintf("steps[%d] screenshot requires name", idx),
				)
			}
		default:
			return newPlanValidationError(
				PlanErrUnsupportedAction,
				idx,
				fmt.Sprintf("steps[%d] unsupported action %q", idx, action),
			)
		}
	}
	return nil
}

func hasString(value *string) bool {
	return value != nil && strings.TrimSpace(*value) != ""
}
