package web

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
)

func TestBrokerFromEnv(t *testing.T) {
	for _, tc := range []struct {
		name, url, token string
		ok               bool
		wantErr          bool
		wantPath         string
	}{
		{name: "unset"},
		{name: "https", url: "https://broker.example.com/v1/proxy/", token: "t", ok: true, wantPath: "/v1/proxy"},
		{name: "loopback http", url: "http://127.0.0.1:8787/proxy", token: "t", ok: true, wantPath: "/proxy"},
		{name: "token only", token: "t", ok: true, wantErr: true},
		{name: "url only", url: "https://broker.example.com", ok: true, wantErr: true},
		{name: "remote http", url: "http://broker.example.com", token: "t", ok: true, wantErr: true},
		{name: "credentials in url", url: "https://user:pass@broker.example.com", token: "t", ok: true, wantErr: true},
		{name: "query", url: "https://broker.example.com/?token=t", token: "t", ok: true, wantErr: true},
		{name: "port without host", url: "https://:443", token: "t", ok: true, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(WebBrokerURLEnv, tc.url)
			t.Setenv(WebBrokerTokenEnv, tc.token)
			broker, ok, err := BrokerFromEnv()
			if ok != tc.ok || (err != nil) != tc.wantErr {
				t.Fatalf("ok=%v err=%v, want ok=%v wantErr=%v", ok, err, tc.ok, tc.wantErr)
			}
			if err != nil && !errors.Is(err, ErrWebBrokerConfig) {
				t.Fatalf("err=%v, want ErrWebBrokerConfig", err)
			}
			if tc.wantPath != "" && broker.URL.Path != tc.wantPath {
				t.Fatalf("path=%q, want %q", broker.URL.Path, tc.wantPath)
			}
		})
	}
}

