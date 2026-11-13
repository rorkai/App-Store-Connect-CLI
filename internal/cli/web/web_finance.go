package web

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

var downloadTransactionTaxReportFn = func(ctx context.Context, client *webcore.Client, request webcore.TransactionTaxReportRequest) (*webcore.TransactionTaxReportDownload, error) {
	return client.DownloadTransactionTaxReport(ctx, request)
}

// The finance page can keep a generated report pending for the full bounded
// poll window. Keep the post-auth request context alive for that window plus
// one normal request timeout, while still preserving caller cancellation.
const transactionTaxWorkflowTimeout = 11 * time.Minute

// WebFinanceCommand returns private finance workflows that require an
// authenticated App Store Connect web session.
func WebFinanceCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web finance", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "finance",
		ShortUsage: "asc web finance <subcommand> [flags]",
		ShortHelp:  "Download finance reports through an Apple web session.",
		LongHelp: `WEB SESSION WORKFLOWS

Download finance reports that are available in the App Store Connect finance
web page. The selected month must expose the Transaction Tax Report option.

Examples:
  asc web finance transaction-tax download --date 2026-07 --output-path ./transaction-tax-2026-07.zip

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebTransactionTaxCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebTransactionTaxCommand returns the Transaction Tax Report workflow.
func WebTransactionTaxCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web finance transaction-tax", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "transaction-tax",
		ShortUsage: "asc web finance transaction-tax <subcommand> [flags]",
		ShortHelp:  "Work with Transaction Tax Reports.",
		LongHelp: `WEB SESSION WORKFLOWS

