package builds

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/urlsanitize"
)

// dsymHTTPClient is the HTTP client used for dSYM downloads.
// No client-level timeout — the request context (from ContextWithDownloadTimeout /
// ASC_UPLOAD_TIMEOUT) controls cancellation so the CLI timeout contract is honored.
// Tests can replace this via SetDSYMHTTPClient.
var dsymHTTPClient = &http.Client{}

type dsymBundleInfo struct {
	BundleID string
	DSYMURL  *string
}

// BuildsDsymsCommand returns the builds dsyms subcommand.
func BuildsDsymsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("dsyms", flag.ExitOnError)

	buildID := shared.BindResourceIDFlag(fs, "build-id", "builds", "Build ID")
	appID := fs.String("app", "", "App ID, bundle ID, or app name (or ASC_APP_ID)")
	version := fs.String("version", "", "Marketing version, latest, or live")
	buildNumber := fs.String("build-number", "", "Build number (CFBundleVersion)")
	platform := fs.String("platform", "", "Platform: IOS, MAC_OS, TV_OS, VISION_OS")
	latest := fs.Bool("latest", false, "Download dSYMs for the latest build")
	excludeExpired := fs.Bool("exclude-expired", false, "Exclude expired builds")
	notExpired := fs.Bool("not-expired", false, "Alias for --exclude-expired")
	all := fs.Bool("all", false, "Download dSYMs for every matching build")
	minVersion := fs.String("min-version", "", "Include builds whose marketing version is at least this version")
	afterUploaded := fs.String("after-uploaded-date", "", "Include builds uploaded after this RFC3339 timestamp")
	wait := fs.Bool("wait", false, "Poll until a dSYM URL is available")
	timeout := fs.Duration("timeout", dsymDefaultWaitTimeout, "Maximum time to wait for dSYM URLs")
	pollInterval := fs.Duration("poll-interval", dsymDefaultPollInterval, "Polling interval for --wait")
	outputDir := fs.String("output-dir", ".", "Output directory for dSYM files")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "dsyms",
		ShortUsage: "asc builds dsyms [--build-id BUILD_ID | --app APP (--latest | --version latest|live|VER | --build-number NUM --platform PLATFORM | --all | --min-version VER | --after-uploaded-date TIME)] [--wait] [flags]",
		ShortHelp:  "Download dSYM files for one or more builds.",
		LongHelp: `Download dSYM debug symbol files for one or more builds.

dSYM files are used for crash symbolication with tools like Crashlytics
and Sentry. Each build bundle that includes symbols will have a dSYM
download URL.

Build selection (one of):
  --build-id BUILD_ID
  --app APP --latest [--version VER] [--platform PLATFORM]
  --app APP --version latest|live|VER [--platform PLATFORM]
  --app APP --build-number NUM --platform PLATFORM [--version VER]
  --app APP [--version VER] [--platform PLATFORM] [--all | --min-version VER | --after-uploaded-date TIME]

--version latest is the same selector as --latest. --version live uses the
newest READY_FOR_SALE or PREORDER_READY_FOR_SALE App Store version. Pass
--platform when more than one platform is live. --min-version and
--after-uploaded-date select every matching build. Single-build file names stay
bundleId-version-buildNumber.dSYM.zip. Bulk selectors append the build ID
to prevent collisions across platforms. An existing file is skipped only when
its size and SHA-256 match the remote artifact. --wait polls build bundles until a dSYM URL appears; a timeout exits
1 with diagnostic dsym_not_ready and a partial receipt.

Examples:
  asc builds dsyms --build-id "BUILD_ID"
  asc builds dsyms --app "com.example.app" --latest
  asc builds dsyms --app "com.example.app" --latest --platform IOS
  asc builds dsyms --app "com.example.app" --latest --version "1.2.3"
  asc builds dsyms --app "com.example.app" --version "1.2.3"
  asc builds dsyms --app "com.example.app" --version live
  asc builds dsyms --app "com.example.app" --min-version "1.2.0" --output-dir "./dsyms"
  asc builds dsyms --app "com.example.app" --version live --wait --timeout 15m
  asc builds dsyms --app "com.example.app" --build-number "42" --platform IOS
  asc builds dsyms --app "com.example.app" --build-number "42" --platform IOS --version "1.2.3" --output-dir "./dsyms"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			timeoutSet := false
			pollSet := false
			fs.Visit(func(f *flag.Flag) {
				switch f.Name {
				case "timeout":
					timeoutSet = true
				case "poll-interval":
					pollSet = true
				}
			})
			selection, err := parseDSYMSelection(dsymFlagInput{
				BuildID:        *buildID,
				AppID:          *appID,
				Version:        *version,
				BuildNumber:    *buildNumber,
				Platform:       *platform,
				Latest:         *latest,
				ExcludeExpired: *excludeExpired || *notExpired,
				All:            *all,
				MinVersion:     *minVersion,
				AfterRaw:       *afterUploaded,
				Wait:           *wait,
				Timeout:        *timeout,
				PollInterval:   *pollInterval,
				TimeoutSet:     timeoutSet,
				PollSet:        pollSet,
			})
			if err != nil {
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

			return downloadDSYMSelection(ctx, client, selection, dirValue, *output.Output, *output.Pretty)
		},
	}
}

type dsymNotReadyError struct {
	BuildID string
}

func (e *dsymNotReadyError) Error() string {
	if e == nil || strings.TrimSpace(e.BuildID) == "" {
		return "builds dsyms: dsym_not_ready: timed out waiting for a dSYM URL"
	}
	return fmt.Sprintf("builds dsyms: dsym_not_ready: timed out waiting for a dSYM URL for build %s", e.BuildID)
}

func downloadDSYMSelection(ctx context.Context, client *asc.Client, selection dsymSelection, dirValue, outputFormat string, pretty bool) error {
	runCtx := ctx
	if selection.Wait {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, selection.Timeout)
		defer cancel()
	}

	targets, err := resolveDSYMTargets(runCtx, client, selection)
	if err != nil {
		return fmt.Errorf("builds dsyms: %w", err)
	}

	files := make([]asc.DSYMDownloadFile, 0)
	var downloadErr error
	status := ""
	for _, target := range targets {
		fmt.Fprintf(os.Stderr, "Resolved build %s", target.ID)
		if target.BuildNumber != "" {
			fmt.Fprintf(os.Stderr, " (build %s)", target.BuildNumber)
		}
		fmt.Fprintln(os.Stderr)

		bundles, err := fetchDSYMBundles(runCtx, client, target.ID, selection)
		if err != nil {
			downloadErr = err
			if errors.As(err, new(*dsymNotReadyError)) || strings.Contains(err.Error(), dsymNotReadyStatus) {
				status = dsymNotReadyStatus
			}
			break
		}
		downloadable := filterBundlesWithDSYM(bundles)
		if len(downloadable) == 0 {
			if len(targets) == 1 {
				fmt.Fprintln(os.Stderr, "No dSYM files available for this build")
			} else {
				fmt.Fprintf(os.Stderr, "No dSYM files available for build %s\n", target.ID)
			}
			continue
		}
		if err := os.MkdirAll(dirValue, 0o755); err != nil {
			return fmt.Errorf("builds dsyms: failed to create output directory: %w", err)
		}
		saved, err := saveDSYMBundles(ctx, downloadable, target, dirValue, selection.Multi)
		files = append(files, saved...)
		if err != nil {
			downloadErr = err
			break
		}
	}

	result := dsymResult(targets, files, dirValue, status)
	printErr := shared.PrintOutputWithRenderers(
		result,
		outputFormat,
		pretty,
		func() error { return printDSYMResultTable(result) },
		func() error { return printDSYMResultMarkdown(result) },
	)
	if downloadErr != nil {
		if printErr != nil {
			return errors.Join(downloadErr, printErr)
		}
		return downloadErr
	}
	return printErr
}

func dsymResult(targets []dsymTarget, files []asc.DSYMDownloadFile, dirValue, status string) asc.DSYMDownloadResult {
	result := asc.DSYMDownloadResult{
		Dir:    dirValue,
		Status: status,
		Files:  files,
	}
	if result.Files == nil {
		result.Files = []asc.DSYMDownloadFile{}
	}
	if len(targets) == 1 {
		result.BuildID = targets[0].ID
		result.Version = targets[0].AppVersion
		result.BuildNumber = targets[0].BuildNumber
	}
	return result
}

func fetchDSYMBundles(ctx context.Context, client *asc.Client, buildID string, selection dsymSelection) ([]dsymBundleInfo, error) {
	load := func(ctx context.Context) ([]dsymBundleInfo, bool, error) {
		requestCtx, cancel := shared.ContextWithTimeout(ctx)
		defer cancel()
		bundles, err := loadDSYMBundles(requestCtx, client, buildID)
		if err != nil {
			return nil, false, err
		}
		if len(filterBundlesWithDSYM(bundles)) == 0 {
			return bundles, false, nil
		}
		return bundles, true, nil
	}
	if !selection.Wait {
		bundles, _, err := load(ctx)
		return bundles, err
	}
	fmt.Fprintf(os.Stderr, "Waiting for dSYM files for build %s\n", buildID)
	bundles, err := asc.PollUntil(ctx, selection.Poll, load)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, shared.WithDiagnostic(&dsymNotReadyError{BuildID: buildID}, shared.DiagnosticDSYMNotReady, "--wait")
		}
		return nil, fmt.Errorf("builds dsyms: %w", err)
	}
	return bundles, nil
}

func loadDSYMBundles(ctx context.Context, client *asc.Client, buildID string) ([]dsymBundleInfo, error) {
	bundlesResp, err := client.GetBuildBundlesForBuild(ctx, buildID)
	if err != nil {
		return nil, fmt.Errorf("builds dsyms: %w", err)
	}
	bundles := make([]dsymBundleInfo, 0, len(bundlesResp.Data))
	for _, bundle := range bundlesResp.Data {
		bundleID := ""
		if bundle.Attributes.BundleID != nil {
			bundleID = *bundle.Attributes.BundleID
		}
		bundles = append(bundles, dsymBundleInfo{
			BundleID: bundleID,
			DSYMURL:  bundle.Attributes.DSYMURL,
		})
	}
	return bundles, nil
}

func saveDSYMBundles(ctx context.Context, bundles []dsymBundleInfo, target dsymTarget, dirValue string, multi bool) ([]asc.DSYMDownloadFile, error) {
	downloadable := filterBundlesWithDSYM(bundles)
	files := make([]asc.DSYMDownloadFile, 0, len(downloadable))
	used := map[string]struct{}{}
	for i, bundle := range downloadable {
		fileName := dsymFileName(bundle.BundleID, target.AppVersion, target.BuildNumber, target.ID, i)
		// Bulk selectors can span platforms that reuse bundle IDs and build
		// numbers. Include the immutable build ID so files remain distinct
		// even when a later invocation selects a different set of builds.
		if multi {
			fileName = strings.TrimSuffix(fileName, ".dSYM.zip") + "-" + target.ID + ".dSYM.zip"
		}
		fileName = uniqueDSYMFileName(fileName, target.ID, used)
		filePath := filepath.Join(dirValue, fileName)
		size, sum, skipped, err := saveOneDSYM(ctx, *bundle.DSYMURL, filePath)
		if err != nil {
			return files, fmt.Errorf("builds dsyms: failed to download %s: %w", fileName, err)
		}
		if skipped {
			fmt.Fprintf(os.Stderr, "Skipping existing %s (%d bytes)\n", filePath, size)
		} else {
			fmt.Fprintf(os.Stderr, "Downloading dSYM for %s...\n", displayBundleID(bundle.BundleID, i))
			fmt.Fprintf(os.Stderr, "  Saved %s (%d bytes)\n", filePath, size)
		}
		files = append(files, asc.DSYMDownloadFile{
			BuildID:     target.ID,
			Version:     target.AppVersion,
			BuildNumber: target.BuildNumber,
			BundleID:    bundle.BundleID,
			FileName:    fileName,
			FilePath:    filePath,
			FileSize:    size,
			SHA256:      sum,
			Skipped:     skipped,
		})
	}
	return files, nil
}

func uniqueDSYMFileName(name, buildID string, used map[string]struct{}) string {
	if _, exists := used[name]; !exists {
		used[name] = struct{}{}
		return name
	}
	ext := ".dSYM.zip"
	base := strings.TrimSuffix(name, ext)
	if buildID != "" && !strings.Contains(base, buildID) {
		base += "-" + buildID
	}
	candidate := base + ext
	for i := 2; ; i++ {
		if _, exists := used[candidate]; !exists {
			used[candidate] = struct{}{}
			return candidate
		}
		candidate = fmt.Sprintf("%s-%d%s", base, i, ext)
	}
}

func saveOneDSYM(ctx context.Context, rawURL, destPath string) (int64, string, bool, error) {
	size, sum, exists, err := hashRegularFile(destPath)
	if err != nil {
		return 0, "", false, err
	}
	downloadCtx, cancel := shared.ContextWithDownloadTimeout(ctx)
	defer cancel()
	if exists {
		length, remoteSum, err := hashRemoteDSYM(downloadCtx, rawURL)
		if err != nil {
			return 0, "", false, err
		}
		if length == size && remoteSum == sum {
			return size, sum, true, nil
		}
		return 0, "", false, fmt.Errorf("output file already exists with different content")
	}
	written, err := downloadDSYM(downloadCtx, rawURL, destPath)
	if err != nil {
		return 0, "", false, err
	}
	_, sum, _, err = hashRegularFile(destPath)
	if err != nil {
		return written, "", false, err
	}
	return written, sum, false, nil
}

func hashRemoteDSYM(ctx context.Context, rawURL string) (int64, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, "", urlsanitize.NewTransportError("create dSYM content request", urlsanitize.RedactURLHostForError(rawURL), err)
	}
	client := dsymHTTPClient
	if client == nil {
		client = &http.Client{}
	}
	safeClient := *client
	safeClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := safeClient.Do(req)
	if err != nil {
		return 0, "", urlsanitize.NewTransportError("dSYM content request", urlsanitize.RedactURLHostForError(rawURL), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, "", fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}
	digest := sha256.New()
	size, err := io.Copy(digest, resp.Body)
	if err != nil {
		return 0, "", urlsanitize.NewTransportError("read dSYM content", urlsanitize.RedactURLHostForError(rawURL), err)
	}
	if resp.ContentLength >= 0 && size != resp.ContentLength {
		return 0, "", fmt.Errorf("downloaded size does not match Content-Length")
	}
	return size, hex.EncodeToString(digest.Sum(nil)), nil
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
		return 0, urlsanitize.NewTransportError(
			"create dSYM download request",
			urlsanitize.RedactURLHostForError(rawURL),
			err,
		)
	}

	client := dsymHTTPClient
	if client == nil {
		client = &http.Client{}
	}
	safeClient := *client
	safeClient.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	resp, err := safeClient.Do(req)
	if err != nil {
		return 0, urlsanitize.NewTransportError(
			"dSYM download request",
			urlsanitize.RedactURLHostForError(rawURL),
			err,
		)
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

func printDSYMResultTable(result asc.DSYMDownloadResult) error {
	fmt.Printf("Build ID: %s\n", result.BuildID)
	if result.Version != "" {
		fmt.Printf("Version: %s (%s)\n", result.Version, result.BuildNumber)
	}
	fmt.Printf("Dir: %s\n", result.Dir)
	if result.Status != "" {
		fmt.Printf("Status: %s\n", result.Status)
	}
	fmt.Printf("Files: %d\n\n", len(result.Files))

	if len(result.Files) == 0 {
		asc.RenderTable([]string{"status"}, [][]string{{dsymTableStatus(result)}})
		return nil
	}

	headers, rows := dsymFileRows(result)
	asc.RenderTable(headers, rows)
	return nil
}

func printDSYMResultMarkdown(result asc.DSYMDownloadResult) error {
	fmt.Printf("**Build ID:** %s\n\n", result.BuildID)
	if result.Version != "" {
		fmt.Printf("**Version:** %s (%s)\n\n", result.Version, result.BuildNumber)
	}
	fmt.Printf("**Dir:** %s\n\n", result.Dir)
	if result.Status != "" {
		fmt.Printf("**Status:** %s\n\n", result.Status)
	}
	fmt.Printf("**Files:** %d\n\n", len(result.Files))

	if len(result.Files) == 0 {
		asc.RenderMarkdown([]string{"status"}, [][]string{{dsymTableStatus(result)}})
		return nil
	}

	headers, rows := dsymFileRows(result)
	asc.RenderMarkdown(headers, rows)
	return nil
}

func dsymTableStatus(result asc.DSYMDownloadResult) string {
	if result.Status != "" {
		return result.Status
	}
	return "no dSYM files available"
}

func dsymFileRows(result asc.DSYMDownloadResult) ([]string, [][]string) {
	headers := []string{"bundleId", "fileName", "fileSize"}
	includeBuild := result.BuildID == ""
	includeSHA := false
	for _, file := range result.Files {
		if file.SHA256 != "" {
			includeSHA = true
		}
		if file.BuildID != "" && file.BuildID != result.BuildID {
			includeBuild = true
		}
	}
	if includeBuild {
		headers = append([]string{"buildId"}, headers...)
	}
	if includeSHA {
		headers = append(headers, "sha256")
	}
	rows := make([][]string, 0, len(result.Files))
	for _, file := range result.Files {
		row := []string{file.BundleID, file.FileName, fmt.Sprintf("%d", file.FileSize)}
		if includeBuild {
			row = append([]string{file.BuildID}, row...)
		}
		if includeSHA {
			row = append(row, file.SHA256)
		}
		rows = append(rows, row)
	}
	return headers, rows
}
