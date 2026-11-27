package screenshots

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPlan_Valid(t *testing.T) {
	path := writePlanFile(t, `{
  "version": 1,
  "app": { "bundle_id": "com.example.app", "udid": "booted", "output_dir": "./screenshots/raw" },
  "steps": [
    { "action": "launch" },
    { "action": "wait", "duration_ms": 1 },
    { "action": "screenshot", "name": "home" }
  ]
}`)

	plan, err := LoadPlan(path)
	if err != nil {
		t.Fatalf("LoadPlan returned error: %v", err)
	}
	if plan.App.BundleID != "com.example.app" {
		t.Fatalf("unexpected bundle_id: %q", plan.App.BundleID)
	}
	if len(plan.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(plan.Steps))
	}
}

func TestLoadPlan_Valid_WithJSONCComments(t *testing.T) {
	path := writePlanFile(t, `{
  // This is a JSONC plan file (comments allowed).
  "version": 1,
  "app": {
    "bundle_id": "com.example.app" // bundle id for simctl launch
  },
  "steps": [
    { "action": "launch" }, // open app
    /* wait for the UI to settle */ { "action": "wait", "duration_ms": 1 },
    { "action": "screenshot", "name": "home" }
  ]
}`)

	plan, err := LoadPlan(path)
	if err != nil {
		t.Fatalf("LoadPlan returned error: %v", err)
	}
	if plan.App.BundleID != "com.example.app" {
		t.Fatalf("unexpected bundle_id: %q", plan.App.BundleID)
	}
	if len(plan.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(plan.Steps))
	}
}

func TestLoadPlan_MissingBundleID(t *testing.T) {
	path := writePlanFile(t, `{
  "version": 1,
  "app": {},
  "steps": [{ "action": "launch" }]
}`)

	_, err := LoadPlan(path)
	if err == nil {
		t.Fatal("expected error for missing bundle_id")
	}
	assertValidationCode(t, err, PlanErrMissingBundleID)
}

func TestLoadPlan_InvalidAction(t *testing.T) {
	path := writePlanFile(t, `{
  "version": 1,
  "app": { "bundle_id": "com.example.app" },
  "steps": [{ "action": "unknown" }]
}`)

	_, err := LoadPlan(path)
	if err == nil {
		t.Fatal("expected error for unsupported action")
	}
	assertValidationCode(t, err, PlanErrUnsupportedAction)
}

func TestLoadPlan_TapRequiresTarget(t *testing.T) {
	path := writePlanFile(t, `{
  "version": 1,
  "app": { "bundle_id": "com.example.app" },
  "steps": [{ "action": "tap" }]
}`)

	_, err := LoadPlan(path)
	if err == nil {
		t.Fatal("expected error for tap without target")
	}
	assertValidationCode(t, err, PlanErrTapMissingTarget)
}

func TestLoadPlan_TapZeroCoordinatesAreValid(t *testing.T) {
	path := writePlanFile(t, `{
  "version": 1,
  "app": { "bundle_id": "com.example.app" },
  "steps": [{ "action": "tap", "x": 0, "y": 0 }]
}`)

	_, err := LoadPlan(path)
	if err != nil {
		t.Fatalf("expected zero coordinates to be valid, got error: %v", err)
	}
}

func TestLoadPlan_ScreenshotRequiresName(t *testing.T) {
	path := writePlanFile(t, `{
  "version": 1,
  "app": { "bundle_id": "com.example.app" },
  "steps": [{ "action": "screenshot" }]
}`)

	_, err := LoadPlan(path)
	if err == nil {
		t.Fatal("expected error for screenshot without name")
	}
	assertValidationCode(t, err, PlanErrScreenshotNoName)
}

func TestLoadPlan_WaitForRequiresMatcher(t *testing.T) {
	path := writePlanFile(t, `{
  "version": 1,
  "app": { "bundle_id": "com.example.app" },
  "steps": [{ "action": "wait_for", "timeout_ms": 1000 }]
}`)

	_, err := LoadPlan(path)
	if err == nil {
		t.Fatal("expected error for wait_for without matcher")
	}
	assertValidationCode(t, err, PlanErrWaitForMissingBy)
}

