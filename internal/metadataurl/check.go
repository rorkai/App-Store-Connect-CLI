// Package metadataurl provides bounded, read-only checks for public metadata
// URL destinations.
package metadataurl

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const (
	// CheckConcurrency bounds simultaneous destination requests.
	CheckConcurrency = 4
	// CheckTimeout bounds each destination request when no ASC timeout override
	// is configured.
	CheckTimeout = 10 * time.Second
	// MaxRedirects bounds the number of HTTP(S) redirects followed.
	MaxRedirects = 10
)

// ErrUnsafeTarget reports a destination that is not safe to contact as a
// public metadata URL.
var ErrUnsafeTarget = errors.New("metadata URL target is not a public internet address")

// Result contains the response metadata needed by callers. Response bodies
// are never retained.
type Result struct {
	FinalURL       *url.URL
	StatusCode     int
	RedirectedHost bool
}

// Outcome is the result of checking one destination.
type Outcome struct {
	Result Result
	Err    error
}

// Checker checks one HTTP(S) destination.
type Checker interface {
	Check(context.Context, string) (Result, error)
}

// CheckAll checks each unique, trimmed URL once, with bounded concurrency.
// The returned map is keyed by the trimmed URL supplied to the checker. An
// individual request failure is retained in Outcome and does not abort the
// remaining checks. Context cancellation is returned to the caller.
func CheckAll(ctx context.Context, checker Checker, rawURLs []string) (map[string]Outcome, error) {
	outcomes := make(map[string]Outcome, len(rawURLs))
	if len(rawURLs) == 0 {
		return outcomes, nil
	}
	if checker == nil {
		return nil, errors.New("metadata URL checker is nil")
	}

	unique := make(map[string]struct{}, len(rawURLs))
	for _, rawURL := range rawURLs {
		if trimmed := strings.TrimSpace(rawURL); trimmed != "" {
			unique[trimmed] = struct{}{}
		}
	}
	urls := make([]string, 0, len(unique))
	for rawURL := range unique {
		urls = append(urls, rawURL)
	}
	sort.Strings(urls)
	if len(urls) == 0 {
		return outcomes, nil
	}

	jobs := make(chan string)
	var workers sync.WaitGroup
	var outcomesMu sync.Mutex
	workerCount := min(CheckConcurrency, len(urls))
	workers.Add(workerCount)
	for range workerCount {
		go func() {
			defer workers.Done()
			for rawURL := range jobs {
				result, err := checker.Check(ctx, rawURL)
				outcomesMu.Lock()
				outcomes[rawURL] = Outcome{Result: result, Err: err}
				outcomesMu.Unlock()
			}
		}()
	}
	for _, rawURL := range urls {
		select {
		case jobs <- rawURL:
		case <-ctx.Done():
			close(jobs)
			workers.Wait()
			return nil, ctx.Err()
		}
	}
	close(jobs)
	workers.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return outcomes, nil
}

// NewChecker returns the production bounded HTTP checker.
func NewChecker() Checker {
	return &httpChecker{client: newHTTPClient()}
}

// newHTTPClient returns the configured bounded HTTP client.
func newHTTPClient() *http.Client {
	timeout := asc.ResolveTimeoutWithDefault(CheckTimeout)
	dialer := &net.Dialer{
		Timeout:        timeout,
		KeepAlive:      30 * time.Second,
		ControlContext: PublicDialControl,
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = dialer.DialContext
	transport.DisableKeepAlives = true
	transport.MaxConnsPerHost = CheckConcurrency
	transport.ResponseHeaderTimeout = timeout
	transport.TLSHandshakeTimeout = timeout
	transport.MaxResponseHeaderBytes = 1 << 20

	return &http.Client{
		Transport:     transport,
		Timeout:       timeout,
		CheckRedirect: RedirectPolicy,
	}
}

// CheckWithClient performs one bounded request using client. It is useful for
// package adapters that need a custom RoundTripper while retaining the shared
// request contract.
func CheckWithClient(ctx context.Context, client *http.Client, rawURL string) (Result, error) {
	if client == nil {
		return Result{}, errors.New("metadata URL HTTP client is nil")
	}
	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	currentURL, err := url.Parse(rawURL)
	if err != nil {
		return Result{}, err
	}
	if err := validateTargetURL(currentURL); err != nil {
		return Result{}, err
	}
	initialHost := strings.ToLower(currentURL.Hostname())

	// Do not let net/http follow redirects: its automatic path reads a portion
	// of each redirect body before closing it. Build a fresh request for every
	// hop so credentials, cookies, and referrer headers cannot cross origins.
	safeClient := *client
	safeClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	safeClient.Jar = nil

	redirectedHost := false
	for redirects := 0; ; {
		req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, currentURL.String(), nil)
		if err != nil {
			return Result{}, err
		}
		req.Header.Set("User-Agent", "asc metadata validate")

		resp, err := safeClient.Do(req)
		if err != nil {
			return Result{}, err
		}
		statusCode := resp.StatusCode
		location := resp.Header.Get("Location")
		if resp.Body != nil {
			_ = resp.Body.Close()
		}

		if !isRedirectStatus(statusCode) || location == "" {
			finalURL := *currentURL
			return Result{
				FinalURL:       &finalURL,
				StatusCode:     statusCode,
				RedirectedHost: redirectedHost,
			}, nil
		}
		if redirects >= MaxRedirects {
			return Result{}, fmt.Errorf("metadata URL exceeded %d redirects", MaxRedirects)
		}

		nextURL, err := currentURL.Parse(location)
		if err != nil {
			return Result{}, err
		}
		if err := validateTargetURL(nextURL); err != nil {
			return Result{}, err
		}
		if strings.ToLower(nextURL.Hostname()) != initialHost {
			redirectedHost = true
		}
		currentURL = nextURL
		redirects++
	}
}

