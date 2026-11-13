package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func findSubcommand(root *ffcli.Command, path ...string) *ffcli.Command {
	cmd := root
	for _, part := range path {
		var next *ffcli.Command
		for _, sub := range cmd.Subcommands {
			if sub.Name == part {
				next = sub
				break
			}
		}
		if next == nil {
			return nil
		}
		cmd = next
	}
	return cmd
}

func TestAppsWallSubmitRequiresConfirmUnlessDryRun(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"apps", "wall", "submit",
			"--app", "1234567890",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--confirm is required unless --dry-run is set") {
		t.Fatalf("expected confirm guidance in stderr, got %q", stderr)
	}
}

func TestAppsWallSubmitInvalidCountryReturnsUsageExitCode(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "country before app",
			args: []string{
				"apps", "wall", "submit",
				"--country", "zz",
				"--app", "1234567890",
				"--dry-run",
			},
		},
		{
			name: "country after dry run",
			args: []string{
				"apps", "wall", "submit",
				"--app", "1234567890",
				"--dry-run",
				"--country", "zz",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := captureOutput(t, func() {
				code := rootcmd.Run(test.args, "1.2.3")
				if code != rootcmd.ExitUsage {
					t.Fatalf("expected exit code %d, got %d", rootcmd.ExitUsage, code)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, "--country") {
				t.Fatalf("expected --country guidance in stderr, got %q", stderr)
			}
		})
	}
}

func TestAppsWallMarkdownColumnsExcludeIcon(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "wall.json")
	sourceJSON := `[
		{"app":"Alpha App","link":"https://example.com/alpha"},
		{"app":"Beta Mac","link":"https://example.com/beta"}
	]`
	if err := os.WriteFile(sourcePath, []byte(sourceJSON), 0o600); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	t.Setenv("ASC_WALL_SOURCE", sourcePath)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"apps", "wall", "--output", "markdown"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "| App") || !strings.Contains(stdout, "| Link") {
		t.Fatalf("expected markdown columns App/Link, got %q", stdout)
	}
	if strings.Contains(stdout, "| Creator |") || strings.Contains(stdout, "| Platform |") || strings.Contains(stdout, "| Icon |") {
		t.Fatalf("did not expect creator/platform/icon columns, got %q", stdout)
	}
}

func TestAppsWallCommunityUsesConfiguredSource(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "wall.json")
	sourceJSON := `[
		{"app":"Alpha App","link":"https://example.com/alpha"},
		{"app":"Zeta App","link":"https://example.com/zeta"},
		{"app":"Beta Mac","link":"https://example.com/beta"}
	]`
	if err := os.WriteFile(sourcePath, []byte(sourceJSON), 0o600); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	t.Setenv("ASC_WALL_SOURCE", sourcePath)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"apps", "wall",
			"--output", "json",
			"--sort", "-name",
			"--limit", "1",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var out struct {
		Data []struct {
			Name        string `json:"name"`
			AppStoreURL string `json:"appStoreUrl"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("parse json output: %v\nstdout: %s", err, stdout)
	}
	if len(out.Data) != 1 {
		t.Fatalf("expected one filtered entry, got %d", len(out.Data))
	}
	if out.Data[0].Name != "Zeta App" {
		t.Fatalf("expected Zeta App after -name sort with limit 1, got %q", out.Data[0].Name)
	}
	if out.Data[0].AppStoreURL != "https://example.com/zeta" {
		t.Fatalf("expected zeta link, got %q", out.Data[0].AppStoreURL)
	}
}

func TestAppsWallCommunityMissingSourceError(t *testing.T) {
	t.Setenv("ASC_WALL_SOURCE", filepath.Join(t.TempDir(), "missing-wall.json"))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"apps", "wall"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected command error, got nil")
	}
	if !strings.Contains(runErr.Error(), "apps wall: failed to read community wall source") {
		t.Fatalf("expected source read error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}
