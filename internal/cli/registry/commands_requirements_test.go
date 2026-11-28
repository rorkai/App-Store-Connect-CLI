package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestCommandInventoryRuntimeRequirements(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	t.Setenv("ASC_DEFAULT_OUTPUT", "json")
	var stdout bytes.Buffer
	deps := shared.DefaultDeps()
	deps.IO.Out = &stdout
	deps.Client = func() (*asc.Client, error) { t.Fatal("discovery created an API client"); return nil, nil }
	catalog := NewCatalogWithDeps("test", deps)
	for _, tt := range []struct {
		path, host, toolCoverage string
		tools                    []string
	}{
		{"xcode build", "darwin", "partial", []string{"xcrun", "xcodebuild"}},
		{"xcode archive", "darwin", "partial", []string{"xcrun", "xcodebuild"}},
		{"xcode export", "darwin", "partial", []string{"xcrun", "xcodebuild"}},
		{"xcode signing plan", "portable", "none", []string{}},
		{"signing resign", "darwin", "partial", []string{"security", "codesign"}},
		{"xcode signing apply", "", "", nil},
		{"commands", "", "", nil},
	} {
		t.Run(tt.path, func(t *testing.T) {
			stdout.Reset()
			if err := catalog.newCommandsCommand(deps).ParseAndRun(context.Background(), []string{"--family", tt.path}); err != nil {
				t.Fatal(err)
			}
			var inventory struct {
				Commands []struct {
					Requirements *struct {
						Host         string   `json:"host"`
						Tools        []string `json:"tools"`
						ToolCoverage string   `json:"toolCoverage"`
					} `json:"runtimeRequirements"`
				} `json:"commands"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &inventory); err != nil {
				t.Fatal(err)
			}
			if len(inventory.Commands) != 1 {
				t.Fatalf("commands = %d", len(inventory.Commands))
			}
			got := inventory.Commands[0].Requirements
			if tt.host == "" {
				if got != nil {
					t.Fatalf("undeclared command requirements must be omitted: %+v", got)
				}
			} else if got == nil || got.Host != tt.host || got.ToolCoverage != tt.toolCoverage || !reflect.DeepEqual(got.Tools, tt.tools) {
				t.Fatalf("runtime requirements = %+v, want host=%s tools=%v coverage=%s", got, tt.host, tt.tools, tt.toolCoverage)
			}
		})
	}
}
