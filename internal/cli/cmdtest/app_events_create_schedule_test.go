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

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	appeventscli "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/app_events"
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

	restore := appeventscli.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"app-events", "create",
			"--app", "app-123",
			"--name", "Launch",
			"--event-type", "CHALLENGE",
			"--start", "2026-06-01T00:00:00Z",
			"--end", "2026-06-30T23:59:59Z",
			"--publish-start", "2026-05-15T00:00:00Z",
			"--territories", "usa, can",
			"--primary-locale", "en-US",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

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

func TestAppEventsCreateDeletesCreatedEventWhenScheduleUpdateFails(t *testing.T) {
	client := newAppEventsTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.Method {
		case http.MethodPost:
			return jsonResponse(http.StatusCreated, `{"data":{"type":"appEvents","id":"event-1","attributes":{"referenceName":"Launch","badge":"CHALLENGE"}}}`)
		case http.MethodPatch:
			if req.URL.Path != "/v1/appEvents/event-1" {
				t.Fatalf("expected update path /v1/appEvents/event-1, got %s", req.URL.Path)
			}
			return jsonResponse(http.StatusConflict, `{"errors":[{"status":"409","code":"ENTITY_ERROR.ATTRIBUTE.INVALID","detail":"territorySchedules are temporarily unavailable"}]}`)
		case http.MethodGet:
			if req.URL.Path != "/v1/appEvents/event-1" {
				t.Fatalf("expected verify path /v1/appEvents/event-1, got %s", req.URL.Path)
			}
			return jsonResponse(http.StatusOK, `{"data":{"type":"appEvents","id":"event-1","attributes":{"referenceName":"Launch","badge":"CHALLENGE"}}}`)
		case http.MethodDelete:
			if req.URL.Path != "/v1/appEvents/event-1" {
				t.Fatalf("expected delete path /v1/appEvents/event-1, got %s", req.URL.Path)
			}
			return jsonResponse(http.StatusNoContent, ``)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	}))

	restore := appeventscli.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"app-events", "create",
			"--app", "app-123",
			"--name", "Launch",
			"--event-type", "CHALLENGE",
			"--start", "2026-06-01T00:00:00Z",
			"--end", "2026-06-30T23:59:59Z",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}

		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected create to fail when schedule update fails")
		}
		if !strings.Contains(err.Error(), `failed to apply schedule after creating event "event-1"`) {
			t.Fatalf("expected partial-failure context, got %v", err)
		}
		if !strings.Contains(err.Error(), "verification confirmed the schedule was not applied and the event was deleted so the command is safe to retry") {
			t.Fatalf("expected safe-to-retry remediation, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout on partial failure, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr on partial failure, got %q", stderr)
	}
}

