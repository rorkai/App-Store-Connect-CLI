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
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
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
	AppID             string                           `json:"appId"`
	Selection         string                           `json:"selection"`
	Submission        *webcore.ReviewSubmission        `json:"submission,omitempty"`
	SubmissionItems   []webcore.ReviewSubmissionItem   `json:"submissionItems,omitempty"`
	Threads           []reviewThreadDetails            `json:"threads,omitempty"`
	AppThreads        []webcore.ResolutionCenterThread `json:"appThreads,omitempty"`
	AppThreadsWarning string                           `json:"appThreadsWarning,omitempty"`
	Attachments       []webcore.ReviewAttachment       `json:"attachments,omitempty"`
	OutputDirectory   string                           `json:"outputDirectory,omitempty"`
	Downloads         []reviewAttachmentDownloadResult `json:"downloads,omitempty"`
	DownloadFailures  []string                         `json:"downloadFailures,omitempty"`
}

// reviewThreadEntry pairs an app-scoped resolution center thread with the
// read-only draft message Apple keeps on it, when drafts were requested.
type reviewThreadEntry struct {
	Thread       webcore.ResolutionCenterThread        `json:"thread"`
	DraftMessage *webcore.ResolutionCenterDraftMessage `json:"draftMessage,omitempty"`
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

func summarizeThreadVersions(thread webcore.ResolutionCenterThread) string {
	if len(thread.AppStoreVersionIDs) == 0 {
		return "n/a"
	}
	return strings.Join(thread.AppStoreVersionIDs, ", ")
}

func summarizeDraftForTable(draft *webcore.ResolutionCenterDraftMessage) string {
	if draft == nil {
		return "none"
	}
	return fmt.Sprintf(
		"id=%s created=%s attachments=%d body=%s",
		normalizeReviewShowValue(draft.ID),
		normalizeReviewShowValue(draft.CreatedDate),
		len(draft.Attachments),
		summarizeHTMLBodyForTable(draft.MessageBodyPlain, draft.MessageBody),
	)
}

func buildReviewThreadsTableRows(entries []reviewThreadEntry, drafts bool) [][]string {
	rows := make([][]string, 0, len(entries))
	for _, entry := range entries {
		row := []string{
			normalizeReviewShowValue(entry.Thread.ID),
			normalizeReviewShowValue(entry.Thread.ThreadType),
			normalizeReviewShowValue(entry.Thread.State),
			normalizeReviewShowValue(entry.Thread.CreatedDate),
			normalizeReviewShowValue(entry.Thread.LastMessageResponseDate),
			normalizeReviewShowValue(entry.Thread.ReviewSubmissionID),
			normalizeReviewShowValue(summarizeThreadVersions(entry.Thread)),
		}
		if drafts {
			row = append(row, normalizeReviewShowValue(summarizeDraftForTable(entry.DraftMessage)))
		}
		rows = append(rows, row)
	}
	return rows
}

func reviewThreadsTableHeaders(drafts bool) []string {
	headers := []string{"Thread ID", "Type", "State", "Created Date", "Last Message Date", "Submission ID", "Version IDs"}
	if drafts {
		headers = append(headers, "Draft")
	}
	return headers
}

func renderReviewThreadsTable(entries []reviewThreadEntry, drafts bool) error {
	asc.RenderTable(reviewThreadsTableHeaders(drafts), buildReviewThreadsTableRows(entries, drafts))
	return nil
}

func renderReviewThreadsMarkdown(entries []reviewThreadEntry, drafts bool) error {
	asc.RenderMarkdown(reviewThreadsTableHeaders(drafts), buildReviewThreadsTableRows(entries, drafts))
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
		part := fmt.Sprintf(
			"%s:%s:%s",
			normalizeReviewShowValue(relation.Relationship),
			normalizeReviewShowValue(relation.Type),
			normalizeReviewShowValue(relation.ID),
		)
		if label := strings.TrimSpace(relation.Label); label != "" {
			part += ":" + normalizeReviewShowValue(label)
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, ", ")
}

func summarizeHTMLBodyForTable(plain, raw string) string {
	if plain = strings.TrimSpace(plain); plain != "" {
		return normalizeReviewShowValue(plain)
	}
	body := strings.TrimSpace(raw)
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

func summarizeMessageForTable(message webcore.ResolutionCenterMessage) string {
	return summarizeHTMLBodyForTable(message.MessageBodyPlain, message.MessageBody)
}

func summarizeReasonForTable(reason webcore.ReviewRejectionReason) string {
	return fmt.Sprintf(
		"code=%s section=%s description=%s",
		normalizeReviewShowValue(reason.ReasonCode),
		normalizeReviewShowValue(reason.ReasonSection),
		normalizeReviewShowValue(reason.ReasonDescription),
	)
}

func summarizeRelatedResource(related webcore.ReviewRelatedResource) string {
	label := strings.TrimSpace(related.Label)
	if label == "" {
		label = related.ID
	}
	typeName := strings.TrimSpace(related.Type)
	if typeName == "" {
		typeName = related.Relationship
	}
	return strings.TrimSpace(typeName + " " + label)
}

func summarizeRejectionArtifacts(related []webcore.ReviewRelatedResource) string {
	if len(related) == 0 {
		return ""
	}
	parts := make([]string, 0, len(related))
	for _, item := range related {
		parts = append(parts, summarizeRelatedResource(item))
	}
	return strings.Join(parts, ", ")
}

func formatRejectionReasonRow(rejection webcore.ReviewRejection, reason webcore.ReviewRejectionReason) string {
	summary := summarizeReasonForTable(reason)
	if artifacts := summarizeRejectionArtifacts(rejection.Related); artifacts != "" {
		return "artifact=" + artifacts + " " + summary
	}
	return summary
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
	addRow("Submission", "App Threads Count", fmt.Sprintf("%d", len(payload.AppThreads)))
	if strings.TrimSpace(payload.AppThreadsWarning) != "" {
		addRow("Submission", "App Threads Warning", payload.AppThreadsWarning)
	}
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
					formatRejectionReasonRow(rejection, webcore.ReviewRejectionReason{}),
				)
				continue
			}
			for _, reason := range rejection.Reasons {
				reasonIndex++
				addRow(
					"Rejections",
					fmt.Sprintf("Reason %d", reasonIndex),
					formatRejectionReasonRow(rejection, reason),
				)
			}
		}
	}

	for _, thread := range payload.AppThreads {
		addRow(
			"App Threads",
			fmt.Sprintf("Thread %s", normalizeReviewShowValue(thread.ID)),
			fmt.Sprintf(
				"type=%s state=%s created=%s lastMessage=%s",
				thread.ThreadType,
				thread.State,
				thread.CreatedDate,
				thread.LastMessageResponseDate,
			),
		)
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
		return nil, "", shared.WithDiagnostic(
			fmt.Errorf("submission %q was not found for this app", preferredID),
			shared.DiagnosticResourceNotFound,
			"--submission",
		)
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
	if out != "" {
		return out
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

// newDownloadRoot anchors attachment downloads for outDir. A directory inside
// the working directory (including the default .asc/web-review location, whose
// components are repository-controlled) is anchored at the working directory so
// every component below it is validated; an operator-selected directory outside
// the working directory is its own trusted root. The returned prefix is the
// root-relative output directory.
func newDownloadRoot(outDir string) (rootfs.Root, string, error) {
	if outDir == "" {
		return rootfs.Root{}, "", fmt.Errorf("output directory is required")
	}
	absolute, err := filepath.Abs(outDir)
	if err != nil {
		return rootfs.Root{}, "", fmt.Errorf("failed to resolve output directory %q: %w", outDir, err)
	}
	if cwd, cwdErr := os.Getwd(); cwdErr == nil {
		if root, rootErr := rootfs.New(cwd); rootErr == nil {
			if relative, relErr := filepath.Rel(root.Path(), absolute); relErr == nil {
				if relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
					return root, relative, nil
				}
			}
			// The output directory lives outside cwd; release this candidate
			// before anchoring a root there instead.
			_ = root.Close()
		}
	}
	root, err := rootfs.New(absolute)
	if err != nil {
		return rootfs.Root{}, "", fmt.Errorf("failed to create output directory %q: %w", outDir, err)
	}
	return root, ".", nil
}

// resolveDownloadPath returns the root-relative destination name for fileName
// beneath the prefix directory. File names that traverse outside the root, or
// that resolve through a symlinked parent or destination, are rejected before
// anything is written.
func resolveDownloadPath(root rootfs.Root, prefix, fileName string, overwrite bool) (string, error) {
	// Validate the attachment-supplied file name on its own so joining it onto
	// the prefix cannot lexically collapse a ".." segment into a sibling path.
	if err := rootfs.ValidateRelative(fileName); err != nil {
		return "", err
	}
	name := filepath.Join(prefix, fileName)
	if err := root.CheckContained(name); err != nil {
		return "", err
	}
	base := filepath.Join(root.Path(), name)
	if overwrite {
		return name, nil
	}
	if _, err := os.Lstat(base); err == nil {
		ext := filepath.Ext(fileName)
		stem := strings.TrimSuffix(fileName, ext)
		if stem == "" {
			stem = "attachment"
		}
		for i := 1; i <= 10_000; i++ {
			candidate := filepath.Join(prefix, fmt.Sprintf("%s-%d%s", stem, i, ext))
			if err := root.CheckContained(candidate); err != nil {
				return "", err
			}
			if _, err := os.Lstat(filepath.Join(root.Path(), candidate)); errors.Is(err, os.ErrNotExist) {
				return candidate, nil
			}
		}
		return "", fmt.Errorf("failed to generate unique filename for %q", fileName)
	} else if errors.Is(err, os.ErrNotExist) {
		return name, nil
	} else {
		return "", fmt.Errorf("failed to check destination path %q: %w", base, err)
	}
}

// writeDownloadedAttachment writes an attachment body beneath the download root.
func writeDownloadedAttachment(root rootfs.Root, name string, body []byte) error {
	return root.WriteFile(name, body, 0o600)
}

// downloadDisplayPath reports the download location the way the operator wrote
// it: the selected (possibly relative) output directory joined with the file
// name resolved beneath the root-relative prefix.
func downloadDisplayPath(outDir, prefix, outputName string) string {
	relative, err := filepath.Rel(prefix, outputName)
	if err != nil {
		return filepath.Join(outDir, filepath.Base(outputName))
	}
	return filepath.Join(outDir, relative)
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

// webReviewDraftMessagesTimeout bounds the whole sequential draft-reading
// phase of "web review threads --drafts". Each draft is one request and the
// web client paces requests by its minimum request interval, so an app with
// many threads needs an operation-sized budget instead of the single-request
// default. An explicit ASC_TIMEOUT still wins.
const webReviewDraftMessagesTimeout = 10 * time.Minute

// reviewDraftMessagesContext returns the operation-sized context used for the
// per-thread draft reads.
func reviewDraftMessagesContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return shared.ContextWithResolvedTimeout(shared.ContextWithoutTimeout(ctx), webReviewDraftMessagesTimeout)
}

// reviewAppThreadsContext returns an independently bounded context for the
// best-effort app-scoped thread lookup, so time spent there cannot expire the
// budget the essential review requests rely on.
func reviewAppThreadsContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return shared.ContextWithTimeout(shared.ContextWithoutTimeout(ctx))
}

