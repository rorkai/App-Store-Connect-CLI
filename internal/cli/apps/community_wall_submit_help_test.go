package apps

import (
	"flag"
	"strings"
	"testing"
)

func TestAppsWallSubmitHelpMentionsPublicAppStoreLookup(t *testing.T) {
	cmd := AppsWallSubmitCommand(flag.NewFlagSet("wall", flag.ContinueOnError))
	if cmd == nil {
		t.Fatal("expected apps wall submit command")
	}

	usage := cmd.UsageFunc(cmd)
	for _, want := range []string{
		"Use --app for the normal flow",
		"resolves the public App Store name, URL,",
		"and icon from the app ID automatically",
		"use --link with --name instead",
	} {
		if !strings.Contains(usage, want) {
			t.Fatalf("expected usage to contain %q, got %q", want, usage)
		}
	}
}
