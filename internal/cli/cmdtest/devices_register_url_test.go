package cmdtest

import (
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	} {
		t.Run(tc.name, func(t *testing.T) {
			outputPath := filepath.Join(t.TempDir(), "devices.tsv")
			root := RootCommand("test")
			args := append([]string{"devices", "register", "--output-file", outputPath}, tc.args...)
			if err := root.Parse(args); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			var runErr error
			_, stderr := captureOutput(t, func() { runErr = root.Run(ctx) })
			if !errors.Is(runErr, flag.ErrHelp) || !strings.Contains(runErr.Error()+stderr, tc.message) {
				t.Fatalf("want usage mentioning %q; err=%v stderr=%q", tc.message, runErr, stderr)
			}
			if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
				t.Fatalf("validation created output file: %v", err)
			}
		})
	}
}

func TestDevicesRegisterURLTableOutput(t *testing.T) {
	root := RootCommand("test")
	path := filepath.Join(t.TempDir(), "devices.tsv")
	if err := root.Parse([]string{"devices", "register", "--via-url", "--ttl", "1ms", "--output-file", path, "--output", "table", "--public-url", "https://tunnel.example/"}); err != nil {
		t.Fatal(err)
	}
	var runErr error
	stdout, _ := captureOutput(t, func() { runErr = root.Run(context.Background()) })
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
