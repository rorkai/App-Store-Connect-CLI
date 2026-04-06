package metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMetadataKeywordsAuditCommandHelpMentionsBlockedTerms(t *testing.T) {
	cmd := MetadataKeywordsAuditCommand()
	if cmd == nil {
		t.Fatal("expected audit command")
	}
	if !strings.Contains(cmd.LongHelp, "--app-info") {
		t.Fatalf("expected long help to mention --app-info, got %q", cmd.LongHelp)
	}
	if !strings.Contains(cmd.LongHelp, "--blocked-term") {
		t.Fatalf("expected long help to mention --blocked-term, got %q", cmd.LongHelp)
	}
	if !strings.Contains(cmd.LongHelp, "--blocked-terms-file") {
		t.Fatalf("expected long help to mention --blocked-terms-file, got %q", cmd.LongHelp)
	}
}

func TestLoadKeywordAuditBlockedTermsTrimsCommentsAndCommaSeparatedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "blocked-terms.txt")
	if err := os.WriteFile(path, []byte("free,premium\n# comment\nsale\nPremium\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	terms, err := loadKeywordAuditBlockedTerms([]string{"free", "  review bomb  "}, path)
	if err != nil {
		t.Fatalf("loadKeywordAuditBlockedTerms() error: %v", err)
	}

	if got := strings.Join(terms, ","); got != "free,review bomb,free,premium,sale,Premium" {
		t.Fatalf("expected trimmed blocked terms in input order, got %q", got)
	}
}