// loadAppThreads reads every resolution center thread on an app. The app-scoped
// relationship is an undocumented web-session surface, so a failure downgrades
// to a warning instead of failing the whole command: the submission-scoped
// review context stays useful without it.
func loadAppThreads(ctx context.Context, client *webcore.Client, appID string) ([]webcore.ResolutionCenterThread, string, error) {
	threads, err := withWebSpinnerValue("Loading app resolution center threads", func() ([]webcore.ResolutionCenterThread, error) {
		return client.ListResolutionCenterThreadsByApp(ctx, appID)
	})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, "", err
		}
		warning := fmt.Sprintf("app-scoped resolution center threads unavailable: %v", err)
		fmt.Fprintf(os.Stderr, "Warning: %s\n", warning)
		return nil, warning, nil
	}
	return threads, "", nil
}

// appThreadsOutsideSubmission returns the app threads that the selected
// submission's threads do not already cover, so both scopes are reported
// without duplication.
func appThreadsOutsideSubmission(appThreads []webcore.ResolutionCenterThread, details []reviewThreadDetails) []webcore.ResolutionCenterThread {
	if len(appThreads) == 0 {
		return nil
	}
	covered := make(map[string]struct{}, len(details))
	for _, detail := range details {
		covered[strings.TrimSpace(detail.Thread.ID)] = struct{}{}
	}
	remaining := make([]webcore.ResolutionCenterThread, 0, len(appThreads))
	for _, thread := range appThreads {
		if _, exists := covered[strings.TrimSpace(thread.ID)]; exists {
			continue
		}
		remaining = append(remaining, thread)
	}
	if len(remaining) == 0 {
		return nil
	}
	return remaining
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
	root, prefix, err := newDownloadRoot(outDir)
	if err != nil {
		return nil, nil, err
	}
	defer root.Close()
	if err := root.MkdirAll(prefix, 0o755); err != nil {
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

		outputName, err := resolveDownloadPath(root, prefix, attachment.FileName, overwrite)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", attachment.FileName, err))
			continue
		}
		if err := writeDownloadedAttachment(root, outputName, body); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", attachment.FileName, err))
			continue
		}
		results = append(results, attachmentDownloadResult(attachment, downloadDisplayPath(outDir, prefix, outputName), refreshed))
	}
	return results, failures, nil
}

