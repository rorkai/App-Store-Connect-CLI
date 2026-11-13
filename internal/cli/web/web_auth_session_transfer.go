package web

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

const (
	webSessionBundleTempPattern   = ".asc-web-session-*"
	webSessionBundleBackupPattern = ".asc-web-session-backup-*"
	webSessionBundleFileMode      = 0o600
	webSessionBundleEnvName       = "ASC_WEB_SESSION"
)

// sessionTransferWarning writes the reminders that an exported bundle is a
// credential and that an imported session still needs live validation.
// os.Stderr is resolved per call so redirected process output is honored.
func sessionTransferWarning(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format, args...)
}

func formatSessionBundleTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func formatOptionalSessionBundleTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return formatSessionBundleTime(*value)
}

// WebAuthExportCommand writes the cached web session to a portable file.
func WebAuthExportCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web auth export", flag.ExitOnError)

	appleID := fs.String("apple-id", "", "Apple Account email to export (default exports the last cached session)")
	outputPath := fs.String("output-path", "", "Destination file for the exported session bundle (required)")
	overwrite := fs.Bool("overwrite", false, "Replace an existing file at --output-path")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "export",
		ShortUsage: "asc web auth export --output-path FILE [--apple-id EMAIL] [--overwrite]",
		ShortHelp:  "Export the cached web session to a file.",
		LongHelp: `WEB SESSION WORKFLOWS

Write the cached Apple web session to a JSON bundle so another machine or a CI
job can reuse it with "asc web auth import" instead of repeating two-factor
verification.

The bundle holds live session cookies. It is written with 0600 permissions;
store it in a secrets manager and delete it once it has been consumed. Cookie
values are never printed to stdout.

The bundle records the Apple Account email and the session cookies for Apple's
App Store Connect and developer origins. Apple's login populates a cookie jar
that keeps only cookie names and values, so an exported bundle usually carries
no expiry date and reports "expiresAt" only when the cached cookies record one.
The export itself does not validate the session. Import performs local bundle
validation by default; pass --validate on the target machine to check the
temporary bundle session with Apple before reading or writing the cache. If
--validate is omitted, run "asc web auth status" afterwards to validate the
resumed session when network access is available.

Examples:
  asc web auth export --output-path ./web-session.json
  asc web auth export --output-path ./web-session.json --apple-id "user@example.com"
  asc web auth export --output-path ./web-session.json --overwrite --output json`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			outputPathValue := *outputPath
			if strings.TrimSpace(outputPathValue) == "" {
				return shared.UsageError("--output-path is required")
			}
			trimmedAppleID := strings.TrimSpace(*appleID)

			bundle, ok, err := webcore.ExportSessionBundle(trimmedAppleID)
			if err != nil {
				return fmt.Errorf("web auth export failed: %w", err)
			}
			if !ok || bundle == nil {
				if trimmedAppleID != "" {
					return fmt.Errorf("web auth export failed: no cached web session for %s; run \"asc web auth login\" first", trimmedAppleID)
				}
				return errors.New("web auth export failed: no cached web session; run \"asc web auth login\" first")
			}

			payload, err := json.MarshalIndent(bundle, "", "  ")
			if err != nil {
				return fmt.Errorf("web auth export failed: %w", err)
			}
			payload = append(payload, '\n')

			overwritten, err := writeWebSessionBundle(outputPathValue, payload, *overwrite)
			if err != nil {
				return fmt.Errorf("web auth export failed: %w", err)
			}

			sessionTransferWarning(
				"Wrote Apple web-session credentials to %s. Treat this file as a secret and remove it once it is stored.\n",
				outputPathValue,
			)

			return shared.PrintOutput(&asc.WebSessionExportResult{
				Path:        outputPathValue,
				AppleID:     bundle.AppleID,
				CookieCount: len(bundle.Cookies),
				ExportedAt:  formatSessionBundleTime(bundle.ExportedAt),
				ExpiresAt:   formatOptionalSessionBundleTime(bundle.ExpiresAt),
				Overwritten: overwritten,
			}, *output.Output, *output.Pretty)
		},
	}
}

