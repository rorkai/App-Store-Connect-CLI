package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNotarizationValidationErrors(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "submit missing file",
			args:    []string{"notarization", "submit"},
			wantErr: "--file is required",
		},
		{
			name:    "status missing id",
			args:    []string{"notarization", "status"},
			wantErr: "--id is required",
		},
		{
			name:    "log missing id",
			args:    []string{"notarization", "log"},
			wantErr: "--id is required",
		},
		{
			name:    "log path without fetch",
			args:    []string{"notarization", "log", "--id", "sub-1", "--path", "log.json"},
			wantErr: "--path requires --fetch",
		},
		{
			name:    "log overwrite without path",
			args:    []string{"notarization", "log", "--id", "sub-1", "--fetch", "--overwrite"},
			wantErr: "--overwrite requires --path",
		},
		{
			name:    "log fetch to stdout with table output",
			args:    []string{"notarization", "log", "--id", "sub-1", "--fetch", "--output", "table"},
			wantErr: "--fetch prints JSON",
		},
		{
			name:    "log empty path",
			args:    []string{"notarization", "log", "--id", "sub-1", "--fetch", "--path", ""},
			wantErr: "--path must be a file path",
		},
		{
			name:    "log directory path",
			args:    []string{"notarization", "log", "--id", "sub-1", "--fetch", "--path", "logs/"},
			wantErr: "--path must be a file path",
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
				t.Fatalf("expected error %q in stderr, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestNotarizationSubmitInvalidPollInterval(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"notarization", "submit", "--file", "/tmp/test.zip", "--poll-interval", "invalid"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		t.Logf("got expected error: %v", err)
	})

	_ = stderr
}

func TestNotarizationSubmitInvalidTimeout(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"notarization", "submit", "--file", "/tmp/test.zip", "--timeout", "invalid"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		t.Logf("got expected error: %v", err)
	})

	_ = stderr
}

func TestNotarizationSubmitNonexistentFile(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	captureOutput(t, func() {
		if err := root.Parse([]string{"notarization", "submit", "--file", "/nonexistent/file.zip"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error for nonexistent file, got nil")
		}
		t.Logf("got expected error: %v", err)
	})
}

func TestNotarizationSubmitDirectory(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	dir := t.TempDir()

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	captureOutput(t, func() {
		if err := root.Parse([]string{"notarization", "submit", "--file", dir}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error for directory, got nil")
		}
		t.Logf("got expected error: %v", err)
	})
}

func TestNotarizationSubmitEmptyFile(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	dir := t.TempDir()
	emptyFile := filepath.Join(dir, "empty.zip")
	if err := os.WriteFile(emptyFile, []byte{}, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	captureOutput(t, func() {
		if err := root.Parse([]string{"notarization", "submit", "--file", emptyFile}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error for empty file, got nil")
		}
		t.Logf("got expected error: %v", err)
	})
}

func TestNotarizationListNegativeLimit(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	captureOutput(t, func() {
		if err := root.Parse([]string{"notarization", "list", "--limit", "-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error for negative limit, got nil")
		}
		t.Logf("got expected error: %v", err)
	})
}

func TestNotarizationSubmitUnsupportedExtension(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	dir := t.TempDir()
	badFile := filepath.Join(dir, "app.txt")
	if err := os.WriteFile(badFile, []byte("not a real archive"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	captureOutput(t, func() {
		if err := root.Parse([]string{"notarization", "submit", "--file", badFile}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error for unsupported file type, got nil")
		}
		t.Logf("got expected error: %v", err)
	})
}

func TestNotarizationHelpOutput(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "notarization help",
			args: []string{"notarization"},
			want: "notarization",
		},
		{
			name: "notarization submit help",
			args: []string{"notarization", "submit"},
			want: "--file",
		},
		{
			name: "notarization status help",
			args: []string{"notarization", "status"},
			want: "--id",
		},
		{
			name: "notarization log help",
			args: []string{"notarization", "log"},
			want: "--id",
		},
		{
			name: "notarization group shows subcommands",
			args: []string{"notarization"},
			want: "submit",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			_, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				// Help output is expected
				if err != nil && !errors.Is(err, flag.ErrHelp) {
					t.Logf("run returned: %v", err)
				}
			})

			if !strings.Contains(stderr, test.want) {
				t.Fatalf("expected %q in help output, got %q", test.want, stderr)
			}
		})
	}
}

