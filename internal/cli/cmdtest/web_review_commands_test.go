package cmdtest

import (
	"strings"
	"testing"
)

func TestWebReviewCommandsAreRegisteredAndLegacyRemoved(t *testing.T) {
	root := RootCommand("1.2.3")

	for _, path := range [][]string{
		{"web", "review"},
		{"web", "review", "list"},
		{"web", "review", "show"},
		// Re-added by issue #775 with an app-scoped contract: it lists every
		// resolution center thread on an app, not the removed
		// submission-oriented listing.
		{"web", "review", "threads"},
	} {
		if sub := findSubcommand(root, path...); sub == nil {
			t.Fatalf("expected command %q to be registered", strings.Join(path, " "))
		}
	}

	for _, legacyPath := range [][]string{
		{"web", "submissions"},
		{"web", "review", "messages"},
		{"web", "review", "rejections"},
		{"web", "review", "draft"},
		{"web", "review", "attachments"},
	} {
		if sub := findSubcommand(root, legacyPath...); sub != nil {
			t.Fatalf("expected legacy command %q to be removed", strings.Join(legacyPath, " "))
		}
	}
}

func TestWebReviewCommandsRejectInvalidFlagsWithUsageExit(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "list requires app", args: []string{"web", "review", "list"}, wantErr: "--app is required"},
		{name: "list rejects unknown state", args: []string{"web", "review", "list", "--app", "123456789", "--state", "UNRESOLVED_ISSUES,NOT_A_REAL_STATE"}, wantErr: "unsupported value"},
		{name: "show requires app", args: []string{"web", "review", "show"}, wantErr: "--app is required"},
		{name: "show rejects invalid pattern", args: []string{"web", "review", "show", "--app", "123456789", "--pattern", "["}, wantErr: "--pattern is invalid"},
		{name: "threads requires app", args: []string{"web", "review", "threads"}, wantErr: "--app is required"},
		{name: "threads plain text requires drafts", args: []string{"web", "review", "threads", "--plain-text", "--app", "123456789"}, wantErr: "--plain-text requires --drafts"},
		{name: "threads rejects positional arguments", args: []string{"web", "review", "threads", "--app", "123456789", "stray"}, wantErr: "unexpected argument"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertUsageExit(t, tt.args, tt.wantErr)
		})
	}
}
