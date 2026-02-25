package web

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

var allowedReviewSubmissionStates = map[string]struct{}{
	"READY_FOR_REVIEW":   {},
	"WAITING_FOR_REVIEW": {},
	"IN_REVIEW":          {},
	"UNRESOLVED_ISSUES":  {},
	"CANCELING":          {},
	"COMPLETING":         {},
	"COMPLETE":           {},
}

type reviewAttachmentDownloadResult struct {
	AttachmentID      string `json:"attachmentId"`
	SourceType        string `json:"sourceType"`
	FileName          string `json:"fileName"`
	Path              string `json:"path"`
	ThreadID          string `json:"threadId,omitempty"`
	MessageID         string `json:"messageId,omitempty"`
	ReviewRejectionID string `json:"reviewRejectionId,omitempty"`
	RefreshedURL      bool   `json:"refreshedUrl,omitempty"`
}

type reviewThreadDetails struct {
	Thread     webcore.ResolutionCenterThread    `json:"thread"`
	Messages   []webcore.ResolutionCenterMessage `json:"messages,omitempty"`
	Rejections []webcore.ReviewRejection         `json:"rejections,omitempty"`
}

type reviewShowOutput struct {
	AppID            string                           `json:"appId"`
	Selection        string                           `json:"selection"`
	Submission       *webcore.ReviewSubmission        `json:"submission,omitempty"`
	SubmissionItems  []webcore.ReviewSubmissionItem   `json:"submissionItems,omitempty"`
	Threads          []reviewThreadDetails            `json:"threads,omitempty"`
	Attachments      []webcore.ReviewAttachment       `json:"attachments,omitempty"`
	OutputDirectory  string                           `json:"outputDirectory,omitempty"`
	Downloads        []reviewAttachmentDownloadResult `json:"downloads,omitempty"`
	DownloadFailures []string                         `json:"downloadFailures,omitempty"`
}

func parseSubmissionStates(stateCSV string) ([]string, error) {
	states := shared.SplitCSVUpper(stateCSV)
	if len(states) == 0 {
		return nil, nil
	}
	invalid := make([]string, 0)
	seen := map[string]struct{}{}
	filtered := make([]string, 0, len(states))
	for _, state := range states {
		if _, exists := allowedReviewSubmissionStates[state]; !exists {
			invalid = append(invalid, state)
			continue
		}
		if _, exists := seen[state]; exists {
			continue
		}
		seen[state] = struct{}{}
		filtered = append(filtered, state)
	}
	if len(invalid) > 0 {
		return nil, shared.UsageErrorf("--state contains unsupported value(s): %s", strings.Join(invalid, ", "))
	}
	return filtered, nil
}

func filterSubmissionsByState(submissions []webcore.ReviewSubmission, states []string) []webcore.ReviewSubmission {
	if len(states) == 0 {
		return submissions
	}
	allowed := make(map[string]struct{}, len(states))
	for _, state := range states {
		allowed[strings.ToUpper(strings.TrimSpace(state))] = struct{}{}
	}
	result := make([]webcore.ReviewSubmission, 0, len(submissions))
	for _, submission := range submissions {
		state := strings.ToUpper(strings.TrimSpace(submission.State))
		if _, ok := allowed[state]; ok {
			result = append(result, submission)
		}
	}
	return result
}

func parseSubmissionTime(value string) time.Time {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}
	}
	if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return parsed
	}
	return time.Time{}
}

func newerSubmission(a, b webcore.ReviewSubmission) bool {
	at := parseSubmissionTime(a.SubmittedDate)
	bt := parseSubmissionTime(b.SubmittedDate)
	switch {
	case !at.IsZero() && !bt.IsZero():
		return at.After(bt)
	case !at.IsZero() && bt.IsZero():
		return true
	case at.IsZero() && !bt.IsZero():
		return false
	default:
		return strings.TrimSpace(a.SubmittedDate) > strings.TrimSpace(b.SubmittedDate)
	}
}

