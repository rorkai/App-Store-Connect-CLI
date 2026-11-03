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
			args:      []string{"--recurrence-start-date", "2026-12-01T00:00:00+01:00", "--recurrence-duration", "PT168H", "--recurrence-rule", "FREQ=DAILY;INTERVAL=7"},
			patchPath: "/v1/gameCenterLeaderboards/LB_ID",
			wantAttrs: `{"recurrenceStartDate":"2026-12-01T00:00:00+01:00","recurrenceDuration":"PT168H","recurrenceRule":"FREQ=DAILY;INTERVAL=7"}`,
		},
		{
			name:      "v1 start date alone",
			args:      []string{"--recurrence-start-date", "2026-12-01T00:00:00Z"},
			patchPath: "/v1/gameCenterLeaderboards/LB_ID",
			wantAttrs: `{"recurrenceStartDate":"2026-12-01T00:00:00Z"}`,
		},
		{
			name:      "v1 duration alone",
			args:      []string{"--recurrence-duration", "PT36H"},
			patchPath: "/v1/gameCenterLeaderboards/LB_ID",
			wantAttrs: `{"recurrenceDuration":"PT36H"}`,
		},
		{
			name:      "v2 rule alone",
			args:      []string{"--v2", "--recurrence-rule", "FREQ=HOURLY;INTERVAL=24"},
			patchPath: "/v2/gameCenterLeaderboards/LB_ID",
			wantAttrs: `{"recurrenceRule":"FREQ=HOURLY;INTERVAL=24"}`,
		},
		{
			name:      "v2 all attributes",
			args:      []string{"--v2", "--recurrence-start-date", "2026-12-01T00:00:00Z", "--recurrence-duration", "PT24H", "--recurrence-rule", "FREQ=DAILY;INTERVAL=1"},
			patchPath: "/v2/gameCenterLeaderboards/LB_ID",
			wantAttrs: `{"recurrenceStartDate":"2026-12-01T00:00:00Z","recurrenceDuration":"PT24H","recurrenceRule":"FREQ=DAILY;INTERVAL=1"}`,
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
		badDuration    = "--recurrence-duration must be an ISO 8601 duration with time components only, such as PT168H for a week"
		badRule        = "--recurrence-rule must be FREQ=MINUTELY, HOURLY, or DAILY with ;INTERVAL=N, such as FREQ=DAILY;INTERVAL=7"
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
		{name: "lowercase duration", flag: "--recurrence-duration", value: "pt1h", wantErr: badDuration},
		// Apple rejects these with "Expected an ISO 8601 duration with time components".
		{name: "day duration Apple rejects", flag: "--recurrence-duration", value: "P7D", wantErr: badDuration},
		{name: "week duration Apple rejects", flag: "--recurrence-duration", value: "P1W", wantErr: badDuration},
		{name: "day with zero time Apple rejects", flag: "--recurrence-duration", value: "P7DT0H", wantErr: badDuration},
		// Apple accepts only FREQ=[MINUTELY,HOURLY,DAILY];INTERVAL=$INT.
		{name: "weekly rule Apple rejects", flag: "--recurrence-rule", value: "FREQ=WEEKLY;INTERVAL=1", wantErr: badRule},
		{name: "rule without interval", flag: "--recurrence-rule", value: "FREQ=DAILY", wantErr: badRule},
		{name: "zero interval", flag: "--recurrence-rule", value: "FREQ=DAILY;INTERVAL=0", wantErr: badRule},
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
