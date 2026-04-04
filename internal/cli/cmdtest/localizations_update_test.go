package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

type locUpdateRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn locUpdateRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func setupLocUpdateAuth(t *testing.T) {
	t.Helper()
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "AuthKey.p8")
	writeECDSAPEM(t, keyPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_KEY_ID", "TEST_KEY")
	t.Setenv("ASC_ISSUER_ID", "TEST_ISSUER")
	t.Setenv("ASC_PRIVATE_KEY_PATH", keyPath)
}

func locUpdateJSONResponse(body string) (*http.Response, error) {
	return &http.Response{
		Status:     fmt.Sprintf("%d %s", http.StatusOK, http.StatusText(http.StatusOK)),
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

func TestLocalizationsUpdateRequiresLocale(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"localizations", "update", "--version", "ver-1", "--description", "test"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "--locale is required") {
		t.Fatalf("expected locale required error, got: %q", stderr)
	}
}

func TestLocalizationsUpdateRequiresAtLeastOneField(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"localizations", "update", "--version", "ver-1", "--locale", "en-US"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "at least one version field") {
		t.Fatalf("expected field required error, got: %q", stderr)
	}
}

func TestLocalizationsUpdateAppInfoRequiresApp(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"localizations", "update", "--type", "app-info", "--locale", "en-US", "--subtitle", "test"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "--app is required") {
		t.Fatalf("expected app required error, got: %q", stderr)
	}
}

func TestLocalizationsUpdateVersionRequiresVersion(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"localizations", "update", "--locale", "en-US", "--description", "test"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "--version is required") {
		t.Fatalf("expected version required error, got: %q", stderr)
	}
}

func TestLocalizationsUpdateAppInfoSubtitle(t *testing.T) {
	setupLocUpdateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	var patchBody string
	http.DefaultTransport = locUpdateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		// Resolve app info ID
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appInfos":
			return locUpdateJSONResponse(`{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{}}]}`)

		// List existing localizations
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appInfos/appinfo-1/appInfoLocalizations":
			return locUpdateJSONResponse(`{"data":[{"type":"appInfoLocalizations","id":"loc-en","attributes":{"locale":"en-US","name":"MyApp","subtitle":"Old"}}],"links":{}}`)

		// Update localization
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appInfoLocalizations/loc-en":
			body, _ := io.ReadAll(req.Body)
			patchBody = string(body)
			return locUpdateJSONResponse(`{"data":{"type":"appInfoLocalizations","id":"loc-en","attributes":{"locale":"en-US","name":"MyApp","subtitle":"New Subtitle"}}}`)

		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"localizations", "update",
			"--type", "app-info",
			"--app", "app-1",
			"--locale", "en-US",
			"--subtitle", "New Subtitle",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	// Verify the PATCH body contains subtitle
	if !strings.Contains(patchBody, "New Subtitle") {
		t.Fatalf("expected subtitle in PATCH body, got: %s", patchBody)
	}

	// Verify JSON output
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v (stdout=%q)", err, stdout)
	}
}

func TestLocalizationsUpdateAppInfoFailsWhenAppInfoIsAmbiguous(t *testing.T) {
	setupLocUpdateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	http.DefaultTransport = locUpdateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appInfos":
			return locUpdateJSONResponse(`{"data":[
				{"type":"appInfos","id":"appinfo-live","attributes":{"state":"READY_FOR_SALE"}},
				{"type":"appInfos","id":"appinfo-rejected","attributes":{"state":"REJECTED"}}
			]}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"localizations", "update",
			"--type", "app-info",
			"--app", "app-1",
			"--locale", "en-US",
			"--subtitle", "New Subtitle",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected run error, got nil")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	for _, want := range []string{
		`multiple app infos found for app "app-1"`,
		`asc apps info list --app "app-1"`,
		"READY_FOR_SALE",
		"REJECTED",
	} {
		if !strings.Contains(runErr.Error(), want) {
			t.Fatalf("expected error to contain %q, got %v", want, runErr)
		}
	}
}

func TestLocalizationsUpdateVersionErrorIncludesAttemptedFields(t *testing.T) {
	setupLocUpdateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	http.DefaultTransport = locUpdateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/ver-1/appStoreVersionLocalizations":
			return locUpdateJSONResponse(`{"data":[{"type":"appStoreVersionLocalizations","id":"loc-fr","attributes":{"locale":"fr-FR","description":"Old","supportUrl":"https://example.com/support-old"}}],"links":{}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersionLocalizations/loc-fr":
			body := `{"errors":[{"status":"409","code":"ENTITY_ERROR.INVALID","title":"One or more parameters passed to the function were not valid.","detail":"(-50)"}]}`
			return &http.Response{
				StatusCode: http.StatusConflict,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"localizations", "update",
			"--version", "ver-1",
			"--locale", "fr-FR",
			"--description", "Updated description",
			"--support-url", "https://example.com/support-new",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected run error, got nil")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	for _, want := range []string{
		`localizations update: update version localization "fr-FR"`,
		"description",
		"supportUrl",
		"(-50)",
	} {
		if !strings.Contains(runErr.Error(), want) {
			t.Fatalf("expected error to contain %q, got %v", want, runErr)
		}
	}
}
