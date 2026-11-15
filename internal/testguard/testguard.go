// Package testguard keeps a test binary away from the App Store Connect
// credentials of the developer who runs it.
//
// A developer who has signed in with asc has a real API key, Apple Ads key
// and web session on the machine: in ~/.asc, in a repo-local .asc above the
// checkout, in the keychain, and in ASC_* variables of their shell. A test that
// did not isolate one of these ran with that account, and a command test that
// expected to stop at "no authentication" sent real requests to Apple instead.
// In a test binary this package hides all of them, so every test sees the
// machine as CI does. It does nothing in any other binary, nor in the opt-in
// integration build, whose tests use the developer's credentials on purpose.
package testguard

import (
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"

	"github.com/99designs/keyring"
)

// isolatedVariable marks an environment that is already isolated: set by
// `make test`, and by the first test binary for the helper processes it starts.
const isolatedVariable = "ASC_TEST_ENV_ISOLATED"

// keptVariables are the test switches that `make test` also passes through
// (TEST_ENV_PASSTHROUGH in the Makefile).
var keptVariables = map[string]bool{
	"ASC_UPDATE_GOLDEN":                      true,
	"ASC_SIGNING_RUN_LIVE_TEST":              true,
	"ASC_SIGNING_KEYCHAIN_INSTALL_LIVE_TEST": true,
}

var active = testing.Testing() && !integrationBuild

// developerDirs are the developer's .asc directories while the guard is
// active, and nil otherwise.
var developerDirs []string

func init() {
	if !active {
		return
	}
	if os.Getenv(isolatedVariable) == "" {
		for _, entry := range os.Environ() {
			name, _, _ := strings.Cut(entry, "=")
			if strings.HasPrefix(name, "ASC_") && !keptVariables[name] {
				_ = os.Unsetenv(name)
			}
		}
		_ = os.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		_ = os.Setenv(isolatedVariable, "1")
	}

	home, _ := os.UserHomeDir()
	var accountHome string
	if current, err := user.Current(); err == nil {
		accountHome = current.HomeDir
	}
	workingDir, _ := os.Getwd()
	developerDirs = developerASCDirs(home, accountHome, workingDir)
}

// developerASCDirs returns the .asc directories that belong to whoever started
// the test binary: under the HOME it started with, under the account's home,
// and in its starting working directory and every directory above it, which
// the upward search for a local config reaches.
func developerASCDirs(home, accountHome, workingDir string) []string {
	var dirs []string
	add := func(dir string) {
		if filepath.IsAbs(dir) {
			dirs = append(dirs, filepath.Join(dir, ".asc"))
		}
	}
	add(home)
	add(accountHome)
	for dir := workingDir; filepath.IsAbs(dir); {
		add(dir)
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return dirs
}

// Hides reports whether path is inside a .asc directory of the developer
// running the tests. It is always false outside a test binary.
func Hides(path string) bool {
	return inDirs(developerDirs, path)
}

func inDirs(dirs []string, path string) bool {
	if len(dirs) == 0 || strings.TrimSpace(path) == "" {
		return false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	for _, dir := range dirs {
		rel, err := filepath.Rel(dir, abs)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// HideForTest also hides dirs, as if they belonged to the developer, until
// restore is called.
func HideForTest(dirs ...string) (restore func()) {
	previous := developerDirs
	developerDirs = append(append([]string(nil), previous...), dirs...)
	return func() { developerDirs = previous }
}

// OpenKeyring opens the system keyring. A test binary gets
// keyring.ErrNoAvailImpl, as on a machine without one; tests of keychain
// behaviour install a fake keyring instead.
func OpenKeyring(cfg keyring.Config) (keyring.Keyring, error) {
	if active {
		return nil, keyring.ErrNoAvailImpl
	}
	return keyring.Open(cfg)
}
