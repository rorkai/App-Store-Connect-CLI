package shared

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestMissingSubmitRequiredLocalizationFields_AllEmpty(t *testing.T) {
	attrs := asc.AppStoreVersionLocalizationAttributes{Locale: "en-US"}
	missing := MissingSubmitRequiredLocalizationFieldsWithOptions(attrs, SubmitReadinessOptions{})
	want := []string{"description", "keywords", "supportUrl"}
	if len(missing) != len(want) {
		t.Fatalf("expected %v, got %v", want, missing)
	}
	for i, field := range want {
		if missing[i] != field {
			t.Fatalf("expected field %q at index %d, got %q", field, i, missing[i])
		}
	}
}

func TestMissingSubmitRequiredLocalizationFields_DoesNotCheckWhatsNew(t *testing.T) {
	attrs := asc.AppStoreVersionLocalizationAttributes{
		Locale:      "en-US",
		Description: "A great app",
		Keywords:    "quran,islam",
		SupportURL:  "https://example.com",
		WhatsNew:    "", // empty but should not be flagged without RequireWhatsNew
	}
	missing := MissingSubmitRequiredLocalizationFieldsWithOptions(attrs, SubmitReadinessOptions{})
	if len(missing) != 0 {
		t.Fatalf("expected no missing fields without RequireWhatsNew, got %v", missing)
	}
}

func TestMissingSubmitRequiredLocalizationFieldsWithOptions_WhatsNewRequired(t *testing.T) {
	attrs := asc.AppStoreVersionLocalizationAttributes{
		Locale:      "en-US",
		Description: "A great app",
		Keywords:    "quran,islam",
		SupportURL:  "https://example.com",
		WhatsNew:    "",
	}
	opts := SubmitReadinessOptions{RequireWhatsNew: true}
	missing := MissingSubmitRequiredLocalizationFieldsWithOptions(attrs, opts)
	if len(missing) != 1 || missing[0] != "whatsNew" {
		t.Fatalf("expected [whatsNew], got %v", missing)
	}
}

func TestMissingSubmitRequiredLocalizationFieldsWithOptions_WhatsNewPresent(t *testing.T) {
	attrs := asc.AppStoreVersionLocalizationAttributes{
		Locale:      "en-US",
		Description: "A great app",
		Keywords:    "quran,islam",
		SupportURL:  "https://example.com",
		WhatsNew:    "Bug fixes and improvements",
	}
	opts := SubmitReadinessOptions{RequireWhatsNew: true}
	missing := MissingSubmitRequiredLocalizationFieldsWithOptions(attrs, opts)
	if len(missing) != 0 {
		t.Fatalf("expected no missing fields, got %v", missing)
	}
}

func TestSubmitReadinessIssuesByLocaleWithOptions_WhatsNewMixedLocales(t *testing.T) {
	localizations := []asc.Resource[asc.AppStoreVersionLocalizationAttributes]{
		{
			ID: "loc-1",
			Attributes: asc.AppStoreVersionLocalizationAttributes{
				Locale:      "en-US",
				Description: "English description",
				Keywords:    "app,test",
				SupportURL:  "https://example.com",
				WhatsNew:    "Bug fixes",
			},
		},
		{
			ID: "loc-2",
			Attributes: asc.AppStoreVersionLocalizationAttributes{
				Locale:      "ar-SA",
				Description: "Arabic description",
				Keywords:    "تطبيق",
				SupportURL:  "https://example.com",
				WhatsNew:    "", // missing
			},
		},
		{
			ID: "loc-3",
			Attributes: asc.AppStoreVersionLocalizationAttributes{
				Locale:      "fr-FR",
				Description: "French description",
				Keywords:    "application",
				SupportURL:  "https://example.com",
				WhatsNew:    "  ", // whitespace-only
			},
		},
	}

	opts := SubmitReadinessOptions{RequireWhatsNew: true}
	issues := SubmitReadinessIssuesByLocaleWithOptions(localizations, opts)

	if len(issues) != 2 {
		t.Fatalf("expected 2 issues (ar-SA, fr-FR), got %d: %v", len(issues), issues)
	}

	// Issues should be sorted by locale
	if issues[0].Locale != "ar-SA" {
		t.Fatalf("expected first issue locale ar-SA, got %q", issues[0].Locale)
	}
	if issues[1].Locale != "fr-FR" {
		t.Fatalf("expected second issue locale fr-FR, got %q", issues[1].Locale)
	}

	for _, issue := range issues {
		if len(issue.MissingFields) != 1 || issue.MissingFields[0] != "whatsNew" {
			t.Fatalf("expected [whatsNew] for %s, got %v", issue.Locale, issue.MissingFields)
		}
	}
}

