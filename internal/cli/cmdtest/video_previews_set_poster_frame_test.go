package cmdtest

import (
	"io"
	"strings"
	"testing"
)

func TestVideoPreviewsSetPosterFrameMissingID(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		_ = root.Parse([]string{"video-previews", "set-poster-frame", "--time-code", "00:00:01:00"})
		_ = root.Run(t.Context())
	})
	if !strings.Contains(stderr, "--id is required") {
		t.Fatalf("expected --id is required error, got stderr: %s", stderr)
	}
}

func TestVideoPreviewsSetPosterFrameMissingTimeCode(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		_ = root.Parse([]string{"video-previews", "set-poster-frame", "--id", "PREVIEW_123"})
		_ = root.Run(t.Context())
	})
	if !strings.Contains(stderr, "--time-code is required") {
		t.Fatalf("expected --time-code is required error, got stderr: %s", stderr)
	}
}

func TestVideoPreviewsSetPosterFrameInvalidTimeCode(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		_ = root.Parse([]string{"video-previews", "set-poster-frame", "--id", "PREVIEW_123", "--time-code", "abc"})
		_ = root.Run(t.Context())
	})
	if !strings.Contains(stderr, "HH:MM:SS:FF or HH:MM:SS.mmm") {
		t.Fatalf("expected timecode format error, got stderr: %s", stderr)
	}
}

func TestVideoPreviewsSetPosterFrameRejectsPositionalArgs(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		_ = root.Parse([]string{"video-previews", "set-poster-frame", "--id", "PREVIEW_123", "--time-code", "00:00:01:00", "extra-arg"})
		_ = root.Run(t.Context())
	})
	if !strings.Contains(stderr, "does not accept positional arguments") {
		t.Fatalf("expected positional arguments error, got stderr: %s", stderr)
	}
}
