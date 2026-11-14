package screenshots

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

// RunStepResult reports one executed step.
type RunStepResult struct {
	Index      int    `json:"index"`
	Action     string `json:"action"`
	Status     string `json:"status"`
	DurationMS int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

// RunResult is the structured output for a plan run.
type RunResult struct {
	BundleID  string          `json:"bundle_id"`
	UDID      string          `json:"udid"`
	OutputDir string          `json:"output_dir"`
	Steps     []RunStepResult `json:"steps"`
}

// RunPlan executes a validated plan.
func RunPlan(ctx context.Context, plan *Plan) (*RunResult, error) {
	return runPlan(ctx, plan, nil)
}

// runPlanWithRoot is the matrix-only execution path. Its screenshot output is
// published through the retained rooted destination instead of handing the
// private attempt pathname to a provider that may write through a swapped
// directory entry.
func runPlanWithRoot(ctx context.Context, plan *Plan, outputRoot rootfs.Root) (*RunResult, error) {
	return runPlan(ctx, plan, &outputRoot)
}

func runPlan(ctx context.Context, plan *Plan, rootedOutput *rootfs.Root) (_ *RunResult, runErr error) {
	if plan == nil {
		return nil, fmt.Errorf("plan is required")
	}
	if err := validatePlan(plan); err != nil {
		return nil, err
	}

	udid := strings.TrimSpace(plan.App.UDID)
	if udid == "" {
		udid = "booted"
	}
	for _, step := range plan.Steps {
		if plan.Defaults.StatusBar != "" || StepAction(strings.TrimSpace(strings.ToLower(string(step.Action)))) != ActionWait {
			resolved, err := resolveSimulatorUDID(ctx, udid)
			if err != nil {
				return nil, err
			}
			udid = resolved
			break
		}
	}
	outputDir := plan.App.OutputDir
	if strings.TrimSpace(outputDir) == "" {
		outputDir = "./screenshots/raw"
	}
	var absOutputDir string
	var err error
	if rootedOutput == nil {
		absOutputDir, err = filepath.Abs(outputDir)
		if err != nil {
			return nil, fmt.Errorf("resolve output dir: %w", err)
		}
		if err := os.MkdirAll(absOutputDir, 0o755); err != nil {
			return nil, fmt.Errorf("create output dir: %w", err)
		}
	} else {
		absOutputDir = rootedOutput.Path()
	}

	result := &RunResult{
		BundleID:  plan.App.BundleID,
		UDID:      udid,
		OutputDir: absOutputDir,
		Steps:     make([]RunStepResult, 0, len(plan.Steps)),
	}
	terminateRunningProcess := plan.App.terminateRunningProcess

	if preset := plan.Defaults.StatusBar; preset != "" {
		if err := runExternal(ctx, "axe", "settings", "status-bar", "preset", preset, "--udid", udid); err != nil {
			return nil, fmt.Errorf("apply status bar preset %q: %w", preset, err)
		}
		defer func() {
			clearCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), matrixSubprocessTimeout)
			defer cancel()
			if clearErr := runExternal(clearCtx, "axe", "settings", "status-bar", "clear", "--udid", udid); clearErr != nil {
				runErr = errors.Join(runErr, fmt.Errorf("clear status bar overrides: %w", clearErr))
			}
		}()
	}

	for i, step := range plan.Steps {
		start := time.Now()
		action := StepAction(strings.TrimSpace(strings.ToLower(string(step.Action))))
		stepResult := RunStepResult{
			Index:  i + 1,
			Action: string(action),
			Status: "ok",
		}

		if err := runStep(ctx, action, step, plan.App.BundleID, plan.App.LaunchArguments, terminateRunningProcess, udid, absOutputDir, rootedOutput); err != nil {
			stepResult.Status = "error"
			stepResult.Error = err.Error()
			stepResult.DurationMS = time.Since(start).Milliseconds()
			result.Steps = append(result.Steps, stepResult)
			return result, fmt.Errorf("step %d (%s): %w", i+1, string(action), err)
		}
		if action == ActionLaunch {
			terminateRunningProcess = false
		}
		stepResult.DurationMS = time.Since(start).Milliseconds()
		result.Steps = append(result.Steps, stepResult)

		if plan.Defaults.PostActionDelayMS > 0 && action != ActionWait && action != ActionWaitFor {
			delay := time.Duration(plan.Defaults.PostActionDelayMS) * time.Millisecond
			if err := waitContext(ctx, delay); err != nil {
				return result, err
			}
		}
	}

	return result, nil
}