func TestSubmitReadinessIssuesByLocale_BackwardCompatible(t *testing.T) {
	localizations := []asc.Resource[asc.AppStoreVersionLocalizationAttributes]{
		{
			ID: "loc-1",
			Attributes: asc.AppStoreVersionLocalizationAttributes{
				Locale:      "en-US",
				Description: "desc",
				Keywords:    "kw",
				SupportURL:  "https://example.com",
				WhatsNew:    "", // empty but should not be flagged by default
			},
		},
	}

	issues := SubmitReadinessIssuesByLocaleWithOptions(localizations, SubmitReadinessOptions{})
	if len(issues) != 0 {
		t.Fatalf("expected no issues from backward-compatible call, got %v", issues)
	}
}

func TestSubmitReadinessCreateWarningForLocale_ReturnsWarningForIncompleteCreate(t *testing.T) {
	attrs := asc.AppStoreVersionLocalizationAttributes{
		Locale:   "fr-FR",
		Keywords: "journal,humeur",
	}

	warning, ok := SubmitReadinessCreateWarningForLocaleWithOptions("", attrs, SubmitReadinessCreateModeApplied, SubmitReadinessOptions{})
	if !ok {
		t.Fatal("expected warning")
	}
	if warning.Locale != "fr-FR" {
		t.Fatalf("expected locale fr-FR, got %q", warning.Locale)
	}
	if warning.Mode != SubmitReadinessCreateModeApplied {
		t.Fatalf("expected applied mode, got %q", warning.Mode)
	}
	want := []string{"description", "supportUrl"}
	if len(warning.MissingFields) != len(want) {
		t.Fatalf("expected %v, got %v", want, warning.MissingFields)
	}
	for i := range want {
		if warning.MissingFields[i] != want[i] {
			t.Fatalf("expected missing field %q at index %d, got %q", want[i], i, warning.MissingFields[i])
		}
	}
}

func TestSubmitReadinessCreateWarningForLocale_ReturnsFalseForCompleteCreate(t *testing.T) {
	attrs := asc.AppStoreVersionLocalizationAttributes{
		Locale:      "en-US",
		Description: "Description",
		Keywords:    "app,test",
		SupportURL:  "https://example.com/support",
	}

	if _, ok := SubmitReadinessCreateWarningForLocaleWithOptions("", attrs, SubmitReadinessCreateModePlanned, SubmitReadinessOptions{}); ok {
		t.Fatal("expected no warning")
	}
}

func TestSubmitReadinessCreateWarningForLocaleWithOptions_RequiresWhatsNew(t *testing.T) {
	attrs := asc.AppStoreVersionLocalizationAttributes{
		Locale:      "en-US",
		Description: "Description",
		Keywords:    "app,test",
		SupportURL:  "https://example.com/support",
	}

	warning, ok := SubmitReadinessCreateWarningForLocaleWithOptions(
		"",
		attrs,
		SubmitReadinessCreateModeApplied,
		SubmitReadinessOptions{RequireWhatsNew: true},
	)
	if !ok {
		t.Fatal("expected warning")
	}
	if warning.Locale != "en-US" {
		t.Fatalf("expected locale en-US, got %q", warning.Locale)
	}
	if warning.Mode != SubmitReadinessCreateModeApplied {
		t.Fatalf("expected applied mode, got %q", warning.Mode)
	}
	if len(warning.MissingFields) != 1 || warning.MissingFields[0] != "whatsNew" {
		t.Fatalf("expected missing fields [whatsNew], got %+v", warning.MissingFields)
	}
}

