package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

type buildGroupMembershipOutput struct {
	BuildID      string                        `json:"buildId"`
	AppID        string                        `json:"appId"`
	Complete     bool                          `json:"complete"`
	LookupMethod string                        `json:"lookupMethod"`
	GroupCount   int                           `json:"groupCount"`
	Groups       []buildGroupMembershipRecord  `json:"groups"`
	Failures     []buildGroupMembershipFailure `json:"failures"`
}

type buildGroupMembershipRecord struct {
	ID                   string `json:"id"`
	Name                 string `json:"name"`
	Type                 string `json:"type"`
	Membership           string `json:"membership"`
	HasAccessToAllBuilds bool   `json:"hasAccessToAllBuilds"`
}

type buildGroupMembershipFailure struct {
	GroupID string `json:"groupId"`
	Error   string `json:"error"`
}

func TestBuildsGroupsListCommandHasNarrowBuildSurface(t *testing.T) {
	root := RootCommand("1.2.3")
	list := findCommand(root, "builds", "groups", "list")
	if list == nil {
		t.Fatal("expected builds groups list command")
	}

	wantFlags := map[string]bool{
		"build-id": true,
		"output":   true,
		"pretty":   true,
	}
	list.FlagSet.VisitAll(func(value *flag.Flag) {
		if !wantFlags[value.Name] {
			t.Errorf("unexpected --%s flag on builds groups list", value.Name)
		}
		delete(wantFlags, value.Name)
	})
	for name := range wantFlags {
		t.Errorf("missing --%s flag on builds groups list", name)
	}

	for _, forbidden := range []string{"app", "global", "internal", "external", "limit", "next", "paginate"} {
		if list.FlagSet.Lookup(forbidden) != nil {
			t.Errorf("builds groups list must not expose --%s", forbidden)
		}
	}
}

func TestBuildsGroupsListRequiresBuildIDBeforeAuth(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_STRICT_AUTH", "")

	tests := []struct {
		name       string
		args       []string
		diagnostic string
	}{
		{name: "missing", args: nil, diagnostic: "Error: --build-id is required"},
		{name: "blank", args: []string{"--build-id", " "}, diagnostic: "Error: --build-id cannot be empty"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			var runErr error
			args := append([]string{"builds", "groups", "list"}, test.args...)
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if got := rootcmd.ExitCodeFromError(runErr); got != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.diagnostic) {
				t.Fatalf("expected %q diagnostic, got %q", test.diagnostic, stderr)
			}
			if strings.Contains(stderr, "authentication") || strings.Contains(stderr, "credentials") {
				t.Fatalf("required flag must fail before auth, got %q", stderr)
			}
		})
	}
}

func TestBuildsGroupsListUsesExistingBuildMembershipLookup(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "ambient-app-must-not-be-used")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-1/relationships/app" {
				t.Fatalf("unexpected app lookup: %s %s", req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"apps","id":"app-1"}}`), nil
		case 2:
			assertBuildMembershipGroupQuery(t, req.URL, "filter[app]", "app-1")
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"External QA","isInternalGroup":false}}]}`), nil
		case 3:
			assertBuildMembershipGroupQuery(t, req.URL, "filter[builds]", "build-1")
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"External QA","isInternalGroup":false}}]}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "groups", "list", "--output", "json", "--build-id", "build-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requestCount != 3 {
		t.Fatalf("expected three requests, got %d", requestCount)
	}

	var got buildGroupMembershipOutput
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode output: %v\n%s", err, stdout)
	}
	if got.BuildID != "build-1" || got.AppID != "app-1" || !got.Complete || got.GroupCount != 1 {
		t.Fatalf("unexpected lookup result: %#v", got)
	}
	if len(got.Groups) != 1 || got.Groups[0].ID != "group-1" || got.Groups[0].Membership != "explicit" {
		t.Fatalf("unexpected groups: %#v", got.Groups)
	}
}

