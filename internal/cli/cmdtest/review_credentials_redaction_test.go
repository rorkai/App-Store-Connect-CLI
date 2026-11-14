package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

// Unique sentinels make a leak unambiguous: no fixture, help text, or unrelated
// field can produce these strings by accident.
const (
	appStoreDemoPasswordSentinel   = "asc-red-sentinel-appstore-demo-pw-9f31c7"
	testFlightDemoPasswordSentinel = "asc-red-sentinel-testflight-demo-pw-4b82ad"
	redactedDemoPasswordText       = "(redacted)"
	includeSensitiveWarningText    = "Warning: --include-sensitive prints secrets"
)

func appStoreReviewDetailJSON() string {
	return `{"data":{"type":"appStoreReviewDetails","id":"detail-1","attributes":{` +
		`"contactFirstName":"Dev","contactLastName":"Support","contactEmail":"dev@example.com",` +
		`"demoAccountRequired":true,"demoAccountName":"reviewer@example.com",` +
		`"demoAccountPassword":"` + appStoreDemoPasswordSentinel + `",` +
		`"notes":"Reviewer notes"}}}`
}

func betaAppReviewDetailsJSON() string {
	return `{"data":[{"type":"betaAppReviewDetails","id":"detail-1","attributes":{` +
		`"contactFirstName":"Dev","contactLastName":"Support","contactEmail":"dev@example.com",` +
		`"demoAccountRequired":true,"demoAccountName":"reviewer@example.com",` +
		`"demoAccountPassword":"` + testFlightDemoPasswordSentinel + `",` +
		`"notes":"Reviewer notes"}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/betaAppReviewDetails"}}`
}

func betaAppReviewDetailJSON() string {
	return `{"data":{"type":"betaAppReviewDetails","id":"detail-1","attributes":{` +
		`"demoAccountRequired":true,"demoAccountName":"reviewer@example.com",` +
		`"demoAccountPassword":"` + testFlightDemoPasswordSentinel + `"}}}`
}

func includedAppStoreReviewDetailJSON() string {
	return `{"type":"appStoreReviewDetails","id":"detail-1","attributes":{` +
		`"demoAccountPassword":"` + appStoreDemoPasswordSentinel + `","notes":"keep"}}`
}

func assertNoSentinel(t *testing.T, label, sentinel, content string) {
	t.Helper()
	if strings.Contains(content, sentinel) {
		t.Fatalf("%s leaked the demo account password sentinel: %q", label, content)
	}
}

func TestReviewDetailsGetRedactsDemoAccountPasswordInEveryFormat(t *testing.T) {
	formats := []struct {
		name string
		args []string
	}{
		{name: "json", args: []string{"--output", "json"}},
		{name: "pretty json", args: []string{"--output", "json", "--pretty"}},
		{name: "table", args: []string{"--output", "table"}},
		{name: "markdown", args: []string{"--output", "markdown"}},
	}

	for _, format := range formats {
		t.Run(format.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET("/v1/appStoreReviewDetails/detail-1"), asctest.JSON(http.StatusOK, appStoreReviewDetailJSON()))

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			args := append([]string{"review", "details-get", "--id", "detail-1"}, format.args...)
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			assertNoSentinel(t, "stdout", appStoreDemoPasswordSentinel, stdout)
			assertNoSentinel(t, "stderr", appStoreDemoPasswordSentinel, stderr)
			if !strings.Contains(stdout, "detail-1") {
				t.Fatalf("expected detail id in output, got %q", stdout)
			}
			if format.name == "json" || format.name == "pretty json" {
				if !strings.Contains(stdout, redactedDemoPasswordText) {
					t.Fatalf("expected %q placeholder in output, got %q", redactedDemoPasswordText, stdout)
				}
			}
		})
	}
}

