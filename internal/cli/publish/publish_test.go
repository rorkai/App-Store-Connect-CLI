package publish

import (
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestReportSkippedInternalAllBuildsGroupsSanitizesProviderID(t *testing.T) {
	const groupID = "group-control-\x1b[31mID\nNEXT"

	d := newPublishTestDeps()
	_, stderr := capturePublishDepsOutput(t, d, func() error {
		reportSkippedInternalAllBuildsGroups(d, []shared.ResolvedBetaGroup{{
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
