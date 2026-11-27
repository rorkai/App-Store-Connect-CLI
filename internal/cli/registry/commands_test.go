package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"slices"
	"strings"
	"testing"
	"time"

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
		{Name: "--file", TakesValue: true, Description: "Read the file used by `asc review details plan`", FlagMetadata: shared.FlagMetadata{Type: "string", DefaultKind: "unknown"}},
		{Name: "--pretty", Description: "Indent output", FlagMetadata: shared.FlagMetadata{Type: "boolean", DefaultKind: "unknown"}},
	}
	if !slices.Equal(plan.Flags, wantFlags) || bytes.Contains(stdout.Bytes(), []byte("private-default")) {
		t.Fatalf("expected public flags with intact descriptions and no defaults, got %s", &stdout)
	}
}

// inventoryOpaqueValue catches attempts to serialize an arbitrary Getter value.
type inventoryOpaqueValue struct{}

func (inventoryOpaqueValue) String() string   { return "PRIVATE_OPAQUE_DEFAULT" }
func (inventoryOpaqueValue) Set(string) error { return nil }
func (inventoryOpaqueValue) Get() any         { return inventoryUnserializable{} }

type inventoryUnserializable struct{}

func (inventoryUnserializable) MarshalJSON() ([]byte, error) {
	panic("command discovery serialized an arbitrary Getter value")
}

func TestCommandInventoryDescribesTypesAndOnlyDeclaredDefaults(t *testing.T) {
	t.Setenv("ASC_DEFAULT_OUTPUT", "table")
	var stdout, stderr bytes.Buffer
	deps := shared.DefaultDeps()
	deps.IO.Out, deps.IO.ErrOut = &stdout, &stderr
	catalog := &Catalog{factories: []factory{commandFactory("example", "Example.", func() *ffcli.Command {
		fs := flag.NewFlagSet("example", flag.ContinueOnError)
		fs.String("secret", "PRIVATE_INITIAL_DEFAULT", "A private input")
		fs.Bool("confirm", false, "Confirm")
		fs.Int("limit", 20, "Limit")
		fs.Uint64("bytes", 1024, "Bytes")
		fs.Float64("ratio", 0.5, "Ratio")
		fs.Duration("timeout", 5*time.Second, "Timeout")
		fs.Var(inventoryOpaqueValue{}, "opaque", "Custom input")
		shared.BindResourceIDFlag(fs, "id", "apps", "Resource ID")
		shared.BindOnceCSVFlag(fs, "fields", "Fields")
		shared.BindMetadataOutputFlags(fs)
		if err := fs.Set("id", "PRIVATE_CURRENT_VALUE"); err != nil {
			t.Fatal(err)
		}
		dynamic := flag.NewFlagSet("dynamic", flag.ContinueOnError)
		deps.BindOutputFlags(dynamic)
		return &ffcli.Command{Name: "example", FlagSet: fs, Subcommands: []*ffcli.Command{{Name: "dynamic", FlagSet: dynamic}}}
	})}}
	if err := catalog.newCommandsCommand(deps).ParseAndRun(context.Background(), []string{"--family", "example"}); err != nil {
		t.Fatal(err)
	}
	var output struct {
		Commands []struct {
			Path  []string         `json:"path"`
			Flags []map[string]any `json:"flags"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatal(err)
	}
	if len(output.Commands) != 2 || stderr.Len() != 0 {
		t.Fatalf("commands=%d stderr=%q", len(output.Commands), stderr.String())
	}
	wantTypes := map[string]string{"--secret": "string", "--confirm": "boolean", "--limit": "integer", "--bytes": "integer", "--ratio": "number", "--timeout": "duration", "--opaque": "unknown", "--id": "string", "--fields": "string", "--output-format": "string", "--pretty": "boolean"}
	if len(output.Commands[0].Flags) != len(wantTypes) {
		t.Fatalf("flag count = %d, want %d", len(output.Commands[0].Flags), len(wantTypes))
	}
	for _, option := range output.Commands[0].Flags {
		name := option["name"].(string)
		if option["type"] != wantTypes[name] {
			t.Errorf("%s type=%v want=%s", name, option["type"], wantTypes[name])
		}
		wantDefault, declared := map[string]string{"--id": "", "--fields": "", "--output-format": "json"}[name]
		if declared {
			if option["defaultKind"] != "static" || option["default"] != wantDefault {
				t.Errorf("%s lacks its declared static default", name)
			}
		} else if _, present := option["default"]; present || option["defaultKind"] != "unknown" {
			t.Errorf("%s exposes an undeclared default", name)
		}
	}
	for _, option := range output.Commands[1].Flags {
		if option["name"] == "--output" {
			_, present := option["default"]
			if option["type"] != "string" || option["defaultKind"] != "dynamic" || present {
				t.Error("environment-dependent output default was not marked dynamic without its value")
			}
		}
	}
	for _, private := range []string{"PRIVATE_INITIAL_DEFAULT", "PRIVATE_CURRENT_VALUE", "PRIVATE_OPAQUE_DEFAULT"} {
		if bytes.Contains(stdout.Bytes(), []byte(private)) {
			t.Fatal("command discovery leaked a private value")
		}
	}
}
