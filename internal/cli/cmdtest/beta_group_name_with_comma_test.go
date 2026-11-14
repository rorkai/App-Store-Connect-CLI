package cmdtest

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"slices"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
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

			reg := asctest.NewRegistry(t)
			reg.Install()
			if test.args[0] == "builds" {
				reg.Register(asctest.GET("/v1/builds/build-1/app"),
					asctest.JSON(http.StatusOK, `{"data":{"type":"apps","id":"123"}}`))
			}
			reg.Register(asctest.GET("/v1/apps/123/betaGroups", "limit=200"),
				asctest.JSON(http.StatusOK, `{"data":[`+
					`{"type":"betaGroups","id":"group-staff","attributes":{"name":"Staff","isInternalGroup":true}},`+
					`{"type":"betaGroups","id":"group-family","attributes":{"name":"Family & Friends","isInternalGroup":true}},`+
					`{"type":"betaGroups","id":"group-comma","attributes":{"name":"Staff, Family & Friends","isInternalGroup":true}}]}`))
			reg.Register(asctest.GET("/v1/builds/build-1"),
				asctest.JSON(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID","expired":false}}}`))
			reg.Register(asctest.POST("/v1/builds/build-1/relationships/betaGroups"),
				asctest.JSON(http.StatusNoContent, ""))

			_, stderr, err := runCommand(t, test.args)
			if err != nil {
				t.Fatalf("run error: %v", err)
			}

			var payload struct {
				Data []struct {
					ID string `json:"id"`
				} `json:"data"`
			}
			requests := reg.Requests()
			if err := json.Unmarshal(requests[len(requests)-1].Body, &payload); err != nil {
				t.Fatalf("failed to decode group assignment payload: %v", err)
			}
			var addedIDs []string
			for _, item := range payload.Data {
				addedIDs = append(addedIDs, item.ID)
			}

			if !slices.Equal(addedIDs, test.wantIDs) {
				t.Fatalf("added group IDs = %v, want %v (stderr %q)", addedIDs, test.wantIDs, stderr)
			}
		})
	}
}
