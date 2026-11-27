package config

import (
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/diagnosticoutput"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/testguard"
)

// accountHomeDir returns the home directory recorded for the current OS
// account, independent of $HOME. Tests replace it to simulate an overridden
// HOME without depending on the host account.
var accountHomeDir = func() (string, error) {
	current, err := user.Current()
	if err != nil {
		return "", err
	}
	return current.HomeDir, nil
}

var (
	accountHomeWarningMu      sync.Mutex
	accountHomeWarningWritten bool
	accountHomeWarningOutput  io.Writer = diagnosticoutput.Writer(os.Stderr)
)

// warnIfAccountHomeLocalConfig warns once per process when the upward local
// config search selected the account home's .asc/config.json while HOME points
// somewhere else. Overriding HOME is a common way to isolate a process from
// stored state, but the upward search still reaches this file whenever the
// working directory is inside the account home, so the process silently uses
// the account's global config as a "local" one. The file is still used for
// compatibility; the warning names the explicit ASC_CONFIG_PATH alternative.
// A test binary does not warn about a file the test guard hides: it is never
// read there.
func warnIfAccountHomeLocalConfig(localPath string) {
	if testguard.Hides(localPath) {
		return
	}
	ownerDir := filepath.Dir(filepath.Dir(localPath))
	home, err := os.UserHomeDir()
	if err != nil || sameDirectory(ownerDir, home) {
		return
	}
	accountHome, err := accountHomeDir()
	if err != nil || strings.TrimSpace(accountHome) == "" || !sameDirectory(ownerDir, accountHome) {
		return
	}

	accountHomeWarningMu.Lock()
	defer accountHomeWarningMu.Unlock()
	if accountHomeWarningWritten {
		return
	}
	accountHomeWarningWritten = true
	fmt.Fprintf(
		accountHomeWarningOutput,
		"Warning: using %s as the local config because the current directory is inside the account home directory, but HOME is %s. Overriding HOME does not isolate this file; set %s to an absolute path to choose the config explicitly. A future release will stop treating the account home config as a local config when HOME points elsewhere.\n",
		localPath,
		home,
		configPathEnvVar,
	)
}

func sameDirectory(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" || b == "" {
		return false
	}
	if filepath.Clean(a) == filepath.Clean(b) {
		return true
	}
	aInfo, err := os.Stat(a)
	if err != nil {
		return false
	}
	bInfo, err := os.Stat(b)
	if err != nil {
		return false
	}
	return os.SameFile(aInfo, bInfo)
}
