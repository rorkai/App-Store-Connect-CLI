package cmdtest

import (
	"encoding/json"
	"errors"
	"flag"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestReviewDetailsCreateRejectsDemoAccountRequiredWithoutCredentials(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	stdout, stderr, runErr := runRootWithDeps(t, stubClient(t, asctest.NewRegistry(t)), "review", "details-create",
		"--version-id", "version-1",
		"--demo-account-required=true")

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--demo-account-required=true requires both --demo-account-name and --demo-account-password") {
		t.Fatalf("expected local demo credential validation error, got %q", stderr)
	}
}

func TestReviewDetailsCreateAllowsDemoAccountRequiredWithBothCredentials(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.POST("/v1/appStoreReviewDetails").JSONBody(`{"data":{"type":"appStoreReviewDetails","attributes":{"contactFirstName":"Dev","contactLastName":"Support","contactPhone":"+1 408 555 0100","contactEmail":"dev@example.com","demoAccountName":"reviewer@example.com","demoAccountPassword":"app-specific-password","demoAccountRequired":true},"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"version-1"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"appStoreReviewDetails","id":"detail-1","attributes":{"demoAccountRequired":true,"demoAccountName":"reviewer@example.com","demoAccountPassword":"app-specific-password"}}}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "review", "details-create",
		"--version-id", "version-1",
		"--contact-first-name", "Dev",
		"--contact-last-name", "Support",
		"--contact-email", "dev@example.com",
		"--contact-phone", "+1 408 555 0100",
		"--demo-account-required=true",
		"--demo-account-name", "reviewer@example.com",
		"--demo-account-password", "app-specific-password")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"detail-1"`) {
		t.Fatalf("expected detail id in output, got %q", stdout)
	}
}

func TestReviewDetailsUpdateRejectsDemoAccountRequiredWhenExistingCredentialsAreIncomplete(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/appStoreReviewDetails/detail-1"), asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreReviewDetails","id":"detail-1","attributes":{"contactFirstName":"Dev","contactLastName":"Support","contactEmail":"dev@example.com","contactPhone":"123","demoAccountRequired":false}}}`))

	stdout, stderr, runErr := runRootWithDeps(t, stubClient(t, reg), "review", "details-update",
		"--id", "detail-1",
		"--demo-account-required=true")

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--demo-account-required=true requires both --demo-account-name and --demo-account-password") {
		t.Fatalf("expected local demo credential validation error, got %q", stderr)
	}
}

func TestReviewDetailsUpdateAcceptsMaxLengthDemoAccountPassword(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.PATCH("/v1/appStoreReviewDetails/detail-1").JSONBody(`{"data":{"type":"appStoreReviewDetails","id":"detail-1","attributes":{"demoAccountPassword":"`+strings.Repeat("p", 100)+`"}}}`),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreReviewDetails","id":"detail-1","attributes":{"demoAccountPassword":"`+strings.Repeat("p", 100)+`"}}}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "review", "details-update",
		"--id", "detail-1",
		"--demo-account-password", strings.Repeat("p", 100))
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"detail-1"`) {
		t.Fatalf("expected detail id in output, got %q", stdout)
	}
}

func TestRunReviewDetailsRejectsOverlongDemoAccountPasswordWithUsageExit(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "details-create",
			args: []string{
				"review", "details-create",
				"--version-id", "version-1",
				"--demo-account-password", strings.Repeat("p", 101),
			},
		},
		{
			name: "details-update",
			args: []string{
				"review", "details-update",
				"--id", "detail-1",
				"--demo-account-password", strings.Repeat("p", 101),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, code := runWithDeps(t, nil, test.args...)
			if code != cmd.ExitUsage {
				t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
			}

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, "--demo-account-password must be 100 characters or fewer") {
				t.Fatalf("expected local demo account password length error, got %q", stderr)
			}
		})
	}
}