// WebReviewCommand returns the detached web review command group.
func WebReviewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web review", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "review",
		ShortUsage: "asc web review <subcommand> [flags]",
		ShortHelp:  "App-centric review and rejection inspection.",
		LongHelp: `WEB SESSION WORKFLOWS

App-centric review workflows over Apple web-session /iris endpoints.
Use --app to scope operations to one app; sandbox-entitlements takes the
macOS version with --version-id instead.

Subcommands:
  list  List review submissions for an app
  show  Show one submission with threads/messages/rejections and auto-download screenshots
  threads  List every resolution center thread on an app, with optional draft messages
  reply  Send a Resolution Center reply through a web session
  drafts  Create, update, or delete an unsent Resolution Center draft
  subscriptions  Inspect and mutate next-version subscription review selection
  iaps  Attach non-renewing IAPs to the next app version review
  sandbox-entitlements  Manage the App Sandbox Information of a macOS version

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebReviewListCommand(),
			WebReviewShowCommand(),
			WebReviewThreadsCommand(),
			WebReviewReplyCommand(),
			WebReviewDraftsCommand(),
			WebReviewSubscriptionsCommand(),
			WebReviewIAPsCommand(),
			WebReviewSandboxEntitlementsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebReviewReplyCommand sends a Resolution Center reply through the
// Apple web-session API. It creates and sends one draft, then
// verifies the resulting message with the existing read path.
func WebReviewReplyCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web review reply", flag.ExitOnError)

	threadID := fs.String("thread-id", "", "Resolution Center thread ID")
	message := fs.String("message", "", "Reply message body")
	confirm := fs.Bool("confirm", false, "Confirm sending the reply")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "reply",
		ShortUsage: "asc web review reply --thread-id THREAD_ID --message MESSAGE --confirm [flags]",
		ShortHelp:  "Send a Resolution Center reply.",
		LongHelp: `WEB SESSION WORKFLOWS

