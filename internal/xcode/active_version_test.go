package xcode

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestParseActiveXcodeMajorVersion(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		want    int
		wantErr string
	}{
		{name: "Xcode 15", output: "Xcode 15.4\nBuild version 15F31d\n", want: 15},
		{name: "Xcode 16", output: "Xcode 16.0\nBuild version 16A242d\n", want: 16},
		{name: "Xcode beta", output: "Xcode 27.0 beta 4\nBuild version 27A5228h\n", want: 27},
		{name: "warning before version", output: "warning\nXcode 16.2.1\nBuild version 16C5032a\n", want: 16},
		{name: "empty", wantErr: `unexpected xcodebuild -version output: "empty output"`},
		{name: "malformed", output: "Command Line Tools 16.0\n", wantErr: "unexpected xcodebuild -version output"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseActiveXcodeMajorVersion(test.output)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("parseActiveXcodeMajorVersion() error = %v, want containing %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseActiveXcodeMajorVersion() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("parseActiveXcodeMajorVersion() = %d, want %d", got, test.want)
			}
		})
	}
}

func TestActiveXcodeMajorVersionIncludesCommandDiagnostic(t *testing.T) {
	originalCommandContext := commandContextFn
	t.Cleanup(func() { commandContextFn = originalCommandContext })
	originalTrustedXcodeToolPath := trustedXcodeToolPathFn
	trustedXcodeToolPathFn = func(context.Context, string, []string) (string, error) {
		return "/usr/bin/xcodebuild", nil
	}
	t.Cleanup(func() { trustedXcodeToolPathFn = originalTrustedXcodeToolPath })
	commandContextFn = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestActiveXcodeVersionHelperProcess")
		cmd.Env = append(os.Environ(), "GO_WANT_ACTIVE_XCODE_VERSION_HELPER=error")
		return cmd
	}

	_, err := ActiveXcodeMajorVersion(context.Background())
	if err == nil || !strings.Contains(err.Error(), "active developer directory is a command line tools instance") {
		t.Fatalf("ActiveXcodeMajorVersion() error = %v", err)
	}
}

func TestActiveXcodeMajorVersionPreservesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := ActiveXcodeMajorVersion(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ActiveXcodeMajorVersion() error = %v, want context.Canceled", err)
	}
}

func TestActiveXcodeVersionHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_ACTIVE_XCODE_VERSION_HELPER") != "error" {
		return
	}
	_, _ = os.Stderr.WriteString("xcode-select: active developer directory is a command line tools instance\n")
	os.Exit(1)
}