func TestReviewDetailsUpdateAllowsDemoAccountRequiredWhenExistingCredentialsArePresent(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/appStoreReviewDetails/detail-1"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreReviewDetails","id":"detail-1","attributes":{"contactFirstName":"Dev","contactLastName":"Support","contactEmail":"dev@example.com","contactPhone":"123","demoAccountRequired":false,"demoAccountName":"reviewer@example.com","demoAccountPassword":"app-specific-password"}}}`))
	reg.Register(asctest.PATCH("/v1/appStoreReviewDetails/detail-1").JSONBody(`{"data":{"type":"appStoreReviewDetails","id":"detail-1","attributes":{"demoAccountRequired":true}}}`),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreReviewDetails","id":"detail-1","attributes":{"demoAccountRequired":true,"demoAccountName":"reviewer@example.com","demoAccountPassword":"app-specific-password"}}}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "review", "details-update",
		"--id", "detail-1",
		"--demo-account-required=true")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"detail-1"`) {
		t.Fatalf("expected detail id in output, got %q", stdout)
	}
}

func TestReviewDetailsForVersionReturnsNotConfiguredStateWhenUnset(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreReviewDetail"), asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1"), asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0"}}}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "review", "details-for-version", "--version-id", "version-1", "--output", "json")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if !strings.Contains(stderr, "Warning: App Store review detail is not configured for version \"version-1\".") {
		t.Fatalf("expected not-configured warning, got %q", stderr)
	}

	var payload struct {
		VersionID  string `json:"versionId"`
		Configured bool   `json:"configured"`
		Message    string `json:"message"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}
	if payload.VersionID != "version-1" {
		t.Fatalf("expected versionId version-1, got %q", payload.VersionID)
	}
	if payload.Configured {
		t.Fatal("expected configured=false")
	}
	if payload.Message == "" {
		t.Fatal("expected message")
	}
}

func TestReviewDetailsForVersionPreservesErrorForUnknownVersionID(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/appStoreVersions/missing-version/appStoreReviewDetail"), asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/missing-version"), asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`))

	stdout, stderr, runErr := runRootWithDeps(t, stubClient(t, reg), "review", "details-for-version", "--version-id", "missing-version", "--output", "json")

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(runErr.Error(), "review details-for-version: failed to fetch:") {
		t.Fatalf("expected wrapped fetch error, got %v", runErr)
	}
	if strings.Contains(runErr.Error(), "not configured") {
		t.Fatalf("expected unknown version to remain a hard error, got %v", runErr)
	}
}

