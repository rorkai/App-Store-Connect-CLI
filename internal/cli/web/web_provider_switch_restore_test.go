package web

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

// fakeOlympusSession answers Apple's session endpoint for one signed-in
// session: GET reports the current provider, POST switches it.
type fakeOlympusSession struct {
	mu       sync.Mutex
	current  int64
	switches []int64
}

var fakeOlympusProviders = map[int64][2]string{
	111: {"TEAM111", "Default Team"},
	222: {"TEAM222", "Cached Team"},
	333: {"TEAM333", "Other Team"},
}

func fakeOlympusProvider(id int64) map[string]any {
	p := fakeOlympusProviders[id]
	return map[string]any{"providerId": id, "publicProviderId": p[0], "name": p[1]}
}

func (s *fakeOlympusSession) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host != "appstoreconnect.apple.com" || req.URL.Path != "/olympus/v1/session" {
		return nil, fmt.Errorf("unexpected request %s %s", req.Method, req.URL)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	switch req.Method {
	case http.MethodGet:
	case http.MethodPost:
		var payload struct {
			Provider struct {
				ProviderID int64 `json:"providerId"`
			} `json:"provider"`
		}
		body, _ := io.ReadAll(req.Body)
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, err
		}
		s.switches = append(s.switches, payload.Provider.ProviderID)
		s.current = payload.Provider.ProviderID
	default:
		return nil, fmt.Errorf("unexpected request %s %s", req.Method, req.URL)
	}
	raw, err := json.Marshal(map[string]any{
		"provider":           fakeOlympusProvider(s.current),
		"availableProviders": []any{fakeOlympusProvider(111), fakeOlympusProvider(222), fakeOlympusProvider(333)},
		"user":               map[string]any{"emailAddress": "user@example.com"},
	})
	if err != nil {
		return nil, err
	}
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(raw))), Request: req}, nil
}

