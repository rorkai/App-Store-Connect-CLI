package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestMetadataInitValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing dir",
			args:    []string{"metadata", "init"},
			wantErr: "Error: --dir is required",
		},
		{
			name:    "invalid locale",
			args:    []string{"metadata", "init", "--dir", "./metadata", "--locale", "../en-US"},
			wantErr: `invalid locale "../en-US"`,
		},
		{
			name:    "invalid version",
			args:    []string{"metadata", "init", "--dir", "./metadata", "--version", "../1.0"},
			wantErr: `invalid version "../1.0"`,
		},
		{
			name:    "positional args",
			args:    []string{"metadata", "init", "--dir", "./metadata", "extra"},
			wantErr: "metadata init does not accept positional arguments",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
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
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected %q in stderr, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestMetadataInitWritesCanonicalTemplates(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "metadata")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "init",
			"--dir", outputDir,
			"--version", "1.2.3",
			"--locale", "en-US",
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

	appInfoPath := filepath.Join(outputDir, "app-info", "en-US.json")
	appInfoData, err := os.ReadFile(appInfoPath)
	if err != nil {
		t.Fatalf("read app-info file: %v", err)
	}
	wantAppInfo := `{"name":"","subtitle":"","privacyPolicyUrl":"","privacyChoicesUrl":"","privacyPolicyText":""}`
	if string(appInfoData) != wantAppInfo {
		t.Fatalf("app-info file = %q, want %q", string(appInfoData), wantAppInfo)
	}

	versionPath := filepath.Join(outputDir, "version", "1.2.3", "en-US.json")
	versionData, err := os.ReadFile(versionPath)
	if err != nil {
		t.Fatalf("read version file: %v", err)
	}
	wantVersion := `{"description":"","keywords":"","marketingUrl":"","promotionalText":"","supportUrl":"","whatsNew":""}`
	if string(versionData) != wantVersion {
		t.Fatalf("version file = %q, want %q", string(versionData), wantVersion)
	}

	var payload struct {
		Locale    string   `json:"locale"`
		Version   string   `json:"version"`
		FileCount int      `json:"fileCount"`
		Files     []string `json:"files"`
		NextSteps []string `json:"nextSteps"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}

	if payload.Locale != "en-US" || payload.Version != "1.2.3" {
		t.Fatalf("unexpected locale/version in payload: %+v", payload)
	}
	if payload.FileCount != 2 || len(payload.Files) != 2 {
		t.Fatalf("expected 2 files in output, got %+v", payload)
	}
	sortedFiles := append([]string(nil), payload.Files...)
	slices.Sort(sortedFiles)
	if !slices.Equal(payload.Files, sortedFiles) {
		t.Fatalf("expected deterministic sorted file list, got %v", payload.Files)
	}
	if len(payload.NextSteps) == 0 {
		t.Fatalf("expected next steps in payload, got %+v", payload)
	}
}

func TestMetadataInitCanCreateAppInfoOnly(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "metadata")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "init",
			"--dir", outputDir,
			"--locale", "en-US",
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

	if _, err := os.Stat(filepath.Join(outputDir, "app-info", "en-US.json")); err != nil {
		t.Fatalf("expected app-info template: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "version")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no version directory, got err=%v", err)
	}

	var payload struct {
		Version   string `json:"version"`
		FileCount int    `json:"fileCount"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if payload.Version != "" || payload.FileCount != 1 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestMetadataInitRequiresForceToOverwriteExistingFiles(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "metadata")
	appInfoDir := filepath.Join(outputDir, "app-info")
	if err := os.MkdirAll(appInfoDir, 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	existingPath := filepath.Join(appInfoDir, "en-US.json")
	if err := os.WriteFile(existingPath, []byte(`{"name":"existing"}`), 0o644); err != nil {
		t.Fatalf("write existing file: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "init",
			"--dir", outputDir,
			"--locale", "en-US",
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
	if !strings.Contains(stderr, "refusing to overwrite existing file") {
		t.Fatalf("expected overwrite refusal, got %q", stderr)
	}

	data, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatalf("read existing file: %v", err)
	}
	if string(data) != `{"name":"existing"}` {
		t.Fatalf("expected existing file to remain unchanged, got %q", string(data))
	}
}

func TestMetadataInitForceOverwritesExistingFiles(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "metadata")
	appInfoDir := filepath.Join(outputDir, "app-info")
	if err := os.MkdirAll(appInfoDir, 0o755); err != nil {
		t.Fatalf("mkdir app-info: %v", err)
	}
	existingPath := filepath.Join(appInfoDir, "en-US.json")
	if err := os.WriteFile(existingPath, []byte(`{"name":"existing"}`), 0o644); err != nil {
		t.Fatalf("write existing file: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"metadata", "init",
			"--dir", outputDir,
			"--locale", "en-US",
			"--force",
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

	data, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatalf("read existing file: %v", err)
	}
	want := `{"name":"","subtitle":"","privacyPolicyUrl":"","privacyChoicesUrl":"","privacyPolicyText":""}`
	if string(data) != want {
		t.Fatalf("expected force overwrite, got %q", string(data))
	}
}
