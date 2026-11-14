package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestProfilesListProfileStateFilter(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantFilter string
		response   string
		wantIDs    []string
		wantStates []string
	}{
		{
			name:       "defaults to active and invalid",
			wantFilter: "ACTIVE,INVALID",
			response:   `{"data":[{"type":"profiles","id":"profile-active","attributes":{"name":"Active","profileType":"IOS_APP_STORE","profileState":"ACTIVE"}},{"type":"profiles","id":"profile-invalid","attributes":{"name":"Expired","profileType":"IOS_APP_ADHOC","profileState":"INVALID"}}]}`,
			wantIDs:    []string{"profile-active", "profile-invalid"},
			wantStates: []string{"ACTIVE", "INVALID"},
		},
		{
			name:       "normalizes an explicit state",
			args:       []string{"--profile-state", "invalid"},
			wantFilter: "INVALID",
			response:   `{"data":[{"type":"profiles","id":"profile-invalid","attributes":{"name":"Expired","profileType":"IOS_APP_ADHOC","profileState":"INVALID"}}]}`,
			wantIDs:    []string{"profile-invalid"},
			wantStates: []string{"INVALID"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET("/v1/profiles", "filter[profileState]="+test.wantFilter), asctest.JSON(http.StatusOK, test.response))

			args := append([]string{"profiles", "list"}, test.args...)
			stdout, stderr, runErr := runRootCommand(t, append(args, "--output", "json"))
			if runErr != nil {
				t.Fatalf("run error: %v", runErr)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			var payload struct {
				Data []struct {
					ID         string `json:"id"`
					Attributes struct {
						ProfileState string `json:"profileState"`
					} `json:"attributes"`
				} `json:"data"`
			}
			if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
				t.Fatalf("unmarshal profiles output: %v\n%s", err, stdout)
			}
			if len(payload.Data) != len(test.wantIDs) {
				t.Fatalf("profile count = %d, want %d", len(payload.Data), len(test.wantIDs))
			}
			for i, profile := range payload.Data {
				if profile.ID != test.wantIDs[i] || profile.Attributes.ProfileState != test.wantStates[i] {
					t.Fatalf("profile[%d] = %+v, want id %q and state %q", i, profile, test.wantIDs[i], test.wantStates[i])
				}
			}
		})
	}
}

func TestProfilesListProfileStateInvalidValueReturnsUsageError(t *testing.T) {
	reg := asctest.NewRegistry(t)
	reg.Install()

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"profiles", "list", "--profile-state", "EXPIRED"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp usage error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--profile-state must be one of: ACTIVE, INVALID") {
		t.Fatalf("expected profile-state usage error, got %q", stderr)
	}
	if requestCount := len(reg.Requests()); requestCount != 0 {
		t.Fatalf("expected 0 requests, got %d", requestCount)
	}
}

func TestProfilesListsPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"profiles", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/profiles?cursor=AQ&limit=200",
			dataType: "profiles",
		},
		{
			args:     []string{"profiles", "links", "certificates"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/profiles/profile-1/relationships/certificates?cursor=AQ&limit=200",
			dataType: "certificates",
		},
		{
			args:     []string{"profiles", "links", "devices"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/profiles/profile-1/relationships/devices?cursor=AQ&limit=200",
			dataType: "devices",
		},
	})
}