Generate and download a Transaction Tax Report for an eligible finance
period.

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebTransactionTaxDownloadCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebTransactionTaxDownloadCommand generates and saves one Transaction Tax
// Report archive without exposing the generated job ID or signed download URL.
func WebTransactionTaxDownloadCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web finance transaction-tax download", flag.ExitOnError)
	date := fs.String("date", "", "Finance period in YYYY-MM format")
	outputPath := fs.String("output-path", "", "Destination path for the downloaded ZIP archive")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "download",
		ShortUsage: "asc web finance transaction-tax download --date YYYY-MM --output-path PATH [flags]",
		ShortHelp:  "Generate and download a Transaction Tax Report.",
		LongHelp: `WEB SESSION WORKFLOWS

Generate a Transaction Tax Report for an eligible finance period and save the
resulting ZIP archive at --output-path. The destination must not already
exist. The archive is staged and published with mode 0600 after its ZIP
signature has been checked. Output contains only the local save receipt; the
generated job ID, download URL, and finance contents are never printed.

The command does not regenerate a report after a generation, polling, or
download error. A timeout after generation leaves the provider job outcome
unknown.

Example:
  asc web finance transaction-tax download --date 2026-07 --output-path ./transaction-tax-2026-07.zip --output json

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}
			resolvedDate := strings.TrimSpace(*date)
			if _, _, err := normalizeTransactionTaxDateForCLI(resolvedDate); err != nil {
				return shared.UsageError(err.Error())
			}
			outPath := *outputPath
			if strings.TrimSpace(outPath) == "" {
				return shared.UsageError("--output-path is required")
			}
			destination, err := newTransactionTaxDownloadDestination(outPath)
			if err != nil {
				return shared.UsageErrorf("--output-path must be a file path: %v", err)
			}
			defer destination.close()
			if err := destination.check(); err != nil {
				return fmt.Errorf("web finance transaction-tax download failed: %w", err)
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return withWebAuthHint(err, "web finance transaction-tax download")
			}
			requestCtx, workflowCancel := shared.ContextWithResolvedTimeout(shared.ContextWithoutTimeout(requestCtx), transactionTaxWorkflowTimeout)
			defer workflowCancel()
			if session == nil || session.ProviderID <= 0 {
				return errors.New("web finance transaction-tax download failed: web session has no selected provider")
			}
			client := newWebClientFn(session)
			var download *webcore.TransactionTaxReportDownload
			err = withWebSpinner("Generating and downloading Transaction Tax Report", func() error {
				var downloadErr error
				download, downloadErr = downloadTransactionTaxReportFn(requestCtx, client, webcore.TransactionTaxReportRequest{
					ProviderID: session.ProviderID,
					Date:       resolvedDate,
				})
				return downloadErr
			})
			if err != nil {
				return withWebAuthHint(err, "web finance transaction-tax download")
			}
			if download == nil || download.Body == nil {
				return errors.New("web finance transaction-tax download failed: missing report archive")
			}
			defer download.Body.Close()
			bytesWritten, err := destination.write(&transactionTaxZIPReader{source: download.Body})
			if err != nil {
				return fmt.Errorf("web finance transaction-tax download failed: report was fetched but saving %q failed: %w", outPath, err)
			}

			result := &asc.WebTransactionTaxDownloadResult{
				Date:         resolvedDate,
				Path:         outPath,
				BytesWritten: bytesWritten,
				ContentType:  download.ContentType,
				PollStatus:   download.PollStatus,
				Verified:     true,
			}
			if err := shared.PrintOutput(result, *output.Output, *output.Pretty); err != nil {
				return fmt.Errorf("web finance transaction-tax download saved report to %q but could not render receipt: %w", outPath, err)
			}
			return nil
		},
	}
}

func normalizeTransactionTaxDateForCLI(value string) (string, string, error) {
	value = strings.TrimSpace(value)
	if len(value) != len("2006-01") || value[4] != '-' {
		return "", "", errors.New("--date must use YYYY-MM")
	}
	parsed, err := time.Parse("2006-01", value)
	if err != nil || parsed.Format("2006-01") != value {
		return "", "", errors.New("--date must use a valid YYYY-MM month")
	}
	return value[:4], fmt.Sprintf("%d", parsed.Month()), nil
}

type transactionTaxDownloadDestination struct {
	root rootfs.Root
	name string
}

func newTransactionTaxDownloadDestination(outPath string) (transactionTaxDownloadDestination, error) {
	if outPath == "" {
		return transactionTaxDownloadDestination{}, errors.New("path is empty")
	}
	if os.IsPathSeparator(outPath[len(outPath)-1]) {
		return transactionTaxDownloadDestination{}, fmt.Errorf("%q ends with a path separator", outPath)
	}
	base := filepath.Base(outPath)
	if base == "." || base == ".." || base == string(filepath.Separator) {
		return transactionTaxDownloadDestination{}, fmt.Errorf("%q has no file name", outPath)
	}
	if err := rootfs.ValidateRelative(base); err != nil {
		return transactionTaxDownloadDestination{}, err
	}
	root, prefix, err := newDownloadRoot(filepath.Dir(outPath))
	if err != nil {
		return transactionTaxDownloadDestination{}, err
	}
	return transactionTaxDownloadDestination{root: root, name: filepath.Join(prefix, base)}, nil
}

func (d transactionTaxDownloadDestination) check() error {
	if err := d.root.CheckCreateNewFile(d.name); err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("%w; choose a different --output-path", err)
		}
		return err
	}
	return nil
}

func (d transactionTaxDownloadDestination) write(reader io.Reader) (int64, error) {
	return d.root.CreateNewFrom(d.name, reader, 0o600)
}

func (d transactionTaxDownloadDestination) close() {
	_ = d.root.Close()
}

type transactionTaxZIPReader struct {
	source    io.Reader
	prefix    [4]byte
	prefixLen int
	checked   bool
}

func (r *transactionTaxZIPReader) Read(p []byte) (int, error) {
	n, err := r.source.Read(p)
	if n > 0 && !r.checked {
		remaining := len(r.prefix) - r.prefixLen
		copyCount := n
		if copyCount > remaining {
			copyCount = remaining
		}
		copy(r.prefix[r.prefixLen:], p[:copyCount])
		r.prefixLen += copyCount
		if r.prefixLen == len(r.prefix) {
			r.checked = true
			if !validTransactionTaxZIPSignature(r.prefix[:]) {
				return n, errors.New("report archive is not a ZIP file")
			}
		}
	}
	if err == io.EOF && !r.checked {
		return n, errors.New("report archive is empty or truncated")
	}
	return n, err
}

func validTransactionTaxZIPSignature(prefix []byte) bool {
	return bytes.Equal(prefix, []byte("PK\x03\x04")) ||
		bytes.Equal(prefix, []byte("PK\x05\x06")) ||
		bytes.Equal(prefix, []byte("PK\x07\x08"))
}