func chooseSubmissionForShow(submissions []webcore.ReviewSubmission, preferredID string) (*webcore.ReviewSubmission, string, error) {
	if len(submissions) == 0 {
		return nil, "none", nil
	}
	preferredID = strings.TrimSpace(preferredID)
	if preferredID != "" {
		for i := range submissions {
			if strings.TrimSpace(submissions[i].ID) == preferredID {
				chosen := submissions[i]
				return &chosen, "explicit", nil
			}
		}
		return nil, "", fmt.Errorf("submission %q was not found for this app", preferredID)
	}

	var unresolved *webcore.ReviewSubmission
	var latest *webcore.ReviewSubmission
	for i := range submissions {
		current := submissions[i]
		if latest == nil || newerSubmission(current, *latest) {
			copy := current
			latest = &copy
		}
		if strings.EqualFold(strings.TrimSpace(current.State), "UNRESOLVED_ISSUES") {
			if unresolved == nil || newerSubmission(current, *unresolved) {
				copy := current
				unresolved = &copy
			}
		}
	}
	if unresolved != nil {
		return unresolved, "latest-unresolved", nil
	}
	return latest, "latest", nil
}

func sanitizePathPart(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "unknown"
	}
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_")
	sanitized := replacer.Replace(trimmed)
	if sanitized == "." || sanitized == ".." {
		return "unknown"
	}
	return sanitized
}

func resolveShowOutDir(appID, submissionID, out string) string {
	trimmedOut := strings.TrimSpace(out)
	if trimmedOut != "" {
		return trimmedOut
	}
	return filepath.Join(".asc", "web-review", sanitizePathPart(appID), sanitizePathPart(submissionID))
}

func normalizeAttachmentFilename(attachment webcore.ReviewAttachment) string {
	name := strings.TrimSpace(attachment.FileName)
	if name != "" {
		base := filepath.Base(name)
		if base != "" && base != "." && base != string(filepath.Separator) && base != ".." {
			return base
		}
	}
	id := strings.TrimSpace(attachment.AttachmentID)
	if id == "" {
		id = "attachment"
	}
	return id + ".bin"
}

func resolveDownloadPath(outDir, fileName string, overwrite bool) (string, error) {
	base := filepath.Join(outDir, fileName)
	if overwrite {
		return base, nil
	}
	if _, err := os.Stat(base); err == nil {
		ext := filepath.Ext(fileName)
		stem := strings.TrimSuffix(fileName, ext)
		if stem == "" {
			stem = "attachment"
		}
		for i := 1; i <= 10_000; i++ {
			candidate := filepath.Join(outDir, fmt.Sprintf("%s-%d%s", stem, i, ext))
			if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
				return candidate, nil
			}
		}
		return "", fmt.Errorf("failed to generate unique filename for %q", fileName)
	} else if errors.Is(err, os.ErrNotExist) {
		return base, nil
	} else {
		return "", fmt.Errorf("failed to check destination path %q: %w", base, err)
	}
}

func attachmentRefreshKey(attachment webcore.ReviewAttachment) string {
	return strings.Join([]string{
		strings.TrimSpace(attachment.SourceType),
		strings.TrimSpace(attachment.AttachmentID),
		strings.TrimSpace(attachment.ThreadID),
		strings.TrimSpace(attachment.MessageID),
		strings.TrimSpace(attachment.ReviewRejectionID),
	}, "|")
}

func indexAttachmentsByRefreshKey(attachments []webcore.ReviewAttachment) map[string]webcore.ReviewAttachment {
	result := make(map[string]webcore.ReviewAttachment, len(attachments))
	for _, attachment := range attachments {
		result[attachmentRefreshKey(attachment)] = attachment
	}
	return result
}

func attachmentDownloadResult(attachment webcore.ReviewAttachment, path string, refreshed bool) reviewAttachmentDownloadResult {
	return reviewAttachmentDownloadResult{
		AttachmentID:      attachment.AttachmentID,
		SourceType:        attachment.SourceType,
		FileName:          normalizeAttachmentFilename(attachment),
		Path:              path,
		ThreadID:          attachment.ThreadID,
		MessageID:         attachment.MessageID,
		ReviewRejectionID: attachment.ReviewRejectionID,
		RefreshedURL:      refreshed,
	}
}

func redactAttachmentURLs(attachments []webcore.ReviewAttachment) []webcore.ReviewAttachment {
	redacted := make([]webcore.ReviewAttachment, 0, len(attachments))
	for _, attachment := range attachments {
		copy := attachment
		copy.DownloadURL = ""
		redacted = append(redacted, copy)
	}
	return redacted
}

