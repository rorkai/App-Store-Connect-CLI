package cmdtest

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

const brokerThreadsFixture = `{"data":[{"id":"thread-1","type":"resolutionCenterThreads","attributes":{"threadType":"REJECTION_BINARY","state":"OPEN","createdDate":"2026-11-16T00:00:00Z"}}]}`

// webBrokerFixture serves a broker that relays canned Apple answers. With
// refusal set, it refuses every request after the session bootstrap.
func webBrokerFixture(t *testing.T, refusal string) (*[]string, string) {
	t.Helper()
	cache := t.TempDir()
	t.Setenv("ASC_WEB_SESSION_CACHE", "1")
	t.Setenv("ASC_WEB_SESSION_CACHE_DIR", cache)
	t.Setenv("ASC_WEB_SESSION_CACHE_BACKEND", "file")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	for _, name := range []string{"ASC_PROFILE", "ASC_KEY_ID", "ASC_ISSUER_ID", "ASC_PRIVATE_KEY_PATH", "ASC_PRIVATE_KEY", "ASC_PRIVATE_KEY_B64", "ASC_STRICT_AUTH", "ASC_WEB_SESSION", "ASC_APP_ID"} {
		t.Setenv(name, "")
	}
	requests := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		if r.Header.Get("Authorization") != "Bearer broker-token" || r.Header.Get("Cookie") != "" {
			t.Error("broker request must carry the broker token and no cookie")
		}
		w.Header().Set("Content-Type", "application/json")
		if refusal != "" && r.URL.Path != "/broker/appstoreconnect.apple.com/olympus/v1/session" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, refusal)
			return
		}
		w.Header().Set("X-Asc-Web-Broker-Upstream", "1")
		switch r.URL.Path {
		case "/broker/appstoreconnect.apple.com/olympus/v1/session":
			_, _ = io.WriteString(w, `{"provider":{"providerId":42,"publicProviderId":"TEAM42"},"availableProviders":[{"providerId":42,"publicProviderId":"TEAM42"},{"providerId":99,"publicProviderId":"TEAM99"}],"user":{"emailAddress":"user@example.com"}}`)
		case "/broker/appstoreconnect.apple.com/iris/v1/apps/app-1/resolutionCenterThreads":
			_, _ = io.WriteString(w, brokerThreadsFixture)
		default:
			t.Errorf("unexpected route: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	t.Setenv("ASC_WEB_BROKER_URL", server.URL+"/broker")
	t.Setenv("ASC_WEB_BROKER_TOKEN", "broker-token")
	return &requests, cache
}

func assertWebSessionCacheEmpty(t *testing.T, cache string) {
	t.Helper()
	entries, err := os.ReadDir(cache)
	if err != nil || len(entries) != 0 {
		t.Fatalf("session cache changed: entries=%v err=%v", entries, err)
	}
}

func TestWebBrokerServesReviewThreadsWithoutLocalSession(t *testing.T) {
	requests, cache := webBrokerFixture(t, "")
	// Sign-in settings left in the environment play no part under a broker.
	t.Setenv("ASC_WEB_2FA_METHOD", "bogus")

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{"web", "review", "threads", "--app", "app-1", "--output", "json"}, "test")
	})
	if code != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	var threads []struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	if err := json.Unmarshal([]byte(stdout), &threads); err != nil || len(threads) != 1 || threads[0].Thread.ID != "thread-1" {
		t.Fatalf("unexpected output %q (err=%v)", stdout, err)
	}
	want := []string{
		"GET /broker/appstoreconnect.apple.com/olympus/v1/session",
		"GET /broker/appstoreconnect.apple.com/iris/v1/apps/app-1/resolutionCenterThreads",
	}
	if !reflect.DeepEqual(*requests, want) {
		t.Fatalf("requests=%v, want %v", *requests, want)
	}
	assertWebSessionCacheEmpty(t, cache)
}

