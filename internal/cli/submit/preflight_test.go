package submit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestSubmitPreflightCommand_MissingApp(t *testing.T) {
	// Ensure no app ID comes from env or config.
	t.Setenv("ASC_APP_ID", "")

	cmd := SubmitPreflightCommand()
	if err := cmd.FlagSet.Parse([]string{"--version", "1.0"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}
	// ResolveAppID may still find an ID from local config — skip Exec test
	// if that's the case, and only assert the command shape.
	resolved := shared.ResolveAppID("")
	if resolved != "" {
		t.Skip("local config provides an app ID; skipping flag validation test")
	}
	if err := cmd.Exec(context.Background(), nil); err != nil && !errors.Is(err, flag.ErrHelp) && !strings.Contains(err.Error(), "authentication") {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
}

func TestSubmitPreflightCommand_MissingVersion(t *testing.T) {
	setupSubmitAuth(t)

	cmd := SubmitPreflightCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "123"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}
	if err := cmd.Exec(context.Background(), nil); err != nil && !errors.Is(err, flag.ErrHelp) && !strings.Contains(err.Error(), "authentication") {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
}

func TestSubmitPreflightCommand_InvalidPlatform(t *testing.T) {
	setupSubmitAuth(t)

	cmd := SubmitPreflightCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "123", "--version", "1.0", "--platform", "INVALID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}
	err := cmd.Exec(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for invalid platform")
	}
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp for invalid platform, got %v", err)
	}
}

func TestSubmitPreflightCommand_Shape(t *testing.T) {
	cmd := SubmitPreflightCommand()
	if cmd.Name != "preflight" {
		t.Fatalf("unexpected command name: %q", cmd.Name)
	}
	if cmd.FlagSet == nil {
		t.Fatal("expected FlagSet to be set")
	}
}

func TestSubmitPreflightCommand_RejectsUnsupportedOutput(t *testing.T) {
	cmd := SubmitPreflightCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "123", "--version", "1.0", "--output", "table"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	var runErr error
	_, stderr := capturePreflightCommandOutput(t, func() {
		runErr = cmd.Exec(context.Background(), nil)
	})
	err := runErr
	if err == nil {
		t.Fatal("expected error for unsupported output")
	}
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %v", err)
	}
	if !strings.Contains(stderr, "unsupported format: table") {
		t.Fatalf("expected unsupported format message on stderr, got %q", stderr)
	}
}

func TestPreflightResult_TallyCounts(t *testing.T) {
	result := &preflightResult{
		Checks: []checkResult{
			{Name: "a", Passed: true},
			{Name: "b", Passed: false},
			{Name: "c", Passed: true},
			{Name: "d", Passed: false},
			{Name: "info", Passed: true, Advisory: true},
		},
	}
	tallyCounts(result)
	if result.PassCount != 3 {
		t.Fatalf("expected 3 passes including advisories, got %d", result.PassCount)
	}
	if result.FailCount != 2 {
		t.Fatalf("expected 2 failures, got %d", result.FailCount)
	}
}

func TestPreflightResult_AllPass(t *testing.T) {
	result := &preflightResult{
		Checks: []checkResult{
			{Name: "a", Passed: true},
			{Name: "b", Passed: true},
			{Name: "info", Passed: true, Advisory: true},
		},
	}
	tallyCounts(result)
	if result.PassCount != 3 {
		t.Fatalf("expected 3 passes including advisories, got %d", result.PassCount)
	}
	if result.FailCount != 0 {
		t.Fatalf("expected 0 failures, got %d", result.FailCount)
	}
}

func TestPrivacyPublishStateAdvisoryCheck_SetsPassedWhenPresent(t *testing.T) {
	check, ok := privacyPublishStateAdvisoryCheck("app-1")
	if !ok {
		t.Fatal("expected advisory check for non-empty app ID")
	}
	if !check.Advisory {
		t.Fatalf("expected advisory flag, got %+v", check)
	}
	if !check.Passed {
		t.Fatalf("expected advisory check to serialize as passed, got %+v", check)
	}
}