type httpChecker struct {
	client *http.Client
}

func (c *httpChecker) Check(ctx context.Context, rawURL string) (Result, error) {
	return CheckWithClient(ctx, c.client, rawURL)
}

// RedirectPolicy rejects non-HTTP(S), userinfo, unsafe literal IP, and
// excessive redirects. DNS-resolved destinations are checked again by
// PublicDialControl.
func RedirectPolicy(req *http.Request, via []*http.Request) error {
	if len(via) >= MaxRedirects {
		return fmt.Errorf("metadata URL exceeded %d redirects", MaxRedirects)
	}
	if req == nil {
		return ErrUnsafeTarget
	}
	return validateTargetURL(req.URL)
}

func validateTargetURL(parsed *url.URL) error {
	if !IsHTTPURL(parsed) || parsed.User != nil {
		return ErrUnsafeTarget
	}
	if ip, err := netip.ParseAddr(parsed.Hostname()); err == nil && !IsPublicIP(ip) {
		return ErrUnsafeTarget
	}
	return nil
}

func isRedirectStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther,
		http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return true
	default:
		return false
	}
}

// IsHTTPURL reports whether parsed is an absolute HTTP(S) URL with a host.
func IsHTTPURL(parsed *url.URL) bool {
	if parsed == nil || parsed.Hostname() == "" {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

// PublicDialControl rejects every resolved non-public address, including
// addresses that otherwise look global-unicast but belong to special-purpose
// IANA ranges.
func PublicDialControl(_ context.Context, _, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return ErrUnsafeTarget
	}
	parsed, err := netip.ParseAddr(host)
	if err != nil || !IsPublicIP(parsed) {
		return ErrUnsafeTarget
	}
	return nil
}

// These IANA special-purpose ranges can still satisfy netip.Addr.IsGlobalUnicast.
// Translation prefixes stay blocked because they can encode non-public IPv4 targets.
var reservedIPPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("::ffff:0:0:0/96"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("100:0:0:1::/64"),
	netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001::/32"),
	netip.MustParsePrefix("2001:2::/48"),
	netip.MustParsePrefix("2001:10::/28"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("3ffe::/16"),
	netip.MustParsePrefix("3fff::/20"),
	netip.MustParsePrefix("5f00::/8"),
	netip.MustParsePrefix("fec0::/10"),
}

// Check these IANA globally reachable allocations before their broader parent ranges.
var globallyReachableIPExceptions = []netip.Prefix{
	netip.MustParsePrefix("192.0.0.9/32"),
	netip.MustParsePrefix("192.0.0.10/32"),
	netip.MustParsePrefix("2001:1::1/128"),
	netip.MustParsePrefix("2001:1::2/128"),
	netip.MustParsePrefix("2001:1::3/128"),
	netip.MustParsePrefix("2001:3::/32"),
	netip.MustParsePrefix("2001:4:112::/48"),
	netip.MustParsePrefix("2001:20::/28"),
	netip.MustParsePrefix("2001:30::/28"),
}

// IsPublicIP reports whether address is suitable for an outbound metadata
// destination.
func IsPublicIP(address netip.Addr) bool {
	address = address.Unmap()
	if !address.IsValid() || !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() {
		return false
	}
	for _, prefix := range globallyReachableIPExceptions {
		if prefix.Contains(address) {
			return true
		}
	}
	for _, prefix := range reservedIPPrefixes {
		if prefix.Contains(address) {
			return false
		}
	}
	return true
}