func TestReviewDetailsGetIncludesDemoAccountPasswordWithIncludeSensitive(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreReviewDetails/detail-1"), asctest.JSON(http.StatusOK, appStoreReviewDetailJSON()))

	stdout, stderr, err := runCommand(t, []string{
		"review", "details-get",
		"--id", "detail-1",
		"--output", "json",
		"--include-sensitive",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	var payload struct {
		Data struct {
			Attributes struct {
				DemoAccountPassword string `json:"demoAccountPassword"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}
	if payload.Data.Attributes.DemoAccountPassword != appStoreDemoPasswordSentinel {
		t.Fatalf("expected real password with --include-sensitive, got %q", payload.Data.Attributes.DemoAccountPassword)
	}
	if !strings.Contains(stderr, includeSensitiveWarningText) {
		t.Fatalf("expected plaintext-secret warning on stderr, got %q", stderr)
	}
}

func TestReviewDetailsForVersionRedactsDemoAccountPassword(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreReviewDetail"), asctest.JSON(http.StatusOK, appStoreReviewDetailJSON()))

	stdout, stderr, err := runCommand(t, []string{"review", "details-for-version", "--version-id", "version-1", "--output", "json"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	assertNoSentinel(t, "stdout", appStoreDemoPasswordSentinel, stdout)
	assertNoSentinel(t, "stderr", appStoreDemoPasswordSentinel, stderr)
	if !strings.Contains(stdout, redactedDemoPasswordText) {
		t.Fatalf("expected redaction placeholder, got %q", stdout)
	}
}

func TestIncludedReviewDetailsRedactDemoAccountPasswordByDefault(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		query []string
		args  []string
		body  string
	}{
		{
			name:  "versions view",
			path:  "/v1/appStoreVersions/version-1",
			query: []string{"include=appStoreReviewDetail"},
			args: []string{
				"versions", "view", "--version-id", "version-1",
				"--include", "appStoreReviewDetail", "--output", "json",
			},
			body: `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}},"included":[` +
				includedAppStoreReviewDetailJSON() + `]}`,
		},
		{
			name:  "review attachment list",
			path:  "/v1/appStoreReviewDetails/detail-1/appStoreReviewAttachments",
			query: []string{"fields[appStoreReviewDetails]=demoAccountPassword", "include=appStoreReviewDetail"},
			args: []string{
				"review", "attachments-list", "--review-detail", "detail-1",
				"--include", "appStoreReviewDetail", "--detail-fields", "demoAccountPassword", "--output", "json",
			},
			body: `{"data":[],"included":[` + includedAppStoreReviewDetailJSON() + `]}`,
		},
		{
			name:  "review attachment get",
			path:  "/v1/appStoreReviewAttachments/attachment-1",
			query: []string{"fields[appStoreReviewDetails]=demoAccountPassword", "include=appStoreReviewDetail"},
			args: []string{
				"review", "attachments-get", "--id", "attachment-1",
				"--include", "appStoreReviewDetail", "--detail-fields", "demoAccountPassword", "--output", "json",
			},
			body: `{"data":{"type":"appStoreReviewAttachments","id":"attachment-1","attributes":{"fileName":"review.pdf"}},"included":[` +
				includedAppStoreReviewDetailJSON() + `]}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET(test.path, test.query...), asctest.JSON(http.StatusOK, test.body))

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			assertNoSentinel(t, "stdout", appStoreDemoPasswordSentinel, stdout)
			assertNoSentinel(t, "stderr", appStoreDemoPasswordSentinel, stderr)
			if !strings.Contains(stdout, redactedDemoPasswordText) {
				t.Fatalf("expected redaction placeholder, got %q", stdout)
			}
		})
	}
}

