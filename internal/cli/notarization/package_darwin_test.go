package notarization

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestNotarizationPackagePreservesSignedApp(t *testing.T) {
	run := func(ctx context.Context, args ...string) (string, error) {
		t.Helper()
		d := newNotarizationTestDeps(func() (*asc.Client, error) {
			t.Fatal("local packaging must not initialize authentication")
			return nil, nil
		})
		cmd := NewNotarizationCommand(d)
		if err := cmd.Parse(append([]string{"package"}, args...)); err != nil {
			t.Fatalf("parse registered package command: %v", err)
		}
		var err error
		stdout, _ := captureNotarizationOutput(t, d, func() { err = cmd.Run(ctx) })
		return stdout, err
	}
	// Exercise registration before building the native fixture.
	_, err := run(context.Background(), "--output", "json")
	if err == nil || !strings.Contains(err.Error(), "--file") {
		t.Fatalf("missing input error = %v", err)
	}
	app := makeSignedPackageFixture(t)
	archive := filepath.Join(t.TempDir(), "Package Fixture.zip")
	// Ambient ditto settings must not disable resource metadata or archiving.
	t.Setenv("DITTONORSRC", "1")
	t.Setenv("DITTO_TEST_OPTIONS", "1")
	stdout, err := run(context.Background(), "--file", app, "--output-path", archive, "--output", "json")
	if err != nil {
		t.Fatalf("package signed app: %v", err)
	}
	var result struct {
		AppPath     string `json:"appPath"`
		ArchivePath string `json:"archivePath"`
		SizeBytes   int64  `json:"sizeBytes"`
		SHA256      string `json:"sha256"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode receipt: %v; stdout=%q", err, stdout)
	}
	data, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	if result.AppPath != app || result.ArchivePath != archive || result.SizeBytes != int64(len(data)) || result.SHA256 != fmt.Sprintf("%x", sha256.Sum256(data)) {
		t.Fatalf("receipt does not describe the published archive: %+v", result)
	}
	t.Setenv("DITTONORSRC", "")
	t.Setenv("DITTO_TEST_OPTIONS", "")
	extracted := filepath.Join(t.TempDir(), "restored")
	packageFixtureTool(t, "/usr/bin/ditto", "-x", "-k", archive, extracted)
	restored := filepath.Join(extracted, filepath.Base(app))
	for _, path := range []string{app, restored} {
		packageFixtureTool(t, "/usr/bin/codesign", "--verify", "--deep", "--strict", path)
	}
	if got := packageFixtureTool(t, filepath.Join(restored, "Contents/MacOS/PackageFixture")); got != "fixture-ok" {
		t.Fatalf("extracted framework failed to load: %q", got)
	}
	for path, want := range map[string]string{"Versions/Current": "A", "FixtureKit": "Versions/Current/FixtureKit", "Resources": "Versions/Current/Resources"} {
		got, err := os.Readlink(filepath.Join(restored, "Contents/Frameworks/FixtureKit.framework", path))
		if err != nil || got != want {
			t.Fatalf("framework symlink %s = %q, %v; want %q", path, got, err, want)
		}
	}
	resource := filepath.Join(restored, "Contents/Resources/data.txt")
	info, err := os.Stat(resource)
	if err != nil || info.Mode().Perm() != 0o640 || info.ModTime().Unix() != 1600000000 {
		t.Fatalf("resource permissions/timestamp changed: %v, %v", info, err)
	}
	if got := packageFixtureTool(t, "/usr/bin/xattr", "-p", "com.example.ascPackaging", resource); got != "preserve-me" {
		t.Fatalf("resource metadata = %q", got)
	}
	zipped, err := zip.OpenReader(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer zipped.Close()
	metadata := false
	for _, file := range zipped.File {
		metadata = metadata || strings.HasPrefix(file.Name, "__MACOSX/")
	}
	if !metadata {
		t.Fatal("archive omitted resource metadata")
	}
	markdown, err := run(context.Background(), "--file", app, "--output-path", filepath.Join(t.TempDir(), "table.zip"), "--output", "markdown")
	if err != nil || !strings.Contains(strings.Join(strings.Fields(markdown), " "), "| App Path | Archive Path | Size (Bytes) | SHA256 |") || !strings.Contains(markdown, filepath.Base(app)) {
		t.Fatalf("package Markdown receipt: %v; output=%q", err, markdown)
	}
	for _, cancelDuringArchive := range []bool{false, true} {
		t.Run(fmt.Sprintf("partial archive canceled=%t", cancelDuringArchive), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			original := runPackageTool
			t.Cleanup(func() { runPackageTool = original })
			archiveErr := errors.New("archive operation interrupted")
			var partialPath string
			runPackageTool = func(ctx context.Context, diagnostics io.Writer, tool string, args ...string) error {
				if tool != "/usr/bin/ditto" {
					return original(ctx, diagnostics, tool, args...)
				}
				partialPath = args[len(args)-1]
				if err := os.WriteFile(partialPath, []byte("incomplete ZIP"), 0o600); err != nil {
					t.Fatal(err)
				}
				if cancelDuringArchive {
					cancel()
					return ctx.Err()
				}
				return archiveErr
			}
			output := filepath.Join(t.TempDir(), "partial.zip")
			stdout, err := run(ctx, "--file", app, "--output-path", output, "--output", "json")
			wantErr := archiveErr
			if cancelDuringArchive {
				wantErr = context.Canceled
			}
			if !errors.Is(err, wantErr) || stdout != "" || partialPath == "" {
				t.Fatalf("partial archive error = %v; stdout=%q; staging=%q", err, stdout, partialPath)
			}
			for _, path := range []string{output, filepath.Dir(partialPath)} {
				if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("failed archive retained %q: %v", path, err)
				}
			}
		})
	}

	for _, tc := range []struct {
		name, output string
		ctx          context.Context
		want         string
	}{
		{"existing output", archive, context.Background(), "exist"},
		{"output inside bundle", filepath.Join(app, "nested.zip"), context.Background(), "outside"},
		{"canceled", filepath.Join(t.TempDir(), "canceled.zip"), canceledPackageContext(), "context canceled"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stdout, err := run(tc.ctx, "--file", app, "--output-path", tc.output, "--output", "json")
			if err == nil || !strings.Contains(err.Error(), tc.want) || stdout != "" {
				t.Fatalf("expected refusal without receipt, got %v; stdout=%q", err, stdout)
			}
			if tc.output != archive {
				if _, err := os.Lstat(tc.output); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("failed operation left output: %v", err)
				}
			}
		})
	}
	if current, err := os.ReadFile(archive); err != nil || string(current) != string(data) {
		t.Fatal("refused overwrite changed existing archive")
	}
	if err := os.WriteFile(filepath.Join(app, "Contents/Resources/data.txt"), []byte("modified after signing"), 0o640); err != nil {
		t.Fatal(err)
	}
	invalidOutput := filepath.Join(t.TempDir(), "invalid.zip")
	stdout, err = run(context.Background(), "--file", app, "--output-path", invalidOutput, "--output", "json")
	if err == nil || !strings.Contains(err.Error(), "signature") || stdout != "" {
		t.Fatalf("invalid signature was not refused: %v; stdout=%q", err, stdout)
	}
	if _, err := os.Lstat(invalidOutput); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid signature left an archive: %v", err)
	}
}

func canceledPackageContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func packageFixtureTool(t *testing.T, tool string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), tool, args...)
	var env []string
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "DITTO") {
			env = append(env, entry)
		}
	}
	cmd.Env = env
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("fixture tool %s: %v\n%s", tool, err, output)
	}
	return strings.TrimSpace(string(output))
}

func makeSignedPackageFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	app := filepath.Join(root, "Package Fixture.app")
	framework := filepath.Join(app, "Contents/Frameworks/FixtureKit.framework")
	version := filepath.Join(framework, "Versions/A")
	files := map[string]string{
		filepath.Join(app, "Contents/Info.plist"):         `<plist version="1.0"><dict><key>CFBundleIdentifier</key><string>com.example.asc.packagefixture</string><key>CFBundleExecutable</key><string>PackageFixture</string><key>CFBundlePackageType</key><string>APPL</string><key>CFBundleVersion</key><string>1</string></dict></plist>`,
		filepath.Join(version, "Resources/Info.plist"):    `<plist version="1.0"><dict><key>CFBundleIdentifier</key><string>com.example.asc.fixturekit</string><key>CFBundleExecutable</key><string>FixtureKit</string><key>CFBundlePackageType</key><string>FMWK</string><key>CFBundleVersion</key><string>1</string></dict></plist>`,
		filepath.Join(app, "Contents/Resources/data.txt"): "package resource fixture\n",
		filepath.Join(root, "framework.c"):                "int fixture_answer(void) { return 42; }\n",
		filepath.Join(root, "main.c"):                     "#include <stdio.h>\nextern int fixture_answer(void);\nint main(void) { if (fixture_answer() != 42) return 1; puts(\"fixture-ok\"); return 0; }\n",
	}
	for path, data := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for path, target := range map[string]string{"Versions/Current": "A", "FixtureKit": "Versions/Current/FixtureKit", "Resources": "Versions/Current/Resources"} {
		if err := os.Symlink(target, filepath.Join(framework, path)); err != nil {
			t.Fatal(err)
		}
	}
	resource := filepath.Join(app, "Contents/Resources/data.txt")
	if err := os.Chmod(resource, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(resource, time.Unix(1600000000, 0), time.Unix(1600000000, 0)); err != nil {
		t.Fatal(err)
	}
	packageFixtureTool(t, "/usr/bin/xattr", "-w", "com.example.ascPackaging", "preserve-me", resource)
	packageFixtureTool(t, "/usr/bin/xcrun", "clang", "-dynamiclib", "-install_name", "@rpath/FixtureKit.framework/Versions/A/FixtureKit", filepath.Join(root, "framework.c"), "-o", filepath.Join(version, "FixtureKit"))
	if err := os.MkdirAll(filepath.Join(app, "Contents/MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	packageFixtureTool(t, "/usr/bin/xcrun", "clang", filepath.Join(root, "main.c"), "-F", filepath.Dir(framework), "-framework", "FixtureKit", "-Wl,-rpath,@executable_path/../Frameworks", "-o", filepath.Join(app, "Contents/MacOS/PackageFixture"))
	for _, path := range []string{framework, app} {
		packageFixtureTool(t, "/usr/bin/codesign", "--force", "--sign", "-", "--timestamp=none", path)
	}
	return app
}