func TestLoadPlan_WaitForNegativePollInterval(t *testing.T) {
	path := writePlanFile(t, `{
  "version": 1,
  "app": { "bundle_id": "com.example.app" },
  "steps": [{ "action": "wait_for", "label": "RESULT", "poll_interval_ms": -1 }]
}`)

	_, err := LoadPlan(path)
	if err == nil {
		t.Fatal("expected error for negative poll_interval_ms")
	}
	assertValidationCode(t, err, PlanErrWaitForNegPoll)
}

func TestLoadPlan_KeySequenceRequiresKeycodes(t *testing.T) {
	path := writePlanFile(t, `{
  "version": 1,
  "app": { "bundle_id": "com.example.app" },
  "steps": [{ "action": "key_sequence" }]
}`)

	_, err := LoadPlan(path)
	if err == nil {
		t.Fatal("expected error for key_sequence without keycodes")
	}
	assertValidationCode(t, err, PlanErrKeycodesMissing)
}

func TestLoadPlan_ScrollAndSwipeValidation(t *testing.T) {
	tests := []struct {
		name string
		step string
		code PlanValidationCode
	}{
		{"scroll missing direction", `{ "action": "scroll" }`, PlanErrScrollDirection},
		{"scroll bad direction", `{ "action": "scroll", "direction": "sideways" }`, PlanErrScrollDirection},
		{"scroll label and id", `{ "action": "scroll", "direction": "up", "label": "Privacy", "id": "privacy" }`, PlanErrScrollLabelAndID},
		{"scroll distance too small", `{ "action": "scroll", "direction": "up", "distance": 0.01 }`, PlanErrScrollDistance},
		{"scroll distance too large", `{ "action": "scroll", "direction": "up", "distance": 1 }`, PlanErrScrollDistance},
		{"scroll max_swipes without target", `{ "action": "scroll", "direction": "up", "max_swipes": 5 }`, PlanErrScrollMaxSwipes},
		{"scroll max_swipes out of range", `{ "action": "scroll", "direction": "up", "label": "Privacy", "max_swipes": 51 }`, PlanErrScrollMaxSwipes},
		{"swipe missing end_y", `{ "action": "swipe", "x": 200, "y": 600, "end_x": 200 }`, PlanErrSwipeCoordinates},
		{"swipe negative coordinate", `{ "action": "swipe", "x": -1, "y": 600, "end_x": 200, "end_y": 300 }`, PlanErrSwipeCoordinates},
		{"swipe zero duration", `{ "action": "swipe", "x": 200, "y": 600, "end_x": 200, "end_y": 300, "duration_ms": 0 }`, PlanErrSwipeDuration},
		{"long_press missing target", `{ "action": "long_press", "duration_ms": 1000 }`, PlanErrTapMissingTarget},
		{"long_press duration too long", `{ "action": "long_press", "label": "Photo", "duration_ms": 10001 }`, PlanErrHoldDuration},
		{"long_press with another action's field", `{ "action": "long_press", "label": "Photo", "end_x": 10 }`, PlanErrUnexpectedField},
		{"button unknown name", `{ "action": "button", "button": "volume-up" }`, PlanErrButtonInvalid},
		{"button with another action's field", `{ "action": "button", "button": "home", "label": "Home" }`, PlanErrUnexpectedField},
		{"pinch missing scale", `{ "action": "pinch" }`, PlanErrPinchScale},
		{"pinch zero scale", `{ "action": "pinch", "scale": 0 }`, PlanErrPinchScale},
		{"pinch scale of 1", `{ "action": "pinch", "scale": 1 }`, PlanErrPinchScale},
		{"pinch x without y", `{ "action": "pinch", "scale": 2, "x": 200 }`, PlanErrGestureCoordinates},
		{"pinch zero radius", `{ "action": "pinch", "scale": 2, "radius": 0 }`, PlanErrGestureRadius},
		{"pinch with another action's field", `{ "action": "pinch", "scale": 2, "degrees": 90 }`, PlanErrUnexpectedField},
		{"rotate missing radius", `{ "action": "rotate", "x": 200, "y": 400, "degrees": 90 }`, PlanErrGestureRadius},
		{"rotate missing y", `{ "action": "rotate", "x": 200, "radius": 80, "degrees": 90 }`, PlanErrGestureCoordinates},
		{"rotate degrees out of range", `{ "action": "rotate", "x": 200, "y": 400, "radius": 80, "degrees": 361 }`, PlanErrRotateDegrees},
		{"rotate zero degrees", `{ "action": "rotate", "x": 200, "y": 400, "radius": 80, "degrees": 0 }`, PlanErrRotateDegrees},
		{"rotate with another action's field", `{ "action": "rotate", "x": 200, "y": 400, "radius": 80, "degrees": 90, "scale": 2 }`, PlanErrUnexpectedField},
		{"double_tap missing y", `{ "action": "double_tap", "x": 200 }`, PlanErrGestureCoordinates},
		{"double_tap negative coordinate", `{ "action": "double_tap", "x": -1, "y": 400 }`, PlanErrGestureCoordinates},
		{"double_tap with another action's field", `{ "action": "double_tap", "x": 200, "y": 400, "label": "Photo" }`, PlanErrUnexpectedField},
		{"scroll with another action's field", `{ "action": "scroll", "direction": "down", "x": 200 }`, PlanErrUnexpectedField},
		{"swipe with another action's field", `{ "action": "swipe", "x": 1, "y": 2, "end_x": 3, "end_y": 4, "label": "Photo" }`, PlanErrUnexpectedField},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writePlanFile(t, `{
  "version": 1,
  "app": { "bundle_id": "com.example.app" },
  "steps": [`+tt.step+`]
}`)
			_, err := LoadPlan(path)
			assertValidationCode(t, err, tt.code)
		})
	}
}

