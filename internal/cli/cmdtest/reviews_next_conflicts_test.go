package cmdtest

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestReviewContinuationsRejectServerQueryFlagsBeforeAuth(t *testing.T) {
	t.Cleanup(func() { shared.SetSelectedProfile("") })

	const summariesNext = "https://api.appstoreconnect.apple.com/v1/apps/app-1/customerReviewSummarizations?cursor=next"
	const attachmentsNext = "https://api.appstoreconnect.apple.com/v1/appStoreReviewDetails/detail-1/appStoreReviewAttachments?cursor=next"

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "summarizations app before next",
			args:    []string{"reviews", "summarizations", "--app", "app-1", "--next", summariesNext},
			wantErr: "reviews summarizations: --next cannot be combined with --app",
		},
		{
			name:    "summarizations app after next with root flag before subcommands",
			args:    []string{"--profile", "ci", "reviews", "summarizations", "--next", summariesNext, "--app", "app-1"},
			wantErr: "reviews summarizations: --next cannot be combined with --app",
		},
		{
			name:    "summarizations platform after next",
			args:    []string{"reviews", "summarizations", "--next", summariesNext, "--platform", "IOS"},
			wantErr: "reviews summarizations: --next cannot be combined with --platform",
		},
		{
			name:    "summarizations territory before next",
			args:    []string{"reviews", "summarizations", "--territory", "USA", "--next", summariesNext},
			wantErr: "reviews summarizations: --next cannot be combined with --territory",
		},
		{
			name:    "summarizations fields after next",
			args:    []string{"reviews", "summarizations", "--next", summariesNext, "--fields", "text"},
			wantErr: "reviews summarizations: --next cannot be combined with --fields",
		},
		{
			name:    "summarizations explicit empty territory fields",
			args:    []string{"reviews", "summarizations", "--territory-fields", "", "--next", summariesNext},
			wantErr: "reviews summarizations: --next cannot be combined with --territory-fields",
		},
		{
			name:    "summarizations include after next",
			args:    []string{"reviews", "summarizations", "--next", summariesNext, "--include", "territory"},
			wantErr: "reviews summarizations: --next cannot be combined with --include",
		},
		{
			name:    "summarizations explicit zero limit",
			args:    []string{"reviews", "summarizations", "--limit", "0", "--next", summariesNext},
			wantErr: "reviews summarizations: --next cannot be combined with --limit",
		},
		{
			name:    "attachments review detail before next",
			args:    []string{"review", "attachments-list", "--review-detail", "detail-1", "--next", attachmentsNext},
			wantErr: "review attachments-list: --next cannot be combined with --review-detail",
		},
		{
			name:    "attachments review detail after next with root flag before subcommands",
			args:    []string{"--profile", "ci", "review", "attachments-list", "--next", attachmentsNext, "--review-detail", "detail-1"},
			wantErr: "review attachments-list: --next cannot be combined with --review-detail",
		},
		{
			name:    "attachments fields after next",
			args:    []string{"review", "attachments-list", "--next", attachmentsNext, "--fields", "fileName"},
			wantErr: "review attachments-list: --next cannot be combined with --fields",
		},
		{
			name:    "attachments explicit empty detail fields",
			args:    []string{"review", "attachments-list", "--detail-fields", "", "--next", attachmentsNext},
			wantErr: "review attachments-list: --next cannot be combined with --detail-fields",
		},
		{
			name:    "attachments include before next",
			args:    []string{"review", "attachments-list", "--include", "appStoreReviewDetail", "--next", attachmentsNext},
			wantErr: "review attachments-list: --next cannot be combined with --include",
		},
		{
			name:    "attachments explicit zero limit",
			args:    []string{"review", "attachments-list", "--next", attachmentsNext, "--limit", "0"},
			wantErr: "review attachments-list: --next cannot be combined with --limit",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientFactoryCalled := false
			restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
				clientFactoryCalled = true
				return nil, errors.New("client factory must not run during validation")
			})
			defer restore()

			assertUsageExit(t, test.args, test.wantErr)
			if clientFactoryCalled {
				t.Fatal("client factory ran before --next conflict validation")
			}
		})
	}
}

func TestReviewPaginationFollowsServerNextURL(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		args       []string
		resource   string
		firstQuery map[string]string
	}{
		{
			name:       "reviews summarizations",
			path:       "/v1/apps/app-1/customerReviewSummarizations",
			args:       []string{"reviews", "summarizations", "--app", "app-1", "--platform", "IOS"},
			resource:   "customerReviewSummarizations",
			firstQuery: map[string]string{"filter[platform]": "IOS", "limit": "200"},
		},
		{
			name:       "review attachments-list",
			path:       "/v1/appStoreReviewDetails/detail-1/appStoreReviewAttachments",
			args:       []string{"review", "attachments-list", "--review-detail", "detail-1"},
			resource:   "appStoreReviewAttachments",
			firstQuery: map[string]string{"limit": "200"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			nextURL := "https://api.appstoreconnect.apple.com" + test.path + "?cursor=next"
			requestCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				requestCount++
				if req.Method != http.MethodGet || req.URL.Path != test.path {
					t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
					http.Error(w, "unexpected request", http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				switch requestCount {
				case 1:
					for key, want := range test.firstQuery {
						if got := req.URL.Query().Get(key); got != want {
							t.Errorf("first request %s = %q, want %q", key, got, want)
						}
					}
					_, _ = io.WriteString(w, `{"data":[{"type":"`+test.resource+`","id":"item-1"}],"links":{"next":"`+nextURL+`"}}`)
				case 2:
					if got := req.URL.Query().Get("cursor"); got != "next" {
						t.Errorf("second request cursor = %q, want next", got)
					}
					_, _ = io.WriteString(w, `{"data":[{"type":"`+test.resource+`","id":"item-2"}],"links":{"next":""}}`)
				default:
					t.Errorf("unexpected request count %d", requestCount)
					http.Error(w, "unexpected request", http.StatusBadRequest)
				}
			}))
			t.Cleanup(server.Close)
			installDefaultTransport(t, serverRoundTripper(t, server))

			stdout, stderr, runErr := runRootCommand(t, append(test.args, "--paginate", "--output", "json"))
			if runErr != nil {
				t.Fatalf("run error: %v", runErr)
			}
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			if !strings.Contains(stdout, `"id":"item-1"`) || !strings.Contains(stdout, `"id":"item-2"`) {
				t.Fatalf("stdout = %q, want both pages", stdout)
			}
			if requestCount != 2 {
				t.Fatalf("request count = %d, want 2", requestCount)
			}
		})
	}
}