func TestNormalizeSubmitReadinessCreateWarnings_SortsAndDedupes(t *testing.T) {
	warnings := []SubmitReadinessCreateWarning{
		{
			Locale:        "fr-FR",
			Mode:          SubmitReadinessCreateModeApplied,
			MissingFields: []string{"supportUrl"},
		},
		{
			Locale:        "EN-us",
			Mode:          SubmitReadinessCreateModePlanned,
			MissingFields: []string{"keywords"},
		},
		{
			Locale:        "en-US",
			Mode:          SubmitReadinessCreateModePlanned,
			MissingFields: []string{"description", "keywords"},
		},
		{
			Locale:        "fr-FR",
			Mode:          SubmitReadinessCreateModeApplied,
			MissingFields: []string{"description"},
		},
	}

	normalized := NormalizeSubmitReadinessCreateWarnings(warnings)
	if len(normalized) != 2 {
		t.Fatalf("expected 2 warnings, got %d: %+v", len(normalized), normalized)
	}
	if normalized[0].Locale != "EN-us" || normalized[0].Mode != SubmitReadinessCreateModePlanned {
		t.Fatalf("expected planned EN-us warning first, got %+v", normalized[0])
	}
	if normalized[1].Locale != "fr-FR" || normalized[1].Mode != SubmitReadinessCreateModeApplied {
		t.Fatalf("expected applied fr-FR warning second, got %+v", normalized[1])
	}
	wantSecond := []string{"description", "supportUrl"}
	if len(normalized[1].MissingFields) != len(wantSecond) {
		t.Fatalf("expected merged fields %v, got %v", wantSecond, normalized[1].MissingFields)
	}
	for i := range wantSecond {
		if normalized[1].MissingFields[i] != wantSecond[i] {
			t.Fatalf("expected missing field %q at index %d, got %q", wantSecond[i], i, normalized[1].MissingFields[i])
		}
	}
}

func TestFormatSubmitReadinessCreateWarning_DistinguishesMode(t *testing.T) {
	planned := FormatSubmitReadinessCreateWarning(SubmitReadinessCreateWarning{
		Locale:        "de-DE",
		Mode:          SubmitReadinessCreateModePlanned,
		MissingFields: []string{"description", "supportUrl"},
	})
	if !strings.Contains(planned, "creating locale de-DE would make it participate in submission validation") {
		t.Fatalf("expected planned wording, got %q", planned)
	}

	applied := FormatSubmitReadinessCreateWarning(SubmitReadinessCreateWarning{
		Locale:        "de-DE",
		Mode:          SubmitReadinessCreateModeApplied,
		MissingFields: []string{"description", "supportUrl"},
	})
	if !strings.Contains(applied, "created locale de-DE now participates in submission validation") {
		t.Fatalf("expected applied wording, got %q", applied)
	}
}

