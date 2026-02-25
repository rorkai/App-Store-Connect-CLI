package web

import (
	"path/filepath"
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

func TestResolveShowOutDirSanitizesDotDotPathPart(t *testing.T) {
	got := resolveShowOutDir("..", "submission-1", "")
	want := filepath.Join(".asc", "web-review", "unknown", "submission-1")
	if got != want {
		t.Fatalf("expected resolved path %q, got %q", want, got)
	}
}