func TestPrivacyPublishStateAdvisoryCheck_SkipsBlankAppID(t *testing.T) {
	if _, ok := privacyPublishStateAdvisoryCheck(" \t "); ok {
		t.Fatal("expected blank app ID to skip advisory check")
	}
}

func TestAgeRatingMissingFields_AllSet(t *testing.T) {
	boolTrue := true
	strNone := "NONE"

	attrs := newAgeRatingAllSet(boolTrue, strNone)
	missing := ageRatingMissingFields(attrs)
	if len(missing) != 0 {
		t.Fatalf("expected no missing fields, got: %v", missing)
	}
}

func TestAgeRatingMissingFields_SomeMissing(t *testing.T) {
	boolTrue := true
	strNone := "NONE"

	attrs := newAgeRatingAllSet(boolTrue, strNone)
	// Unset a few
	attrs.Gambling = nil
	attrs.ViolenceRealistic = nil

	missing := ageRatingMissingFields(attrs)
	if len(missing) != 2 {
		t.Fatalf("expected 2 missing fields, got %d: %v", len(missing), missing)
	}

	found := map[string]bool{}
	for _, m := range missing {
		found[m] = true
	}
	if !found["gambling"] {
		t.Fatal("expected 'gambling' in missing fields")
	}
	if !found["violenceRealistic"] {
		t.Fatal("expected 'violenceRealistic' in missing fields")
	}
}

func TestAgeRatingMissingFields_AllMissing(t *testing.T) {
	attrs := asc.AgeRatingDeclarationAttributes{}
	missing := ageRatingMissingFields(attrs)
	// 3 boolean + 10 enum = 13
	if len(missing) != 13 {
		t.Fatalf("expected 13 missing fields, got %d: %v", len(missing), missing)
	}
}

func TestCheckBuildEncryption_NilAttrs(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
	}))
	check := checkBuildEncryption(context.Background(), client, "build-1", nil)
	if check.Passed {
		t.Fatal("expected fail when attrs is nil")
	}
	if !strings.Contains(check.Message, "not set") {
		t.Fatalf("unexpected message: %q", check.Message)
	}
}

func TestCheckBuildEncryption_NotSet(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
	}))
	attrs := &asc.BuildAttributes{Version: "1"}
	check := checkBuildEncryption(context.Background(), client, "build-1", attrs)
	if check.Passed {
		t.Fatal("expected fail when UsesNonExemptEncryption is nil")
	}
	if !strings.Contains(check.Hint, "App Store Connect") {
		t.Fatalf("expected App Store Connect hint, got %q", check.Hint)
	}
	if strings.Contains(check.Hint, "builds update") {
		t.Fatalf("did not expect nonexistent builds update command in hint, got %q", check.Hint)
	}
}

func TestCheckBuildEncryption_False(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
	}))
	enc := false
	attrs := &asc.BuildAttributes{Version: "1", UsesNonExemptEncryption: &enc}
	check := checkBuildEncryption(context.Background(), client, "build-1", attrs)
	if !check.Passed {
		t.Fatalf("expected pass when encryption=false, got: %q", check.Message)
	}
}

