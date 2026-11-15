package versions

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var validPhasedReleaseStates = map[string]asc.PhasedReleaseState{
	"INACTIVE": asc.PhasedReleaseStateInactive,
	"ACTIVE":   asc.PhasedReleaseStateActive,
	"PAUSED":   asc.PhasedReleaseStatePaused,
	"COMPLETE": asc.PhasedReleaseStateComplete,
}

// validCreateStates are states allowed when creating a phased release
var validCreateStates = []string{"INACTIVE", "ACTIVE"}

// validUpdateStates are states allowed when updating a phased release
var validUpdateStates = []string{"ACTIVE", "PAUSED", "COMPLETE"}

// newPhasedReleaseCommand returns the phased-release command group.
func newPhasedReleaseCommand(d *shared.Deps) *ffcli.Command {
	return &ffcli.Command{
		Name:       "phased-release",
		ShortUsage: "asc versions phased-release <subcommand> [flags]",
		ShortHelp:  "Manage phased release for app store versions.",
		LongHelp: `Manage phased release for app store versions.

Phased release gradually rolls out your app update over 7 days:
  Day 1: 1%, Day 2: 2%, Day 3: 5%, Day 4: 10%, Day 5: 20%, Day 6: 50%, Day 7: 100%

You can pause, resume, or complete the rollout at any time.

Examples:
  asc versions phased-release view --version-id "VERSION_ID"
  asc versions phased-release create --version-id "VERSION_ID"
  asc versions phased-release update --id "PHASED_ID" --state PAUSED
  asc versions phased-release delete --id "PHASED_ID" --confirm
  asc versions phased-release guard --version-id "VERSION_ID" --confirm -- ./scripts/health-check.sh`,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newPhasedReleaseGetCommand(d),
			newPhasedReleaseCreateCommand(d),
			newPhasedReleaseUpdateCommand(d),
			newPhasedReleaseDeleteCommand(d),
			PhasedReleaseGuardCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newPhasedReleaseGetCommand returns the get subcommand.
func newPhasedReleaseGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:        d,
		FlagSetName: "phased-release view",
		Name:        "view",
		ShortUsage:  "asc versions phased-release view [flags]",
		ShortHelp:   "View phased release status for an app store version.",
		LongHelp: `View phased release status for an app store version.

Examples:
  asc versions phased-release view --version-id "VERSION_ID"`,
		IDFlag:             "version-id",
		IDUsage:            "App Store version ID (required)",
		IDType:             "appStoreVersions",
		RequiredDiagnostic: true,
		ErrorPrefix:        "phased-release view",
		FetchErrorPrefix:   "phased-release view",
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetAppStoreVersionPhasedRelease(ctx, id)
		},
	})
}

// newPhasedReleaseCreateCommand returns the create subcommand.
func newPhasedReleaseCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("phased-release create", flag.ExitOnError)

	versionID := shared.BindResourceIDFlag(fs, "version-id", "appStoreVersions", "App Store version ID (required)")
	state := fs.String("state", "", "Initial state: INACTIVE, ACTIVE (optional, defaults to INACTIVE)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc versions phased-release create [flags]",
		ShortHelp:  "Create a phased release for an app store version.",
		LongHelp: `Create a phased release for an app store version.

The phased release will start when the app is released to the App Store.
Use --state ACTIVE to start immediately, or leave empty to start as INACTIVE.

Examples:
  asc versions phased-release create --version-id "VERSION_ID"
  asc versions phased-release create --version-id "VERSION_ID" --state ACTIVE`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			version := strings.TrimSpace(*versionID)
			if version == "" {
				return d.RequiredFlag("--version-id")
			}

			var phasedState asc.PhasedReleaseState
			stateValue := strings.TrimSpace(strings.ToUpper(*state))
			if stateValue != "" {
				var ok bool
				phasedState, ok = validPhasedReleaseStates[stateValue]
				if !ok || (stateValue != "INACTIVE" && stateValue != "ACTIVE") {
					fmt.Fprintf(d.IO.ErrOut, "Error: --state must be one of: %s\n", strings.Join(validCreateStates, ", "))
					return flag.ErrHelp
				}
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("phased-release create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.CreateAppStoreVersionPhasedRelease(requestCtx, version, phasedState)
			if err != nil {
				return fmt.Errorf("phased-release create: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newPhasedReleaseUpdateCommand returns the update subcommand.
func newPhasedReleaseUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("phased-release update", flag.ExitOnError)

	phasedID := shared.BindResourceIDFlag(fs, "id", "appStoreVersionPhasedReleases", "Phased release ID (required)")
	state := fs.String("state", "", "New state: ACTIVE, PAUSED, COMPLETE (required)")
	confirm := fs.Bool("confirm", false, "Confirm COMPLETE, which releases the update to all users immediately")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc versions phased-release update [flags]",
		ShortHelp:  "Update a phased release state.",
		LongHelp: `Update a phased release state.

States:
  ACTIVE   - Resume or continue the phased rollout
  PAUSED   - Pause the rollout (users who already have the update keep it)
  COMPLETE - Release to all users immediately (requires --confirm)

Examples:
  asc versions phased-release update --id "PHASED_ID" --state PAUSED
  asc versions phased-release update --id "PHASED_ID" --state ACTIVE
  asc versions phased-release update --id "PHASED_ID" --state COMPLETE --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*phasedID)
			if id == "" {
				return d.RequiredFlag("--id")
			}

			stateValue := strings.TrimSpace(strings.ToUpper(*state))
			if stateValue == "" {
				return d.RequiredFlag("--state")
			}

			phasedState, ok := validPhasedReleaseStates[stateValue]
			if !ok || stateValue == "INACTIVE" {
				fmt.Fprintf(d.IO.ErrOut, "Error: --state must be one of: %s\n", strings.Join(validUpdateStates, ", "))
				return flag.ErrHelp
			}
			if stateValue == "COMPLETE" && !*confirm {
				fmt.Fprintln(d.IO.ErrOut, "Error: --confirm is required to set phased release state COMPLETE")
				return shared.MissingRequiredUsageError("--confirm")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("phased-release update: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.UpdateAppStoreVersionPhasedRelease(requestCtx, id, phasedState)
			if err != nil {
				return fmt.Errorf("phased-release update: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newPhasedReleaseDeleteCommand returns the delete subcommand.
func newPhasedReleaseDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Deps:        d,
		FlagSetName: "phased-release delete",
		Name:        "delete",
		ShortUsage:  "asc versions phased-release delete [flags]",
		ShortHelp:   "Delete a phased release.",
		LongHelp: `Delete a phased release.

This removes the phased release configuration. The app will release to all users
immediately when it goes live (no gradual rollout).

Examples:
  asc versions phased-release delete --id "PHASED_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Phased release ID (required)",
		IDType:             "appStoreVersionPhasedReleases",
		RequiredDiagnostic: true,
		ConfirmUsage:       "Confirm deletion (required)",
		ErrorPrefix:        "phased-release delete",
		DeleteErrorPrefix:  "phased-release delete",
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteAppStoreVersionPhasedRelease(ctx, id)
		},
		Result: func(id string) any {
			return &asc.AppStoreVersionPhasedReleaseDeleteResult{ID: id, Deleted: true}
		},
	})
}
