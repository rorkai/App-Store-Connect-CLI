package initcmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/docs"
)

func TestInitCommandMetadata(t *testing.T) {
	cmd := InitCommand()
	if cmd == nil {
		t.Fatal("expected init command")
	}
	if cmd.Name != "init" {
		t.Fatalf("expected command name init, got %q", cmd.Name)
	}
	if cmd.ShortUsage != "asc init [flags]" {
		t.Fatalf("unexpected short usage %q", cmd.ShortUsage)
	}
	if cmd.UsageFunc == nil {
		t.Fatal("expected usage func to be set")
	}
}

func TestInitCommandPrefixesErrors(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("create .git: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "ASC.md"), []byte("# Existing\n"), 0o644); err != nil {
		t.Fatalf("write ASC.md: %v", err)
	}

	cmd := InitCommand()
	if err := cmd.FlagSet.Parse([]string{"--path", repo}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), nil)
	if err == nil {
		t.Fatal("expected command to fail when ASC.md exists")
	}
	if !errors.Is(err, docs.ErrASCReferenceExists) {
		t.Fatalf("expected ErrASCReferenceExists, got %v", err)
	}
	if !strings.Contains(err.Error(), "init:") {
		t.Fatalf("expected init error prefix, got %v", err)
	}
}
