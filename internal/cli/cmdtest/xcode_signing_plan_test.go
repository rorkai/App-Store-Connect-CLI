package cmdtest

import (
	"errors"
	"flag"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

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
			stdout, stderr, runErr := runRootWithDeps(t, nil, test.args...)
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
