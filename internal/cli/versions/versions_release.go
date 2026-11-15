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

// newVersionsReleaseCommand releases a version in pending developer release.
func newVersionsReleaseCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("versions release", flag.ExitOnError)

	versionID := shared.BindResourceIDFlag(fs, "version-id", "appStoreVersions", "App Store version ID (required)")
	confirm := fs.Bool("confirm", false, "Confirm release request (required)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "release",
		ShortUsage: "asc versions release [flags]",
		ShortHelp:  "Release an approved version pending developer release.",
		LongHelp: `Release an approved version in the Pending Developer Release state.

The request is sent once. If Apple answers with a server error or the request
times out, the version is read back: when it has left Pending Developer Release,
the command succeeds with reconciled set to true and a note on stderr.

Examples:
  asc versions release --version-id "VERSION_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			version := strings.TrimSpace(*versionID)
			if version == "" {
				return d.RequiredFlag("--version-id")
			}
			if err := d.RequireConfirm(*confirm); err != nil {
				return err
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("versions release: %w", err)
			}

			result, _, err := shared.RunReconciledMutationNoReplayWith(
				ctx,
				d,
				func(requestCtx context.Context) (*asc.AppStoreVersionReleaseRequestResult, error) {
					resp, err := client.CreateAppStoreVersionReleaseRequest(requestCtx, version)
					if err != nil {
						return nil, err
					}
					return &asc.AppStoreVersionReleaseRequestResult{
						ReleaseRequestID: resp.Data.ID,
						VersionID:        version,
					}, nil
				},
				func(readbackCtx context.Context) (*asc.AppStoreVersionReleaseRequestResult, bool, error) {
					current, err := shared.RetryReadWithFreshTimeoutWith(readbackCtx, d, func(requestCtx context.Context) (*asc.AppStoreVersionResponse, error) {
						return client.GetAppStoreVersion(requestCtx, version)
					})
					if err != nil {
						return nil, false, err
					}
					if !versionReleaseAccepted(current.Data.Attributes) {
						return nil, false, nil
					}
					return &asc.AppStoreVersionReleaseRequestResult{
						VersionID:    version,
						Reconciled:   true,
						VersionState: shared.ResolveAppStoreVersionState(current.Data.Attributes),
					}, true, nil
				},
			)
			if err != nil {
				return fmt.Errorf("versions release: %w", err)
			}
			if result.Reconciled {
				fmt.Fprintf(d.IO.ErrOut, "App Store Connect answered with an error, but version %s is now %s, so the release was accepted.\n", version, result.VersionState)
			}

			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// versionReleaseAccepted reports whether a version has moved past Pending
// Developer Release. It decides from the resolved state alone, and a pre-order
// listing state does not count because it can persist while release is pending.
func versionReleaseAccepted(attrs asc.AppStoreVersionAttributes) bool {
	switch strings.ToUpper(strings.TrimSpace(shared.ResolveAppStoreVersionState(attrs))) {
	case "PROCESSING_FOR_DISTRIBUTION", "READY_FOR_DISTRIBUTION",
		"PROCESSING_FOR_APP_STORE", "READY_FOR_SALE":
		return true
	default:
		return false
	}
}
