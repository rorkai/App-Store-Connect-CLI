package versions

import (
	"context"
	"flag"
	"fmt"
	"os"
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

// PhasedReleaseCommand returns the phased-release command group.
func PhasedReleaseCommand() *ffcli.Command {
	return &ffcli.Command{
		Name:       "phased-release",
		ShortUsage: "asc versions phased-release <subcommand> [flags]",
		ShortHelp:  "Manage phased release for app store versions.",
		LongHelp: `Manage phased release for app store versions.

Phased release gradually rolls out your app update over 7 days:
  Day 1: 1%, Day 2: 2%, Day 3: 5%, Day 4: 10%, Day 5: 20%, Day 6: 50%, Day 7: 100%

You can pause, resume, or complete the rollout at any time.

Examples:
  asc versions phased-release get --version-id "VERSION_ID"
  asc versions phased-release create --version-id "VERSION_ID"
  asc versions phased-release update --id "PHASED_ID" --state PAUSED
  asc versions phased-release delete --id "PHASED_ID" --confirm`,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			PhasedReleaseGetCommand(),
			PhasedReleaseCreateCommand(),
			PhasedReleaseUpdateCommand(),
			PhasedReleaseDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// PhasedReleaseGetCommand returns the get subcommand.
func PhasedReleaseGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("phased-release get", flag.ExitOnError)

	versionID := fs.String("version-id", "", "App Store version ID (required)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc versions phased-release get [flags]",
		ShortHelp:  "Get phased release status for an app store version.",
		LongHelp: `Get phased release status for an app store version.

Examples:
  asc versions phased-release get --version-id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			version := strings.TrimSpace(*versionID)
			if version == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("phased-release get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppStoreVersionPhasedRelease(requestCtx, version)
			if err != nil {
				return fmt.Errorf("phased-release get: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// PhasedReleaseCreateCommand returns the create subcommand.
func PhasedReleaseCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("phased-release create", flag.ExitOnError)

	versionID := fs.String("version-id", "", "App Store version ID (required)")
	state := fs.String("state", "", "Initial state: INACTIVE, ACTIVE (optional, defaults to INACTIVE)")
	output := shared.BindOutputFlags(fs)

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
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			version := strings.TrimSpace(*versionID)
			if version == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return flag.ErrHelp
			}

			var phasedState asc.PhasedReleaseState
			stateValue := strings.TrimSpace(strings.ToUpper(*state))
			if stateValue != "" {
				var ok bool
				phasedState, ok = validPhasedReleaseStates[stateValue]
				if !ok || (stateValue != "INACTIVE" && stateValue != "ACTIVE") {
					fmt.Fprintf(os.Stderr, "Error: --state must be one of: %s\n", strings.Join(validCreateStates, ", "))
					return flag.ErrHelp
				}
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("phased-release create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateAppStoreVersionPhasedRelease(requestCtx, version, phasedState)
			if err != nil {
				return fmt.Errorf("phased-release create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// PhasedReleaseUpdateCommand returns the update subcommand.
func PhasedReleaseUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("phased-release update", flag.ExitOnError)

	phasedID := fs.String("id", "", "Phased release ID (required)")
	state := fs.String("state", "", "New state: ACTIVE, PAUSED, COMPLETE (required)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc versions phased-release update [flags]",
		ShortHelp:  "Update a phased release state.",
		LongHelp: `Update a phased release state.

States:
  ACTIVE   - Resume or continue the phased rollout
  PAUSED   - Pause the rollout (users who already have the update keep it)
  COMPLETE - Release to all users immediately

Examples:
  asc versions phased-release update --id "PHASED_ID" --state PAUSED
  asc versions phased-release update --id "PHASED_ID" --state ACTIVE
  asc versions phased-release update --id "PHASED_ID" --state COMPLETE`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*phasedID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			stateValue := strings.TrimSpace(strings.ToUpper(*state))
			if stateValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --state is required")
				return flag.ErrHelp
			}

			phasedState, ok := validPhasedReleaseStates[stateValue]
			if !ok || stateValue == "INACTIVE" {
				fmt.Fprintf(os.Stderr, "Error: --state must be one of: %s\n", strings.Join(validUpdateStates, ", "))
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("phased-release update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateAppStoreVersionPhasedRelease(requestCtx, id, phasedState)
			if err != nil {
				return fmt.Errorf("phased-release update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// PhasedReleaseDeleteCommand returns the delete subcommand.
func PhasedReleaseDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("phased-release delete", flag.ExitOnError)

	phasedID := fs.String("id", "", "Phased release ID (required)")
	confirm := fs.Bool("confirm", false, "Confirm deletion (required)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc versions phased-release delete [flags]",
		ShortHelp:  "Delete a phased release.",
		LongHelp: `Delete a phased release.

This removes the phased release configuration. The app will release to all users
immediately when it goes live (no gradual rollout).

Examples:
  asc versions phased-release delete --id "PHASED_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*phasedID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("phased-release delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteAppStoreVersionPhasedRelease(requestCtx, id); err != nil {
				return fmt.Errorf("phased-release delete: %w", err)
			}

			result := &asc.AppStoreVersionPhasedReleaseDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
