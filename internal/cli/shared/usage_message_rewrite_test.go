package shared

import (
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"testing"
)

func TestRewriteUsageMessage(t *testing.T) {
	tests := []struct {
		name     string
		install  func(context.Context) context.Context
		message  string
		expected string
	}{
		{
			name:     "no rewrites leaves the message alone",
			install:  func(ctx context.Context) context.Context { return ctx },
			message:  "subscriptions introductory-offers list: --limit must be between 1 and 200",
			expected: "subscriptions introductory-offers list: --limit must be between 1 and 200",
		},
		{
			name: "anchored prefix renames the leading command path",
			install: func(ctx context.Context) context.Context {
				return ContextWithUsageMessageRewrites(ctx, UsageMessageRewrite{
					Old:            "subscriptions introductory-offers list",
					New:            "subscriptions offers introductory list",
					AnchoredPrefix: true,
				})
			},
			message:  "subscriptions introductory-offers list: --limit must be between 1 and 200",
			expected: "subscriptions offers introductory list: --limit must be between 1 and 200",
		},
		{
			name: "anchored prefix ignores a match that is not leading",
			install: func(ctx context.Context) context.Context {
				return ContextWithUsageMessageRewrites(ctx, UsageMessageRewrite{
					Old:            "beta-groups list",
					New:            "groups list",
					AnchoredPrefix: true,
				})
			},
			message:  "--next is not a beta-groups list URL",
			expected: "--next is not a beta-groups list URL",
		},
		{
			name: "unanchored rewrite replaces every occurrence",
			install: func(ctx context.Context) context.Context {
				return ContextWithUsageMessageRewrites(ctx, UsageMessageRewrite{
					Old: "beta-groups",
					New: "groups",
				})
			},
			message:  "beta-groups list: --next must be a beta-groups URL",
			expected: "groups list: --next must be a groups URL",
		},
		{
			name: "an empty or identity rewrite is ignored",
			install: func(ctx context.Context) context.Context {
				return ContextWithUsageMessageRewrites(
					ctx,
					UsageMessageRewrite{Old: "", New: "groups"},
					UsageMessageRewrite{Old: "beta-groups", New: ""},
					UsageMessageRewrite{Old: "beta-groups", New: "beta-groups"},
				)
			},
			message:  "beta-groups list: --limit must be between 1 and 200",
			expected: "beta-groups list: --limit must be between 1 and 200",
		},
		{
			name: "the innermost wrapper's rewrite is applied first",
			install: func(ctx context.Context) context.Context {
				// The outer tree is entered first, so it installs first.
				ctx = ContextWithUsageMessageRewrites(ctx, UsageMessageRewrite{
					Old: "offer-codes one-time-codes",
					New: "offers offer-codes one-time-codes",
				})
				return ContextWithUsageMessageRewrites(ctx, UsageMessageRewrite{
					Old: "one-time-codes list",
					New: "one-time-codes view",
				})
			},
			message:  "offer-codes one-time-codes list: --limit must be between 1 and 200",
			expected: "offers offer-codes one-time-codes view: --limit must be between 1 and 200",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := RewriteUsageMessage(test.install(context.Background()), test.message)
			if got != test.expected {
				t.Fatalf("RewriteUsageMessage() = %q, want %q", got, test.expected)
			}
		})
	}
}

func TestRewriteUsageMessageToleratesNilContext(t *testing.T) {
	//nolint:staticcheck // SA1012: a nil context must not panic here.
	if got := RewriteUsageMessage(nil, "webhooks list: --limit must be between 1 and 200"); got != "webhooks list: --limit must be between 1 and 200" {
		t.Fatalf("RewriteUsageMessage(nil) = %q, want the message unchanged", got)
	}
	//nolint:staticcheck // SA1012: a nil context must not panic here.
	if ctx := ContextWithUsageMessageRewrites(nil, UsageMessageRewrite{Old: "a", New: "b"}); ctx != nil {
		t.Fatalf("ContextWithUsageMessageRewrites(nil) = %v, want nil", ctx)
	}
}

// TestUsageErrorCtxAppliesRewritesBeforeReporting locks the reason the context
// carries these rewrites at all: UsageError writes to stderr while the command
// is still running, so the printed diagnostic must already name the rewritten
// command path.
func TestUsageErrorCtxAppliesRewritesBeforeReporting(t *testing.T) {
	ctx := ContextWithUsageMessageRewrites(context.Background(), UsageMessageRewrite{
		Old:            "beta-groups list",
		New:            "groups list",
		AnchoredPrefix: true,
	})

	var err error
	stderr := captureUsageRewriteStderr(t, func() {
		err = DefaultDeps().UsageErrorfCtx(ctx, "beta-groups list: --limit must be between 1 and %d", 200)
	})

	const want = "groups list: --limit must be between 1 and 200"
	if stderr != "Error: "+want+"\n" {
		t.Fatalf("stderr = %q, want %q", stderr, "Error: "+want+"\n")
	}
	if err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("errors.Is(flag.ErrHelp) = false, want usage exit semantics: %v", err)
	}
}

func captureUsageRewriteStderr(t *testing.T, run func()) string {
	t.Helper()

	original := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}
	readResult := make(chan []byte, 1)
	readError := make(chan error, 1)
	go func() {
		data, readErr := io.ReadAll(reader)
		readResult <- data
		readError <- readErr
	}()

	os.Stderr = writer
	run()
	if err := writer.Close(); err != nil {
		t.Fatalf("close stderr writer: %v", err)
	}
	os.Stderr = original

	data := <-readResult
	if err := <-readError; err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	return string(data)
}
