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
