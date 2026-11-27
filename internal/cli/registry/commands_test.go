package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"slices"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestCommandInventoryOmitsRemovedAliasesFromFamilyExpansion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	deps := shared.DefaultDeps()
	deps.IO.Out, deps.IO.ErrOut = &stdout, &stderr
	catalog := NewCatalogWithDeps("test", deps)
	for _, family := range []string{"migrate", "migrate metadata"} {
		stdout.Reset()
		if err := catalog.newCommandsCommand(deps).ParseAndRun(context.Background(), []string{"--family", family}); err != nil {
			t.Fatal(err)
		}
		var inventory commandInventory
		if err := json.Unmarshal(stdout.Bytes(), &inventory); err != nil {
			t.Fatal(err)
		}
		paths := make([]string, 0, len(inventory.Commands))
		for _, entry := range inventory.Commands {
			paths = append(paths, strings.Join(entry.Path, " "))
		}
		want := []string{"migrate", "migrate import", "migrate export", "migrate validate"}
		if family == "migrate metadata" {
			want = []string{"migrate metadata"}
		}
		slices.Sort(paths)
		slices.Sort(want)
		if !slices.Equal(paths, want) || stderr.Len() != 0 {
			t.Fatalf("family %q: paths=%v want=%v stderr=%q", family, paths, want, &stderr)
		}
	}
}

func TestCommandInventoryOnlyMaterializesRequestedFamily(t *testing.T) {
	requested := 0
	catalog := &Catalog{factories: []factory{
		commandFactory("other", "Other family.", func() *ffcli.Command {
			t.Fatal("inventory built an unrelated command")
			return nil
		}),
		commandFactory("review", "Review commands.", func() *ffcli.Command {
			requested++
			fs := flag.NewFlagSet("plan", flag.ContinueOnError)
			fs.String("file", "private-default", "Read the file used by `asc review details plan`")
			fs.Bool("pretty", false, "Indent output")
			fs.String("private", "", "Hidden input")
			shared.HideFlagFromHelp(fs.Lookup("private"))
			return &ffcli.Command{
				Name: "review",
				Subcommands: []*ffcli.Command{{
					Name: "details",
					Subcommands: []*ffcli.Command{
						{Name: "plan", FlagSet: fs},
						{Name: "set"},
					},
				}},
			}
		}),
	}}
	var stdout, stderr bytes.Buffer
	deps := shared.DefaultDeps()
	deps.IO.Out, deps.IO.ErrOut = &stdout, &stderr

	if err := catalog.newCommandsCommand(deps).ParseAndRun(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	var index commandInventory
	if err := json.Unmarshal(stdout.Bytes(), &index); err != nil {
		t.Fatal(err)
	}
	if requested != 0 || index.SchemaVersion != 1 || len(index.Families) != 2 || len(index.Commands) != 0 {
		t.Fatalf("unexpected family index: %+v, materializations: %d", index, requested)
	}

	stdout.Reset()
	args := []string{"--family", "review details", "--output", "json"}
	if err := catalog.newCommandsCommand(deps).ParseAndRun(context.Background(), args); err != nil {
		t.Fatal(err)
	}
	var family commandInventory
	if err := json.Unmarshal(stdout.Bytes(), &family); err != nil {
		t.Fatal(err)
	}
	if requested != 1 || family.Family != "review details" || len(family.Commands) != 3 || len(family.Families) != 0 {
		t.Fatalf("unexpected scoped inventory: %+v, materializations: %d", family, requested)
	}
	plan, apply := family.Commands[1], family.Commands[2]
	if !slices.Equal(plan.Path, []string{"review", "details", "plan"}) || plan.WriteIntent != "none" || apply.WriteIntent != "remote" {
		t.Fatalf("unexpected plan and apply metadata: %+v, %+v", plan, apply)
	}
	wantFlags := []commandFlag{
		{Name: "--file", TakesValue: true, Description: "Read the file used by `asc review details plan`"},
		{Name: "--pretty", Description: "Indent output"},
	}
	if !slices.Equal(plan.Flags, wantFlags) || bytes.Contains(stdout.Bytes(), []byte("private-default")) {
		t.Fatalf("expected public flags with intact descriptions and no defaults, got %s", &stdout)
	}
}
