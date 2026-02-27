package shared

import (
	"flag"
	"io"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestApplyRootLoggingOverridesDebugFalseWinsOverEnv(t *testing.T) {
	t.Setenv("ASC_DEBUG", "1")
	resetRootLoggingFlagsForTest()
	asc.SetDebugOverride(nil)
	asc.SetDebugHTTPOverride(nil)
	asc.SetRetryLogOverride(nil)
	t.Cleanup(func() {
		resetRootLoggingFlagsForTest()
		asc.SetDebugOverride(nil)
		asc.SetDebugHTTPOverride(nil)
		asc.SetRetryLogOverride(nil)
	})

	fs := flag.NewFlagSet("asc", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	BindRootFlags(fs)
	if err := fs.Parse([]string{"--debug=false"}); err != nil {
		t.Fatalf("parse root flags: %v", err)
	}

	ApplyRootLoggingOverrides()
	if asc.ResolveDebugEnabled() {
		t.Fatal("expected --debug=false to disable debug logging")
	}
}

func TestApplyRootLoggingOverridesAPIDebugEnablesDebug(t *testing.T) {
	t.Setenv("ASC_DEBUG", "")
	resetRootLoggingFlagsForTest()
	asc.SetDebugOverride(nil)
	asc.SetDebugHTTPOverride(nil)
	asc.SetRetryLogOverride(nil)
	t.Cleanup(func() {
		resetRootLoggingFlagsForTest()
		asc.SetDebugOverride(nil)
		asc.SetDebugHTTPOverride(nil)
		asc.SetRetryLogOverride(nil)
	})

	fs := flag.NewFlagSet("asc", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	BindRootFlags(fs)
	if err := fs.Parse([]string{"--api-debug"}); err != nil {
		t.Fatalf("parse root flags: %v", err)
	}

	ApplyRootLoggingOverrides()
	if !asc.ResolveDebugEnabled() {
		t.Fatal("expected --api-debug to enable debug logging")
	}
}

func resetRootLoggingFlagsForTest() {
	retryLog = OptionalBool{}
	debug = OptionalBool{}
	apiDebug = OptionalBool{}
}
