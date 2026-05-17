package categories

import (
	"context"
	"errors"
	"flag"
	"testing"
)

func TestCategoriesCommandShape(t *testing.T) {
	cmd := CategoriesCommand()
	if cmd == nil {
		t.Fatal("expected categories command")
		return
	}
	if cmd.Name != "categories" {
		t.Fatalf("unexpected command name: %q", cmd.Name)
	}
	if len(cmd.Subcommands) != 5 {
		t.Fatalf("expected 5 subcommands, got %d", len(cmd.Subcommands))
	}
	if got := CategoriesCommand(); got == nil {
		t.Fatal("expected Command wrapper to return command")
	}
}

func TestCategoriesValidationErrors(t *testing.T) {
	t.Run("list invalid limit", func(t *testing.T) {
		cmd := CategoriesListCommand()
		if err := cmd.FlagSet.Parse([]string{"--limit", "0"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := cmd.Exec(context.Background(), nil)
		if err == nil || errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected non-ErrHelp error for invalid limit, got %v", err)
		}
	})

	t.Run("get missing category-id", func(t *testing.T) {
		cmd := CategoriesGetCommand()
		if err := cmd.FlagSet.Parse([]string{}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	t.Run("parent missing category-id", func(t *testing.T) {
		cmd := CategoriesParentCommand()
		if err := cmd.FlagSet.Parse([]string{}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	t.Run("subcategories missing category-id and next", func(t *testing.T) {
		cmd := CategoriesSubcategoriesCommand()
		if err := cmd.FlagSet.Parse([]string{}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})
}

func TestCategoriesSetCommand_SubcategoryFlagsRegistered(t *testing.T) {
	cmd := CategoriesSetCommand()

	for _, name := range []string{
		"primary-subcategory-one",
		"primary-subcategory-two",
		"secondary-subcategory-one",
		"secondary-subcategory-two",
	} {
		if cmd.FlagSet.Lookup(name) == nil {
			t.Fatalf("expected --%s flag to be registered", name)
		}
	}
}
