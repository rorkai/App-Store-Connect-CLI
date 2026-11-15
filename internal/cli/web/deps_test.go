package web

import (
	"bytes"
	"regexp"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func newWebTestDeps() (*shared.Deps, *bytes.Buffer, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer
	return &shared.Deps{
		IO:      &shared.IO{Out: &stdout, ErrOut: &stderr},
		Now:     time.Now,
		Timeout: shared.ContextWithTimeout,
		After:   time.After,
	}, &stdout, &stderr
}

// runWithWebTestDeps runs fn and returns what it wrote to the deps streams.
func runWithWebTestDeps(stdout, stderr *bytes.Buffer, fn func()) (string, string) {
	fn()
	return stdout.String(), stderr.String()
}

// newWebSpinnerTestDeps returns deps whose stdout and stderr are terminals and
// whose spinner delays have already elapsed, so each web spinner draws its
// label on stderr. Run the command inside synctest.Test and let the work under
// each spinner call synctest.Wait before it returns.
func newWebSpinnerTestDeps(t *testing.T) (*shared.Deps, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	t.Setenv("ASC_SPINNER_DISABLED", "")
	d, stdout, stderr := newWebTestDeps()
	d.IO.StdoutTTY, d.IO.StderrTTY = true, true
	d.After = func(time.Duration) <-chan time.Time {
		fired := make(chan time.Time, 1)
		fired <- time.Time{}
		return fired
	}
	return d, stdout, stderr
}

var spinnerLabelPattern = regexp.MustCompile("\r⣾ (.+?) *\r")

// spinnerLabels lists the labels the spinner drew on stderr, in order.
func spinnerLabels(stderr string) []string {
	labels := []string{}
	for _, match := range spinnerLabelPattern.FindAllStringSubmatch(stderr, -1) {
		labels = append(labels, match[1])
	}
	return labels
}