func TestPrintSubmitReadinessCreateWarnings_NormalizesBeforePrinting(t *testing.T) {
	var stderr bytes.Buffer
	err := PrintSubmitReadinessCreateWarnings(&stderr, []SubmitReadinessCreateWarning{
		{
			Locale:        "fr-FR",
			Mode:          SubmitReadinessCreateModeApplied,
			MissingFields: []string{"supportUrl"},
		},
		{
			Locale:        "fr-FR",
			Mode:          SubmitReadinessCreateModeApplied,
			MissingFields: []string{"description"},
		},
		{
			Locale:        "en-US",
			Mode:          SubmitReadinessCreateModePlanned,
			MissingFields: []string{"keywords"},
		},
	})
	if err != nil {
		t.Fatalf("PrintSubmitReadinessCreateWarnings() error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(stderr.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 warning lines, got %d: %q", len(lines), stderr.String())
	}
	if !strings.Contains(lines[0], "en-US") || !strings.Contains(lines[0], "creating locale") {
		t.Fatalf("expected planned en-US warning first, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "fr-FR") || !strings.Contains(lines[1], "description, supportUrl") {
		t.Fatalf("expected merged fr-FR warning second, got %q", lines[1])
	}
}

func TestResolveSubmitReadinessOptionsForVersion_UsesProvidedAppAndPlatform(t *testing.T) {
	versionRequests := 0
	versionsRequests := 0
	client := newSubmitReadinessTestClient(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-123/appStoreVersions":
			versionsRequests++
			query := req.URL.Query()
			if query.Get("filter[platform]") != "IOS" {
				t.Fatalf("expected filter[platform]=IOS, got %q", query.Get("filter[platform]"))
			}
			if query.Get("filter[appStoreState]") != "READY_FOR_SALE,DEVELOPER_REMOVED_FROM_SALE,REMOVED_FROM_SALE" {
				t.Fatalf("unexpected state filter %q", query.Get("filter[appStoreState]"))
			}
			if query.Get("limit") != "1" {
				t.Fatalf("expected limit=1, got %q", query.Get("limit"))
			}
			return submitReadinessJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"live-1","attributes":{"platform":"IOS"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1":
			versionRequests++
			t.Fatalf("did not expect GetAppStoreVersion request when app and platform are provided")
			return nil, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	opts, err := ResolveSubmitReadinessOptionsForVersion(context.Background(), client, "version-1", "app-123", "IOS")
	if err != nil {
		t.Fatalf("ResolveSubmitReadinessOptionsForVersion() error: %v", err)
	}
	if !opts.RequireWhatsNew {
		t.Fatalf("expected RequireWhatsNew=true when released versions exist, got %+v", opts)
	}
	if versionRequests != 0 {
		t.Fatalf("expected zero version detail requests, got %d", versionRequests)
	}
	if versionsRequests != 1 {
		t.Fatalf("expected one app versions request, got %d", versionsRequests)
	}
}

func TestResolveSubmitReadinessOptionsForVersion_ResolvesMissingContextFromVersion(t *testing.T) {
	client := newSubmitReadinessTestClient(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-2":
			if req.URL.Query().Get("include") != "app" {
				t.Fatalf("expected include=app, got %q", req.URL.Query().Get("include"))
			}
			return submitReadinessJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-2","attributes":{"platform":"MAC_OS"},"relationships":{"app":{"data":{"type":"apps","id":"app-999"}}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-999/appStoreVersions":
			query := req.URL.Query()
			if query.Get("filter[platform]") != "MAC_OS" {
				t.Fatalf("expected filter[platform]=MAC_OS, got %q", query.Get("filter[platform]"))
			}
			return submitReadinessJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	opts, err := ResolveSubmitReadinessOptionsForVersion(context.Background(), client, "version-2", "", "")
	if err != nil {
		t.Fatalf("ResolveSubmitReadinessOptionsForVersion() error: %v", err)
	}
	if opts.RequireWhatsNew {
		t.Fatalf("expected RequireWhatsNew=false when no released versions exist, got %+v", opts)
	}
}

func TestResolveSubmitReadinessOptionsForVersion_RequiresVersionIDWhenContextUnknown(t *testing.T) {
	client := newSubmitReadinessTestClient(t, func(req *http.Request) (*http.Response, error) {
		t.Fatalf("did not expect request for validation failure, got %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	_, err := ResolveSubmitReadinessOptionsForVersion(context.Background(), client, "", "", "")
	if err == nil {
		t.Fatal("expected validation error for missing version id")
	}
	if !strings.Contains(err.Error(), "version id is required when app or platform is unknown") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveSubmitReadinessOptionsForVersionBestEffort_SwallowsResolutionError(t *testing.T) {
	client := newSubmitReadinessTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/appStoreVersions/version-3" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		return submitReadinessJSONResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","detail":"temporary failure"}]}`)
	})

	opts := ResolveSubmitReadinessOptionsForVersionBestEffort(context.Background(), client, "version-3", "", "")
	if opts.RequireWhatsNew {
		t.Fatalf("expected best-effort fallback to return zero options on error, got %+v", opts)
	}
}

type submitReadinessRoundTripFunc func(*http.Request) (*http.Response, error)

func (f submitReadinessRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newSubmitReadinessTestClient(t *testing.T, transport submitReadinessRoundTripFunc) *asc.Client {
	t.Helper()

	keyPath := filepath.Join(t.TempDir(), "key.p8")
	writeECDSAPEM(t, keyPath)

	httpClient := &http.Client{Transport: transport}
	client, err := asc.NewClientWithHTTPClient("KEY123", "ISS456", keyPath, httpClient)
	if err != nil {
		t.Fatalf("NewClientWithHTTPClient() error: %v", err)
	}
	return client
}

func submitReadinessJSONResponse(status int, body string) (*http.Response, error) {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}
