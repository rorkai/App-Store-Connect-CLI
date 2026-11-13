package web

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

var getAgreementsStatusFn = func(ctx context.Context, client *webcore.Client) (*asc.WebAgreementsStatusResult, error) {
	return client.GetAgreementsStatus(ctx)
}

var getAgreementHistoryFn = func(ctx context.Context, client *webcore.Client) (*asc.WebAgreementsStatusResult, error) {
	return client.GetAgreementHistory(ctx)
}

var acceptAgreementsFn = func(ctx context.Context, client *webcore.Client, req webcore.AgreementsAcceptRequest) (*asc.WebAgreementsAcceptResult, error) {
	return client.AcceptAgreements(ctx, req)
}

var downloadAgreementFn = func(ctx context.Context, client *webcore.Client, agreementID string) (*webcore.AgreementDownload, error) {
	return client.DownloadAgreement(ctx, agreementID)
}

// WebAgreementsCommand returns the Apple Developer Program agreements group.
func WebAgreementsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web agreements", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "agreements",
		ShortUsage: "asc web agreements <subcommand> [flags]",
		ShortHelp:  "Check, download, and accept Apple Developer Program agreements.",
		LongHelp: `WEB SESSION WORKFLOWS

Check, download, and accept Apple Developer Program agreements, such as the
Apple Developer Program License Agreement, through Apple web-session
endpoints. The public App Store Connect API has no endpoint for these
agreements.

Examples:
  asc web agreements status
  asc web agreements download --agreement-id "AGREEMENT_ID" --out ./agreement.pdf
  asc web agreements accept --agreement-id "AGREEMENT_ID" --confirm

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebAgreementsStatusCommand(),
			WebAgreementsDownloadCommand(),
			WebAgreementsAcceptCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebAgreementsStatusCommand reports pending and accepted program agreements.
func WebAgreementsStatusCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web agreements status", flag.ExitOnError)

	authFlags := bindWebSessionFlags(fs)
	portalFlags := bindDeveloperPortalFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "status",
		ShortUsage: "asc web agreements status [flags]",
		ShortHelp:  "Show Apple Developer Program agreement status.",
		LongHelp: `WEB SESSION WORKFLOWS

Show the App Store Connect agreement alert banner and the team's Apple
Developer Program agreement history and outstanding agreement notifications,
including updates that do not yet appear in accepted history.

The compatibility field pending is true when App Store Connect shows an
agreement alert, an agreement's accepted date is older than its effective
date, or Developer Portal reports an outstanding agreement. pendingAgreementIds
lists IDs identified as pending in history or outstanding notifications.
An alert alone does not identify an agreement to accept. Review warnings and
the Apple account before choosing an ID; do not reaccept an accepted row
solely because pending is true.