func TestCheckBuildEncryption_TrueWithApprovedDeclaration(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && req.URL.Path == "/v1/builds/build-1/appEncryptionDeclaration" {
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"appEncryptionDeclarations","id":"decl-1","attributes":{"appEncryptionDeclarationState":"APPROVED"}}}`)
		}
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
	}))
	enc := true
	attrs := &asc.BuildAttributes{Version: "1", UsesNonExemptEncryption: &enc}
	check := checkBuildEncryption(context.Background(), client, "build-1", attrs)
	if !check.Passed {
		t.Fatalf("expected pass when encryption=true with declaration, got: %q", check.Message)
	}
}

func TestCheckBuildEncryption_NonApprovedDeclarationStatesFail(t *testing.T) {
	tests := []struct {
		name        string
		response    string
		wantMessage string
	}{
		{
			name:        "created",
			response:    `{"data":{"type":"appEncryptionDeclarations","id":"decl-1","attributes":{"appEncryptionDeclarationState":"CREATED"}}}`,
			wantMessage: "CREATED",
		},
		{
			name:        "in review",
			response:    `{"data":{"type":"appEncryptionDeclarations","id":"decl-1","attributes":{"appEncryptionDeclarationState":"IN_REVIEW"}}}`,
			wantMessage: "IN_REVIEW",
		},
		{
			name:        "rejected",
			response:    `{"data":{"type":"appEncryptionDeclarations","id":"decl-1","attributes":{"appEncryptionDeclarationState":"REJECTED"}}}`,
			wantMessage: "REJECTED",
		},
		{
			name:        "invalid",
			response:    `{"data":{"type":"appEncryptionDeclarations","id":"decl-1","attributes":{"appEncryptionDeclarationState":"INVALID"}}}`,
			wantMessage: "INVALID",
		},
		{
			name:        "expired",
			response:    `{"data":{"type":"appEncryptionDeclarations","id":"decl-1","attributes":{"appEncryptionDeclarationState":"EXPIRED"}}}`,
			wantMessage: "EXPIRED",
		},
		{
			name:        "missing state",
			response:    `{"data":{"type":"appEncryptionDeclarations","id":"decl-1","attributes":{}}}`,
			wantMessage: "missing approval state",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method == http.MethodGet && req.URL.Path == "/v1/builds/build-1/appEncryptionDeclaration" {
					return submitJSONResponse(http.StatusOK, tt.response)
				}
				return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
			}))
			enc := true
			attrs := &asc.BuildAttributes{Version: "1", UsesNonExemptEncryption: &enc}
			check := checkBuildEncryption(context.Background(), client, "build-1", attrs)
			if check.Passed {
				t.Fatalf("expected fail for %s declaration state", tt.name)
			}
			if !strings.Contains(check.Message, tt.wantMessage) {
				t.Fatalf("expected %q in failure message, got %q", tt.wantMessage, check.Message)
			}
		})
	}
}

func TestCheckBuildEncryption_TrueWithoutDeclaration(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && req.URL.Path == "/v1/builds/build-1/appEncryptionDeclaration" {
			return submitJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		}
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
	}))
	enc := true
	attrs := &asc.BuildAttributes{Version: "1", UsesNonExemptEncryption: &enc}
	check := checkBuildEncryption(context.Background(), client, "build-1", attrs)
	if check.Passed {
		t.Fatal("expected fail when encryption=true but declaration missing")
	}
	if !strings.Contains(check.Message, "no encryption declaration attached") {
		t.Fatalf("unexpected message: %q", check.Message)
	}
}

func TestResolveAppInfoID_UsesVersionScopedResolution(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1":
			return submitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "appStoreVersions",
					"id": "version-1",
					"attributes": {"appStoreState": "PREPARE_FOR_SUBMISSION"},
					"relationships": {"app": {"data": {"type": "apps", "id": "app-1"}}}
				}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appInfos":
			return submitJSONResponse(http.StatusOK, `{
				"data": [
					{"type": "appInfos", "id": "info-z", "attributes": {"appStoreState": "PREPARE_FOR_SUBMISSION"}},
					{"type": "appInfos", "id": "info-a", "attributes": {"appStoreState": "READY_FOR_SALE"}}
				]
			}`)
		}
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
	}))

	appInfoID, err := resolveAppInfoID(context.Background(), client, "app-1", "version-1")
	if err != nil {
		t.Fatalf("expected version-scoped app info resolution to succeed, got %v", err)
	}
	if appInfoID != "info-z" {
		t.Fatalf("expected PREPARE_FOR_SUBMISSION app info, got %q", appInfoID)
	}
}

func TestCheckVersionExists_NotFound(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/appStoreVersions") {
			return submitJSONResponse(http.StatusOK, `{"data":[]}`)
		}
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
	}))

	versionID, check := checkVersionExists(context.Background(), client, "app-123", "9.9", "IOS")
	if versionID != "" {
		t.Fatalf("expected empty versionID, got %q", versionID)
	}
	if check.Passed {
		t.Fatal("expected check to fail")
	}
	if check.Hint == "" {
		t.Fatal("expected hint to be set")
	}
}

func TestRunPreflight_VersionFailureStillRunsAppLevelChecks(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		path := req.URL.Path

		switch {
		case req.Method == http.MethodGet && strings.Contains(path, "/appStoreVersions") && strings.Contains(path, "/apps/"):
			return submitJSONResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodGet && strings.HasSuffix(path, "/appInfos"):
			return submitJSONResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"info-1","attributes":{"appStoreState":"PREPARE_FOR_SUBMISSION"}}]}`)
		case req.Method == http.MethodGet && strings.HasSuffix(path, "/ageRatingDeclaration"):
			return submitJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case req.Method == http.MethodGet && path == "/v1/apps/app-1":
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"apps","id":"app-1","attributes":{"name":"Test","bundleId":"com.test","sku":"test"}}}`)
		case req.Method == http.MethodGet && strings.HasSuffix(path, "/primaryCategory"):
			return submitJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		}

		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, path)
	}))

	result := runPreflight(context.Background(), client, "app-1", "9.9", "IOS")
	checksByName := make(map[string]checkResult, len(result.Checks))
	for _, check := range result.Checks {
		checksByName[check.Name] = check
	}

	for _, name := range []string{"Version exists", "Age rating", "Content rights", "Primary category", "App Privacy"} {
		if _, ok := checksByName[name]; !ok {
			t.Fatalf("expected %q check to run even when version lookup fails", name)
		}
	}
	if _, ok := checksByName["Build attached"]; ok {
		t.Fatal("did not expect build check without a resolved version")
	}
	if result.FailCount != 4 {
		t.Fatalf("expected four app/version failures, got %d", result.FailCount)
	}
}

func checkBuildAttachedWrapper(ctx context.Context, client *asc.Client, versionID string) checkResult {
	_, _, check := checkBuildAttachedWithAttrs(ctx, client, versionID)
	return check
}

func TestCheckBuildAttached_NoBuild(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/build") {
			return submitJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		}
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
	}))

	check := checkBuildAttachedWrapper(context.Background(), client, "version-123")
	if check.Passed {
		t.Fatal("expected check to fail for missing build")
	}
}

func TestCheckBuildAttached_HasBuild(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/build") {
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42"}}}`)
		}
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
	}))

	check := checkBuildAttachedWrapper(context.Background(), client, "version-123")
	if !check.Passed {
		t.Fatalf("expected check to pass, got message: %s", check.Message)
	}
	if !strings.Contains(check.Message, "42") {
		t.Fatalf("expected build version in message, got: %s", check.Message)
	}
}