func TestNotarizationLogFetch(t *testing.T) {
	const (
		submissionID = "550e8400-e29b-41d4-a716-446655440000"
		logBody      = `{"status":"Invalid","issues":[{"severity":"error","path":"App.zip/App.app"}]}`
		logSignature = "asc-sentinel-log-signature"
		logURL       = "https://notary-artifacts-prod.s3.amazonaws.com/prod/" + submissionID + "/developer_log.json?X-Amz-Signature=" + logSignature
		existing     = "existing"
	)

	tests := []struct {
		name         string
		flags        func(dir string) []string
		wantErr      string
		wantAPICalls int
		wantLogCalls int
		check        func(t *testing.T, dir, stdout string)
	}{
		{
			name:         "default prints the URL and leaves the log unfetched",
			flags:        func(string) []string { return []string{"--output", "json"} },
			wantAPICalls: 1,
			check: func(t *testing.T, _, stdout string) {
				var resp struct {
					Data struct {
						ID         string `json:"id"`
						Attributes struct {
							DeveloperLogURL string `json:"developerLogUrl"`
						} `json:"attributes"`
					} `json:"data"`
				}
				if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
					t.Fatalf("decode stdout %q: %v", stdout, err)
				}
				if resp.Data.ID != submissionID || resp.Data.Attributes.DeveloperLogURL != logURL {
					t.Fatalf("unexpected default output: %s", stdout)
				}
			},
		},
		{
			name:         "fetch prints the log JSON",
			flags:        func(string) []string { return []string{"--fetch"} },
			wantAPICalls: 1,
			wantLogCalls: 1,
			check: func(t *testing.T, _, stdout string) {
				if strings.TrimSpace(stdout) != logBody {
					t.Fatalf("stdout = %q, want log JSON", stdout)
				}
			},
		},
		{
			name: "path writes the log and prints a receipt",
			flags: func(dir string) []string {
				return []string{"--fetch", "--path", filepath.Join(dir, "log.json"), "--output", "json"}
			},
			wantAPICalls: 1,
			wantLogCalls: 1,
			check: func(t *testing.T, dir, stdout string) {
				path := filepath.Join(dir, "log.json")
				assertNotarizationLogFile(t, path, logBody)
				var receipt map[string]any
				if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
					t.Fatalf("decode receipt %q: %v", stdout, err)
				}
				if receipt["id"] != submissionID || receipt["outputPath"] != path || receipt["bytesWritten"] != float64(len(logBody)) {
					t.Fatalf("unexpected receipt: %s", stdout)
				}
				if strings.Contains(stdout, logSignature) {
					t.Fatalf("receipt leaked the signed URL: %s", stdout)
				}
			},
		},
		{
			name: "existing file is refused before any request",
			flags: func(dir string) []string {
				path := filepath.Join(dir, "log.json")
				writeNotarizationLogFixture(t, path)
				return []string{"--fetch", "--path", path}
			},
			wantErr: "notarization log:",
			check: func(t *testing.T, dir, _ string) {
				assertNotarizationLogFile(t, filepath.Join(dir, "log.json"), existing)
			},
		},
		{
			name: "overwrite replaces an existing file",
			flags: func(dir string) []string {
				path := filepath.Join(dir, "log.json")
				writeNotarizationLogFixture(t, path)
				return []string{"--fetch", "--path", path, "--overwrite"}
			},
			wantAPICalls: 1,
			wantLogCalls: 1,
			check: func(t *testing.T, dir, _ string) {
				assertNotarizationLogFile(t, filepath.Join(dir, "log.json"), logBody)
			},
		},
		{
			name: "overwrite refuses a symlink before any request",
			flags: func(dir string) []string {
				target := filepath.Join(dir, "target.json")
				writeNotarizationLogFixture(t, target)
				link := filepath.Join(dir, "log.json")
				if err := os.Symlink(target, link); err != nil {
					t.Fatalf("symlink: %v", err)
				}
				return []string{"--fetch", "--path", link, "--overwrite"}
			},
			wantErr: "notarization log:",
			check: func(t *testing.T, dir, _ string) {
				assertNotarizationLogFile(t, filepath.Join(dir, "target.json"), existing)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

			var apiCalls, logCalls int
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				respond := func(body string) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body:       io.NopCloser(strings.NewReader(body)),
						Request:    req,
					}, nil
				}
				switch req.URL.Host {
				case "appstoreconnect.apple.com":
					apiCalls++
					if !strings.HasPrefix(req.Header.Get("Authorization"), "Bearer ") {
						t.Errorf("notary request has no bearer token")
					}
					return respond(`{"data":{"id":"` + submissionID + `","type":"submissionsLog","attributes":{"developerLogUrl":"` + logURL + `"}}}`)
				case "notary-artifacts-prod.s3.amazonaws.com":
					logCalls++
					if req.Header.Get("Authorization") != "" {
						t.Errorf("bearer token sent to the log host")
					}
					return respond(logBody)
				}
				t.Errorf("unexpected request to %s", req.URL.Host)
				return nil, errors.New("unexpected request")
			}))

			dir := t.TempDir()
			args := append([]string{"notarization", "log", "--id", submissionID}, test.flags(dir)...)
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			stdout, _ := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if test.wantErr == "" && runErr != nil {
				t.Fatalf("run error: %v", runErr)
			}
			if test.wantErr != "" && (runErr == nil || !strings.Contains(runErr.Error(), test.wantErr)) {
				t.Fatalf("run error = %v, want %q", runErr, test.wantErr)
			}
			if apiCalls != test.wantAPICalls || logCalls != test.wantLogCalls {
				t.Fatalf("api calls = %d, log calls = %d, want %d and %d", apiCalls, logCalls, test.wantAPICalls, test.wantLogCalls)
			}
			test.check(t, dir, stdout)
		})
	}
}

func writeNotarizationLogFixture(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("existing"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
}

func assertNotarizationLogFile(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("%s = %q, want %q", path, got, want)
	}
}
