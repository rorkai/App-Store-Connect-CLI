package asc

import (
	"strings"
	"testing"
)

// The artifact-shape tests for these outputs live in internal/cli/xcode,
// because comparing against the domain artifact requires internal/xcode and
// that package imports internal/asc. These tests cover the registered output
// types and their renderers without the domain dependency.

func TestXcodeSigningOutputsUseRegisteredHumanRenderers(t *testing.T) {
	plan := &XcodeSigningPlanOutput{
		Ready:    true,
		PlanPath: "/tmp/plan.json",
		PlanHash: "plan-hash",
		Changes: []XcodeSigningSettingChangeOutput{{
			Target:        "Demo",
			Configuration: "Release",
			Setting:       "CODE_SIGN_STYLE",
			Operation:     "set",
			Resolution:    "direct",
		}},
	}
	apply := &XcodeSigningApplyOutput{
		Completed:    true,
		PlanPath:     "/tmp/plan.json",
		ReceiptPath:  "/tmp/receipt.json",
		PlanHash:     "plan-hash",
		ChangedFiles: []string{"/tmp/Demo.xcodeproj/project.pbxproj"},
	}

	for _, test := range []struct {
		name string
		data any
		want []string
	}{
		{name: "plan", data: plan, want: []string{"ready", "plan hash", "changes"}},
		{name: "apply", data: apply, want: []string{"completed", "receipt", "changed files"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, renderer := range []struct {
				name string
				fn   func(any) error
			}{
				{name: "table", fn: PrintTable},
				{name: "markdown", fn: PrintMarkdown},
			} {
				renderer := renderer
				t.Run(renderer.name, func(t *testing.T) {
					output := captureStdout(t, func() error { return renderer.fn(test.data) })
					for _, want := range test.want {
						if !strings.Contains(strings.ToLower(output), want) {
							t.Fatalf("output missing %q: %s", want, output)
						}
					}
				})
			}
		})
	}
}

func TestXcodeSigningApplyOutputDoesNotClaimIncompleteApplyWasApplied(t *testing.T) {
	output := &XcodeSigningApplyOutput{
		Completed: false,
		PlanPath:  "/tmp/plan.json",
	}

	rendered := captureStdout(t, func() error { return PrintTable(output) })
	if strings.Contains(strings.ToLower(rendered), "applied") {
		t.Fatalf("incomplete apply output made an applied claim: %s", rendered)
	}
	if !strings.Contains(strings.ToLower(rendered), "completed") {
		t.Fatalf("incomplete apply output omitted completion state: %s", rendered)
	}
}
