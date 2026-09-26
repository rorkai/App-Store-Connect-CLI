package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestXcodeSigningPlanApplyCommandsExist(t *testing.T) {
	root := RootCommand("1.2.3")
	xcode := findSubcommand(root, "xcode")
	if xcode == nil {
		t.Fatal("expected xcode command group")
	}

	if strings.Contains(xcode.ShortHelp, "macOS only") ||
		!strings.Contains(xcode.LongHelp, "Signing-plan generation is cross-platform") ||
		!strings.Contains(xcode.LongHelp, "Windows before modifying project or receipt files") {
		t.Fatalf("xcode root help has incorrect platform scope: %q", xcode.ShortHelp+"\n"+xcode.LongHelp)
	}

	group := findSubcommand(root, "xcode", "signing")
	if group == nil {
		t.Fatal("expected xcode signing command group")
	}

	plan := findSubcommand(root, "xcode", "signing", "plan")
	if plan == nil {
		t.Fatal("expected xcode signing plan command")
	}
	apply := findSubcommand(root, "xcode", "signing", "apply")
	if apply == nil {
		t.Fatal("expected xcode signing apply command")
	}
}

func TestXcodeSigningRequiredFlagsReportConciseDiagnostics(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		parameter string
		message   string
	}{
		{name: "plan project", args: []string{"xcode", "signing", "plan"}, parameter: "--project"},
		{name: "plan settings file", args: []string{"xcode", "signing", "plan", "--project", "App.xcodeproj"}, parameter: "--settings-file", message: "--settings-file or --profile"},
		{name: "apply plan", args: []string{"xcode", "signing", "apply"}, parameter: "--plan"},
		{name: "apply confirm", args: []string{"xcode", "signing", "apply", "--plan", "plan.json"}, parameter: "--confirm"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})
			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("run error = %v, want usage error", runErr)
			}
			diagnostic, ok := shared.DiagnosticFromError(runErr)
			if !ok || diagnostic.Code != shared.DiagnosticRequiredInputMissing || diagnostic.Parameter != test.parameter {
				t.Fatalf("diagnostic = %+v, found=%t, want missing %s", diagnostic, ok, test.parameter)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			message := test.parameter
			if test.message != "" {
				message = test.message
			}
			wantDiagnostic := "Error: " + message + " is required\n"
			if !strings.HasPrefix(stderr, wantDiagnostic) || strings.Count(stderr, wantDiagnostic) != 1 {
				t.Fatalf("stderr = %q, want one leading %q diagnostic", stderr, wantDiagnostic)
			}
		})
	}
}