func buildThreadDetails(ctx context.Context, client *webcore.Client, threads []webcore.ResolutionCenterThread, plainText bool) ([]reviewThreadDetails, []webcore.ReviewAttachment, error) {
	details := make([]reviewThreadDetails, 0, len(threads))
	attachments := make([]webcore.ReviewAttachment, 0)
	seenAttachments := map[string]struct{}{}
	for _, thread := range threads {
		threadDetails, err := client.ListReviewThreadDetails(ctx, thread.ID, plainText, true)
		if err != nil {
			return nil, nil, err
		}
		details = append(details, reviewThreadDetails{
			Thread:     thread,
			Messages:   threadDetails.Messages,
			Rejections: threadDetails.Rejections,
		})
		for _, attachment := range threadDetails.Attachments {
			key := attachmentRefreshKey(attachment)
			if _, exists := seenAttachments[key]; exists {
				continue
			}
			seenAttachments[key] = struct{}{}
			attachments = append(attachments, attachment)
		}
	}
	return details, attachments, nil
}

func downloadAttachmentsForShow(
	ctx context.Context,
	client *webcore.Client,
	attachments []webcore.ReviewAttachment,
	submissionID string,
	outDir string,
	pattern string,
	overwrite bool,
) ([]reviewAttachmentDownloadResult, []string, error) {
	selected := make([]webcore.ReviewAttachment, 0, len(attachments))
	for _, attachment := range attachments {
		attachment.FileName = normalizeAttachmentFilename(attachment)
		if !attachment.Downloadable || strings.TrimSpace(attachment.DownloadURL) == "" {
			continue
		}
		if strings.TrimSpace(pattern) != "" {
			matched, err := filepath.Match(pattern, attachment.FileName)
			if err != nil {
				return nil, nil, shared.UsageErrorf("--pattern is invalid: %v", err)
			}
			if !matched {
				continue
			}
		}
		selected = append(selected, attachment)
	}
	if len(selected) == 0 {
		return []reviewAttachmentDownloadResult{}, nil, nil
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("failed to create output directory %q: %w", outDir, err)
	}

	results := make([]reviewAttachmentDownloadResult, 0, len(selected))
	failures := make([]string, 0)
	var refreshedIndex map[string]webcore.ReviewAttachment

	for _, attachment := range selected {
		body, statusCode, downloadErr := client.DownloadAttachment(ctx, attachment.DownloadURL)
		refreshed := false

		if downloadErr != nil && (statusCode == http.StatusForbidden || statusCode == http.StatusGone) {
			if refreshedIndex == nil {
				refreshedAttachments, refreshErr := client.ListReviewAttachmentsBySubmission(ctx, submissionID, true)
				if refreshErr != nil {
					failures = append(failures, fmt.Sprintf("%s: refresh failed (%v)", attachment.FileName, refreshErr))
					continue
				}
				refreshedIndex = indexAttachmentsByRefreshKey(refreshedAttachments)
			}
			if refreshedAttachment, ok := refreshedIndex[attachmentRefreshKey(attachment)]; ok && strings.TrimSpace(refreshedAttachment.DownloadURL) != "" {
				body, _, downloadErr = client.DownloadAttachment(ctx, refreshedAttachment.DownloadURL)
				if downloadErr == nil {
					attachment = refreshedAttachment
					attachment.FileName = normalizeAttachmentFilename(attachment)
					refreshed = true
				}
			}
		}
		if downloadErr != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", attachment.FileName, downloadErr))
			continue
		}

		outputPath, err := resolveDownloadPath(outDir, attachment.FileName, overwrite)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", attachment.FileName, err))
			continue
		}
		if err := os.WriteFile(outputPath, body, 0o600); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", attachment.FileName, err))
			continue
		}
		results = append(results, attachmentDownloadResult(attachment, outputPath, refreshed))
	}
	return results, failures, nil
}

// WebReviewCommand returns the detached web review command group.
func WebReviewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web review", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "review",
		ShortUsage: "asc web review <subcommand> [flags]",
		ShortHelp:  "EXPERIMENTAL: App-centric review and rejection inspection.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

App-centric review workflows over Apple web-session /iris endpoints.
Use --app to scope all operations to one app.

Subcommands:
  list  List review submissions for an app
  show  Show one submission with threads/messages/rejections and auto-download screenshots