func TestBuildsGroupsListMatchesTestFlightBuildMembershipSurface(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "ambient-app-must-not-be-used")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	run := func(args []string) (string, string, []string, error) {
		t.Helper()
		requests := make([]string, 0, 3)
		http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requests = append(requests, req.Method+" "+req.URL.String())
			switch len(requests) {
			case 1:
				return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"apps","id":"app-1"}}`), nil
			case 2, 3:
				return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"External QA","isInternalGroup":false}}]}`), nil
			default:
				t.Fatalf("unexpected request: %s", req.URL.String())
				return nil, nil
			}
		})

		root := RootCommand("1.2.3")
		root.FlagSet.SetOutput(io.Discard)
		var runErr error
		stdout, stderr := captureOutput(t, func() {
			if err := root.Parse(args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			runErr = root.Run(context.Background())
		})
		return stdout, stderr, requests, runErr
	}

	buildsStdout, buildsStderr, buildsRequests, buildsErr := run([]string{"builds", "groups", "list", "--build-id", "build-1"})
	testflightStdout, testflightStderr, testflightRequests, testflightErr := run([]string{"testflight", "groups", "list", "--build-id", "build-1"})

	if buildsErr != nil || testflightErr != nil {
		t.Fatalf("unexpected parity errors: builds=%v testflight=%v", buildsErr, testflightErr)
	}
	if buildsStdout != testflightStdout || buildsStderr != testflightStderr {
		t.Fatalf("surface output differs:\nbuilds stdout=%q stderr=%q\ntestflight stdout=%q stderr=%q", buildsStdout, buildsStderr, testflightStdout, testflightStderr)
	}
	if strings.Join(buildsRequests, "\n") != strings.Join(testflightRequests, "\n") {
		t.Fatalf("HTTP behavior differs:\nbuilds=%v\ntestflight=%v", buildsRequests, testflightRequests)
	}
}

func TestBuildsGroupsListPreservesLookupAPIErrors(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/builds/build-1/relationships/app" {
			t.Fatalf("unexpected request: %s", req.URL.String())
		}
		return jsonHTTPResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","title":"Temporary failure"}]}`), nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "groups", "list", "--build-id", "build-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil || !strings.Contains(runErr.Error(), "builds groups list: failed to resolve app for build") {
		t.Fatalf("unexpected error: %v", runErr)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("expected empty streams, got stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestTestFlightGroupsListBuildMembershipFlagIsExperimental(t *testing.T) {
	root := RootCommand("1.2.3")
	list := findCommand(root, "testflight", "groups", "list")
	if list == nil {
		t.Fatal("expected testflight groups list command")
	}
	buildID := list.FlagSet.Lookup("build-id")
	if buildID == nil {
		t.Fatal("expected --build-id flag")
	}
	if !strings.HasPrefix(buildID.Usage, "[experimental] ") {
		t.Fatalf("--build-id usage = %q, want [experimental] prefix", buildID.Usage)
	}
}

func TestTestFlightGroupsListBuildMembershipUsesOfficialFilterAndPaginates(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "ambient-app-must-not-be-an-assertion")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	const appNext = "https://api.appstoreconnect.apple.com/v1/betaGroups?cursor=app-next"
	const buildNext = "https://api.appstoreconnect.apple.com/v1/betaGroups?cursor=build-next"
	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-1/relationships/app" {
				t.Fatalf("unexpected app lookup: %s %s", req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"apps","id":"app-1"}}`), nil
		case 2:
			assertBuildMembershipGroupQuery(t, req.URL, "filter[app]", "app-1")
			return jsonHTTPResponse(http.StatusOK, `{
				"data":[
					{"type":"betaGroups","id":"group-explicit","attributes":{"name":"External QA","isInternalGroup":false}},
					{"type":"betaGroups","id":"group-all","attributes":{"name":"Internal All Builds","isInternalGroup":true,"hasAccessToAllBuilds":true}}
				],
				"links":{"next":"`+appNext+`"}
			}`), nil
		case 3:
			if req.URL.String() != appNext {
				t.Fatalf("unexpected app groups next URL: %s", req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{
				"data":[
					{"type":"betaGroups","id":"group-explicit","attributes":{"name":"External QA duplicate","isInternalGroup":false}},
					{"type":"betaGroups","id":"group-both","attributes":{"name":"Partner Preview","isInternalGroup":false,"hasAccessToAllBuilds":true}},
					{"type":"betaGroups","id":"group-none","attributes":{"name":"Old Group","isInternalGroup":false}}
				]
			}`), nil
		case 4:
			assertBuildMembershipGroupQuery(t, req.URL, "filter[builds]", "build-1")
			return jsonHTTPResponse(http.StatusOK, `{
				"data":[
					{"type":"betaGroups","id":"group-explicit","attributes":{"name":"External QA","isInternalGroup":false}}
				],
				"links":{"next":"`+buildNext+`"}
			}`), nil
		case 5:
			if req.URL.String() != buildNext {
				t.Fatalf("unexpected build-filter next URL: %s", req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-explicit","attributes":{"name":"duplicate"}}]}`), nil
		case 6:
			if req.URL.Path != "/v1/betaGroups/group-all/relationships/builds" {
				t.Fatalf("unexpected all-build verification: %s", req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[]}`), nil
		case 7:
			if req.URL.Path != "/v1/betaGroups/group-both/relationships/builds" {
				t.Fatalf("unexpected explicit all-build verification: %s", req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"builds","id":"build-1"}]}`), nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "groups", "list", "--build-id", "build-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requestCount != 7 {
		t.Fatalf("expected seven requests, got %d", requestCount)
	}

	var got buildGroupMembershipOutput
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode output: %v\n%s", err, stdout)
	}
	if got.BuildID != "build-1" || got.AppID != "app-1" || !got.Complete || got.LookupMethod != "server_filter_with_inverse_all_builds" {
		t.Fatalf("unexpected lookup context: %#v", got)
	}
	if got.GroupCount != 3 || len(got.Groups) != 3 {
		t.Fatalf("expected three deduplicated groups, got %#v", got.Groups)
	}
	want := []buildGroupMembershipRecord{
		{ID: "group-explicit", Name: "External QA", Type: "external", Membership: "explicit", HasAccessToAllBuilds: false},
		{ID: "group-all", Name: "Internal All Builds", Type: "internal", Membership: "all-builds", HasAccessToAllBuilds: true},
		{ID: "group-both", Name: "Partner Preview", Type: "external", Membership: "explicit-and-all-builds", HasAccessToAllBuilds: true},
	}
	for i := range want {
		if got.Groups[i] != want[i] {
			t.Fatalf("group %d = %#v, want %#v", i, got.Groups[i], want[i])
		}
	}
}