func TestCheckContentRights_NotSet(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/v1/apps/") {
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"apps","id":"app-123","attributes":{"name":"Test","bundleId":"com.test","sku":"test"}}}`)
		}
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
	}))

	check := checkContentRights(context.Background(), client, "app-123")
	if check.Passed {
		t.Fatal("expected check to fail when contentRightsDeclaration is nil")
	}
	if check.Hint != "asc apps update --id app-123 --content-rights DOES_NOT_USE_THIRD_PARTY_CONTENT" {
		t.Fatalf("unexpected hint: %q", check.Hint)
	}
}

func TestCheckContentRights_Set(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/v1/apps/") {
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"apps","id":"app-123","attributes":{"name":"Test","bundleId":"com.test","sku":"test","contentRightsDeclaration":"DOES_NOT_USE_THIRD_PARTY_CONTENT"}}}`)
		}
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
	}))

	check := checkContentRights(context.Background(), client, "app-123")
	if !check.Passed {
		t.Fatalf("expected check to pass, got message: %s", check.Message)
	}
}

func TestCheckLocalizationMetadata_UsesSubmitReadinessRulesPerLocale(t *testing.T) {
	check := checkLocalizationMetadata([]asc.Resource[asc.AppStoreVersionLocalizationAttributes]{
		{
			ID: "loc-en",
			Attributes: asc.AppStoreVersionLocalizationAttributes{
				Locale:      "en-US",
				Description: "Ready",
				Keywords:    "one,two",
				SupportURL:  "https://example.com/support",
			},
		},
		{
			ID: "loc-fr",
			Attributes: asc.AppStoreVersionLocalizationAttributes{
				Locale:      "fr-FR",
				Description: "Pret",
				Keywords:    "un,deux",
			},
		},
	}, "app-123", "1.0", "IOS", shared.SubmitReadinessOptions{})

	if check.Passed {
		t.Fatal("expected localization metadata check to fail")
	}
	if !strings.Contains(check.Message, "fr-FR (supportUrl)") {
		t.Fatalf("expected missing supportUrl to be reported, got %q", check.Message)
	}
	if check.Hint != "asc metadata push --app app-123 --version 1.0 --platform IOS --dir ./metadata" {
		t.Fatalf("expected metadata push hint, got %q", check.Hint)
	}
}