func runStep(ctx context.Context, action StepAction, step PlanStep, bundleID string, launchArguments []string, terminateRunningProcess bool, udid, outputDir string, rootedOutput *rootfs.Root) error {
	switch action {
	case ActionLaunch:
		args := []string{"simctl", "launch"}
		if terminateRunningProcess {
			args = append(args, "--terminate-running-process")
		}
		args = append(args, udid, bundleID)
		args = append(args, launchArguments...)
		return runExternal(ctx, "xcrun", args...)
	case ActionTap:
		return runTapStep(ctx, step, udid)
	case ActionType:
		return runExternal(ctx, "axe", "type", stringValue(step.Text), "--udid", udid)
	case ActionKeySequence:
		keycodes := make([]string, 0, len(step.Keycodes))
		for _, keycode := range step.Keycodes {
			keycodes = append(keycodes, strconv.Itoa(keycode))
		}
		return runExternal(ctx, "axe", "key-sequence", "--keycodes", strings.Join(keycodes, ","), "--udid", udid)
	case ActionScroll:
		return runScrollStep(ctx, step, udid)
	case ActionSwipe:
		args := []string{
			"swipe",
			"--start-x", formatFloat(floatValue(step.X)),
			"--start-y", formatFloat(floatValue(step.Y)),
			"--end-x", formatFloat(floatValue(step.EndX)),
			"--end-y", formatFloat(floatValue(step.EndY)),
		}
		if step.DurationMS != nil {
			args = append(args, "--duration", formatFloat(float64(*step.DurationMS)/1000))
		}
		args = append(args, "--udid", udid)
		return runExternal(ctx, "axe", args...)
	case ActionWait:
		return waitContext(ctx, time.Duration(intValue(step.DurationMS))*time.Millisecond)
	case ActionWaitFor:
		return runWaitForStep(ctx, step, udid)
	case ActionScreenshot:
		req := CaptureRequest{
			Provider: ProviderAXe,
			// Screenshot steps capture the current app session state; launch is explicit.
			BundleID:  "",
			UDID:      udid,
			Name:      stringValue(step.Name),
			OutputDir: outputDir,
		}
		var err error
		if rootedOutput != nil {
			_, err = captureWithRoot(ctx, req, *rootedOutput)
		} else {
			_, err = Capture(ctx, req)
		}
		return err
	default:
		return fmt.Errorf("unsupported action %q", action)
	}
}

func runWaitForStep(ctx context.Context, step PlanStep, udid string) error {
	timeout := intValue(step.TimeoutMS)
	if timeout <= 0 {
		timeout = 15000
	}
	poll := intValue(step.PollIntervalMS)
	if poll <= 0 {
		poll = 400
	}

	deadline := time.Now().Add(time.Duration(timeout) * time.Millisecond)
	for {
		matched, err := axeMatchesTarget(ctx, udid, step)
		if err != nil {
			return err
		}
		if matched {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("wait_for timed out after %dms", timeout)
		}
		if err := waitContext(ctx, time.Duration(poll)*time.Millisecond); err != nil {
			return err
		}
	}
}

func runTapStep(ctx context.Context, step PlanStep, udid string) error {
	switch {
	case hasString(step.Label):
		return runExternal(ctx, "axe", "tap", "--label", stringValue(step.Label), "--udid", udid)
	case hasString(step.ID):
		return runExternal(ctx, "axe", "tap", "--id", stringValue(step.ID), "--udid", udid)
	default:
		x := strconv.FormatFloat(floatValue(step.X), 'f', -1, 64)
		y := strconv.FormatFloat(floatValue(step.Y), 'f', -1, 64)
		return runExternal(ctx, "axe", "tap", "-x", x, "-y", y, "--udid", udid)
	}
}

func runScrollStep(ctx context.Context, step PlanStep, udid string) error {
	args := []string{"gesture", "scroll-" + stringValue(step.Direction)}
	if step.Distance != nil {
		args = append(args, "--distance", formatFloat(*step.Distance))
	}
	switch {
	case hasString(step.Label):
		args = append(args, "--until-label", stringValue(step.Label))
	case hasString(step.ID):
		args = append(args, "--until-id", stringValue(step.ID))
	}
	if step.MaxSwipes != nil {
		args = append(args, "--max-swipes", strconv.Itoa(*step.MaxSwipes))
	}
	args = append(args, "--udid", udid)
	return runExternal(ctx, "axe", args...)
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func axeMatchesTarget(ctx context.Context, udid string, step PlanStep) (bool, error) {
	out, err := runExternalOutput(ctx, "axe", "describe-ui", "--udid", udid)
	if err != nil {
		return false, err
	}

	var root any
	if err := json.Unmarshal([]byte(out), &root); err != nil {
		return false, fmt.Errorf("axe describe-ui: parse JSON: %w", err)
	}

	targetID := strings.TrimSpace(stringValue(step.ID))
	targetLabel := strings.TrimSpace(stringValue(step.Label))
	targetContains := strings.ToLower(strings.TrimSpace(stringValue(step.Contains)))
	return nodeMatches(root, targetID, targetLabel, targetContains), nil
}

func nodeMatches(node any, targetID, targetLabel, targetContains string) bool {
	switch n := node.(type) {
	case map[string]any:
		id := toString(n["AXUniqueId"])
		label := toString(n["AXLabel"])
		value := toString(n["AXValue"])

		if targetID != "" && strings.EqualFold(id, targetID) {
			return true
		}
		if targetLabel != "" && strings.EqualFold(label, targetLabel) {
			return true
		}
		if targetContains != "" {
			labelLC := strings.ToLower(label)
			valueLC := strings.ToLower(value)
			if strings.Contains(labelLC, targetContains) || strings.Contains(valueLC, targetContains) {
				return true
			}
		}

		for _, v := range n {
			if nodeMatches(v, targetID, targetLabel, targetContains) {
				return true
			}
		}
	case []any:
		for _, v := range n {
			if nodeMatches(v, targetID, targetLabel, targetContains) {
				return true
			}
		}
	}
	return false
}

func toString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return fmt.Sprintf("%v", t)
	}
}

func runExternal(ctx context.Context, name string, args ...string) error {
	_, err := runExternalOutput(ctx, name, args...)
	return err
}

func runExternalOutput(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.Output()
	if err != nil {
		output := strings.TrimSpace(string(out))
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			stderr := strings.TrimSpace(string(exitErr.Stderr))
			if stderr != "" {
				if output != "" {
					output += "\n" + stderr
				} else {
					output = stderr
				}
			}
		}
		if output == "" {
			return "", fmt.Errorf("%s: %w", name, err)
		}
		return "", fmt.Errorf("%s: %w (output: %s)", name, err, output)
	}
	return string(out), nil
}

func waitContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func intValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func floatValue(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}