func TestTestFlightGroupsListBuildMembershipRejectsExplicitBlankIDs(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("blank build ID must fail before HTTP request: %s", req.URL.String())
		return nil, nil
	})

	tests := []struct {
		name       string
		args       []string
		diagnostic string
	}{
		{name: "build ID", args: []string{"--app", "app-1", "--build-id", " "}, diagnostic: "--build-id cannot be empty"},
		{name: "app assertion", args: []string{"--build-id", "build-1", "--app", " "}, diagnostic: "--app cannot be empty"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			var runErr error
			args := append([]string{"testflight", "groups", "list"}, test.args...)
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})
			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected usage error, got %v", runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.diagnostic) {
				t.Fatalf("expected %q diagnostic, got %q", test.diagnostic, stderr)
			}
		})
	}
}

func TestTestFlightGroupsListBuildMembershipEmptyIsSuccessful(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"apps","id":"app-1"}}`), nil
		case 2:
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-none","attributes":{"name":"No Access"}}]}`), nil
		case 3:
			return jsonHTTPResponse(http.StatusOK, `{"data":[]}`), nil
		default:
			t.Fatalf("unexpected request: %s", req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "groups", "list", "--build-id", "build-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("empty membership must succeed, got %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var got buildGroupMembershipOutput
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if !got.Complete || got.GroupCount != 0 || got.Groups == nil || len(got.Groups) != 0 {
		t.Fatalf("expected complete empty result, got %#v", got)
	}
}

func TestTestFlightGroupsListBuildMembershipRendersTable(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"apps","id":"app-1"}}`), nil
		case 2, 3:
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"External QA","isInternalGroup":false}}]}`), nil
		default:
			t.Fatalf("unexpected request: %s", req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "groups", "list", "--build-id", "build-1", "--output", "table"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	for _, want := range []string{"Build ID", "Group ID", "Membership", "build-1", "group-1", "explicit", "External QA"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected table to contain %q, got %q", want, stdout)
		}
	}
}

