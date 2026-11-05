package metadataurl

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

type fakeChecker struct {
	mu    sync.Mutex
	calls map[string]int
}

func (f *fakeChecker) Check(_ context.Context, rawURL string) (Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.calls == nil {
		f.calls = make(map[string]int)
	}
	f.calls[rawURL]++
	return Result{FinalURL: mustParseURL(rawURL), StatusCode: http.StatusOK}, nil
}

func TestCheckAllTrimsSortsAndDeduplicatesURLs(t *testing.T) {
	checker := &fakeChecker{}
	outcomes, err := CheckAll(context.Background(), checker, []string{
		" https://example.com/b ",
		"https://example.com/a",
		"https://example.com/a",
		" ",
	})
	if err != nil {
		t.Fatalf("CheckAll() error: %v", err)
	}
	if len(outcomes) != 2 || checker.calls["https://example.com/a"] != 1 || checker.calls["https://example.com/b"] != 1 {
		t.Fatalf("outcomes = %+v, calls = %+v, want two unique trimmed checks", outcomes, checker.calls)
	}
}

func TestCheckAllPreservesIndividualErrorsAndContextCancellation(t *testing.T) {
	checker := checkerFunc(func(ctx context.Context, rawURL string) (Result, error) {
		if rawURL == "https://example.com/fail" {
			return Result{}, errors.New("request failed")
		}
		return Result{FinalURL: mustParseURL(rawURL), StatusCode: http.StatusOK}, nil
	})
	outcomes, err := CheckAll(context.Background(), checker, []string{"https://example.com/fail"})
	if err != nil {
		t.Fatalf("CheckAll() error: %v", err)
	}
	if outcomes["https://example.com/fail"].Err == nil {
		t.Fatal("expected individual request error to be retained")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = CheckAll(ctx, checker, []string{"https://example.com/cancel"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("CheckAll() error = %v, want context cancellation", err)
	}
}

type checkerFunc func(context.Context, string) (Result, error)

func (f checkerFunc) Check(ctx context.Context, rawURL string) (Result, error) {
	return f(ctx, rawURL)
}

func TestRedirectPolicyRejectsUnsafeAndExcessiveRedirects(t *testing.T) {
	publicRequest, err := http.NewRequest(http.MethodGet, "https://8.8.8.8/support", nil)
	if err != nil {
		t.Fatalf("http.NewRequest() error: %v", err)
	}
	if err := RedirectPolicy(publicRequest, nil); err != nil {
		t.Fatalf("RedirectPolicy() public error: %v", err)
	}

	privateRequest, err := http.NewRequest(http.MethodGet, "http://127.0.0.1/support", nil)
	if err != nil {
		t.Fatalf("http.NewRequest() error: %v", err)
	}
	if err := RedirectPolicy(privateRequest, nil); !errors.Is(err, ErrUnsafeTarget) {
		t.Fatalf("RedirectPolicy() private error = %v, want ErrUnsafeTarget", err)
	}
	userinfoRequest, err := http.NewRequest(http.MethodGet, "https://user:password@example.com/support", nil)
	if err != nil {
		t.Fatalf("http.NewRequest() error: %v", err)
	}
	if err := RedirectPolicy(userinfoRequest, nil); !errors.Is(err, ErrUnsafeTarget) {
		t.Fatalf("RedirectPolicy() userinfo error = %v, want ErrUnsafeTarget", err)
	}

	via := make([]*http.Request, MaxRedirects)
	if err := RedirectPolicy(publicRequest, via); err == nil || err.Error() != "metadata URL exceeded 10 redirects" {
		t.Fatalf("RedirectPolicy() limit error = %v, want redirect limit", err)
	}
}

func TestPublicDialControlRejectsPrivateAddresses(t *testing.T) {
	if err := PublicDialControl(context.Background(), "tcp4", "127.0.0.1:443", nil); !errors.Is(err, ErrUnsafeTarget) {
		t.Fatalf("PublicDialControl() private error = %v, want ErrUnsafeTarget", err)
	}
	if err := PublicDialControl(context.Background(), "tcp4", "8.8.8.8:443", nil); err != nil {
		t.Fatalf("PublicDialControl() public error = %v", err)
	}
}

func TestIsPublicIP(t *testing.T) {
	tests := map[string]bool{
		"8.8.8.8":              true,
		"2606:4700::1111":      true,
		"0.0.0.0":              false,
		"10.0.0.1":             false,
		"100.64.0.1":           false,
		"127.0.0.1":            false,
		"169.254.1.1":          false,
		"192.0.0.9":            true,
		"192.0.0.10":           true,
		"192.0.2.1":            false,
		"192.88.99.1":          false,
		"192.88.99.2":          false,
		"198.18.0.1":           false,
		"203.0.113.1":          false,
		"224.0.0.1":            false,
		"255.255.255.255":      false,
		"::1":                  false,
		"fc00::1":              false,
		"fe80::1":              false,
		"::ffff:0:192.168.1.1": false,
		"64:ff9b::a9fe:a9fe":   false,
		"100:0:0:1::1":         false,
		"2001::1":              false,
		"2001:1::1":            true,
		"2001:2::1":            false,
		"2001:3::1":            true,
		"2001:4:112::1":        true,
		"2001:20::1":           true,
		"2001:30::1":           true,
		"2001:100::1":          false,
		"2002:a9fe:a9fe::1":    false,
		"2001:db8::1":          false,
		"3ffe::1":              false,
		"3fff::1":              false,
		"5f00::1":              false,
		"5f01::1":              false,
		"fec0::1":              false,
	}
	for rawIP, want := range tests {
		t.Run(rawIP, func(t *testing.T) {
			address := netip.MustParseAddr(rawIP)
			if got := IsPublicIP(address); got != want {
				t.Fatalf("IsPublicIP(%s) = %t, want %t", rawIP, got, want)
			}
		})
	}
}

func TestCheckWithClientUsesOrdinaryGET(t *testing.T) {
	var request *http.Request
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		request = req
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("ignored body")),
			Request:    req,
		}, nil
	})}

	result, err := CheckWithClient(context.Background(), client, "https://example.com/support")
	if err != nil {
		t.Fatalf("CheckWithClient() error: %v", err)
	}
	if request == nil || request.Method != http.MethodGet {
		t.Fatalf("expected GET request, got %+v", request)
	}
	if request.Header.Get("Range") != "" {
		t.Fatalf("expected an ordinary GET without a Range header, got %q", request.Header.Get("Range"))
	}
	if result.StatusCode != http.StatusOK || result.FinalURL.String() != "https://example.com/support" {
		t.Fatalf("unexpected check result: %+v", result)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestCheckWithClientRejectsInitialUserinfoBeforeRoundTrip(t *testing.T) {
	var requests atomic.Int64
	var authorization atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		authorization.Store(r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	transport := newMetadataURLTestTransport(t, map[string]string{
		"origin.example.test": server.URL,
	}, nil)
	client := &http.Client{Transport: transport}

	_, err := CheckWithClient(context.Background(), client, "http://user:password@origin.example.test/start")
	if !errors.Is(err, ErrUnsafeTarget) {
		t.Fatalf("CheckWithClient() error = %v, want ErrUnsafeTarget", err)
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("server requests = %d, want zero", got)
	}
	if got := authorization.Load(); got != nil {
		t.Fatalf("server Authorization = %v, want no request", got)
	}
}

func TestCheckWithClientRejectsRedirectUserinfoBeforeNextRequest(t *testing.T) {
	var destinationRequests atomic.Int64
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "http://user:password@destination.example.test/final")
		w.Header().Set("Content-Length", "0")
		w.WriteHeader(http.StatusFound)
	}))
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		destinationRequests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(origin.Close)
	t.Cleanup(destination.Close)

	transport := newMetadataURLTestTransport(t, map[string]string{
		"origin.example.test":      origin.URL,
		"destination.example.test": destination.URL,
	}, nil)
	client := &http.Client{Transport: transport}

	_, err := CheckWithClient(context.Background(), client, "http://origin.example.test/start")
	if !errors.Is(err, ErrUnsafeTarget) {
		t.Fatalf("CheckWithClient() error = %v, want ErrUnsafeTarget", err)
	}
	if got := destinationRequests.Load(); got != 0 {
		t.Fatalf("destination requests = %d, want zero", got)
	}
}

