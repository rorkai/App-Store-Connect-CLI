package cmdtest

import (
	"context"
	"errors"
	"flag"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestDevicesRegisterURLValidation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []string
		message string
	}{
		{"zero ttl", []string{"--via-url", "--ttl", "0s"}, "--ttl"},
		{"negative ttl", []string{"--via-url", "--ttl", "-1s"}, "--ttl"},
		{"relative public url", []string{"--via-url", "--public-url", "phone"}, "--public-url"},
		{"public url query", []string{"--via-url", "--public-url", "https://example.com/?foo=bar"}, "--public-url"},
		{"public url path", []string{"--via-url", "--public-url", "https://example.com/asc"}, "--public-url"},
		{"unused listen", []string{"--listen", "127.0.0.1:0", "--name", "phone", "--udid", "123", "--platform", "IOS"}, "--listen requires --via-url"},
		{"output with confirm", []string{"--via-url", "--confirm"}, "--output-file cannot"},
		{"stream table", []string{"--via-url", "--stream", "--output", "table"}, "--stream requires --output json"},
		{"stream markdown", []string{"--via-url", "--stream", "--output", "markdown"}, "--stream requires --output json"},
		{"stream pretty", []string{"--via-url", "--stream", "--output", "json", "--pretty"}, "--stream cannot be combined with --pretty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outputPath := filepath.Join(t.TempDir(), "devices.tsv")
			d, _, stderrOut := newTestDeps(t, nil)
			root := rootcmd.RootCommandWithDeps("test", d)
			args := append([]string{"devices", "register", "--output-file", outputPath}, tc.args...)
			if err := root.Parse(args); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			runErr := root.Run(ctx)
			stderr := stderrOut.String()
			if !errors.Is(runErr, flag.ErrHelp) || !strings.Contains(runErr.Error()+stderr, tc.message) {
				t.Fatalf("want usage mentioning %q; err=%v stderr=%q", tc.message, runErr, stderr)
			}
			if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
				t.Fatalf("validation created output file: %v", err)
			}
		})
	}
}

func TestDevicesRegisterStreamRequiresViaURL(t *testing.T) {
	stdout, stderr, runErr := runRootWithDeps(t, nil, "devices", "register", "--stream", "--name", "phone", "--udid", "123", "--platform", "IOS")
	if !errors.Is(runErr, flag.ErrHelp) || !strings.Contains(stderr, "--stream requires --via-url") {
		t.Fatalf("want --stream usage error; err=%v stderr=%q", runErr, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q", stdout)
	}
}

func TestDevicesRegisterURLTableOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "devices.tsv")
	stdout, _, runErr := runRootWithDeps(t, nil, "devices", "register", "--via-url", "--ttl", "1ms", "--output-file", path, "--output", "table", "--public-url", "https://tunnel.example/")
	if runErr != nil {
		t.Fatal(runErr)
	}
	if !strings.Contains(stdout, "https://tunnel.example/enroll?") {
		t.Fatalf("root public URL did not generate the enrollment route: %s", stdout)
	}
	if !strings.Contains(stdout, "Output File") || !strings.Contains(stdout, path) {
		t.Fatalf("missing collection receipt: %s", stdout)
	}
}

func TestDevicesRegisterURLRejectsExistingOutputBeforeSession(t *testing.T) {
	for _, occupied := range []bool{false, true} {
		name := "available listener"
		if occupied {
			name = "occupied listener"
		}
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "devices.tsv")
			const original = "existing device data\n"
			if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
				t.Fatal(err)
			}
			listen := "127.0.0.1:0"
			if occupied {
				listener, err := net.Listen("tcp", listen)
				if err != nil {
					t.Fatal(err)
				}
				defer listener.Close()
				listen = listener.Addr().String()
			}
			d, stdoutOut, stderrOut := newTestDeps(t, nil)
			root := rootcmd.RootCommandWithDeps("test", d)
			if err := root.Parse([]string{"devices", "register", "--via-url", "--listen", listen, "--output-file", path}); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			runErr := root.Run(ctx)
			stdout, stderr := stdoutOut.String(), stderrOut.String()
			data, err := os.ReadFile(path)
			if err != nil || string(data) != original {
				t.Fatalf("existing output changed: data=%q err=%v", data, err)
			}
			if !errors.Is(runErr, os.ErrExist) {
				t.Fatalf("want existing output error before listener startup; got %v", runErr)
			}
			if stdout != "" || strings.Contains(stderr, "Open this URL") || strings.Contains(stderr, "/enroll?") {
				t.Fatalf("session started before rejecting output: stdout=%q stderr=%q", stdout, stderr)
			}
		})
	}
}