func TestCheckScreenshots_PartialFetchErrorsAreReported(t *testing.T) {
	localizations := []asc.Resource[asc.AppStoreVersionLocalizationAttributes]{
		{ID: "loc-1", Attributes: asc.AppStoreVersionLocalizationAttributes{Locale: "en-US"}},
		{ID: "loc-2", Attributes: asc.AppStoreVersionLocalizationAttributes{Locale: "fr-FR"}},
	}
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/appStoreVersionLocalizations/loc-1/appScreenshotSets":
			return submitJSONResponse(http.StatusBadGateway, `{"errors":[{"status":"502","code":"BAD_GATEWAY","title":"Bad Gateway"}]}`)
		case "/v1/appStoreVersionLocalizations/loc-2/appScreenshotSets":
			return submitJSONResponse(http.StatusOK, `{"data":[]}`)
		}
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
	}))

	check := checkScreenshots(context.Background(), client, localizations, "IOS")
	if check.Passed {
		t.Fatal("expected screenshot check to fail")
	}
	if !strings.Contains(check.Message, "Could not fully verify screenshots") {
		t.Fatalf("expected fetch error message, got %q", check.Message)
	}
	if check.Hint != "" {
		t.Fatalf("expected no upload hint when screenshot verification is incomplete, got %q", check.Hint)
	}
}

func TestScreenshotUploadHint_UsesPlatformDefaults(t *testing.T) {
	tests := []struct {
		platform string
		want     string
	}{
		{platform: "IOS", want: "IPHONE_65"},
		{platform: "MAC_OS", want: "DESKTOP"},
		{platform: "TV_OS", want: "APPLE_TV"},
		{platform: "VISION_OS", want: "APPLE_VISION_PRO"},
	}

	for _, tt := range tests {
		t.Run(tt.platform, func(t *testing.T) {
			hint := screenshotUploadHint("loc-1", tt.platform)
			if !strings.Contains(hint, tt.want) {
				t.Fatalf("expected %q device type in hint, got %q", tt.want, hint)
			}
		})
	}
}

func TestRunPreflight_AllPass(t *testing.T) {
	setupSubmitAuth(t)

	client := newSubmitTestClient(t, submitAllPassTransport())

	result := runPreflight(context.Background(), client, "app-1", "1.0", "IOS")
	if result.FailCount != 0 {
		for _, c := range result.Checks {
			if !c.Passed && !c.Advisory {
				t.Errorf("check %q failed: %s (hint: %s)", c.Name, c.Message, c.Hint)
			}
		}
		t.Fatalf("expected 0 failures, got %d", result.FailCount)
	}
	foundPrivacyAdvisory := false
	for _, check := range result.Checks {
		if check.Name == "App Privacy" {
			foundPrivacyAdvisory = true
			if !check.Advisory {
				t.Fatalf("expected App Privacy check to be advisory, got %+v", check)
			}
			if !check.Passed {
				t.Fatalf("expected App Privacy advisory to serialize as passed, got %+v", check)
			}
			if strings.Contains(strings.ToLower(check.Hint), "asc web") {
				t.Fatalf("did not expect web command hint in advisory, got %q", check.Hint)
			}
		}
	}
	if !foundPrivacyAdvisory {
		t.Fatalf("expected App Privacy advisory in result, got %+v", result.Checks)
	}
}