func TestWebBrokerRefusalIsReportedAsBrokers(t *testing.T) {
	for _, args := range [][]string{
		{"web", "review", "threads", "--app", "app-1"},
		{"web", "auth", "capabilities", "--key-id", "KEY123"},
	} {
		t.Run(args[2], func(t *testing.T) {
			_, cache := webBrokerFixture(t, `{"error":{"code":"ASC_WEB_SESSION_EXPIRED","message":"Apple ended the stored sign-in for team TEAM42."}}`)

			var code int
			stdout, stderr := captureOutput(t, func() { code = cmd.Run(args, "test") })
			if code == 0 {
				t.Fatalf("expected failure, stdout=%q", stdout)
			}
			if !strings.Contains(stderr, "ASC_WEB_SESSION_EXPIRED") || !strings.Contains(stderr, "Apple ended the stored sign-in for team TEAM42.") {
				t.Fatalf("stderr=%q", stderr)
			}
			if strings.Contains(stderr, "asc web auth login") || strings.Contains(stderr, "network connection") {
				t.Fatalf("broker refusal must not suggest a local sign-in or a network problem: %q", stderr)
			}
			assertWebSessionCacheEmpty(t, cache)
		})
	}
}

func TestWebBrokerRefusesLocalSessionCommandsBeforeRequests(t *testing.T) {
	for _, tc := range []struct {
		name       string
		args       []string
		env, value string
		diagnostic string
	}{
		{name: "login", args: []string{"web", "auth", "login", "--apple-id", "user@example.com"}, diagnostic: "asc web auth login is unavailable while ASC_WEB_BROKER_URL is set"},
		{name: "import", args: []string{"web", "auth", "import", "--from-env"}, diagnostic: "asc web auth import is unavailable"},
		{name: "export", args: []string{"web", "auth", "export", "--output-path", "session.json"}, diagnostic: "asc web auth export is unavailable"},
		{name: "logout", args: []string{"web", "auth", "logout", "--all"}, diagnostic: "asc web auth logout is unavailable"},
		{name: "sign-in flag", args: []string{"web", "review", "threads", "--app", "app-1", "--two-factor-method", "sms"}, diagnostic: "--two-factor-method is unavailable"},
		{name: "session from env", args: []string{"web", "removed-apps", "list", "--session-from-env"}, diagnostic: "--session-from-env is unavailable"},
		{name: "token missing", args: []string{"web", "review", "threads", "--app", "app-1"}, env: "ASC_WEB_BROKER_TOKEN", diagnostic: "set both ASC_WEB_BROKER_URL and ASC_WEB_BROKER_TOKEN"},
		{name: "remote http", args: []string{"web", "review", "threads", "--app", "app-1"}, env: "ASC_WEB_BROKER_URL", value: "http://broker.example.com", diagnostic: "must use https"},
		{name: "deep validation with token missing", args: []string{"validate", "--app", "app-1", "--version", "1.0", "--deep"}, env: "ASC_WEB_BROKER_TOKEN", diagnostic: "set both ASC_WEB_BROKER_URL and ASC_WEB_BROKER_TOKEN"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests, cache := webBrokerFixture(t, "")
			if tc.env != "" {
				t.Setenv(tc.env, tc.value)
			}
			var code int
			stdout, stderr := captureOutput(t, func() { code = cmd.Run(tc.args, "test") })
			if code != 2 || !strings.Contains(stderr, tc.diagnostic) {
				t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
			if len(*requests) != 0 {
				t.Fatalf("unexpected requests: %v", *requests)
			}
			assertWebSessionCacheEmpty(t, cache)
		})
	}
}

// The lent session keeps its account and provider: a command that names
// another one stops instead of acting on, or switching, the broker's session.
func TestWebBrokerKeepsTheLentSessionsAccountAndProvider(t *testing.T) {
	threads := []string{"web", "review", "threads", "--app", "app-1"}
	for _, tc := range []struct {
		name       string
		args       []string
		appleIDEnv string
		diagnostic string
	}{
		{name: "the session's provider", args: append(threads, "--provider-id", "42")},
		{name: "another provider", args: append(threads, "--provider-id", "99"), diagnostic: "only the broker's owner can change"},
		{name: "another account in the environment", args: threads, appleIDEnv: "other@example.com", diagnostic: "ASC_WEB_APPLE_ID does not match the account of the brokered web session"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests, cache := webBrokerFixture(t, "")
			t.Setenv("ASC_WEB_APPLE_ID", tc.appleIDEnv)
			var code int
			stdout, stderr := captureOutput(t, func() { code = cmd.Run(tc.args, "test") })
			if (code == 0) != (tc.diagnostic == "") || !strings.Contains(stderr, tc.diagnostic) {
				t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
			for _, request := range *requests {
				if !strings.HasPrefix(request, "GET ") {
					t.Fatalf("the lent session was changed: %v", *requests)
				}
			}
			assertWebSessionCacheEmpty(t, cache)
		})
	}
}

