package cmdtest

import (
	"context"
	"errors"
	"flag"
	"strings"
	"testing"
)

func TestProductPagesCustomPagesListRequiresApp(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"product-pages", "custom-pages", "list"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--app is required") {
		t.Fatalf("expected missing app error, got %q", stderr)
	}
}

func TestProductPagesCustomPagesDeleteRequiresConfirm(t *testing.T) {
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"product-pages", "custom-pages", "delete", "--custom-page-id", "page-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--confirm is required") {
		t.Fatalf("expected missing confirm error, got %q", stderr)
	}
}

func TestProductPagesExperimentsCreateRequiresVersionID(t *testing.T) {
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"product-pages", "experiments", "create", "--name", "Icon Test", "--traffic-proportion", "50"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--version-id is required") {
		t.Fatalf("expected missing version-id error, got %q", stderr)
	}
}

func TestProductPagesExperimentsCreateRequiresAppForV2(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"product-pages", "experiments", "create", "--v2", "--platform", "IOS", "--name", "Icon Test", "--traffic-proportion", "50"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--app is required") {
		t.Fatalf("expected missing app error, got %q", stderr)
	}
}

func TestProductPagesExperimentsDeleteRequiresConfirm(t *testing.T) {
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"product-pages", "experiments", "delete", "--experiment-id", "exp-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--confirm is required") {
		t.Fatalf("expected missing confirm error, got %q", stderr)
	}
}

func TestProductPagesCustomPagesListRejectsInvalidLimit(t *testing.T) {
	root := RootCommand("1.2.3")

	tests := []struct {
		name  string
		limit string
	}{
		{
			name:  "limit below range",
			limit: "-1",
		},
		{
			name:  "limit above range",
			limit: "201",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{"product-pages", "custom-pages", "list", "--app", "APP_ID", "--limit", test.limit}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if errors.Is(err, flag.ErrHelp) {
					t.Fatalf("unexpected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	}
}

func TestProductPagesCustomPagesListRejectsInvalidNextURL(t *testing.T) {
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"product-pages", "custom-pages", "list", "--app", "APP_ID", "--next", "not-a-url"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if errors.Is(err, flag.ErrHelp) {
			t.Fatalf("unexpected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestProductPagesCustomPagesLocalizationsPreviewSetsListRejectsInvalidLimit(t *testing.T) {
	root := RootCommand("1.2.3")

	tests := []struct {
		name  string
		limit string
	}{
		{
			name:  "limit below range",
			limit: "-1",
		},
		{
			name:  "limit above range",
			limit: "201",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{"product-pages", "custom-pages", "localizations", "preview-sets", "list", "--localization-id", "loc-1", "--limit", test.limit}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if errors.Is(err, flag.ErrHelp) {
					t.Fatalf("unexpected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	}
}

func TestProductPagesCustomPagesLocalizationsPreviewSetsListRejectsInvalidNextURL(t *testing.T) {
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"product-pages", "custom-pages", "localizations", "preview-sets", "list", "--localization-id", "loc-1", "--next", "not-a-url"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if errors.Is(err, flag.ErrHelp) {
			t.Fatalf("unexpected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestProductPagesCustomPagesLocalizationsScreenshotSetsListRejectsInvalidLimit(t *testing.T) {
	root := RootCommand("1.2.3")

	tests := []struct {
		name  string
		limit string
	}{
		{
			name:  "limit below range",
			limit: "-1",
		},
		{
			name:  "limit above range",
			limit: "201",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{"product-pages", "custom-pages", "localizations", "screenshot-sets", "list", "--localization-id", "loc-1", "--limit", test.limit}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if errors.Is(err, flag.ErrHelp) {
					t.Fatalf("unexpected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	}
}

func TestProductPagesCustomPagesLocalizationsScreenshotSetsListRejectsInvalidNextURL(t *testing.T) {
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"product-pages", "custom-pages", "localizations", "screenshot-sets", "list", "--localization-id", "loc-1", "--next", "not-a-url"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if errors.Is(err, flag.ErrHelp) {
			t.Fatalf("unexpected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestProductPagesCustomPagesLocalizationsSearchKeywordsListRequiresLocalizationID(t *testing.T) {
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"product-pages", "custom-pages", "localizations", "search-keywords", "list"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--localization-id is required") {
		t.Fatalf("expected missing localization-id error, got %q", stderr)
	}
}

func TestProductPagesCustomPagesLocalizationsSearchKeywordsAddRequiresLocalizationID(t *testing.T) {
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"product-pages", "custom-pages", "localizations", "search-keywords", "add", "--keywords", "kw-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--localization-id is required") {
		t.Fatalf("expected missing localization-id error, got %q", stderr)
	}
}

func TestProductPagesCustomPagesLocalizationsSearchKeywordsAddRequiresKeywords(t *testing.T) {
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"product-pages", "custom-pages", "localizations", "search-keywords", "add", "--localization-id", "loc-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--keywords is required") {
		t.Fatalf("expected missing keywords error, got %q", stderr)
	}
}

func TestProductPagesCustomPagesLocalizationsSearchKeywordsDeleteRequiresConfirm(t *testing.T) {
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"product-pages", "custom-pages", "localizations", "search-keywords", "delete", "--localization-id", "loc-1", "--keywords", "kw-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--confirm is required") {
		t.Fatalf("expected missing confirm error, got %q", stderr)
	}
}

func TestProductPagesCustomPagesLocalizationsSearchKeywordsDeleteRequiresKeywords(t *testing.T) {
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"product-pages", "custom-pages", "localizations", "search-keywords", "delete", "--localization-id", "loc-1", "--confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--keywords is required") {
		t.Fatalf("expected missing keywords error, got %q", stderr)
	}
}

func TestProductPagesCustomPagesLocalizationsPreviewSetsListRequiresLocalizationID(t *testing.T) {
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"product-pages", "custom-pages", "localizations", "preview-sets", "list"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--localization-id is required") {
		t.Fatalf("expected missing localization-id error, got %q", stderr)
	}
}

func TestProductPagesCustomPagesLocalizationsScreenshotSetsListRequiresLocalizationID(t *testing.T) {
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"product-pages", "custom-pages", "localizations", "screenshot-sets", "list"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--localization-id is required") {
		t.Fatalf("expected missing localization-id error, got %q", stderr)
	}
}
