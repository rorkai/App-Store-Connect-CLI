package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"slices"
	"testing"
)

func TestBetaGroupFlagResolvesWholeNameBeforeSplittingOnCommas(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantIDs []string
	}{
		{
			name:    "builds add-groups name with comma",
			args:    []string{"builds", "add-groups", "--build-id", "build-1", "--group", "Staff, Family & Friends"},
			wantIDs: []string{"group-comma"},
		},
		{
			name:    "builds add-groups comma-separated names",
			args:    []string{"builds", "add-groups", "--build-id", "build-1", "--group", "Staff,Family & Friends"},
			wantIDs: []string{"group-staff", "group-family"},
		},
		{
			name:    "publish testflight name with comma",
			args:    []string{"publish", "testflight", "--app", "123", "--build-id", "build-1", "--group", "Staff, Family & Friends"},
			wantIDs: []string{"group-comma"},
		},
		{
			name:    "publish testflight comma-separated names",
			args:    []string{"publish", "testflight", "--app", "123", "--build-id", "build-1", "--group", "Staff,Family & Friends"},
			wantIDs: []string{"group-staff", "group-family"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			t.Setenv("ASC_APP_ID", "")

			originalTransport := http.DefaultTransport
			t.Cleanup(func() {
				http.DefaultTransport = originalTransport
			})

			var addedIDs []string
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/build-1/app":
					return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"apps","id":"123"}}`), nil
				case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/123/betaGroups":
					return jsonHTTPResponse(http.StatusOK, `{"data":[`+
						`{"type":"betaGroups","id":"group-staff","attributes":{"name":"Staff","isInternalGroup":true}},`+
						`{"type":"betaGroups","id":"group-family","attributes":{"name":"Family & Friends","isInternalGroup":true}},`+
						`{"type":"betaGroups","id":"group-comma","attributes":{"name":"Staff, Family & Friends","isInternalGroup":true}}]}`), nil
				case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/build-1":
					return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID","expired":false}}}`), nil
				case req.Method == http.MethodPost && req.URL.Path == "/v1/builds/build-1/relationships/betaGroups":
					var payload struct {
						Data []struct {
							ID string `json:"id"`
						} `json:"data"`
					}
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("failed to decode group assignment payload: %v", err)
					}
					for _, item := range payload.Data {
						addedIDs = append(addedIDs, item.ID)
					}
					return jsonHTTPResponse(http.StatusNoContent, ""), nil
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			})

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			_, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			if !slices.Equal(addedIDs, test.wantIDs) {
				t.Fatalf("added group IDs = %v, want %v (stderr %q)", addedIDs, test.wantIDs, stderr)
			}
		})
	}
}
