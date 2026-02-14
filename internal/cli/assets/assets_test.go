package assets

import (
	"strings"
	"testing"
)

func TestAssetsSubcommandConstructors(t *testing.T) {
	if got := AssetsScreenshotsCommand(); got == nil {
		t.Fatal("expected screenshots command")
	}
	if got := AssetsPreviewsCommand(); got == nil {
		t.Fatal("expected previews command")
	} else if !strings.Contains(got.ShortHelp, "app preview videos") {
		t.Fatalf("expected previews short help to mention app preview videos, got %q", got.ShortHelp)
	}
}