// TestResolveWebSessionForCommandKeepsCacheOnItsProviderAfterReauth covers a
// cached session on provider 222 that expired: the command signs in again and
// Apple puts the new session on the account's default provider 111. Returning
// it to 222 is a restore, so read-only mode and --dry-run allow it, and a
// refused switch to any other provider leaves the cache on 222. Provider
// selection runs through the real webcore.SelectProvider against a fake
// Apple session endpoint.
func TestResolveWebSessionForCommandKeepsCacheOnItsProviderAfterReauth(t *testing.T) {
	tests := []struct {
		name          string
		readOnly      bool
		dryRun        bool
		providerID    int64
		wantSwitches  []int64
		wantPersisted []int64
		wantErr       string
		wantReadOnly  bool
	}{
		{
			name:          "read-only, --provider-id names the cached provider",
			readOnly:      true,
			providerID:    222,
			wantSwitches:  []int64{222},
			wantPersisted: []int64{222},
		},
		{
			name:          "dry-run, --provider-id names the cached provider",
			dryRun:        true,
			providerID:    222,
			wantSwitches:  []int64{222},
			wantPersisted: []int64{222},
		},
		{
			name:          "read-only, no --provider-id restores the cached provider",
			readOnly:      true,
			wantSwitches:  []int64{222},
			wantPersisted: []int64{222},
		},
		{
			name:          "read-only, --provider-id names another provider",
			readOnly:      true,
			providerID:    333,
			wantSwitches:  []int64{222},
			wantPersisted: []int64{222},
			wantErr:       "--read-only is set; refusing to switch the web session from provider 222 / TEAM222 / Cached Team to provider 333 / TEAM333 / Other Team",
			wantReadOnly:  true,
		},
		{
			name:          "dry-run, --provider-id names another provider",
			dryRun:        true,
			providerID:    333,
			wantSwitches:  []int64{222},
			wantPersisted: []int64{222},
			wantErr:       "--dry-run is set; refusing to switch the web session from provider 222 / TEAM222 / Cached Team to provider 333 / TEAM333 / Other Team",
		},
		{
			name:          "writes allowed, --provider-id names another provider",
			providerID:    333,
			wantSwitches:  []int64{333},
			wantPersisted: []int64{333},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(readonly.EnvVar, "")
			t.Setenv(webPasswordEnv, "env-secret")
			readonly.SetFlagEnabled(test.readOnly)
			t.Cleanup(func() { readonly.SetFlagEnabled(false) })

			origTryResume := tryResumeSessionFn
			origLoadCached := loadCachedSessionFn
			origLoginWithClient := webLoginWithClientFn
			origLogin := webLoginFn
			origPersist := persistWebSessionFn
			origSelect := selectWebProviderFn
			t.Cleanup(func() {
				tryResumeSessionFn = origTryResume
				loadCachedSessionFn = origLoadCached
				webLoginWithClientFn = origLoginWithClient
				webLoginFn = origLogin
				persistWebSessionFn = origPersist
				selectWebProviderFn = origSelect
			})
			selectWebProviderFn = webcore.SelectProvider

			apple := &fakeOlympusSession{current: 111}
			cachedClient := &http.Client{Transport: apple}
			tryResumeSessionFn = func(context.Context, string) (*webcore.AuthSession, bool, error) {
				return nil, false, webcore.ErrCachedSessionExpired
			}
			loadCachedSessionFn = func(string) (*webcore.AuthSession, bool, error) {
				return &webcore.AuthSession{Client: cachedClient, UserEmail: "user@example.com", ProviderID: 222, DeveloperTeamID: "DEVTEAM222"}, true, nil
			}
			webLoginWithClientFn = func(ctx context.Context, client *http.Client, creds webcore.LoginCredentials) (*webcore.AuthSession, error) {
				return &webcore.AuthSession{Client: client, UserEmail: "user@example.com", ProviderID: 111}, nil
			}
			webLoginFn = func(context.Context, webcore.LoginCredentials) (*webcore.AuthSession, error) {
				t.Fatal("did not expect a fresh sign-in")
				return nil, nil
			}
			var persisted []int64
			persistWebSessionFn = func(session *webcore.AuthSession) error {
				persisted = append(persisted, session.ProviderID)
				return nil
			}

			d, _, stderr := newWebTestDeps()
			fs := flag.NewFlagSet("web apps delete", flag.ContinueOnError)
			dryRun := fs.Bool("dry-run", false, "")
			flags := bindWebSessionFlags(fs)
			*dryRun = test.dryRun
			*flags.appleID = "user@example.com"
			*flags.providerID = test.providerID

			_, _, cancel, err := resolveWebSessionForCommandWith(context.Background(), d, flags)
			defer cancel()

			if fmt.Sprint(apple.switches) != fmt.Sprint(test.wantSwitches) {
				t.Fatalf("provider switches sent = %v, want %v", apple.switches, test.wantSwitches)
			}
			if fmt.Sprint(persisted) != fmt.Sprint(test.wantPersisted) {
				t.Fatalf("session cached on providers %v, want %v", persisted, test.wantPersisted)
			}
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("resolveWebSessionForCommandWith() error = %v", err)
				}
				return
			}
			if got := errors.Is(err, readonly.ErrRefused); got != test.wantReadOnly {
				t.Fatalf("errors.Is(err, readonly.ErrRefused) = %t, want %t (err %v)", got, test.wantReadOnly, err)
			}
			if test.dryRun {
				if !shared.IsReportedUsageError(err) || !strings.Contains(stderr.String(), "Error: "+test.wantErr+"\n") {
					t.Fatalf("expected reported usage error %q, got %v, stderr %q", test.wantErr, err, stderr.String())
				}
				return
			}
			if err == nil || err.Error() != test.wantErr {
				t.Fatalf("error = %v, want %q", err, test.wantErr)
			}
		})
	}
}