Send one reply to an App Store Connect Resolution Center thread through the
Apple web-session API. The command creates one draft, sends it,
and re-reads the thread to verify the created message. It never retries a send
automatically because a failed response can follow a successful Apple write.

Attachments are not supported by this command until Apple's attachment write
contract is captured. The message body is never printed in the receipt or
errors.

Examples:
  asc web review reply --thread-id "THREAD_ID" --message "We updated the demo account." --confirm
  asc web review reply --thread-id "THREAD_ID" --message "Please retry." --confirm --output json`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}
			trimmedThreadID := strings.TrimSpace(*threadID)
			if trimmedThreadID == "" {
				return shared.UsageError("--thread-id is required")
			}
			messageBody := *message
			if strings.TrimSpace(messageBody) == "" {
				return shared.UsageError("--message must not be empty")
			}
			if !*confirm {
				return shared.UsageError("--confirm is required")
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return err
			}
			client := newWebClientFn(session)

			var draft *webcore.ResolutionCenterDraftMessage
			err = withWebSpinner("Creating Resolution Center reply draft", func() error {
				var err error
				draft, err = client.CreateResolutionCenterDraftMessage(requestCtx, trimmedThreadID, messageBody)
				return err
			})
			if err != nil {
				return webReviewMutationError(err, "web review reply")
			}
			if draft == nil || strings.TrimSpace(draft.ID) == "" {
				return fmt.Errorf("web review reply failed: draft create returned no draft id; send was not attempted")
			}

			var sent *webcore.ResolutionCenterMessage
			err = withWebSpinner("Sending Resolution Center reply", func() error {
				var err error
				sent, err = client.SendResolutionCenterDraftMessage(requestCtx, draft.ID)
				return err
			})
			if err != nil {
				return fmt.Errorf("web review reply failed: draft %s was created but send outcome is unknown; do not retry automatically: %w", draft.ID, err)
			}
			if sent == nil || strings.TrimSpace(sent.ID) == "" {
				return fmt.Errorf("web review reply failed: send response returned no message id; send outcome may be ambiguous and must not be retried")
			}

			messages, err := client.ListResolutionCenterMessages(requestCtx, trimmedThreadID, false)
			if err != nil {
				return fmt.Errorf("web review reply failed: message %s was sent but post-read verification failed; do not retry automatically: %w", sent.ID, err)
			}
			verified := false
			for _, candidate := range messages {
				if strings.TrimSpace(candidate.ID) == strings.TrimSpace(sent.ID) {
					verified = true
					break
				}
			}
			if !verified {
				return fmt.Errorf("web review reply failed: message %s was sent but post-read did not return it; do not retry automatically", sent.ID)
			}

			result := &asc.WebReviewReplyResult{
				ThreadID:  trimmedThreadID,
				DraftID:   strings.TrimSpace(draft.ID),
				MessageID: strings.TrimSpace(sent.ID),
				Verified:  true,
			}
			if err := shared.PrintOutput(result, *output.Output, *output.Pretty); err != nil {
				return fmt.Errorf("web review reply message %s was sent and verified, but receipt output failed; do not retry automatically: %w", result.MessageID, err)
			}
			return nil
		},
	}
}

// reviewPublicAPIAppArg renders the app for a suggested App Store Connect API
// command, using a placeholder for anything other than a plain numeric app ID.
func reviewPublicAPIAppArg(appID string) string {
	if shared.IsNumericAppID(appID) {
		return appID
	}
	return "APP_ID"
}

// reviewSubmissionsPublicAPIAlternative names the public API command that lists
// an app's review submissions without a web session.
func reviewSubmissionsPublicAPIAlternative(appID string) string {
	return fmt.Sprintf(`Without a web session, 'asc review submissions-list --app "%s"' lists review submissions through the App Store Connect API.`, reviewPublicAPIAppArg(appID))
}

// reviewStatusPublicAPIAlternative names the public API command that reports an
// app's App Review state without a web session; Resolution Center threads and
// messages have no public API equivalent.
func reviewStatusPublicAPIAlternative(appID string) string {
	return fmt.Sprintf(`Without a web session, 'asc review status --app "%s"' reports App Review state through the App Store Connect API; Resolution Center messages need a web session.`, reviewPublicAPIAppArg(appID))
}

// WebReviewListCommand lists review submissions for an app.
func WebReviewListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web review list", flag.ExitOnError)

	appID := fs.String("app", "", "App ID (or ASC_APP_ID env)")
	stateCSV := fs.String("state", "", "Optional comma-separated state filter")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc web review list --app APP_ID [--state CSV] [flags]",
		ShortHelp:  "List app review submissions.",
		FlagSet:    fs,
		UsageFunc:  shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedAppID := strings.TrimSpace(shared.ResolveAppID(*appID))
			if trimmedAppID == "" {
				return shared.UsageError("--app is required (or set ASC_APP_ID)")
			}
			states, err := parseSubmissionStates(*stateCSV)
			if err != nil {
				return err
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(contextWithPublicAPIAlternative(ctx, reviewSubmissionsPublicAPIAlternative(trimmedAppID)), authFlags)
			defer cancel()
			if err != nil {
				return err
			}
			client := webcore.NewClient(session)

			var submissions []webcore.ReviewSubmission
			err = withWebSpinner("Loading review submissions", func() error {
				var err error
				submissions, err = client.ListReviewSubmissions(requestCtx, trimmedAppID)
				return err
			})
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

// WebReviewThreadsCommand lists app-scoped resolution center threads.
func WebReviewThreadsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web review threads", flag.ExitOnError)

	appID := fs.String("app", "", "App ID (or ASC_APP_ID env)")
	drafts := fs.Bool("drafts", false, "Also read each thread's unsent draft message (one extra request per thread)")
	plainText := fs.Bool("plain-text", false, "Project draft messageBody HTML into plain text (requires --drafts)")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "threads",
		ShortUsage: "asc web review threads --app APP_ID [--drafts] [--plain-text] [flags]",
		ShortHelp:  "List every resolution center thread on an app.",
		LongHelp: `WEB SESSION WORKFLOWS

