package cmdtest

import (
	"errors"
	"flag"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// TestXcodeCloudBuildRunsListMissingWorkflowIDPointsAtDiscoveryCommand asserts
// that the bare invocation tells callers how to obtain a workflow ID instead of
// dead-ending on the bare required-flag message.
func TestXcodeCloudBuildRunsListMissingWorkflowIDPointsAtDiscoveryCommand(t *testing.T) {
	for _, args := range [][]string{
		{"xcode-cloud", "build-runs"},
		{"xcode-cloud", "build-runs", "list"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			_, stderr, runErr := runRootWithDeps(t, nil, args...)

			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected ErrHelp, got %v", runErr)
			}
			if !strings.Contains(stderr, "Error: --workflow-id is required.") {
				t.Fatalf("expected required-flag error in stderr, got %q", stderr)
			}
			if !strings.Contains(stderr, `Find workflow IDs with: asc xcode-cloud workflows list --app "APP_ID"`) {
				t.Fatalf("expected workflow discovery hint in stderr, got %q", stderr)
			}
			if strings.ContainsAny(stderr, "<>") {
				t.Fatalf("workflow discovery hint must not contain shell redirection characters: %q", stderr)
			}

			diagnostic, ok := shared.DiagnosticFromError(runErr)
			if !ok {
				t.Fatalf("expected structured diagnostic, got %v", runErr)
			}
			if diagnostic.Code != shared.DiagnosticRequiredInputMissing || diagnostic.Parameter != "--workflow-id" {
				t.Fatalf("diagnostic = %+v, want required_input_missing for --workflow-id", diagnostic)
			}
		})
	}
}

// TestXcodeCloudParentListMissingIDsKeepUnhintedMessage guards the sibling
// commands that share the parent-list helper: they must keep the plain
// required-flag message and their own structured parameter.
func TestXcodeCloudParentListMissingIDsKeepUnhintedMessage(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		wantParameter string
	}{
		{
			name:          "build run builds",
			args:          []string{"xcode-cloud", "build-runs", "builds"},
			wantParameter: "--run-id",
		},
		{
			name:          "build action list",
			args:          []string{"xcode-cloud", "actions", "list"},
			wantParameter: "--run-id",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, stderr, runErr := runRootWithDeps(t, nil, test.args...)

			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected ErrHelp, got %v", runErr)
			}
			if !strings.Contains(stderr, "Error: "+test.wantParameter+" is required\n") {
				t.Fatalf("expected plain required-flag error in stderr, got %q", stderr)
			}
			if strings.Contains(stderr, "Find workflow IDs with") {
				t.Fatalf("did not expect workflow hint for %v, got %q", test.args, stderr)
			}

			diagnostic, ok := shared.DiagnosticFromError(runErr)
			if !ok {
				t.Fatalf("expected structured diagnostic, got %v", runErr)
			}
			if diagnostic.Code != shared.DiagnosticRequiredInputMissing || diagnostic.Parameter != test.wantParameter {
				t.Fatalf("diagnostic = %+v, want required_input_missing for %q", diagnostic, test.wantParameter)
			}
		})
	}
}