// writeWebSessionBundle publishes the bundle without following symlinks and
// reports whether an existing file was replaced. The no-replace attempt runs
// first so a missing --overwrite fails before the destination is touched.
func writeWebSessionBundle(path string, payload []byte, overwrite bool) (bool, error) {
	write := func(replace bool) error {
		_, err := shared.SafeWriteFileNoSymlinkWithPreparationAndCreator(
			path,
			webSessionBundleFileMode,
			replace,
			webSessionBundleTempPattern,
			webSessionBundleBackupPattern,
			prepareWebSessionBundleOutput,
			createWebSessionBundleStagingFile,
			func(file *os.File) (int64, error) {
				written, writeErr := file.Write(payload)
				return int64(written), writeErr
			},
		)
		return err
	}

	err := write(false)
	if err == nil {
		return false, nil
	}
	if !errors.Is(err, os.ErrExist) {
		return false, err
	}
	if !overwrite {
		return false, fmt.Errorf("%w; pass --overwrite to replace it", err)
	}
	if err := write(true); err != nil {
		return false, err
	}
	return true, nil
}

// WebAuthImportCommand loads a previously exported session bundle into the
// cache used by "asc web" commands.
func WebAuthImportCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web auth import", flag.ExitOnError)

	filePath := fs.String("file", "", "Path to a session bundle produced by \"asc web auth export\" (mutually exclusive with --from-env)")
	fromEnv := fs.Bool("from-env", false, "Read the canonical session bundle from ASC_WEB_SESSION")
	appleID := fs.String("apple-id", "", "Require the bundle to belong to this Apple Account email")
	overwrite := fs.Bool("overwrite", false, "Replace an existing cached session for the bundle Apple Account")
	validate := fs.Bool("validate", false, "Validate the bundle with Apple before importing it")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "import",
		ShortUsage: "asc web auth import (--file FILE | --from-env) [--apple-id EMAIL] [--overwrite] [--validate]",
		ShortHelp:  "Import a web session bundle into the session cache.",
		LongHelp: `WEB SESSION WORKFLOWS

Load a session bundle written by "asc web auth export" into the same cache
"asc web auth login" writes, so CI can reuse an existing Apple web session
instead of repeating two-factor verification.

Use --from-env to read the same versioned JSON bundle from ASC_WEB_SESSION when
a CI secret store provides environment input. The environment value is
accepted only with this explicit flag; it is never treated as a global session
mode. --file and --from-env are mutually exclusive. Raw cookie headers,
opaque tokens, and other environment variables are not accepted. The same
versioned bundle schema and 1 MiB parser ceiling are used. Process launchers
and operating systems can impose lower per-value or total-environment limits
before asc starts; use --file when the CI environment cannot carry the bundle.
For an environment import, the JSON receipt's path is the fixed
ASC_WEB_SESSION source marker; the value itself is never echoed or staged as an
extra source file. The decoded session is persisted through the configured
cache, just as with --file. Mask the CI secret and disable shell tracing when
using this source.

The bundle is checked before anything is stored: the document kind and version
must match, cookies must belong to Apple's supported session origins, cookie
domains must be storable for those origins, and expired cookies are dropped. A
bundle with no usable cookie is refused without writing to the cache. Pass
--apple-id to refuse a bundle that belongs to a different Apple Account. Pass
--overwrite to replace a cached session that already exists for that account.
The imported session also becomes the last cached session, so "asc web"
commands resume it without --apple-id.

Import performs local bundle validation before writing anything to the cache.
Invalid, expired, malformed, unsupported-origin, and unstorable-cookie bundles
are rejected without changing the cache. Import does not contact Apple unless
--validate is provided. With --validate, asc sends the bundle's temporary
cookie jar to Apple's session endpoint before reading or writing the cache and
requires the returned Apple Account to match the bundle. A failed live
validation leaves the cache unchanged. Without --validate, run "asc web auth
status" afterwards to validate the resumed session with Apple when network
access is available.

Examples:
  asc web auth import --file ./web-session.json
  ASC_WEB_SESSION="$(cat ./web-session.json)" asc web auth import --from-env --validate
  asc web auth import --file ./web-session.json --apple-id "user@example.com" --overwrite
  asc web auth import --file ./web-session.json --validate
  asc web auth import --file ./web-session.json --output json`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			filePathValue := *filePath
			fileFlagProvided := false
			fs.Visit(func(flagDef *flag.Flag) {
				fileFlagProvided = fileFlagProvided || flagDef.Name == "file"
			})
			if *fromEnv && fileFlagProvided {
				return shared.UsageError("--file and --from-env are mutually exclusive")
			}

			var sourceLabel string
			var payload []byte
			var err error
			if *fromEnv {
				sourceLabel = webSessionBundleEnvName
				value, ok := os.LookupEnv(webSessionBundleEnvName)
				if !ok || strings.TrimSpace(value) == "" {
					return shared.UsageError(webSessionBundleEnvName + " is unset or empty when --from-env is used")
				}
				if len(value) > webcore.MaxSessionBundleSize {
					return fmt.Errorf("web auth import failed: %s exceeds %d-byte limit", webSessionBundleEnvName, webcore.MaxSessionBundleSize)
				}
				payload = []byte(value)
			} else {
				if strings.TrimSpace(filePathValue) == "" {
					return shared.UsageError("one of --file or --from-env is required")
				}
				sourceLabel = filePathValue
				payload, err = readWebSessionBundleFile(filePathValue)
			}
			if err != nil {
				return fmt.Errorf("web auth import failed: %w", err)
			}
			bundle, err := webcore.DecodeSessionBundle(payload)
			if err != nil {
				if *fromEnv {
					return fmt.Errorf("web auth import failed: invalid session bundle from %s", webSessionBundleEnvName)
				}
				return fmt.Errorf("web auth import failed: %w", err)
			}
			trimmedAppleID := strings.TrimSpace(*appleID)
			if trimmedAppleID != "" && !strings.EqualFold(trimmedAppleID, bundle.AppleID) {
				return fmt.Errorf("web auth import failed: bundle appleId %s does not match --apple-id %s", bundle.AppleID, trimmedAppleID)
			}
			if *validate {
				requestCtx, cancel := newWebRequestContext(ctx)
				defer cancel()
				if err := webcore.ValidateSessionBundle(requestCtx, bundle); err != nil {
					return fmt.Errorf("web auth import failed: live session validation: %w", err)
				}
			}
			existing, ok, err := webcore.LoadCachedSession(bundle.AppleID)
			if err != nil {
				if !*overwrite {
					return fmt.Errorf("web auth import failed: cached web session for %s could not be read; pass --overwrite to replace it: %w", bundle.AppleID, err)
				}
			} else if ok && existing != nil && !*overwrite {
				return fmt.Errorf("web auth import failed: a cached web session for %s already exists; pass --overwrite to replace it", bundle.AppleID)
			}
			summary, err := webcore.ImportSessionBundleWithOptions(bundle, *overwrite)
			if err != nil {
				return fmt.Errorf("web auth import failed: %w", err)
			}

			sourceNote := ""
			if *fromEnv {
				sourceNote = " from " + webSessionBundleEnvName
			}
			if *validate {
				sessionTransferWarning("Imported web session for %s%s after Apple validation.\n", summary.AppleID, sourceNote)
			} else {
				sessionTransferWarning(
					"Imported web session for %s%s after local bundle validation. Run \"asc web auth status\" to validate it with Apple.\n",
					summary.AppleID, sourceNote,
				)
			}

			return shared.PrintOutput(&asc.WebSessionImportResult{
				Path:                  sourceLabel,
				AppleID:               summary.AppleID,
				CookieCount:           summary.CookieCount,
				SkippedExpiredCookies: summary.SkippedExpired,
				ExpiresAt:             formatOptionalSessionBundleTime(summary.ExpiresAt),
				Imported:              true,
			}, *output.Output, *output.Pretty)
		},
	}
}

func readWebSessionBundleFile(path string) ([]byte, error) {
	file, err := rootfs.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("open --file: %w", err)
	}
	defer file.Close()

	payload, err := io.ReadAll(io.LimitReader(file, webcore.MaxSessionBundleSize+1))
	if err != nil {
		return nil, fmt.Errorf("read --file: %w", err)
	}
	if len(payload) > webcore.MaxSessionBundleSize {
		return nil, fmt.Errorf("session bundle exceeds %d-byte limit", webcore.MaxSessionBundleSize)
	}
	return payload, nil
}