List every resolution center thread App Store Connect keeps for an app,
including binary, metadata, and informational threads that are not attached to
the review submission "asc web review show" selects.

Threads are read from the app-scoped /iris relationship and every page of
links.next is followed. With --drafts, each thread's unsent draft message is
read as well; draft bodies keep Apple's raw HTML, and attachment download URLs
are never returned because this surface is read-only.

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedAppID := strings.TrimSpace(shared.ResolveAppID(*appID))
			if trimmedAppID == "" {
				return shared.WithDiagnostic(
					shared.UsageError("--app is required (or set ASC_APP_ID)"),
					shared.DiagnosticRequiredInputMissing,
					"--app",
				)
			}
			if *plainText && !*drafts {
				return shared.WithDiagnostic(
					shared.UsageError("--plain-text requires --drafts"),
					shared.DiagnosticInvalidInput,
					"--plain-text",
				)
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(contextWithPublicAPIAlternative(ctx, reviewStatusPublicAPIAlternative(trimmedAppID)), authFlags)
			defer cancel()
			if err != nil {
				return err
			}
			client := webcore.NewClient(session)

			var threads []webcore.ResolutionCenterThread
			err = withWebSpinner("Loading app resolution center threads", func() error {
				var err error
				threads, err = client.ListResolutionCenterThreadsByApp(requestCtx, trimmedAppID)
				return err
			})
			if err != nil {
				return withWebAuthHint(err, "web review threads")
			}

			entries := make([]reviewThreadEntry, 0, len(threads))
			for _, thread := range threads {
				entries = append(entries, reviewThreadEntry{Thread: thread})
			}
			if *drafts {
				draftCtx, cancelDrafts := reviewDraftMessagesContext(ctx)
				err = withWebSpinner("Loading resolution center draft messages", func() error {
					for index := range entries {
						draft, err := client.GetResolutionCenterDraftMessage(draftCtx, entries[index].Thread.ID, *plainText)
						if err != nil {
							return err
						}
						entries[index].DraftMessage = draft
					}
					return nil
				})
				cancelDrafts()
				if err != nil {
					return withWebAuthHint(err, "web review threads")
				}
			}

			return shared.PrintOutputWithRenderers(
				entries,
				*output.Output,
				*output.Pretty,
				func() error { return renderReviewThreadsTable(entries, *drafts) },
				func() error { return renderReviewThreadsMarkdown(entries, *drafts) },
			)
		},
	}
}