func TestTestFlightGroupsListBuildMembershipFallsBackAndFindsLargeGroupMatch(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	const nextURL = "https://api.appstoreconnect.apple.com/v1/betaGroups/group-large/relationships/builds?cursor=next"
	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"apps","id":"app-1"}}`), nil
		case 2:
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-large","attributes":{"name":"Large External","isInternalGroup":false}}]}`), nil
		case 3:
			return jsonHTTPResponse(http.StatusBadRequest, `{"errors":[{"status":"400","code":"PARAMETER_ERROR","title":"Unsupported filter"}]}`), nil
		case 4:
			if req.URL.Path != "/v1/betaGroups/group-large/relationships/builds" || req.URL.Query().Get("limit") != "200" {
				t.Fatalf("unexpected inverse lookup: %s", req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, buildLinkagesPage(200, "other", nextURL)), nil
		case 5:
			if req.URL.String() != nextURL {
				t.Fatalf("unexpected inverse next URL: %s", req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"builds","id":"build-1"}]}`), nil
		default:
			t.Fatalf("unexpected request: %s", req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "groups", "list", "--build-id", "build-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if !strings.Contains(stderr, "falling back to inverse group build relationships") {
		t.Fatalf("expected transparent fallback warning, got %q", stderr)
	}

	var got buildGroupMembershipOutput
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if !got.Complete || got.LookupMethod != "inverse_relationships" || got.GroupCount != 1 || got.Groups[0].ID != "group-large" {
		t.Fatalf("unexpected fallback result: %#v", got)
	}
}

func TestTestFlightGroupsListBuildMembershipPreservesPartialResultAndFails(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"apps","id":"app-1"}}`), nil
		case 2:
			return jsonHTTPResponse(http.StatusOK, `{"data":[
				{"type":"betaGroups","id":"group-all","attributes":{"name":"All Builds","isInternalGroup":true,"hasAccessToAllBuilds":true}},
				{"type":"betaGroups","id":"group-explicit","attributes":{"name":"Explicit","isInternalGroup":false}},
				{"type":"betaGroups","id":"group-failed","attributes":{"name":"Failed","isInternalGroup":false}}
			]}`), nil
		case 3:
			return jsonHTTPResponse(http.StatusBadRequest, `{"errors":[{"status":"400","code":"PARAMETER_ERROR","title":"Unsupported filter"}]}`), nil
		case 4:
			return jsonHTTPResponse(http.StatusOK, `{"data":[]}`), nil
		case 5:
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"builds","id":"build-1"}]}`), nil
		case 6:
			return jsonHTTPResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","title":"Temporary failure"}]}`), nil
		default:
			t.Fatalf("unexpected request: %s", req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "groups", "list", "--build-id", "build-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr == nil {
		t.Fatal("expected incomplete lookup to fail")
	}
	if !strings.Contains(runErr.Error(), "membership lookup incomplete") {
		t.Fatalf("unexpected error: %v", runErr)
	}
	if !strings.Contains(stderr, "1 group relationship lookup failed") {
		t.Fatalf("expected partial failure diagnostic, got %q", stderr)
	}

	var got buildGroupMembershipOutput
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode output: %v\n%s", err, stdout)
	}
	if got.Complete || got.GroupCount != 2 || len(got.Failures) != 1 || got.Failures[0].GroupID != "group-failed" {
		t.Fatalf("unexpected partial result: %#v", got)
	}
	if got.Groups[0].ID != "group-all" || got.Groups[0].Membership != "all-builds" || got.Groups[1].ID != "group-explicit" {
		t.Fatalf("unexpected partial memberships: %#v", got.Groups)
	}
}

func TestTestFlightGroupsListBuildMembershipRejectsPageControlsAndGlobal(t *testing.T) {
	tests := [][]string{
		{"--global"},
		{"--global=false"},
		{"--limit", "20"},
		{"--limit", "0"},
		{"--paginate"},
		{"--paginate=false"},
		{"--next", "https://api.appstoreconnect.apple.com/v1/betaGroups?cursor=next"},
		{"--next", ""},
	}

	for _, extra := range tests {
		t.Run(strings.TrimPrefix(strings.Join(extra, "_"), "--"), func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			args := append([]string{"testflight", "groups", "list", "--build-id", "build-1"}, extra...)
			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})
			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected usage error, got %v", runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, "cannot be used with --build-id") {
				t.Fatalf("expected incompatibility diagnostic, got %q", stderr)
			}
		})
	}
}

func assertBuildMembershipGroupQuery(t *testing.T, requestURL *url.URL, filterName, filterValue string) {
	t.Helper()
	if requestURL.Path != "/v1/betaGroups" {
		t.Fatalf("unexpected beta groups path: %s", requestURL.String())
	}
	query := requestURL.Query()
	if query.Get(filterName) != filterValue {
		t.Fatalf("%s = %q, want %q in %s", filterName, query.Get(filterName), filterValue, requestURL.String())
	}
	if query.Get("limit") != "200" {
		t.Fatalf("limit = %q, want 200", query.Get("limit"))
	}
	if query.Get("fields[betaGroups]") != "name,isInternalGroup,hasAccessToAllBuilds" {
		t.Fatalf("unexpected sparse fields: %q", query.Get("fields[betaGroups]"))
	}
}

func buildLinkagesPage(count int, prefix, next string) string {
	data := make([]map[string]string, 0, count)
	for i := 0; i < count; i++ {
		data = append(data, map[string]string{
			"type": "builds",
			"id":   fmt.Sprintf("%s-%03d", prefix, i),
		})
	}
	payload := struct {
		Data  []map[string]string `json:"data"`
		Links map[string]string   `json:"links,omitempty"`
	}{Data: data}
	if next != "" {
		payload.Links = map[string]string{"next": next}
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}