func TestPreflightTextOutput(t *testing.T) {
	// Ensure printPreflightText doesn't panic with various result shapes.
	var buf bytes.Buffer
	printPreflightText(&buf, &preflightResult{
		AppID:    "123",
		Version:  "1.0",
		Platform: "IOS",
		Checks: []checkResult{
			{Name: "Version exists", Passed: true, Message: "Version 1.0 found"},
			{Name: "Build attached", Passed: false, Message: "No build", Hint: "asc submit create ..."},
			{Name: "App Privacy", Advisory: true, Message: "App Privacy publish state is not verifiable via the public App Store Connect API and may still block submission", Hint: "Confirm App Privacy is published in App Store Connect before submitting: https://appstoreconnect.apple.com/apps/123/appPrivacy"},
		},
		PassCount: 1,
		FailCount: 1,
	})
	if !strings.Contains(buf.String(), "Preflight check for app 123 v1.0 (IOS)") {
		t.Fatalf("expected header in text output, got %q", buf.String())
	}
	if !strings.Contains(buf.String(), "App Privacy publish state is not verifiable via the public App Store Connect API") {
		t.Fatalf("expected advisory in text output, got %q", buf.String())
	}
}

func TestPreflightTextOutput_AdvisoryOnlyDoesNotClaimReadyToSubmit(t *testing.T) {
	var buf bytes.Buffer
	printPreflightText(&buf, &preflightResult{
		AppID:    "123",
		Version:  "1.0",
		Platform: "IOS",
		Checks: []checkResult{
			{
				Name:     "App Privacy",
				Passed:   true,
				Advisory: true,
				Message:  "App Privacy publish state is not verifiable via the public App Store Connect API and may still block submission",
				Hint:     "Confirm App Privacy is published in App Store Connect before submitting: https://appstoreconnect.apple.com/apps/123/appPrivacy",
			},
		},
	})

	output := buf.String()
	if strings.Contains(output, "Ready to submit") {
		t.Fatalf("did not expect advisory-only result to claim readiness, got %q", output)
	}
	if !strings.Contains(output, "Result: Required checks passed, but 1 advisory should be reviewed before submitting.") {
		t.Fatalf("expected advisory summary in text output, got %q", output)
	}
}

