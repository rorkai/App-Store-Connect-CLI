package cmdtest

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestXcodeInstallLaunchStillRequiresIPA(t *testing.T) {
	home := setCmdtestHome(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(home, "config.json"))
	stdout, stderr, code := runWithDeps(t, nil, "xcode", "install", "--launch", "--device-id", "SELECTOR_CANARY")
	if code != 2 || stdout != "" || !strings.Contains(stderr, "--ipa is required") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}
