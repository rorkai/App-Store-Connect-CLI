package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestBundleIDCapabilitiesSettingsValidationStopsBeforeHTTP(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()

	tests := []struct {
		name        string
		args        []string
		wantErr     string
		wantDetails []string
	}{
		{
			name:    "update requires id",
			args:    []string{"bundle-ids", "capabilities", "update", "--settings", `[{"key":"ICLOUD_VERSION"}]`},
			wantErr: "--id is required",
		},
		{
			name:    "update requires an update field",
			args:    []string{"bundle-ids", "capabilities", "update", "--id", "cap1"},
			wantErr: "at least one update field is required",
		},
		{
			name:    "update rejects empty settings array",
			args:    []string{"bundle-ids", "capabilities", "update", "--id", "cap1", "--settings", "[]"},
			wantErr: "at least one update field is required",
		},
		{
			name:    "update rejects invalid settings JSON",
			args:    []string{"bundle-ids", "capabilities", "update", "--id", "cap1", "--settings", "not-json"},
			wantErr: "--settings must be valid JSON array",
		},
		{
			name:    "add rejects unknown field",
			args:    []string{"bundle-ids", "capabilities", "add", "--bundle", "bundle1", "--capability", "ICLOUD", "--settings", `[{"key":"ICLOUD_VERSION","typo":true}]`},
			wantErr: `unknown field "typo"`,
		},
		{
			name:    "add rejects incorrectly cased field",
			args:    []string{"bundle-ids", "capabilities", "add", "--bundle", "bundle1", "--capability", "ICLOUD", "--settings", `[{"KEY":"ICLOUD_VERSION"}]`},
			wantErr: `unknown field "KEY"`,
		},
		{
			name:    "update rejects empty allowed instances",
			args:    []string{"bundle-ids", "capabilities", "update", "--id", "cap1", "--settings", `[{"key":"ICLOUD_VERSION","allowedInstances":""}]`},
			wantErr: `allowedInstances at setting index 0 must not be empty`,
		},
		{
			name:    "update rejects whitespace allowed instances",
			args:    []string{"bundle-ids", "capabilities", "update", "--id", "cap1", "--settings", `[{"key":"ICLOUD_VERSION","allowedInstances":" \t"}]`},
			wantErr: `allowedInstances at setting index 0 must not be blank`,
		},
		{
			name:    "add rejects empty description",
			args:    []string{"bundle-ids", "capabilities", "add", "--bundle", "bundle1", "--capability", "ICLOUD", "--settings", `[{"key":"ICLOUD_VERSION","description":""}]`},
			wantErr: `description at setting index 0 must not be empty`,
		},
		{
			name:    "update rejects missing setting key",
			args:    []string{"bundle-ids", "capabilities", "update", "--id", "cap1", "--settings", `[{"options":[]}]`},
			wantErr: `capability setting key at index 0 must not be empty`,
		},
		{
			name:        "add rejects malformed options",
			args:        []string{"bundle-ids", "capabilities", "add", "--bundle", "bundle1", "--capability", "ICLOUD", "--settings", `[{"key":"FUTURE_SETTING","options":{}}]`},
			wantErr:     `--settings must be valid JSON array: json: cannot unmarshal object`,
			wantDetails: []string{"options", "of type []asc.CapabilityOption"},
		},
		{
			name:    "add rejects null schema field",
			args:    []string{"bundle-ids", "capabilities", "add", "--bundle", "bundle1", "--capability", "ICLOUD", "--settings", `[{"key":"ICLOUD_VERSION","options":null}]`},
			wantErr: `settings[0].options must not be null`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			requestsBefore := len(reg.Requests())
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(tc.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			diagnostic := strings.SplitN(stderr, "\n", 2)[0]
			for _, want := range append([]string{tc.wantErr}, tc.wantDetails...) {
				if !strings.Contains(diagnostic, want) {
					t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
				}
			}
			if got := rootcmd.ExitCodeFromError(runErr); got != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, runErr)
			}
			if requestCount := len(reg.Requests()) - requestsBefore; requestCount != 0 {
				t.Fatalf("HTTP request count = %d, want 0", requestCount)
			}
		})
	}
}

