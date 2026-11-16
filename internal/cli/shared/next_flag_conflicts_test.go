package shared

import (
	"errors"
	"flag"
	"testing"
)

func TestRejectNextFlagConflictsDetectsExplicitDefaultValue(t *testing.T) {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	_ = fs.Int("limit", 0, "")
	_ = fs.String("output", "json", "")
	if err := fs.Parse([]string{"--limit", "0"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	var err error
	_, stderr := captureOutput(t, func() {
		err = DefaultDeps().RejectNextFlagConflicts(
			fs,
			"https://api.appstoreconnect.apple.com/v1/resources?cursor=next",
			"resources list",
			"limit",
		)
	})
	if err == nil {
		t.Fatal("RejectNextFlagConflicts() error = nil, want conflict")
	}
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("RejectNextFlagConflicts() error = %v, want usage error", err)
	}
	if got, want := err.Error(), "resources list: --next cannot be combined with --limit"; got != want {
		t.Fatalf("RejectNextFlagConflicts() error = %q, want %q", got, want)
	}
	if stderr != "Error: resources list: --next cannot be combined with --limit\n" {
		t.Fatalf("RejectNextFlagConflicts() stderr = %q", stderr)
	}
	diagnostic, ok := DiagnosticFromError(err)
	if !ok || diagnostic.Code != DiagnosticConflictingInput || diagnostic.Parameter != "--limit" {
		t.Fatalf("DiagnosticFromError() = %+v, %t", diagnostic, ok)
	}
}

func TestRejectNextFlagConflictsAllowsUnrelatedFlags(t *testing.T) {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	_ = fs.Int("limit", 0, "")
	_ = fs.String("output", "json", "")
	if err := fs.Parse([]string{"--output", "table"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if err := DefaultDeps().RejectNextFlagConflicts(
		fs,
		"https://api.appstoreconnect.apple.com/v1/resources?cursor=next",
		"resources list",
		"limit",
	); err != nil {
		t.Fatalf("RejectNextFlagConflicts() error = %v", err)
	}
}

func TestRejectNextFlagConflictsAllowsQueryFlagsWithoutNext(t *testing.T) {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	_ = fs.Int("limit", 0, "")
	if err := fs.Parse([]string{"--limit", "25"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if err := DefaultDeps().RejectNextFlagConflicts(fs, "", "resources list", "limit"); err != nil {
		t.Fatalf("RejectNextFlagConflicts() error = %v", err)
	}
}
