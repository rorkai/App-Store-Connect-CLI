package xcodecloud

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newXcodeCloudArtifactsCommand returns the xcode-cloud artifacts command with subcommands.
func newXcodeCloudArtifactsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("artifacts", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "artifacts",
		ShortUsage: "asc xcode-cloud artifacts <subcommand> [flags]",
		ShortHelp:  "Manage Xcode Cloud build artifacts.",
		LongHelp: `Manage Xcode Cloud build artifacts.

Examples:
  asc xcode-cloud artifacts list --action-id "ACTION_ID"
  asc xcode-cloud artifacts list --run-id "BUILD_RUN_ID"
  asc xcode-cloud artifacts view --id "ARTIFACT_ID"
  asc xcode-cloud artifacts download --id "ARTIFACT_ID" --path ./artifact.zip`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newXcodeCloudArtifactsListCommand(d),
			newXcodeCloudArtifactsGetCommand(d),
			newXcodeCloudArtifactsDownloadCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newXcodeCloudArtifactsListCommand returns the xcode-cloud artifacts list subcommand.
func newXcodeCloudArtifactsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("list", flag.ExitOnError)

	actionID := shared.BindResourceIDFlag(fs, "action-id", "ciBuildActions", "Build action ID to list artifacts for")
	runID := shared.BindResourceIDFlag(fs, "run-id", "ciBuildRuns", "Build run ID to aggregate artifacts across all actions")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc xcode-cloud artifacts list [flags]",
		ShortHelp:  "List artifacts for a build action or build run.",
		LongHelp: `List artifacts for a build action or across all actions in a build run.

Examples:
  asc xcode-cloud artifacts list --action-id "ACTION_ID"
  asc xcode-cloud artifacts list --run-id "BUILD_RUN_ID"
  asc xcode-cloud artifacts list --action-id "ACTION_ID" --output table
  asc xcode-cloud artifacts list --action-id "ACTION_ID" --limit 50
  asc xcode-cloud artifacts list --action-id "ACTION_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "xcode-cloud artifacts list", 200); err != nil {
				return err
			}
			return runXcodeCloudActionResourceList(
				ctx,
				d,
				*actionID,
				*runID,
				*limit,
				*next,
				*paginate,
				*output.Output,
				*output.Pretty,
				"xcode-cloud artifacts list",
				func(ctx context.Context, client *asc.Client, resolvedActionID string, limit int, next string) (asc.PaginatedResponse, error) {
					opts := []asc.CiArtifactsOption{
						asc.Limit[asc.CiArtifactsOption](limit),
						asc.NextURL[asc.CiArtifactsOption](next),
					}
					return client.GetCiBuildActionArtifacts(ctx, resolvedActionID, opts...)
				},
				aggregateXcodeCloudArtifactsFromRun,
			)
		},
	}
}

// newXcodeCloudArtifactsGetCommand returns the xcode-cloud artifacts view subcommand.
func newXcodeCloudArtifactsGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc xcode-cloud artifacts view --id \"ARTIFACT_ID\"",
		ShortHelp:  "View details for a build artifact.",
		LongHelp: `View details for a build artifact.

Examples:
  asc xcode-cloud artifacts view --id "ARTIFACT_ID"
  asc xcode-cloud artifacts view --id "ARTIFACT_ID" --output table`,
		IDFlag:           "id",
		IDUsage:          "Artifact ID",
		IDType:           "ciArtifacts",
		FetchErrorPrefix: "xcode-cloud artifacts view",
		Deps:             d,
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(d, ctx, 0)
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetCiArtifact(ctx, id)
		},
	})
}

// newXcodeCloudArtifactsDownloadCommand returns the xcode-cloud artifacts download subcommand.
func newXcodeCloudArtifactsDownloadCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("download", flag.ExitOnError)

	id := shared.BindResourceIDFlag(fs, "id", "ciArtifacts", "Artifact ID")
	path := fs.String("path", "", "Output file path for the artifact")
	overwrite := fs.Bool("overwrite", false, "Overwrite existing file")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "download",
		ShortUsage: "asc xcode-cloud artifacts download --id \"ARTIFACT_ID\" --path ./artifact.zip",
		ShortHelp:  "Download a build artifact.",
		LongHelp: `Download a build artifact.

Examples:
  asc xcode-cloud artifacts download --id "ARTIFACT_ID" --path ./artifact.zip
  asc xcode-cloud artifacts download --id "ARTIFACT_ID" --path ./artifact.zip --overwrite`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				return d.RequiredFlag("--id")
			}
			pathValue := strings.TrimSpace(*path)
			if pathValue == "" {
				return d.RequiredFlag("--path")
			}
			if err := validateArtifactDestination(pathValue, *overwrite); err != nil {
				return fmt.Errorf("xcode-cloud artifacts download: %w", err)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("xcode-cloud artifacts download: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(d, ctx, 0)
			defer cancel()

			artifactResp, err := client.GetCiArtifact(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("xcode-cloud artifacts download: failed to fetch artifact: %w", err)
			}

			downloadURL := strings.TrimSpace(artifactResp.Data.Attributes.DownloadURL)
			if downloadURL == "" {
				return fmt.Errorf("xcode-cloud artifacts download: artifact has no download URL")
			}

			bytesWritten, err := asc.RetryDownload(requestCtx, downloadURL, func() (int64, error) {
				download, err := client.DownloadCiArtifact(requestCtx, downloadURL)
				if err != nil {
					return 0, err
				}
				defer download.Body.Close()
				return writeArtifactFile(pathValue, download.Body, *overwrite)
			}, asc.ResolveRetryOptions())
			if err != nil {
				return fmt.Errorf("xcode-cloud artifacts download: %w", err)
			}

			result := &asc.CiArtifactDownloadResult{
				ID:           artifactResp.Data.ID,
				FileName:     artifactResp.Data.Attributes.FileName,
				FileType:     artifactResp.Data.Attributes.FileType,
				FileSize:     artifactResp.Data.Attributes.FileSize,
				OutputPath:   pathValue,
				BytesWritten: bytesWritten,
			}

			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func validateArtifactDestination(path string, overwrite bool) error {
	if len(path) > 0 && os.IsPathSeparator(path[len(path)-1]) {
		return fmt.Errorf("output path %q must be a file", path)
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !overwrite {
		return fmt.Errorf("output file already exists: %w", os.ErrExist)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to overwrite symlink %q", path)
	}
	if info.IsDir() {
		return fmt.Errorf("output path %q is a directory", path)
	}
	return nil
}

func writeArtifactFile(path string, reader io.Reader, overwrite bool) (int64, error) {
	return shared.SafeWriteFileNoSymlink(
		path,
		0o600,
		overwrite,
		".asc-artifact-*",
		".asc-artifact-backup-*",
		func(file *os.File) (int64, error) {
			return io.Copy(file, reader)
		},
	)
}