func TestSubmitPreflightCommand_AllPassIncludesPrivacyAdvisory(t *testing.T) {
	setupSubmitAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = submitAllPassTransport()

	cmd := SubmitPreflightCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--app", "app-1", "--version", "1.0", "--output", "text"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	var runErr error
	stdout, stderr := capturePreflightCommandOutput(t, func() {
		runErr = cmd.Exec(context.Background(), nil)
	})
	if runErr != nil {
		t.Fatalf("expected success when only advisory is present, got %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "App Privacy publish state is not verifiable via the public App Store Connect API") {
		t.Fatalf("expected App Privacy advisory in stdout, got %q", stdout)
	}
	if strings.Contains(strings.ToLower(stdout), "asc web") {
		t.Fatalf("did not expect private/web command references in stdout, got %q", stdout)
	}
}

func TestSubmitPreflightCommand_JSONAllPassIncludesPrivacyAdvisoryAsPassed(t *testing.T) {
	setupSubmitAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = submitAllPassTransport()

	cmd := SubmitPreflightCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--app", "app-1", "--version", "1.0", "--output", "json"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	var (
		runErr error
		result preflightResult
	)
	stdout, stderr := capturePreflightCommandOutput(t, func() {
		runErr = cmd.Exec(context.Background(), nil)
	})
	if runErr != nil {
		t.Fatalf("expected success when only advisory is present, got %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("failed to parse JSON output %q: %v", stdout, err)
	}
	if result.FailCount != 0 {
		t.Fatalf("expected advisory-only JSON result to stay non-blocking, got %+v", result)
	}
	if result.PassCount != len(result.Checks) {
		t.Fatalf("expected advisory-only JSON pass count to include every check, got %+v", result)
	}

	foundPrivacyAdvisory := false
	for _, check := range result.Checks {
		if check.Name != "App Privacy" {
			continue
		}
		foundPrivacyAdvisory = true
		if !check.Advisory {
			t.Fatalf("expected App Privacy advisory in JSON output, got %+v", check)
		}
		if !check.Passed {
			t.Fatalf("expected App Privacy advisory to serialize as passed, got %+v", check)
		}
	}
	if !foundPrivacyAdvisory {
		t.Fatalf("expected App Privacy advisory in JSON output, got %+v", result.Checks)
	}
}

func TestSubmitPreflightCommand_JSONOutput(t *testing.T) {
	setupSubmitAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/appStoreVersions"):
			return submitJSONResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/appInfos"):
			return submitJSONResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"info-1","attributes":{"appStoreState":"PREPARE_FOR_SUBMISSION"}}]}`)
		case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/ageRatingDeclaration"):
			return submitJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/123":
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"apps","id":"123","attributes":{"name":"Test","bundleId":"com.test","sku":"test"}}}`)
		case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/primaryCategory"):
			return submitJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		}
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
	})

	cmd := SubmitPreflightCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--app", "123", "--version", "1.0", "--output", "json"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error when version not found")
	}
	if !strings.Contains(err.Error(), "issue(s) found") {
		t.Fatalf("expected preflight failure error, got: %v", err)
	}
}

func TestSubmitPreflightCommand_TextOutput(t *testing.T) {
	setupSubmitAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/appStoreVersions"):
			return submitJSONResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/appInfos"):
			return submitJSONResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"info-1","attributes":{"appStoreState":"PREPARE_FOR_SUBMISSION"}}]}`)
		case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/ageRatingDeclaration"):
			return submitJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/123":
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"apps","id":"123","attributes":{"name":"Test","bundleId":"com.test","sku":"test"}}}`)
		case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/primaryCategory"):
			return submitJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		}
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
	})

	cmd := SubmitPreflightCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--app", "123", "--version", "1.0", "--output", "text"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	var runErr error
	stdout, _ := capturePreflightCommandOutput(t, func() {
		runErr = cmd.Exec(context.Background(), nil)
	})
	if runErr == nil {
		t.Fatal("expected error when version not found")
	}
	if !strings.Contains(runErr.Error(), "issue(s) found") {
		t.Fatalf("expected preflight failure error, got: %v", runErr)
	}
	if !strings.Contains(stdout, "Preflight check for app 123 v1.0 (IOS)") {
		t.Fatalf("expected text output header, got %q", stdout)
	}
	if !strings.Contains(stdout, "asc age-rating edit --app 123 --gambling false --violence-realistic NONE ...") {
		t.Fatalf("expected text output to suggest age-rating edit, got %q", stdout)
	}
	if strings.Contains(stdout, "asc age-rating set --app 123") {
		t.Fatalf("expected text output to avoid stale age-rating set hint, got %q", stdout)
	}
}

// --- Helpers ---

func capturePreflightCommandOutput(t *testing.T, fn func()) (string, string) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe error: %v", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe error: %v", err)
	}

	os.Stdout = stdoutW
	os.Stderr = stderrW

	stdoutCh := make(chan string, 1)
	stderrCh := make(chan string, 1)

	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, stdoutR)
		_ = stdoutR.Close()
		stdoutCh <- buf.String()
	}()
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, stderrR)
		_ = stderrR.Close()
		stderrCh <- buf.String()
	}()

	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
		_ = stdoutW.Close()
		_ = stderrW.Close()
	}()

	fn()

	_ = stdoutW.Close()
	_ = stderrW.Close()

	return <-stdoutCh, <-stderrCh
}

