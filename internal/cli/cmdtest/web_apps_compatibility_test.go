package cmdtest

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestWebAppsCompatibilityCommandSurface(t *testing.T) {
	root := RootCommand("1.2.3")

	group := findSubcommand(root, "web", "apps", "compatibility")
	if group == nil {
		t.Fatal("expected web apps compatibility command")
	}
	if findSubcommand(root, "web", "apps", "compatibility", "view") == nil {
		t.Fatal("expected web apps compatibility view command")
	}
	if findSubcommand(root, "web", "apps", "compatibility", "edit") == nil {
		t.Fatal("expected web apps compatibility edit command")
	}
}

func TestWebAppsCompatibilityInvalidBooleanExitCodes(t *testing.T) {
	bin := buildCLIBinary(t)

	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name: "web mac flag",
			args: []string{
				"web", "apps", "compatibility", "edit",
				"--app", "app-1",
				"--ios-app-on-mac=maybe",
			},
			wantStderr: `invalid value "maybe" for flag -ios-app-on-mac: must be true or false`,
		},
		{
			name: "web vision pro flag",
			args: []string{
				"web", "apps", "compatibility", "edit",
				"--app", "app-1",
				"--ios-app-on-vision-pro=maybe",
			},
			wantStderr: `invalid value "maybe" for flag -ios-app-on-vision-pro: must be true or false`,
		},
		{
			name: "flag value matches subcommand name",
			args: []string{
				"web", "apps", "compatibility", "edit",
				"--app", "app-1",
				"--ios-app-on-mac=edit",
			},
			wantStderr: `invalid value "edit" for flag -ios-app-on-mac: must be true or false`,
		},
		{
			name: "mixed flag order",
			args: []string{
				"web", "apps", "compatibility", "edit",
				"--ios-app-on-vision-pro=maybe",
				"--app", "app-1",
			},
			wantStderr: `invalid value "maybe" for flag -ios-app-on-vision-pro: must be true or false`,
		},
		{
			name: "flag before subcommands",
			args: []string{
				"--ios-app-on-mac=maybe",
				"web", "apps", "compatibility", "edit",
				"--app", "app-1",
			},
			wantStderr: "flag provided but not defined: -ios-app-on-mac",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := exec.Command(bin, test.args...)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				t.Fatalf("expected exit error, got %v", err)
			}
			if code := exitErr.ExitCode(); code != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
			}
			if stdout.String() != "" {
				t.Fatalf("expected empty stdout, got %q", stdout.String())
			}
			if !strings.Contains(stderr.String(), test.wantStderr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantStderr, stderr.String())
			}
		})
	}
}
