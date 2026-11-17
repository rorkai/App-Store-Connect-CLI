package web

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
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

	want := []string{
		"GET /proxy/appstoreconnect.apple.com/olympus/v1/session?  ",
		`POST /proxy/appstoreconnect.apple.com/iris/v1/resolutionCenterThreads/a%2Fb/messages?include=x&limit=2 itc {"m":1}`,
		"GET /proxy/appstoreconnect.apple.com/iris/v1/moved?  ",
		"GET /proxy/appstoreconnect.apple.com/iris/v1/target?  ",
	}
	if strings.Join(seen, "\n") != strings.Join(want, "\n") {
		t.Fatalf("broker saw:\n%s\nwant:\n%s", strings.Join(seen, "\n"), strings.Join(want, "\n"))
	}
	if direct != 1 {
		t.Fatalf("direct requests=%d, want 1", direct)
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
