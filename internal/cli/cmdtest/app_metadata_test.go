package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
)

func TestAppsSearchKeywordsValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "apps search-keywords list missing app",
			args:    []string{"apps", "search-keywords", "list"},
			wantErr: "--app is required",
		},
		{
			name:    "apps search-keywords set missing app",
			args:    []string{"apps", "search-keywords", "set", "--keywords", "kw1", "--confirm"},
			wantErr: "--app is required",
		},
		{
			name:    "apps search-keywords set missing confirm",
			args:    []string{"apps", "search-keywords", "set", "--app", "123", "--keywords", "kw1"},
			wantErr: "--confirm is required",
		},
		{
			name:    "apps search-keywords set missing keywords",
			args:    []string{"apps", "search-keywords", "set", "--app", "123", "--confirm"},
			wantErr: "--keywords is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
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
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestLocalizationsSearchKeywordsValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "localizations search-keywords list missing localization",
			args:    []string{"localizations", "search-keywords", "list"},
			wantErr: "--localization-id is required",
		},
		{
			name:    "localizations search-keywords add missing keywords",
			args:    []string{"localizations", "search-keywords", "add", "--localization-id", "loc-1"},
			wantErr: "--keywords is required",
		},
		{
			name:    "localizations search-keywords delete missing confirm",
			args:    []string{"localizations", "search-keywords", "delete", "--localization-id", "loc-1", "--keywords", "kw1"},
			wantErr: "--confirm is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
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
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestLocalizationsMediaSetsValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "preview sets list missing localization",
			args:    []string{"localizations", "preview-sets", "list"},
			wantErr: "--localization-id is required",
		},
		{
			name:    "preview sets relationships missing localization",
			args:    []string{"localizations", "preview-sets", "relationships"},
			wantErr: "--localization-id is required",
		},
		{
			name:    "preview sets get missing id",
			args:    []string{"localizations", "preview-sets", "get"},
			wantErr: "--id is required",
		},
		{
			name:    "screenshot sets list missing localization",
			args:    []string{"localizations", "screenshot-sets", "list"},
			wantErr: "--localization-id is required",
		},
		{
			name:    "screenshot sets relationships missing localization",
			args:    []string{"localizations", "screenshot-sets", "relationships"},
			wantErr: "--localization-id is required",
		},
		{
			name:    "screenshot sets get missing id",
			args:    []string{"localizations", "screenshot-sets", "get"},
			wantErr: "--id is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
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
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestLocalizationsMediaSetsOutputErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "preview sets get unsupported output",
			args: []string{"localizations", "preview-sets", "get", "--id", "SET_ID", "--output", "yaml"},
		},
		{
			name: "preview sets get pretty with table",
			args: []string{"localizations", "preview-sets", "get", "--id", "SET_ID", "--output", "table", "--pretty"},
		},
		{
			name: "screenshot sets get unsupported output",
			args: []string{"localizations", "screenshot-sets", "get", "--id", "SET_ID", "--output", "yaml"},
		},
		{
			name: "screenshot sets get pretty with markdown",
			args: []string{"localizations", "screenshot-sets", "get", "--id", "SET_ID", "--output", "markdown", "--pretty"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected non-help error, got %v", err)
				}
			})

			_ = stdout
			_ = stderr
		})
	}
}

func TestVersionsRelationshipsValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "versions relationships missing type",
			args:    []string{"versions", "relationships", "--version-id", "id-1"},
			wantErr: "--type is required",
		},
		{
			name:    "versions relationships missing version id",
			args:    []string{"versions", "relationships", "--type", "appStoreReviewDetail"},
			wantErr: "--version-id is required",
		},
		{
			name:    "versions relationships invalid type",
			args:    []string{"versions", "relationships", "--version-id", "id-1", "--type", "nope"},
			wantErr: "--type must be one of",
		},
		{
			name:    "versions relationships invalid limit for single",
			args:    []string{"versions", "relationships", "--version-id", "id-1", "--type", "appStoreReviewDetail", "--limit", "10"},
			wantErr: "--limit, --next, and --paginate are only valid for to-many relationships",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
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
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestVersionsRelatedCommandsValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "versions experiments v2 list missing version id",
			args: []string{"versions", "experiments-v2", "list"},
		},
		{
			name: "versions customer reviews list missing version id",
			args: []string{"versions", "customer-reviews", "list"},
		},
		{
			name: "versions app clip default experience get missing version id",
			args: []string{"versions", "app-clip-default-experience", "get"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
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
			if stderr == "" {
				t.Fatalf("expected stderr output")
			}
		})
	}
}

func TestCategoriesValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "categories get missing id",
			args:    []string{"categories", "get"},
			wantErr: "--category-id is required",
		},
		{
			name:    "categories parent missing id",
			args:    []string{"categories", "parent"},
			wantErr: "--category-id is required",
		},
		{
			name:    "categories subcategories missing id",
			args:    []string{"categories", "subcategories"},
			wantErr: "--category-id is required",
		},
		{
			name:    "categories subcategories invalid limit",
			args:    []string{"categories", "subcategories", "--category-id", "GAMES", "--limit", "201"},
			wantErr: "--limit must be between 1 and 200",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
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
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestAppInfoIncludeValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "app-info id without include",
			args:    []string{"app-info", "get", "--app-info", "info-1"},
			wantErr: "--app-info requires --include",
		},
		{
			name:    "app-info include missing app",
			args:    []string{"app-info", "get", "--include", "ageRatingDeclaration"},
			wantErr: "--app or --app-info is required",
		},
		{
			name:    "app-info include with version flags",
			args:    []string{"app-info", "get", "--include", "ageRatingDeclaration", "--app", "123", "--version", "1.2.3", "--platform", "IOS"},
			wantErr: "--include cannot be used with version localization flags",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
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
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}
