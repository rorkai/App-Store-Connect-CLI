package publish

import (
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestReportSkippedInternalAllBuildsGroupsSanitizesProviderID(t *testing.T) {
	const groupID = "group-control-\x1b[31mID\nNEXT"

	_, stderr := capturePublishCommandOutput(t, func() error {
		reportSkippedInternalAllBuildsGroups([]shared.ResolvedBetaGroup{{
			ID:   groupID,
			Name: "QA-\x1b[31mNAME\nINJECT",
		}})
		return nil
	})

	want := `Skipped internal group "QA-\x1b[31mNAME\nINJECT" (group-control-[31mID NEXT) because it already receives all builds` + "\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
	if strings.Contains(stderr, "\x1b") || strings.Contains(stderr, groupID) {
		t.Fatalf("stderr contains unsanitized provider ID: %q", stderr)
	}
}
