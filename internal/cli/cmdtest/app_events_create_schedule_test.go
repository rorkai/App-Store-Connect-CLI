package cmdtest

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func newAppEventsTestClient(t *testing.T, transport roundTripFunc) *asc.Client {
	t.Helper()

	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "key.p8")
	writeECDSAPEM(t, keyPath)

	httpClient := &http.Client{Transport: transport}
	client, err := asc.NewClientWithHTTPClient("KEY123", "ISS456", keyPath, httpClient)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	return client
}

func TestAppEventsCreateAppliesScheduleAfterCreate(t *testing.T) {
	requests := 0

	client := newAppEventsTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++

		var captured map[string]any
		if err := json.NewDecoder(req.Body).Decode(&captured); err != nil {
			t.Fatalf("request %d: failed to decode request body: %v", requests, err)
		}

		switch requests {
		case 1:
			if req.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", req.Method)
			}
			if req.URL.Path != "/v1/appEvents" {
				t.Fatalf("expected path /v1/appEvents, got %s", req.URL.Path)
			}

			data, ok := captured["data"].(map[string]any)
			if !ok {
				t.Fatalf("expected data object, got %#v", captured["data"])
			}
			attrs, ok := data["attributes"].(map[string]any)
			if !ok {
				t.Fatalf("expected attributes object, got %#v", data["attributes"])
			}
			if _, ok := attrs["territorySchedules"]; ok {
				t.Fatalf("expected territorySchedules to be omitted from create, got %#v", attrs["territorySchedules"])
			}
			if attrs["referenceName"] != "Launch" {
				t.Fatalf("expected referenceName Launch, got %#v", attrs["referenceName"])
			}
			if attrs["badge"] != "CHALLENGE" {
				t.Fatalf("expected badge CHALLENGE, got %#v", attrs["badge"])
			}
			if attrs["primaryLocale"] != "en-US" {
				t.Fatalf("expected primaryLocale en-US, got %#v", attrs["primaryLocale"])
			}

			relationships, ok := data["relationships"].(map[string]any)
			if !ok {
				t.Fatalf("expected relationships object, got %#v", data["relationships"])
			}
			appRel, ok := relationships["app"].(map[string]any)
			if !ok {
				t.Fatalf("expected app relationship, got %#v", relationships["app"])
			}
			appData, ok := appRel["data"].(map[string]any)
			if !ok {
				t.Fatalf("expected app relationship data, got %#v", appRel["data"])
			}
			if appData["id"] != "app-123" {
				t.Fatalf("expected app id app-123, got %#v", appData["id"])
			}

			return jsonResponse(http.StatusCreated, `{"data":{"type":"appEvents","id":"event-1","attributes":{"referenceName":"Launch","badge":"CHALLENGE"}}}`)
		case 2:
			if req.Method != http.MethodPatch {
				t.Fatalf("expected PATCH, got %s", req.Method)
			}
			if req.URL.Path != "/v1/appEvents/event-1" {
				t.Fatalf("expected path /v1/appEvents/event-1, got %s", req.URL.Path)
			}

			data, ok := captured["data"].(map[string]any)
			if !ok {
				t.Fatalf("expected data object, got %#v", captured["data"])
			}
			if data["id"] != "event-1" {
				t.Fatalf("expected update id event-1, got %#v", data["id"])
			}
			attrs, ok := data["attributes"].(map[string]any)
			if !ok {
				t.Fatalf("expected update attributes object, got %#v", data["attributes"])
			}
			schedules, ok := attrs["territorySchedules"].([]any)
			if !ok {
				t.Fatalf("expected territorySchedules array, got %#v", attrs["territorySchedules"])
			}
			if len(schedules) != 1 {
				t.Fatalf("expected 1 territory schedule, got %d", len(schedules))
			}
			schedule, ok := schedules[0].(map[string]any)
			if !ok {
				t.Fatalf("expected territory schedule object, got %#v", schedules[0])
			}
			if schedule["publishStart"] != "2026-05-15T00:00:00Z" {
				t.Fatalf("expected publishStart to be preserved, got %#v", schedule["publishStart"])
			}
			if schedule["eventStart"] != "2026-06-01T00:00:00Z" {
				t.Fatalf("expected eventStart to be preserved, got %#v", schedule["eventStart"])
			}
			if schedule["eventEnd"] != "2026-06-30T23:59:59Z" {
				t.Fatalf("expected eventEnd to be preserved, got %#v", schedule["eventEnd"])
			}
			territories, ok := schedule["territories"].([]any)
			if !ok {
				t.Fatalf("expected territories array, got %#v", schedule["territories"])
			}
			if len(territories) != 2 || territories[0] != "USA" || territories[1] != "CAN" {
				t.Fatalf("expected normalized territories [USA CAN], got %#v", territories)
			}

			return jsonResponse(http.StatusOK, `{"data":{"type":"appEvents","id":"event-1","attributes":{"referenceName":"Launch","badge":"CHALLENGE","primaryLocale":"en-US","territorySchedules":[{"territories":["USA","CAN"],"publishStart":"2026-05-15T00:00:00Z","eventStart":"2026-06-01T00:00:00Z","eventEnd":"2026-06-30T23:59:59Z"}]}}}`)
		default:
			t.Fatalf("unexpected extra request %d: %s %s", requests, req.Method, req.URL.Path)
			return nil, nil
		}
	}))

	stdout, stderr, err := runRootWithDeps(t, func() (*asc.Client, error) { return client, nil },
		"app-events", "create",
		"--app", "app-123",
		"--name", "Launch",
		"--event-type", "CHALLENGE",
		"--start", "2026-06-01T00:00:00Z",
		"--end", "2026-06-30T23:59:59Z",
		"--publish-start", "2026-05-15T00:00:00Z",
		"--territories", "usa, can",
		"--primary-locale", "en-US",
	)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var resp asc.AppEventResponse
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if resp.Data.ID != "event-1" {
		t.Fatalf("expected event id event-1, got %q", resp.Data.ID)
	}
	if len(resp.Data.Attributes.TerritorySchedules) != 1 {
		t.Fatalf("expected exactly one territory schedule, got %d", len(resp.Data.Attributes.TerritorySchedules))
	}
	schedule := resp.Data.Attributes.TerritorySchedules[0]
	if schedule.EventStart != "2026-06-01T00:00:00Z" {
		t.Fatalf("expected eventStart to be preserved, got %q", schedule.EventStart)
	}
	if schedule.EventEnd != "2026-06-30T23:59:59Z" {
		t.Fatalf("expected eventEnd to be preserved, got %q", schedule.EventEnd)
	}
	if schedule.PublishStart != "2026-05-15T00:00:00Z" {
		t.Fatalf("expected publishStart to be preserved, got %q", schedule.PublishStart)
	}
	if len(schedule.Territories) != 2 || schedule.Territories[0] != "USA" || schedule.Territories[1] != "CAN" {
		t.Fatalf("expected normalized territories [USA CAN], got %#v", schedule.Territories)
	}
	if requests != 2 {
		t.Fatalf("expected create+update flow, got %d requests", requests)
	}
}

