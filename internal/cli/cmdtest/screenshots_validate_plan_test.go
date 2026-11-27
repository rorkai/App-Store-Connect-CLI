package cmdtest

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestScreenshotsValidatePlanOffline(t *testing.T) {
	home := setCmdtestHome(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(home, "config.json"))
	bin := t.TempDir()
	calls := filepath.Join(t.TempDir(), "device-calls")
	t.Setenv("PLAN_DEVICE_CALLS", calls)
	script := `#!/bin/sh
printf '%s\n' device >> "$PLAN_DEVICE_CALLS"
exit 1
`
	for _, name := range []string{"axe", "xcrun"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	cases := []struct {
		name, root, app, defaults, steps, output string
		want                                     []string
	}{
		{name: "valid JSONC", output: "json"},
		{name: "table", output: "table"},
		{name: "markdown", output: "markdown"},
		{name: "root field", root: `, "version_extra": 1`, want: []string{`unknown field "version_extra"`}},
		{name: "unknown field in repeated object", root: `, "app":{"bundle_idd":"typo"}`, want: []string{`unknown field "app.bundle_idd"`}},
		{name: "app field", app: `, "bundle_idd":"typo"`, want: []string{`unknown field "app.bundle_idd"`}},
		{name: "defaults field", defaults: `"post_action_delay_mss":1`, want: []string{`unknown field "defaults.post_action_delay_mss"`}},
		{name: "step field", steps: `[{"action":"launch"},{"action":"wait","duration_ms":1,"duration_mss":2500}]`, want: []string{`unknown field "steps[1].duration_mss"`, "step 2"}},
		{name: "independent fields", root: `, "version_extra":1`, defaults: `"delay_mss":1`, steps: `[{"action":"wait","duration_ms":1,"duration_mss":2500}]`, want: []string{`unknown field "version_extra"`, `unknown field "defaults.delay_mss"`, `unknown field "steps[0].duration_mss"`, "step 1"}},
		{name: "semantic error", steps: `[{"action":"wait","duration_ms":0}]`, want: []string{"wait requires duration_ms > 0"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			outputDir := filepath.Join(dir, "must-not-exist")
			plan := filepath.Join(dir, "plan.json")
			steps := tc.steps
			if steps == "" {
				steps = `[{"action":"launch"},{"action":"wait","duration_ms":1}]`
			}
			data := `{"version":1` + tc.root + `,"app":{"bundle_id":"example.fixture","udid":"unavailable-device","output_dir":` + strconv.Quote(outputDir) + tc.app + `},"defaults":{` + tc.defaults + `},"steps":` + steps + ` // allowed comment
}`
			if err := os.WriteFile(plan, []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			var clientCalls int
			client := func() (*asc.Client, error) {
				clientCalls++
				return nil, errors.New("offline validation requested API client")
			}
			format := tc.output
			if format == "" {
				format = "json"
			}
			stdout, stderr, code := runWithDeps(t, client, "screenshots", "validate-plan", "--plan", plan, "--output", format)
			if len(tc.want) > 0 {
				if code != 2 || stdout != "" {
					t.Fatalf("exit=%d stdout=%q stderr=%q, want usage error without stdout", code, stdout, stderr)
				}
				for _, want := range tc.want {
					if !strings.Contains(stderr, want) {
						t.Errorf("stderr=%q, want %q", stderr, want)
					}
				}
			} else {
				if code != 0 || stderr != "" {
					t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
				}
				if format == "json" {
					var result struct {
						Valid      bool   `json:"valid"`
						PlanPath   string `json:"planPath"`
						TotalSteps int    `json:"totalSteps"`
					}
					if err := json.Unmarshal([]byte(stdout), &result); err != nil {
						t.Fatal(err)
					}
					if !result.Valid || result.PlanPath != plan || result.TotalSteps != 2 {
						t.Fatalf("unexpected validation receipt: %+v", result)
					}
				}
			}
			if clientCalls != 0 {
				t.Fatalf("offline validation requested API client %d times", clientCalls)
			}
			if _, err := os.Stat(outputDir); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("validation touched run output directory: %v", err)
			}
			if got, err := os.ReadFile(calls); err == nil {
				t.Fatalf("device interactions: %s", got)
			} else if !errors.Is(err, os.ErrNotExist) {
				t.Fatal(err)
			}
		})
	}
}
