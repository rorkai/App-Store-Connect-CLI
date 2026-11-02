package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"testing"
)

func TestGameCenterLeaderboardsUpdateSendsOnlyRecurrenceFlagsSet(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		patchPath string
		wantAttrs string
	}{
		{
			name:      "v1 all attributes",
			args:      []string{"--recurrence-start-date", "2026-12-01T00:00:00+01:00", "--recurrence-duration", "P1W", "--recurrence-rule", "FREQ=WEEKLY"},
			patchPath: "/v1/gameCenterLeaderboards/LB_ID",
			wantAttrs: `{"recurrenceStartDate":"2026-12-01T00:00:00+01:00","recurrenceDuration":"P1W","recurrenceRule":"FREQ=WEEKLY"}`,
		},
		{
			name:      "v1 start date alone",
			args:      []string{"--recurrence-start-date", "2026-12-01T00:00:00Z"},
			patchPath: "/v1/gameCenterLeaderboards/LB_ID",
			wantAttrs: `{"recurrenceStartDate":"2026-12-01T00:00:00Z"}`,
		},
		{
			name:      "v1 duration alone",
			args:      []string{"--recurrence-duration", "P1DT12H"},
			patchPath: "/v1/gameCenterLeaderboards/LB_ID",
			wantAttrs: `{"recurrenceDuration":"P1DT12H"}`,
		},
		{
			name:      "v2 rule alone",
			args:      []string{"--v2", "--recurrence-rule", "FREQ=DAILY"},
			patchPath: "/v2/gameCenterLeaderboards/LB_ID",
			wantAttrs: `{"recurrenceRule":"FREQ=DAILY"}`,
		},
		{
			name:      "v2 all attributes",
			args:      []string{"--v2", "--recurrence-start-date", "2026-12-01T00:00:00Z", "--recurrence-duration", "PT30M", "--recurrence-rule", "FREQ=HOURLY"},
			patchPath: "/v2/gameCenterLeaderboards/LB_ID",
			wantAttrs: `{"recurrenceStartDate":"2026-12-01T00:00:00Z","recurrenceDuration":"PT30M","recurrenceRule":"FREQ=HOURLY"}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			callCount := &lockedCounter{}
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				callCount.Inc()
				if req.Method != http.MethodPatch || req.URL.Path != test.patchPath {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				}

				body, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("read request body: %v", err)
				}
				want := `{"data":{"type":"gameCenterLeaderboards","id":"LB_ID","attributes":` + test.wantAttrs + `}}`
				assertJSONDocumentEqual(t, body, []byte(want))

				return gameCenterCreateResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboards","id":"LB_ID","attributes":{}}}`), nil
			}))

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			args := append([]string{"game-center", "leaderboards", "update", "--id", "LB_ID"}, test.args...)
			captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			if got := callCount.Load(); got != 1 {
				t.Fatalf("request count = %d, want 1", got)
			}
		})
	}
}

func TestGameCenterLeaderboardsRecurrenceFlagsRejectBadValuesBeforeRequests(t *testing.T) {
	const (
		blankStartDate = "--recurrence-start-date must not be blank"
		blankDuration  = "--recurrence-duration must not be blank"
		blankRule      = "--recurrence-rule must not be blank"
		badStartDate   = "--recurrence-start-date must be an RFC 3339 date-time, such as 2026-12-01T00:00:00Z"
		badDuration    = "--recurrence-duration must be an ISO 8601 duration, such as P7D or PT12H"
	)

	commands := []struct {
		name string
		args []string
	}{
		{
			name: "create",
			args: []string{"game-center", "leaderboards", "create", "--app", "APP_ID", "--reference-name", "Test", "--vendor-id", "com.test", "--formatter", "INTEGER", "--sort", "DESC", "--submission-type", "BEST_SCORE"},
		},
		{
			name: "create v2",
			args: []string{"game-center", "leaderboards", "create", "--app", "APP_ID", "--reference-name", "Test", "--vendor-id", "com.test", "--formatter", "INTEGER", "--sort", "DESC", "--submission-type", "BEST_SCORE", "--v2"},
		},
		{
			name: "update",
			args: []string{"game-center", "leaderboards", "update", "--id", "LB_ID"},
		},
		{
			name: "update v2",
			args: []string{"game-center", "leaderboards", "update", "--id", "LB_ID", "--v2"},
		},
	}
	values := []struct {
		name    string
		flag    string
		value   string
		wantErr string
	}{
		{name: "empty start date", flag: "--recurrence-start-date", value: "", wantErr: blankStartDate},
		{name: "whitespace start date", flag: "--recurrence-start-date", value: "  ", wantErr: blankStartDate},
		{name: "empty duration", flag: "--recurrence-duration", value: "", wantErr: blankDuration},
		{name: "empty rule", flag: "--recurrence-rule", value: "", wantErr: blankRule},
		{name: "whitespace rule", flag: "--recurrence-rule", value: " ", wantErr: blankRule},
		{name: "date without time", flag: "--recurrence-start-date", value: "2026-12-01", wantErr: badStartDate},
		{name: "date without offset", flag: "--recurrence-start-date", value: "2026-12-01T00:00:00", wantErr: badStartDate},
		{name: "go-style duration", flag: "--recurrence-duration", value: "24h", wantErr: badDuration},
		{name: "bare designator", flag: "--recurrence-duration", value: "P", wantErr: badDuration},
		{name: "time designator with no time", flag: "--recurrence-duration", value: "P1DT", wantErr: badDuration},
		{name: "time unit before date unit", flag: "--recurrence-duration", value: "PT1H1D", wantErr: badDuration},
		{name: "lowercase duration", flag: "--recurrence-duration", value: "p7d", wantErr: badDuration},
	}

	for _, command := range commands {
		for _, value := range values {
			t.Run(command.name+"/"+value.name, func(t *testing.T) {
				setupAuth(t)
				t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
				installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
					return nil, nil
				}))

				args := append(append([]string{}, command.args...), value.flag, value.value)
				assertUsageExitCode(t, args, value.wantErr)
			})
		}
	}
}
