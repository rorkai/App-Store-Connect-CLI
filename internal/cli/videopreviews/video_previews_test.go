package videopreviews

import "testing"

func TestVideoPreviewsCommandConstructors(t *testing.T) {
	cmd := VideoPreviewsCommand()
	if cmd == nil {
		t.Fatal("expected video-previews command")
	}
	if cmd.Name != "video-previews" {
		t.Fatalf("expected command name video-previews, got %q", cmd.Name)
	}
	if len(cmd.Subcommands) == 0 {
		t.Fatal("expected video-previews subcommands")
	}
	if got := Command(); got == nil {
		t.Fatal("expected Command wrapper to return command")
	}
}
