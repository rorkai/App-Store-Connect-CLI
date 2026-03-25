package apps

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCollectCommunityWallSubmitInputAllowsAppIDOnlyWhenNonInteractive(t *testing.T) {
	previousPromptEnabled := communityWallPromptEnabled
	communityWallPromptEnabled = func() bool { return false }
	t.Cleanup(func() { communityWallPromptEnabled = previousPromptEnabled })

	input, err := collectCommunityWallSubmitInput(
		"1234567890",
		"",
		"",
	)
	if err != nil {
		t.Fatalf("collect input: %v", err)
	}

	if input.AppID != "1234567890" {
		t.Fatalf("expected app ID to be preserved, got %q", input.AppID)
	}
}

func TestCollectCommunityWallSubmitInputNormalizesAppStoreIDPrefix(t *testing.T) {
	previousPromptEnabled := communityWallPromptEnabled
	communityWallPromptEnabled = func() bool { return false }
	t.Cleanup(func() { communityWallPromptEnabled = previousPromptEnabled })

	input, err := collectCommunityWallSubmitInput("id1234567890", "", "")
	if err != nil {
		t.Fatalf("collect input: %v", err)
	}

	if input.AppID != "1234567890" {
		t.Fatalf("expected app ID prefix to be normalized, got %q", input.AppID)
	}
}

