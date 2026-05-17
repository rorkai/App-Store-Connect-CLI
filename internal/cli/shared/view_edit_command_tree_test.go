package shared

import (
	"context"
	"errors"
	"flag"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"
)

func TestRenameLeafVerbRenamesCanonicalFlagSetPath(t *testing.T) {
	fs := flag.NewFlagSet("apps get", flag.ContinueOnError)
	id := ""
	fs.StringVar(&id, "id", "", "App ID")

	cmd := &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc apps get --id APP_ID",
		ShortHelp:  "Get an app.",
		FlagSet:    fs,
	}

	replacements := renameLeafVerb(cmd, "asc apps get", "view")
	if cmd.FlagSet == nil {
		t.Fatal("expected canonical flagset to remain available")
	}
	if len(replacements) != 2 {
		t.Fatalf("expected command replacements, got %+v", replacements)
	}

	if got := cmd.FlagSet.Name(); got != "apps view" {
		t.Fatalf("expected canonical flagset name %q, got %q", "apps view", got)
	}
	if err := cmd.FlagSet.Parse([]string{"--id", "app-123"}); err != nil {
		t.Fatalf("expected canonical parse to succeed, got %v", err)
	}
	if id != "app-123" {
		t.Fatalf("expected canonical parse to update shared flag value, got %q", id)
	}
}

func TestNormalizeViewEditCommandTreeRemovesLegacyVerbPath(t *testing.T) {
	root := &ffcli.Command{
		Name:       "apps",
		ShortUsage: "asc apps <subcommand> [flags]",
		Subcommands: []*ffcli.Command{
			{
				Name:       "get",
				ShortUsage: "asc apps get --id APP_ID",
				ShortHelp:  "Get an app.",
				FlagSet:    flag.NewFlagSet("apps get", flag.ContinueOnError),
				Exec: func(context.Context, []string) error {
					return errors.New("apps get: failed to fetch app")
				},
			},
		},
	}

	NormalizeViewEditCommandTree(root, nil)

	var canonical *ffcli.Command
	for _, sub := range root.Subcommands {
		if sub != nil && sub.Name == "view" {
			canonical = sub
			break
		}
	}
	if canonical == nil {
		t.Fatal("expected canonical view command to remain in tree")
		return
	}
	for _, sub := range root.Subcommands {
		if sub != nil && sub.Name == "get" {
			t.Fatal("expected legacy get alias to be removed from the tree")
		}
	}

	runErr := canonical.Exec(context.Background(), nil)
	if runErr == nil {
		t.Fatal("expected canonical execution to return rewritten error")
	}
	if runErr.Error() != "apps view: failed to fetch app" {
		t.Fatalf("expected rewritten canonical error, got %q", runErr.Error())
	}
}