func TestAppEventsCreateReportsCleanupFailureWhenDeleteFails(t *testing.T) {
	client := newAppEventsTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.Method {
		case http.MethodPost:
			return jsonResponse(http.StatusCreated, `{"data":{"type":"appEvents","id":"event-1","attributes":{"referenceName":"Launch","badge":"CHALLENGE"}}}`)
		case http.MethodPatch:
			return jsonResponse(http.StatusConflict, `{"errors":[{"status":"409","code":"ENTITY_ERROR.ATTRIBUTE.INVALID","detail":"territorySchedules are temporarily unavailable"}]}`)
		case http.MethodGet:
			return jsonResponse(http.StatusOK, `{"data":{"type":"appEvents","id":"event-1","attributes":{"referenceName":"Launch","badge":"CHALLENGE"}}}`)
		case http.MethodDelete:
			return jsonResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"UNEXPECTED_ERROR","detail":"cleanup failed"}]}`)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	}))

	restore := appeventscli.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"app-events", "create",
			"--app", "app-123",
			"--name", "Launch",
			"--event-type", "CHALLENGE",
			"--start", "2026-06-01T00:00:00Z",
			"--end", "2026-06-30T23:59:59Z",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}

		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected create to fail when cleanup fails")
		}
		if !strings.Contains(err.Error(), `created event "event-1" but failed to apply schedule`) {
			t.Fatalf("expected created-event context, got %v", err)
		}
		if !strings.Contains(err.Error(), "verification confirmed the schedule is still missing") {
			t.Fatalf("expected verification context, got %v", err)
		}
		if !strings.Contains(err.Error(), "cleanup also failed") {
			t.Fatalf("expected cleanup failure context, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout on cleanup failure, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr on cleanup failure, got %q", stderr)
	}
}

func TestAppEventsCreateSucceedsWhenVerificationShowsScheduleApplied(t *testing.T) {
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
			return jsonResponse(http.StatusOK, `{"data":{"type":"appEvents","id":"event-1","attributes":{"referenceName":"Launch","badge":"CHALLENGE","territorySchedules":[{"territories":["USA"],"publishStart":"2026-05-15T00:00:00Z","eventStart":"2026-06-01T00:00:00Z","eventEnd":"2026-06-30T23:59:59Z"}]}}}`)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	}))

	restore := appeventscli.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"app-events", "create",
			"--app", "app-123",
			"--name", "Launch",
			"--event-type", "CHALLENGE",
			"--start", "2026-06-01T00:00:00Z",
			"--end", "2026-06-30T23:59:59Z",
			"--publish-start", "2026-05-15T00:00:00Z",
			"--territories", "USA",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var resp asc.AppEventResponse
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if !appEventResponseHasSingleSchedule(resp, "2026-06-01T00:00:00Z", "2026-06-30T23:59:59Z", "2026-05-15T00:00:00Z", []string{"USA"}) {
		t.Fatalf("expected verified response to preserve schedule, got %#v", resp.Data.Attributes.TerritorySchedules)
	}
}

func TestAppEventsCreateSucceedsWhenVerificationReordersTerritories(t *testing.T) {
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
			return jsonResponse(http.StatusOK, `{"data":{"type":"appEvents","id":"event-1","attributes":{"referenceName":"Launch","badge":"CHALLENGE","territorySchedules":[{"territories":["CAN","USA"],"publishStart":"2026-05-15T00:00:00Z","eventStart":"2026-06-01T00:00:00Z","eventEnd":"2026-06-30T23:59:59Z"}]}}}`)
		case http.MethodDelete:
			t.Fatal("did not expect delete when verification only reorders territories")
			return nil, nil
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	}))

	restore := appeventscli.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"app-events", "create",
			"--app", "app-123",
			"--name", "Launch",
			"--event-type", "CHALLENGE",
			"--start", "2026-06-01T00:00:00Z",
			"--end", "2026-06-30T23:59:59Z",
			"--publish-start", "2026-05-15T00:00:00Z",
			"--territories", "USA, CAN",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var resp asc.AppEventResponse
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if !appEventResponseHasSingleSchedule(resp, "2026-06-01T00:00:00Z", "2026-06-30T23:59:59Z", "2026-05-15T00:00:00Z", []string{"CAN", "USA"}) {
		t.Fatalf("expected verified response to preserve reordered territories, got %#v", resp.Data.Attributes.TerritorySchedules)
	}
}

func TestAppEventsCreateSucceedsWhenVerificationCanonicalizesScheduleTimes(t *testing.T) {
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
			return jsonResponse(http.StatusOK, `{"data":{"type":"appEvents","id":"event-1","attributes":{"referenceName":"Launch","badge":"CHALLENGE","territorySchedules":[{"territories":["USA","CAN"],"publishStart":"2026-05-15T02:00:00+02:00","eventStart":"2026-06-01T02:00:00+02:00","eventEnd":"2026-07-01T01:59:59+02:00"}]}}}`)
		case http.MethodDelete:
			t.Fatal("did not expect delete when verification only canonicalizes timestamp formatting")
			return nil, nil
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	}))

	restore := appeventscli.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"app-events", "create",
			"--app", "app-123",
			"--name", "Launch",
			"--event-type", "CHALLENGE",
			"--start", "2026-06-01T00:00:00Z",
			"--end", "2026-06-30T23:59:59Z",
			"--publish-start", "2026-05-15T00:00:00Z",
			"--territories", "USA, CAN",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var resp asc.AppEventResponse
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if !appEventResponseHasSingleSchedule(resp, "2026-06-01T02:00:00+02:00", "2026-07-01T01:59:59+02:00", "2026-05-15T02:00:00+02:00", []string{"USA", "CAN"}) {
		t.Fatalf("expected verified response to preserve canonicalized timestamps, got %#v", resp.Data.Attributes.TerritorySchedules)
	}
}

func TestAppEventsCreateLeavesEventInPlaceWhenScheduleOutcomeIsAmbiguous(t *testing.T) {
	client := newAppEventsTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.Method {
		case http.MethodPost:
			return jsonResponse(http.StatusCreated, `{"data":{"type":"appEvents","id":"event-1","attributes":{"referenceName":"Launch","badge":"CHALLENGE"}}}`)
		case http.MethodPatch:
			return nil, errors.New("connection reset by peer")
		case http.MethodGet:
			return nil, errors.New("verification timed out")
		case http.MethodDelete:
			t.Fatal("did not expect delete for ambiguous update result")
			return nil, nil
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	}))

	restore := appeventscli.SetClientFactory(func() (*asc.Client, error) {
		return client, nil
	})
	defer restore()

	root := RootCommand("1.2.3")
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"app-events", "create",
			"--app", "app-123",
			"--name", "Launch",
			"--event-type", "CHALLENGE",
			"--start", "2026-06-01T00:00:00Z",
			"--end", "2026-06-30T23:59:59Z",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}

		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected create to fail when verification is ambiguous")
		}
		if !strings.Contains(err.Error(), `created event "event-1" but could not confirm whether the schedule update succeeded`) {
			t.Fatalf("expected ambiguous outcome context, got %v", err)
		}
		if !strings.Contains(err.Error(), "verification failed") {
			t.Fatalf("expected verification failure context, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout on ambiguous failure, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr on ambiguous failure, got %q", stderr)
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
	restore := appeventscli.SetClientFactory(func() (*asc.Client, error) {
		t.Fatal("did not expect client creation for invalid schedule flags")
		return nil, nil
	})
	defer restore()

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"app-events", "create",
			"--app", "app-123",
			"--name", "Launch",
			"--event-type", "CHALLENGE",
			"--start", "not-a-date",
			"--end", "2026-06-30T23:59:59Z",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--start must be in RFC3339 format") {
		t.Fatalf("expected RFC3339 validation error, got %q", stderr)
	}
}