Example:
  asc web agreements status

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := validateDeveloperPortalFlags(portalFlags); err != nil {
				return err
			}
			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return err
			}
			client := newDeveloperPortalClient(session, portalFlags)

			var result *asc.WebAgreementsStatusResult
			err = withWebSpinner("Fetching Apple Developer Program agreement status", func() error {
				var statusErr error
				result, statusErr = getAgreementsStatusFn(requestCtx, client)
				return statusErr
			})
			if err != nil {
				return withWebAuthHint(err, "web agreements status")
			}
			if result == nil {
				return fmt.Errorf("web agreements status failed: missing status result")
			}
			persistDeveloperPortalSession(session)

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// WebAgreementsDownloadCommand saves the content of one program agreement.
func WebAgreementsDownloadCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web agreements download", flag.ExitOnError)

	agreementID := fs.String("agreement-id", "", "Developer Portal agreement ID to download (from `asc web agreements status`)")
	out := fs.String("out", "", "Destination file path for the agreement content")
	overwrite := fs.Bool("overwrite", false, "Replace an existing file at --out")
	authFlags := bindWebSessionFlags(fs)
	portalFlags := bindDeveloperPortalFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "download",
		ShortUsage: "asc web agreements download --agreement-id AGREEMENT_ID --out ./agreement.pdf [flags]",
		ShortHelp:  "Download an Apple Developer Program agreement.",
		LongHelp: `WEB SESSION WORKFLOWS

Download the content of an Apple Developer Program agreement, such as the
Apple Developer Program License Agreement PDF, so it can be reviewed before
it is accepted. The file is fetched with the web session from the Developer
Portal origin only; redirects to other origins or to plain HTTP are rejected.

Download requires the agreement to appear in Developer Portal history with a
download URL. Outstanding IDs reported only by notifications may need to be
reviewed in the Apple Developer account before acceptance.

The complete content is staged in a temporary file and renamed into --out
with mode 0600, so a partial or empty download never lands at --out; on
Windows, replacing an existing file with --overwrite may briefly leave the
destination absent while the rename completes. File contents are never
printed to command output, and this command's receipt and errors never
include the download URL (which may be signed). The URL is still reported by
'asc web agreements status' as downloadUrl.

Examples:
  asc web agreements download --agreement-id "AGREEMENT_ID" --out ./agreement.pdf
  asc web agreements download --agreement-id "AGREEMENT_ID" --out ./agreement.pdf --overwrite

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}

			resolvedAgreementID := strings.TrimSpace(*agreementID)
			if resolvedAgreementID == "" {
				return shared.UsageError("--agreement-id is required")
			}
			// Keep the operator's path byte-for-byte; trimming would silently
			// redirect the write (and any --overwrite) to a different file.
			outPath := *out
			if strings.TrimSpace(outPath) == "" {
				return shared.UsageError("--out is required")
			}
			destination, err := newAgreementDownloadDestination(outPath)
			if err != nil {
				return shared.UsageErrorf("--out must be a file path: %v", err)
			}
			defer destination.close()
			if err := destination.check(*overwrite); err != nil {
				return fmt.Errorf("web agreements download failed: %w", err)
			}

			if err := validateDeveloperPortalFlags(portalFlags); err != nil {
				return err
			}
			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return err
			}
			client := newDeveloperPortalClient(session, portalFlags)

			var download *webcore.AgreementDownload
			err = withWebSpinner("Downloading Apple Developer Program agreement", func() error {
				var downloadErr error
				download, downloadErr = downloadAgreementFn(requestCtx, client, resolvedAgreementID)
				return downloadErr
			})
			if err != nil {
				return withWebAuthHint(err, "web agreements download")
			}
			if download == nil {
				return fmt.Errorf("web agreements download failed: missing download result")
			}
			// Persist before the local save so a later retry without
			// --developer-team still targets the team that produced this download.
			persistDeveloperPortalSession(session)

			if err := destination.write(download.Body, *overwrite); err != nil {
				return fmt.Errorf("web agreements download failed: agreement %q was downloaded but saving %q failed: %w", resolvedAgreementID, outPath, err)
			}

			return shared.PrintOutput(&asc.WebAgreementDownloadResult{
				AgreementID:  download.AgreementID,
				TeamID:       download.TeamID,
				Title:        download.Title,
				Version:      download.Version,
				Path:         outPath,
				BytesWritten: int64(len(download.Body)),
				ContentType:  download.ContentType,
			}, *output.Output, *output.Pretty)
		},
	}
}

// agreementDownloadDestination anchors an agreement download at the
// operator-selected output directory (or the working directory when --out is
// inside it) so every component below the root is validated before writing.
type agreementDownloadDestination struct {
	root rootfs.Root
	name string
}

func newAgreementDownloadDestination(outPath string) (agreementDownloadDestination, error) {
	if os.IsPathSeparator(outPath[len(outPath)-1]) {
		return agreementDownloadDestination{}, fmt.Errorf("%q ends with a path separator", outPath)
	}
	base := filepath.Base(outPath)
	if base == "." || base == ".." || base == string(filepath.Separator) {
		return agreementDownloadDestination{}, fmt.Errorf("%q has no file name", outPath)
	}
	if err := rootfs.ValidateRelative(base); err != nil {
		return agreementDownloadDestination{}, err
	}
	root, prefix, err := newDownloadRoot(filepath.Dir(outPath))
	if err != nil {
		return agreementDownloadDestination{}, err
	}
	return agreementDownloadDestination{root: root, name: filepath.Join(prefix, base)}, nil
}

// check validates the destination before any network activity. Without
// overwrite an existing file is refused; with overwrite only a regular,
// non-symlinked file may be replaced (by rename, so it need not be writable).
func (d agreementDownloadDestination) check(overwrite bool) error {
	if overwrite {
		return d.root.CheckWriteFile(d.name)
	}
	if err := d.root.CheckCreateNewFile(d.name); err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("%w; pass --overwrite to replace it", err)
		}
		return err
	}
	return nil
}

// write publishes the complete body atomically. Without overwrite it insists
// on a native no-replace rename and fails instead of degrading to a direct
// O_EXCL write that could expose a partial agreement.
func (d agreementDownloadDestination) write(body []byte, overwrite bool) error {
	if overwrite {
		return d.root.WriteFile(d.name, body, 0o600)
	}
	if err := d.root.CreateNewFileAtomic(d.name, body, 0o600); err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("%w; pass --overwrite to replace it", err)
		}
		return err
	}
	return nil
}

// close releases the directory descriptor pinned by the destination root.
func (d agreementDownloadDestination) close() {
	_ = d.root.Close()
}

// WebAgreementsAcceptCommand accepts one or more pending program agreements.
func WebAgreementsAcceptCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web agreements accept", flag.ExitOnError)

	var agreementIDs shared.MultiStringFlag
	fs.Var(&agreementIDs, "agreement-id", "Developer Portal agreement ID to accept (from `asc web agreements status`; repeatable)")
	confirm := fs.Bool("confirm", false, "Confirm accepting the agreements on behalf of the Account Holder")
	authFlags := bindWebSessionFlags(fs)
	portalFlags := bindDeveloperPortalFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "accept",
		ShortUsage: "asc web agreements accept --agreement-id AGREEMENT_ID [--agreement-id AGREEMENT_ID ...] --confirm [flags]",
		ShortHelp:  "Accept Apple Developer Program agreements.",
		LongHelp: `WEB SESSION WORKFLOWS

