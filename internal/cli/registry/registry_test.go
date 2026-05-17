package registry

import (
	"context"
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	cmd := VersionCommand("v1.2.3")
	if cmd == nil {
		t.Fatal("expected non-nil version command")
		return
	}
	if cmd.Name != "version" {
		t.Fatalf("unexpected command name: %q", cmd.Name)
	}
	if err := cmd.Exec(context.Background(), nil); err != nil {
		t.Fatalf("expected version command exec to succeed, got %v", err)
	}
}

func TestSubcommandsIncludesCoreEntries(t *testing.T) {
	subs := Subcommands("dev")
	if len(subs) == 0 {
		t.Fatal("expected non-empty root subcommands")
	}

	names := make(map[string]struct{}, len(subs))
	for _, sub := range subs {
		if sub == nil {
			t.Fatal("expected no nil root subcommands")
			return
		}
		name := strings.TrimSpace(sub.Name)
		if name == "" {
			t.Fatal("expected all root subcommands to have names")
		}
		names[name] = struct{}{}
	}

	required := []string{"auth", "doctor", "account", "insights", "builds", "reviews", "version", "completion"}
	for _, name := range required {
		if _, ok := names[name]; !ok {
			t.Fatalf("expected root subcommands to include %q", name)
		}
	}
}
