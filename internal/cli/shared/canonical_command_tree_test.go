package shared

import (
	"context"
	"errors"
	"flag"
	"reflect"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"
)

func TestPrepareCanonicalCommandTreeDoesNotRewriteCanonicalCommand(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("apps view: failed to fetch app")
	view := &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc apps view --id APP_ID",
		ShortHelp:  "View an app.",
		FlagSet:    flag.NewFlagSet("apps view", flag.ContinueOnError),
		Exec: func(context.Context, []string) error {
			return wantErr
		},
	}
	root := &ffcli.Command{
		Name:        "apps",
		Subcommands: []*ffcli.Command{view},
	}

	PrepareCanonicalCommandTree(root, nil)

	if view.Name != "view" || view.ShortUsage != "asc apps view --id APP_ID" || view.ShortHelp != "View an app." {
		t.Fatalf("canonical command text changed: %+v", view)
	}
	if view.FlagSet.Name() != "apps view" {
		t.Fatalf("canonical flag set changed: %q", view.FlagSet.Name())
	}
	if got := view.Exec(context.Background(), nil); !errors.Is(got, wantErr) || reflect.TypeOf(got) != reflect.TypeOf(wantErr) {
		t.Fatalf("canonical error was wrapped or replaced: got %T %v, want %T %v", got, got, wantErr, wantErr)
	}
}

func TestPrepareCanonicalCommandTreePreservesRemovedVerbGuidance(t *testing.T) {
	t.Parallel()

	root := &ffcli.Command{
		Name: "apps",
		Subcommands: []*ffcli.Command{
			{Name: "view", Exec: func(context.Context, []string) error { return nil }},
		},
		Exec: func(context.Context, []string) error { return flag.ErrHelp },
	}

	PrepareCanonicalCommandTree(root, nil)
	if err := root.Exec(context.Background(), []string{"get"}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("removed get command error = %v, want flag.ErrHelp", err)
	}
}
