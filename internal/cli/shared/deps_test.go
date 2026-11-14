package shared

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"reflect"
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

func TestDepsUsageErrorsMatchProcessVariantsOnDepsStderr(t *testing.T) {
	const next = "https://api.appstoreconnect.apple.com/v1/actors?cursor=AQ"
	nextWithLimit := func() *flag.FlagSet {
		fs := flag.NewFlagSet("list", flag.ContinueOnError)
		fs.Int("limit", 0, "")
		if err := fs.Parse([]string{"--limit", "5"}); err != nil {
			t.Fatalf("parse --limit: %v", err)
		}
		return fs
	}
	tests := []struct {
		name    string
		process func() error
		deps    func(*Deps) error
	}{
		{
			name:    "UsageError",
			process: func() error { return UsageError("actors list: --limit must be between 1 and 200") },
			deps:    func(d *Deps) error { return d.UsageError("actors list: --limit must be between 1 and 200") },
		},
		{
			name:    "UsageErrorf",
			process: func() error { return UsageErrorf("%s: --limit must be between 1 and %d", "actors view", 200) },
			deps:    func(d *Deps) error { return d.UsageErrorf("%s: --limit must be between 1 and %d", "actors view", 200) },
		},
		{
			name:    "RequiredFlag",
			process: func() error { return RequiredFlag("--id") },
			deps:    func(d *Deps) error { return d.RequiredFlag("--id") },
		},
		{
			name:    "RejectNextFlagConflicts",
			process: func() error { return RejectNextFlagConflicts(nextWithLimit(), next, "actors list", "limit") },
			deps:    func(d *Deps) error { return d.RejectNextFlagConflicts(nextWithLimit(), next, "actors list", "limit") },
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var wantErr error
			_, wantStderr := captureOutput(t, func() { wantErr = test.process() })

			var stdout, stderr bytes.Buffer
			d := &Deps{IO: &IO{Out: &stdout, ErrOut: &stderr}}
			var gotErr error
			processStdout, processStderr := captureOutput(t, func() { gotErr = test.deps(d) })

			if processStdout != "" || processStderr != "" {
				t.Fatalf("process stdout = %q, stderr = %q, want both empty", processStdout, processStderr)
			}
			if stdout.String() != "" {
				t.Fatalf("deps stdout = %q, want empty", stdout.String())
			}
			if wantStderr == "" || stderr.String() != wantStderr {
				t.Fatalf("deps stderr = %q, want %q", stderr.String(), wantStderr)
			}
			if !reflect.DeepEqual(gotErr, wantErr) {
				t.Fatalf("error = %#v, want %#v", gotErr, wantErr)
			}
		})
	}
}