func TestReviewDetailsGetSelectsByVersionOrApp(t *testing.T) {
	type stub struct {
		matcher asctest.Matcher
		body    string
	}
	detail := stub{
		matcher: asctest.GET("/v1/appStoreVersions/version-1/appStoreReviewDetail"),
		body:    `{"data":{"type":"appStoreReviewDetails","id":"detail-1","attributes":{"notes":"Sign in with the demo account"}}}`,
	}
	versionOf := func(versionID, appID string) stub {
		return stub{
			matcher: asctest.GET("/v1/appStoreVersions/"+versionID, "include=app"),
			body:    `{"data":{"type":"appStoreVersions","id":"` + versionID + `","attributes":{"versionString":"1.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"` + appID + `"}}}}}`,
		}
	}
	tests := []struct {
		name       string
		args       []string
		stubs      []stub
		wantStdout string
		wantStderr string
		wantErr    string
	}{
		{
			name:       "version ID",
			args:       []string{"--version-id", "version-1"},
			stubs:      []stub{detail},
			wantStdout: `"id":"detail-1"`,
		},
		{
			name:       "version ID of the app",
			args:       []string{"--app", "100000001", "--version-id", "version-1"},
			stubs:      []stub{versionOf("version-1", "100000001"), detail},
			wantStdout: `"id":"detail-1"`,
		},
		{
			name:    "version ID of another app",
			args:    []string{"--app", "100000001", "--version-id", "version-9"},
			stubs:   []stub{versionOf("version-9", "200000002")},
			wantErr: `version "version-9" belongs to app "200000002", not "100000001"`,
		},
		{
			name: "app alone reads its editable version",
			args: []string{"--app", "100000001"},
			stubs: []stub{
				{
					matcher: asctest.GET("/v1/apps/100000001/appStoreVersions", "filter[appVersionState]=DEVELOPER_REJECTED,INVALID_BINARY,METADATA_REJECTED,PREPARE_FOR_SUBMISSION,READY_FOR_REVIEW,REJECTED,WAITING_FOR_REVIEW", "limit=200"),
					body:    `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.1","platform":"IOS","appVersionState":"PREPARE_FOR_SUBMISSION"}}]}`,
				},
				detail,
			},
			wantStdout: `"id":"detail-1"`,
			wantStderr: "Using version 1.1 (PREPARE_FOR_SUBMISSION) for platform IOS; pass --version-id to override",
		},
		{
			name:    "detail ID with explicit app",
			args:    []string{"--id", "detail-1", "--app", "100000001"},
			wantErr: "--app selects the review detail through the app's version; pass --app with or without --version-id, not with --id",
		},
		{
			name:    "detail ID with explicit app and platform",
			args:    []string{"--id", "detail-1", "--app", "100000001", "--platform", "IOS"},
			wantErr: "--app selects the review detail through the app's version; pass --app with or without --version-id, not with --id",
		},
		{
			name:    "detail ID with version ID",
			args:    []string{"--id", "detail-1", "--version-id", "version-1"},
			wantErr: "--id and --version-id are mutually exclusive",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_APP_ID", "")
			reg := asctest.NewRegistry(t)
			for _, stub := range test.stubs {
				reg.Register(stub.matcher, asctest.JSON(http.StatusOK, stub.body))
			}

			args := append([]string{"review", "details-get", "--output", "json"}, test.args...)
			stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), args...)

			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error()+stderr, test.wantErr) {
					t.Fatalf("err = %v, stderr = %q, want %q", err, stderr, test.wantErr)
				}
				if stdout != "" {
					t.Fatalf("expected no output, got %q", stdout)
				}
				return
			}
			if err != nil {
				t.Fatalf("run error: %v", err)
			}
			if !strings.Contains(stdout, test.wantStdout) {
				t.Fatalf("stdout = %q, want %s", stdout, test.wantStdout)
			}
			if !strings.Contains(stderr, test.wantStderr) {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
			}
		})
	}
}

func TestReviewDetailsVersionSelectorsCheckSelectedAppBeforeReadsOrWrites(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
	}{
		{
			name: "for version",
			args: []string{"review", "details-for-version", "--version-id", "version-1"},
		},
		{
			name: "create",
			args: []string{
				"review", "details-create", "--version-id", "version-1",
				"--contact-first-name", "Dev", "--contact-phone", "+1 408 555 0100",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_APP_ID", "100000001")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			reg := asctest.NewRegistry(t)
			reg.Register(asctest.GET("/v1/appStoreVersions/version-1", "include=app"), asctest.JSON(http.StatusOK,
				`{"data":{"type":"appStoreVersions","id":"version-1","relationships":{"app":{"data":{"type":"apps","id":"200000002"}}}}}`))

			stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), test.args...)
			if err == nil || !strings.Contains(err.Error()+stderr, `version "version-1" belongs to app "200000002", not "100000001"`) {
				t.Fatalf("err=%v stderr=%q, want selected-app ownership failure", err, stderr)
			}
			if stdout != "" {
				t.Fatalf("expected no output, got %q", stdout)
			}
			if requests := reg.Requests(); len(requests) != 1 {
				t.Fatalf("expected one ownership lookup and no detail read/write, got %d requests", len(requests))
			}
		})
	}
}