func TestNormalizeCommunityWallAppID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "strips lowercase id prefix",
			input: "id1234567890",
			want:  "1234567890",
		},
		{
			name:  "strips uppercase id prefix",
			input: "ID1234567890",
			want:  "1234567890",
		},
		{
			name:  "trims whitespace",
			input: "  1234567890  ",
			want:  "1234567890",
		},
		{
			name:  "keeps bare numeric id",
			input: "1234567890",
			want:  "1234567890",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := normalizeCommunityWallAppID(test.input); got != test.want {
				t.Fatalf("normalizeCommunityWallAppID(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestCommunityWallAppStoreURLUsesBareAppID(t *testing.T) {
	got := communityWallAppStoreURL("1234567890")
	want := "https://apps.apple.com/app/id1234567890"
	if got != want {
		t.Fatalf("communityWallAppStoreURL() = %q, want %q", got, want)
	}
}

func TestCommunityWallAppStoreURLCanonicalizesZeroPaddedID(t *testing.T) {
	got := communityWallAppStoreURL("00123")
	want := "https://apps.apple.com/app/id123"
	if got != want {
		t.Fatalf("communityWallAppStoreURL() = %q, want %q", got, want)
	}
}

func TestResolveCommunityWallCandidateCanonicalizesFallbackAppStoreURL(t *testing.T) {
	previousLookupDetails := communityWallLookupAppDetails
	communityWallLookupAppDetails = func(ctx context.Context, ids []string) (map[string]communityWallAppDetails, error) {
		return map[string]communityWallAppDetails{
			"00123": {
				Name: "Beta",
				Link: "",
			},
		}, nil
	}
	t.Cleanup(func() {
		communityWallLookupAppDetails = previousLookupDetails
	})

	candidate, warnings, err := resolveCommunityWallCandidate(context.Background(), communityWallSubmitInput{
		AppID: "00123",
	})
	if err != nil {
		t.Fatalf("resolve candidate: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %+v", warnings)
	}
	if candidate.App != "Beta" {
		t.Fatalf("App = %q, want Beta", candidate.App)
	}
	if candidate.Link != "https://apps.apple.com/app/id123" {
		t.Fatalf("Link = %q, want canonical App Store URL", candidate.Link)
	}
}

func TestSubmitCommunityWallEntryDryRunReturnsPlan(t *testing.T) {
	sourceJSON := `[
  {
    "app": "Alpha",
    "link": "https://example.com/alpha",
    "creator": "alpha-dev",
    "platform": ["iOS"]
  }
]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/tester/App-Store-Connect-CLI":
			http.NotFound(w, r)
		case r.Method == http.MethodGet && r.URL.Path == "/repos/rudrankriyam/App-Store-Connect-CLI/git/ref/heads/main":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"object": map[string]any{
					"sha": "base-sha-123",
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/repos/rudrankriyam/App-Store-Connect-CLI/contents/docs/wall-of-apps.json":
			if got := r.URL.Query().Get("ref"); got != "base-sha-123" {
				t.Fatalf("expected ref=base-sha-123, got %q", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sha":      "blob123",
				"encoding": "base64",
				"content":  base64.StdEncoding.EncodeToString([]byte(sourceJSON)),
			})
		default:
			t.Fatalf("unexpected request during dry-run: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	previousAPIBase := communityWallGitHubAPIBase
	previousHTTPClient := communityWallGitHubClient
	previousLookupDetails := communityWallLookupAppDetails
	previousNow := communityWallNow
	communityWallGitHubAPIBase = server.URL
	communityWallGitHubClient = func() *http.Client { return server.Client() }
	communityWallLookupAppDetails = func(ctx context.Context, ids []string) (map[string]communityWallAppDetails, error) {
		return map[string]communityWallAppDetails{
			"1234567890": {
				Name: "Beta",
				Link: "https://apps.apple.com/us/app/beta/id1234567890",
				Icon: "https://example.com/icon.png",
			},
		}, nil
	}
	communityWallNow = func() time.Time {
		return time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)
	}
	t.Cleanup(func() {
		communityWallGitHubAPIBase = previousAPIBase
		communityWallGitHubClient = previousHTTPClient
		communityWallLookupAppDetails = previousLookupDetails
		communityWallNow = previousNow
	})

	result, err := submitCommunityWallEntry(context.Background(), communityWallSubmitRequest{
		Input: communityWallSubmitInput{
			AppID: "1234567890",
		},
		GitHubToken: "token",
		GitHubLogin: "tester",
		DryRun:      true,
	})
	if err != nil {
		t.Fatalf("submit dry-run: %v", err)
	}

	if result.Mode != "dry-run" {
		t.Fatalf("expected dry-run mode, got %q", result.Mode)
	}
	if !result.WillCreateFork {
		t.Fatalf("expected dry-run to indicate fork creation")
	}
	if result.PullRequestURL != "" {
		t.Fatalf("expected no PR URL in dry-run, got %q", result.PullRequestURL)
	}
	if len(result.ChangedFiles) != 1 || result.ChangedFiles[0] != communityWallSourcePath {
		t.Fatalf("expected only %s to change, got %+v", communityWallSourcePath, result.ChangedFiles)
	}
	if result.AppID != "1234567890" {
		t.Fatalf("expected app ID in result, got %q", result.AppID)
	}
	if result.Link != "https://apps.apple.com/us/app/beta/id1234567890" {
		t.Fatalf("expected resolved App Store link, got %q", result.Link)
	}
	if !strings.Contains(result.PullRequestTitle, "apps wall: add Beta") {
		t.Fatalf("unexpected PR title %q", result.PullRequestTitle)
	}
}

func TestSubmitCommunityWallEntryRejectsDuplicateAppID(t *testing.T) {
	sourceJSON := `[
  {
    "app": "Beta",
    "link": "https://apps.apple.com/us/app/beta/id1234567890",
    "creator": "beta-dev",
    "platform": ["iOS"]
  }
]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/tester/App-Store-Connect-CLI":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"full_name":"tester/App-Store-Connect-CLI","fork":true,"parent":{"full_name":"rudrankriyam/App-Store-Connect-CLI"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/repos/rudrankriyam/App-Store-Connect-CLI/git/ref/heads/main":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"object": map[string]any{
					"sha": "base-sha-123",
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/repos/rudrankriyam/App-Store-Connect-CLI/contents/docs/wall-of-apps.json":
			if got := r.URL.Query().Get("ref"); got != "base-sha-123" {
				t.Fatalf("expected ref=base-sha-123, got %q", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sha":      "blob123",
				"encoding": "base64",
				"content":  base64.StdEncoding.EncodeToString([]byte(sourceJSON)),
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	previousAPIBase := communityWallGitHubAPIBase
	previousHTTPClient := communityWallGitHubClient
	previousLookupDetails := communityWallLookupAppDetails
	communityWallGitHubAPIBase = server.URL
	communityWallGitHubClient = func() *http.Client { return server.Client() }
	communityWallLookupAppDetails = func(ctx context.Context, ids []string) (map[string]communityWallAppDetails, error) {
		return map[string]communityWallAppDetails{
			"1234567890": {
				Name: "Beta 2",
				Link: "https://apps.apple.com/us/app/beta-2/id1234567890",
			},
		}, nil
	}
	t.Cleanup(func() {
		communityWallGitHubAPIBase = previousAPIBase
		communityWallGitHubClient = previousHTTPClient
		communityWallLookupAppDetails = previousLookupDetails
	})

	_, err := submitCommunityWallEntry(context.Background(), communityWallSubmitRequest{
		Input: communityWallSubmitInput{
			AppID: "1234567890",
		},
		GitHubToken: "token",
		GitHubLogin: "tester",
		DryRun:      true,
	})
	if err == nil {
		t.Fatal("expected duplicate app ID error")
	}
	if !strings.Contains(err.Error(), `app ID "1234567890" already exists`) {
		t.Fatalf("expected duplicate app ID message, got %v", err)
	}
}

func TestSubmitCommunityWallEntryRejectsMalformedExistingSource(t *testing.T) {
	sourceJSON := `[
  {
    "app": "Alpha",
    "link": "",
    "creator": "",
    "platform": ["iOS"]
  }
]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/tester/App-Store-Connect-CLI":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"full_name":"tester/App-Store-Connect-CLI","fork":true,"parent":{"full_name":"rudrankriyam/App-Store-Connect-CLI"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/repos/rudrankriyam/App-Store-Connect-CLI/git/ref/heads/main":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"object": map[string]any{
					"sha": "base-sha-123",
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/repos/rudrankriyam/App-Store-Connect-CLI/contents/docs/wall-of-apps.json":
			if got := r.URL.Query().Get("ref"); got != "base-sha-123" {
				t.Fatalf("expected ref=base-sha-123, got %q", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sha":      "blob123",
				"encoding": "base64",
				"content":  base64.StdEncoding.EncodeToString([]byte(sourceJSON)),
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	previousAPIBase := communityWallGitHubAPIBase
	previousHTTPClient := communityWallGitHubClient
	previousLookupDetails := communityWallLookupAppDetails
	communityWallGitHubAPIBase = server.URL
	communityWallGitHubClient = func() *http.Client { return server.Client() }
	communityWallLookupAppDetails = func(ctx context.Context, ids []string) (map[string]communityWallAppDetails, error) {
		return map[string]communityWallAppDetails{
			"1234567890": {
				Name: "Beta",
				Link: "https://apps.apple.com/us/app/beta/id1234567890",
			},
		}, nil
	}
	t.Cleanup(func() {
		communityWallGitHubAPIBase = previousAPIBase
		communityWallGitHubClient = previousHTTPClient
		communityWallLookupAppDetails = previousLookupDetails
	})

	_, err := submitCommunityWallEntry(context.Background(), communityWallSubmitRequest{
		Input: communityWallSubmitInput{
			AppID: "1234567890",
		},
		GitHubToken: "token",
		GitHubLogin: "tester",
		DryRun:      true,
	})
	if err == nil {
		t.Fatal("expected malformed source error")
	}
	if !strings.Contains(err.Error(), "entry #1: 'link' is required") {
		t.Fatalf("expected source validation error, got %v", err)
	}
}

func TestSubmitCommunityWallEntryRejectsExistingNonForkRepo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/repos/tester/App-Store-Connect-CLI" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		_, _ = w.Write([]byte(`{"full_name":"tester/App-Store-Connect-CLI","fork":false}`))
	}))
	defer server.Close()

	previousAPIBase := communityWallGitHubAPIBase
	previousHTTPClient := communityWallGitHubClient
	previousLookupDetails := communityWallLookupAppDetails
	communityWallGitHubAPIBase = server.URL
	communityWallGitHubClient = func() *http.Client { return server.Client() }
	communityWallLookupAppDetails = func(ctx context.Context, ids []string) (map[string]communityWallAppDetails, error) {
		return map[string]communityWallAppDetails{
			"1234567890": {
				Name: "Beta",
				Link: "https://apps.apple.com/us/app/beta/id1234567890",
			},
		}, nil
	}
	t.Cleanup(func() {
		communityWallGitHubAPIBase = previousAPIBase
		communityWallGitHubClient = previousHTTPClient
		communityWallLookupAppDetails = previousLookupDetails
	})

	_, err := submitCommunityWallEntry(context.Background(), communityWallSubmitRequest{
		Input: communityWallSubmitInput{
			AppID: "1234567890",
		},
		GitHubToken: "token",
		GitHubLogin: "tester",
		DryRun:      true,
	})
	if err == nil {
		t.Fatal("expected non-fork repo error")
	}
	if !strings.Contains(err.Error(), "is not a fork of rudrankriyam/App-Store-Connect-CLI") {
		t.Fatalf("expected non-fork repo error, got %v", err)
	}
}

func TestRenderCommunityWallSourceEntriesOmitsOptionalMetadata(t *testing.T) {
	entries := []communityWallEntry{
		{
			App:  "Beta",
			Link: "https://apps.apple.com/app/id1234567890",
			Icon: "https://example.com/icon.png",
		},
	}

	rendered, err := renderCommunityWallSourceEntries(entries)
	if err != nil {
		t.Fatalf("render wall source: %v", err)
	}

	expected := `[
  {
    "app": "Beta",
    "link": "https://apps.apple.com/app/id1234567890",
    "icon": "https://example.com/icon.png"
  }
]
`
	if rendered != expected {
		t.Fatalf("unexpected rendered wall source:\n%s", rendered)
	}

	parsed, err := parseCommunityWallSourceEntries([]byte(rendered), "testdata")
	if err != nil {
		t.Fatalf("parse rendered wall source: %v", err)
	}
	if len(parsed) != 1 {
		t.Fatalf("expected one parsed entry, got %d", len(parsed))
	}
}

func TestCommunityWallPullRequestBodyOmitsOptionalMetadata(t *testing.T) {
	body := communityWallPullRequestBody(
		communityWallSubmitInput{AppID: "1234567890"},
		communityWallEntry{
			App:  "Beta",
			Link: "https://apps.apple.com/app/id1234567890",
		},
	)

	if strings.Contains(body, "- Creator:") {
		t.Fatalf("expected creator line to be omitted, got %q", body)
	}
	if strings.Contains(body, "- Platform:") {
		t.Fatalf("expected platform line to be omitted, got %q", body)
	}
}

func TestWaitForRepoReturnsFriendlyTimeoutAfterSleepCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/repos/tester/App-Store-Connect-CLI" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	previousAPIBase := communityWallGitHubAPIBase
	previousHTTPClient := communityWallGitHubClient
	previousSleep := communityWallSleep
	communityWallGitHubAPIBase = server.URL
	communityWallGitHubClient = func() *http.Client { return server.Client() }
	t.Cleanup(func() {
		communityWallGitHubAPIBase = previousAPIBase
		communityWallGitHubClient = previousHTTPClient
		communityWallSleep = previousSleep
	})

	ctx, cancel := context.WithCancel(context.Background())
	communityWallSleep = func(time.Duration) {
		cancel()
	}

	client := communityWallGitHubClientAPI{Token: "token"}
	err := client.waitForRepo(ctx, "tester", "App-Store-Connect-CLI")
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out waiting for fork tester/App-Store-Connect-CLI") {
		t.Fatalf("expected friendly timeout error, got %v", err)
	}
}

func TestFetchCommunityWallAppDetailsOmitsCountryFilter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/lookup" {
			t.Fatalf("expected /lookup path, got %q", got)
		}
		if got := r.URL.Query().Get("id"); got != "1234567890" {
			t.Fatalf("expected id query, got %q", got)
		}
		if got := r.URL.Query().Get("country"); got != "" {
			t.Fatalf("expected no country query filter, got %q", got)
		}
		if got := r.URL.Query().Get("entity"); got != "software" {
			t.Fatalf("expected entity=software, got %q", got)
		}
		_, _ = w.Write([]byte(`{"results":[{"trackId":1234567890,"trackName":"Beta","trackViewUrl":"https://apps.apple.com/app/id1234567890","artworkUrl100":"https://example.com/icon.png"}]}`))
	}))
	defer server.Close()

	previousLookupURL := communityWallAppStoreLookupURL
	communityWallAppStoreLookupURL = server.URL
	t.Cleanup(func() {
		communityWallAppStoreLookupURL = previousLookupURL
	})

	details, err := fetchCommunityWallAppDetails(context.Background(), []string{"1234567890"})
	if err != nil {
		t.Fatalf("fetch app details: %v", err)
	}
	if got := details["1234567890"].Name; got != "Beta" {
		t.Fatalf("expected app details for requested ID, got %+v", details)
	}
}

func TestFetchCommunityWallAppDetailsPreservesZeroPaddedRequestKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/lookup" {
			t.Fatalf("expected /lookup path, got %q", got)
		}
		if got := r.URL.Query().Get("id"); got != "123" {
			t.Fatalf("expected canonical id query, got %q", got)
		}
		_, _ = w.Write([]byte(`{"results":[{"trackId":123,"trackName":"Beta","trackViewUrl":"https://apps.apple.com/app/id123","artworkUrl100":"https://example.com/icon.png"}]}`))
	}))
	defer server.Close()

	previousLookupURL := communityWallAppStoreLookupURL
	communityWallAppStoreLookupURL = server.URL
	t.Cleanup(func() {
		communityWallAppStoreLookupURL = previousLookupURL
	})

	details, err := fetchCommunityWallAppDetails(context.Background(), []string{"00123"})
	if err != nil {
		t.Fatalf("fetch app details: %v", err)
	}
	if got := details["00123"].Name; got != "Beta" {
		t.Fatalf("expected app details for zero-padded requested ID, got %+v", details)
	}
}

func TestCommunityWallIconForLinkUsesStorefrontCountry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/lookup" {
			t.Fatalf("expected /lookup path, got %q", got)
		}
		if got := r.URL.Query().Get("id"); got != "6758220887" {
			t.Fatalf("expected id query, got %q", got)
		}
		if got := r.URL.Query().Get("country"); got != "il" {
			t.Fatalf("expected country=il, got %q", got)
		}
		if got := r.URL.Query().Get("entity"); got != "software" {
			t.Fatalf("expected entity=software, got %q", got)
		}
		_, _ = w.Write([]byte(`{"results":[{"trackId":6758220887,"trackName":"Tamloot","trackViewUrl":"https://apps.apple.com/il/app/tamloot/id6758220887","artworkUrl100":"https://example.com/tamloot.png"}]}`))
	}))
	defer server.Close()

	previousLookupURL := communityWallAppStoreLookupURL
	communityWallAppStoreLookupURL = server.URL
	t.Cleanup(func() {
		communityWallAppStoreLookupURL = previousLookupURL
	})

	iconURL, err := communityWallIconForLink(context.Background(), "https://apps.apple.com/il/app/tamloot/id6758220887")
	if err != nil {
		t.Fatalf("refresh icon: %v", err)
	}
	if iconURL != "https://example.com/tamloot.png" {
		t.Fatalf("icon URL = %q, want storefront lookup result", iconURL)
	}
}