func TestReviewDetailsCreateSendsPasswordButRedactsResponse(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.POST("/v1/appStoreReviewDetails"), asctest.JSON(http.StatusCreated, appStoreReviewDetailJSON()))

	stdout, stderr, err := runCommand(t, []string{
		"review", "details-create",
		"--version-id", "version-1",
		"--contact-first-name", "Dev",
		"--contact-last-name", "Support",
		"--contact-email", "dev@example.com",
		"--contact-phone", "+1 408 555 0100",
		"--output", "json",
		"--demo-account-required=true",
		"--demo-account-name", "reviewer@example.com",
		"--demo-account-password", appStoreDemoPasswordSentinel,
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	requests := reg.Requests()
	sentRealPassword := len(requests) == 1 && strings.Contains(string(requests[0].Body), `"demoAccountPassword":"`+appStoreDemoPasswordSentinel+`"`)
	if !sentRealPassword {
		t.Fatal("expected the create request payload to carry the explicitly supplied password")
	}
	assertNoSentinel(t, "stdout", appStoreDemoPasswordSentinel, stdout)
	assertNoSentinel(t, "stderr", appStoreDemoPasswordSentinel, stderr)
}

func TestReviewDetailsUpdateSendsPasswordButRedactsResponse(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.PATCH("/v1/appStoreReviewDetails/detail-1"), asctest.JSON(http.StatusOK, appStoreReviewDetailJSON()))

	stdout, stderr, err := runCommand(t, []string{
		"review", "details-update",
		"--id", "detail-1",
		"--output", "json",
		"--demo-account-password", appStoreDemoPasswordSentinel,
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	requests := reg.Requests()
	sentRealPassword := len(requests) == 1 && strings.Contains(string(requests[0].Body), `"demoAccountPassword":"`+appStoreDemoPasswordSentinel+`"`)
	if !sentRealPassword {
		t.Fatal("expected the update request payload to carry the explicitly supplied password")
	}
	assertNoSentinel(t, "stdout", appStoreDemoPasswordSentinel, stdout)
	assertNoSentinel(t, "stderr", appStoreDemoPasswordSentinel, stderr)
}

func TestTestFlightReviewViewRedactsDemoAccountPassword(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/betaAppReviewDetails", "filter[app]=app-1"), asctest.JSON(http.StatusOK, betaAppReviewDetailsJSON()))

	stdout, stderr, err := runCommand(t, []string{"testflight", "review", "view", "--app", "app-1", "--output", "json"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	assertNoSentinel(t, "stdout", testFlightDemoPasswordSentinel, stdout)
	assertNoSentinel(t, "stderr", testFlightDemoPasswordSentinel, stderr)
	if !strings.Contains(stdout, redactedDemoPasswordText) {
		t.Fatalf("expected redaction placeholder, got %q", stdout)
	}
}

func TestTestFlightReviewEditSendsPasswordButRedactsResponse(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.PATCH("/v1/betaAppReviewDetails/detail-1"), asctest.JSON(http.StatusOK, betaAppReviewDetailJSON()))

	stdout, stderr, err := runCommand(t, []string{
		"testflight", "review", "edit",
		"--id", "detail-1",
		"--output", "json",
		"--demo-account-password", testFlightDemoPasswordSentinel,
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	requests := reg.Requests()
	sentRealPassword := len(requests) == 1 && strings.Contains(string(requests[0].Body), `"demoAccountPassword":"`+testFlightDemoPasswordSentinel+`"`)
	if !sentRealPassword {
		t.Fatal("expected the update request payload to carry the explicitly supplied password")
	}
	assertNoSentinel(t, "stdout", testFlightDemoPasswordSentinel, stdout)
	assertNoSentinel(t, "stderr", testFlightDemoPasswordSentinel, stderr)
}

func TestTestFlightReviewViewIncludesDemoAccountPasswordWithIncludeSensitive(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/betaAppReviewDetails", "filter[app]=app-1"), asctest.JSON(http.StatusOK, betaAppReviewDetailsJSON()))

	stdout, stderr, err := runCommand(t, []string{
		"testflight", "review", "view",
		"--app", "app-1",
		"--output", "json",
		"--include-sensitive",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if !strings.Contains(stdout, testFlightDemoPasswordSentinel) {
		t.Fatalf("expected real password with --include-sensitive, got %q", stdout)
	}
	if !strings.Contains(stderr, includeSensitiveWarningText) {
		t.Fatalf("expected plaintext-secret warning on stderr, got %q", stderr)
	}
}

func stubTransport(t *testing.T, fn roundTripFunc) {
	t.Helper()
	original := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = original
	})
	http.DefaultTransport = fn
}
