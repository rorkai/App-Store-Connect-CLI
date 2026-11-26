package web

import (
	"path/filepath"
	"strings"
	"testing"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestNormalizeAttachmentFilenameStripsPathComponents(t *testing.T) {
	attachment := webcore.ReviewAttachment{
		AttachmentID: "attachment-id",
		FileName:     "../../etc/passwd",
	}

	got := normalizeAttachmentFilename(attachment)
	if got != "passwd" {
		t.Fatalf("expected sanitized filename %q, got %q", "passwd", got)
	}
}

func TestNormalizeAttachmentFilenameFallsBackWhenBasenameIsInvalid(t *testing.T) {
	attachment := webcore.ReviewAttachment{
		AttachmentID: "attachment-id",
		FileName:     "../",
	}

	got := normalizeAttachmentFilename(attachment)
	if got != "attachment-id.bin" {
		t.Fatalf("expected fallback filename %q, got %q", "attachment-id.bin", got)
	}
}

func TestNormalizeAttachmentFilenameSanitizesFallbackAttachmentID(t *testing.T) {
	attachment := webcore.ReviewAttachment{
		AttachmentID: "../../nested/path",
		FileName:     "../",
	}

	got := normalizeAttachmentFilename(attachment)
	if filepath.Base(got) != got {
		t.Fatalf("expected basename-only filename, got %q", got)
	}
	if strings.Contains(got, "..") || strings.Contains(got, "/") || strings.Contains(got, "\\") {
		t.Fatalf("expected sanitized fallback filename, got %q", got)
	}
}

func TestResolveDownloadPathRejectsEscapingOutDir(t *testing.T) {
	d, _, _ := newWebTestDeps()
	root, prefix, err := newDownloadRootWith(d, t.TempDir())
	if err != nil {
		t.Fatalf("newDownloadRootWith() error: %v", err)
	}
	_, err = resolveDownloadPath(root, prefix, "../outside.txt", true)
	if err == nil {
		t.Fatal("expected path escape error")
	}
	if !strings.Contains(err.Error(), "escapes trusted root") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveShowOutDirSanitizesDotDotPathPart(t *testing.T) {
	got := resolveShowOutDir("..", "submission-1", "")
	want := filepath.Join(".asc", "web-review", "unknown", "submission-1")
	if got != want {
		t.Fatalf("expected resolved path %q, got %q", want, got)
	}
}

func TestResolveShowOutDirPreservesWhitespacePathBytes(t *testing.T) {
	want := filepath.Join(t.TempDir(), " downloads ")
	if got := resolveShowOutDir("app", "submission", want); got != want {
		t.Fatalf("resolveShowOutDir() = %q, want %q", got, want)
	}
}

func TestBuildReviewListTableRows(t *testing.T) {
	submissions := []webcore.ReviewSubmission{
		{
			ID:            "sub-1",
			State:         "UNRESOLVED_ISSUES",
			SubmittedDate: "2026-02-25T10:00:00Z",
			Platform:      "IOS",
			AppStoreVersionForReview: &webcore.AppStoreVersionForReview{
				ID:      "ver-1",
				Version: "1.2.3",
			},
		},
		{
			ID: "sub-2",
		},
	}

	rows := buildReviewListTableRows(submissions)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if got := rows[0][0]; got != "sub-1" {
		t.Fatalf("expected first row submission id %q, got %q", "sub-1", got)
	}
	if got := rows[0][3]; got != "1.2.3" {
		t.Fatalf("expected first row version %q, got %q", "1.2.3", got)
	}
	if got := rows[1][1]; got != "n/a" {
		t.Fatalf("expected fallback state %q, got %q", "n/a", got)
	}
	if got := rows[1][3]; got != "n/a" {
		t.Fatalf("expected fallback version %q, got %q", "n/a", got)
	}
}

func TestBuildReviewShowTableRowsIncludesExpectedSections(t *testing.T) {
	payload := reviewShowOutput{
		AppID:     "6567933550",
		Selection: "explicit",
		Submission: &webcore.ReviewSubmission{
			ID:            "submission-1",
			State:         "UNRESOLVED_ISSUES",
			SubmittedDate: "2026-02-24T08:45:26.513Z",
			Platform:      "IOS",
			AppStoreVersionForReview: &webcore.AppStoreVersionForReview{
				ID:       "version-1",
				Version:  "2.2.1",
				Platform: "IOS",
			},
		},
		SubmissionItems: []webcore.ReviewSubmissionItem{
			{
				ID:   "item-1",
				Type: "reviewSubmissionItems",
				Related: []webcore.ReviewSubmissionItemRelation{
					{
						Relationship: "appStoreVersion",
						Type:         "appStoreVersions",
						ID:           "version-1",
						Label:        "2.2.1",
					},
				},
			},
		},
		Threads: []reviewThreadDetails{
			{
				Thread: webcore.ResolutionCenterThread{
					ID:         "thread-1",
					ThreadType: "REJECTION_REVIEW_SUBMISSION",
					State:      "OPEN",
				},
				Messages: []webcore.ResolutionCenterMessage{
					{
						ID:          "message-1",
						CreatedDate: "2026-02-24T08:45:26.513Z",
						MessageBody: "<b>Hello</b><br>Issue details",
						FromActor: &webcore.ReviewActor{
							ID:        "APPLE",
							ActorType: "APPLE",
						},
					},
				},
				Rejections: []webcore.ReviewRejection{
					{
						ID: "rejection-1",
						Reasons: []webcore.ReviewRejectionReason{
							{
								ReasonCode:        "2.1.0",
								ReasonSection:     "2.1",
								ReasonDescription: "Performance: App Completeness",
							},
						},
						Related: []webcore.ReviewRelatedResource{
							{
								Relationship: "appStoreVersion",
								Type:         "appStoreVersions",
								ID:           "version-1",
								Label:        "2.2.1",
							},
							{
								Relationship: "build",
								Type:         "builds",
								ID:           "build-1",
								Label:        "88",
							},
						},
					},
				},
			},
		},
		Attachments: []webcore.ReviewAttachment{
			{
				AttachmentID: "attachment-1",
				FileName:     "Screenshot-1.png",
				FileSize:     123,
				Downloadable: true,
			},
		},
		Downloads: []reviewAttachmentDownloadResult{
			{
				AttachmentID: "attachment-1",
				FileName:     "Screenshot-1.png",
				Path:         ".asc/web-review/6567933550/submission-1/Screenshot-1.png",
			},
		},
	}

	rows := buildReviewShowTableRows(payload)
	if len(rows) == 0 {
		t.Fatal("expected non-empty rows")
	}

	assertRowContains(t, rows, "Submission", "Review Status", "UNRESOLVED_ISSUES")
	assertRowContains(t, rows, "Items Reviewed", "Item 1", "appStoreVersion")
	assertRowContains(t, rows, "Items Reviewed", "Item 1", "2.2.1")
	assertRowContains(t, rows, "Rejections", "Reason 1", "2.2.1")
	assertRowContains(t, rows, "Rejections", "Reason 1", "88")
	assertRowContains(t, rows, "Rejections", "Reason 1", "code=2.1.0 section=2.1 description=Performance: App Completeness")
	assertRowEquals(t, rows, "Messages", "Message 1", "Hello Issue details")
	assertRowContains(t, rows, "Screenshots", "Attachment 1", "Screenshot-1.png")
	assertRowContains(t, rows, "Downloads", "Downloaded 1", ".asc/web-review/6567933550/submission-1/Screenshot-1.png")
}

func TestRenderReviewShowMarkdownPrintsMessageTextBelowTable(t *testing.T) {
	d, stdout, _ := newWebTestDeps()
	payload := reviewShowOutput{
		AppID: "6567933550",
		Threads: []reviewThreadDetails{{
			Thread: webcore.ResolutionCenterThread{ID: "thread-1"},
			Messages: []webcore.ResolutionCenterMessage{{
				ID:          "message-1",
				CreatedDate: "2026-02-24T08:45:26.513Z",
				MessageBody: "Hello,<br><br>The app uses an entitlement.<br><br><h3>Guideline 2.4.5</h3>Describe how the app uses it.",
				FromActor:   &webcore.ReviewActor{ID: "APPLE", ActorType: "APPLE"},
			}},
		}},
	}

	if err := renderReviewShowMarkdown(d, payload); err != nil {
		t.Fatalf("renderReviewShowMarkdown() error = %v", err)
	}

	out := stdout.String()
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "|") && strings.Contains(line, "entitlement") {
			t.Fatalf("message text belongs below the table, found in table row %q", line)
		}
	}
	if !strings.Contains(out, "| Messages") || !strings.Contains(out, "APPLE") {
		t.Fatalf("expected the message row to name its sender, got:\n%s", out)
	}
	want := "> Hello,\n>\n> The app uses an entitlement.\n>\n> Guideline 2.4.5\n> Describe how the app uses it.\n"
	if !strings.Contains(out, want) {
		t.Fatalf("expected message text with its paragraphs below the table:\n%s\ngot:\n%s", want, out)
	}
}