func TestReviewDetailsCreateSelectedAppWritesOnceAfterOwnershipCheck(t *testing.T) {
	t.Setenv("ASC_APP_ID", "100000001")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1", "include=app"), asctest.JSON(http.StatusOK,
		`{"data":{"type":"appStoreVersions","id":"version-1","relationships":{"app":{"data":{"type":"apps","id":"100000001"}}}}}`))
	reg.Register(asctest.POST("/v1/appStoreReviewDetails"), asctest.JSON(http.StatusCreated,
		`{"data":{"type":"appStoreReviewDetails","id":"detail-1","attributes":{"contactFirstName":"Dev","contactPhone":"+1 408 555 0100"}}}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg),
		"review", "details-create", "--version-id", "version-1",
		"--contact-first-name", "Dev", "--contact-phone", "+1 408 555 0100")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if stderr != "" || !strings.Contains(stdout, `"id":"detail-1"`) {
		t.Fatalf("stdout=%q stderr=%q", stdout, stderr)
	}
	requests := reg.Requests()
	if len(requests) != 2 || requests[0].Method != http.MethodGet || requests[1].Method != http.MethodPost {
		t.Fatalf("expected one ownership GET followed by one create POST, got %+v", requests)
	}
}

func TestReviewDetailsDirectIDsCheckSelectedAppBeforeReadOrWrite(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
	}{
		{
			name: "get mismatch",
			args: []string{"review", "details-get", "--id", "detail-1"},
		},
		{
			name: "update mismatch",
			args: []string{"review", "details-update", "--id", "detail-1", "--notes", "Updated"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_APP_ID", "100000001")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			reg := asctest.NewRegistry(t)
			reg.Register(asctest.GET("/v1/appStoreReviewDetails/detail-1", "fields[appStoreReviewDetails]=appStoreVersion", "include=appStoreVersion"), asctest.JSON(http.StatusOK,
				`{"data":{"type":"appStoreReviewDetails","id":"detail-1","relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"version-1"}}}}}`))
			reg.Register(asctest.GET("/v1/appStoreVersions/version-1", "include=app"), asctest.JSON(http.StatusOK,
				`{"data":{"type":"appStoreVersions","id":"version-1","relationships":{"app":{"data":{"type":"apps","id":"200000002"}}}}}`))

			stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), test.args...)
			if err == nil || !strings.Contains(err.Error()+stderr, `version "version-1" belongs to app "200000002", not "100000001"`) {
				t.Fatalf("err=%v stderr=%q, want selected-app ownership failure", err, stderr)
			}
			if stdout != "" {
				t.Fatalf("expected no output, got %q", stdout)
			}
			if requests := reg.Requests(); len(requests) != 2 {
				t.Fatalf("expected sparse detail and version ownership reads only, got %d requests", len(requests))
			}
		})
	}
}