func TestCheckWithClientDoesNotReadRedirectBodiesOrForwardHeaders(t *testing.T) {
	var destinationAuthorization atomic.Value
	var destinationCookie atomic.Value
	var destinationProxyAuthorization atomic.Value
	var destinationReferer atomic.Value
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		destinationAuthorization.Store(r.Header.Get("Authorization"))
		destinationCookie.Store(r.Header.Get("Cookie"))
		destinationProxyAuthorization.Store(r.Header.Get("Proxy-Authorization"))
		destinationReferer.Store(r.Header.Get("Referer"))
		w.Header().Set("Content-Length", "5")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "final")
	}))
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "http://destination.example.test/final")
		body := "redirect body that must not be read"
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.WriteHeader(http.StatusFound)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(destination.Close)
	t.Cleanup(origin.Close)

	stats := &metadataURLBodyStats{}
	transport := newMetadataURLTestTransport(t, map[string]string{
		"origin.example.test":      origin.URL,
		"destination.example.test": destination.URL,
	}, stats)
	client := &http.Client{Transport: transport}

	result, err := CheckWithClient(context.Background(), client, "http://origin.example.test/start")
	if err != nil {
		t.Fatalf("CheckWithClient() error = %v", err)
	}
	if result.StatusCode != http.StatusOK {
		t.Fatalf("result.StatusCode = %d, want %d", result.StatusCode, http.StatusOK)
	}
	if got := stats.reads.Load(); got != 0 {
		t.Fatalf("redirect response body reads = %d, want zero", got)
	}
	if got := stats.closes.Load(); got != 2 {
		t.Fatalf("response body closes = %d, want 2", got)
	}
	for name, value := range map[string]*atomic.Value{
		"Authorization":       &destinationAuthorization,
		"Cookie":              &destinationCookie,
		"Proxy-Authorization": &destinationProxyAuthorization,
		"Referer":             &destinationReferer,
	} {
		if got := value.Load(); got != "" {
			t.Errorf("destination %s = %q, want empty", name, got)
		}
	}
}