// A session asc cached earlier is not the broker's: brokered commands neither
// use it nor remove it.
func TestWebBrokerLeavesACachedSessionAlone(t *testing.T) {
	_, cache := webBrokerFixture(t, "")
	jar, _ := cookiejar.New(nil)
	jar.SetCookies(&url.URL{Scheme: "https", Host: "appstoreconnect.apple.com", Path: "/"}, []*http.Cookie{{Name: "myacinfo", Value: "cached", Path: "/"}})
	if err := webcore.PersistSession(&webcore.AuthSession{Client: &http.Client{Jar: jar}, UserEmail: "user@example.com"}); err != nil {
		t.Fatalf("seed cached session: %v", err)
	}
	snapshot := func() string {
		entries, err := os.ReadDir(cache)
		if err != nil {
			t.Fatal(err)
		}
		var files []string
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".json") {
				data, _ := os.ReadFile(filepath.Join(cache, entry.Name()))
				files = append(files, entry.Name()+"="+string(data))
			}
		}
		return strings.Join(files, "\n")
	}
	before := snapshot()
	if before == "" {
		t.Fatal("no cached session was seeded")
	}

	for _, args := range [][]string{
		{"web", "review", "threads", "--app", "app-1"},
		{"web", "auth", "status"},
		{"web", "auth", "logout", "--all"},
	} {
		captureOutput(t, func() { cmd.Run(args, "test") })
		if after := snapshot(); after != before {
			t.Fatalf("%v changed the cached session:\n%s\nwas:\n%s", args, after, before)
		}
	}
}

// A request asc does not hand to the broker never reached the network, so it
// is not reported as a connection problem.
func TestWebBrokerUnrelayableRequestIsNotANetworkFailure(t *testing.T) {
	requests, _ := webBrokerFixture(t, "")

	var code int
	_, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{"web", "review", "threads", "--app", ".."}, "test")
	})
	if code == 0 || !strings.Contains(stderr, "dot segments") || strings.Contains(stderr, "network connection") {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	if want := []string{"GET /broker/appstoreconnect.apple.com/olympus/v1/session"}; !reflect.DeepEqual(*requests, want) {
		t.Fatalf("requests=%v, want %v", *requests, want)
	}
}

// When Apple itself ends the lent session, the advice is to sign in where the
// session lives, never the asc sign-in a broker makes unavailable.
func TestWebBrokerRelayedAppleSignOutPointsAtTheBroker(t *testing.T) {
	for _, tc := range []struct {
		name          string
		args          []string
		sessionStatus int
	}{
		{name: "session read", args: []string{"web", "auth", "status"}, sessionStatus: http.StatusUnauthorized},
		{name: "command request", args: []string{"web", "review", "threads", "--app", "app-1"}, sessionStatus: http.StatusOK},
		{name: "capabilities", args: []string{"web", "auth", "capabilities", "--key-id", "KEY123"}, sessionStatus: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			webBrokerFixture(t, "")
			apple := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Asc-Web-Broker-Upstream", "1")
				if strings.HasSuffix(r.URL.Path, "/olympus/v1/session") && tc.sessionStatus == http.StatusOK {
					_, _ = io.WriteString(w, `{"provider":{"providerId":42,"publicProviderId":"TEAM42"},"user":{"emailAddress":"user@example.com"}}`)
					return
				}
				w.WriteHeader(http.StatusUnauthorized)
			}))
			t.Cleanup(apple.Close)
			t.Setenv("ASC_WEB_BROKER_URL", apple.URL)

			var code int
			_, stderr := captureOutput(t, func() { code = cmd.Run(tc.args, "test") })
			if code == 0 || !strings.Contains(stderr, "through the broker's owner") || strings.Contains(stderr, "asc web auth login") {
				t.Fatalf("exit=%d stderr=%q", code, stderr)
			}
		})
	}
}