func TestReviewDetailsDirectIDOwnershipRejectsMalformedRelationship(t *testing.T) {
	t.Setenv("ASC_APP_ID", "100000001")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/appStoreReviewDetails/detail-1", "fields[appStoreReviewDetails]=appStoreVersion", "include=appStoreVersion"), asctest.JSON(http.StatusOK,
		`{"data":{"type":"appStoreReviewDetails","id":"detail-1","relationships":{"appStoreVersion":{"data":{"type":"appInfos","id":"version-1"}}}}}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "review", "details-get", "--id", "detail-1")
	if err == nil || !strings.Contains(err.Error()+stderr, `expected review detail appStoreVersion relationship type "appStoreVersions", got "appInfos"`) {
		t.Fatalf("err=%v stderr=%q, want malformed relationship error", err, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected no output, got %q", stdout)
	}
	if requests := reg.Requests(); len(requests) != 1 {
		t.Fatalf("expected only sparse ownership read, got %d requests", len(requests))
	}
}

func TestReviewDetailsDirectIDOwnershipRejectsWrongIdentity(t *testing.T) {
	t.Setenv("ASC_APP_ID", "100000001")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/appStoreReviewDetails/detail-1", "fields[appStoreReviewDetails]=appStoreVersion", "include=appStoreVersion"), asctest.JSON(http.StatusOK,
		`{"data":{"type":"appStoreReviewDetails","id":"detail-other","relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"version-1"}}}}}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "review", "details-get", "--id", "detail-1")
	if err == nil || !strings.Contains(err.Error()+stderr, `expected review detail "detail-1", got "detail-other"`) {
		t.Fatalf("err=%v stderr=%q, want wrong identity error", err, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected no output, got %q", stdout)
	}
	if requests := reg.Requests(); len(requests) != 1 {
		t.Fatalf("expected only sparse ownership read, got %d requests", len(requests))
	}
}

func TestReviewDetailsDirectIDMatchingAppWritesAfterOwnershipCheck(t *testing.T) {
	t.Setenv("ASC_APP_ID", "100000001")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	for _, test := range []struct {
		name string
		args []string
	}{
		{name: "get", args: []string{"review", "details-get", "--id", "detail-1"}},
		{name: "update", args: []string{"review", "details-update", "--id", "detail-1", "--notes", "Updated"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			reg := asctest.NewRegistry(t)
			reg.Register(asctest.GET("/v1/appStoreReviewDetails/detail-1", "fields[appStoreReviewDetails]=appStoreVersion", "include=appStoreVersion"), asctest.JSON(http.StatusOK,
				`{"data":{"type":"appStoreReviewDetails","id":"detail-1","relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"version-1"}}}}}`))
			reg.Register(asctest.GET("/v1/appStoreVersions/version-1", "include=app"), asctest.JSON(http.StatusOK,
				`{"data":{"type":"appStoreVersions","id":"version-1","relationships":{"app":{"data":{"type":"apps","id":"100000001"}}}}}`))
			if test.name == "get" {
				reg.Register(asctest.GET("/v1/appStoreReviewDetails/detail-1"), asctest.JSON(http.StatusOK,
					`{"data":{"type":"appStoreReviewDetails","id":"detail-1","attributes":{"notes":"Updated"}}}`))
			} else {
				reg.Register(asctest.PATCH("/v1/appStoreReviewDetails/detail-1"), asctest.JSON(http.StatusOK,
					`{"data":{"type":"appStoreReviewDetails","id":"detail-1","attributes":{"notes":"Updated"}}}`))
			}

			stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), test.args...)
			if err != nil {
				t.Fatalf("run error: %v", err)
			}
			if stderr != "" || !strings.Contains(stdout, `"id":"detail-1"`) {
				t.Fatalf("stdout=%q stderr=%q", stdout, stderr)
			}
			requests := reg.Requests()
			if len(requests) != 3 || requests[0].Method != http.MethodGet || requests[1].Method != http.MethodGet || requests[2].Method != map[string]string{"get": http.MethodGet, "update": http.MethodPatch}[test.name] {
				t.Fatalf("expected two ownership reads and one final operation, got %+v", requests)
			}
		})
	}
}

func TestReviewDetailsDirectIDUpdateWithoutSelectedAppWritesOnce(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	reg := asctest.NewRegistry(t)
	reg.Register(asctest.PATCH("/v1/appStoreReviewDetails/detail-1"), asctest.JSON(http.StatusOK,
		`{"data":{"type":"appStoreReviewDetails","id":"detail-1","attributes":{"notes":"Updated"}}}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg),
		"review", "details-update", "--id", "detail-1", "--notes", "Updated")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if stderr != "" || !strings.Contains(stdout, `"id":"detail-1"`) {
		t.Fatalf("stdout=%q stderr=%q", stdout, stderr)
	}
	requests := reg.Requests()
	if len(requests) != 1 || requests[0].Method != http.MethodPatch {
		t.Fatalf("expected one unrestricted direct-ID PATCH, got %+v", requests)
	}
}