func TestCheckWithClientRejectsMalformedAndUnsupportedRedirectTargets(t *testing.T) {
	for name, location := range map[string]string{
		"unsupported scheme": "ftp://destination.example.test/final",
		"malformed URL":      "http://%zz",
	} {
		t.Run(name, func(t *testing.T) {
			var destinationRequests atomic.Int64
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", location)
				w.Header().Set("Content-Length", "0")
				w.WriteHeader(http.StatusFound)
			}))
			destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				destinationRequests.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			t.Cleanup(origin.Close)
			t.Cleanup(destination.Close)

			transport := newMetadataURLTestTransport(t, map[string]string{
				"origin.example.test":      origin.URL,
				"destination.example.test": destination.URL,
			}, nil)
			_, err := CheckWithClient(context.Background(), &http.Client{Transport: transport}, "http://origin.example.test/start")
			if err == nil {
				t.Fatal("CheckWithClient() error = nil, want rejected redirect target")
			}
			if name == "unsupported scheme" && !errors.Is(err, ErrUnsafeTarget) {
				t.Fatalf("CheckWithClient() error = %v, want ErrUnsafeTarget", err)
			}
			if got := destinationRequests.Load(); got != 0 {
				t.Fatalf("destination requests = %d, want zero", got)
			}
		})
	}
}

func TestCheckWithClientBoundsRedirectLoopWithoutReadingBodies(t *testing.T) {
	var requests atomic.Int64
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		body := "redirect body"
		w.Header().Set("Location", "http://origin.example.test/next")
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.WriteHeader(http.StatusFound)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(origin.Close)

	stats := &metadataURLBodyStats{}
	transport := newMetadataURLTestTransport(t, map[string]string{
		"origin.example.test": origin.URL,
	}, stats)
	_, err := CheckWithClient(context.Background(), &http.Client{Transport: transport}, "http://origin.example.test/start")
	if err == nil || err.Error() != "metadata URL exceeded 10 redirects" {
		t.Fatalf("CheckWithClient() error = %v, want redirect limit", err)
	}
	if got := requests.Load(); got > int64(MaxRedirects+1) {
		t.Fatalf("redirect requests = %d, want at most %d", got, MaxRedirects+1)
	}
	if got := stats.reads.Load(); got != 0 {
		t.Fatalf("redirect response body reads = %d, want zero", got)
	}
}

func TestCheckWithClientTreatsRedirectWithoutLocationAsFinalResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := "redirect body"
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.WriteHeader(http.StatusFound)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)
	stats := &metadataURLBodyStats{}
	transport := newMetadataURLTestTransport(t, map[string]string{
		"origin.example.test": server.URL,
	}, stats)

	result, err := CheckWithClient(context.Background(), &http.Client{Transport: transport}, "http://origin.example.test/start")
	if err != nil {
		t.Fatalf("CheckWithClient() error = %v", err)
	}
	if result.StatusCode != http.StatusFound {
		t.Fatalf("result.StatusCode = %d, want %d", result.StatusCode, http.StatusFound)
	}
	if got := result.FinalURL.String(); got != "http://origin.example.test/start" {
		t.Fatalf("result.FinalURL = %q, want original URL", got)
	}
	if got := stats.reads.Load(); got != 0 {
		t.Fatalf("redirect response body reads = %d, want zero", got)
	}
	if got := stats.closes.Load(); got != 1 {
		t.Fatalf("redirect response body closes = %d, want one", got)
	}
}

