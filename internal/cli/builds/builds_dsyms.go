package builds

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// dsymHTTPClient is the HTTP client used for dSYM downloads.
// No client-level timeout — the request context (from ContextWithTimeout /
// ASC_TIMEOUT) controls cancellation so the CLI timeout contract is honored.
// Tests can replace this via SetDSYMHTTPClient.
var dsymHTTPClient = &http.Client{}

// DSYMDownloadResult is the structured output for dSYM downloads.
type DSYMDownloadResult struct {
	BuildID     string             `json:"buildId"`
	Version     string             `json:"version,omitempty"`
	BuildNumber string             `json:"buildNumber,omitempty"`
	Dir         string             `json:"dir"`
	Files       []DSYMDownloadFile `json:"files"`
}

// DSYMDownloadFile describes one downloaded dSYM file.
type DSYMDownloadFile struct {
	BundleID string `json:"bundleId,omitempty"`
	FileName string `json:"fileName"`
	FilePath string `json:"filePath"`
	FileSize int64  `json:"fileSize"`
}

type dsymBundleInfo struct {
	BundleID string
	DSYMURL  *string
}

// BuildsDsymsCommand returns the builds dsyms subcommand.
func BuildsDsymsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("dsyms", flag.ExitOnError)

	buildID := fs.String("build", "", "Build ID")
	appID := fs.String("app", "", "App ID, bundle ID, or app name (or ASC_APP_ID)")
	version := fs.String("version", "", "App version string (e.g., 1.2.3)")
	buildNumber := fs.String("build-number", "", "Build number (CFBundleVersion)")
	platform := fs.String("platform", "", "Platform: IOS, MAC_OS, TV_OS, VISION_OS")
	latest := fs.Bool("latest", false, "Download dSYMs for the latest build")
	outputDir := fs.String("output-dir", ".", "Output directory for dSYM files")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "dsyms",
		ShortUsage: "asc builds dsyms [--build BUILD_ID | --app APP --latest | --app APP --build-number NUM] [flags]",
		ShortHelp:  "Download dSYM files for a build.",
		LongHelp: `Download dSYM debug symbol files for a build.

dSYM files are used for crash symbolication with tools like Crashlytics
and Sentry. Each build bundle that includes symbols will have a dSYM
download URL.

Build selection (one of):
  --build BUILD_ID                    Direct build ID
  --app APP --latest                  Most recently uploaded build
  --app APP --build-number NUM        Specific build number
  --app APP --version VER --latest    Latest build for a version

Examples:
  asc builds dsyms --build "BUILD_ID"
  asc builds dsyms --app "com.example.app" --latest
  asc builds dsyms --app "com.example.app" --latest --platform IOS
  asc builds dsyms --app "com.example.app" --version "1.2.3" --latest
  asc builds dsyms --app "com.example.app" --build-number "42"
  asc builds dsyms --app "com.example.app" --build-number "42" --output-dir "./dsyms"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedBuildID := strings.TrimSpace(*buildID)
			appInput := strings.TrimSpace(*appID)
			resolveOpts := ResolveBuildOptions{
				BuildID:     trimmedBuildID,
				AppID:       appInput,
				Version:     strings.TrimSpace(*version),
				BuildNumber: strings.TrimSpace(*buildNumber),
				Platform:    strings.TrimSpace(*platform),
				Latest:      *latest,
			}
			if err := validateResolveBuildOptions(resolveOpts); err != nil {
				return fmt.Errorf("builds dsyms: %w", err)
			}

			dirValue := strings.TrimSpace(*outputDir)
			if dirValue == "" {
				dirValue = "."
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds dsyms: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			buildResp, err := ResolveBuild(requestCtx, client, resolveOpts)
			if err != nil {
				return fmt.Errorf("builds dsyms: %w", err)
			}

			resolvedBuildID := buildResp.Data.ID
			buildVersion := buildResp.Data.Attributes.Version
			// App version (CFBundleShortVersionString) requires the preReleaseVersion
			// relationship. Use the user-supplied --version when available.
			appVersion := strings.TrimSpace(*version)

			fmt.Fprintf(os.Stderr, "Resolved build %s", resolvedBuildID)
			if buildVersion != "" {
				fmt.Fprintf(os.Stderr, " (build %s)", buildVersion)
			}
			fmt.Fprintln(os.Stderr)

			bundlesResp, err := client.GetBuildBundlesForBuild(requestCtx, resolvedBuildID)
			if err != nil {
				return fmt.Errorf("builds dsyms: %w", err)
			}

			bundles := make([]dsymBundleInfo, 0, len(bundlesResp.Data))
			for _, b := range bundlesResp.Data {
				bundleID := ""
				if b.Attributes.BundleID != nil {
					bundleID = *b.Attributes.BundleID
				}
				bundles = append(bundles, dsymBundleInfo{
					BundleID: bundleID,
					DSYMURL:  b.Attributes.DSYMURL,
				})
			}

			downloadable := filterBundlesWithDSYM(bundles)
			if len(downloadable) == 0 {
				fmt.Fprintln(os.Stderr, "No dSYM files available for this build")
				result := DSYMDownloadResult{
					BuildID:     resolvedBuildID,
					Version:     appVersion,
					BuildNumber: buildVersion,
					Dir:         dirValue,
					Files:       []DSYMDownloadFile{},
				}
				return shared.PrintOutputWithRenderers(
					result,
					*output.Output,
					*output.Pretty,
					func() error { return printDSYMResultTable(result) },
					func() error { return printDSYMResultMarkdown(result) },
				)
			}

			if err := os.MkdirAll(dirValue, 0o755); err != nil {
				return fmt.Errorf("builds dsyms: failed to create output directory: %w", err)
			}

			files := make([]DSYMDownloadFile, 0, len(downloadable))
			for i, bundle := range downloadable {
				fileName := dsymFileName(bundle.BundleID, appVersion, buildVersion, resolvedBuildID, i)
				filePath := filepath.Join(dirValue, fileName)

				fmt.Fprintf(os.Stderr, "Downloading dSYM for %s...\n", displayBundleID(bundle.BundleID, i))

				size, err := downloadDSYM(requestCtx, *bundle.DSYMURL, filePath)
				if err != nil {
					return fmt.Errorf("builds dsyms: failed to download %s: %w", fileName, err)
				}

				fmt.Fprintf(os.Stderr, "  Saved %s (%d bytes)\n", filePath, size)

				files = append(files, DSYMDownloadFile{
					BundleID: bundle.BundleID,
					FileName: fileName,
					FilePath: filePath,
					FileSize: size,
				})
			}

			result := DSYMDownloadResult{
				BuildID:     resolvedBuildID,
				Version:     appVersion,
				BuildNumber: buildVersion,
				Dir:         dirValue,
				Files:       files,
			}

			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return printDSYMResultTable(result) },
				func() error { return printDSYMResultMarkdown(result) },
			)
		},
	}
}

func filterBundlesWithDSYM(bundles []dsymBundleInfo) []dsymBundleInfo {
	result := make([]dsymBundleInfo, 0, len(bundles))
	for _, b := range bundles {
		if b.DSYMURL != nil && strings.TrimSpace(*b.DSYMURL) != "" {
			result = append(result, b)
		}
	}
	return result
}

// dsymFileName builds a descriptive file name. Prefers bundleId-version-buildNumber
// format (matching fastlane convention). Falls back to buildID-based names.
func dsymFileName(bundleID, appVersion, buildVersion, buildID string, index int) string {
	if bundleID != "" && appVersion != "" && buildVersion != "" {
		return fmt.Sprintf("%s-%s-%s.dSYM.zip", bundleID, appVersion, buildVersion)
	}
	if bundleID != "" && buildVersion != "" {
		return fmt.Sprintf("%s-%s.dSYM.zip", bundleID, buildVersion)
	}
	if bundleID != "" {
		return fmt.Sprintf("%s-%s.dSYM.zip", bundleID, buildID)
	}
	return fmt.Sprintf("%s_%d.dSYM.zip", buildID, index)
}

func displayBundleID(bundleID string, index int) string {
	if bundleID != "" {
		return bundleID
	}
	return fmt.Sprintf("bundle %d", index)
}

func downloadDSYM(ctx context.Context, rawURL, destPath string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, fmt.Errorf("download failed: %w", err)
	}

	resp, err := dsymHTTPClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}

	return shared.WriteStreamToFile(destPath, resp.Body)
}

// SetDSYMHTTPClient replaces the HTTP client for tests.
func SetDSYMHTTPClient(c *http.Client) func() {
	prev := dsymHTTPClient
	dsymHTTPClient = c
	return func() { dsymHTTPClient = prev }
}

func printDSYMResultTable(result DSYMDownloadResult) error {
	fmt.Printf("Build ID: %s\n", result.BuildID)
	if result.Version != "" {
		fmt.Printf("Version: %s (%s)\n", result.Version, result.BuildNumber)
	}
	fmt.Printf("Dir: %s\n", result.Dir)
	fmt.Printf("Files: %d\n\n", len(result.Files))

	if len(result.Files) == 0 {
		asc.RenderTable([]string{"status"}, [][]string{{"no dSYM files available"}})
		return nil
	}

	rows := make([][]string, 0, len(result.Files))
	for _, f := range result.Files {
		rows = append(rows, []string{f.BundleID, f.FileName, fmt.Sprintf("%d", f.FileSize)})
	}
	asc.RenderTable([]string{"bundleId", "fileName", "fileSize"}, rows)
	return nil
}

func printDSYMResultMarkdown(result DSYMDownloadResult) error {
	fmt.Printf("**Build ID:** %s\n\n", result.BuildID)
	if result.Version != "" {
		fmt.Printf("**Version:** %s (%s)\n\n", result.Version, result.BuildNumber)
	}
	fmt.Printf("**Dir:** %s\n\n", result.Dir)
	fmt.Printf("**Files:** %d\n\n", len(result.Files))

	if len(result.Files) == 0 {
		asc.RenderMarkdown([]string{"status"}, [][]string{{"no dSYM files available"}})
		return nil
	}

	rows := make([][]string, 0, len(result.Files))
	for _, f := range result.Files {
		rows = append(rows, []string{f.BundleID, f.FileName, fmt.Sprintf("%d", f.FileSize)})
	}
	asc.RenderMarkdown([]string{"bundleId", "fileName", "fileSize"}, rows)
	return nil
}
