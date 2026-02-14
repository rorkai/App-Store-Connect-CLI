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

// VersionsReleaseCommand releases a version in pending developer release.
func VersionsReleaseCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions release", flag.ExitOnError)

	versionID := fs.String("version-id", "", "App Store version ID (required)")
	confirm := fs.Bool("confirm", false, "Confirm release request (required)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "release",
		ShortUsage: "asc versions release [flags]",
		ShortHelp:  "Release an approved version pending developer release.",
		LongHelp: `Release an approved version in the Pending Developer Release state.

Examples:
  asc versions release --version-id "VERSION_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			version := strings.TrimSpace(*versionID)
			if version == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required to release a version")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("versions release: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateAppStoreVersionReleaseRequest(requestCtx, version)
			if err != nil {
				return fmt.Errorf("versions release: %w", err)
			}

			result := &asc.AppStoreVersionReleaseRequestResult{
				ReleaseRequestID: resp.Data.ID,
				VersionID:        version,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
