package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
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

			originalTransport := http.DefaultTransport
			t.Cleanup(func() {
				http.DefaultTransport = originalTransport
			})

			requests := 0
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				if req.Method != http.MethodPatch || req.URL.Path != "/v1/betaGroups/bg-1" {
					t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
				}

				var payload struct {
					Data struct {
						Attributes map[string]any `json:"attributes"`
					} `json:"data"`
				}
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					t.Fatalf("decode body error: %v", err)
				}
				if !reflect.DeepEqual(payload.Data.Attributes, tt.want) {
					t.Errorf("attributes = %#v, want %#v", payload.Data.Attributes, tt.want)
				}

				response := `{"data":{"type":"betaGroups","id":"bg-1","attributes":{"name":"Group"}}}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(response)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			})

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			_, stderr := captureOutput(t, func() {
				args := append([]string{"testflight", "groups", "edit", "--id", "bg-1"}, tt.args...)
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if requests != 1 {
				t.Fatalf("requests = %d, want 1", requests)
			}
		})
	}
}
