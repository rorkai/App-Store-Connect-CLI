package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestCommandInventoryConstraintsMatchRuntimeValidation(t *testing.T) {
	for _, tt := range []struct {
		name, path, diagnostic string
		args                   []string
		rule                   map[string]any
		acceptedArgs           []string
	}{
		{"required ID", "app-events localizations view", "--localization-id is required", nil, map[string]any{"kind": "required", "flags": []string{"--localization-id"}}, nil},
		{"required locale", "app-events localizations create", "--locale is required", []string{"--event-id", "event"}, map[string]any{"kind": "required", "flags": []string{"--event-id", "--locale"}}, nil},
		{"parent or cursor", "app-events localizations list", "--event-id is required", nil, map[string]any{"kind": "anyOf", "flags": []string{"--event-id", "--next"}}, nil},
		{"cursor conflict", "app-events localizations list", "--next cannot be combined with --limit", []string{"--next", "https://api.appstoreconnect.apple.com/v1/appEvents/event/localizations?cursor=x", "--limit", "0"}, map[string]any{"kind": "conflicts", "whenNonEmpty": "--next", "flags": []string{"--limit", "--include", "--placements-limit"}}, nil},
		{"update alternatives", "app-events localizations update", "at least one update flag is required", []string{"--localization-id", "localization"}, map[string]any{"kind": "anyOf", "flags": []string{"--name", "--short-description", "--long-description"}}, nil},
		{"text length", "app-events localizations create", "--name must be at most 30 characters", []string{"--event-id", "event", "--locale", "en-US", "--name", strings.Repeat("n", 31)}, map[string]any{"kind": "maxLength", "flags": []string{"--name"}, "maximum": 30, "unit": "unicodeCodePoints", "trimSpace": true}, nil},
		{"page limit", "app-events localizations list", "--limit must be between 1 and 200", []string{"--event-id", "event", "--limit", "201"}, map[string]any{"kind": "integerRange", "flags": []string{"--limit"}, "minimum": 0, "maximum": 200}, nil},
		{"output values", "app-events localizations view", "--output must be one of: json, table, markdown", []string{"--localization-id", "localization", "--output", "yaml"}, map[string]any{"kind": "values", "flags": []string{"--output"}, "values": []string{"json", "table", "markdown"}, "aliases": map[string]string{"md": "markdown"}, "caseInsensitive": true, "trimSpace": true, "emptyValue": "json"}, nil},
		{"pretty output", "app-events localizations view", "--pretty is only valid with JSON output", []string{"--localization-id", "localization", "--output", "table", "--pretty"}, map[string]any{"kind": "requiresValue", "flags": []string{"--output"}, "values": []string{"json"}, "whenTrue": "--pretty"}, nil},
		{"explicit empty update", "marketplace search-details update", "at least one update flag is required", []string{"--search-detail-id", "detail"}, map[string]any{"kind": "anyOf", "providedFlags": []string{"--catalog-url"}}, []string{"--search-detail-id", "detail", "--catalog-url", ""}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
			t.Setenv("ASC_APP_ID", "")
			t.Setenv("ASC_DEFAULT_OUTPUT", "json")
			var stdout, stderr bytes.Buffer
			deps := shared.DefaultDeps()
			deps.IO.Out, deps.IO.ErrOut = &stdout, &stderr
			calls := 0
			boundary := errors.New("client boundary")
			deps.Client = func() (*asc.Client, error) { calls++; return nil, boundary }
			catalog := NewCatalogWithDeps("test", deps)
			if err := catalog.newCommandsCommand(deps).ParseAndRun(context.Background(), []string{"--family", tt.path}); err != nil {
				t.Fatal(err)
			}
			var inventory struct {
				Commands []struct {
					Constraints struct {
						Coverage string           `json:"coverage"`
						Rules    []map[string]any `json:"rules"`
					} `json:"constraints"`
				} `json:"commands"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &inventory); err != nil {
				t.Fatal(err)
			}
			if len(inventory.Commands) != 1 || inventory.Commands[0].Constraints.Coverage != "partial" {
				t.Fatalf("missing partial constraint metadata: %s", stdout.String())
			}
			want, _ := json.Marshal(tt.rule)
			found := slices.ContainsFunc(inventory.Commands[0].Constraints.Rules, func(rule map[string]any) bool {
				got, _ := json.Marshal(rule)
				return bytes.Equal(got, want)
			})
			if !found {
				t.Fatalf("missing declared rule %s in %s", want, stdout.String())
			}
			stdout.Reset()
			path := strings.Fields(tt.path)
			command := findExactCommand(catalog.CommandsFor(path[0]), path)
			shared.WrapCommandOutputValidation(command, deps)
			err := command.ParseAndRun(context.Background(), tt.args)
			if (!errors.Is(err, flag.ErrHelp) && !shared.IsReportedUsageError(err)) || !strings.Contains(stderr.String(), tt.diagnostic) || calls != 0 || stdout.Len() != 0 {
				t.Fatalf("err=%v stderr=%q client calls=%d stdout=%q", err, stderr.String(), calls, stdout.String())
			}
			if tt.acceptedArgs != nil {
				command = findExactCommand(catalog.CommandsFor(path[0]), path)
				if err := command.ParseAndRun(context.Background(), tt.acceptedArgs); !errors.Is(err, boundary) || calls != 1 {
					t.Fatalf("explicit empty update did not pass validation: err=%v calls=%d", err, calls)
				}
			}
		})
	}
}

func TestCommandInventoryDoesNotInventRequiredFlagsForRuntimeAppFallback(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	t.Setenv("ASC_APP_ID", "runtime-app")
	t.Setenv("ASC_DEFAULT_OUTPUT", "json")
	var stdout, stderr bytes.Buffer
	deps := shared.DefaultDeps()
	deps.IO.Out, deps.IO.ErrOut = &stdout, &stderr
	calls := 0
	boundary := errors.New("client boundary")
	deps.Client = func() (*asc.Client, error) { calls++; return nil, boundary }
	catalog := NewCatalogWithDeps("test", deps)
	for _, family := range []string{"commands", "encryption declarations list"} {
		stdout.Reset()
		if err := catalog.newCommandsCommand(deps).ParseAndRun(context.Background(), []string{"--family", family}); err != nil {
			t.Fatal(err)
		}
		var result struct {
			Commands []struct {
				Constraints struct {
					Coverage     string           `json:"coverage"`
					Rules        []map[string]any `json:"rules"`
					DynamicRules []string         `json:"dynamicRules"`
				} `json:"constraints"`
			} `json:"commands"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if len(result.Commands) != 1 {
			t.Fatalf("commands = %d", len(result.Commands))
		}
		metadata := result.Commands[0].Constraints
		if family == "commands" {
			if metadata.Coverage != "unknown" || len(metadata.Rules) != 0 {
				t.Fatalf("undeclared constraints were presented as known: %+v", metadata)
			}
			continue
		}
		if metadata.Coverage != "partial" || !slices.Contains(metadata.DynamicRules, "appIdFallback") {
			t.Fatalf("runtime fallback not marked dynamic: %+v", metadata)
		}
		for _, rule := range metadata.Rules {
			if rule["kind"] == "required" || rule["kind"] == "anyOf" {
				t.Fatalf("fallback does not require a CLI flag: %+v", rule)
			}
		}
	}
	command := findExactCommand(catalog.CommandsFor("encryption"), []string{"encryption", "declarations", "list"})
	if err := command.ParseAndRun(context.Background(), nil); !errors.Is(err, boundary) || calls != 1 {
		t.Fatalf("runtime fallback did not pass input validation: err=%v calls=%d", err, calls)
	}
}