// WebReviewShowCommand shows a submission with full review context and downloads screenshots.
func WebReviewShowCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web review show", flag.ExitOnError)

	appID := fs.String("app", "", "App ID (or ASC_APP_ID env)")
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
		ShortHelp:  "Show review details and auto-download screenshots.",
		LongHelp: `WEB SESSION WORKFLOWS

Show one submission's review context (threads, messages, rejections) and
auto-download available screenshots/attachments in the same command.

Selection:
  - --submission ID          Use an explicit submission
  - without --submission     Pick latest UNRESOLVED_ISSUES submission, otherwise latest submission

`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedAppID := strings.TrimSpace(shared.ResolveAppID(*appID))
			if trimmedAppID == "" {
				return shared.WithDiagnostic(
					shared.UsageError("--app is required (or set ASC_APP_ID)"),
					shared.DiagnosticRequiredInputMissing,
					"--app",
				)
			}
			trimmedPattern := strings.TrimSpace(*pattern)
			if trimmedPattern != "" {
				if _, err := filepath.Match(trimmedPattern, "sample.png"); err != nil {
					return shared.WithDiagnostic(
						shared.UsageErrorf("--pattern is invalid: %v", err),
						shared.DiagnosticInvalidInput,
						"--pattern",
					)
				}
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(contextWithPublicAPIAlternative(ctx, reviewStatusPublicAPIAlternative(trimmedAppID)), authFlags)
			defer cancel()
			if err != nil {
				return err
			}
			client := webcore.NewClient(session)

			var submissions []webcore.ReviewSubmission
			err = withWebSpinner("Loading review submissions", func() error {
				var err error
				submissions, err = client.ListReviewSubmissions(requestCtx, trimmedAppID)
				return err
			})
			if err != nil {
				return withWebAuthHint(err, "web review show")
			}
			selectedSubmission, selection, err := chooseSubmissionForShow(submissions, *submissionID)
			if err != nil {
				return err
			}
			if selectedSubmission == nil {
				appThreadsCtx, cancelAppThreads := reviewAppThreadsContext(ctx)
				appThreads, appThreadsWarning, err := loadAppThreads(appThreadsCtx, client, trimmedAppID)
				cancelAppThreads()
				if err != nil {
					return withWebAuthHint(err, "web review show")
				}
				payload := reviewShowOutput{
					AppID:             trimmedAppID,
					Selection:         selection,
					AppThreads:        appThreads,
					AppThreadsWarning: appThreadsWarning,
				}
				return shared.PrintOutputWithRenderers(
					payload,
					*output.Output,
					*output.Pretty,
					func() error { return renderReviewShowTable(payload) },
					func() error { return renderReviewShowMarkdown(payload) },
				)
			}

			var (
				items              []webcore.ReviewSubmissionItem
				threadDetails      []reviewThreadDetails
				attachmentsWithURL []webcore.ReviewAttachment
			)
			err = withWebSpinner("Loading review details and attachments", func() error {
				var err error
				items, err = client.ListReviewSubmissionItems(requestCtx, selectedSubmission.ID)
				if err != nil {
					return err
				}
				threads, err := client.ListResolutionCenterThreadsBySubmission(requestCtx, selectedSubmission.ID)
				if err != nil {
					return err
				}
				threadDetails, attachmentsWithURL, err = buildThreadDetails(requestCtx, client, threads, *plainText)
				return err
			})
			if err != nil {
				return withWebAuthHint(err, "web review show")
			}

			outDirResolved := resolveShowOutDir(trimmedAppID, selectedSubmission.ID, *outDir)
			var (
				downloads        []reviewAttachmentDownloadResult
				downloadFailures []string
			)
			err = withWebSpinner("Downloading review attachments", func() error {
				var err error
				downloads, downloadFailures, err = downloadAttachmentsForShow(
					requestCtx,
					client,
					attachmentsWithURL,
					selectedSubmission.ID,
					outDirResolved,
					trimmedPattern,
					*overwrite,
				)
				return err
			})
			if err != nil {
				return err
			}

			appThreadsCtx, cancelAppThreads := reviewAppThreadsContext(ctx)
			appThreads, appThreadsWarning, err := loadAppThreads(appThreadsCtx, client, trimmedAppID)
			cancelAppThreads()
			if err != nil {
				return withWebAuthHint(err, "web review show")
			}

			payload := reviewShowOutput{
				AppID:             trimmedAppID,
				Selection:         selection,
				Submission:        selectedSubmission,
				SubmissionItems:   items,
				Threads:           threadDetails,
				AppThreads:        appThreadsOutsideSubmission(appThreads, threadDetails),
				AppThreadsWarning: appThreadsWarning,
				Attachments:       redactAttachmentURLs(attachmentsWithURL),
				OutputDirectory:   outDirResolved,
				Downloads:         downloads,
				DownloadFailures:  downloadFailures,
			}
			if len(payload.Downloads) == 0 {
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
			if len(payload.DownloadFailures) > 0 {
				return fmt.Errorf("review show completed with %d download failure(s)", len(payload.DownloadFailures))
			}
			return nil
		},
	}
}

func webReviewMutationError(err error, operation string) error {
	hinted := withWebAuthHint(err, operation)
	var apiErr *webcore.APIError
	if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 && apiErr.Status != http.StatusRequestTimeout {
		return hinted
	}
	return fmt.Errorf("%w; mutation outcome may be unknown and may already have been applied; do not retry automatically", hinted)
}
