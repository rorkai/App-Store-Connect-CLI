package assets

import (
	"strings"
	"testing"
)

func TestAssetsCommandConstructors(t *testing.T) {
	top := AssetsCommand()
	if top == nil {
		t.Fatal("expected assets command")
	}
	if top.Name == "" {
		t.Fatal("expected command name")
	}
	if len(top.Subcommands) == 0 {
		t.Fatal("expected assets subcommands")
	}
	if !strings.Contains(top.ShortHelp, "app preview videos") {
		t.Fatalf("expected assets short help to mention app preview videos, got %q", top.ShortHelp)
	}

	if got := Command(); got == nil {
		t.Fatal("expected Command wrapper to return command")
	}

	if got := AssetsScreenshotsCommand(); got == nil {
		t.Fatal("expected screenshots command")
	}
	if got := AssetsPreviewsCommand(); got == nil {
		t.Fatal("expected previews command")
	} else if !strings.Contains(got.ShortHelp, "app preview videos") {
		t.Fatalf("expected previews short help to mention app preview videos, got %q", got.ShortHelp)
	}
}
