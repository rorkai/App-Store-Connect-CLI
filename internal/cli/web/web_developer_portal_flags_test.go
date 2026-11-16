package web

import (
	"context"
	"flag"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"testing"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebAppGroupsCreateUnknownDeveloperTeamFailsBeforeMutation(t *testing.T) {
	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "1ms")

	origResolve := resolveSessionFn
	origNewClient := newWebClientFn
	origCreate := createDeveloperAppGroupFn
	origPersist := persistWebSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolve
		newWebClientFn = origNewClient
		createDeveloperAppGroupFn = origCreate
		persistWebSessionFn = origPersist
	})

	var mutationHits int
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	session := &webcore.AuthSession{
		UserEmail: "user@example.com",
		Client: &http.Client{
			Jar: jar,
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path == "/services-account/QH65B2/account/listTeams.action" {
					return &http.Response{
						StatusCode: http.StatusOK,
						Header: http.Header{
							"Content-Type": []string{"application/json"},
							"csrf":         []string{"csrf-secret-DO-NOT-LEAK-abc123"},
							"csrf_ts":      []string{"csrf-ts"},
							"Set-Cookie":   []string{"myacinfo=session-cookie-DO-NOT-LEAK-xyz789; Path=/; Secure"},
						},
						Body: io.NopCloser(strings.NewReader(`{"teams":[{"teamId":"TEAMONE123","name":"Example One"},{"teamId":"TEAMTWO456","name":"Example Two"}]}`)),
					}, nil
				}
				mutationHits++
				t.Errorf("mutation request %s %s must not run", r.Method, r.URL.Path)
				return &http.Response{StatusCode: http.StatusInternalServerError, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
			}),
		},
	}
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		return session, "cache", nil
	}
	newWebClientFn = webcore.NewClient
	persistWebSessionFn = func(*webcore.AuthSession) error { return nil }

	command := WebAppGroupsCreateCommand()
	if err := command.FlagSet.Parse([]string{"--name", "X", "--identifier", "group.x", "--confirm", "--developer-team", "NOPE", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var runErr error
	stdout, stderr := captureWebCommandOutput(t, func() {
		runErr = command.Exec(context.Background(), nil)
	})
	if runErr == nil {
		t.Fatal("expected unknown Developer Portal team error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	combined := runErr.Error() + "\n" + stderr
	for _, want := range []string{"NOPE", "TEAMONE123", "Example One", "--developer-team"} {
		if !strings.Contains(combined, want) {
			t.Fatalf("output %q does not contain %q", combined, want)
		}
	}
	if strings.Contains(combined, "csrf-secret-DO-NOT-LEAK-abc123") || strings.Contains(combined, "session-cookie-DO-NOT-LEAK-xyz789") {
		t.Fatalf("output leaked CSRF or cookie secret: %q", combined)
	}
	if mutationHits != 0 {
		t.Fatalf("mutation requests = %d, want 0", mutationHits)
	}
}

func TestWebDeveloperTeamFlagAcceptedOnPortalCommands(t *testing.T) {
	commands := []struct {
		name    string
		command func() *flag.FlagSet
	}{
		{"bundle-ids capabilities enable", func() *flag.FlagSet {
			d, _, _ := newWebTestDeps()
			return newWebBundleIDCapabilitiesEnableCommand(d).FlagSet
		}},
		{"app-groups list", func() *flag.FlagSet { return WebAppGroupsListCommand().FlagSet }},
		{"app-groups create", func() *flag.FlagSet { return WebAppGroupsCreateCommand().FlagSet }},
		{"agreements status", func() *flag.FlagSet { return WebAgreementsStatusCommand().FlagSet }},
	}
	for _, test := range commands {
		t.Run(test.name, func(t *testing.T) {
			if test.command().Lookup("developer-team") == nil {
				t.Fatal("expected --developer-team flag")
			}
		})
	}
}

func TestWebDeveloperTeamPersistedAndReusedBySecondCommand(t *testing.T) {
	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "1ms")

	origResolve := resolveSessionFn
	origNewClient := newWebClientFn
	origList := listDeveloperAppGroupsFn
	origPersist := persistWebSessionFn
	t.Cleanup(func() {
		resolveSessionFn = origResolve
		newWebClientFn = origNewClient
		listDeveloperAppGroupsFn = origList
		persistWebSessionFn = origPersist
	})

	var listedTeamIDs []string
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	session := &webcore.AuthSession{
		UserEmail: "user@example.com",
		Client: &http.Client{
			Jar: jar,
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				header := http.Header{"Content-Type": []string{"application/json"}}
				switch {
				case r.URL.Path == "/services-account/QH65B2/account/listTeams.action":
					header.Set("csrf", "token")
					header.Set("csrf_ts", "ts")
					return &http.Response{StatusCode: http.StatusOK, Header: header, Body: io.NopCloser(strings.NewReader(`{"teams":[{"teamId":"TEAMONE123","name":"Example One"},{"teamId":"TEAMTWO456","name":"Example Two"}]}`))}, nil
				case strings.Contains(r.URL.Path, "listApplicationGroups.action"):
					if err := r.ParseForm(); err != nil {
						t.Errorf("ParseForm: %v", err)
					}
					listedTeamIDs = append(listedTeamIDs, r.PostForm.Get("teamId"))
					return &http.Response{StatusCode: http.StatusOK, Header: header, Body: io.NopCloser(strings.NewReader(`{"resultCode":0,"pageNumber":1,"pageSize":500,"totalRecords":0,"applicationGroupList":[]}`))}, nil
				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
					return &http.Response{StatusCode: http.StatusNotFound, Header: header, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
				}
			}),
		},
	}
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		return session, "cache", nil
	}
	newWebClientFn = webcore.NewClient
	var persisted []*webcore.AuthSession
	persistWebSessionFn = func(session *webcore.AuthSession) error {
		persisted = append(persisted, session)
		return nil
	}

	first := WebAppGroupsListCommand()
	if err := first.FlagSet.Parse([]string{"--developer-team", "TEAMTWO456", "--output", "json"}); err != nil {
		t.Fatalf("parse first: %v", err)
	}
	if _, stderr := captureWebCommandOutput(t, func() {
		if err := first.Exec(context.Background(), nil); err != nil {
			t.Fatalf("first list: %v", err)
		}
	}); stderr != "" && !strings.Contains(stderr, "Warning:") {
		t.Fatalf("unexpected stderr from first list: %q", stderr)
	}
	if len(listedTeamIDs) != 1 || listedTeamIDs[0] != "TEAMTWO456" {
		t.Fatalf("first list teamIds = %v", listedTeamIDs)
	}
	if len(persisted) == 0 || strings.TrimSpace(session.DeveloperTeamID) != "TEAMTWO456" {
		t.Fatalf("expected persisted DeveloperTeamID TEAMTWO456, got %q after %d persist calls", session.DeveloperTeamID, len(persisted))
	}

	second := WebAppGroupsListCommand()
	if err := second.FlagSet.Parse([]string{"--output", "json"}); err != nil {
		t.Fatalf("parse second: %v", err)
	}
	if _, errStdout := captureWebCommandOutput(t, func() {
		if err := second.Exec(context.Background(), nil); err != nil {
			t.Fatalf("second list: %v", err)
		}
	}); errStdout != "" && !strings.Contains(errStdout, "Warning:") {
		t.Fatalf("unexpected stderr from second list: %q", errStdout)
	}
	if len(listedTeamIDs) != 2 || listedTeamIDs[1] != "TEAMTWO456" {
		t.Fatalf("second list should reuse persisted team, got %v", listedTeamIDs)
	}
}

func TestValidateDeveloperPortalFlagsRejectsBlankSelector(t *testing.T) {
	omitted := bindDeveloperPortalFlags(flag.NewFlagSet("omit", flag.ContinueOnError))
	if err := validateDeveloperPortalFlags(omitted); err != nil {
		t.Fatalf("omitted flag: %v", err)
	}

	blank := bindDeveloperPortalFlags(flag.NewFlagSet("blank", flag.ContinueOnError))
	if err := blank.fs.Parse([]string{"--developer-team", ""}); err != nil {
		t.Fatalf("parse blank: %v", err)
	}
	if err := validateDeveloperPortalFlags(blank); err == nil || !strings.Contains(err.Error(), "--developer-team must be") {
		t.Fatalf("blank selector error = %v", err)
	}

	spaces := bindDeveloperPortalFlags(flag.NewFlagSet("spaces", flag.ContinueOnError))
	if err := spaces.fs.Parse([]string{"--developer-team", "   "}); err != nil {
		t.Fatalf("parse spaces: %v", err)
	}
	if err := validateDeveloperPortalFlags(spaces); err == nil {
		t.Fatal("expected whitespace-only --developer-team to fail")
	}
}
