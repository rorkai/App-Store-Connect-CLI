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

	if len(guides) != 2 {
		t.Fatalf("expected 2 guides, got %d", len(guides))
	}

	expectedSlugs := []string{"api-notes", "reference"}
	for i, expectedSlug := range expectedSlugs {
		if guides[i].Slug != expectedSlug {
			t.Fatalf("expected guide %d slug %q, got %q", i, expectedSlug, guides[i].Slug)
		}
		if strings.TrimSpace(guides[i].Description) == "" {
			t.Fatalf("expected guide %q to include a description", guides[i].Slug)
		}
	}
}

func TestDocsListSupportsMarkdownOutput(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"docs", "list", "--output", "markdown"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	for _, slug := range []string{"api-notes", "reference"} {
		if !strings.Contains(stdout, slug) {
			t.Fatalf("expected markdown output to contain %q, got %q", slug, stdout)
		}
	}
	if strings.Contains(stdout, "workflows") {
		t.Fatalf("expected workflows guide to be removed, got %q", stdout)
	}
}

func TestDocsListSupportsTableOutput(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"docs", "list", "--output", "table"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "slug") || !strings.Contains(stdout, "description") {
		t.Fatalf("expected table output headers, got %q", stdout)
	}
	if !strings.Contains(stdout, "api-notes") || !strings.Contains(stdout, "reference") {
		t.Fatalf("expected table output to include all guide slugs, got %q", stdout)
	}
	if strings.Contains(stdout, "workflows") {
		t.Fatalf("expected workflows guide to be removed, got %q", stdout)
	}
}

func TestDocsShowPrintsAPINotesGuide(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"docs", "show", "api-notes"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "# API Notes") {
		t.Fatalf("expected API notes heading in output, got %q", stdout)
	}
	if !strings.Contains(stdout, "Finance reports use Apple fiscal months") {
		t.Fatalf("expected API notes guide to cover finance quirks, got %q", stdout)
	}
	if !strings.Contains(stdout, "Sandbox Testers") {
		t.Fatalf("expected API notes guide to keep sandbox guidance discoverable, got %q", stdout)
	}
}

func TestDocsShowPrintsReferenceGuide(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"docs", "show", "reference"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "# asc cli reference") {
		t.Fatalf("expected reference guide heading, got %q", stdout)
	}
	if !strings.Contains(stdout, "Release (full pipeline)") {
		t.Fatalf("expected reference guide to lead with release pipeline guidance, got %q", stdout)
	}
	if !strings.Contains(stdout, "Submit for review (low-level)") {
		t.Fatalf("expected reference guide to keep low-level submit guidance discoverable, got %q", stdout)
	}
	if !strings.Contains(stdout, `asc status --app "APP_ID"`) {
		t.Fatalf("expected reference guide to mention status monitoring, got %q", stdout)
	}
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
	if !strings.Contains(stderr, "api-notes") || !strings.Contains(stderr, "reference") {
		t.Fatalf("expected stderr to list available guides, got %q", stderr)
	}
	if strings.Contains(stderr, "workflows") {
		t.Fatalf("expected workflows guide to be removed from available guides, got %q", stderr)
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
