package cmdtest

import (
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestMisplacedGlobalFlagExplainsRootPlacement(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		commandName string
		flagName    string
	}{
		{
			name:        "boolean flag",
			args:        []string{"apps", "list", "--debug"},
			commandName: "asc apps list",
			flagName:    "--debug",
		},
		{
			name:        "separate value",
			args:        []string{"apps", "list", "--report", "junit"},
			commandName: "asc apps list",
			flagName:    "--report",
		},
		{
			name:        "inline value",
			args:        []string{"apps", "list", "--report-file=results.xml"},
			commandName: "asc apps list",
			flagName:    "--report-file",
		},
		{
			name:        "nested command",
			args:        []string{"testflight", "groups", "list", "--api-debug"},
			commandName: "asc testflight groups list",
			flagName:    "--api-debug",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var code int
			stdout, stderr := captureOutput(t, func() {
				code = rootcmd.Run(test.args, "1.2.3")
			})

			if code != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			want := "Error: unknown flag `" + test.flagName + "` for `" + test.commandName + "`\n" +
				"`" + test.flagName + "` is a global flag; the flag and any required valid value must appear before the command name.\n" +
				"For help:\n  asc --help\n"
			if stderr != want {
				t.Fatalf("stderr = %q, want %q", stderr, want)
			}
		})
	}
}

func TestNonGlobalFlagSuggestionIsUnchanged(t *testing.T) {
	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{"apps", "list", "--bundle-idd", "com.example.app"}, "1.2.3")
	})

	if code != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "Try:\n  --bundle-id\n") {
		t.Fatalf("stderr = %q, want command flag suggestion", stderr)
	}
	if strings.Contains(stderr, "global flag") {
		t.Fatalf("stderr = %q, want no global classification", stderr)
	}
}

func TestMalformedGlobalFlagGuidanceDoesNotPromiseValueCorrection(t *testing.T) {
	for _, args := range [][]string{
		{"apps", "list", "--report"},
		{"apps", "list", "--debug=maybe"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var code int
			stdout, stderr := captureOutput(t, func() {
				code = rootcmd.Run(args, "1.2.3")
			})

			if code != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, "is a global flag; the flag and any required valid value must appear before the command name.\n") {
				t.Fatalf("stderr = %q, want value-qualified placement guidance", stderr)
			}
			if !strings.Contains(stderr, "For help:\n  asc --help\n") {
				t.Fatalf("stderr = %q, want root help", stderr)
			}
		})
	}
}
