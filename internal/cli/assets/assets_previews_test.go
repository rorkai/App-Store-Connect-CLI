package assets

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
)

func TestAssetsPreviewsUploadCommandRejectsSkipExistingWithReplace(t *testing.T) {
	cmd := AssetsPreviewsUploadCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--version-localization", "LOC_ID",
		"--path", "./previews",
		"--device-type", "IPHONE_65",
		"--skip-existing",
		"--replace",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		runErr = cmd.Exec(context.Background(), cmd.FlagSet.Args())
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "--skip-existing and --replace are mutually exclusive") {
		t.Fatalf("expected mutually exclusive error in stderr, got %q", stderr)
	}
}

func TestIsValidPreviewFrameTimeCode(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "frame format", value: "00:00:05:00", want: true},
		{name: "millisecond format", value: "00:00:05.000", want: true},
		{name: "frame upper bound", value: "99:59:59:29", want: true},
		{name: "non numeric", value: "abc", want: false},
		{name: "missing component", value: "00:00:05", want: false},
		{name: "invalid minute", value: "00:60:05:00", want: false},
		{name: "invalid second", value: "00:00:60.000", want: false},
		{name: "invalid frame", value: "00:00:05:30", want: false},
		{name: "invalid millisecond width", value: "00:00:05.00", want: false},
		{name: "invalid separator", value: "00-00-05-00", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidPreviewFrameTimeCode(tt.value); got != tt.want {
				t.Fatalf("isValidPreviewFrameTimeCode(%q) = %t, want %t", tt.value, got, tt.want)
			}
		})
	}
}
