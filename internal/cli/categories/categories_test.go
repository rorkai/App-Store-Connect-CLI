package categories

import (
	"bytes"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestCategoriesSetCommand_SubcategoryFlagsRegistered(t *testing.T) {
	d, _, _ := newTestDeps()
	cmd := newCategoriesSetCommand(d)

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

func newTestDeps() (*shared.Deps, *bytes.Buffer, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer
	return &shared.Deps{IO: &shared.IO{Out: &stdout, ErrOut: &stderr}}, &stdout, &stderr
}
