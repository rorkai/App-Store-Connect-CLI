package assets

import (
	"errors"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestDuplicateScreenshotWarningSanitizesTerminal(t *testing.T) {
	for _, name := range []string{"home.png", "\x1b[2Jhome.png", "\x1b]0;changed-title\x07home.png", "home\rhidden.png", "home\nforged-diagnostic.png"} {
		t.Run(name, func(t *testing.T) {
			d, stdoutBuf, stderrBuf := newAssetsTestDeps(nil)
			warnScreenshotFileNamesAlreadyInSet(d, []string{"dir/" + name}, []asc.Resource[asc.AppScreenshotAttributes]{
				{Attributes: asc.AppScreenshotAttributes{FileName: name}},
			})
			stdout, stderr := stdoutBuf.String(), stderrBuf.String()
			if stdout != "" || !strings.Contains(stderr, "1 file(s) already in the target screenshot set") || !strings.Contains(stderr, "--skip-existing") {
				t.Fatalf("duplicate warning lost: stdout=%q stderr=%q", stdout, stderr)
			}
			if strings.ContainsAny(stderr, "\x1b\x07\r") || strings.Count(stderr, "\n") != 1 {
				t.Fatalf("terminal controls survived duplicate warning: %q", stderr)
			}
		})
	}
}

func TestScreenshotRetryHintSanitizesTerminal(t *testing.T) {
	cause := errors.New("upload failed")
	d, stdoutBuf, stderrBuf := newAssetsTestDeps(nil)
	err := screenshotUploadRetryError(d,
		asc.AppScreenshotUploadResult{FailureArtifactPath: "reports/\x1b]0;changed-title\x07broken\nreport.json", Total: 1, Pending: 1},
		screenshotUploadProgress{PendingFiles: []string{"home.png"}}, false, cause,
	)
	if !errors.Is(err, cause) {
		t.Fatalf("upload failure cause lost: %v", err)
	}
	stdout, stderr := stdoutBuf.String(), stderrBuf.String()
	if stdout != "" || !strings.Contains(stderr, "upload failed") || !strings.Contains(stderr, "--resume") {
		t.Fatalf("retry diagnostic lost: stdout=%q stderr=%q", stdout, stderr)
	}
	if strings.ContainsAny(stderr, "\x1b\x07\r") || strings.Count(stderr, "\n") != 2 {
		t.Fatalf("terminal controls survived retry hint: %q", stderr)
	}
}
