package screenshots

import "testing"

func TestScreenshotsCommandConstructors(t *testing.T) {
	cmd := ScreenshotsCommand()
	if cmd == nil {
		t.Fatal("expected screenshots command")
	}
	if cmd.Name != "screenshots" {
		t.Fatalf("expected command name screenshots, got %q", cmd.Name)
	}
	if len(cmd.Subcommands) == 0 {
		t.Fatal("expected screenshots subcommands")
	}
	if got := Command(); got == nil {
		t.Fatal("expected Command wrapper to return command")
	}
}