` + webWarningText,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebReviewListCommand(),
			WebReviewShowCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebReviewListCommand lists review submissions for an app.
func WebReviewListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web review list", flag.ExitOnError)

	appID := fs.String("app", "", "App ID")
	stateCSV := fs.String("state", "", "Optional comma-separated state filter")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc web review list --app APP_ID [--state CSV] [flags]",
		ShortHelp:  "EXPERIMENTAL: List app review submissions.",
		FlagSet:    fs,
		UsageFunc:  shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedAppID := strings.TrimSpace(*appID)
			if trimmedAppID == "" {
				return shared.UsageError("--app is required")
			}
			states, err := parseSubmissionStates(*stateCSV)
			if err != nil {
				return err
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			session, err := resolveWebSessionForCommand(requestCtx, authFlags)
			if err != nil {
				return err
			}
			client := webcore.NewClient(session)

			submissions, err := client.ListReviewSubmissions(requestCtx, trimmedAppID)
			if err != nil {
				return withWebAuthHint(err, "web review list")
			}
			filtered := filterSubmissionsByState(submissions, states)
			return shared.PrintOutput(filtered, *output.Output, *output.Pretty)
		},
	}
}

// WebReviewShowCommand shows a submission with full review context and downloads screenshots.
func WebReviewShowCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web review show", flag.ExitOnError)

	appID := fs.String("app", "", "App ID")
	submissionID := fs.String("submission", "", "Review submission ID (default: latest unresolved, else latest)")
	outDir := fs.String("out", "", "Directory for auto-downloaded screenshots (default: ./.asc/web-review/<app>/<submission>)")
	pattern := fs.String("pattern", "", "Optional filename glob filter for auto-download (for example: *.png)")
	overwrite := fs.Bool("overwrite", false, "Overwrite existing files instead of suffixing")
	plainText := fs.Bool("plain-text", false, "Project messageBody HTML into plain text")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "show",
		ShortUsage: "asc web review show --app APP_ID [--submission ID] [--out DIR] [--pattern GLOB] [--overwrite] [flags]",
		ShortHelp:  "EXPERIMENTAL: Show review details and auto-download screenshots.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Show one submission's review context (threads, messages, rejections) and
auto-download available screenshots/attachments in the same command.

Selection:
  - --submission ID          Use an explicit submission
  - without --submission     Pick latest UNRESOLVED_ISSUES submission, otherwise latest submission

` + webWarningText,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedAppID := strings.TrimSpace(*appID)
			if trimmedAppID == "" {
				return shared.UsageError("--app is required")
			}
			trimmedPattern := strings.TrimSpace(*pattern)
			if trimmedPattern != "" {
				if _, err := filepath.Match(trimmedPattern, "sample.png"); err != nil {
					return shared.UsageErrorf("--pattern is invalid: %v", err)
				}
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			session, err := resolveWebSessionForCommand(requestCtx, authFlags)
			if err != nil {
				return err
			}
			client := webcore.NewClient(session)

			submissions, err := client.ListReviewSubmissions(requestCtx, trimmedAppID)
			if err != nil {
				return withWebAuthHint(err, "web review show")
			}
			selectedSubmission, selection, err := chooseSubmissionForShow(submissions, *submissionID)
			if err != nil {
				return err
			}
			if selectedSubmission == nil {
				payload := reviewShowOutput{
					AppID:     trimmedAppID,
					Selection: selection,
				}
				return shared.PrintOutput(payload, *output.Output, *output.Pretty)
			}

			items, err := client.ListReviewSubmissionItems(requestCtx, selectedSubmission.ID)
			if err != nil {
				return withWebAuthHint(err, "web review show")
			}
			threads, err := client.ListResolutionCenterThreadsBySubmission(requestCtx, selectedSubmission.ID)
			if err != nil {
				return withWebAuthHint(err, "web review show")
			}
			threadDetails, attachmentsWithURL, err := buildThreadDetails(requestCtx, client, threads, *plainText)
			if err != nil {
				return withWebAuthHint(err, "web review show")
			}
			outDirResolved := resolveShowOutDir(trimmedAppID, selectedSubmission.ID, *outDir)
			downloads, downloadFailures, err := downloadAttachmentsForShow(
				requestCtx,
				client,
				attachmentsWithURL,
				selectedSubmission.ID,
				outDirResolved,
				trimmedPattern,
				*overwrite,
			)
			if err != nil {
				return err
			}

			payload := reviewShowOutput{
				AppID:            trimmedAppID,
				Selection:        selection,
				Submission:       selectedSubmission,
				SubmissionItems:  items,
				Threads:          threadDetails,
				Attachments:      redactAttachmentURLs(attachmentsWithURL),
				OutputDirectory:  outDirResolved,
				Downloads:        downloads,
				DownloadFailures: downloadFailures,
			}
			if len(downloads) == 0 {
				payload.OutputDirectory = ""
			}

			if err := shared.PrintOutput(payload, *output.Output, *output.Pretty); err != nil {
				return err
			}
			if len(downloadFailures) > 0 {
				return fmt.Errorf("review show completed with %d download failure(s)", len(downloadFailures))
			}
			return nil
		},
	}
}
