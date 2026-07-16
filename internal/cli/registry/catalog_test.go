package registry

import (
	"testing"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"
)

func TestCatalogMetadataMatchesMaterializedCommands(t *testing.T) {
	t.Parallel()

	catalog := NewCatalog("test")
	metadata := catalog.MetadataCommands()
	if len(metadata) != len(catalog.factories) {
		t.Fatalf("metadata command count = %d, want %d", len(metadata), len(catalog.factories))
	}

	seen := make(map[string]struct{}, len(metadata))
	for i, factory := range catalog.factories {
		if _, ok := seen[factory.name]; ok {
			t.Fatalf("duplicate root command metadata for %q", factory.name)
		}
		seen[factory.name] = struct{}{}

		cmd := materialize(factory)
		if cmd == nil {
			t.Fatalf("factory %q returned nil", factory.name)
		}
		if metadata[i].Name != cmd.Name {
			t.Errorf("factory %q metadata name = %q, command name = %q", factory.name, metadata[i].Name, cmd.Name)
		}
		if metadata[i].ShortHelp != cmd.ShortHelp {
			t.Errorf("factory %q metadata help = %q, command help = %q", factory.name, metadata[i].ShortHelp, cmd.ShortHelp)
		}
		if metadata[i].UsageFunc == nil {
			t.Errorf("factory %q metadata command has nil UsageFunc", factory.name)
		}
	}
}

func TestCatalogCommandsForDoesNotBuildUnselectedFactory(t *testing.T) {
	t.Parallel()

	slowCalled := false
	catalog := &Catalog{factories: []factory{
		commandFactory("selected", "Selected command.", func() *ffcli.Command {
			return &ffcli.Command{Name: "selected", ShortHelp: "Selected command."}
		}),
		commandFactory("slow", "Slow command.", func() *ffcli.Command {
			slowCalled = true
			time.Sleep(10 * time.Second)
			return &ffcli.Command{Name: "slow", ShortHelp: "Slow command."}
		}),
	}}

	started := time.Now()
	commands := catalog.CommandsFor("selected")
	if elapsed := time.Since(started); elapsed >= time.Second {
		t.Fatalf("selected command construction took %s; unselected factory likely ran", elapsed)
	}
	if slowCalled {
		t.Fatal("unselected slow factory was invoked")
	}
	if commands[0].Name != "selected" || commands[1].Name != "slow" {
		t.Fatalf("root metadata order changed: %+v", commands)
	}
}

func BenchmarkCatalogCommandsForBuilds(b *testing.B) {
	catalog := NewCatalog("test")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = catalog.CommandsFor("builds")
	}
}
