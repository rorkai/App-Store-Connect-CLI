package cmdtest

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestXcodeInstallSimulatorRequiresExactInputs(t *testing.T) {
	home := setCmdtestHome(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(home, "config.json"))
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"missing app", []string{"--udid", "11111111-1111-1111-1111-111111111111"}, "--app is required"},
		{"missing simulator", []string{"--app", "Fixture.app"}, "--udid is required"},
		{"implicit simulator", []string{"--app", "Fixture.app", "--udid", "booted"}, "--udid must be an exact simulator UUID"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := runWithDeps(t, nil, append([]string{"xcode", "install-simulator"}, tc.args...)...)
			if code != 2 || stdout != "" || !strings.Contains(stderr, tc.want) {
				t.Fatalf("exit=%d stdout=%q stderr=%q; want usage error %q", code, stdout, stderr, tc.want)
			}
		})
	}
}
