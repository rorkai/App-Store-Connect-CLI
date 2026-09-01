package cmdtest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	webcmd "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/web"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func stubRatingResetWebSession(t *testing.T, transport roundTripFunc) {
	t.Helper()
	restore := webcmd.SetResolveWebSession(func(context.Context, string, string, string, string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{Client: &http.Client{Transport: transport}}, "cache", nil
	})
	t.Cleanup(restore)
}

func TestVersionsRatingResetCreateRootIntegration(t *testing.T) {
	stubRatingResetWebSession(t, func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.URL.Path != "/iris/v1/resetRatingsRequests" {
			t.Fatalf("request = %s %s, want POST /iris/v1/resetRatingsRequests", request.Method, request.URL.Path)
		}
		if got := request.Header.Get("X-CSRF-ITC"); got != "[asc-ui]" {
			t.Fatalf("X-CSRF-ITC = %q, want [asc-ui]", got)
		}

		var payload struct {
			Data struct {
				Type          string `json:"type"`
				Relationships struct {
					AppStoreVersion struct {
						Data struct {
							Type string `json:"type"`
							ID   string `json:"id"`
						} `json:"data"`
					} `json:"appStoreVersion"`
				} `json:"relationships"`
			} `json:"data"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		version := payload.Data.Relationships.AppStoreVersion.Data
		if payload.Data.Type != "resetRatingsRequests" || version.Type != "appStoreVersions" || version.ID != "version-1" {
			t.Fatalf("payload = %#v, want resetRatingsRequests for version-1", payload)
		}
		return jsonResponse(http.StatusCreated, `{
			"data": {"type": "resetRatingsRequests", "id": "reset-1", "attributes": {"resetDate": null}}
		}`)
	})

	stdout, stderr := captureOutput(t, func() {
		code := rootcmd.Run([]string{
			"versions", "rating-reset", "create",
			"--output", "json",
			"--version-id", "version-1",
			"--confirm",
		}, "test")
		if code != rootcmd.ExitSuccess {
			t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitSuccess)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}

	var result struct {
		RatingResetRequestID string `json:"ratingResetRequestId"`
		VersionID            string `json:"versionId"`
		Scheduled            bool   `json:"scheduled"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal stdout: %v; stdout=%q", err, stdout)
	}
	if result.RatingResetRequestID != "reset-1" || result.VersionID != "version-1" || !result.Scheduled {
		t.Fatalf("result = %#v, want scheduled reset receipt", result)
	}
}

func TestVersionsRatingResetViewRootIntegration(t *testing.T) {
	stubRatingResetWebSession(t, func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet || request.URL.Path != "/iris/v1/appStoreVersions/version-1/resetRatingsRequest" {
			t.Fatalf("request = %s %s, want version rating reset GET", request.Method, request.URL.Path)
		}
		return jsonResponse(http.StatusOK, `{
			"data": {
				"type": "resetRatingsRequests",
				"id": "reset-1",
				"attributes": {"resetDate": "2026-09-01T08:00:00Z"}
			}
		}`)
	})

	stdout, stderr := captureOutput(t, func() {
		code := rootcmd.Run([]string{
			"versions", "rating-reset", "view",
			"--version-id", "version-1",
			"--output", "table",
		}, "test")
		if code != rootcmd.ExitSuccess {
			t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitSuccess)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	for _, want := range []string{"Rating Reset Request ID", "Reset Date", "reset-1", "2026-09-01T08:00:00Z"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout = %q, want %q", stdout, want)
		}
	}
}

func TestVersionsRatingResetDeleteRootIntegration(t *testing.T) {
	stubRatingResetWebSession(t, func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodDelete || request.URL.Path != "/iris/v1/resetRatingsRequests/reset-1" {
			t.Fatalf("request = %s %s, want DELETE rating reset", request.Method, request.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("")),
		}, nil
	})

	stdout, stderr := captureOutput(t, func() {
		code := rootcmd.Run([]string{
			"versions", "rating-reset", "delete",
			"--confirm",
			"--id", "reset-1",
			"--output", "json",
		}, "test")
		if code != rootcmd.ExitSuccess {
			t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitSuccess)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}

	var result struct {
		RatingResetRequestID string `json:"ratingResetRequestId"`
		Cancelled            bool   `json:"cancelled"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal stdout: %v; stdout=%q", err, stdout)
	}
	if result.RatingResetRequestID != "reset-1" || !result.Cancelled {
		t.Fatalf("result = %#v, want cancelled reset receipt", result)
	}
}

func TestVersionsRatingResetValidationBeforeSessionResolution(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{name: "view missing version", args: []string{"versions", "rating-reset", "view"}, wantStderr: "--version-id is required"},
		{name: "create missing version", args: []string{"versions", "rating-reset", "create", "--confirm"}, wantStderr: "--version-id is required"},
		{name: "create missing confirmation", args: []string{"versions", "rating-reset", "create", "--version-id", "version-1"}, wantStderr: "--confirm is required"},
		{name: "delete missing id", args: []string{"versions", "rating-reset", "delete", "--confirm"}, wantStderr: "--id is required"},
		{name: "delete missing confirmation", args: []string{"versions", "rating-reset", "delete", "--id", "reset-1"}, wantStderr: "--confirm is required"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resolverCalls := 0
			restore := webcmd.SetResolveWebSession(func(context.Context, string, string, string, string) (*webcore.AuthSession, string, error) {
				resolverCalls++
				return nil, "", fmt.Errorf("session should not be resolved")
			})
			t.Cleanup(restore)

			stdout, stderr := captureOutput(t, func() {
				if code := rootcmd.Run(test.args, "test"); code != rootcmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
				}
			})
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, "Error: "+test.wantStderr) {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
			}
			if resolverCalls != 0 {
				t.Fatalf("session resolver calls = %d, want 0", resolverCalls)
			}
		})
	}
}

func TestVersionsRatingResetCommandIsExperimentalAndDocumentsRisk(t *testing.T) {
	command := findSubcommand(RootCommand("test"), "versions", "rating-reset")
	if command == nil {
		t.Fatal("versions rating-reset command not found")
	}
	if !strings.HasPrefix(command.ShortHelp, "[experimental] ") {
		t.Fatalf("ShortHelp = %q, want experimental lifecycle marker", command.ShortHelp)
	}
	help := strings.Join(strings.Fields(command.LongHelp), " ")
	for _, want := range []string{
		"does not include this resource in its published OpenAPI specification",
		"authenticated Apple Account web session",
		"you cannot restore the previous overview rating",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("LongHelp = %q, want %q", command.LongHelp, want)
		}
	}

	for _, name := range []string{"create", "delete"} {
		subcommand := findSubcommand(command, name)
		if subcommand == nil || subcommand.FlagSet.Lookup("confirm") == nil {
			t.Fatalf("rating-reset %s must require --confirm", name)
		}
	}
}
