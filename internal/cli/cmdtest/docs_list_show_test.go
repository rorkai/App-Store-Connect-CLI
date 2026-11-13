package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
)

func TestDocsListOutputsEmbeddedGuidesAsJSON(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"docs", "list"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var guides []struct {
		Slug        string `json:"slug"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal([]byte(stdout), &guides); err != nil {
		t.Fatalf("failed to unmarshal guides JSON: %v\nstdout=%s", err, stdout)
	}

	if len(guides) != 4 {
		t.Fatalf("expected 4 guides, got %d", len(guides))
	}

	expectedSlugs := []string{"api-notes", "reference", "workflows", "third-party-notices"}
	for i, expectedSlug := range expectedSlugs {
		if guides[i].Slug != expectedSlug {
			t.Fatalf("expected guide %d slug %q, got %q", i, expectedSlug, guides[i].Slug)
		}
		if strings.TrimSpace(guides[i].Description) == "" {
			t.Fatalf("expected guide %q to include a description", guides[i].Slug)
		}
	}
}

func TestDocsListSupportsTextOutputs(t *testing.T) {
	tests := []struct {
		output string
		want   []string
	}{
		{output: "markdown", want: []string{"api-notes", "reference", "workflows"}},
		{output: "table", want: []string{"slug", "description", "api-notes", "reference", "workflows"}},
	}

	for _, test := range tests {
		t.Run(test.output, func(t *testing.T) {
			stdout := runDocsCommand(t, "docs", "list", "--output", test.output)
			for _, want := range test.want {
				if !strings.Contains(stdout, want) {
					t.Fatalf("expected %s output to contain %q, got %q", test.output, want, stdout)
				}
			}
		})
	}
}

func TestDocsShowPrintsGuide(t *testing.T) {
	tests := []struct {
		guide string
		want  []string
	}{
		{
			guide: "api-notes",
			want:  []string{"# API Notes", "Finance reports use Apple fiscal months", "Sandbox Testers"},
		},
		{
			guide: "reference",
			want:  []string{"# asc cli reference", "Publish to App Store (canonical)", "Submission readiness (canonical)", `asc status --app "APP_ID"`},
		},
		{
			guide: "workflows",
			want:  []string{"# Workflow Patterns", "asc publish appstore", "asc publish testflight"},
		},
	}

	for _, test := range tests {
		t.Run(test.guide, func(t *testing.T) {
			stdout := runDocsCommand(t, "docs", "show", test.guide)
			for _, want := range test.want {
				if !strings.Contains(stdout, want) {
					t.Fatalf("expected %s guide to contain %q, got %q", test.guide, want, stdout)
				}
			}
		})
	}
}

func runDocsCommand(t *testing.T, args ...string) string {
	t.Helper()

	stdout, stderr, err := runCommand(t, args)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	return stdout
}

func TestDocsShowUnknownGuideReturnsUsageError(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"docs", "show", "unknown-guide"}); err != nil {
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
	if !strings.Contains(stderr, "unknown guide") {
		t.Fatalf("expected unknown guide error message, got %q", stderr)
	}
	if !strings.Contains(stderr, "api-notes") || !strings.Contains(stderr, "reference") || !strings.Contains(stderr, "workflows") {
		t.Fatalf("expected stderr to list available guides, got %q", stderr)
	}
}

func TestDocsShowRequiresGuideName(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"docs", "show"}); err != nil {
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
}
