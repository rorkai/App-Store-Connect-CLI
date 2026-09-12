package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseStage_MissingMetadataSource(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"release", "stage",
			"--app", "APP_123",
			"--version", "1.2.3",
			"--build-id", "BUILD_123",
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "exactly one of --metadata-dir or --copy-metadata-from is required") {
		t.Fatalf("expected metadata source error, got %q", stderr)
	}
}

func TestReleaseStage_InvalidCopyFieldsValue(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"release", "stage",
			"--app", "APP_123",
			"--version", "1.2.3",
			"--build-id", "BUILD_123",
			"--copy-metadata-from", "1.2.2",
			"--copy-fields", "description,notAField",
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "--copy-fields must be one of") {
		t.Fatalf("expected invalid copy-fields error, got %q", stderr)
	}
}

func TestReleaseStage_DryRunCopyMetadataFromVersion(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		body := ""
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/BUILD_123/relationships/app":
			body = `{"data":{"type":"apps","id":"APP_123"}}`
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/BUILD_123/preReleaseVersion":
			body = `{"data":{"type":"preReleaseVersions","id":"PRE_RELEASE_123","attributes":{"version":"1.2.3","platform":"IOS"}}}`
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/APP_123/appStoreVersions":
			body = `{"data":[]}`
		default:
			t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"release", "stage",
			"--app", "APP_123",
			"--version", "1.2.3",
			"--build-id", "BUILD_123",
			"--copy-metadata-from", "1.2.2",
			"--copy-fields", "description,keywords",
			"--exclude-fields", "keywords",
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"status":"dry-run"`) {
		t.Fatalf("expected dry-run status, got %q", stdout)
	}
	if !strings.Contains(stdout, `"copyMetadataFrom":"1.2.2"`) {
		t.Fatalf("expected copy metadata source in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"message":"would create app store version"`) {
		t.Fatalf("expected ensure version dry-run message, got %q", stdout)
	}
	if !strings.Contains(stdout, `"message":"metadata copy plan deferred until version exists"`) {
		t.Fatalf("expected deferred metadata copy message, got %q", stdout)
	}
	if !strings.Contains(stdout, `"name":"validate_build","status":"dry-run"`) {
		t.Fatalf("expected the build precondition check in the plan, got %q", stdout)
	}
	if requestCount != 3 {
		t.Fatalf("expected the build ownership and platform checks plus the version lookup, got %d requests", requestCount)
	}
}

// TestReleaseStage_ResolvesTrimmedCheckpointFile pins the CLI surface for
// --checkpoint-file: like every other path flag it is trimmed, so the reported
// and used checkpoint path never carries the operator's surrounding whitespace.
func TestReleaseStage_ResolvesTrimmedCheckpointFile(t *testing.T) {
	setupAuth(t)

	workDir := t.TempDir()
	t.Chdir(workDir)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body := ""
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/BUILD_123/relationships/app":
			body = `{"data":{"type":"apps","id":"APP_123"}}`
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/BUILD_123/preReleaseVersion":
			body = `{"data":{"type":"preReleaseVersions","id":"PRE_RELEASE_123","attributes":{"version":"1.2.3","platform":"IOS"}}}`
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/APP_123/appStoreVersions":
			body = `{"data":[]}`
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"release", "stage",
			"--app", "APP_123",
			"--version", "1.2.3",
			"--build-id", "BUILD_123",
			"--copy-metadata-from", "1.2.2",
			"--checkpoint-file", "  stage-checkpoint.json  ",
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	var plan struct {
		CheckpointFile string `json:"checkpointFile"`
	}
	if err := json.Unmarshal([]byte(stdout), &plan); err != nil {
		t.Fatalf("parse plan output %q: %v", stdout, err)
	}
	if !filepath.IsAbs(plan.CheckpointFile) {
		t.Fatalf("expected an absolute checkpoint path, got %q", plan.CheckpointFile)
	}
	if filepath.Base(plan.CheckpointFile) != "stage-checkpoint.json" {
		t.Fatalf("expected the checkpoint file name to be trimmed, got %q", plan.CheckpointFile)
	}
}
