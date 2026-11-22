package cmd

import (
	"strings"
	"testing"
)

// TestResolveFlagAliasesSpellsJSONAsOutputJSON pins `--json` as the second
// spelling of `--output json` on a command whose output selector offers json:
// it is rewritten before parsing, kept once next to the same choice, and a
// usage error next to a different one.
func TestResolveFlagAliasesSpellsJSONAsOutputJSON(t *testing.T) {
	tests := []struct {
		name    string
		args    string
		want    string
		wantErr string
	}{
		{name: "bare", args: "builds list --app 1 --json", want: "builds list --app 1 --output json"},
		{name: "before other flags", args: "builds list --json --app 1", want: "builds list --output json --app 1"},
		{name: "inline true", args: "builds list --app 1 --json=true", want: "builds list --app 1 --output json"},
		{name: "with the same output", args: "builds list --app 1 --output json --json", want: "builds list --app 1 --output json"},
		{name: "with pretty", args: "apps list --json --pretty", want: "apps list --output json --pretty"},
		{name: "with another output", args: "builds list --app 1 --output table --json", wantErr: "`--output` and `--json` are the same flag; pass one value"},
	}
	root := RootCommand("1.0.0")
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := resolveFlagAliases(root, strings.Fields(test.args), nil)
			if test.wantErr != "" {
				if err == nil || err.Error() != test.wantErr {
					t.Fatalf("resolveFlagAliases(%q) error = %v, want %q", test.args, err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveFlagAliases(%q) error = %v", test.args, err)
			}
			if strings.Join(got, " ") != test.want {
				t.Fatalf("resolveFlagAliases(%q) = %q, want %q", test.args, strings.Join(got, " "), test.want)
			}
		})
	}
}

// TestRunRejectsAFalseJSONFlag keeps `--json=false` from being read as a
// request for the default format: it is a usage error.
func TestRunRejectsAFalseJSONFlag(t *testing.T) {
	resetReportFlags(t)
	var code int
	_, stderr := captureCommandOutput(t, func() {
		code = Run([]string{"apps", "list", "--json=false"}, "1.0.0")
	})
	if code != ExitUsage {
		t.Fatalf("Run() exit code = %d, want %d; stderr %q", code, ExitUsage, stderr)
	}
	if !strings.Contains(stderr, "--json takes no value") {
		t.Fatalf("stderr = %q, want the --json value error", stderr)
	}
}
