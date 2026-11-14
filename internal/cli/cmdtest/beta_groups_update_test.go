package cmdtest

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestBetaGroupsUpdateSendsOnlyRequestedPlatformAvailability(t *testing.T) {
	const macFlag = "--ios-builds-available-for-apple-silicon-mac"
	const visionFlag = "--ios-builds-available-for-apple-vision"

	tests := []struct {
		name string
		args []string
		want map[string]any
	}{
		{
			name: "bare flags send true",
			args: []string{macFlag, visionFlag},
			want: map[string]any{
				"iosBuildsAvailableForAppleSiliconMac": true,
				"iosBuildsAvailableForAppleVision":     true,
			},
		},
		{
			name: "explicit false is sent",
			args: []string{macFlag + "=false", visionFlag + "=false"},
			want: map[string]any{
				"iosBuildsAvailableForAppleSiliconMac": false,
				"iosBuildsAvailableForAppleVision":     false,
			},
		},
		{
			name: "mac alone is an update and leaves vision out",
			args: []string{macFlag + "=false"},
			want: map[string]any{"iosBuildsAvailableForAppleSiliconMac": false},
		},
		{
			name: "vision alone is an update and leaves mac out",
			args: []string{visionFlag + "=false"},
			want: map[string]any{"iosBuildsAvailableForAppleVision": false},
		},
		{
			name: "omitted flags leave both out",
			args: []string{"--name", "Renamed"},
			want: map[string]any{"name": "Renamed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			attributes, err := json.Marshal(tt.want)
			if err != nil {
				t.Fatalf("encode attributes: %v", err)
			}
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.PATCH("/v1/betaGroups/bg-1").JSONBody(`{"data":{"type":"betaGroups","id":"bg-1","attributes":`+string(attributes)+`}}`),
				asctest.JSON(http.StatusOK, `{"data":{"type":"betaGroups","id":"bg-1","attributes":{"name":"Group"}}}`))

			_, stderr, err := runCommand(t, append([]string{"testflight", "groups", "edit", "--id", "bg-1"}, tt.args...))
			if err != nil {
				t.Fatalf("run error: %v", err)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if requests := len(reg.Requests()); requests != 1 {
				t.Fatalf("requests = %d, want 1", requests)
			}
		})
	}
}
