package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestUsersUpdateWarnsDeprecatedAccessToReportsRole(t *testing.T) {
	setupAuth(t)

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPatch || req.URL.Path != "/v1/users/user-1" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
		return jsonResponse(http.StatusOK, `{"data":{"type":"users","id":"user-1","attributes":{"username":"user@example.com","roles":["ACCESS_TO_REPORTS"],"allAppsVisible":true,"provisioningAllowed":false}}}`)
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"users", "update",
			"--id", "user-1",
			"--roles", "ACCESS_TO_REPORTS",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("run error: %v; stderr=%q stdout=%q", runErr, stderr, stdout)
	}
	requireStderrContainsWarning(t, stderr, "Warning: ACCESS_TO_REPORTS is deprecated in App Store Connect API 4.4")

	var got struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v; stdout=%q", err, stdout)
	}
	if got.Data.ID != "user-1" {
		t.Fatalf("expected user-1, got %#v", got.Data)
	}
}

func TestUsersInviteWarnsDeprecatedAccessToReportsRole(t *testing.T) {
	setupAuth(t)

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/v1/userInvitations" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
		return jsonResponse(http.StatusOK, `{"data":{"type":"userInvitations","id":"invite-1","attributes":{"email":"user@example.com","roles":["ACCESS_TO_REPORTS"],"allAppsVisible":true,"provisioningAllowed":false}}}`)
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"users", "invite",
			"--email", "user@example.com",
			"--first-name", "Jane",
			"--last-name", "Doe",
			"--roles", "ACCESS_TO_REPORTS",
			"--all-apps",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("run error: %v; stderr=%q stdout=%q", runErr, stderr, stdout)
	}
	requireStderrContainsWarning(t, stderr, "Warning: ACCESS_TO_REPORTS is deprecated in App Store Connect API 4.4")

	var got struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v; stdout=%q", err, stdout)
	}
	if got.Data.ID != "invite-1" {
		t.Fatalf("expected invite-1, got %#v", got.Data)
	}
}