Accept one or more Apple Developer Program agreements, such as an updated
Apple Developer Program License Agreement, for the web session's team.
Repeat --agreement-id to accept several agreements in a single request; every
agreement must be named explicitly.

Accepting an agreement is a legal action. Apple only allows the team's
Account Holder to accept agreements; sessions for other roles fail with an
Apple error. Find pending agreement IDs with:
  asc web agreements status

After the write, the command re-reads the team's agreement history and fails
with a non-zero exit when any requested agreement is still pending or missing.
The receipt reflects that re-read server state.

Examples:
  asc web agreements accept --agreement-id "AGREEMENT_ID" --confirm
  asc web agreements accept --agreement-id "AGREEMENT_ID" --agreement-id "OTHER_AGREEMENT_ID" --confirm

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAgreementIDs := uniqueTrimmedStrings(agreementIDs)
			switch {
			case len(resolvedAgreementIDs) == 0:
				return shared.UsageError("--agreement-id is required")
			case !*confirm:
				return shared.UsageError("--confirm is required")
			}

			if err := validateDeveloperPortalFlags(portalFlags); err != nil {
				return err
			}
			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return err
			}
			client := newDeveloperPortalClient(session, portalFlags)

			var accepted *asc.WebAgreementsAcceptResult
			err = withWebSpinner("Accepting Apple Developer Program agreements", func() error {
				var acceptErr error
				accepted, acceptErr = acceptAgreementsFn(requestCtx, client, webcore.AgreementsAcceptRequest{
					AgreementIDs: resolvedAgreementIDs,
				})
				return acceptErr
			})
			// Persist after the accept attempt so a later status/retry without
			// --developer-team still inspects the team that may have accepted,
			// including malformed 2xx bodies that never produce a receipt.
			persistDeveloperPortalSession(session)
			if err != nil {
				return withWebAuthHint(err, "web agreements accept")
			}
			if accepted == nil {
				return fmt.Errorf("web agreements accept failed: missing accept result")
			}

			// Verify against the Developer Portal history alone; the combined
			// status read also depends on the App Store Connect banner endpoint,
			// whose failure would falsely report an unverified acceptance.
			// The write already happened; give the verification read its own
			// timeout window instead of whatever the accept request left over.
			verifyCtx, cancelVerify := shared.ContextWithTimeout(ctx)
			defer cancelVerify()
			var status *asc.WebAgreementsStatusResult
			err = withWebSpinner("Verifying Apple Developer Program agreement status", func() error {
				var statusErr error
				status, statusErr = getAgreementHistoryFn(verifyCtx, client)
				return statusErr
			})
			if err != nil {
				return fmt.Errorf("web agreements accept failed: the accept request succeeded but re-reading agreement history failed; run 'asc web agreements status' to confirm the result: %w", err)
			}
			if status == nil {
				return fmt.Errorf("web agreements accept failed: the accept request succeeded but the agreement history re-read returned no result")
			}
			result, err := verifyAcceptedAgreements(accepted.TeamID, resolvedAgreementIDs, status)
			if err != nil {
				return fmt.Errorf("web agreements accept failed: %w", err)
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// verifyAcceptedAgreements builds the accept receipt from the re-read agreement
// history. Every requested agreement must be present and no longer pending.
func verifyAcceptedAgreements(teamID string, requestedIDs []string, status *asc.WebAgreementsStatusResult) (*asc.WebAgreementsAcceptResult, error) {
	byID := make(map[string]asc.WebAgreement, len(status.Agreements))
	for _, agreement := range status.Agreements {
		byID[agreement.AgreementID] = agreement
	}
	if strings.TrimSpace(teamID) == "" {
		teamID = status.TeamID
	}

	var missing, pending []string
	agreements := make([]asc.WebAgreement, 0, len(requestedIDs))
	for _, id := range requestedIDs {
		agreement, ok := byID[id]
		switch {
		case !ok:
			missing = append(missing, id)
		case agreement.Pending:
			pending = append(pending, id)
		default:
			agreements = append(agreements, agreement)
		}
	}
	if len(missing) > 0 || len(pending) > 0 {
		var problems []string
		if len(pending) > 0 {
			problems = append(problems, fmt.Sprintf("agreement(s) %s are still pending after acceptance", strings.Join(pending, ", ")))
		}
		if len(missing) > 0 {
			problems = append(problems, fmt.Sprintf("agreement(s) %s are not present in the team's agreement history", strings.Join(missing, ", ")))
		}
		return nil, fmt.Errorf("%s; run 'asc web agreements status' to inspect", strings.Join(problems, "; "))
	}
	return &asc.WebAgreementsAcceptResult{
		TeamID:       teamID,
		AgreementIDs: requestedIDs,
		Status:       "accepted",
		Verified:     true,
		Agreements:   agreements,
	}, nil
}

func uniqueTrimmedStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
