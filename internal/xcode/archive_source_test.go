package xcode

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchiveObservesProjectCheckoutBeforeBuild(t *testing.T) {
	for _, mode := range []string{"clean", "tracked change", "untracked file", "no commit"} {
		t.Run(mode, func(t *testing.T) {
			checkout := t.TempDir()
			project := filepath.Join(checkout, "Demo.xcodeproj")
			if err := os.Mkdir(project, 0o755); err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(project, "project.pbxproj")
			if err := os.WriteFile(file, []byte("fixture"), 0o600); err != nil {
				t.Fatal(err)
			}
			var environment []string
			for _, value := range os.Environ() {
				if !strings.HasPrefix(strings.ToUpper(value), "GIT_") {
					environment = append(environment, value)
				}
			}
			environment = append(environment, "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
			git := func(args ...string) string {
				t.Helper()
				command := exec.Command("git", append([]string{"-C", checkout, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false", "-c", "core.hooksPath=" + t.TempDir()}, args...)...)
				command.Env = environment
				output, err := command.CombinedOutput()
				if err != nil {
					t.Fatalf("git %s: %v: %s", args[0], err, output)
				}
				return strings.TrimSpace(string(output))
			}
			git("init", "--quiet")
			revision := ""
			if mode != "no commit" {
				git("add", ".")
				git("commit", "--quiet", "--message", "Fixture")
				revision = git("rev-parse", "HEAD")
			}
			if mode == "tracked change" || mode == "untracked file" {
				if mode == "untracked file" {
					file = filepath.Join(checkout, "New.swift")
				}
				if err := os.WriteFile(file, []byte("changed"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "unrelated.git"))
			t.Setenv("GIT_WORK_TREE", t.TempDir())
			t.Cleanup(overrideTestEnvironment(t))
			runtimeGOOS = "darwin"
			lookPathFn = func(string) (string, error) { return "/usr/bin/xcodebuild", nil }
			logPath := filepath.Join(t.TempDir(), "commands.log")
			commandContextFn = helperCommandContext(t, logPath)
			result, err := Archive(context.Background(), ArchiveOptions{
				ProjectPath: project, Scheme: "Demo", SourceProvenance: true,
				ArchivePath: filepath.Join(checkout, "Output.xcarchive"),
			})
			if mode == "no commit" {
				if err == nil || !strings.Contains(err.Error(), "committed HEAD") {
					t.Fatalf("expected actionable checkout error, got %v", err)
				}
				commands, _ := os.ReadFile(logPath)
				if strings.Contains(string(commands), "|archive|") {
					t.Fatal("archive started before source preflight failed")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if result.ProjectCheckout == nil || result.ProjectCheckout.Revision != revision || result.ProjectCheckout.Dirty != (mode != "clean") || result.ProjectCheckout.ObservedAt != "buildStart" {
				t.Fatalf("checkout observation = %+v", result.ProjectCheckout)
			}
			if mode == "clean" && git("status", "--porcelain") == "" {
				t.Fatal("fixture must become dirty during the build to verify observation timing")
			}
		})
	}
}