func TestRunPlan_WaitOnly(t *testing.T) {
	plan := &Plan{
		Version: 1,
		App: PlanApp{
			BundleID:  "com.example.app",
			OutputDir: t.TempDir(),
		},
		Steps: []PlanStep{
			{Action: ActionWait, DurationMS: intPtr(1)},
		},
	}

	result, err := RunPlan(context.Background(), plan)
	if err != nil {
		t.Fatalf("RunPlan returned error: %v", err)
	}
	if len(result.Steps) != 1 {
		t.Fatalf("expected 1 step result, got %d", len(result.Steps))
	}
	if result.Steps[0].Status != "ok" {
		t.Fatalf("expected step status ok, got %q", result.Steps[0].Status)
	}
}

func writePlanFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "screenshots.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write plan file: %v", err)
	}
	return path
}

func assertValidationCode(t *testing.T, err error, code PlanValidationCode) {
	t.Helper()

	var validationErr *PlanValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected PlanValidationError, got: %v", err)
	}
	if validationErr.Code != code {
		t.Fatalf("expected validation code %q, got %q", code, validationErr.Code)
	}
}

func intPtr(value int) *int {
	return &value
}

func TestLoadPlan_UnknownFieldsRemainCompatible(t *testing.T) {
	path := writePlanFile(t, `{"app":{"bundle_id":"example.fixture"},"steps":[{"action":"wait","duration_ms":1,"duration_mss":2500}]}`)
	plan, err := LoadPlan(path)
	if err != nil {
		t.Fatalf("plan load rejected an unknown field: %v", err)
	}
	if plan.Version != 1 || len(plan.Steps) != 1 || plan.Steps[0].DurationMS == nil || *plan.Steps[0].DurationMS != 1 {
		t.Fatalf("plan values changed: %+v", plan)
	}
}