func TestBundleIDCapabilitiesAddWithSettings(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	wantPayload := `{"data":{"type":"bundleIdCapabilities","attributes":{"capabilityType":"MARZIPAN","settings":[{"key":"ENABLED_FOR_MAC_APP_SETUP","options":[{"key":"USE_IOS_APPID","enabled":true}]}]},"relationships":{"bundleId":{"data":{"type":"bundleIds","id":"bundle1"}}}}}` + "\n"
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.POST("/v1/bundleIdCapabilities").JSONBody(wantPayload),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"bundleIdCapabilities","id":"cap1","attributes":{"capabilityType":"MARZIPAN","settings":[{"key":"ENABLED_FOR_MAC_APP_SETUP","options":[{"key":"USE_IOS_APPID","enabled":true}]}]}}}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"bundle-ids", "capabilities", "add",
			"--bundle", "bundle1", "--capability", "MARZIPAN",
			"--settings", `[{"key":"ENABLED_FOR_MAC_APP_SETUP","options":[{"key":"USE_IOS_APPID","enabled":true}]}]`,
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if payload := reg.Requests()[0].Body; string(payload) != wantPayload {
		t.Fatalf("request body = %q, want %q", payload, wantPayload)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"cap1"`) {
		t.Fatalf("unexpected stdout: %q", stdout)
	}
	if !strings.Contains(stdout, `"capabilityType":"MARZIPAN"`) ||
		!strings.Contains(stdout, `"key":"ENABLED_FOR_MAC_APP_SETUP"`) ||
		!strings.Contains(stdout, `"key":"USE_IOS_APPID"`) {
		t.Fatalf("forward-compatible response fields were not preserved: %q", stdout)
	}
}

func TestBundleIDCapabilitiesUpdateSuccessOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	wantPayload := `{"data":{"type":"bundleIdCapabilities","id":"cap1","attributes":{"settings":[{"key":"APP_GROUP_IDENTIFIERS","allowedInstances":"FUTURE_INSTANCE_MODE","options":[{"key":"group.com.example.shared","enabled":true}]}]}}}` + "\n"
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.PATCH("/v1/bundleIdCapabilities/cap1").JSONBody(wantPayload),
		asctest.JSON(http.StatusOK, `{"data":{"type":"bundleIdCapabilities","id":"cap1","attributes":{"capabilityType":"APP_GROUPS","settings":[{"key":"APP_GROUP_IDENTIFIERS","allowedInstances":"FUTURE_INSTANCE_MODE","options":[{"key":"group.com.example.shared","enabled":true}]}]}}}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"bundle-ids", "capabilities", "update", "--id", "cap1", "--settings", `[{"key":"APP_GROUP_IDENTIFIERS","allowedInstances":"FUTURE_INSTANCE_MODE","options":[{"key":"group.com.example.shared","enabled":true}]}]`}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if payload := reg.Requests()[0].Body; string(payload) != wantPayload {
		t.Fatalf("request body = %q, want %q", payload, wantPayload)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"cap1"`) {
		t.Fatalf("expected capability id in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"capabilityType":"APP_GROUPS"`) {
		t.Fatalf("expected capabilityType in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"key":"APP_GROUP_IDENTIFIERS"`) ||
		!strings.Contains(stdout, `"allowedInstances":"FUTURE_INSTANCE_MODE"`) ||
		!strings.Contains(stdout, `"key":"group.com.example.shared"`) {
		t.Fatalf("forward-compatible response fields were not preserved: %q", stdout)
	}
}

func TestBundleIDCapabilitiesUpdateWithCapabilityType(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.PATCH("/v1/bundleIdCapabilities/cap1").JSONBody(`{"data":{"type":"bundleIdCapabilities","id":"cap1","attributes":{"capabilityType":"PUSH_NOTIFICATIONS"}}}`),
		asctest.JSON(http.StatusOK, `{"data":{"type":"bundleIdCapabilities","id":"cap1","attributes":{"capabilityType":"PUSH_NOTIFICATIONS"}}}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"bundle-ids", "capabilities", "update", "--id", "cap1", "--capability", "PUSH_NOTIFICATIONS"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"cap1"`) {
		t.Fatalf("expected capability id in output, got %q", stdout)
	}
}
