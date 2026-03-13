package preorders

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"path/filepath"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestPreOrdersGetCommand_MissingApp(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	cmd := PreOrdersGetCommand()
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --app is missing, got %v", err)
	}
}

func TestPreOrdersListCommand_MissingAvailability(t *testing.T) {
	cmd := PreOrdersListCommand()

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --availability is missing, got %v", err)
	}
}

func TestPreOrdersEnableCommand_MissingFlags(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	tests := []struct {
		name string
		args []string
	}{
		{name: "missing app", args: []string{"--territory", "USA", "--release-date", "2026-01-20"}},
		{name: "missing territory", args: []string{"--app", "APP", "--release-date", "2026-01-20"}},
		{name: "missing release date", args: []string{"--app", "APP", "--territory", "USA"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := PreOrdersEnableCommand()
			if err := cmd.FlagSet.Parse(test.args); err != nil {
				t.Fatalf("failed to parse flags: %v", err)
			}

			if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", err)
			}
		})
	}
}

func TestPreOrdersEnableCommand_InvalidDate(t *testing.T) {
	cmd := PreOrdersEnableCommand()

	if err := cmd.FlagSet.Parse([]string{"--app", "APP", "--territory", "USA", "--release-date", "invalid"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if err == nil {
		t.Fatal("expected error for invalid release date")
	}
	if errors.Is(err, flag.ErrHelp) {
		t.Fatal("expected non-ErrHelp error for invalid release date")
	}
}

func TestPreOrdersUpdateCommand_MissingID(t *testing.T) {
	cmd := PreOrdersUpdateCommand()

	if err := cmd.FlagSet.Parse([]string{"--release-date", "2026-02-01"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --territory-availability is missing, got %v", err)
	}
}

func TestPreOrdersUpdateCommand_MissingAllAttributes(t *testing.T) {
	cmd := PreOrdersUpdateCommand()

	if err := cmd.FlagSet.Parse([]string{"--territory-availability", "ta-1"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when no attributes are provided, got %v", err)
	}
}

func TestPreOrdersUpdateCommand_InvalidDate(t *testing.T) {
	cmd := PreOrdersUpdateCommand()

	if err := cmd.FlagSet.Parse([]string{"--territory-availability", "ta-1", "--release-date", "invalid"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if err == nil {
		t.Fatal("expected error for invalid release date")
	}
	if errors.Is(err, flag.ErrHelp) {
		t.Fatal("expected non-ErrHelp error for invalid release date")
	}
}

func TestPreOrdersDisableCommand_MissingID(t *testing.T) {
	cmd := PreOrdersDisableCommand()

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --territory-availability is missing, got %v", err)
	}
}

func TestPreOrdersEndCommand_MissingIDs(t *testing.T) {
	cmd := PreOrdersEndCommand()

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --territory-availability is missing, got %v", err)
	}
}

func TestPreOrdersCommand_FlagDefinitions(t *testing.T) {
	getCmd := PreOrdersGetCommand()
	for _, name := range []string{"app", "output", "pretty"} {
		if getCmd.FlagSet.Lookup(name) == nil {
			t.Errorf("get: expected flag --%s to be defined", name)
		}
	}

	listCmd := PreOrdersListCommand()
	for _, name := range []string{"availability", "output", "pretty"} {
		if listCmd.FlagSet.Lookup(name) == nil {
			t.Errorf("list: expected flag --%s to be defined", name)
		}
	}

	enableCmd := PreOrdersEnableCommand()
	for _, name := range []string{"app", "territory", "release-date", "available-in-new-territories", "output", "pretty"} {
		if enableCmd.FlagSet.Lookup(name) == nil {
			t.Errorf("enable: expected flag --%s to be defined", name)
		}
	}

	updateCmd := PreOrdersUpdateCommand()
	for _, name := range []string{"territory-availability", "release-date", "pre-order-enabled", "available", "output", "pretty"} {
		if updateCmd.FlagSet.Lookup(name) == nil {
			t.Errorf("update: expected flag --%s to be defined", name)
		}
	}

	disableCmd := PreOrdersDisableCommand()
	for _, name := range []string{"territory-availability", "output", "pretty"} {
		if disableCmd.FlagSet.Lookup(name) == nil {
			t.Errorf("disable: expected flag --%s to be defined", name)
		}
	}

	endCmd := PreOrdersEndCommand()
	for _, name := range []string{"territory-availability", "output", "pretty"} {
		if endCmd.FlagSet.Lookup(name) == nil {
			t.Errorf("end: expected flag --%s to be defined", name)
		}
	}
}

func TestPreOrdersCommand_DefaultOutputJSON(t *testing.T) {
	commands := []*struct {
		name string
		cmd  func() *ffcli.Command
	}{
		{"get", PreOrdersGetCommand},
		{"list", PreOrdersListCommand},
		{"enable", PreOrdersEnableCommand},
		{"update", PreOrdersUpdateCommand},
		{"disable", PreOrdersDisableCommand},
		{"end", PreOrdersEndCommand},
	}

	for _, tc := range commands {
		t.Run(tc.name, func(t *testing.T) {
			cmd := tc.cmd()
			f := cmd.FlagSet.Lookup("output")
			if f == nil {
				t.Fatal("expected --output flag to be defined")
			}
			if f.DefValue != "json" {
				t.Errorf("expected --output default to be 'json', got %q", f.DefValue)
			}
		})
	}
}

func TestMapTerritoryAvailabilityIDs(t *testing.T) {
	relationships := asc.TerritoryAvailabilityRelationships{
		Territory: asc.Relationship{
			Data: asc.ResourceData{
				Type: asc.ResourceTypeTerritories,
				ID:   "usa",
			},
		},
	}
	relationshipsJSON, err := json.Marshal(relationships)
	if err != nil {
		t.Fatalf("failed to marshal relationships: %v", err)
	}

	resp := &asc.TerritoryAvailabilitiesResponse{
		Data: []asc.Resource[asc.TerritoryAvailabilityAttributes]{
			{
				Type:          asc.ResourceTypeTerritoryAvailabilities,
				ID:            "ta-1",
				Relationships: relationshipsJSON,
			},
		},
	}

	ids, err := shared.MapTerritoryAvailabilityIDs(resp)
	if err != nil {
		t.Fatalf("MapTerritoryAvailabilityIDs() error: %v", err)
	}
	if ids["USA"] != "ta-1" {
		t.Fatalf("expected territory USA to map to ta-1, got %q", ids["USA"])
	}
}

func TestMapTerritoryAvailabilityIDs_FallbackID(t *testing.T) {
	payload := `{"s":"6740467361","t":"USA"}`
	encoded := base64.RawStdEncoding.EncodeToString([]byte(payload))

	resp := &asc.TerritoryAvailabilitiesResponse{
		Data: []asc.Resource[asc.TerritoryAvailabilityAttributes]{
			{
				Type: asc.ResourceTypeTerritoryAvailabilities,
				ID:   encoded,
			},
		},
	}

	ids, err := shared.MapTerritoryAvailabilityIDs(resp)
	if err != nil {
		t.Fatalf("MapTerritoryAvailabilityIDs() error: %v", err)
	}
	if ids["USA"] != encoded {
		t.Fatalf("expected territory USA to map to %q, got %q", encoded, ids["USA"])
	}
}

func TestPreOrdersUpdateCommand_PreOrderEnabledAlone(t *testing.T) {
	cmd := PreOrdersUpdateCommand()

	if err := cmd.FlagSet.Parse([]string{"--territory-availability", "ta-1", "--pre-order-enabled", "true"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if errors.Is(err, flag.ErrHelp) {
		t.Fatal("did not expect flag.ErrHelp when --pre-order-enabled is set")
	}
}

func TestPreOrdersUpdateCommand_AvailableAlone(t *testing.T) {
	cmd := PreOrdersUpdateCommand()

	if err := cmd.FlagSet.Parse([]string{"--territory-availability", "ta-1", "--available", "false"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if errors.Is(err, flag.ErrHelp) {
		t.Fatal("did not expect flag.ErrHelp when --available is set")
	}
}

func TestPreOrdersUpdateCommand_DisablePreOrderRejectsReleaseDate(t *testing.T) {
	cmd := PreOrdersUpdateCommand()

	if err := cmd.FlagSet.Parse([]string{
		"--territory-availability", "ta-1",
		"--pre-order-enabled", "false",
		"--release-date", "2026-06-01",
	}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp for conflicting flags, got %v", err)
	}
}

func TestPreOrdersUpdateCommand_DisablePreOrderAlone(t *testing.T) {
	cmd := PreOrdersUpdateCommand()

	if err := cmd.FlagSet.Parse([]string{
		"--territory-availability", "ta-1",
		"--pre-order-enabled", "false",
	}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if errors.Is(err, flag.ErrHelp) {
		t.Fatal("did not expect flag.ErrHelp when --pre-order-enabled false is set")
	}
}