func TestCheckWithClientTracksCrossHostRedirectEvenWhenItReturns(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			w.Header().Set("Location", "http://destination.example.test/middle")
			w.Header().Set("Content-Length", "0")
			w.WriteHeader(http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "http://origin.example.test/final")
		w.Header().Set("Content-Length", "0")
		w.WriteHeader(http.StatusFound)
	}))
	t.Cleanup(origin.Close)
	t.Cleanup(destination.Close)

	transport := newMetadataURLTestTransport(t, map[string]string{
		"origin.example.test":      origin.URL,
		"destination.example.test": destination.URL,
	}, nil)
	result, err := CheckWithClient(context.Background(), &http.Client{Transport: transport}, "http://origin.example.test/start")
	if err != nil {
		t.Fatalf("CheckWithClient() error = %v", err)
	}
	if !result.RedirectedHost {
		t.Fatal("result.RedirectedHost = false, want true")
	}
	if got := result.FinalURL.String(); got != "http://origin.example.test/final" {
		t.Fatalf("result.FinalURL = %q, want original host final URL", got)
	}
}

func TestCheckWithClientDoesNotMarkSameHostRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			w.Header().Set("Location", "/final")
			w.Header().Set("Content-Length", "0")
			w.WriteHeader(http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	transport := newMetadataURLTestTransport(t, map[string]string{
		"origin.example.test": server.URL,
	}, nil)
	result, err := CheckWithClient(context.Background(), &http.Client{Transport: transport}, "http://origin.example.test/start")
	if err != nil {
		t.Fatalf("CheckWithClient() error = %v", err)
	}
	if result.RedirectedHost {
		t.Fatal("result.RedirectedHost = true, want false")
	}
}

type metadataURLBodyStats struct {
	reads  atomic.Int64
	closes atomic.Int64
}

type metadataURLTrackingBody struct {
	io.ReadCloser
	stats *metadataURLBodyStats
}

func (b *metadataURLTrackingBody) Read(p []byte) (int, error) {
	b.stats.reads.Add(1)
	return b.ReadCloser.Read(p)
}

func (b *metadataURLTrackingBody) Close() error {
	b.stats.closes.Add(1)
	return b.ReadCloser.Close()
}

type metadataURLTestTransport struct {
	t                *testing.T
	base             http.RoundTripper
	routes           map[string]*url.URL
	stats            *metadataURLBodyStats
	injectedFirstHop atomic.Bool
}

func newMetadataURLTestTransport(t *testing.T, routes map[string]string, stats *metadataURLBodyStats) http.RoundTripper {
	t.Helper()
	parsedRoutes := make(map[string]*url.URL, len(routes))
	for host, rawURL := range routes {
		parsed, err := url.Parse(rawURL)
		if err != nil {
			t.Fatalf("url.Parse(%q): %v", rawURL, err)
		}
		parsedRoutes[host] = parsed
	}
	if stats == nil {
		stats = &metadataURLBodyStats{}
	}
	base := http.DefaultTransport.(*http.Transport).Clone()
	base.Proxy = nil
	return &metadataURLTestTransport{
		t:      t,
		base:   base,
		routes: parsedRoutes,
		stats:  stats,
	}
}

func (t *metadataURLTestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.t.Helper()
	route, ok := t.routes[req.URL.Hostname()]
	if !ok {
		return nil, errors.New("metadata URL test route not found")
	}
	request := req.Clone(req.Context())
	requestURL := *req.URL
	requestURL.Scheme = route.Scheme
	requestURL.Host = route.Host
	requestURL.User = nil
	request.URL = &requestURL
	request.Host = ""
	if t.injectedFirstHop.CompareAndSwap(false, true) {
		request.Header.Set("Authorization", "Bearer first-hop")
		request.Header.Set("Cookie", "session=first-hop")
		request.Header.Set("Proxy-Authorization", "Basic first-hop")
		request.Header.Set("Referer", "https://referrer.example.test/source")
	}
	response, err := t.base.RoundTrip(request)
	if err != nil {
		return nil, err
	}
	response.Request = req
	response.Body = &metadataURLTrackingBody{ReadCloser: response.Body, stats: t.stats}
	return response, nil
}

func mustParseURL(rawURL string) *url.URL {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}
	return parsed
}
