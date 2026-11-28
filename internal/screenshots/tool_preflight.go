package screenshots

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

func checkPlanCaptureTools(ctx context.Context, plan *Plan) error {
	requirement, needsVersionTwo := planCaptureToolRequirement(plan)
	if requirement == "" {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	binary, err := exec.LookPath("axe")
	if err != nil {
		return fmt.Errorf("AXe is required by %s; install axe and add it to PATH", requirement)
	}
	if !needsVersionTwo {
		return nil
	}
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	command := exec.CommandContext(probeCtx, binary, "--version")
	command.WaitDelay = time.Second
	output := cappedMatrixBuffer{limit: 1024}
	command.Stdout, command.Stderr = &output, io.Discard
	err = command.Run()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if probeCtx.Err() != nil {
		return fmt.Errorf("AXe version check timed out while checking %s", requirement)
	}
	if err != nil {
		return fmt.Errorf("check AXe version for %s: %w", requirement, err)
	}
	version := strings.TrimPrefix(strings.TrimSpace(string(output.Bytes())), "v")
	if output.exceeded || !semver.IsValid("v"+version) {
		return fmt.Errorf("cannot determine AXe version required by %s; install AXe 2.0.0 or later", requirement)
	}
	if semver.Compare("v"+version, "v2.0.0") < 0 {
		return fmt.Errorf("%s requires AXe 2.0.0 or later; found %s", requirement, version)
	}
	return nil
}

func planCaptureToolRequirement(plan *Plan) (string, bool) {
	if plan.Defaults.StatusBar != "" {
		return "defaults.status_bar", true
	}
	var first string
	for index, step := range plan.Steps {
		action := StepAction(strings.TrimSpace(strings.ToLower(string(step.Action))))
		if action == ActionLaunch || action == ActionWait {
			continue
		}
		requirement := fmt.Sprintf("step %d (%s)", index+1, action)
		if first == "" {
			first = requirement
		}
		switch action {
		case ActionLongPress, ActionPinch, ActionRotate, ActionDoubleTap:
			return requirement, true
		case ActionScroll:
			if step.Distance != nil || hasString(step.Label) || hasString(step.ID) || step.MaxSwipes != nil {
				return requirement, true
			}
		}
	}
	return first, false
}