func newAgeRatingAllSet(boolVal bool, strVal string) asc.AgeRatingDeclarationAttributes {
	return asc.AgeRatingDeclarationAttributes{
		Gambling:                                    &boolVal,
		LootBox:                                     &boolVal,
		UnrestrictedWebAccess:                       &boolVal,
		AlcoholTobaccoOrDrugUseOrReferences:         &strVal,
		GamblingSimulated:                           &strVal,
		HorrorOrFearThemes:                          &strVal,
		MatureOrSuggestiveThemes:                    &strVal,
		ProfanityOrCrudeHumor:                       &strVal,
		SexualContentGraphicAndNudity:               &strVal,
		SexualContentOrNudity:                       &strVal,
		ViolenceCartoonOrFantasy:                    &strVal,
		ViolenceRealistic:                           &strVal,
		ViolenceRealisticProlongedGraphicOrSadistic: &strVal,
	}
}

func submitAllPassTransport() http.RoundTripper {
	return submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		path := req.URL.Path

		switch {
		case req.Method == http.MethodGet && strings.Contains(path, "/appStoreVersions") && strings.Contains(path, "/apps/"):
			return submitJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "appStoreVersions",
					"id": "version-1",
					"attributes": {"platform": "IOS", "versionString": "1.0"}
				}]
			}`)
		case req.Method == http.MethodGet && path == "/v1/appStoreVersions/version-1":
			return submitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "appStoreVersions",
					"id": "version-1",
					"attributes": {"appStoreState": "PREPARE_FOR_SUBMISSION", "platform": "IOS", "versionString": "1.0"},
					"relationships": {"app": {"data": {"type": "apps", "id": "app-1"}}}
				}
			}`)
		case req.Method == http.MethodGet && strings.HasSuffix(path, "/build"):
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"1","usesNonExemptEncryption":false}}}`)
		case req.Method == http.MethodGet && strings.HasSuffix(path, "/appInfos"):
			return submitJSONResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"info-1","attributes":{"appStoreState":"PREPARE_FOR_SUBMISSION"}}]}`)
		case req.Method == http.MethodGet && strings.HasSuffix(path, "/ageRatingDeclaration"):
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"ageRatingDeclarations","id":"rating-1","attributes":{
				"gambling": false, "lootBox": false, "unrestrictedWebAccess": false,
				"alcoholTobaccoOrDrugUseOrReferences": "NONE", "gamblingSimulated": "NONE",
				"horrorOrFearThemes": "NONE", "matureOrSuggestiveThemes": "NONE",
				"profanityOrCrudeHumor": "NONE", "sexualContentGraphicAndNudity": "NONE",
				"sexualContentOrNudity": "NONE", "violenceCartoonOrFantasy": "NONE",
				"violenceRealistic": "NONE", "violenceRealisticProlongedGraphicOrSadistic": "NONE"
			}}}`)
		case req.Method == http.MethodGet && path == "/v1/apps/app-1":
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"apps","id":"app-1","attributes":{"name":"Test","bundleId":"com.test","sku":"test","contentRightsDeclaration":"DOES_NOT_USE_THIRD_PARTY_CONTENT"}}}`)
		case req.Method == http.MethodGet && strings.HasSuffix(path, "/primaryCategory"):
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"appCategories","id":"SPORTS","attributes":{}}}`)
		case req.Method == http.MethodGet && strings.Contains(path, "/appStoreVersionLocalizations"):
			return submitJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "appStoreVersionLocalizations",
					"id": "loc-1",
					"attributes": {
						"locale": "en-US",
						"description": "A great app",
						"keywords": "test,app",
						"supportUrl": "https://example.com/support",
						"whatsNew": "Bug fixes"
					}
				}]
			}`)
		case req.Method == http.MethodGet && strings.Contains(path, "/appScreenshotSets"):
			return submitJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "appScreenshotSets",
					"id": "ss-1",
					"attributes": {"screenshotDisplayType": "APP_IPHONE_67"}
				}]
			}`)
		}

		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, path)
	})
}