// The broker sees Apple's host, path, query, method, body and headers, its own
// token instead of any cookie, and relays Apple's answer, redirects included.
// The token stays with the broker when Apple redirects elsewhere, and a cookie
// the broker relays never reaches the caller.
func TestBrokeredSessionRelaysRequestsThroughBroker(t *testing.T) {
	var seen []string
	broker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer broker-token" {
			t.Errorf("Authorization=%q", got)
		}
		if r.Header.Get("Cookie") != "" {
			t.Error("request carried a cookie to the broker")
		}
		body, _ := io.ReadAll(r.Body)
		seen = append(seen, r.Method+" "+r.URL.EscapedPath()+"?"+r.URL.RawQuery+" "+r.Header.Get("X-Csrf-Itc")+" "+string(body))
		w.Header().Set(WebBrokerUpstreamHeader, "1")
		switch r.URL.EscapedPath() {
		case "/proxy/appstoreconnect.apple.com/olympus/v1/session":
			_, _ = io.WriteString(w, `{"provider":{"providerId":42,"publicProviderId":"TEAM42","name":"Team"},"user":{"emailAddress":"user@example.com"}}`)
		case "/proxy/appstoreconnect.apple.com/iris/v1/moved":
			w.Header().Set("Location", "https://appstoreconnect.apple.com/iris/v1/target")
			w.WriteHeader(http.StatusFound)
		case "/proxy/appstoreconnect.apple.com/iris/v1/target":
			_, _ = io.WriteString(w, `{"data":[]}`)
		case "/proxy/appstoreconnect.apple.com/iris/v1/offsite":
			w.Header().Set("Location", "https://iosapps.example-cdn.com/signed?sig=2")
			w.WriteHeader(http.StatusFound)
		default:
			w.Header().Set("Set-Cookie", "myacinfo=rotated; Domain=apple.com; Path=/")
			_, _ = io.WriteString(w, `{"ok":true}`)
		}
	}))
	t.Cleanup(broker.Close)
	direct := 0
	base := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host == "iosapps.example-cdn.com" {
			direct++
			if req.Header.Get("Authorization") != "" {
				t.Error("direct request carried the broker token")
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("file")), Request: req}, nil
		}
		return http.DefaultTransport.RoundTrip(req)
	})
	t.Setenv(WebBrokerURLEnv, broker.URL+"/proxy")
	t.Setenv(WebBrokerTokenEnv, "broker-token")
	config, _, err := BrokerFromEnv()
	if err != nil {
		t.Fatal(err)
	}

	session, err := openBrokeredSession(context.Background(), config, base)
	if err != nil {
		t.Fatalf("openBrokeredSession: %v", err)
	}
	if session.UserEmail != "user@example.com" || session.ProviderID != 42 || session.PublicProviderID != "TEAM42" {
		t.Fatalf("session=%+v", session)
	}

	post, _ := http.NewRequest(http.MethodPost, "https://appstoreconnect.apple.com/iris/v1/resolutionCenterThreads/a%2Fb/messages?include=x&limit=2", strings.NewReader(`{"m":1}`))
	post.Header.Set("Cookie", "myacinfo=leaked")
	post.Header.Set("X-Csrf-Itc", "itc")
	resp, err := session.Client.Do(post)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	_ = resp.Body.Close()
	if resp.Header.Get(WebBrokerUpstreamHeader) != "" {
		t.Fatal("broker marker leaked to the caller")
	}
	if resp.Header.Get("Set-Cookie") != "" {
		t.Fatal("a cookie the broker relayed reached the caller")
	}
	if session.Client.Jar != nil {
		t.Fatal("brokered session holds a cookie jar")
	}

	redirected, err := session.Client.Get("https://appstoreconnect.apple.com/iris/v1/moved")
	if err != nil {
		t.Fatalf("GET redirect: %v", err)
	}
	_ = redirected.Body.Close()
	if redirected.Request.URL.String() != "https://appstoreconnect.apple.com/iris/v1/target" {
		t.Fatalf("redirect ended at %s", redirected.Request.URL)
	}

	download, err := session.Client.Get("https://iosapps.example-cdn.com/signed?sig=1")
	if err != nil {
		t.Fatalf("direct GET: %v", err)
	}
	_ = download.Body.Close()

	offsite, err := session.Client.Get("https://appstoreconnect.apple.com/iris/v1/offsite")
	if err != nil {
		t.Fatalf("GET offsite redirect: %v", err)
	}
	_ = offsite.Body.Close()

	want := []string{
		"GET /proxy/appstoreconnect.apple.com/olympus/v1/session?  ",
		`POST /proxy/appstoreconnect.apple.com/iris/v1/resolutionCenterThreads/a%2Fb/messages?include=x&limit=2 itc {"m":1}`,
		"GET /proxy/appstoreconnect.apple.com/iris/v1/moved?  ",
		"GET /proxy/appstoreconnect.apple.com/iris/v1/target?  ",
		"GET /proxy/appstoreconnect.apple.com/iris/v1/offsite?  ",
	}
	if strings.Join(seen, "\n") != strings.Join(want, "\n") {
		t.Fatalf("broker saw:\n%s\nwant:\n%s", strings.Join(seen, "\n"), strings.Join(want, "\n"))
	}
	if direct != 2 {
		t.Fatalf("direct requests=%d, want 2", direct)
	}
}

// A response without the upstream marker is the broker's own refusal, never
// Apple's answer, so an expired stored session is not read as Apple's 401.
func TestBrokeredSessionReportsBrokerRefusal(t *testing.T) {
	broker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":{"code":"ASC_WEB_SESSION_EXPIRED","message":"Apple ended the stored sign-in."}}`)
	}))
	t.Cleanup(broker.Close)
	t.Setenv(WebBrokerURLEnv, broker.URL)
	t.Setenv(WebBrokerTokenEnv, "broker-token")
	config, _, _ := BrokerFromEnv()

	_, err := openBrokeredSession(context.Background(), config, http.DefaultTransport)
	var brokerErr *BrokerError
	if !errors.As(err, &brokerErr) {
		t.Fatalf("err=%v, want BrokerError", err)
	}
	if brokerErr.Status != http.StatusUnauthorized || brokerErr.Code != "ASC_WEB_SESSION_EXPIRED" || brokerErr.Message != "Apple ended the stored sign-in." {
		t.Fatalf("brokerErr=%+v", brokerErr)
	}
	if IsSessionAuthFailure(err) {
		t.Fatal("a broker refusal must not read as an expired local web session")
	}
}

