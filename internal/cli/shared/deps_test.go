package shared

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestDepsBindOutputFlagsDefaultFollowsStdoutTTY(t *testing.T) {
	resetDefaultOutput(t)
	clearCIEnvironment(t)
	t.Setenv("ASC_DEFAULT_OUTPUT", "")
	os.Unsetenv("ASC_DEFAULT_OUTPUT")

	for _, tt := range []struct {
		stdoutTTY bool
		want      string
	}{
		{stdoutTTY: true, want: "table"},
		{stdoutTTY: false, want: "json"},
	} {
		d := &Deps{IO: &IO{StdoutTTY: tt.stdoutTTY}}
		output := d.BindOutputFlags(flag.NewFlagSet("list", flag.ContinueOnError))
		if *output.Output != tt.want {
			t.Fatalf("StdoutTTY=%t: default output = %q, want %q", tt.stdoutTTY, *output.Output, tt.want)
		}
	}
}

func TestDepsBindOutputFlagsReportsInvalidDefaultOnDepsStderr(t *testing.T) {
	resetDefaultOutput(t)
	t.Setenv("ASC_DEFAULT_OUTPUT", "xml")

	var stderr bytes.Buffer
	d := &Deps{IO: &IO{ErrOut: &stderr, StdoutTTY: true}}
	var output OutputFlags
	processStdout, processStderr := captureOutput(t, func() {
		output = d.BindOutputFlags(flag.NewFlagSet("list", flag.ContinueOnError))
	})

	if processStdout != "" || processStderr != "" {
		t.Fatalf("process stdout = %q, stderr = %q, want both empty", processStdout, processStderr)
	}
	if got, want := stderr.String(), "Warning: invalid ASC_DEFAULT_OUTPUT value \"xml\" (expected json, table, markdown, or md); using json\n"; got != want {
		t.Fatalf("deps stderr = %q, want %q", got, want)
	}
	if *output.Output != "json" {
		t.Fatalf("default output = %q, want json", *output.Output)
	}
}

func TestDepsPrintOutputMatchesPrintOutputOnDepsStreams(t *testing.T) {
	var page asc.ActorsResponse
	if err := json.Unmarshal([]byte(`{
		"data":[{"type":"actors","id":"actor-1","attributes":{"actorType":"USER","userEmail":"a@example.com"}}],
		"links":{"self":"https://api.appstoreconnect.apple.com/v1/actors","next":"https://api.appstoreconnect.apple.com/v1/actors?cursor=abc"}
	}`), &page); err != nil {
		t.Fatalf("unmarshal page: %v", err)
	}

	for _, tt := range []struct {
		format string
		pretty bool
	}{
		{format: "json"},
		{format: "json", pretty: true},
		{format: "table"},
		{format: "markdown"},
	} {
		t.Run(tt.format, func(t *testing.T) {
			wantStdout, wantStderr := captureOutput(t, func() {
				if err := PrintOutput(&page, tt.format, tt.pretty); err != nil {
					t.Errorf("PrintOutput() error = %v", err)
				}
			})

			var stdout, stderr bytes.Buffer
			d := &Deps{IO: &IO{Out: &stdout, ErrOut: &stderr}}
			processStdout, processStderr := captureOutput(t, func() {
				if err := d.PrintOutput(&page, tt.format, tt.pretty); err != nil {
					t.Errorf("Deps.PrintOutput() error = %v", err)
				}
			})

			if processStdout != "" || processStderr != "" {
				t.Fatalf("process stdout = %q, stderr = %q, want both empty", processStdout, processStderr)
			}
			if stdout.String() != wantStdout {
				t.Fatalf("deps stdout = %q, want %q", stdout.String(), wantStdout)
			}
			if stderr.String() != wantStderr {
				t.Fatalf("deps stderr = %q, want %q", stderr.String(), wantStderr)
			}
			if wantStdout == "" || wantStderr == "" {
				t.Fatalf("expected PrintOutput to write a page and a more-pages warning, got stdout %q, stderr %q", wantStdout, wantStderr)
			}
		})
	}
}