func TestSummarizeMessageForTableStripsHTMLAndLinks(t *testing.T) {
	message := webcore.ResolutionCenterMessage{
		MessageBody: `Hello,<br><b>Review</b> message. See <a href="https://developer.apple.com/documentation/xcode/testing-a-release-build">Testing a Release Build</a>.`,
	}

	got := summarizeMessageForTable(message)
	if strings.Contains(got, "<a ") || strings.Contains(got, "</a>") || strings.Contains(got, "<b>") {
		t.Fatalf("expected markup to be removed, got %q", got)
	}
	if !strings.Contains(got, "Testing a Release Build") {
		t.Fatalf("expected link text to remain, got %q", got)
	}
}

func TestSummarizeHTMLBodyForTablePreservesPlainAngleBrackets(t *testing.T) {
	got := summarizeHTMLBodyForTable("a < b > c", "<p>a &lt; b &gt; c</p>")
	if got != "a < b > c" {
		t.Fatalf("plain text with comparison operators = %q, want %q", got, "a < b > c")
	}

	got = summarizeHTMLBodyForTable("", "<p>a &lt; b &gt; c</p>")
	if got != "a < b > c" {
		t.Fatalf("raw HTML fallback = %q, want %q", got, "a < b > c")
	}
}

func assertRowContains(t *testing.T, rows [][]string, section, field, value string) {
	t.Helper()
	for _, row := range rows {
		if len(row) < 3 {
			continue
		}
		if row[0] == section && strings.Contains(row[1], field) && strings.Contains(row[2], value) {
			return
		}
	}
	t.Fatalf("expected row section=%q field contains %q value contains %q", section, field, value)
}

func assertRowEquals(t *testing.T, rows [][]string, section, field, value string) {
	t.Helper()
	for _, row := range rows {
		if len(row) < 3 {
			continue
		}
		if row[0] == section && row[1] == field && row[2] == value {
			return
		}
	}
	t.Fatalf("expected row section=%q field=%q value=%q", section, field, value)
}