// The broker routes on the first path segment and reaches Apple on the default
// https port, so a request that would land somewhere else is never sent.
func TestBrokerTransportRefusesRequestsTheBrokerWouldRouteElsewhere(t *testing.T) {
	var relayed []string
	transport := &brokerTransport{
		broker: Broker{URL: &url.URL{Scheme: "https", Host: "broker.example.com", Path: "/proxy"}, Token: "broker-token"},
		next: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			relayed = append(relayed, req.URL.Host+req.URL.EscapedPath())
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{WebBrokerUpstreamHeader: {"1"}}, Body: http.NoBody, Request: req}, nil
		}),
	}
	for _, target := range []string{
		"http://appstoreconnect.apple.com/olympus/v1/session",
		"https://appstoreconnect.apple.com:8443/olympus/v1/session",
		"https://appstoreconnect.apple.com/iris/v1/apps/../resolutionCenterThreads",
		"https://appstoreconnect.apple.com/iris/v1/apps/%2e%2e/resolutionCenterThreads",
	} {
		req, _ := http.NewRequest(http.MethodGet, target, nil)
		if resp, err := transport.RoundTrip(req); err == nil {
			_ = resp.Body.Close()
			t.Errorf("%s was relayed", target)
		}
	}
	req, _ := http.NewRequest(http.MethodGet, "https://AppStoreConnect.apple.com.:443/olympus/v1/session", nil)
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	_ = resp.Body.Close()
	if want := []string{"broker.example.com/proxy/appstoreconnect.apple.com/olympus/v1/session"}; strings.Join(relayed, "\n") != strings.Join(want, "\n") {
		t.Fatalf("relayed=%v, want %v", relayed, want)
	}
}

// The session is the broker's: asc neither switches its provider, which every
// other borrower of that session would then be on, nor writes it to the cache.
func TestBrokeredSessionKeepsItsProviderAndStaysOutOfTheCache(t *testing.T) {
	var methods []string
	broker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		w.Header().Set(WebBrokerUpstreamHeader, "1")
		_, _ = io.WriteString(w, `{"provider":{"providerId":42,"publicProviderId":"TEAM42"},"availableProviders":[{"providerId":42,"publicProviderId":"TEAM42"},{"providerId":99,"publicProviderId":"TEAM99"}],"user":{"emailAddress":"user@example.com"}}`)
	}))
	t.Cleanup(broker.Close)
	brokerURL, _ := url.Parse(broker.URL)
	session, err := openBrokeredSession(context.Background(), Broker{URL: brokerURL, Token: "broker-token"}, http.DefaultTransport)
	if err != nil {
		t.Fatalf("openBrokeredSession: %v", err)
	}

	if err := SelectProvider(context.Background(), session, ProviderSelection{ProviderID: 42}); err != nil {
		t.Fatalf("selecting the session's own provider: %v", err)
	}
	if err := SelectProvider(context.Background(), session, ProviderSelection{ProviderID: 99}); !errors.Is(err, ErrInvalidProviderSelection) {
		t.Fatalf("err=%v, want ErrInvalidProviderSelection", err)
	}
	if strings.Join(methods, " ") != "GET GET GET" {
		t.Fatalf("broker saw %v, want only session reads", methods)
	}

	cache := t.TempDir()
	t.Setenv("ASC_WEB_SESSION_CACHE", "1")
	t.Setenv("ASC_WEB_SESSION_CACHE_BACKEND", "file")
	t.Setenv("ASC_WEB_SESSION_CACHE_DIR", cache)
	jar, _ := cookiejar.New(nil)
	jar.SetCookies(&url.URL{Scheme: "https", Host: "appstoreconnect.apple.com", Path: "/"}, []*http.Cookie{{Name: "myacinfo", Value: "v", Path: "/"}})
	session.Client.Jar = jar
	if err := PersistSession(session); err != nil {
		t.Fatalf("PersistSession: %v", err)
	}
	if entries, err := os.ReadDir(cache); err != nil || len(entries) != 0 {
		t.Fatalf("brokered session reached the cache: entries=%v err=%v", entries, err)
	}
}
