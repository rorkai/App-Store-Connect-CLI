package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestReviewDeprecatedItemSurfacesAreRemoved(t *testing.T) {
	root := RootCommand("4.0.0")

	for _, path := range [][]string{
		{"review", "items", "view"},
		{"review", "items-get"},
	} {
		if command := findSubcommand(root, path...); command != nil {
			t.Fatalf("deprecated command %q is still registered", strings.Join(path, " "))
		}
	}

	for _, path := range [][]string{
		{"review", "items", "update"},
		{"review", "items-update"},
	} {
		command := findSubcommand(root, path...)
		if command == nil {
			t.Fatalf("supported command %q is not registered", strings.Join(path, " "))
		}
		if command.FlagSet.Lookup("state") != nil {
			t.Fatalf("deprecated --state flag is still registered on %q", strings.Join(path, " "))
		}
		for _, flagName := range []string{"resolved", "removed", "clear-resolved", "clear-removed"} {
			if command.FlagSet.Lookup(flagName) == nil {
				t.Fatalf("supported --%s flag is not registered on %q", flagName, strings.Join(path, " "))
			}
		}
	}
}

func TestReviewRemovedItemCommandsProvideMigrationGuidance(t *testing.T) {
	for _, args := range [][]string{
		{"review", "items", "view", "--id", "ITEM_ID"},
		{"review", "items-get", "--id", "ITEM_ID"},
	} {
		t.Run(strings.Join(args[:len(args)-2], " "), func(t *testing.T) {
			stdout, stderr := captureOutput(t, func() {
				if code := cmd.Run(args, "4.0.0"); code != cmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d", code, cmd.ExitUsage)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			const want = "was removed in 4.0.0; use `asc review items list --submission \"SUBMISSION_ID\"` instead"
			if !strings.Contains(stderr, want) {
				t.Fatalf("expected migration guidance %q, got %q", want, stderr)
			}
		})
	}
}

// TestReviewItemsAddRejectsRemovedItemTypesWithMigrationGuidance keeps the
// migration text on the item types the CLI no longer accepts, instead of
// falling back to the generic supported-value list.
func TestReviewItemsAddRejectsRemovedItemTypesWithMigrationGuidance(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	const customProductPageGuidance = "Error: --item-type appCustomProductPages is deprecated and no longer supported by App Store Connect; " +
		"pass an app custom product page version ID with --item-type appCustomProductPageVersions"
	const experimentTreatmentGuidance = "Error: --item-type appStoreVersionExperimentTreatments is deprecated and no longer supported by App Store Connect; " +
		"experiment treatments cannot be added as review submission items"
	const experimentAliasGuidance = "Error: --item-type appStoreVersionExperimentV2 was removed in 4.0.0; " +
		"use --item-type appStoreVersionExperimentsV2"

	tests := []struct {
		name     string
		command  []string
		itemType string
		wantErr  string
	}{
		{name: "custom product page", command: []string{"review", "items-add"}, itemType: "appCustomProductPages", wantErr: customProductPageGuidance},
		{name: "nested custom product page", command: []string{"review", "items", "add"}, itemType: "appCustomProductPages", wantErr: customProductPageGuidance},
		{name: "experiment treatment", command: []string{"review", "items-add"}, itemType: "appStoreVersionExperimentTreatments", wantErr: experimentTreatmentGuidance},
		{name: "nested experiment treatment", command: []string{"review", "items", "add"}, itemType: "appStoreVersionExperimentTreatments", wantErr: experimentTreatmentGuidance},
		{name: "removed experiment alias", command: []string{"review", "items-add"}, itemType: "appStoreVersionExperimentV2", wantErr: experimentAliasGuidance},
		{name: "nested removed experiment alias", command: []string{"review", "items", "add"}, itemType: "appStoreVersionExperimentV2", wantErr: experimentAliasGuidance},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			args := append(
				append([]string{}, test.command...),
				"--submission", "SUBMISSION_ID",
				"--item-type", test.itemType,
				"--item-id", "ITEM_ID",
			)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected migration guidance %q, got %q", test.wantErr, stderr)
			}
			if strings.Contains(stderr, "--item-type must be one of:") {
				t.Fatalf("expected targeted guidance instead of the generic value list, got %q", stderr)
			}
		})
	}
}

func TestReviewItemsGroupRejectsUnknownSubcommands(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "legacy get spelling",
			args:    []string{"review", "items", "get", "--id", "ITEM_ID"},
			wantErr: "Error: unexpected argument(s): get",
		},
		{
			name:    "removed view spelling",
			args:    []string{"review", "items", "view", "--id", "ITEM_ID"},
			wantErr: "was removed in 4.0.0; use `asc review items list --submission \"SUBMISSION_ID\"` instead",
		},
		{
			name:    "unknown child",
			args:    []string{"review", "items", "nope"},
			wantErr: "Error: unexpected argument(s): nope",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := captureOutput(t, func() {
				if code := cmd.Run(test.args, "4.0.0"); code != cmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d", code, cmd.ExitUsage)
				}
			})
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected %q, got %q", test.wantErr, stderr)
			}
		})
	}
}