// App creation says whose session it runs on; under a broker that is never the
// local cache.
func TestWebBrokerAppsCreateNamesTheBrokeredSession(t *testing.T) {
	webBrokerFixture(t, "")
	apple := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Asc-Web-Broker-Upstream", "1")
		if strings.HasSuffix(r.URL.Path, "/olympus/v1/session") {
			_, _ = io.WriteString(w, `{"provider":{"providerId":42,"publicProviderId":"TEAM42"},"user":{"emailAddress":"user@example.com"}}`)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(apple.Close)
	t.Setenv("ASC_WEB_BROKER_URL", apple.URL)

	_, stderr := captureOutput(t, func() {
		cmd.Run([]string{"web", "apps", "create", "--name", "Example", "--bundle-id", "com.example.app", "--sku", "EXAMPLE1", "--platform", "IOS", "--primary-locale", "en-US"}, "test")
	})
	if !strings.Contains(stderr, "Using the brokered web session.") || strings.Contains(stderr, "cached web session") {
		t.Fatalf("stderr=%q", stderr)
	}
}

// Agreement status lives on Apple pages a broker does not lend. When the
// broker refuses them, the caller learns who can read the agreements and
// where, instead of retrying or signing in again.
func TestWebBrokerAgreementsRefusalNamesTheAccountHolderHandOff(t *testing.T) {
	for _, tc := range []struct {
		name         string
		args         []string
		status       int
		body         string
		wantHandOff  bool
		wantExitCode int
	}{
		{name: "agreements status refused", args: []string{"web", "agreements", "status"}, status: http.StatusForbidden, body: `{"error":{"code":"ASC_WEB_REQUEST_NOT_ALLOWED","message":"this request is not lent"}}`, wantHandOff: true, wantExitCode: cmd.ExitAuth},
		{name: "business agreements list refused", args: []string{"web", "business", "agreements", "list"}, status: http.StatusForbidden, body: `{"error":{"code":"ASC_WEB_REQUEST_NOT_ALLOWED","message":"this request is not lent"}}`, wantHandOff: true, wantExitCode: cmd.ExitAuth},
		{name: "agreements status rate limited", args: []string{"web", "agreements", "status"}, status: http.StatusTooManyRequests, body: `{"error":{"code":"RATE_LIMITED","message":"slow down"}}`, wantExitCode: 39},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, cache := webBrokerFixture(t, "")
			var requests []string
			broker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests = append(requests, r.Method+" "+r.URL.Path)
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/appstoreconnect.apple.com/olympus/v1/session" {
					w.Header().Set("X-Asc-Web-Broker-Upstream", "1")
					_, _ = io.WriteString(w, `{"provider":{"providerId":42,"publicProviderId":"TEAM42"},"user":{"emailAddress":"user@example.com"}}`)
					return
				}
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			t.Cleanup(broker.Close)
			t.Setenv("ASC_WEB_BROKER_URL", broker.URL)

			var code int
			stdout, stderr := captureOutput(t, func() { code = cmd.Run(tc.args, "test") })
			if code != tc.wantExitCode {
				t.Fatalf("exit=%d, want %d; stdout=%q stderr=%q", code, tc.wantExitCode, stdout, stderr)
			}
			if len(requests) != 2 {
				t.Fatalf("expected the session read and one refused request, got %v", requests)
			}
			handOff := []string{
				"Account Holder",
				"https://appstoreconnect.apple.com/business",
				"https://developer.apple.com/account",
				"do not retry",
			}
			for _, want := range handOff {
				if got := strings.Contains(stderr, want); got != tc.wantHandOff {
					t.Fatalf("contains %q = %v, want %v; stderr=%q", want, got, tc.wantHandOff, stderr)
				}
			}
			if !strings.Contains(stderr, "web session broker refused the request") {
				t.Fatalf("expected the broker's own refusal, stderr=%q", stderr)
			}
			if tc.wantHandOff && (strings.Contains(stderr, "asc web auth login") || strings.Contains(stderr, "network connection") || strings.Contains(strings.ToLower(stderr), "permission")) {
				t.Fatalf("the refusal must not suggest a sign-in, a network problem, or missing permissions: %q", stderr)
			}
			assertWebSessionCacheEmpty(t, cache)
		})
	}
}
