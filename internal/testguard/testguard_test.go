//go:build !integration

package testguard

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/99designs/keyring"
)

func TestHidesTheDevelopersASCDirectories(t *testing.T) {
	base := t.TempDir()
	home := filepath.Join(base, "home")
	accountHome := filepath.Join(base, "account")
	workingDir := filepath.Join(base, "src", "asc", "internal", "pkg")
	dirs := developerASCDirs(home, accountHome, workingDir)

	for _, test := range []struct {
		path string
		want bool
	}{
		{filepath.Join(home, ".asc", "config.json"), true},
		{filepath.Join(home, ".asc"), true},
		{filepath.Join(accountHome, ".asc", "web", "last.json"), true},
		{filepath.Join(workingDir, ".asc", "config.json"), true},
		{filepath.Join(base, "src", "asc", ".asc", "config.json"), true},
		{filepath.Join(base, ".asc", "config.json"), true},
		{filepath.Join(workingDir, "testdata", ".asc", "config.json"), false},
		{filepath.Join(home, "project", ".asc", "config.json"), false},
		{filepath.Join(home, ".ascx", "config.json"), false},
		{filepath.Join(t.TempDir(), ".asc", "config.json"), false},
		{"", false},
	} {
		if got := inDirs(dirs, test.path); got != test.want {
			t.Errorf("hidden(%q) = %v, want %v", test.path, got, test.want)
		}
	}
}

func TestOpenKeyringRefusesTheSystemKeychain(t *testing.T) {
	if _, err := OpenKeyring(keyring.Config{ServiceName: "asc-testguard"}); !errors.Is(err, keyring.ErrNoAvailImpl) {
		t.Fatalf("OpenKeyring() error = %v, want %v", err, keyring.ErrNoAvailImpl)
	}
}

const reportVariable = "TESTGUARD_REPORT_ENV"

func TestTestBinaryDropsInheritedCredentialVariablesOnce(t *testing.T) {
	if os.Getenv(reportVariable) == "1" {
		report := map[string]string{}
		for _, entry := range os.Environ() {
			if name, value, _ := strings.Cut(entry, "="); strings.HasPrefix(name, "ASC_") {
				report[name] = value
			}
		}
		_ = json.NewEncoder(os.Stdout).Encode(report)
		os.Exit(0)
	}

	var inherited []string
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "ASC_") {
			inherited = append(inherited, entry)
		}
	}
	run := func(env ...string) map[string]string {
		t.Helper()
		cmd := exec.Command(os.Args[0], "-test.run=^TestTestBinaryDropsInheritedCredentialVariablesOnce$")
		cmd.Env = append(append(append([]string(nil), inherited...), reportVariable+"=1"), env...)
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		if err := cmd.Run(); err != nil {
			t.Fatalf("run test binary: %v", err)
		}
		report := map[string]string{}
		if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
			t.Fatalf("decode report %q: %v", stdout.String(), err)
		}
		return report
	}

	fromShell := run("ASC_KEY_ID=developer-key", "ASC_PRIVATE_KEY_PATH=/developer/key.p8", "ASC_CONFIG_PATH=/developer/config.json",
		"ASC_APP_ID=developer-app", "ASC_ADS_CLIENT_ID=developer-ads", "ASC_WEB_APPLE_ID=developer@example.com",
		"ASC_BYPASS_KEYCHAIN=0", "ASC_UPDATE_GOLDEN=1", "ASC_EXAMPLE_HELPER_PROCESS=1")
	want := map[string]string{isolatedVariable: "1", "ASC_BYPASS_KEYCHAIN": "0", "ASC_UPDATE_GOLDEN": "1", "ASC_EXAMPLE_HELPER_PROCESS": "1"}
	if !maps.Equal(fromShell, want) {
		t.Errorf("environment inherited from a shell = %v, want %v", fromShell, want)
	}

	fromTest := run(isolatedVariable+"=1", "ASC_CONFIG_PATH=/test/config.json", "ASC_EXAMPLE_HELPER_PROCESS=1")
	want = map[string]string{isolatedVariable: "1", "ASC_CONFIG_PATH": "/test/config.json", "ASC_EXAMPLE_HELPER_PROCESS": "1"}
	if !maps.Equal(fromTest, want) {
		t.Errorf("environment set by a test for its helper process = %v, want %v", fromTest, want)
	}
}

func TestSystemKeychainIsOpenedOnlyThroughOpenKeyring(t *testing.T) {
	self, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join("..", "..")
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if name := entry.Name(); path != root && (strings.HasPrefix(name, ".") || name == "testdata" || name == "website") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if dir, err := filepath.Abs(filepath.Dir(path)); err != nil || dir == self {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.Contains(data, []byte("keyring.Open")) {
			t.Errorf("%s calls keyring.Open; open the system keychain with testguard.OpenKeyring so test binaries cannot reach it", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