func TestAppEventsCreateReportsScheduleFailureOutcome(t *testing.T) {
	type response func() (*http.Response, error)
	const unchangedEvent = `{"data":{"type":"appEvents","id":"event-1","attributes":{"referenceName":"Launch","badge":"CHALLENGE"}}}`
	patchConflict := func() (*http.Response, error) {
		return jsonResponse(http.StatusConflict, `{"errors":[{"status":"409","code":"ENTITY_ERROR.ATTRIBUTE.INVALID","detail":"territorySchedules are temporarily unavailable"}]}`)
	}
	verifyUnchanged := func() (*http.Response, error) { return jsonResponse(http.StatusOK, unchangedEvent) }

	tests := []struct {
		name    string
		patch   response
		verify  response
		delete  response
		wantErr []string
	}{
		{
			name:    "deletes created event when schedule update fails",
			patch:   patchConflict,
			verify:  verifyUnchanged,
			delete:  func() (*http.Response, error) { return jsonResponse(http.StatusNoContent, ``) },
			wantErr: []string{`failed to apply schedule after creating event "event-1"`, "verification confirmed the schedule was not applied and the event was deleted so the command is safe to retry"},
		},
		{
			name:   "reports cleanup failure when delete fails",
			patch:  patchConflict,
			verify: verifyUnchanged,
			delete: func() (*http.Response, error) {
				return jsonResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"UNEXPECTED_ERROR","detail":"cleanup failed"}]}`)
			},
			wantErr: []string{`created event "event-1" but failed to apply schedule`, "verification confirmed the schedule is still missing", "cleanup also failed"},
		},
		{
			name:    "leaves event in place when outcome is ambiguous",
			patch:   func() (*http.Response, error) { return nil, errors.New("connection reset by peer") },
			verify:  func() (*http.Response, error) { return nil, errors.New("verification timed out") },
			wantErr: []string{`created event "event-1" but could not confirm whether the schedule update succeeded`, "verification failed"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newAppEventsTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodPost && req.URL.Path != "/v1/appEvents/event-1" {
					t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
				}
				switch req.Method {
				case http.MethodPost:
					return jsonResponse(http.StatusCreated, unchangedEvent)
				case http.MethodPatch:
					return test.patch()
				case http.MethodGet:
					return test.verify()
				case http.MethodDelete:
					if test.delete == nil {
						t.Fatal("did not expect delete for ambiguous update result")
					}
					return test.delete()
				default:
					t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
					return nil, nil
				}
			}))
			stdout, stderr, err := runRootWithDeps(t, func() (*asc.Client, error) { return client, nil },
				"app-events", "create",
				"--app", "app-123",
				"--name", "Launch",
				"--event-type", "CHALLENGE",
				"--start", "2026-06-01T00:00:00Z",
				"--end", "2026-06-30T23:59:59Z",
				"--territories", "USA",
			)
			if err == nil {
				t.Fatal("expected create to fail")
			}
			for _, want := range test.wantErr {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("expected error to contain %q, got %v", want, err)
				}
			}

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	}
}

func TestAppEventsCreateTerritoriesAllUsesAppAvailability(t *testing.T) {
	const availabilityPath = "/v1/apps/app-123/appAvailabilityV2"
	const territoriesPath = "/v2/appAvailabilities/availability-1/territoryAvailabilities"
	territory := func(id string, available bool) string {
		return fmt.Sprintf(`{"type":"territoryAvailabilities","id":"ta-%s","attributes":{"available":%t},"relationships":{"territory":{"data":{"type":"territories","id":"%s"}}}}`, id, available, id)
	}

	tests := []struct {
		name            string
		territories     string
		wantRequests    []string
		wantTerritories []string
		wantErr         string
	}{
		{
			name:            "available territories become the schedule",
			territories:     `{"data":[` + territory("USA", true) + `,` + territory("FRA", false) + `,` + territory("CAN", true) + `],"links":{"next":""}}`,
			wantRequests:    []string{"GET " + availabilityPath, "GET " + territoriesPath, "POST /v1/appEvents", "PATCH /v1/appEvents/event-1"},
			wantTerritories: []string{"CAN", "USA"},
		},
		{
			name:         "app with no available territories creates nothing",
			territories:  `{"data":[` + territory("FRA", false) + `],"links":{"next":""}}`,
			wantRequests: []string{"GET " + availabilityPath, "GET " + territoriesPath},
			wantErr:      `app-events create: --territories all: app "app-123" is not available in any territory`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var requests []string
			var gotTerritories []string
			client := newAppEventsTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests = append(requests, req.Method+" "+req.URL.Path)
				switch req.Method + " " + req.URL.Path {
				case "GET " + availabilityPath:
					return jsonResponse(http.StatusOK, `{"data":{"type":"appAvailabilities","id":"availability-1"}}`)
				case "GET " + territoriesPath:
					return jsonResponse(http.StatusOK, test.territories)
				case "POST /v1/appEvents":
					return jsonResponse(http.StatusCreated, `{"data":{"type":"appEvents","id":"event-1","attributes":{"referenceName":"Launch"}}}`)
				case "PATCH /v1/appEvents/event-1":
					var body struct {
						Data struct {
							Attributes struct {
								TerritorySchedules []asc.AppEventTerritorySchedule `json:"territorySchedules"`
							} `json:"attributes"`
						} `json:"data"`
					}
					if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
						t.Fatalf("failed to decode schedule update: %v", err)
					}
					if len(body.Data.Attributes.TerritorySchedules) != 1 {
						t.Fatalf("territorySchedules = %#v, want one schedule", body.Data.Attributes.TerritorySchedules)
					}
					gotTerritories = body.Data.Attributes.TerritorySchedules[0].Territories
					return jsonResponse(http.StatusOK, `{"data":{"type":"appEvents","id":"event-1","attributes":{"referenceName":"Launch"}}}`)
				default:
					t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
					return nil, nil
				}
			}))

			stdout, stderr, err := runRootWithDeps(t, func() (*asc.Client, error) { return client, nil },
				"app-events", "create",
				"--app", "app-123",
				"--name", "Launch",
				"--start", "2026-06-01T00:00:00Z",
				"--end", "2026-06-30T23:59:59Z",
				"--territories", "all",
			)
			if !slices.Equal(requests, test.wantRequests) {
				t.Fatalf("requests = %q, want %q", requests, test.wantRequests)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if test.wantErr != "" {
				if err == nil || err.Error() != test.wantErr {
					t.Fatalf("error = %v, want %q", err, test.wantErr)
				}
				if stdout != "" {
					t.Fatalf("expected empty stdout, got %q", stdout)
				}
				return
			}
			if err != nil {
				t.Fatalf("create failed: %v", err)
			}
			if !slices.Equal(gotTerritories, test.wantTerritories) {
				t.Fatalf("schedule territories = %q, want %q", gotTerritories, test.wantTerritories)
			}
		})
	}
}

func TestAppEventsCreateSucceedsWhenVerificationShowsScheduleApplied(t *testing.T) {
	tests := []struct {
		name            string
		territoriesFlag string
		verified        string
		wantStart       string
		wantEnd         string
		wantPublish     string
		wantTerritories []string
	}{
		{
			name:            "exact match",
			territoriesFlag: "USA",
			verified:        `{"territories":["USA"],"publishStart":"2026-05-15T00:00:00Z","eventStart":"2026-06-01T00:00:00Z","eventEnd":"2026-06-30T23:59:59Z"}`,
			wantStart:       "2026-06-01T00:00:00Z",
			wantEnd:         "2026-06-30T23:59:59Z",
			wantPublish:     "2026-05-15T00:00:00Z",
			wantTerritories: []string{"USA"},
		},
		{
			name:            "reordered territories",
			territoriesFlag: "USA, CAN",
			verified:        `{"territories":["CAN","USA"],"publishStart":"2026-05-15T00:00:00Z","eventStart":"2026-06-01T00:00:00Z","eventEnd":"2026-06-30T23:59:59Z"}`,
			wantStart:       "2026-06-01T00:00:00Z",
			wantEnd:         "2026-06-30T23:59:59Z",
			wantPublish:     "2026-05-15T00:00:00Z",
			wantTerritories: []string{"CAN", "USA"},
		},
		{
			name:            "canonicalized timestamps",
			territoriesFlag: "USA, CAN",
			verified:        `{"territories":["USA","CAN"],"publishStart":"2026-05-15T02:00:00+02:00","eventStart":"2026-06-01T02:00:00+02:00","eventEnd":"2026-07-01T01:59:59+02:00"}`,
			wantStart:       "2026-06-01T02:00:00+02:00",
			wantEnd:         "2026-07-01T01:59:59+02:00",
			wantPublish:     "2026-05-15T02:00:00+02:00",
			wantTerritories: []string{"USA", "CAN"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newAppEventsTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch req.Method {
				case http.MethodPost:
					return jsonResponse(http.StatusCreated, `{"data":{"type":"appEvents","id":"event-1","attributes":{"referenceName":"Launch","badge":"CHALLENGE"}}}`)
				case http.MethodPatch:
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{"data":`)),
						Request:    req,
					}, nil
				case http.MethodGet:
					return jsonResponse(http.StatusOK, `{"data":{"type":"appEvents","id":"event-1","attributes":{"referenceName":"Launch","badge":"CHALLENGE","territorySchedules":[`+test.verified+`]}}}`)
				default:
					t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
					return nil, nil
				}
			}))

			stdout, stderr, err := runRootWithDeps(t, func() (*asc.Client, error) { return client, nil },
				"app-events", "create",
				"--app", "app-123",
				"--name", "Launch",
				"--event-type", "CHALLENGE",
				"--start", "2026-06-01T00:00:00Z",
				"--end", "2026-06-30T23:59:59Z",
				"--publish-start", "2026-05-15T00:00:00Z",
				"--territories", test.territoriesFlag,
			)
			if err != nil {
				t.Fatalf("run error: %v", err)
			}

			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}

			var resp asc.AppEventResponse
			if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
				t.Fatalf("failed to parse JSON output: %v", err)
			}
			if !appEventResponseHasSingleSchedule(resp, test.wantStart, test.wantEnd, test.wantPublish, test.wantTerritories) {
				t.Fatalf("expected verified response to preserve schedule, got %#v", resp.Data.Attributes.TerritorySchedules)
			}
		})
	}
}

