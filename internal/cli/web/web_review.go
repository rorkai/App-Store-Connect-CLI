package web

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"html"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
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

var reviewHTMLTagPattern = regexp.MustCompile(`(?s)<[^>]*>`)

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

func buildReviewListTableRows(submissions []webcore.ReviewSubmission) [][]string {
	if len(submissions) == 0 {
		return [][]string{}
	}
	rows := make([][]string, 0, len(submissions))
	for _, submission := range submissions {
		version := ""
		if submission.AppStoreVersionForReview != nil {
			version = strings.TrimSpace(submission.AppStoreVersionForReview.Version)
		}
		if version == "" {
			version = "n/a"
		}

		platform := strings.TrimSpace(submission.Platform)
		if platform == "" && submission.AppStoreVersionForReview != nil {
			platform = strings.TrimSpace(submission.AppStoreVersionForReview.Platform)
		}
		if platform == "" {
			platform = "n/a"
		}

		submitted := strings.TrimSpace(submission.SubmittedDate)
		if submitted == "" {
			submitted = "n/a"
		}
		id := strings.TrimSpace(submission.ID)
		if id == "" {
			id = "n/a"
		}
		state := strings.TrimSpace(submission.State)
		if state == "" {
			state = "n/a"
		}

		rows = append(rows, []string{
			id,
			state,
			submitted,
			version,
			platform,
		})
	}
	return rows
}

func renderReviewListTable(submissions []webcore.ReviewSubmission) error {
	headers := []string{"Submission ID", "State", "Submitted Date", "Version", "Platform"}
	asc.RenderTable(headers, buildReviewListTableRows(submissions))
	return nil
}

func renderReviewListMarkdown(submissions []webcore.ReviewSubmission) error {
	headers := []string{"Submission ID", "State", "Submitted Date", "Version", "Platform"}
	asc.RenderMarkdown(headers, buildReviewListTableRows(submissions))
	return nil
}

func normalizeReviewShowValue(value string) string {
	value = strings.ReplaceAll(value, "\r\n", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\t", " ")
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "n/a"
	}
	return strings.Join(strings.Fields(trimmed), " ")
}

func summarizeSubmissionItemRelated(related []webcore.ReviewSubmissionItemRelation) string {
	if len(related) == 0 {
		return "n/a"
	}
	parts := make([]string, 0, len(related))
	for _, relation := range related {
		parts = append(parts, fmt.Sprintf(
			"%s:%s:%s",
			normalizeReviewShowValue(relation.Relationship),
			normalizeReviewShowValue(relation.Type),
			normalizeReviewShowValue(relation.ID),
		))
	}
	return strings.Join(parts, ", ")
}

func summarizeMessageForTable(message webcore.ResolutionCenterMessage) string {
	body := strings.TrimSpace(message.MessageBodyPlain)
	if body == "" {
		body = strings.TrimSpace(message.MessageBody)
	}
	body = strings.NewReplacer(
		"<br>", "\n",
		"<br/>", "\n",
		"<br />", "\n",
		"</p>", "\n",
		"</h3>", "\n",
		"</li>", "\n",
		"&nbsp;", " ",
	).Replace(body)
	body = reviewHTMLTagPattern.ReplaceAllString(body, " ")
	body = html.UnescapeString(body)
	return normalizeReviewShowValue(body)
}

func summarizeReasonForTable(reason webcore.ReviewRejectionReason) string {
	return fmt.Sprintf(
		"code=%s section=%s description=%s",
		normalizeReviewShowValue(reason.ReasonCode),
		normalizeReviewShowValue(reason.ReasonSection),
		normalizeReviewShowValue(reason.ReasonDescription),
	)
}

func countReviewMessages(threads []reviewThreadDetails) int {
	total := 0
	for _, detail := range threads {
		total += len(detail.Messages)
	}
	return total
}

func countReviewRejections(threads []reviewThreadDetails) int {
	total := 0
	for _, detail := range threads {
		total += len(detail.Rejections)
	}
	return total
}

