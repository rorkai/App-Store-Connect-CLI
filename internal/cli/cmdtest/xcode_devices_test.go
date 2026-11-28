package cmdtest

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestXcodeDevicesValidatesBeforeDiscovery(t *testing.T) {
	home := setCmdtestHome(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(home, "config.json"))
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"timeout", []string{"--timeout", "1ms"}, "--timeout must be between 5s and 10m"},
		{"output", []string{"--output", "xml"}, `--output must be one of: json, table, markdown (got "xml")`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := runWithDeps(t, nil, append([]string{"xcode", "devices"}, tc.args...)...)
			if code != 2 || stdout != "" || !strings.Contains(stderr, tc.want) {
				t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
		})
	}
}