func appEventResponseHasSingleSchedule(resp asc.AppEventResponse, start, end, publishStart string, territories []string) bool {
	if len(resp.Data.Attributes.TerritorySchedules) != 1 {
		return false
	}
	schedule := resp.Data.Attributes.TerritorySchedules[0]
	if schedule.EventStart != start || schedule.EventEnd != end || schedule.PublishStart != publishStart {
		return false
	}
	if len(schedule.Territories) != len(territories) {
		return false
	}
	for i, territory := range territories {
		if schedule.Territories[i] != territory {
			return false
		}
	}
	return true
}

func TestAppEventsCreateScheduleFlagsStillValidateRFC3339(t *testing.T) {
	stdout, stderr, err := runRootWithDeps(t, func() (*asc.Client, error) {
		t.Fatal("did not expect client creation for invalid schedule flags")
		return nil, nil
	},
		"app-events", "create",
		"--app", "app-123",
		"--name", "Launch",
		"--event-type", "CHALLENGE",
		"--start", "not-a-date",
		"--end", "2026-06-30T23:59:59Z",
	)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", err)
	}

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--start must be in RFC3339 format") {
		t.Fatalf("expected RFC3339 validation error, got %q", stderr)
	}
}

func TestAppEventsSubmitReportsPartialCreateWithoutAddingOrSubmitting(t *testing.T) {
	requests := 0
	client := newAppEventsTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if requests == 1 && req.Method == http.MethodGet && req.URL.Path == "/v1/appEvents/event-1" {
			return jsonResponse(http.StatusOK, `{"data":{"type":"appEvents","id":"event-1","attributes":{"deepLink":"https://example.com/event"}}}`)
		}
		if req.Method != http.MethodPost || req.URL.Path != "/v1/reviewSubmissions" {
			t.Fatalf("unexpected request %d: %s %s", requests, req.Method, req.URL.Path)
		}
		return jsonResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"sub-partial"},"errors":[{"status":"500","detail":"partial response"}]}`)
	}))

	stdout, stderr, runErr := runRootWithDeps(t, func() (*asc.Client, error) { return client, nil },
		"app-events", "submit",
		"--event-id", "event-1",
		"--app", "app-123",
		"--confirm",
	)

	// The partial submission is an empty READY_FOR_REVIEW draft, which App
	// Store Connect refuses to cancel, so the guidance must finish it instead.
	if runErr == nil || !strings.Contains(runErr.Error(), "sub-partial") ||
		!strings.Contains(runErr.Error(), "asc review items add --submission sub-partial --item-type appEvents --item-id event-1") ||
		!strings.Contains(runErr.Error(), "asc review submissions-submit --id sub-partial --confirm") ||
		strings.Contains(runErr.Error(), "submit cancel") {
		t.Fatalf("run error = %v, want partial submission ID and draft guidance without a cancel", runErr)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want the event lookup and the create request", requests)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("stdout = %q, stderr = %q, want no command output", stdout, stderr)
	}
}

func TestAppEventsSubmitWarnsWhenEventHasNoDeepLinkAndStillSubmits(t *testing.T) {
	const deepLinkWarning = `Warning: app-events submit: event "event-1" has no deep link; App Store Connect asks for one so Open on the event card leads to the event in your app. Set it while the event is editable. After submitting, remove the event from review before updating it. Update with: asc app-events update --event-id event-1 --deep-link "URL"` + "\n"

	tests := []struct {
		name       string
		lookup     func() (*http.Response, error)
		wantStderr string
	}{
		{
			name: "event with deep link submits without a warning",
			lookup: func() (*http.Response, error) {
				return jsonResponse(http.StatusOK, `{"data":{"type":"appEvents","id":"event-1","attributes":{"deepLink":"https://example.com/event"}}}`)
			},
		},
		{
			name: "event without deep link warns and still submits",
			lookup: func() (*http.Response, error) {
				return jsonResponse(http.StatusOK, `{"data":{"type":"appEvents","id":"event-1","attributes":{"referenceName":"Launch"}}}`)
			},
			wantStderr: deepLinkWarning,
		},
		{
			name: "failed event lookup does not block submission",
			lookup: func() (*http.Response, error) {
				return jsonResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"UNEXPECTED_ERROR","detail":"lookup failed"}]}`)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requests := 0
			client := newAppEventsTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				switch requests {
				case 1:
					if req.Method != http.MethodGet || req.URL.Path != "/v1/appEvents/event-1" {
						t.Fatalf("unexpected event lookup: %s %s", req.Method, req.URL.Path)
					}
					return test.lookup()
				case 2:
					if req.Method != http.MethodPost || req.URL.Path != "/v1/reviewSubmissions" {
						t.Fatalf("unexpected create request: %s %s", req.Method, req.URL.Path)
					}
					return jsonResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"sub-clean"}}`)
				case 3:
					if req.Method != http.MethodPost || req.URL.Path != "/v1/reviewSubmissionItems" {
						t.Fatalf("unexpected item request: %s %s", req.Method, req.URL.Path)
					}
					return jsonResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"item-clean"}}`)
				case 4:
					if req.Method != http.MethodPatch || req.URL.Path != "/v1/reviewSubmissions/sub-clean" {
						t.Fatalf("unexpected submit request: %s %s", req.Method, req.URL.Path)
					}
					return jsonResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"sub-clean","attributes":{"submittedDate":"2026-09-18T00:00:00Z"}}}`)
				default:
					t.Fatalf("unexpected extra request %d: %s %s", requests, req.Method, req.URL.Path)
					return nil, nil
				}
			}))

			stdout, stderr, err := runRootWithDeps(t, func() (*asc.Client, error) { return client, nil },
				"app-events", "submit",
				"--event-id", "event-1",
				"--app", "app-123",
				"--confirm",
			)
			if err != nil {
				t.Fatalf("run error: %v", err)
			}

			if requests != 4 {
				t.Fatalf("requests = %d, want lookup, create, add, and submit", requests)
			}
			var result struct {
				SubmissionID string `json:"submissionId"`
				EventID      string `json:"eventId"`
			}
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatalf("failed to parse JSON output %q: %v", stdout, err)
			}
			if result.SubmissionID != "sub-clean" || result.EventID != "event-1" {
				t.Fatalf("output = %+v, want submission sub-clean for event event-1", result)
			}
			if stderr != test.wantStderr {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
			}
		})
	}
}

func TestAppEventsSubmitAdvisoryTimeoutPreservesSubmissionBudget(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "1s")
	t.Setenv("ASC_MAX_RETRIES", "0")
	synctest.Test(t, func(t *testing.T) {
		writes := 0
		client := newAppEventsTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method == http.MethodGet {
				<-req.Context().Done()
				return nil, req.Context().Err()
			}
			if err := req.Context().Err(); err != nil {
				return nil, err
			}
			writes++
			deadline, ok := req.Context().Deadline()
			if !ok || time.Until(deadline) != time.Second {
				t.Fatalf("submission did not get a fresh timeout: %s", time.Until(deadline))
			}
			return jsonResponse(http.StatusForbidden, `{"errors":[{"status":"403","code":"FORBIDDEN_ERROR","title":"Forbidden"}]}`)
		}))
		started := time.Now()
		_, _, err := runRootWithDeps(t, func() (*asc.Client, error) { return client, nil },
			"app-events", "submit", "--event-id", "event-1", "--app", "123", "--confirm")
		if !errors.Is(err, asc.ErrForbidden) || writes != 1 {
			t.Fatalf("error=%v, writes=%d; want forbidden after one submission request", err, writes)
		}
		if elapsed := time.Since(started); elapsed != time.Second {
			t.Fatalf("lookup did not honor the shorter configured timeout: %s", elapsed)
		}
	})
}
