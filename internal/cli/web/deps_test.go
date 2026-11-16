package web

import (
	"bytes"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func newWebTestDeps() (*shared.Deps, *bytes.Buffer, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer
	return &shared.Deps{
		IO:             &shared.IO{Out: &stdout, ErrOut: &stderr},
		Now:            time.Now,
		Timeout:        shared.ContextWithTimeout,
		ResolveTimeout: shared.ResolveTimeoutSetting,
		After:          time.After,
		Getwd:          os.Getwd,
		LookupEnv:      os.LookupEnv,
		HTTPTransport:  func() http.RoundTripper { return http.DefaultTransport },
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
// each spinner wait for it with spinnerShownTransport.
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

// spinnerShownTransport answers through rt once every other goroutine in the
// synctest bubble is blocked, which is after the spinner has drawn its label.
func spinnerShownTransport(rt http.RoundTripper) roundTripFunc {
	return roundTripFunc(func(req *http.Request) (*http.Response, error) {
		synctest.Wait()
		return rt.RoundTrip(req)
	})
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

// runWithWebTestDepsStderr runs fn and returns what it wrote to the deps
// stderr, with CRLF line endings normalized as captureWebDiagnosticStderr does.
func runWithWebTestDepsStderr(stderr *bytes.Buffer, fn func()) string {
	fn()
	return strings.ReplaceAll(stderr.String(), "\r\n", "\n")
}

var spinnerDrawingPattern = regexp.MustCompile("\r[\u28fe\u28fd\u28fb\u28bf\u287f\u28df\u28ef\u28f7] [^\r]*|\r +\r")

// withoutSpinner removes what the spinner drew from stderr.
func withoutSpinner(stderr string) string {
	return spinnerDrawingPattern.ReplaceAllString(stderr, "")
}

// captureWebTestDeps runs fn with d's buffers emptied first and returns what it
// wrote to them.
func captureWebTestDeps(d *shared.Deps, fn func()) (string, string) {
	stdout, stderr := d.IO.Out.(*bytes.Buffer), d.IO.ErrOut.(*bytes.Buffer)
	stdout.Reset()
	stderr.Reset()
	fn()
	return stdout.String(), stderr.String()
}

// runWithWebSpinnerTestDeps runs fn in a synctest bubble on deps from
// newWebSpinnerTestDeps and returns stdout and stderr without the spinner's
// drawing.
func runWithWebSpinnerTestDeps(t *testing.T, stdout, stderr *bytes.Buffer, fn func(t *testing.T)) (string, string) {
	t.Helper()
	synctest.Test(t, fn)
	return stdout.String(), withoutSpinner(stderr.String())
}
