package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMetadataApplyValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "metadata apply missing dir",
			args:    []string{"metadata", "apply", "--app", "123456789", "--version", "1.2.3"},
			wantErr: "--dir is required",
		},
		{
			name:    "metadata apply positional args rejected",
			args:    []string{"metadata", "apply", "--app", "123456789", "--version", "1.2.3", "--dir", "./metadata", "extra"},
			wantErr: "metadata apply does not accept positional arguments",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestMetadataApplyDryRunSuggestsApplyCommandForAmbiguousAppInfos(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "en-US.json"), []byte(`{"name":"App Name"}`), 0o644); err != nil {
		t.Fatalf("write app-info file: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-1/appStoreVersions":
			body := `{
				"data":[
					{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION"}}
				],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/apps/app-1/appInfos":
			body := `{
				"data":[
					{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}},
					{"type":"appInfos","id":"appinfo-2","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}
				]
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected path: %s", req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "apply",
			"--app", "app-1",
			"--version", "1.2.3",
			"--platform", "IOS",
			"--dir", dir,
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, `Error: multiple app infos found for app "app-1"`) {
		t.Fatalf("expected ambiguous app-info error, got %q", stderr)
	}
	if !strings.Contains(stderr, `asc apps info list --app "app-1"`) {
		t.Fatalf("expected remediation to mention apps info list, got %q", stderr)
	}
	if !strings.Contains(stderr, `asc metadata apply --app "app-1" --version "1.2.3" --platform IOS --dir "`+dir+`" --app-info "appinfo-1" --dry-run`) {
		t.Fatalf("expected remediation example to use metadata apply, got %q", stderr)
	}
	if !strings.Contains(stderr, "appinfo-2") {
		t.Fatalf("expected all candidate app info ids in remediation, got %q", stderr)
	}
}

func TestMetadataApplyFailsOnPartialMutation(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-info"), 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "version", "1.2.3"), 0o755); err != nil {
		t.Fatalf("mkdir version dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app-info", "en-US.json"), []byte(`{"name":"App Name","subtitle":"Local subtitle"}`), 0o644); err != nil {
		t.Fatalf("write app-info file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "version", "1.2.3", "en-US.json"), []byte(`{"description":"Local description"}`), 0o644); err != nil {
		t.Fatalf("write version file: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	patchCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-1/appInfos":
			body := `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/apps/app-1/appStoreVersions":
			body := `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appInfos/appinfo-1/appInfoLocalizations":
			body := `{"data":[{"type":"appInfoLocalizations","id":"loc-app-en","attributes":{"locale":"en-US","name":"App Name","subtitle":"Remote subtitle"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			body := `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-ver-en","attributes":{"locale":"en-US","description":"Remote description"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/appInfoLocalizations/loc-app-en":
			if req.Method == http.MethodPatch {
				patchCount++
				body := `{"data":{"type":"appInfoLocalizations","id":"loc-app-en","attributes":{"locale":"en-US","name":"App Name","subtitle":"Local subtitle"}}}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}
		case "/v1/appStoreVersionLocalizations/loc-ver-en":
			if req.Method == http.MethodPatch {
				patchCount++
				body := `{"errors":[{"status":"500","code":"INTERNAL_ERROR","title":"Internal Error","detail":"boom"}]}`
				return &http.Response{
					StatusCode: http.StatusInternalServerError,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}
		}
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		return nil, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "apply",
			"--app", "app-1",
			"--version", "1.2.3",
			"--dir", dir,
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected apply failure error")
	}
	if patchCount != 2 {
		t.Fatalf("expected two patch attempts before failure, got %d", patchCount)
	}
	if !strings.Contains(runErr.Error(), "metadata apply: update version localization en-US") {
		t.Fatalf("expected apply-prefixed version-localization failure, got %v", runErr)
	}
	if !strings.Contains(runErr.Error(), "description") {
		t.Fatalf("expected version-localization failure to include attempted fields, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout on failure, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr on failure, got %q", stderr)
	}
}
