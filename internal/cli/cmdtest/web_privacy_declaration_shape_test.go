package cmdtest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

// A declaration file in the wrong shape is rejected before any web session
// is resolved, and the error states the shape plan and apply accept.
func TestWebPrivacyDeclarationShapeErrorsStateTheExpectedShape(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	const expectedShape = `each dataUsages entry has only "category", "purposes" (array) and "dataProtections" (array)`
	tests := []struct {
		name        string
		command     string
		contents    string
		wantProblem string
	}{
		{
			name:        "plan names an invented field",
			command:     "plan",
			contents:    `{"schemaVersion":1,"dataUsages":[{"type":"collected","category":"NAME","purpose":"APP_FUNCTIONALITY","linkedToIdentity":true,"tracking":false}]}`,
			wantProblem: `unknown field "type"`,
		},
		{
			name:        "plan names the singular purpose field",
			command:     "plan",
			contents:    `{"schemaVersion":1,"dataUsages":[{"category":"NAME","purpose":"APP_FUNCTIONALITY","dataProtections":["DATA_LINKED_TO_YOU"]}]}`,
			wantProblem: `unknown field "purpose"`,
		},
		{
			name:        "apply names purposes given as a string",
			command:     "apply",
			contents:    `{"schemaVersion":1,"dataUsages":[{"category":"NAME","purposes":"APP_FUNCTIONALITY","dataProtections":["DATA_LINKED_TO_YOU"]}]}`,
			wantProblem: `cannot unmarshal string`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "privacy.json")
			if err := os.WriteFile(path, []byte(test.contents), 0o600); err != nil {
				t.Fatalf("write declaration: %v", err)
			}

			stdout, stderr, code := runWithDeps(t, nil, "web", "privacy", test.command, "--app", "123456789", "--file", path)
			if code != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d; stderr = %q", code, rootcmd.ExitUsage, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			firstLine, _, _ := strings.Cut(stderr, "\n")
			for _, want := range []string{
				"Error: invalid privacy declaration JSON: ",
				test.wantProblem,
				expectedShape,
				`"DATA_USED_TO_TRACK_YOU"`,
				"asc web privacy pull --app APP_ID --out FILE",
			} {
				if !strings.Contains(firstLine, want) {
					t.Fatalf("first stderr line = %q, want it to contain %q", firstLine, want)
				}
			}
		})
	}
}
