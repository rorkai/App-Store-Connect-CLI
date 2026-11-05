package signing

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

func TestReadSigningSyncTargetsFileSortsAndTrimsBundleIDs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "targets.json")
	writeSigningSyncTargetsFile(t, path, `{"schemaVersion":1,"targets":[{"bundleId":" com.example.z "},{"bundleId":"com.example.a"},{"bundleId":"com.example.m"}]}`, 0o644)
	t.Chdir(dir)

	got, err := readSigningSyncTargetsFile("targets.json")
	if err != nil {
		t.Fatalf("readSigningSyncTargetsFile() error = %v", err)
	}
	want := []string{"com.example.a", "com.example.m", "com.example.z"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("bundle IDs = %v, want %v", got, want)
	}
}

func TestReadSigningSyncTargetsFileAcceptsReadableNonSecretPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are not portable on Windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "targets.json")
	writeSigningSyncTargetsFile(t, path, `{"schemaVersion":1,"targets":[{"bundleId":"com.example.app"}]}`, 0o644)
	t.Chdir(dir)
	if _, err := readSigningSyncTargetsFile("targets.json"); err != nil {
		t.Fatalf("readSigningSyncTargetsFile() rejected readable 0644 manifest: %v", err)
	}
}

func TestSigningSyncPushPreservesTargetsFilePathWhitespace(t *testing.T) {
	dir := t.TempDir()
	filename := " targets.json "
	writeSigningSyncTargetsFile(t, filepath.Join(dir, filename), `{"schemaVersion":1,"targets":[{"bundleId":"com.example.app"}]}`, 0o644)
	t.Chdir(dir)

	clientCalls := 0
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		clientCalls++
		return nil, errors.New("client reached after manifest")
	}))

	t.Setenv(signingSyncPasswordEnvVar, "repository-password")
	cmd := syncPushCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.Parse([]string{
		"--targets-file", filename,
		"--profile-type", "IOS_APP_STORE",
		"--repo", "git@example.com:team/signing.git",
	}); err != nil {
		t.Fatal(err)
	}
	err := cmd.Run(context.Background())
	if err == nil || err.Error() != "signing sync push: client reached after manifest" {
		t.Fatalf("error = %v, want client error after manifest read", err)
	}
	if clientCalls != 1 {
		t.Fatalf("client factory calls = %d, want 1", clientCalls)
	}
}

func TestReadSigningSyncTargetsFileRejectsUnsafeManifest(t *testing.T) {
	tooManyTargets := make([]string, 33)
	for index := range tooManyTargets {
		tooManyTargets[index] = `{"bundleId":"com.example.target` + fmt.Sprint(index) + `"}`
	}
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "wrong schema", body: `{"schemaVersion":2,"targets":[{"bundleId":"com.example.app"}]}`, want: "schemaVersion"},
		{name: "unknown top-level field", body: `{"schemaVersion":1,"targets":[{"bundleId":"com.example.app"}],"secret":"no"}`, want: "unknown field"},
		{name: "unknown target field", body: `{"schemaVersion":1,"targets":[{"bundleId":"com.example.app","profileType":"IOS_APP_STORE"}]}`, want: "unknown field"},
		{name: "case-sensitive top-level key", body: `{"SchemaVersion":1,"targets":[{"bundleId":"com.example.app"}]}`, want: "unknown field"},
		{name: "case-sensitive target key", body: `{"schemaVersion":1,"targets":[{"bundleID":"com.example.app"}]}`, want: "unknown field"},
		{name: "duplicate top-level key", body: `{"schemaVersion":1,"targets":[{"bundleId":"com.example.app"}],"targets":[{"bundleId":"com.example.other"}]}`, want: "duplicate"},
		{name: "duplicate target key", body: `{"schemaVersion":1,"targets":[{"bundleId":"com.example.app","bundleId":"com.example.other"}]}`, want: "duplicate"},
		{name: "empty targets", body: `{"schemaVersion":1,"targets":[]}`, want: "between 1 and 32"},
		{name: "empty bundle ID", body: `{"schemaVersion":1,"targets":[{"bundleId":"  "}]}`, want: "bundleId"},
		{name: "path separator", body: `{"schemaVersion":1,"targets":[{"bundleId":"com/example.app"}]}`, want: "path"},
		{name: "control character", body: "{\"schemaVersion\":1,\"targets\":[{\"bundleId\":\"com.example.\\u0001\"}]}", want: "control"},
		{name: "bidi character", body: "{\"schemaVersion\":1,\"targets\":[{\"bundleId\":\"com.example.\\u202eapp\"}]}", want: "bidi"},
		{name: "duplicate case insensitive", body: `{"schemaVersion":1,"targets":[{"bundleId":"com.example.app"},{"bundleId":"COM.EXAMPLE.APP"}]}`, want: "duplicate"},
		{name: "trailing JSON", body: `{"schemaVersion":1,"targets":[{"bundleId":"com.example.app"}]} {}`, want: "trailing"},
		{name: "more than 32 targets", body: `{"schemaVersion":1,"targets":[` + strings.Join(tooManyTargets, ",") + `]}`, want: "between 1 and 32"},
		{name: "oversize manifest", body: `{"schemaVersion":1,"targets":[{"bundleId":"com.example.app","padding":"` + strings.Repeat("x", 65536) + `"}]}`, want: "64 KiB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "targets.json")
			writeSigningSyncTargetsFile(t, path, tt.body, 0o600)
			t.Chdir(dir)
			if _, err := readSigningSyncTargetsFile("targets.json"); err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.want)) {
				t.Fatalf("readSigningSyncTargetsFile() error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestReadSigningSyncTargetsFileRejectsSymlinkAndDirectory(t *testing.T) {
	t.Run("symlink", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("symlink creation is not portable on Windows")
		}
		dir := t.TempDir()
		target := filepath.Join(dir, "real.json")
		writeSigningSyncTargetsFile(t, target, `{"schemaVersion":1,"targets":[{"bundleId":"com.example.app"}]}`, 0o600)
		path := filepath.Join(dir, "targets.json")
		if err := os.Symlink(target, path); err != nil {
			t.Fatal(err)
		}
		t.Chdir(dir)
		if _, err := readSigningSyncTargetsFile("targets.json"); !errors.Is(err, rootfs.ErrSymlink) {
			t.Fatalf("readSigningSyncTargetsFile() error = %v, want rootfs.ErrSymlink", err)
		}
	})

	t.Run("directory", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "targets.json")
		if err := os.Mkdir(path, 0o755); err != nil {
			t.Fatal(err)
		}
		t.Chdir(filepath.Dir(path))
		if _, err := readSigningSyncTargetsFile(filepath.Base(path)); err == nil || !strings.Contains(err.Error(), "regular") {
			t.Fatalf("readSigningSyncTargetsFile() error = %v, want regular-file error", err)
		}
	})
}