func buildReviewShowTableRows(payload reviewShowOutput) [][]string {
	rows := make([][]string, 0)
	addRow := func(section, field, value string) {
		rows = append(rows, []string{
			normalizeReviewShowValue(section),
			normalizeReviewShowValue(field),
			normalizeReviewShowValue(value),
		})
	}

	addRow("Submission", "App ID", payload.AppID)
	addRow("Submission", "Selection", payload.Selection)
	if payload.Submission != nil {
		addRow("Submission", "Submission ID", payload.Submission.ID)
		addRow("Submission", "Review Status", payload.Submission.State)
		addRow("Submission", "Submitted Date", payload.Submission.SubmittedDate)
		addRow("Submission", "Platform", payload.Submission.Platform)
		version := "n/a"
		if payload.Submission.AppStoreVersionForReview != nil {
			version = payload.Submission.AppStoreVersionForReview.Version
		}
		addRow("Submission", "App Version", version)
	}
	addRow("Submission", "Items Reviewed Count", fmt.Sprintf("%d", len(payload.SubmissionItems)))
	addRow("Submission", "Threads Count", fmt.Sprintf("%d", len(payload.Threads)))
	addRow("Submission", "Messages Count", fmt.Sprintf("%d", countReviewMessages(payload.Threads)))
	addRow("Submission", "Rejections Count", fmt.Sprintf("%d", countReviewRejections(payload.Threads)))
	addRow("Submission", "Screenshots Found", fmt.Sprintf("%d", len(payload.Attachments)))
	addRow("Submission", "Screenshots Downloaded", fmt.Sprintf("%d", len(payload.Downloads)))
	if strings.TrimSpace(payload.OutputDirectory) != "" {
		addRow("Submission", "Output Directory", payload.OutputDirectory)
	}

	for index, item := range payload.SubmissionItems {
		addRow(
			"Items Reviewed",
			fmt.Sprintf("Item %d", index+1),
			fmt.Sprintf("id=%s type=%s related=%s", item.ID, item.Type, summarizeSubmissionItemRelated(item.Related)),
		)
	}

	messageIndex := 0
	reasonIndex := 0
	for _, detail := range payload.Threads {
		addRow(
			"Threads",
			fmt.Sprintf("Thread %s", normalizeReviewShowValue(detail.Thread.ID)),
			fmt.Sprintf(
				"type=%s state=%s created=%s",
				detail.Thread.ThreadType,
				detail.Thread.State,
				detail.Thread.CreatedDate,
			),
		)

		for _, message := range detail.Messages {
			messageIndex++
			addRow(
				"Messages",
				fmt.Sprintf("Message %d", messageIndex),
				summarizeMessageForTable(message),
			)
		}

		for _, rejection := range detail.Rejections {
			if len(rejection.Reasons) == 0 {
				reasonIndex++
				addRow(
					"Rejections",
					fmt.Sprintf("Reason %d", reasonIndex),
					summarizeReasonForTable(webcore.ReviewRejectionReason{}),
				)
				continue
			}
			for _, reason := range rejection.Reasons {
				reasonIndex++
				addRow(
					"Rejections",
					fmt.Sprintf("Reason %d", reasonIndex),
					summarizeReasonForTable(reason),
				)
			}
		}
	}

	for index, attachment := range payload.Attachments {
		addRow(
			"Screenshots",
			fmt.Sprintf("Attachment %d", index+1),
			fmt.Sprintf(
				"id=%s file=%s size=%d downloadable=%t",
				attachment.AttachmentID,
				normalizeAttachmentFilename(attachment),
				attachment.FileSize,
				attachment.Downloadable,
			),
		)
	}

	for index, download := range payload.Downloads {
		addRow(
			"Downloads",
			fmt.Sprintf("Downloaded %d", index+1),
			fmt.Sprintf(
				"id=%s file=%s path=%s refreshedUrl=%t",
				download.AttachmentID,
				download.FileName,
				download.Path,
				download.RefreshedURL,
			),
		)
	}
	for index, failure := range payload.DownloadFailures {
		addRow("Download Failures", fmt.Sprintf("Failure %d", index+1), failure)
	}
	return rows
}

func renderReviewShowTable(payload reviewShowOutput) error {
	headers := []string{"Section", "Field", "Value"}
	asc.RenderTable(headers, buildReviewShowTableRows(payload))
	return nil
}

func renderReviewShowMarkdown(payload reviewShowOutput) error {
	headers := []string{"Section", "Field", "Value"}
	asc.RenderMarkdown(headers, buildReviewShowTableRows(payload))
	return nil
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

func sanitizeFilenamePart(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(trimmed))
	for _, r := range trimmed {
		isASCIIAlpha := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		isDigit := r >= '0' && r <= '9'
		switch {
		case isASCIIAlpha || isDigit || r == '.' || r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	sanitized := strings.TrimSpace(b.String())
	sanitized = strings.Trim(sanitized, "._-")
	if sanitized == "" || sanitized == "." || sanitized == ".." {
		return ""
	}
	return sanitized
}

func normalizeAttachmentFilename(attachment webcore.ReviewAttachment) string {
	name := strings.TrimSpace(attachment.FileName)
	if name != "" {
		base := sanitizeFilenamePart(filepath.Base(name))
		if base != "" {
			return base
		}
	}
	id := sanitizeFilenamePart(strings.TrimSpace(attachment.AttachmentID))
	if id == "" {
		id = "attachment"
	}
	return id + ".bin"
}

func ensurePathWithinDir(root, candidate string) error {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("failed to resolve output directory %q: %w", root, err)
	}
	absCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return fmt.Errorf("failed to resolve destination path %q: %w", candidate, err)
	}
	rel, err := filepath.Rel(absRoot, absCandidate)
	if err != nil {
		return fmt.Errorf("failed to compare destination path %q: %w", candidate, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("destination path escapes output directory")
	}
	return nil
}

func resolveDownloadPath(outDir, fileName string, overwrite bool) (string, error) {
	base := filepath.Join(outDir, fileName)
	if err := ensurePathWithinDir(outDir, base); err != nil {
		return "", err
	}
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
			if err := ensurePathWithinDir(outDir, candidate); err != nil {
				return "", err
			}
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
			return shared.PrintOutputWithRenderers(
				filtered,
				*output.Output,
				*output.Pretty,
				func() error { return renderReviewListTable(filtered) },
				func() error { return renderReviewListMarkdown(filtered) },
			)
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

			if err := shared.PrintOutputWithRenderers(
				payload,
				*output.Output,
				*output.Pretty,
				func() error { return renderReviewShowTable(payload) },
				func() error { return renderReviewShowMarkdown(payload) },
			); err != nil {
				return err
			}
			if len(downloadFailures) > 0 {
				return fmt.Errorf("review show completed with %d download failure(s)", len(downloadFailures))
			}
			return nil
		},
	}
}
