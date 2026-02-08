package xcodecloud

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// XcodeCloudArtifactsCommand returns the xcode-cloud artifacts command with subcommands.
func XcodeCloudArtifactsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("artifacts", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "artifacts",
		ShortUsage: "asc xcode-cloud artifacts <subcommand> [flags]",
		ShortHelp:  "Manage Xcode Cloud build artifacts.",
		LongHelp: `Manage Xcode Cloud build artifacts.

Examples:
  asc xcode-cloud artifacts list --action-id "ACTION_ID"
  asc xcode-cloud artifacts get --id "ARTIFACT_ID"
  asc xcode-cloud artifacts download --id "ARTIFACT_ID" --path ./artifact.zip`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			XcodeCloudArtifactsListCommand(),
			XcodeCloudArtifactsGetCommand(),
			XcodeCloudArtifactsDownloadCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// XcodeCloudArtifactsListCommand returns the xcode-cloud artifacts list subcommand.
func XcodeCloudArtifactsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	actionID := fs.String("action-id", "", "Build action ID to list artifacts for")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc xcode-cloud artifacts list [flags]",
		ShortHelp:  "List artifacts for a build action.",
		LongHelp: `List artifacts for a build action.

Examples:
  asc xcode-cloud artifacts list --action-id "ACTION_ID"
  asc xcode-cloud artifacts list --action-id "ACTION_ID" --output table
  asc xcode-cloud artifacts list --action-id "ACTION_ID" --limit 50
  asc xcode-cloud artifacts list --action-id "ACTION_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("xcode-cloud artifacts list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("xcode-cloud artifacts list: %w", err)
			}

			resolvedActionID := strings.TrimSpace(*actionID)
			if resolvedActionID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --action-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("xcode-cloud artifacts list: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
			defer cancel()

			opts := []asc.CiArtifactsOption{
				asc.WithCiArtifactsLimit(*limit),
				asc.WithCiArtifactsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithCiArtifactsLimit(200))
				firstPage, err := client.GetCiBuildActionArtifacts(requestCtx, resolvedActionID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("xcode-cloud artifacts list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetCiBuildActionArtifacts(ctx, resolvedActionID, asc.WithCiArtifactsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("xcode-cloud artifacts list: %w", err)
				}

				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetCiBuildActionArtifacts(requestCtx, resolvedActionID, opts...)
			if err != nil {
				return fmt.Errorf("xcode-cloud artifacts list: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// XcodeCloudArtifactsGetCommand returns the xcode-cloud artifacts get subcommand.
func XcodeCloudArtifactsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	id := fs.String("id", "", "Artifact ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc xcode-cloud artifacts get --id \"ARTIFACT_ID\"",
		ShortHelp:  "Get details for a build artifact.",
		LongHelp: `Get details for a build artifact.

Examples:
  asc xcode-cloud artifacts get --id "ARTIFACT_ID"
  asc xcode-cloud artifacts get --id "ARTIFACT_ID" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("xcode-cloud artifacts get: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
			defer cancel()

			resp, err := client.GetCiArtifact(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("xcode-cloud artifacts get: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// XcodeCloudArtifactsDownloadCommand returns the xcode-cloud artifacts download subcommand.
func XcodeCloudArtifactsDownloadCommand() *ffcli.Command {
	fs := flag.NewFlagSet("download", flag.ExitOnError)

	id := fs.String("id", "", "Artifact ID")
	path := fs.String("path", "", "Output file path for the artifact")
	overwrite := fs.Bool("overwrite", false, "Overwrite existing file")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "download",
		ShortUsage: "asc xcode-cloud artifacts download --id \"ARTIFACT_ID\" --path ./artifact.zip",
		ShortHelp:  "Download a build artifact.",
		LongHelp: `Download a build artifact.

Examples:
  asc xcode-cloud artifacts download --id "ARTIFACT_ID" --path ./artifact.zip
  asc xcode-cloud artifacts download --id "ARTIFACT_ID" --path ./artifact.zip --overwrite`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			pathValue := strings.TrimSpace(*path)
			if pathValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --path is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("xcode-cloud artifacts download: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
			defer cancel()

			artifactResp, err := client.GetCiArtifact(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("xcode-cloud artifacts download: failed to fetch artifact: %w", err)
			}

			downloadURL := strings.TrimSpace(artifactResp.Data.Attributes.DownloadURL)
			if downloadURL == "" {
				return fmt.Errorf("xcode-cloud artifacts download: artifact has no download URL")
			}

			download, err := client.DownloadCiArtifact(requestCtx, downloadURL)
			if err != nil {
				return fmt.Errorf("xcode-cloud artifacts download: %w", err)
			}
			defer download.Body.Close()

			bytesWritten, err := writeArtifactFile(pathValue, download.Body, *overwrite)
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

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}

func writeArtifactFile(path string, reader io.Reader, overwrite bool) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 0, err
	}

	if !overwrite {
		file, err := shared.OpenNewFileNoFollow(path, 0o600)
		if err != nil {
			if errors.Is(err, os.ErrExist) {
				return 0, fmt.Errorf("output file already exists: %w", err)
			}
			return 0, err
		}
		defer file.Close()

		n, err := io.Copy(file, reader)
		if err != nil {
			return 0, err
		}
		if err := file.Sync(); err != nil {
			return 0, err
		}
		return n, nil
	}

	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return 0, fmt.Errorf("refusing to overwrite symlink %q", path)
		}
		if info.IsDir() {
			return 0, fmt.Errorf("output path %q is a directory", path)
		}
		if err := os.Remove(path); err != nil {
			return 0, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return 0, err
	}

	tempFile, err := os.CreateTemp(filepath.Dir(path), ".asc-artifact-*")
	if err != nil {
		return 0, err
	}
	defer tempFile.Close()

	tempPath := tempFile.Name()
	success := false
	defer func() {
		if !success {
			_ = os.Remove(tempPath)
		}
	}()

	n, err := io.Copy(tempFile, reader)
	if err != nil {
		return 0, err
	}
	if err := tempFile.Sync(); err != nil {
		return 0, err
	}
	if err := tempFile.Close(); err != nil {
		return 0, err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return 0, err
	}

	success = true
	return n, nil
}