func TestReadSigningSyncTargetsFileRejectsRootEscape(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), filepath.Base(root)+"-outside.json")
	t.Cleanup(func() { _ = os.Remove(outside) })
	writeSigningSyncTargetsFile(t, outside, `{"schemaVersion":1,"targets":[{"bundleId":"com.example.app"}]}`, 0o644)
	t.Chdir(root)

	_, err := readSigningSyncTargetsFile(filepath.Join("..", filepath.Base(outside)))
	if !errors.Is(err, rootfs.ErrEscapesRoot) {
		t.Fatalf("readSigningSyncTargetsFile() error = %v, want rootfs.ErrEscapesRoot", err)
	}
}

func TestReadSigningSyncTargetsFileRejectsAbsoluteRootEscape(t *testing.T) {
	root := t.TempDir()
	outsideDir := t.TempDir()
	outside := filepath.Join(outsideDir, "targets.json")
	writeSigningSyncTargetsFile(t, outside, `{"schemaVersion":1,"targets":[{"bundleId":"com.example.app"}]}`, 0o644)
	t.Chdir(root)

	_, err := readSigningSyncTargetsFile(outside)
	if !errors.Is(err, rootfs.ErrEscapesRoot) {
		t.Fatalf("readSigningSyncTargetsFile() error = %v, want rootfs.ErrEscapesRoot", err)
	}
}

func TestReadSigningSyncTargetsFileRejectsSymlinkedParentComponent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation is not portable on Windows")
	}
	root := t.TempDir()
	realDir := filepath.Join(root, "real")
	path := filepath.Join(realDir, "targets.json")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSigningSyncTargetsFile(t, path, `{"schemaVersion":1,"targets":[{"bundleId":"com.example.app"}]}`, 0o644)
	if err := os.Symlink(realDir, filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)

	_, err := readSigningSyncTargetsFile(filepath.Join("linked", "targets.json"))
	if !errors.Is(err, rootfs.ErrSymlink) {
		t.Fatalf("readSigningSyncTargetsFile() error = %v, want rootfs.ErrSymlink", err)
	}
}

func TestReadSigningSyncTargetsFileRejectsBlankPath(t *testing.T) {
	if _, err := readSigningSyncTargetsFile(" "); err == nil || !errors.Is(err, errSigningSyncTargetsManifestPath) {
		t.Fatalf("readSigningSyncTargetsFile() error = %v, want blank-path sentinel", err)
	}
}

func writeSigningSyncTargetsFile(t *testing.T, path, contents string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), mode); err != nil {
		t.Fatal(err)
	}
}
