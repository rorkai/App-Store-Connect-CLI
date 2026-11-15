package assetlibrary

import (
	"context"
	"errors"
	"flag"
	"io"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestCommandGroupHelp(t *testing.T) {
	cmd := NewCommand(newAssetLibraryTestDeps())
	if len(cmd.Subcommands) == 0 || cmd.UsageFunc == nil || !errors.Is(cmd.Exec(context.Background(), nil), flag.ErrHelp) {
		t.Fatal("expected read command group with standard help")
	}
	for _, child := range cmd.Subcommands {
		if child.Name == "upload" || child.Name == "submit" || child.Name == "delete" {
			t.Fatal("unverified mutation exposed")
		}
	}
}

// newAssetLibraryTestDeps builds the command tree with streams that discard
// its help.
func newAssetLibraryTestDeps() *shared.Deps {
	return &shared.Deps{IO: &shared.IO{Out: io.Discard, ErrOut: io.Discard}}
}
